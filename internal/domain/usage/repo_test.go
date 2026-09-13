package usage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mildronize/my-token/internal/dbquery"
)

func sampleEvent(id string) Event {
	return Event{
		ID:                       id,
		SessionID:                "session-1",
		Actor:                    "freya",
		Path:                     "my-token",
		Machine:                  "install-1",
		Model:                    "claude-sonnet-4-5-20250929",
		ScanRoot:                 "/home/thw-home/.claude",
		InputTokens:              100,
		OutputTokens:             50,
		CacheReadInputTokens:     10,
		CacheCreationInputTokens: 5,
		Cost:                     CostForUsage("claude-sonnet-4-5-20250929", 100, 50, 10, 5),
		Source:                   Source,
		CreatedAt:                time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC),
	}
}

func TestRepo_InsertBatch_NewEventsAllLand(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	inserted, err := repo.InsertBatch(ctx, []Event{sampleEvent("msg-1"), sampleEvent("msg-2")})
	require.NoError(t, err)
	assert.Equal(t, int64(2), inserted)
	assert.Equal(t, 2, countRows(t, conn, "usage_events"))
}

// TestRepo_InsertBatch_DuplicateIDWithinBatch_IdempotentInsertOrIgnore is
// this ticket's own core demoable criterion, exercised directly at the
// repo layer: a batch that lists the same id twice (a deliberately
// repeated id, matching the ticket's own acceptance test) lands exactly
// once, not twice — INSERT OR IGNORE on the primary key, not a batch-level
// dedup step this repo would otherwise have to do itself.
func TestRepo_InsertBatch_DuplicateIDWithinBatch_IdempotentInsertOrIgnore(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	inserted, err := repo.InsertBatch(ctx, []Event{
		sampleEvent("msg-dup"),
		sampleEvent("msg-other"),
		sampleEvent("msg-dup"), // repeated id, same batch
	})
	require.NoError(t, err)
	// Only the two distinct ids were newly inserted — the third attempt
	// (a repeat of msg-dup) affected zero rows.
	assert.Equal(t, int64(2), inserted)
	assert.Equal(t, 2, countRows(t, conn, "usage_events"))
}

// TestRepo_InsertBatch_ResendAcrossCalls_ChangesNothing proves the
// idempotency also holds across separate calls (a resent batch, ticket
// 11's own "a resend of an already-ingested batch changes nothing"), not
// just within one call.
func TestRepo_InsertBatch_ResendAcrossCalls_ChangesNothing(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	inserted, err := repo.InsertBatch(ctx, []Event{sampleEvent("msg-1")})
	require.NoError(t, err)
	assert.Equal(t, int64(1), inserted)

	// Resend the exact same batch.
	inserted, err = repo.InsertBatch(ctx, []Event{sampleEvent("msg-1")})
	require.NoError(t, err)
	assert.Equal(t, int64(0), inserted, "a resent id must insert nothing new")
	assert.Equal(t, 1, countRows(t, conn, "usage_events"))
}

// TestI3_UsageEventsScopingDoesNotApplyToThisDomain: usage_events
// carries no owner/creator reference at all (it's a machine-reported
// fact about a session, not a
// resource an authenticated user owns), so there is no "wrong owner"
// lookup for I3 to ever have applied to in the first place. Named with
// the TestI3_ prefix per internal/invariants_test.go's TestDoneWhen12,
// which requires a dedicated TestI3_ test inside every domain module's
// own package (per-domain-module scope), regardless of whether the
// invariant's reach actually includes that module.
func TestI3_UsageEventsScopingDoesNotApplyToThisDomain(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	// InsertBatch takes no caller/actor argument at all — there is no
	// "whose" for a lookup to be scoped by. This is a direct, structural
	// negative-control proof: two events attributed to two different
	// domain-level `actor` values both land, with nothing about the
	// insert path treating either as "the caller's own" versus "someone
	// else's".
	e1 := sampleEvent("msg-a")
	e1.Actor = "freya"
	e2 := sampleEvent("msg-b")
	e2.Actor = "nicole"

	inserted, err := repo.InsertBatch(ctx, []Event{e1, e2})
	require.NoError(t, err)
	assert.Equal(t, int64(2), inserted)
	assert.Equal(t, 2, countRows(t, conn, "usage_events"))
}

