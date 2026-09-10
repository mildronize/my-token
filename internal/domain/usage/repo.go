package usage

import (
	"context"
	"database/sql"
	"time"

	"github.com/mildronize/my-token/internal/db"
)

// Event is this package's own representation of one usage_events row to
// be written — deliberately distinct from db.UsageEvent (there isn't
// one; this domain has no read path yet, only ingestion) for the same
// reason internal/domain/todo's Todo type is distinct from db.Todo: so
// every other file in this package can talk about "a usage event"
// without importing internal/db itself (ARCHITECTURE.md rule 2: only
// repo.go/*_repo.go may import the sqlc-generated package).
//
// Cost and Source are always set by Service before this ever reaches
// Repo (this ticket's own requirement: the server computes both, never
// trusts the client) — Repo itself has no opinion on how they were
// derived, it only ever writes whatever Event it's given.
type Event struct {
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
	Cost                     float64
	Source                   string
	CreatedAt                time.Time
}

// Repository is the subset of Repo's methods Service depends on —
// declared here, not in repo.go's own type, so tests can supply a fake
// without a real database (mirrors internal/domain/todo's own
// Repository/Repo split).
type Repository interface {
	// InsertBatch idempotently inserts every event in batch (INSERT OR
	// IGNORE on id — a repeat id changes nothing) and returns how many
	// were newly inserted (0 <= inserted <= len(batch)). Runs inside one
	// transaction: either every insert attempt in the batch is applied
	// (each independently idempotent on its own id) or, on a real error
	// partway through, none of them are — there is no partial-batch state
	// for a caller to have to reason about.
	InsertBatch(ctx context.Context, batch []Event) (inserted int64, err error)

	// ListEventsInWindow returns every event whose CreatedAt falls in the
	// half-open range [start, end) — story-1/ticket-14's own read path,
	// backing Service.Summary/Service.Windows. This is the only place the
	// window boundary is applied at the data layer; everything else
	// (group_by aggregation, totals, reporting_installs) is pure Go over
	// the returned slice (summary.go's Aggregate) so it's testable
	// without a database.
	ListEventsInWindow(ctx context.Context, start, end time.Time) ([]Event, error)
}

// Repo is the only type in this package that imports the sqlc-generated
// package (internal/db) — every other file reaches the database only
// through Repo's methods (ARCHITECTURE.md rule 2). This domain has no
// owner-scoping (I3 does not apply — see doc.go and repo_test.go's own
// TestI3_ test) and no update/delete path at all: usage_events is
// write-once, append-only by construction (there is no UPDATE/DELETE
// query anywhere in db/queries/usage_events.sql).
type Repo struct {
	conn *sql.DB
	q    *db.Queries
}

// NewRepo builds a Repo on top of an already-open *sql.DB (see
// platform.OpenDB).
func NewRepo(conn *sql.DB) *Repo {
	return &Repo{conn: conn, q: db.New(conn)}
}

// InsertBatch runs every event's insert-or-ignore inside a single
// transaction, summing the affected-row count sqlc's `:execrows` query
// reports per call (0 when a row already existed, 1 when it was newly
// inserted — db/queries/usage_events.sql's own comment explains why
// `:execrows`, not `:one`/RETURNING, is required for an INSERT OR IGNORE
// that can affect zero rows).
func (r *Repo) InsertBatch(ctx context.Context, batch []Event) (int64, error) {
	tx, err := r.conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	q := r.q.WithTx(tx)

	var inserted int64
	for _, e := range batch {
		rows, err := q.InsertUsageEventIgnoreDuplicate(ctx, db.InsertUsageEventIgnoreDuplicateParams{
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
			Cost:                     e.Cost,
			Source:                   e.Source,
			CreatedAt:                e.CreatedAt,
		})
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				return 0, rbErr
			}
			return 0, err
		}
		inserted += rows
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

// ListEventsInWindow implements Repository.ListEventsInWindow — a single
// read query, no aggregation at this layer (summary.go's Aggregate does
// that in Go, over whatever this returns).
func (r *Repo) ListEventsInWindow(ctx context.Context, start, end time.Time) ([]Event, error) {
	rows, err := r.q.ListUsageEventsInWindow(ctx, db.ListUsageEventsInWindowParams{
		RangeStart: start,
		RangeEnd:   end,
	})
	if err != nil {
		return nil, err
	}

	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		events = append(events, Event{
			ID:                       row.ID,
			SessionID:                row.SessionID,
			Actor:                    row.Actor,
			Path:                     row.Path,
			Machine:                  row.Machine,
			Model:                    row.Model,
			InputTokens:              row.InputTokens,
			OutputTokens:             row.OutputTokens,
			CacheReadInputTokens:     row.CacheReadInputTokens,
			CacheCreationInputTokens: row.CacheCreationInputTokens,
			Cost:                     row.Cost,
			Source:                   row.Source,
			CreatedAt:                row.CreatedAt,
		})
	}
	return events, nil
}
