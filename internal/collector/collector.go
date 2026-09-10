package collector

import "fmt"

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
//  3. resolve each row's session's `path` via ticket 13's full method —
//     touched-paths majority vote first, falling back to the simple
//     method (path.go: first cwd-bearing record -> gitRootResolver -> raw
//     cwd fallback), falling back further to "(unknown)" — computed once
//     per session_id, not once per row (scanSessionPaths below)
//  4. resolve each row's `actor` from that same cwd (actor.go) — ticket
//     13 does not change actor derivation, only path
//  5. drop rows this install has already successfully sent (state.go)
//  6. POST the remainder as one batch to poster, then mark them sent and
//     persist statePath — only after a successful POST, so a failed
//     send is retried on the next run rather than silently lost
//
// gitRootResolver and poster are both injected (not hardcoded to
// RealGitRootResolver / a real *Client) so this whole pipeline is
// testable without a subprocess or a network call — collector_test.go's
// own integration-style unit tests exercise exactly this function.
// pathStorePath is ticket 13's own local SQLite cache (git_root_cache +
// session_path_votes + session_scan_progress, pathstore.go) — a sibling
// file next to statePath, the same pattern cmd/collector/main.go already
// uses for statePath itself.
func Run(cfg Config, statePath string, pathStorePath string, gitRootResolver GitRootResolver, hostname string, poster BatchPoster) (RunResult, error) {
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

	pathStore, err := OpenPathStore(pathStorePath)
	if err != nil {
		return result, err
	}
	defer pathStore.Close()

	pathBySession, err := scanSessionPaths(files, allRows, pathStore, gitRootResolver)
	if err != nil {
		return result, err
	}

	// `actor` stays session-cwd-based, unchanged from tickets 7/8/12 —
	// ticket 13 only upgrades `path`.
	actorBySession := make(map[string]string)
	actorForSession := func(sessionID string) string {
		if a, ok := actorBySession[sessionID]; ok {
			return a
		}
		a := unknownActor
		if cwd, ok := FirstCwdBearingRow(allRows, sessionID); ok {
			if resolved, ok := ActorFromCwd(cwd); ok {
				a = resolved
			}
		}
		actorBySession[sessionID] = a
		return a
	}

	events := make([]Event, 0, len(newRows))
	for _, r := range newRows {
		path, actor := pathBySession[r.SessionID], actorForSession(r.SessionID)
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

// scanSessionPaths implements ticket 13's whole path-attribution pipeline
// for one Run: incrementally scans each file's lines that haven't been
// scanned yet (per pathStore's own persisted session_scan_progress) for
// touched paths, tallies their git-roots into pathStore's running
// per-session vote counts (session_path_votes) — caching every
// directory->git-root resolution along the way, seeded from and persisted
// back to pathStore's own git_root_cache, so a directory is never
// re-resolved via a real subprocess call twice, across runs as well as
// within one — then resolves every session referenced by rows to its
// final `path`: the majority-vote winner (MajorityGitRoot), falling back
// to the simple cwd method (ResolveSessionPath, tickets 7/8/12) when a
// session has no real touched-path votes at all, falling back further to
// the "(unknown)" sentinel when even that produces nothing usable.
func scanSessionPaths(files []string, rows []UsageRow, store *PathStore, gitRootResolver GitRootResolver) (map[string]string, error) {
	cache := NewGitRootCache()
	seed, err := store.AllGitRoots()
	if err != nil {
		return nil, err
	}
	cache.Seed(seed)
	cachedResolver := CachedGitRootResolver(gitRootResolver, cache)

	for _, f := range files {
		lines, err := ReadTranscriptLines(f)
		if err != nil {
			return nil, fmt.Errorf("reading %s for touched-path scanning: %w", f, err)
		}
		sessionID, ok := firstSessionID(lines)
		if !ok {
			continue // no sessionId anywhere in this file — nothing to attribute (ticket 9's noop-session edge case)
		}

		startLine, err := store.ScanOffset(sessionID, f)
		if err != nil {
			return nil, err
		}
		totalLines := int64(len(lines))
		if startLine > totalLines {
			startLine = totalLines // defensive: transcripts are append-only in practice, but never trust a stale offset past EOF
		}

		if newLines := lines[startLine:]; len(newLines) > 0 {
			if touched := ExtractTouchedPaths(newLines); len(touched) > 0 {
				if votes := TallyGitRoots(touched, cachedResolver); len(votes) > 0 {
					if err := store.AddVotes(sessionID, votes); err != nil {
						return nil, err
					}
				}
			}
		}

		if err := store.SetScanOffset(sessionID, f, totalLines); err != nil {
			return nil, err
		}
	}

	if err := store.SaveGitRoots(cache.Snapshot()); err != nil {
		return nil, err
	}

	pathBySession := make(map[string]string)
	seenSessions := make(map[string]bool)
	for _, r := range rows {
		if seenSessions[r.SessionID] {
			continue
		}
		seenSessions[r.SessionID] = true

		voteCounts, err := store.VoteCounts(r.SessionID)
		if err != nil {
			return nil, err
		}

		cwd, cwdOK := FirstCwdBearingRow(rows, r.SessionID)
		// Same fallback-chain function path_test.go's own unit tests
		// exercise directly (path.go's resolvePathFromVoteCounts) — reading
		// vote counts back from PathStore instead of re-tallying fresh
		// touchedPaths is the only difference from ResolveSessionPathFull,
		// not a second copy of the fallback logic itself.
		pathBySession[r.SessionID] = resolvePathFromVoteCounts(voteCounts, cwd, cwdOK, cachedResolver)
	}

	return pathBySession, nil
}