// TestI4_UsageRepoOnlyQueriesUsageEventsTable — I4 ("one seam reads
// identity"; "one repo, one table") — db/queries/usage_events.sql must
// only ever reference the usage_events table.
// internal/invariants_test.go's TestDoneWhen12 requires this
// test to exist inside every domain module's own package
// (perDomainModuleScopePackages, updated for this module in
// internal/invariants_test.go).
// TestRepo_ListEventsInWindow_HalfOpenRange proves the repo layer's own
// SQL filter is genuinely half-open ([start, end), start inclusive, end
// exclusive) against a real database — summary_test.go/service_test.go
// already cover the window-boundary math and the aggregation logic as
// pure functions with a fake repo; this is the one place the real SQL
// comparison operators (`>=`/`<`) are exercised.
func TestRepo_ListEventsInWindow_HalfOpenRange(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	windowStart := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

	before := sampleEvent("before")
	before.CreatedAt = windowStart.Add(-time.Second)
	atStart := sampleEvent("at-start")
	atStart.CreatedAt = windowStart
	inside := sampleEvent("inside")
	inside.CreatedAt = windowStart.Add(12 * time.Hour)
	atEnd := sampleEvent("at-end")
	atEnd.CreatedAt = windowEnd
	after := sampleEvent("after")
	after.CreatedAt = windowEnd.Add(time.Second)

	_, err := repo.InsertBatch(ctx, []Event{before, atStart, inside, atEnd, after})
	require.NoError(t, err)

	got, err := repo.ListEventsInWindow(ctx, windowStart, windowEnd, Filter{})
	require.NoError(t, err)

	gotIDs := make([]string, 0, len(got))
	for _, e := range got {
		gotIDs = append(gotIDs, e.ID)
	}
	assert.ElementsMatch(t, []string{"at-start", "inside"}, gotIDs,
		"start is inclusive, end is exclusive")
}

// TestRepo_ListEventsInWindow_ReturnsEveryColumnUnchanged proves the
// round-trip through the sqlc-generated row type back into this
// package's own Event preserves every field InsertBatch wrote — not just
// the ones the window filter itself depends on (CreatedAt).
func TestRepo_ListEventsInWindow_ReturnsEveryColumnUnchanged(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	want := sampleEvent("msg-full")
	_, err := repo.InsertBatch(ctx, []Event{want})
	require.NoError(t, err)

	got, err := repo.ListEventsInWindow(ctx, want.CreatedAt, want.CreatedAt.Add(time.Second), Filter{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, want, got[0])
}

// --- splitScanRootFilter: pure repo-query-building logic (story-2/
// ticket-10, contract's Verifiable section: "no database needed for the
// pure half") -------------------------------------------------------

func TestSplitScanRootFilter_WellFormed_SplitsOnFirstColon(t *testing.T) {
	installID, path, ok := splitScanRootFilter("install-a:/home/thw-home/.claude")
	assert.True(t, ok)
	assert.Equal(t, "install-a", installID)
	assert.Equal(t, "/home/thw-home/.claude", path)
}

func TestSplitScanRootFilter_Malformed_NoColon_ReturnsNotOK(t *testing.T) {
	_, _, ok := splitScanRootFilter("no-colon-here")
	assert.False(t, ok, "a composite with no ':' at all is a malformed shape")
}

func TestSplitScanRootFilter_Empty_ReturnsNotOK(t *testing.T) {
	_, _, ok := splitScanRootFilter("")
	assert.False(t, ok)
}

// TestBuildListEventsInWindowParams_FilterComposition is this ticket's
// own explicitly-named pure unit test (contract's Verifiable section:
// "Unit test: ... repo-query-building logic applies both filters
// correctly when both are set together (AND, not OR) — table-driven,
// constructed Event slices, no database needed for the pure half").
// buildListEventsInWindowParams (repo.go) is the actual production step
// that turns a Filter into the SQL query's own optional arguments —
// ListEventsInWindow itself is a thin wrapper around this plus the real
// db.Queries call. No *sql.DB, no newTestDB, anywhere in this test — the
// DB-backed equivalent that proves the SQL text itself behaves the same
// way is TestRepo_ListEventsInWindow_BothFiltersSet_ANDComposeToIntersection,
// below.
func TestBuildListEventsInWindowParams_FilterComposition(t *testing.T) {
	start := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name                  string
		filter                Filter
		wantOK                bool
		wantMachine           interface{}
		wantScanRootInstallID interface{}
		wantScanRootPath      interface{}
	}{
		{
			name:   "no filter at all: every optional arg stays SQL NULL",
			filter: Filter{},
			wantOK: true,
		},
		{
			name:        "machine alone",
			filter:      Filter{Machine: "install-a"},
			wantOK:      true,
			wantMachine: "install-a",
		},
		{
			name:                  "scan_root alone",
			filter:                Filter{ScanRoot: "install-a:/home/thw-home/.claude"},
			wantOK:                true,
			wantScanRootInstallID: "install-a",
			wantScanRootPath:      "/home/thw-home/.claude",
		},
		{
			// This is the ticket's own core case: both filters land in
			// the params TOGETHER, neither one dropping/overriding the
			// other -- proving AND, not OR (an OR-shaped bug would only
			// ever populate one of Machine/ScanRootInstallID+Path, never
			// both at once).
			name:                  "both set together: AND-composes, neither field suppresses the other",
			filter:                Filter{Machine: "install-a", ScanRoot: "install-a:/home/thw-home/.claude"},
			wantOK:                true,
			wantMachine:           "install-a",
			wantScanRootInstallID: "install-a",
			wantScanRootPath:      "/home/thw-home/.claude",
		},
		{
			name:   "malformed scan_root shape (no colon): not ok, no params built at all",
			filter: Filter{ScanRoot: "no-colon-here"},
			wantOK: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params, ok := buildListEventsInWindowParams(start, end, tc.filter)
			require.Equal(t, tc.wantOK, ok)
			if !ok {
				return
			}
			assert.Equal(t, start, params.RangeStart)
			assert.Equal(t, end, params.RangeEnd)
			assert.Equal(t, tc.wantMachine, params.Machine)
			assert.Equal(t, tc.wantScanRootInstallID, params.ScanRootInstallID)
			assert.Equal(t, tc.wantScanRootPath, params.ScanRootPath)
		})
	}
}

