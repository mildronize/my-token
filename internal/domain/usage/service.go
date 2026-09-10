package usage

import (
	"context"
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
	ID                       string
	SessionID                string
	Actor                    string
	Path                     string
	Machine                  string
	Model                    string
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

// Service implements the usage domain contract (story-1/ticket-11) on
// top of a Repository. This package never resolves an actor itself
// (mirrors internal/domain/todo's own I4 note) — the transport layer
// authenticates the caller (the collector's own API key) before this is
// ever reached; the "actor" field on each IngestEvent below is an
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
func (s *Service) Summary(ctx context.Context, window Window, groupBy GroupBy, now time.Time) (SummaryResult, error) {
	start, end, err := WindowBounds(window, now)
	if err != nil {
		return SummaryResult{}, err
	}

	events, err := s.Repo.ListEventsInWindow(ctx, start, end)
	if err != nil {
		return SummaryResult{}, err
	}

	return Aggregate(events, groupBy), nil
}

// WindowTotals is one row of GET /api/bff/usage/windows' fixed table —
// Window names which of the five fixed ranges this row covers, Totals is
// that range's turns/tokens/cost (no group_by on this endpoint at all,
// contract's API surface section).
type WindowTotals struct {
	Window Window
	Totals Totals
}

// Windows backs GET /api/bff/usage/windows: one row per entry in the
// package-level Windows slice (summary.go), in that fixed order, each
// computed the same way Summary computes a single window's totals — this
// just loops over all five instead of taking one from the caller.
func (s *Service) Windows(ctx context.Context, now time.Time) ([]WindowTotals, error) {
	rows := make([]WindowTotals, 0, len(Windows))
	for _, w := range Windows {
		start, end, err := WindowBounds(w, now)
		if err != nil {
			return nil, err
		}
		events, err := s.Repo.ListEventsInWindow(ctx, start, end)
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
