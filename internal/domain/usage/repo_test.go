package usage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mildronize/my-template/internal/dbquery"
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

// TestI3_UsageEventsScopingDoesNotApplyToThisDomain — same shape as
// internal/domain/todo's own TestI3_GetByIDReadsAnyCreator_
// ScopingRetiredForThisDomain: usage_events carries no owner/creator
// reference at all (it's a machine-reported fact about a session, not a
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
// only ever reference the usage_events table. Mirrors
// internal/domain/todo/repo_test.go's own TestI4_TodoRepoOnlyQueriesTodosTable
// exactly; internal/invariants_test.go's TestDoneWhen12 requires this
// test to exist inside every domain module's own package
// (perDomainModuleScopePackages, updated for this module in
// internal/invariants_test.go).
func TestI4_UsageRepoOnlyQueriesUsageEventsTable(t *testing.T) {
	root := repoRootForTests(t)
	queriesDir := filepath.Join(root, "db", "queries")

	dbquery.AssertQueryFileReferencesOnlyOwnTable(t, queriesDir, "usage_events.sql", "usage_events")
}