// --- ListEventsInWindow: machine/scan_root filters (story-2/ticket-10)
// -------------------------------------------------------------------
// Contract's Verifiable section: "repo-query-building logic applies both
// filters correctly when both are set together (AND, not OR)" and the
// two HTTP-integration-test bullets, exercised here at the repo/SQL
// layer directly (bff/usage_handler_test.go covers the same scenarios
// end to end through the real HTTP route).

func TestRepo_ListEventsInWindow_MachineFilter_NarrowsToThatMachineOnly(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)
	windowStart := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

	a := sampleEvent("event-a")
	a.Machine = "install-a"
	a.CreatedAt = windowStart.Add(time.Hour)
	b := sampleEvent("event-b")
	b.Machine = "install-b"
	b.CreatedAt = windowStart.Add(time.Hour)
	_, err := repo.InsertBatch(ctx, []Event{a, b})
	require.NoError(t, err)

	got, err := repo.ListEventsInWindow(ctx, windowStart, windowEnd, Filter{Machine: "install-a"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "event-a", got[0].ID)
}

func TestRepo_ListEventsInWindow_ScanRootFilter_NarrowsToThatMachineAndPathOnly(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)
	windowStart := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

	// Same literal scan_root path as `matching`, but on a different
	// machine — must not match (the composite is machine-prefixed for
	// exactly this ambiguity, contract's API surface section).
	matching := sampleEvent("matching")
	matching.Machine = "install-a"
	matching.ScanRoot = "/home/thw-home/.claude"
	matching.CreatedAt = windowStart.Add(time.Hour)

	sameMachineDifferentRoot := sampleEvent("same-machine-different-root")
	sameMachineDifferentRoot.Machine = "install-a"
	sameMachineDifferentRoot.ScanRoot = "/home/thw-home/.claude-local"
	sameMachineDifferentRoot.CreatedAt = windowStart.Add(time.Hour)

	sameRootDifferentMachine := sampleEvent("same-root-different-machine")
	sameRootDifferentMachine.Machine = "install-b"
	sameRootDifferentMachine.ScanRoot = "/home/thw-home/.claude"
	sameRootDifferentMachine.CreatedAt = windowStart.Add(time.Hour)

	_, err := repo.InsertBatch(ctx, []Event{matching, sameMachineDifferentRoot, sameRootDifferentMachine})
	require.NoError(t, err)

	got, err := repo.ListEventsInWindow(ctx, windowStart, windowEnd, Filter{ScanRoot: "install-a:/home/thw-home/.claude"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "matching", got[0].ID)
}

