package usage

import (
	"context"
	"strings"
	"time"
)

// Source is fixed to "claude_code" for this story (goal: non-Claude-Code
// sources out of scope — contract's Data model). A package-level
// constant, not a caller-suppliable value anywhere in this package's own
// API, so there is no signature that could accidentally accept a
// caller-chosen source.
const Source = "claude_code"

// IngestEvent is IngestBatch's own per-event input shape — everything the
// transport layer (internal/transport/publicapi/usage_handler.go) reads
// off the request body, minus Cost and Source: this ticket's own
// requirement is that the server computes both itself and never trusts
// the client for either, so there is no field here a caller could set
// them through even if the transport layer forwarded one.
type IngestEvent struct {
	ID        string
	SessionID string
	Actor     string
	Path      string
	Machine   string
	Model     string
	// ScanRoot is the raw, resolved scan-root path the reporting
	// transcript file was found under (story-2/ticket-8, contract's Data
	// model) — distinct from Path, which is git-rooted from touched
	// files, not the transcript's own location.
	ScanRoot                 string
	InputTokens              int64
	OutputTokens             int64
	CacheReadInputTokens     int64
	CacheCreationInputTokens int64
	// Timestamp is the collector-supplied time the turn actually happened
	// (contract's request body: `timestamp`) — used as created_at.
	// Unlike Cost/Source this is a legitimate client-supplied field: the
	// collector reports usage after the fact, sometimes with a real delay
	// (batched incremental scans — ticket 12), so "when this was ingested"
	// (time.Now, server-side) would be the wrong value to store here.
	Timestamp time.Time
}

// Filter narrows Service.Summary/Service.Windows' own result set,
// AND-composed with the window bound and with each other when both
// fields are set (story-2/ticket-10, contract's API surface section:
// GET /usage/summary and GET /usage/windows both gain optional
// `machine`/`scan_root` query params). Both fields empty is the zero
// value and means "no filter at all" — every pre-ticket-10 caller of
// Summary/Windows behaves exactly as before by simply passing Filter{}.
//
//   - Machine is the raw install_id (bare, not a hostname) — the wire's
//     own `machine` query param, unchanged from how usage_events.machine
//     and group_by=machine's own raw key already work.
//   - ScanRoot is the wire's own composite `<install_id>:<scan_root_path>`
//     string — same machine-prefix convention story-2/ticket-7
//     established for group_by=path, for the identical reason a bare
//     scan-root path/name is ambiguous across machines (naming is
//     per-install, contract's Data model). Repo.ListEventsInWindow
//     (repo.go) is the only place this composite is ever decomposed into
//     its two halves — Service and every caller above it pass the single
//     wire-shaped string straight through, untouched.
type Filter struct {
	Machine  string
	ScanRoot string
}

// Service implements the usage domain contract (story-1/ticket-11) on
// top of a Repository. This package never resolves an actor itself (I4)
// — the transport layer authenticates the caller (the collector's own
// API key) before this is ever reached; the "actor" field on each
// IngestEvent below is an
// unrelated, purely domain-level fact about which crew produced the
// reported session, not an identity claim about who is calling this API.
type Service struct {
	Repo Repository
}

// NewService wires a Service on top of a Repository.
func NewService(repo Repository) *Service {
	return &Service{Repo: repo}
}

// IngestBatch computes each event's Cost (the ported pricing table, a
// pure function — pricing.go) and stamps Source itself, then delegates
// the idempotent insert-or-ignore to Repo. Returns (received, inserted):
// received is len(events) (how many events this batch named at all,
// duplicates included), inserted is how many were newly written — the
// difference is exactly how many were already-ingested repeats, without
// the caller needing a second query to find out.
func (s *Service) IngestBatch(ctx context.Context, events []IngestEvent) (received, inserted int, err error) {
	received = len(events)
	if received == 0 {
		return 0, 0, nil
	}

	batch := make([]Event, 0, len(events))
	for _, e := range events {
		batch = append(batch, Event{
			ID:                       e.ID,
			SessionID:                e.SessionID,
			Actor:                    e.Actor,
			Path:                     e.Path,
			Machine:                  e.Machine,
			Model:                    e.Model,
			ScanRoot:                 e.ScanRoot,
			InputTokens:              e.InputTokens,
			OutputTokens:             e.OutputTokens,
			CacheReadInputTokens:     e.CacheReadInputTokens,
			CacheCreationInputTokens: e.CacheCreationInputTokens,
			Cost:                     CostForUsage(e.Model, e.InputTokens, e.OutputTokens, e.CacheReadInputTokens, e.CacheCreationInputTokens),
			Source:                   Source,
			CreatedAt:                e.Timestamp,
		})
	}

	insertedRows, err := s.Repo.InsertBatch(ctx, batch)
	if err != nil {
		return received, 0, err
	}
	return received, int(insertedRows), nil
}

