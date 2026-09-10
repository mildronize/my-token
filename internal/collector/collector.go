package collector

// unknownActor is the fallback used when a session's cwd doesn't match
// the `.typ-crews/<name>` pattern at all — mirrors the contract's
// console-side "(unknown)" final fallback convention for `path`, applied
// here to `actor` for the same reason: never silently drop a row just
// because one derived field couldn't be determined.
const unknownActor = "(unknown)"

// BatchPoster is the subset of *Client's behavior Run depends on — an
// interface so collector_test.go's unit tests can inject a fake instead
// of a real HTTP round trip (client_test.go already covers *Client's own
// wire behavior against a real httptest.Server).
type BatchPoster interface {
	PostBatch(events []Event) (BatchResult, error)
}

// RunResult summarizes one collector run, for the CLI to print and for
// tests to assert against.
type RunResult struct {
	FilesScanned    int
	TotalRowsFound  int // every deduped usage row this run's scan found, already-sent ones included
	NewRowsFound    int // rows not already in the local sent-state — what this run actually attempted to send
	Received        int64
	Inserted        int64
	EstimatedCostUS float64 // this run's own local pricing.go estimate for NewRowsFound, never the authoritative figure
}

// Run is the collector's whole pipeline for one invocation:
//
//  1. find every *.jsonl under cfg.ScanPaths (scan.go, subagents included)
//  2. parse and dedupe by message.id across all of them (transcript.go)
//  3. resolve each row's session's `path` via the simple method
//     (path.go: first cwd-bearing record -> gitRootResolver -> raw cwd
//     fallback), computed once per session_id, not once per row
//  4. resolve each row's `actor` from that same cwd (actor.go)
//  5. drop rows this install has already successfully sent (state.go)
//  6. POST the remainder as one batch to poster, then mark them sent and
//     persist statePath — only after a successful POST, so a failed
//     send is retried on the next run rather than silently lost
//
// gitRootResolver and poster are both injected (not hardcoded to
// RealGitRootResolver / a real *Client) so this whole pipeline is
// testable without a subprocess or a network call — collector_test.go's
// own integration-style unit tests exercise exactly this function.
func Run(cfg Config, statePath string, gitRootResolver GitRootResolver, hostname string, poster BatchPoster) (RunResult, error) {
	var result RunResult

	files, err := FindTranscriptFiles(cfg.ScanPaths)
	if err != nil {
		return result, err
	}
	result.FilesScanned = len(files)

	var allRows []UsageRow
	seenMessageIDs := make(map[string]bool)
	for _, f := range files {
		rows, err := ExtractUsageRowsFromFile(f)
		if err != nil {
			return result, err
		}
		for _, r := range rows {
			if seenMessageIDs[r.MessageID] {
				continue // the same message.id can appear in more than one scanned file in principle — dedupe globally, not per-file
			}
			seenMessageIDs[r.MessageID] = true
			allRows = append(allRows, r)
		}
	}
	result.TotalRowsFound = len(allRows)

	state, err := LoadSentState(statePath)
	if err != nil {
		return result, err
	}
	newRows := state.FilterUnsent(allRows)
	result.NewRowsFound = len(newRows)

	// Resolve `path` once per session_id (tickets 7/8: it's a
	// session-level fact, the session's *first* cwd-bearing record — not
	// recomputed per row, and not based on each row's own cwd, which
	// could differ from the session's first cwd if the assistant `cd`'d
	// partway through).
	pathBySession := make(map[string]string)
	actorBySession := make(map[string]string)
	resolvePathAndActor := func(sessionID string) (path, actor string) {
		if p, ok := pathBySession[sessionID]; ok {
			return p, actorBySession[sessionID]
		}
		cwd, ok := FirstCwdBearingRow(allRows, sessionID)
		if !ok {
			pathBySession[sessionID] = ""
			actorBySession[sessionID] = unknownActor
			return "", unknownActor
		}
		p := ResolveSessionPath(cwd, gitRootResolver)
		a, aok := ActorFromCwd(cwd)
		if !aok {
			a = unknownActor
		}
		pathBySession[sessionID] = p
		actorBySession[sessionID] = a
		return p, a
	}

	events := make([]Event, 0, len(newRows))
	for _, r := range newRows {
		path, actor := resolvePathAndActor(r.SessionID)
		events = append(events, Event{
			ID:                       r.MessageID,
			SessionID:                r.SessionID,
			Actor:                    actor,
			Path:                     path,
			Machine:                  cfg.InstallID,
			Model:                    r.Model,
			InputTokens:              r.InputTokens,
			OutputTokens:             r.OutputTokens,
			CacheReadInputTokens:     r.CacheReadInputTokens,
			CacheCreationInputTokens: r.CacheCreationInputTokens,
			Timestamp:                r.Timestamp,
		})
		result.EstimatedCostUS += CostForUsage(r.Model, r.InputTokens, r.OutputTokens, r.CacheReadInputTokens, r.CacheCreationInputTokens)
	}

	if len(events) == 0 {
		return result, nil
	}

	batchResult, err := poster.PostBatch(events)
	if err != nil {
		// Nothing marked sent, nothing saved — every one of these rows is
		// retried on the next run, exactly as if this run never happened.
		return result, err
	}
	result.Received = batchResult.Received
	result.Inserted = batchResult.Inserted

	for _, e := range events {
		state.MarkSent(e.ID)
	}
	if err := state.Save(statePath); err != nil {
		return result, err
	}

	return result, nil
}