// TestRepo_ListEventsInWindow_BothFiltersSet_ANDComposeToIntersection is
// this ticket's own core acceptance test: machine and scan_root set
// together narrow to their intersection, not their union (contract's
// Verifiable section: "AND, not OR").
func TestRepo_ListEventsInWindow_BothFiltersSet_ANDComposeToIntersection(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)
	windowStart := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

	bothMatch := sampleEvent("both-match")
	bothMatch.Machine = "install-a"
	bothMatch.ScanRoot = "/home/thw-home/.claude"
	bothMatch.CreatedAt = windowStart.Add(time.Hour)

	// Matches the machine filter alone, but its scan_root doesn't match
	// the scan_root filter — must be excluded once both are set (an OR
	// implementation would wrongly include this).
	machineOnlyMatch := sampleEvent("machine-only-match")
	machineOnlyMatch.Machine = "install-a"
	machineOnlyMatch.ScanRoot = "/home/thw-home/.claude-local"
	machineOnlyMatch.CreatedAt = windowStart.Add(time.Hour)

	// Matches the scan_root filter's own path, but on a different
	// machine — the scan_root filter itself already excludes this
	// (machine-prefixed composite), and the machine filter excludes it a
	// second, independent way.
	scanRootPathOnlyMatch := sampleEvent("scan-root-path-only-match")
	scanRootPathOnlyMatch.Machine = "install-b"
	scanRootPathOnlyMatch.ScanRoot = "/home/thw-home/.claude"
	scanRootPathOnlyMatch.CreatedAt = windowStart.Add(time.Hour)

	_, err := repo.InsertBatch(ctx, []Event{bothMatch, machineOnlyMatch, scanRootPathOnlyMatch})
	require.NoError(t, err)

	got, err := repo.ListEventsInWindow(ctx, windowStart, windowEnd, Filter{
		Machine:  "install-a",
		ScanRoot: "install-a:/home/thw-home/.claude",
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "both-match", got[0].ID)
}

// TestRepo_ListEventsInWindow_ScanRootFilter_MalformedShape_EmptyResult
// covers the contract's own explicitly-named tolerance: a scan_root
// value with no ":" at all yields an empty result, not an error.
func TestRepo_ListEventsInWindow_ScanRootFilter_MalformedShape_EmptyResult(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)
	windowStart := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

	e := sampleEvent("e1")
	e.CreatedAt = windowStart.Add(time.Hour)
	_, err := repo.InsertBatch(ctx, []Event{e})
	require.NoError(t, err)

	got, err := repo.ListEventsInWindow(ctx, windowStart, windowEnd, Filter{ScanRoot: "no-colon-here"})
	require.NoError(t, err, "a malformed scan_root shape must not error")
	assert.Empty(t, got)
}

// TestRepo_ListEventsInWindow_ScanRootFilter_WellFormedButNonexistentPair_EmptyResult
// covers the contract's other named tolerance: a well-formed
// install_id:scan_root_path pair that simply doesn't exist in the table
// yields an empty result, same as the malformed-shape case above.
func TestRepo_ListEventsInWindow_ScanRootFilter_WellFormedButNonexistentPair_EmptyResult(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)
	windowStart := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

	e := sampleEvent("e1")
	e.Machine = "install-a"
	e.ScanRoot = "/home/thw-home/.claude"
	e.CreatedAt = windowStart.Add(time.Hour)
	_, err := repo.InsertBatch(ctx, []Event{e})
	require.NoError(t, err)

	got, err := repo.ListEventsInWindow(ctx, windowStart, windowEnd, Filter{ScanRoot: "install-nonexistent:/nowhere"})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestI4_UsageRepoOnlyQueriesUsageEventsTable(t *testing.T) {
	root := repoRootForTests(t)
	queriesDir := filepath.Join(root, "db", "queries")

	dbquery.AssertQueryFileReferencesOnlyOwnTable(t, queriesDir, "usage_events.sql", "usage_events")
}

