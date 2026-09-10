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

	got, err := repo.ListEventsInWindow(ctx, windowStart, windowEnd)
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

	got, err := repo.ListEventsInWindow(ctx, want.CreatedAt, want.CreatedAt.Add(time.Second))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, want, got[0])
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
