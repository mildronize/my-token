package usage

import (
	"context"
	"database/sql"
	"time"

	"github.com/mildronize/my-token/internal/db"
)

// Event is this package's own representation of one usage_events row to
// be written — deliberately distinct from db.UsageEvent (there isn't
// one; this domain has no read path yet, only ingestion), so every other
// file in this package can talk about "a usage event" without importing
// internal/db itself (ARCHITECTURE.md rule 2: only repo.go/*_repo.go may
// import the sqlc-generated package).
//
// Cost and Source are always set by Service before this ever reaches
// Repo (this ticket's own requirement: the server computes both, never
// trusts the client) — Repo itself has no opinion on how they were
// derived, it only ever writes whatever Event it's given.
type Event struct {
	ID        string
	SessionID string
	Actor     string
	Path      string
	Machine   string
	Model     string
	// ScanRoot is the raw, resolved scan-root path the reporting
	// transcript file was found under (story-2/ticket-8, contract's Data
	// model) — distinct from Path, which is git-rooted from touched
	// files (ticket 13), not the transcript's own location.
	ScanRoot                 string
	InputTokens              int64
	OutputTokens             int64
	CacheReadInputTokens     int64
	CacheCreationInputTokens int64
	Cost                     float64
	Source                   string
	CreatedAt                time.Time
}

// ScanRootRecord is one row of the scan_roots table, as read back —
// story-2/ticket-9. Distinct from UpsertScanRootInput (service.go, the
// write-side per-entry shape) and from the wire ScanRoot type
// (internal/bffapi, generated from bff-openapi.yaml), same "distinct name
// per layer" convention Event/IngestEvent already follow, so a
// reader/grep never has to guess which package's type is meant. Carries
// no hostname of its own — Service.ScanRoots joins that in from
// MachineHostnames (below), same as the existing group_by=machine/
// group_by=path substitution pattern.
type ScanRootRecord struct {
	InstallID    string
	ScanRootPath string
	Name         string
}

// Repository is the subset of Repo's methods Service depends on —
// declared here, not in repo.go's own type, so tests can supply a fake
// without a real database.
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

	// UpsertMachine writes (installID, hostname) into machines, refreshing
	// lastSeenAt on every call regardless of whether installID already had
	// a row — story-1/ticket-18: called on every ingestion batch
	// (Service.UpsertMachine) so a renamed machine's console label catches
	// up rather than staying stuck on whatever hostname it was first seen
	// with (contract's Data model: `machines`).
	UpsertMachine(ctx context.Context, installID, hostname string, lastSeenAt time.Time) error

	// MachineHostnames returns every known machine's install_id ->
	// hostname mapping — story-1/ticket-18: Service.Summary's own
	// hostname-substitution pass over Aggregate's output, when
	// group_by=machine (contract's "machine label" display rule). A
	// install_id with no row in machines simply has no entry in the
	// returned map — the caller (Service.Summary) is the one that decides
	// what "no entry" means (fall back to the raw install_id), not this
	// method.
	MachineHostnames(ctx context.Context) (map[string]string, error)

	// UpsertScanRoot writes (installID, scanRootPath, name, sourceType)
	// into scan_roots, refreshing lastSeenAt on every call regardless of
	// whether the (installID, scanRootPath) pair already had a row —
	// story-2/ticket-8: called once per entry of every ingestion batch's
	// own scan_roots array (Service.UpsertScanRoots) so a renamed scan
	// root's console label catches up rather than staying stuck on
	// whatever name it was first seen with (contract's Data model:
	// `scan_roots`), mirroring UpsertMachine's own upsert-on-every-batch
	// design.
	UpsertScanRoot(ctx context.Context, installID, scanRootPath, name, sourceType string, lastSeenAt time.Time) error

	// ListScanRoots returns every registered scan_roots row across every
	// reporting install — story-2/ticket-9: Service.ScanRoots' own read
	// path backing GET /api/bff/usage/scan-roots (contract's API
	// surface). No window filter, no group_by, no hostname join at this
	// layer (Service.ScanRoots does that in Go, over MachineHostnames'
	// existing result, mirroring the group_by=machine/group_by=path
	// substitution pattern above). A no-rows result is an empty slice,
	// not an error — mirrors MachineHostnames' own "no known rows" case.
	ListScanRoots(ctx context.Context) ([]ScanRootRecord, error)
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
			ScanRoot:                 e.ScanRoot,
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
			ScanRoot:                 row.ScanRoot,
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

// UpsertMachine implements Repository.UpsertMachine over the sqlc-generated
// UpsertMachine query (db/queries/machines.sql) — a real upsert (INSERT OR
// REPLACE on the install_id primary key), not an insert-once: a second
// call for the same installID with a different hostname overwrites the
// stored value rather than leaving the first-seen one in place.
func (r *Repo) UpsertMachine(ctx context.Context, installID, hostname string, lastSeenAt time.Time) error {
	return r.q.UpsertMachine(ctx, db.UpsertMachineParams{
		InstallID:  installID,
		Hostname:   hostname,
		LastSeenAt: lastSeenAt,
	})
}

// MachineHostnames implements Repository.MachineHostnames over the
// sqlc-generated ListMachines query.
func (r *Repo) MachineHostnames(ctx context.Context) (map[string]string, error) {
	rows, err := r.q.ListMachines(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		out[row.InstallID] = row.Hostname
	}
	return out, nil
}

// UpsertScanRoot implements Repository.UpsertScanRoot over the
// sqlc-generated UpsertScanRoot query (db/queries/scan_roots.sql) — a
// real upsert (INSERT OR REPLACE on the (install_id, scan_root_path)
// composite primary key), not an insert-once: a second call for the
// same pair with a different name overwrites the stored value rather
// than leaving the first-seen one in place — mirrors UpsertMachine
// above.
func (r *Repo) UpsertScanRoot(ctx context.Context, installID, scanRootPath, name, sourceType string, lastSeenAt time.Time) error {
	return r.q.UpsertScanRoot(ctx, db.UpsertScanRootParams{
		InstallID:    installID,
		ScanRootPath: scanRootPath,
		Name:         name,
		SourceType:   sourceType,
		LastSeenAt:   lastSeenAt,
	})
}

// ListScanRoots implements Repository.ListScanRoots over the
// sqlc-generated ListScanRoots query (db/queries/scan_roots.sql) —
// story-2/ticket-9. No SQL JOIN against machines here (that query's own
// doc comment explains why); Service.ScanRoots is where the hostname
// join happens, in Go, over MachineHostnames' existing result.
func (r *Repo) ListScanRoots(ctx context.Context) ([]ScanRootRecord, error) {
	rows, err := r.q.ListScanRoots(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ScanRootRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, ScanRootRecord{
			InstallID:    row.InstallID,
			ScanRootPath: row.ScanRootPath,
			Name:         row.Name,
		})
	}
	return out, nil
}