// --- machines: install_id -> hostname (story-1/ticket-18) ---------------

// TestI4_MachinesRepoOnlyQueriesMachinesTable mirrors
// TestI4_UsageRepoOnlyQueriesUsageEventsTable above, for
// db/queries/machines.sql — machines is a second table owned by this same
// "usage" domain module (dbquery.TableOwnership), not a separate one.
func TestI4_MachinesRepoOnlyQueriesMachinesTable(t *testing.T) {
	root := repoRootForTests(t)
	queriesDir := filepath.Join(root, "db", "queries")

	dbquery.AssertQueryFileReferencesOnlyOwnTable(t, queriesDir, "machines.sql", "machines")
}

// TestRepo_UpsertMachine_FirstCall_Inserts proves the plain first-seen
// case: no prior row, UpsertMachine creates one.
func TestRepo_UpsertMachine_FirstCall_Inserts(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	firstSeen := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertMachine(ctx, "install-1", "thw-home", firstSeen))

	assert.Equal(t, 1, countRows(t, conn, "machines"))
	hostnames, err := repo.MachineHostnames(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"install-1": "thw-home"}, hostnames)
}

// TestRepo_UpsertMachine_SecondCallWithChangedHostname_UpdatesStoredValue
// is this ticket's own explicitly-called-out acceptance test: "a second
// batch with a CHANGED hostname for the same install_id updates the
// stored value, proving it's a real upsert not just an insert-once."
// Also asserts the row count stays at 1 (INSERT OR REPLACE on the
// install_id primary key, not a second row) and that last_seen_at is
// refreshed to the second call's timestamp, not left at the first.
func TestRepo_UpsertMachine_SecondCallWithChangedHostname_UpdatesStoredValue(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	firstSeen := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertMachine(ctx, "install-1", "old-hostname", firstSeen))

	secondSeen := time.Date(2026, 9, 10, 9, 30, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertMachine(ctx, "install-1", "new-hostname", secondSeen))

	assert.Equal(t, 1, countRows(t, conn, "machines"), "a resend must update the one existing row, not insert a second")

	hostnames, err := repo.MachineHostnames(ctx)
	require.NoError(t, err)
	assert.Equal(t, "new-hostname", hostnames["install-1"], "the stored hostname must be the changed one, not the first-seen one")

	var gotLastSeenAt time.Time
	require.NoError(t, conn.QueryRow(`SELECT last_seen_at FROM machines WHERE install_id = ?`, "install-1").Scan(&gotLastSeenAt))
	assert.True(t, gotLastSeenAt.Equal(secondSeen), "last_seen_at must be refreshed to the second call's timestamp")
}

// TestRepo_UpsertMachine_DifferentInstallIDs_BothLand proves UpsertMachine
// only ever touches the one row named by installID — a second, distinct
// machine reporting in must not clobber the first's row.
func TestRepo_UpsertMachine_DifferentInstallIDs_BothLand(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	now := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertMachine(ctx, "install-1", "thw-home", now))
	require.NoError(t, repo.UpsertMachine(ctx, "install-2", "thw-laptop", now))

	assert.Equal(t, 2, countRows(t, conn, "machines"))
	hostnames, err := repo.MachineHostnames(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"install-1": "thw-home", "install-2": "thw-laptop"}, hostnames)
}

// TestRepo_MachineHostnames_NoRows_EmptyMap proves the zero-rows case
// returns an empty map, not an error — Service.substituteMachineHostnames
// relies on this to mean "no known hostnames," not a failure.
func TestRepo_MachineHostnames_NoRows_EmptyMap(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	hostnames, err := repo.MachineHostnames(ctx)
	require.NoError(t, err)
	assert.Empty(t, hostnames)
}

// --- scan_roots: (install_id, scan_root_path) -> name/source_type
// (story-2/ticket-8) ------------------------------------------------