// Summary backs GET /api/bff/usage/summary (story-1/ticket-14): resolves
// window's own [start, now) range (WindowBounds, summary.go), fetches
// every event in it, and aggregates by groupBy (Aggregate, summary.go —
// the pure, unit-tested half of this method; this function's only job is
// wiring now/window into a database call). now is a parameter, not
// time.Now() called internally, so WindowBounds' calendar-anchoring is
// exercised deterministically by tests calling this method directly,
// without needing to fake a clock.
//
// filter (story-2/ticket-10) is passed straight through to
// Repo.ListEventsInWindow, unmodified — the repo layer is the only place
// filter.ScanRoot's composite is decomposed (repo.go). Because filtering
// happens before Aggregate ever runs, totals/breakdown/reporting_installs
// are all scoped together automatically (contract's "Console filters"
// section: global, not per-panel) — there is no separate "filter the
// totals" step to forget.
func (s *Service) Summary(ctx context.Context, window Window, groupBy GroupBy, filter Filter, now time.Time) (SummaryResult, error) {
	start, end, err := WindowBounds(window, now)
	if err != nil {
		return SummaryResult{}, err
	}

	events, err := s.Repo.ListEventsInWindow(ctx, start, end, filter)
	if err != nil {
		return SummaryResult{}, err
	}

	result := Aggregate(events, groupBy)

	switch groupBy {
	case GroupByMachine:
		if err := s.substituteMachineHostnames(ctx, result.Breakdown); err != nil {
			return SummaryResult{}, err
		}
	case GroupByPath:
		// story-2/ticket-7: the same hostname-substitution pass, extended
		// to group_by=path now that Aggregate's own keyFor(GroupByPath)
		// key is machine-prefixed (summary.go).
		if err := s.substituteMachineHostnamesInPathKeys(ctx, result.Breakdown); err != nil {
			return SummaryResult{}, err
		}
	case GroupByActor:
		// story-2/ticket-16: same fix, same reason, applied to actor now
		// that keyFor(GroupByActor) is machine-prefixed too.
		if err := s.substituteMachineHostnamesInActorKeys(ctx, result.Breakdown); err != nil {
			return SummaryResult{}, err
		}
	}

	return result, nil
}

// substituteMachineHostnames replaces each breakdown row's Key (currently
// the raw install_id Aggregate grouped by) with machines.hostname,
// stashing the original install_id in RawKey so the console can still
// show it (contract's "machine label" rule: hostname is the label,
// install_id stays reachable via tooltip — summary.go's BreakdownRow doc
// comment). A row whose install_id has no machines row at all (contract:
// "shouldn't normally happen given upsert-on-every-batch, but don't let
// that case produce a blank/null key") is left exactly as Aggregate
// produced it — Key stays the raw install_id, RawKey stays empty, so the
// console falls back to showing the install_id itself rather than a
// blank label.
func (s *Service) substituteMachineHostnames(ctx context.Context, breakdown []BreakdownRow) error {
	if len(breakdown) == 0 {
		return nil
	}
	hostnames, err := s.Repo.MachineHostnames(ctx)
	if err != nil {
		return err
	}
	for i := range breakdown {
		installID := breakdown[i].Key
		if hostname, ok := hostnames[installID]; ok && hostname != "" {
			breakdown[i].Key = hostname
			breakdown[i].RawKey = installID
		}
	}
	return nil
}

