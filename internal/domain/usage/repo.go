package usage

import (
	"context"
	"database/sql"
	"strings"
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

// MachineSummary is one row of ListMachineSummaries (db/queries/
// machines.sql) — story-3/ticket-1: a machine's lifetime summary, already
// fully aggregated and hostname-paired in SQL. Unlike ScanRootRecord/
// ScanRootWithHostname's split (service.go), this query needs no further
// Go-side join or transform, so one type carries it through both Repo and
// Service — Service.MachineSummaries (service.go) is a thin pass-through
// over this exact shape, not a second, identically-shaped type, since
// there is nothing for that layer to add.
type MachineSummary struct {
	InstallID       string
	Hostname        string
	LastSeenAt      time.Time
	LifetimeCost    float64
	LifetimeTokens  int64
	CollectedPaths  int64
	CollectedActors int64
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
	//
	// story-2/ticket-10 adds filter, AND-composed with the window bound
	// itself: filter.Machine (raw install_id) and filter.ScanRoot (the
	// wire's own <install_id>:<scan_root_path> composite, contract's API
	// surface section) each narrow the result further when set, and
	// AND-compose with each other when both are set. An empty Filter{}
	// (both fields "") applies no additional constraint beyond the
	// window — every existing caller before this ticket behaves exactly
	// as before. Repo.ListEventsInWindow (repo.go) is the only place that
	// decomposes filter.ScanRoot's composite string into the two
	// underlying SQL arguments the query needs (splitScanRootFilter,
	// below) — this interface still only ever hands callers the single
	// wire-shaped Filter, matching the contract's own "shared Filter{...}
	// struct" description of Service.Summary/Windows' own signature.
	ListEventsInWindow(ctx context.Context, start, end time.Time, filter Filter) ([]Event, error)

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

	// ListMachineSummaries returns every machines row's lifetime summary,
	// sorted last_seen_at descending — story-3/ticket-1: Service.
	// MachineSummaries' own read path backing GET /api/bff/machines
	// (contract's API surface). Aggregation (SUM/COUNT DISTINCT) and the
	// last_seen_at DESC ordering both happen in SQL (db/queries/
	// machines.sql's own doc comment explains why, unlike MachineHostnames/
	// ListScanRoots above), so this method needs no Go-side join or sort —
	// a machines row with no matching usage_events rows still comes back
	// as one row with every numeric field zeroed, never omitted or errored
	// (the query's own LEFT JOIN/COALESCE).
	ListMachineSummaries(ctx context.Context) ([]MachineSummary, error)
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

// splitScanRootFilter decomposes the wire's own
// <install_id>:<scan_root_path> composite (contract's API surface
// section, same convention ticket 7 established for group_by=path) into
// the two halves ListEventsInWindow's own SQL query filters on
// separately (db/queries/usage_events.sql's scan_root_install_id/
// scan_root_path narg pair). Pure, no database — this is this ticket's
// own isolatable "repo-query-building logic" seam (contract's Verifiable
// section: "no database needed for the pure half").
//
// ok is false when scanRoot has no ":" at all — a malformed shape.
// ListEventsInWindow's own caller (below) treats that the same as a
// well-formed-but-nonexistent pair: an empty/zero result, never an error
// (contract's own tolerance) — but deliberately short-circuits before
// ever reaching SQL, rather than composing a filter from the unparsed
// value, so a malformed value can never accidentally match real data by
// coincidence (e.g. a malformed scan_root string that happens to equal a
// real install_id, with no scan_root-path constraint left to narrow it).
func splitScanRootFilter(scanRoot string) (installID, scanRootPath string, ok bool) {
	installID, scanRootPath, ok = strings.Cut(scanRoot, ":")
	return installID, scanRootPath, ok
}

// nullableFilterArg converts an empty string ("no filter on this axis")
// into a real SQL NULL for the generated narg parameter, and a non-empty
// string into itself — sqlc's sqlite narg support types these params as
// `interface{}` (internal/db/usage_events.sql.go), so a plain untyped nil
// is exactly what the driver needs to see to bind SQL NULL, matching the
// query's own "sqlc.narg(x) IS NULL OR column = sqlc.narg(x)" idiom.
func nullableFilterArg(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

// buildListEventsInWindowParams is ListEventsInWindow's own
// repo-query-building step, split out as a pure function so this
// ticket's AND-composition ("both filters set together (AND, not OR)")
// has a seam that's unit-testable without a database (contract's
// Verifiable section: "table-driven, constructed Event slices, no
// database needed for the pure half" — the AND-composition itself lives
// in this params-building step, not in a query executed against a real
// connection, so this is where that pure test targets). ok is false
// only when filter.ScanRoot has a malformed shape (splitScanRootFilter's
// own doc comment) — the caller (ListEventsInWindow, below) treats that
// as "build no query at all, return empty," never an error.
//
// Every field that applies is set independently of every other — there
// is no branch here that lets one filter suppress or override another,
// which is what actually makes the composition AND rather than OR.
func buildListEventsInWindowParams(start, end time.Time, filter Filter) (params db.ListUsageEventsInWindowParams, ok bool) {
	params = db.ListUsageEventsInWindowParams{
		RangeStart: start,
		RangeEnd:   end,
		Machine:    nullableFilterArg(filter.Machine),
		// ScanRootInstallID/ScanRootPath default to their zero value
		// (nil, for an interface{} field) — SQL NULL, meaning "no
		// scan_root filter" — unless filter.ScanRoot names one, below.
	}

	if filter.ScanRoot == "" {
		return params, true
	}

	installID, path, splitOK := splitScanRootFilter(filter.ScanRoot)
	if !splitOK {
		// Malformed shape (no ":") — contract's own tolerance: the
		// caller returns an empty result, not an error, without ever
		// composing a partial/coincidental SQL match.
		return db.ListUsageEventsInWindowParams{}, false
	}
	// Bind installID/path as literal SQL values even when a half is ""
	// (e.g. a leading/trailing ":") rather than routing them back
	// through nullableFilterArg — usage_events.machine/scan_root are
	// both NOT NULL for every real row (contract's Data model), so an
	// empty literal here matches nothing real, which is exactly this
	// contract's own "well-formed but non-existent pair" tolerance.
	// filter.Machine's own top-level "" is different: that "" always
	// means "no filter at all" (nullableFilterArg, above), never a
	// literal match target — there is no wire way to ask for
	// `machine=""` the way an oddly-shaped scan_root can still name an
	// empty half.
	params.ScanRootInstallID = installID
	params.ScanRootPath = path
	return params, true
}

// ListEventsInWindow implements Repository.ListEventsInWindow — a single
// read query, no aggregation at this layer (summary.go's Aggregate does
// that in Go, over whatever this returns). story-2/ticket-10: filter is
// translated into the query's optional SQL arguments by
// buildListEventsInWindowParams (above, the pure half) before this ever
// touches the real database connection.
func (r *Repo) ListEventsInWindow(ctx context.Context, start, end time.Time, filter Filter) ([]Event, error) {
	params, ok := buildListEventsInWindowParams(start, end, filter)
	if !ok {
		return []Event{}, nil
	}

	rows, err := r.q.ListUsageEventsInWindow(ctx, params)
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

// ListMachineSummariesRow's two COALESCE(SUM(...), 0) columns
// (LifetimeCost, LifetimeTokens) come back from sqlc typed as
// `interface{}` (internal/db/machines.sql.go), not a fixed numeric type —
// SQLite is dynamically typed and this expression's runtime value
// genuinely varies: a machine with matching usage_events rows gets back
// whatever numeric type the driver chose for the real SUM (float64 for
// lifetime_cost's REAL column, int64 for lifetime_tokens' INTEGER-typed
// sum), while a machine with none gets back the query's own literal `0`
// (int64). float64/int64 are the only two shapes SQLite's own type
// affinity rules can ever produce here; a driver-level nil (no COALESCE
// fallback at all) can never happen given the query always coalesces.
//
// numericToFloat64/numericToInt64 are two separate, explicitly-named
// conversions rather than one shared helper — lifetime_cost (a monetary
// amount) and lifetime_tokens (a count) are different units, and keeping
// them textually distinct at each call site says so; it also means
// LifetimeTokens converts straight to int64 without a float64 round trip
// (a real int64 SUM staying a real int64, not passing through a type
// that only exactly represents integers up to 2^53). Both fall back to
// the zero value on an unexpected shape rather than panicking, matching
// every other defensive fallback in this package.
func numericToFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	default:
		return 0
	}
}

func numericToInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	default:
		return 0
	}
}

// ListMachineSummaries implements Repository.ListMachineSummaries over the
// sqlc-generated ListMachineSummaries query (db/queries/machines.sql) —
// story-3/ticket-1. No further aggregation, join, or re-sort here: the
// query itself already does all of it (that file's own doc comment
// explains why), so this method's only job is converting the
// sqlc-generated row's two dynamically-typed numeric columns
// (numericToFloat64/numericToInt64, above) into MachineSummary's own
// fixed-type fields.
func (r *Repo) ListMachineSummaries(ctx context.Context) ([]MachineSummary, error) {
	rows, err := r.q.ListMachineSummaries(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]MachineSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, MachineSummary{
			InstallID:       row.InstallID,
			Hostname:        row.Hostname,
			LastSeenAt:      row.LastSeenAt,
			LifetimeCost:    numericToFloat64(row.LifetimeCost),
			LifetimeTokens:  numericToInt64(row.LifetimeTokens),
			CollectedPaths:  row.CollectedPaths,
			CollectedActors: row.CollectedActors,
		})
	}
	return out, nil
}