// TestI4_ScanRootsRepoOnlyQueriesScanRootsTable mirrors
// TestI4_MachinesRepoOnlyQueriesMachinesTable above, for
// db/queries/scan_roots.sql — scan_roots is a third table owned by this
// same "usage" domain module (dbquery.TableOwnership), not a separate one.
func TestI4_ScanRootsRepoOnlyQueriesScanRootsTable(t *testing.T) {
	root := repoRootForTests(t)
	queriesDir := filepath.Join(root, "db", "queries")

	dbquery.AssertQueryFileReferencesOnlyOwnTable(t, queriesDir, "scan_roots.sql", "scan_roots")
}

// TestRepo_UpsertScanRoot_FirstCall_Inserts proves the plain first-seen
// case: no prior row, UpsertScanRoot creates one.
func TestRepo_UpsertScanRoot_FirstCall_Inserts(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	firstSeen := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertScanRoot(ctx, "install-1", "/home/thw-home/.claude", "main", "claude_code", firstSeen))

	assert.Equal(t, 1, countRows(t, conn, "scan_roots"))
	var gotName, gotSourceType string
	require.NoError(t, conn.QueryRow(
		`SELECT name, source_type FROM scan_roots WHERE install_id = ? AND scan_root_path = ?`,
		"install-1", "/home/thw-home/.claude",
	).Scan(&gotName, &gotSourceType))
	assert.Equal(t, "main", gotName)
	assert.Equal(t, "claude_code", gotSourceType)
}

// TestRepo_UpsertScanRoot_SecondCallWithRenamedScanRoot_UpdatesStoredValue
// is this ticket's own explicitly-called-out acceptance test (mirroring
// TestRepo_UpsertMachine_SecondCallWithChangedHostname_UpdatesStoredValue):
// "a second batch with a renamed scan root updates the stored name, not
// just the first-seen value." Also asserts the row count stays at 1
// (INSERT OR REPLACE on the composite primary key, not a second row) and
// that last_seen_at is refreshed to the second call's timestamp.
func TestRepo_UpsertScanRoot_SecondCallWithRenamedScanRoot_UpdatesStoredValue(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	firstSeen := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertScanRoot(ctx, "install-1", "/home/thw-home/.claude", "old-name", "claude_code", firstSeen))

	secondSeen := time.Date(2026, 9, 10, 9, 30, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertScanRoot(ctx, "install-1", "/home/thw-home/.claude", "new-name", "claude_code", secondSeen))

	assert.Equal(t, 1, countRows(t, conn, "scan_roots"), "a resend must update the one existing row, not insert a second")

	var gotName string
	var gotLastSeenAt time.Time
	require.NoError(t, conn.QueryRow(
		`SELECT name, last_seen_at FROM scan_roots WHERE install_id = ? AND scan_root_path = ?`,
		"install-1", "/home/thw-home/.claude",
	).Scan(&gotName, &gotLastSeenAt))
	assert.Equal(t, "new-name", gotName, "the stored name must be the changed one, not the first-seen one")
	assert.True(t, gotLastSeenAt.Equal(secondSeen), "last_seen_at must be refreshed to the second call's timestamp")
}

// TestRepo_UpsertScanRoot_DifferentScanRootPaths_BothLand proves
// UpsertScanRoot only ever touches the one row named by (installID,
// scanRootPath) — a second, distinct scan root on the same install must
// not clobber the first's row.
func TestRepo_UpsertScanRoot_DifferentScanRootPaths_BothLand(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	now := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertScanRoot(ctx, "install-1", "/home/thw-home/.claude", "main", "claude_code", now))
	require.NoError(t, repo.UpsertScanRoot(ctx, "install-1", "/home/thw-home/.claude-local", "backup-install", "claude_code", now))

	assert.Equal(t, 2, countRows(t, conn, "scan_roots"))
}

// TestRepo_UpsertScanRoot_SameScanRootPathDifferentInstalls_BothLand
// proves the primary key is genuinely composite — two different installs
// registering a scan root under the *same* path (or even the same name,
// goal.md's own "no shared/global naming registry across machines" rule)
// don't collide.
func TestRepo_UpsertScanRoot_SameScanRootPathDifferentInstalls_BothLand(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	now := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertScanRoot(ctx, "install-1", "/home/thw-home/.claude", "main", "claude_code", now))
	require.NoError(t, repo.UpsertScanRoot(ctx, "install-2", "/home/thw-home/.claude", "main", "claude_code", now))

	assert.Equal(t, 2, countRows(t, conn, "scan_roots"))
}