// substituteMachineHostnamesInPathKeys is substituteMachineHostnames'
// group_by=path counterpart (story-2/ticket-7, contract's "Cross-machine
// path identity" section): Aggregate's own GroupByPath key is
// `<install_id>:<path>` (summary.go's keyFor), not a bare install_id, so
// this substitutes only the install_id prefix — `<install_id>:<path>`
// becomes `<hostname>:<path>` — rather than replacing the whole key the
// way the group_by=machine pass does. RawKey stashes the original,
// unsubstituted `<install_id>:<path>` key so the console can still show
// the ground truth (same tooltip pattern as `machine`'s own RawKey). A
// row whose install_id has no machines row at all is left exactly as
// Aggregate produced it (Key stays `<install_id>:<path>`, RawKey stays
// empty) — same "never show a blank label" fallback as
// substituteMachineHostnames.
func (s *Service) substituteMachineHostnamesInPathKeys(ctx context.Context, breakdown []BreakdownRow) error {
	if len(breakdown) == 0 {
		return nil
	}
	hostnames, err := s.Repo.MachineHostnames(ctx)
	if err != nil {
		return err
	}
	for i := range breakdown {
		rawKey := breakdown[i].Key // "<install_id>:<path>", per keyFor(GroupByPath)
		installID, path, found := strings.Cut(rawKey, ":")
		if !found {
			continue // defensive: keyFor always produces machine+":"+path
		}
		if hostname, ok := hostnames[installID]; ok && hostname != "" {
			breakdown[i].Key = hostname + ":" + path
			breakdown[i].RawKey = rawKey
		}
	}
	return nil
}

// substituteMachineHostnamesInActorKeys is
// substituteMachineHostnamesInPathKeys' group_by=actor counterpart
// (story-2/ticket-16): Aggregate's own GroupByActor key is
// `<install_id>:<actor>` (summary.go's keyFor), identical shape to
// GroupByPath's own compound key — same substitution, same "never show a
// blank label" fallback, just over the actor half instead of the path
// half.
func (s *Service) substituteMachineHostnamesInActorKeys(ctx context.Context, breakdown []BreakdownRow) error {
	if len(breakdown) == 0 {
		return nil
	}
	hostnames, err := s.Repo.MachineHostnames(ctx)
	if err != nil {
		return err
	}
	for i := range breakdown {
		rawKey := breakdown[i].Key // "<install_id>:<actor>", per keyFor(GroupByActor)
		installID, actor, found := strings.Cut(rawKey, ":")
		if !found {
			continue // defensive: keyFor always produces machine+":"+actor
		}
		if hostname, ok := hostnames[installID]; ok && hostname != "" {
			breakdown[i].Key = hostname + ":" + actor
			breakdown[i].RawKey = rawKey
		}
	}
	return nil
}

// UpsertMachine records the reporting collector's own (installID,
// hostname) pair, refreshing last_seen_at to now — story-1/ticket-18:
// called on every POST /api/v1/usage-events/batch
// (internal/transport/publicapi/usage_handler.go's
// IngestUsageEventsBatch), additive to IngestBatch, not a replacement for
// it (contract's Data model: `machines` is "added on top of ticket 11's
// original design", usage_events.machine itself is untouched).
func (s *Service) UpsertMachine(ctx context.Context, installID, hostname string, now time.Time) error {
	return s.Repo.UpsertMachine(ctx, installID, hostname, now)
}

// UpsertScanRootInput is Service.UpsertScanRoots' own per-entry input
// shape, mirroring the batch's own scan_roots wire array (story-2/
// ticket-8, contract's API surface: "scan_roots: [{path, name,
// source_type}]"). Named UpsertScanRootInput, not the wire type's own
// ScanRootInput (api.ScanRootInput, internal/api/openapi.gen.go) —
// distinct names at each layer, the same convention the sibling event
// flow already follows (api.UsageEventInput -> usage.IngestEvent ->
// usage.Event), so a reader/grep never has to guess which package's type
// is meant.
type UpsertScanRootInput struct {
	Path       string
	Name       string
	SourceType string
}

// UpsertScanRoots upserts every entry of one ingestion batch's own
// scan_roots array into the scan_roots table, refreshing last_seen_at to
// now for each — story-2/ticket-8: called on every POST
// /api/v1/usage-events/batch (internal/transport/publicapi/
// usage_handler.go's IngestUsageEventsBatch), additive to IngestBatch,
// not a replacement for it, mirroring UpsertMachine above exactly except
// for handling a whole array (scan_roots is batch-level but can name
// more than one root, unlike hostname).
func (s *Service) UpsertScanRoots(ctx context.Context, installID string, scanRoots []UpsertScanRootInput, now time.Time) error {
	for _, sr := range scanRoots {
		if err := s.Repo.UpsertScanRoot(ctx, installID, sr.Path, sr.Name, sr.SourceType, now); err != nil {
			return err
		}
	}
	return nil
}