// TestRepo_ListScanRoots_NoRows_EmptySlice mirrors
// TestRepo_MachineHostnames_NoRows_EmptyMap: the zero-rows case returns an
// empty (nil-or-empty) slice, not an error — story-2/ticket-9's
// Service.ScanRoots relies on this to mean "nothing registered yet," not a
// failure.
func TestRepo_ListScanRoots_NoRows_EmptySlice(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	rows, err := repo.ListScanRoots(ctx)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// TestRepo_ListScanRoots_ReturnsEveryRegisteredRoot is this ticket's own
// core acceptance test at the repo layer: every scan_roots row upserted so
// far comes back, across more than one install_id.
func TestRepo_ListScanRoots_ReturnsEveryRegisteredRoot(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	now := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertScanRoot(ctx, "install-1", "/home/thw-home/.claude", "main", "claude_code", now))
	require.NoError(t, repo.UpsertScanRoot(ctx, "install-1", "/home/thw-home/.claude-local", "backup-install", "claude_code", now))
	require.NoError(t, repo.UpsertScanRoot(ctx, "install-2", "/home/thw-home/.claude", "main", "claude_code", now))

	got, err := repo.ListScanRoots(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []ScanRootRecord{
		{InstallID: "install-1", ScanRootPath: "/home/thw-home/.claude", Name: "main"},
		{InstallID: "install-1", ScanRootPath: "/home/thw-home/.claude-local", Name: "backup-install"},
		{InstallID: "install-2", ScanRootPath: "/home/thw-home/.claude", Name: "main"},
	}, got)
}

// TestRepo_ListScanRoots_ReflectsRenames proves ListScanRoots reads back
// whatever UpsertScanRoot most recently wrote — a renamed scan root's new
// name, not the first-seen one (mirrors
// TestRepo_UpsertScanRoot_SecondCallWithRenamedScanRoot_UpdatesStoredValue's
// own direct-SQL assertion, this time through the repo's own read method).
func TestRepo_ListScanRoots_ReflectsRenames(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	firstSeen := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertScanRoot(ctx, "install-1", "/home/thw-home/.claude", "old-name", "claude_code", firstSeen))

	secondSeen := time.Date(2026, 9, 10, 9, 30, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertScanRoot(ctx, "install-1", "/home/thw-home/.claude", "new-name", "claude_code", secondSeen))

	got, err := repo.ListScanRoots(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "new-name", got[0].Name)
}

// --- ListMachineSummaries: GET /api/bff/machines (story-3/ticket-1)
// -------------------------------------------------------------------
// Contract's Verifiable section: seed usage_events across >=2 machines
// with distinct paths/actors/costs, assert correct per-machine
// SUM/COUNT(DISTINCT ...) values and last_seen_at DESC ordering, plus the
// zero-events edge case (a machines row with no matching usage_events
// rows returns zeroed numeric fields, not a null/error).

// machineSummaryEvent builds one usage_events row with every field this
// query's own SUM/COUNT(DISTINCT ...) columns read, Cost set directly
// (not via CostForUsage/sampleEvent) so each test's expected totals are
// exact, arbitrary numbers rather than tied to the pricing table.
func machineSummaryEvent(id, machine, path, actor string, cost float64, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens int64, createdAt time.Time) Event {
	return Event{
		ID:                       id,
		SessionID:                "session-" + id,
		Actor:                    actor,
		Path:                     path,
		Machine:                  machine,
		Model:                    "claude-sonnet-4-5-20250929",
		ScanRoot:                 "/home/thw-home/.claude",
		InputTokens:              inputTokens,
		OutputTokens:             outputTokens,
		CacheReadInputTokens:     cacheReadTokens,
		CacheCreationInputTokens: cacheCreationTokens,
		Cost:                     cost,
		Source:                   Source,
		CreatedAt:                createdAt,
	}
}

// TestRepo_ListMachineSummaries_AggregatesPerMachine_SortedByLastSeenDesc
// is this ticket's own core acceptance test: two machines, each with
// events across distinct paths/actors/costs, come back with correct
// per-machine SUM(cost)/SUM(tokens)/COUNT(DISTINCT path)/COUNT(DISTINCT
// actor), sorted last_seen_at descending (the more recently reported
// machine first, regardless of insertion order).
func TestRepo_ListMachineSummaries_AggregatesPerMachine_SortedByLastSeenDesc(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	earlierSeen := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	laterSeen := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

	// install-1 last reported most recently but is upserted first here --
	// proves ordering comes from last_seen_at, not insertion/table order.
	require.NoError(t, repo.UpsertMachine(ctx, "install-1", "thw-home", laterSeen))
	require.NoError(t, repo.UpsertMachine(ctx, "install-2", "thw-laptop", earlierSeen))

	_, err := repo.InsertBatch(ctx, []Event{
		// install-1: two events, two distinct paths, two distinct actors.
		machineSummaryEvent("install-1-a", "install-1", "repo-a", "freya", 1.0, 100, 50, 10, 5, laterSeen.Add(-time.Hour)),
		machineSummaryEvent("install-1-b", "install-1", "repo-b", "nicole", 2.5, 200, 100, 20, 10, laterSeen.Add(-30*time.Minute)),
		// install-2: one event, one path, one actor.
		machineSummaryEvent("install-2-a", "install-2", "repo-c", "freya", 0.5, 50, 25, 5, 0, earlierSeen.Add(-time.Hour)),
	})
	require.NoError(t, err)

	got, err := repo.ListMachineSummaries(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// last_seen_at DESC: install-1 (later) must come first.
	assert.Equal(t, "install-1", got[0].InstallID)
	assert.Equal(t, "thw-home", got[0].Hostname)
	assert.True(t, got[0].LastSeenAt.Equal(laterSeen))
	assert.InDelta(t, 3.5, got[0].LifetimeCost, 0.0001, "SUM(cost) across both install-1 events")
	assert.Equal(t, int64(495), got[0].LifetimeTokens, "SUM of all four token columns across both events: (100+50+10+5)+(200+100+20+10)")
	assert.Equal(t, int64(2), got[0].CollectedPaths, "repo-a and repo-b are distinct")
	assert.Equal(t, int64(2), got[0].CollectedActors, "freya and nicole are distinct")

	assert.Equal(t, "install-2", got[1].InstallID)
	assert.Equal(t, "thw-laptop", got[1].Hostname)
	assert.True(t, got[1].LastSeenAt.Equal(earlierSeen))
	assert.InDelta(t, 0.5, got[1].LifetimeCost, 0.0001)
	assert.Equal(t, int64(80), got[1].LifetimeTokens, "50+25+5+0")
	assert.Equal(t, int64(1), got[1].CollectedPaths)
	assert.Equal(t, int64(1), got[1].CollectedActors)
}

// TestRepo_ListMachineSummaries_NoMatchingUsageEvents_ZeroedNotNull covers
// the contract's own explicitly-named defensive edge case: a machines row
// with no matching usage_events rows (not currently reachable via the
// collector, per that file's own doc comment) still comes back as one
// row with every numeric field zeroed -- the LEFT JOIN/COALESCE must
// hold, not silently omit the machine or error.
func TestRepo_ListMachineSummaries_NoMatchingUsageEvents_ZeroedNotNull(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	seenAt := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertMachine(ctx, "install-orphan", "thw-orphan", seenAt))
	// Deliberately no usage_events rows for install-orphan at all.

	got, err := repo.ListMachineSummaries(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "install-orphan", got[0].InstallID)
	assert.Equal(t, "thw-orphan", got[0].Hostname)
	assert.Equal(t, float64(0), got[0].LifetimeCost)
	assert.Equal(t, int64(0), got[0].LifetimeTokens)
	assert.Equal(t, int64(0), got[0].CollectedPaths)
	assert.Equal(t, int64(0), got[0].CollectedActors)
}

// TestRepo_ListMachineSummaries_NoMachines_EmptySlice mirrors
// TestRepo_ListScanRoots_NoRows_EmptySlice: the zero-machines case
// returns an empty slice, not an error.
func TestRepo_ListMachineSummaries_NoMachines_EmptySlice(t *testing.T) {
	ctx := context.Background()
	conn := newTestDB(t)
	repo := NewRepo(conn)

	got, err := repo.ListMachineSummaries(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
}