// ScanRootWithHostname is one row of GET /api/bff/usage/scan-roots
// (story-2/ticket-9, contract's API surface) — a scan_roots row joined,
// in Go rather than via a SQL JOIN, with its machine's hostname. Named
// distinctly from both ScanRootRecord (repo.go, the raw un-joined row)
// and the wire ScanRoot type (internal/bffapi, generated), same
// "distinct name per layer" convention this package already follows.
//
// SourceType (story-3/ticket-2) passes ScanRootRecord's own SourceType
// straight through — no join, no transformation, same as Name/
// ScanRootPath below.
type ScanRootWithHostname struct {
	InstallID    string
	Hostname     string
	ScanRootPath string
	Name         string
	SourceType   string
}

// ScanRoots backs GET /api/bff/usage/scan-roots (story-2/ticket-9): every
// registered scan_roots row across every reporting install, each joined
// with machines.hostname the same way Service.Summary's own
// group_by=machine/group_by=path substitution already does — two
// separate repo reads (ListScanRoots, MachineHostnames) combined here in
// Go, not a SQL JOIN (bff-openapi.yaml's own operation doc explains why:
// consistency with that existing precedent, not a new pattern). A
// scan_roots row whose install_id has no corresponding machines row
// falls back to showing the raw install_id as Hostname — the same
// "never a blank label" fallback substituteMachineHostnames already
// establishes — rather than an empty string or an error. No scan roots
// registered at all short-circuits to an empty slice without even
// calling MachineHostnames, mirroring Summary's own len(breakdown)==0
// guard on its substitution passes.
func (s *Service) ScanRoots(ctx context.Context) ([]ScanRootWithHostname, error) {
	records, err := s.Repo.ListScanRoots(ctx)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return []ScanRootWithHostname{}, nil
	}

	hostnames, err := s.Repo.MachineHostnames(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]ScanRootWithHostname, 0, len(records))
	for _, r := range records {
		hostname := r.InstallID
		if h, ok := hostnames[r.InstallID]; ok && h != "" {
			hostname = h
		}
		out = append(out, ScanRootWithHostname{
			InstallID:    r.InstallID,
			Hostname:     hostname,
			ScanRootPath: r.ScanRootPath,
			Name:         r.Name,
			SourceType:   r.SourceType,
		})
	}
	return out, nil
}

// MachineSummaries backs GET /api/bff/machines (story-3/ticket-1,
// contract's API surface): every machines row's lifetime summary, sorted
// last_seen_at descending. A thin pass-through over Repo.
// ListMachineSummaries — unlike Service.ScanRoots' own hostname-joining
// pass above, there is no Go-side work left for this layer to do: the
// query already aggregates and orders everything the response needs
// (db/queries/machines.sql's own doc comment), so this method exists
// mainly to keep the transport handler (internal/transport/bff/
// usage_handler.go) talking to Service, never Repo, directly — the same
// seam every other read path on this surface goes through.
func (s *Service) MachineSummaries(ctx context.Context) ([]MachineSummary, error) {
	return s.Repo.ListMachineSummaries(ctx)
}

// WindowTotals is one row of GET /api/bff/usage/windows' fixed table —
// Window names which of the fixed ranges this row covers, Totals is
// that range's turns/tokens/cost (no group_by on this endpoint at all,
// contract's API surface section).
type WindowTotals struct {
	Window Window
	Totals Totals
}

// Windows backs GET /api/bff/usage/windows: one row per entry in the
// package-level Windows slice (summary.go — story-1/ticket-20: now seven
// entries, 5h/24h/today/week/month/year/lifetime), in that fixed order,
// each computed the same way Summary computes a single window's totals —
// this just loops over all of them instead of taking one from the caller.
//
// filter (story-2/ticket-10) is applied to every one of the seven
// per-window queries identically, mirroring Summary's own filter
// handling — the contract's own requirement that an active console
// filter scope "every row of the fixed table," not just one.
func (s *Service) Windows(ctx context.Context, filter Filter, now time.Time) ([]WindowTotals, error) {
	rows := make([]WindowTotals, 0, len(Windows))
	for _, w := range Windows {
		start, end, err := WindowBounds(w, now)
		if err != nil {
			return nil, err
		}
		events, err := s.Repo.ListEventsInWindow(ctx, start, end, filter)
		if err != nil {
			return nil, err
		}
		// GroupBy is irrelevant here — only .Totals is read — but
		// Aggregate needs a value to switch on; GroupByActor is as good
		// as any other since Breakdown is discarded.
		rows = append(rows, WindowTotals{Window: w, Totals: Aggregate(events, GroupByActor).Totals})
	}
	return rows, nil
}
