// story-1/ticket-14: HTTP integration tests for GET /api/bff/usage/summary
// and GET /api/bff/usage/windows against seeded usage_events rows —
// summary_test.go/service_test.go (internal/domain/usage) already cover
// the window-boundary math, group_by aggregation, and reporting_installs
// counting as pure/fake-repo tests; these exist to prove the real HTTP
// route (real middleware chain, real SQLite-backed repo) wires the same
// logic together correctly end to end.
package bff

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mildronize/my-token/internal/bffapi"
	"github.com/mildronize/my-token/internal/domain/usage"
	"github.com/mildronize/my-token/internal/identity"
)

// seedUsageEvent inserts one usage_events row directly (bypassing
// usage.Repo/usage.Service's own write path) — mirrors seedUser's own
// reasoning (bff_testutil_test.go): these tests shouldn't need to trust a
// second package's ingestion path just to set up a fixture. id defaults
// to a fresh UUID-shaped string when empty.
func seedUsageEvent(t *testing.T, conn *sql.DB, id, actor, path, machine string, tokens int64, cost float64, createdAt time.Time) {
	t.Helper()
	if id == "" {
		id = "msg-" + t.Name() + "-" + createdAt.Format(time.RFC3339Nano)
	}
	_, err := conn.Exec(
		`INSERT INTO usage_events (id, session_id, actor, path, machine, model, input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens, cost, source, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, 0, ?, 'claude_code', ?)`,
		id, "session-"+id, actor, path, machine, "claude-sonnet-4-5-20250929", tokens, cost, createdAt,
	)
	require.NoError(t, err)
}

// seedUsageEventWithScanRoot mirrors seedUsageEvent above, plus a
// scan_root column value — story-2/ticket-10's own filter tests need
// distinct scan_root values per row, which seedUsageEvent's own fixed
// insert (scan_root always defaulted) can't express. Kept as a separate
// helper rather than widening seedUsageEvent's own signature, so every
// existing call site of that helper stays unchanged.
func seedUsageEventWithScanRoot(t *testing.T, conn *sql.DB, id, actor, path, machine, scanRoot string, tokens int64, cost float64, createdAt time.Time) {
	t.Helper()
	if id == "" {
		id = "msg-" + t.Name() + "-" + createdAt.Format(time.RFC3339Nano)
	}
	_, err := conn.Exec(
		`INSERT INTO usage_events (id, session_id, actor, path, machine, model, scan_root, input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens, cost, source, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 0, ?, 'claude_code', ?)`,
		id, "session-"+id, actor, path, machine, "claude-sonnet-4-5-20250929", scanRoot, tokens, cost, createdAt,
	)
	require.NoError(t, err)
}

// newBFFRouterForUsage builds one /api/bff router with a real
// usage.Service/usage.Repo on top of a fresh test DB, plus a signed
// session cookie for a freshly seeded owner — the usage-domain analogue
// of bff_testutil_test.go's own newBFFRouterForOwner. conn is returned so
// tests can seed usage_events rows directly (seedUsageEvent, above)
// before issuing a request.
func newBFFRouterForUsage(t *testing.T) (router *gin.Engine, sessionValue string, conn *sql.DB) {
	t.Helper()
	conn = newTestDB(t)
	repo := identity.NewRepo(conn)
	identitySvc := identity.NewService(repo, repo, nil, nil)
	usageSvc := usage.NewService(usage.NewRepo(conn))

	owner := seedUser(t, conn, "owner", "owner-sub-"+t.Name(), true)

	idp := newFakeIDP(t, "test-client")
	cfg := idp.testConfig()
	signer := NewSigner([]byte(cfg.SessionSecret))
	router = newTestRouter(cfg, signer, newIDVerifier(t, idp), repo, identitySvc, usageSvc)

	var err error
	sessionValue, err = signer.NewSessionCookie(owner.ID)
	require.NoError(t, err)

	return router, sessionValue, conn
}

// seedMachine inserts one machines row directly (bypassing
// usage.Repo/usage.Service's own UpsertMachine path) — mirrors
// seedUsageEvent's own reasoning (above): these tests shouldn't need to
// trust a second package's write path just to set up a fixture.
func seedMachine(t *testing.T, conn *sql.DB, installID, hostname string, lastSeenAt time.Time) {
	t.Helper()
	_, err := conn.Exec(
		`INSERT INTO machines (install_id, hostname, last_seen_at) VALUES (?, ?, ?)`,
		installID, hostname, lastSeenAt,
	)
	require.NoError(t, err)
}

func decodeUsageSummary(t *testing.T, rec *httptest.ResponseRecorder) bffapi.UsageSummary {
	t.Helper()
	var got bffapi.UsageSummary
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	return got
}

func decodeUsageWindows(t *testing.T, rec *httptest.ResponseRecorder) bffapi.UsageWindows {
	t.Helper()
	var got bffapi.UsageWindows
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	return got
}

func decodeUsageScanRootList(t *testing.T, rec *httptest.ResponseRecorder) bffapi.UsageScanRootList {
	t.Helper()
	var got bffapi.UsageScanRootList
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	return got
}

// seedScanRoot inserts one scan_roots row directly (bypassing
// usage.Repo/usage.Service's own UpsertScanRoot path) — mirrors
// seedMachine's own reasoning (above): these tests shouldn't need to
// trust a second package's write path just to set up a fixture.
func seedScanRoot(t *testing.T, conn *sql.DB, installID, scanRootPath, name string, lastSeenAt time.Time) {
	t.Helper()
	_, err := conn.Exec(
		`INSERT INTO scan_roots (install_id, scan_root_path, name, source_type, last_seen_at) VALUES (?, ?, ?, 'claude_code', ?)`,
		installID, scanRootPath, name, lastSeenAt,
	)
	require.NoError(t, err)
}

// TestGetUsageSummary_GroupsAndTotalsMatchSeededRows is this endpoint's
// own core acceptance test: three events across two actors, all inside
// the requested window, group_by=actor. story-2/ticket-16 makes
// group_by=actor's key machine-prefixed too (`<hostname>:<actor>` once
// substituted, `<install_id>:<actor>` raw) — no `machines` row is seeded
// here, so both rows keep their raw install_id prefix.
func TestGetUsageSummary_GroupsAndTotalsMatchSeededRows(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedUsageEvent(t, conn, "e1", "freya", "/gits/my-task", "install-a", 100, 1.0, now.Add(-1*time.Hour))
	seedUsageEvent(t, conn, "e2", "freya", "/gits/my-token", "install-a", 200, 2.0, now.Add(-2*time.Hour))
	seedUsageEvent(t, conn, "e3", "nicole", "/gits/my-task", "install-b", 50, 0.5, now.Add(-3*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=actor", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageSummary(t, rec)
	assert.Equal(t, int64(350), got.Totals.Tokens)
	assert.InDelta(t, 3.5, got.Totals.Cost, 1e-9)
	assert.Equal(t, int64(3), got.Totals.Turns)
	assert.Equal(t, int64(2), got.ReportingInstalls, "two distinct machines reported in this window")

	require.Len(t, got.Breakdown, 2)
	assert.Equal(t, "install-a:freya", got.Breakdown[0].Key, "freya has the higher cost (3.0), sorted first")
	assert.InDelta(t, 3.0, got.Breakdown[0].Cost, 1e-9)
	assert.Equal(t, int64(300), got.Breakdown[0].Tokens)
	assert.Equal(t, int64(2), got.Breakdown[0].Turns)
	assert.Equal(t, "install-b:nicole", got.Breakdown[1].Key)
	assert.InDelta(t, 0.5, got.Breakdown[1].Cost, 1e-9)
}

// TestGetUsageSummary_GroupByPath proves group_by actually switches the
// breakdown dimension, not just the query string. story-2/ticket-7 makes
// group_by=path's key machine-prefixed (`<hostname>:<path>` once
// substituted, `<install_id>:<path>` raw) — both events here share the
// same machine, so they still collapse into one row, just with the new
// key shape.
func TestGetUsageSummary_GroupByPath(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedMachine(t, conn, "install-a", "thw-home", now)
	seedUsageEvent(t, conn, "e1", "freya", "/gits/my-task", "install-a", 100, 1.0, now.Add(-1*time.Hour))
	seedUsageEvent(t, conn, "e2", "nicole", "/gits/my-task", "install-a", 100, 1.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=path", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageSummary(t, rec)
	require.Len(t, got.Breakdown, 1, "both events share the same path AND the same machine — group_by=path collapses them into one row")
	assert.Equal(t, "thw-home:/gits/my-task", got.Breakdown[0].Key, "key's install_id prefix substituted for the hostname")
	require.NotNil(t, got.Breakdown[0].RawKey)
	assert.Equal(t, "install-a:/gits/my-task", *got.Breakdown[0].RawKey)
	assert.Equal(t, int64(2), got.Breakdown[0].Turns)
}

// TestGetUsageSummary_GroupByPath_CrossMachineIdenticalPath_DoesNotMerge
// is story-2/ticket-7's own HTTP-level regression test for the false-merge
// bug story-1 documented and deferred (contract's "Cross-machine path
// identity" section): the exact same literal path reported by two
// different machines must produce two distinct group_by=path rows, not
// one silently merged row.
func TestGetUsageSummary_GroupByPath_CrossMachineIdenticalPath_DoesNotMerge(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedMachine(t, conn, "install-a", "thw-home", now)
	seedMachine(t, conn, "install-b", "thw-laptop", now)
	seedUsageEvent(t, conn, "e1", "freya", "/gits/my-task", "install-a", 100, 1.0, now.Add(-1*time.Hour))
	seedUsageEvent(t, conn, "e2", "nicole", "/gits/my-task", "install-b", 100, 3.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=path", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageSummary(t, rec)
	require.Len(t, got.Breakdown, 2, "the same literal path from two different machines must not merge into one row")
	assert.Equal(t, "thw-laptop:/gits/my-task", got.Breakdown[0].Key, "higher cost first")
	assert.Equal(t, "thw-home:/gits/my-task", got.Breakdown[1].Key)
}

// TestGetUsageSummary_GroupByActor_CrossMachineIdenticalActor_DoesNotMerge
// is story-2/ticket-16's own HTTP-level regression test, mirroring
// TestGetUsageSummary_GroupByPath_CrossMachineIdenticalPath_DoesNotMerge
// exactly: the exact same literal actor path reported by two different
// machines must produce two distinct group_by=actor rows, not one
// silently merged row (มายด์'s own real-world finding: two collector
// installs sharing one filesystem can genuinely report an identical raw
// actor string).
func TestGetUsageSummary_GroupByActor_CrossMachineIdenticalActor_DoesNotMerge(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedMachine(t, conn, "install-a", "thw-home", now)
	seedMachine(t, conn, "install-b", "thw-home-openrouter", now)
	seedUsageEvent(t, conn, "e1", "/home/thw-home/.typ-crews/naomi", "/gits/p1", "install-a", 100, 1.0, now.Add(-1*time.Hour))
	seedUsageEvent(t, conn, "e2", "/home/thw-home/.typ-crews/naomi", "/gits/p2", "install-b", 100, 3.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=actor", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageSummary(t, rec)
	require.Len(t, got.Breakdown, 2, "the same literal actor path from two different machines must not merge into one row")
	assert.Equal(t, "thw-home-openrouter:/home/thw-home/.typ-crews/naomi", got.Breakdown[0].Key, "higher cost first")
	assert.Equal(t, "thw-home:/home/thw-home/.typ-crews/naomi", got.Breakdown[1].Key)
}

// TestGetUsageSummary_GroupByMachine_ReturnsHostnamesNotUUIDs is this
// ticket's own core acceptance test for the read side: seeded
// usage_events + machines rows, group_by=machine's breakdown key comes
// back as the seeded hostname, not the raw install_id — and RawKey
// carries that raw install_id so the console can still show it via
// tooltip (contract's "machine label" rule).
func TestGetUsageSummary_GroupByMachine_ReturnsHostnamesNotUUIDs(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedMachine(t, conn, "install-a-uuid", "thw-home", now)
	seedMachine(t, conn, "install-b-uuid", "thw-laptop", now)

	seedUsageEvent(t, conn, "e1", "freya", "/gits/my-task", "install-a-uuid", 100, 2.0, now.Add(-1*time.Hour))
	seedUsageEvent(t, conn, "e2", "freya", "/gits/my-task", "install-b-uuid", 100, 1.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=machine", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageSummary(t, rec)
	require.Len(t, got.Breakdown, 2)

	// install-a-uuid has the higher cost, sorted first (Aggregate's own
	// cost-descending order — unaffected by the hostname substitution
	// happening after aggregation).
	assert.Equal(t, "thw-home", got.Breakdown[0].Key, "key must be the hostname, not the raw install_id")
	require.NotNil(t, got.Breakdown[0].RawKey)
	assert.Equal(t, "install-a-uuid", *got.Breakdown[0].RawKey, "the raw install_id must still be reachable")

	assert.Equal(t, "thw-laptop", got.Breakdown[1].Key)
	require.NotNil(t, got.Breakdown[1].RawKey)
	assert.Equal(t, "install-b-uuid", *got.Breakdown[1].RawKey)

	// Neither raw install_id string ever appears as a `key` value —
	// exactly what this ticket's "not UUIDs" wording is asserting.
	for _, row := range got.Breakdown {
		assert.NotEqual(t, "install-a-uuid", row.Key)
		assert.NotEqual(t, "install-b-uuid", row.Key)
	}
}

// TestGetUsageSummary_GroupByMachine_NoMachinesRow_FallsBackToInstallID
// covers the contract's own explicitly-named edge case end to end: a
// machine with usage_events rows but no corresponding machines row
// (shouldn't happen given upsert-on-every-batch, but must never show a
// blank/null key).
func TestGetUsageSummary_GroupByMachine_NoMachinesRow_FallsBackToInstallID(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	// Deliberately no seedMachine call for "install-orphan".
	seedUsageEvent(t, conn, "e1", "freya", "/gits/my-task", "install-orphan", 100, 1.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=machine", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageSummary(t, rec)
	require.Len(t, got.Breakdown, 1)
	assert.Equal(t, "install-orphan", got.Breakdown[0].Key, "must fall back to the raw install_id, never a blank key")
	assert.Nil(t, got.Breakdown[0].RawKey, "no substitution happened, so there is no separate raw_key on the wire")
}

// TestGetUsageSummary_GroupByActor_RowsCarryNoRawKey proves raw_key is
// scoped to group_by=machine's own substitution — every other group_by's
// rows must carry no raw_key at all on the wire (bff-openapi.yaml's own
// doc comment: "absent for every other group_by").
func TestGetUsageSummary_GroupByActor_RowsCarryNoRawKey(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedUsageEvent(t, conn, "e1", "freya", "/gits/my-task", "install-a", 100, 1.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=actor", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageSummary(t, rec)
	require.Len(t, got.Breakdown, 1)
	assert.Nil(t, got.Breakdown[0].RawKey)
}

// TestGetUsageSummary_WindowExcludesEventsOutsideRange proves the window
// parameter is actually applied against seeded created_at values, not
// just accepted and ignored.
func TestGetUsageSummary_WindowExcludesEventsOutsideRange(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedUsageEvent(t, conn, "recent", "freya", "p", "m", 100, 1.0, now.Add(-1*time.Hour))
	seedUsageEvent(t, conn, "old", "freya", "p", "m", 999, 99.0, now.Add(-48*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=5h&group_by=actor", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageSummary(t, rec)
	assert.Equal(t, int64(1), got.Totals.Turns, "the 48h-old event must not be inside a 5h window")
	assert.Equal(t, int64(100), got.Totals.Tokens)
}

// TestGetUsageSummary_ReportingInstalls_WindowScopedNotLifetime is the
// contract's own explicitly-flagged decision (ticket 14's own report):
// a machine that only reported outside the selected window must not
// count towards reporting_installs, even though it has reported at some
// point in this service's lifetime.
func TestGetUsageSummary_ReportingInstalls_WindowScopedNotLifetime(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedUsageEvent(t, conn, "in-window", "freya", "p", "install-recent", 100, 1.0, now.Add(-1*time.Hour))
	seedUsageEvent(t, conn, "out-of-window", "freya", "p", "install-old", 100, 1.0, now.Add(-48*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=actor", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageSummary(t, rec)
	assert.Equal(t, int64(1), got.ReportingInstalls, "install-old reported outside this window and must not be counted")
}

func TestGetUsageSummary_MissingSession_Unauthorized(t *testing.T) {
	router, _, _ := newBFFRouterForUsage(t)
	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=actor", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetUsageSummary_InvalidWindow_Rejected(t *testing.T) {
	router, session, _ := newBFFRouterForUsage(t)
	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=fortnight&group_by=actor", session, nil)
	// bff-openapi.yaml's request validator rejects an out-of-enum value
	// before the handler is ever reached.
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetUsageSummary_InvalidGroupBy_Rejected(t *testing.T) {
	router, session, _ := newBFFRouterForUsage(t)
	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=session", session, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetUsageSummary_MissingRequiredParams_Rejected(t *testing.T) {
	router, session, _ := newBFFRouterForUsage(t)
	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary", session, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- GET /api/bff/usage/summary & /usage/windows: machine/scan_root
// filters (story-2/ticket-10, contract's API surface section) ----------
// These seed usage_events across (at least) two machines and two
// scan-roots and exercise both endpoints end to end through the real
// HTTP route/middleware chain/SQLite-backed repo — the pure filter/AND
// composition logic itself is already covered at the repo layer
// (internal/domain/usage/repo_test.go's TestRepo_ListEventsInWindow_*
// tests) and the service-wiring layer (service_test.go's
// TestService_Summary_PassesFilterToRepoUnmodified and friends); these
// prove the whole stack (query-param parsing through to the JSON
// response) wires the same behavior together correctly.

// TestGetUsageSummary_MachineFilter_NarrowsWholeResult is this ticket's
// own core acceptance test: machine=<install_id> alone scopes
// totals/breakdown/reporting_installs to just that machine's events, not
// just one panel's own dimension (contract's Console filters section:
// global, not per-panel).
func TestGetUsageSummary_MachineFilter_NarrowsWholeResult(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedUsageEvent(t, conn, "e-a", "freya", "/gits/my-task", "install-a", 100, 1.0, now.Add(-1*time.Hour))
	seedUsageEvent(t, conn, "e-b", "nicole", "/gits/my-task", "install-b", 200, 5.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=actor&machine=install-a", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageSummary(t, rec)
	assert.Equal(t, int64(1), got.Totals.Turns, "totals must be scoped to the filtered machine")
	assert.Equal(t, int64(100), got.Totals.Tokens)
	require.Len(t, got.Breakdown, 1)
	assert.Equal(t, "install-a:freya", got.Breakdown[0].Key)
	assert.Equal(t, int64(1), got.ReportingInstalls, "reporting_installs must also be scoped, not lifetime/unfiltered")
}

// TestGetUsageSummary_ScanRootFilter_NarrowsWholeResult proves
// scan_root=<install_id>:<scan_root_path> alone narrows correctly — same
// literal scan-root path on a different machine must not match (the
// machine-prefixed composite, same convention as group_by=path's own
// cross-machine fix).
func TestGetUsageSummary_ScanRootFilter_NarrowsWholeResult(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedUsageEventWithScanRoot(t, conn, "e-a", "freya", "/gits/my-task", "install-a", "/home/thw-home/.claude", 100, 1.0, now.Add(-1*time.Hour))
	seedUsageEventWithScanRoot(t, conn, "e-a2", "freya", "/gits/other", "install-a", "/home/thw-home/.claude-local", 300, 9.0, now.Add(-1*time.Hour))
	seedUsageEventWithScanRoot(t, conn, "e-b", "nicole", "/gits/my-task", "install-b", "/home/thw-home/.claude", 200, 5.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=actor&scan_root=install-a:/home/thw-home/.claude", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageSummary(t, rec)
	assert.Equal(t, int64(1), got.Totals.Turns)
	assert.Equal(t, int64(100), got.Totals.Tokens)
	require.Len(t, got.Breakdown, 1)
	assert.Equal(t, "install-a:freya", got.Breakdown[0].Key)
}

// TestGetUsageSummary_BothFiltersSet_ANDComposeToIntersection is the
// contract's own explicitly-named requirement: machine and scan_root set
// together narrow to their intersection (AND), not their union (OR).
func TestGetUsageSummary_BothFiltersSet_ANDComposeToIntersection(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	// Matches both filters.
	seedUsageEventWithScanRoot(t, conn, "both-match", "freya", "/gits/my-task", "install-a", "/home/thw-home/.claude", 100, 1.0, now.Add(-1*time.Hour))
	// Matches machine alone (different scan_root) -- an OR implementation
	// would wrongly include this.
	seedUsageEventWithScanRoot(t, conn, "machine-only", "freya", "/gits/other", "install-a", "/home/thw-home/.claude-local", 300, 9.0, now.Add(-1*time.Hour))
	// Matches scan_root's own path alone (different machine).
	seedUsageEventWithScanRoot(t, conn, "path-only", "nicole", "/gits/my-task", "install-b", "/home/thw-home/.claude", 200, 5.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet,
		"/api/bff/usage/summary?window=24h&group_by=actor&machine=install-a&scan_root=install-a:/home/thw-home/.claude", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageSummary(t, rec)
	assert.Equal(t, int64(1), got.Totals.Turns, "only the row matching BOTH filters should count")
	assert.Equal(t, int64(100), got.Totals.Tokens)
	require.Len(t, got.Breakdown, 1)
	assert.Equal(t, "install-a:freya", got.Breakdown[0].Key)
}

// TestGetUsageSummary_InvalidScanRoot_MalformedShape_EmptyResultNotError
// covers the contract's own explicitly-named tolerance ("mirrors this
// API's existing tolerance for 'no matching data' versus 'malformed
// request'"): a scan_root value with no ":" at all yields 200 with an
// empty/zero result, not a 400.
func TestGetUsageSummary_InvalidScanRoot_MalformedShape_EmptyResultNotError(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()
	seedUsageEvent(t, conn, "e1", "freya", "/gits/my-task", "install-a", 100, 1.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=actor&scan_root=no-colon-here", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String(), "a malformed scan_root shape must not 400")

	got := decodeUsageSummary(t, rec)
	assert.Equal(t, int64(0), got.Totals.Turns)
	assert.Empty(t, got.Breakdown)
	assert.Equal(t, int64(0), got.ReportingInstalls)
}

// TestGetUsageSummary_ScanRootFilter_WellFormedButNonexistentPair_EmptyResult
// covers the contract's other named tolerance: a well-formed
// install_id:scan_root_path pair that simply doesn't exist yields the
// same empty/zero result, not an error.
func TestGetUsageSummary_ScanRootFilter_WellFormedButNonexistentPair_EmptyResult(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()
	seedUsageEventWithScanRoot(t, conn, "e1", "freya", "/gits/my-task", "install-a", "/home/thw-home/.claude", 100, 1.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=actor&scan_root=install-nonexistent:/nowhere", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageSummary(t, rec)
	assert.Equal(t, int64(0), got.Totals.Turns)
	assert.Empty(t, got.Breakdown)
}

// --- GET /api/bff/usage/windows: same two filters (story-2/ticket-10) --

// TestGetUsageWindows_MachineFilter_NarrowsEveryRow proves the fixed
// table also respects an active machine filter, on every one of its
// rows, not just one window (contract's API surface section: "story-1
// shipped this endpoint with zero params").
func TestGetUsageWindows_MachineFilter_NarrowsEveryRow(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedUsageEvent(t, conn, "e-a", "freya", "/gits/my-task", "install-a", 100, 1.0, now.Add(-1*time.Hour))
	seedUsageEvent(t, conn, "e-b", "nicole", "/gits/my-task", "install-b", 200, 5.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/windows?machine=install-a", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageWindows(t, rec)
	require.Len(t, got.Windows, 7)
	// Both seeded events are 1 hour ago, so every one of the 7 fixed
	// windows would otherwise show 2 turns -- the filter must bring every
	// single row down to install-a's own 1, proving it scopes the whole
	// table, not just one row.
	for _, w := range got.Windows {
		assert.Equal(t, int64(1), w.Turns, "window %s must only count install-a's own event", w.Window)
		assert.Equal(t, int64(100), w.Tokens, "window %s", w.Window)
	}
}

// TestGetUsageWindows_ScanRootFilter_NarrowsEveryRow mirrors
// TestGetUsageSummary_ScanRootFilter_NarrowsWholeResult for the
// fixed-table endpoint: scan_root=<install_id>:<scan_root_path> alone
// must narrow every one of the seven rows, not just one, and the same
// literal scan-root path on a different machine must not match.
func TestGetUsageWindows_ScanRootFilter_NarrowsEveryRow(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedUsageEventWithScanRoot(t, conn, "e-a", "freya", "/gits/my-task", "install-a", "/home/thw-home/.claude", 100, 1.0, now.Add(-1*time.Hour))
	seedUsageEventWithScanRoot(t, conn, "e-a2", "freya", "/gits/other", "install-a", "/home/thw-home/.claude-local", 300, 9.0, now.Add(-1*time.Hour))
	seedUsageEventWithScanRoot(t, conn, "e-b", "nicole", "/gits/my-task", "install-b", "/home/thw-home/.claude", 200, 5.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/windows?scan_root=install-a:/home/thw-home/.claude", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageWindows(t, rec)
	require.Len(t, got.Windows, 7)
	for _, w := range got.Windows {
		assert.Equal(t, int64(1), w.Turns, "window %s must only count the matching scan_root's own event", w.Window)
		assert.Equal(t, int64(100), w.Tokens, "window %s", w.Window)
	}
}

// TestGetUsageWindows_BothFiltersSet_ANDComposeToIntersection mirrors
// TestGetUsageSummary_BothFiltersSet_ANDComposeToIntersection for the
// fixed-table endpoint.
func TestGetUsageWindows_BothFiltersSet_ANDComposeToIntersection(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedUsageEventWithScanRoot(t, conn, "both-match", "freya", "/gits/my-task", "install-a", "/home/thw-home/.claude", 100, 1.0, now.Add(-1*time.Hour))
	seedUsageEventWithScanRoot(t, conn, "machine-only", "freya", "/gits/other", "install-a", "/home/thw-home/.claude-local", 300, 9.0, now.Add(-1*time.Hour))
	seedUsageEventWithScanRoot(t, conn, "path-only", "nicole", "/gits/my-task", "install-b", "/home/thw-home/.claude", 200, 5.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet,
		"/api/bff/usage/windows?machine=install-a&scan_root=install-a:/home/thw-home/.claude", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageWindows(t, rec)
	byWindow := map[string]bffapi.UsageWindowRow{}
	for _, w := range got.Windows {
		byWindow[string(w.Window)] = w
	}
	assert.Equal(t, int64(1), byWindow["24h"].Turns, "only the row matching BOTH filters should count")
	assert.Equal(t, int64(100), byWindow["24h"].Tokens)
}

// TestGetUsageWindows_InvalidScanRoot_MalformedShape_EmptyResultNotError
// mirrors TestGetUsageSummary_InvalidScanRoot_MalformedShape_EmptyResultNotError
// for the fixed-table endpoint: a scan_root value with no ":" at all
// yields 200 with every row zeroed, not a 400.
func TestGetUsageWindows_InvalidScanRoot_MalformedShape_EmptyResultNotError(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()
	seedUsageEvent(t, conn, "e1", "freya", "/gits/my-task", "install-a", 100, 1.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/windows?scan_root=no-colon-here", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String(), "a malformed scan_root shape must not 400")

	got := decodeUsageWindows(t, rec)
	require.Len(t, got.Windows, 7)
	for _, w := range got.Windows {
		assert.Equal(t, int64(0), w.Turns, "window %s must be zeroed, not errored", w.Window)
		assert.Equal(t, int64(0), w.Tokens, "window %s", w.Window)
	}
}

// TestGetUsageWindows_ScanRootFilter_WellFormedButNonexistentPair_EmptyResult
// mirrors TestGetUsageSummary_ScanRootFilter_WellFormedButNonexistentPair_EmptyResult
// for the fixed-table endpoint: a well-formed install_id:scan_root_path
// pair that simply doesn't exist yields the same empty/zero result, not
// an error.
func TestGetUsageWindows_ScanRootFilter_WellFormedButNonexistentPair_EmptyResult(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()
	seedUsageEventWithScanRoot(t, conn, "e1", "freya", "/gits/my-task", "install-a", "/home/thw-home/.claude", 100, 1.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/windows?scan_root=install-nonexistent:/nowhere", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageWindows(t, rec)
	require.Len(t, got.Windows, 7)
	for _, w := range got.Windows {
		assert.Equal(t, int64(0), w.Turns, "window %s must be zeroed, not errored", w.Window)
		assert.Equal(t, int64(0), w.Tokens, "window %s", w.Window)
	}
}

// --- GET /api/bff/usage/windows -----------------------------------------

// TestGetUsageWindows_EachRowReflectsItsOwnRange is the fixed-table
// endpoint's own core acceptance test: an event 3 hours ago falls inside
// both 5h and 24h; an event 10 hours ago falls inside 24h but is outside
// the narrower 5h window — proving each row is computed against its own
// range, not one shared filter reused for every row.
func TestGetUsageWindows_EachRowReflectsItsOwnRange(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedUsageEvent(t, conn, "recent", "freya", "p", "m1", 100, 1.0, now.Add(-3*time.Hour))
	seedUsageEvent(t, conn, "mid", "freya", "p", "m1", 200, 2.0, now.Add(-10*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/windows", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageWindows(t, rec)
	require.Len(t, got.Windows, 7)

	byWindow := map[string]bffapi.UsageWindowRow{}
	for _, w := range got.Windows {
		byWindow[string(w.Window)] = w
	}

	assert.Equal(t, int64(1), byWindow["5h"].Turns, "only the 3-hours-ago event falls inside 5h")
	assert.Equal(t, int64(100), byWindow["5h"].Tokens)
	assert.Equal(t, int64(2), byWindow["24h"].Turns, "both events fall inside a rolling 24h window")
	assert.Equal(t, int64(300), byWindow["24h"].Tokens)
}

// TestGetUsageWindows_ReturnsSevenRowsInOrder is story-1/ticket-20's own
// core acceptance test for this endpoint: the fixed table grew from five
// rows to seven (year and lifetime are now real, selectable tabs), and
// must come back in exactly the stated order — not just as a set of
// seven unordered rows.
func TestGetUsageWindows_ReturnsSevenRowsInOrder(t *testing.T) {
	router, session, _ := newBFFRouterForUsage(t)

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/windows", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageWindows(t, rec)
	require.Len(t, got.Windows, 7)

	wantOrder := []string{"5h", "24h", "today", "week", "month", "year", "lifetime"}
	gotOrder := make([]string, len(got.Windows))
	for i, w := range got.Windows {
		gotOrder[i] = string(w.Window)
	}
	assert.Equal(t, wantOrder, gotOrder)
}

func TestGetUsageWindows_MissingSession_Unauthorized(t *testing.T) {
	router, _, _ := newBFFRouterForUsage(t)
	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/windows", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- GET /api/bff/usage/scan-roots (story-2/ticket-9) -------------------

// TestGetUsageScanRoots_ReturnsEveryRootAcrossTwoInstalls is this
// ticket's own core acceptance test: machines + scan_roots rows seeded
// across two install_ids, every row comes back with correct hostname
// joins.
func TestGetUsageScanRoots_ReturnsEveryRootAcrossTwoInstalls(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedMachine(t, conn, "install-a", "thw-home", now)
	seedMachine(t, conn, "install-b", "thw-laptop", now)
	seedScanRoot(t, conn, "install-a", "/home/thw-home/.claude", "main", now)
	seedScanRoot(t, conn, "install-a", "/home/thw-home/.claude-local", "backup-install", now)
	seedScanRoot(t, conn, "install-b", "/home/laptop/.claude", "main", now)

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/scan-roots", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageScanRootList(t, rec)
	assert.ElementsMatch(t, []bffapi.UsageScanRoot{
		{InstallId: "install-a", Hostname: "thw-home", ScanRootPath: "/home/thw-home/.claude", Name: "main"},
		{InstallId: "install-a", Hostname: "thw-home", ScanRootPath: "/home/thw-home/.claude-local", Name: "backup-install"},
		{InstallId: "install-b", Hostname: "thw-laptop", ScanRootPath: "/home/laptop/.claude", Name: "main"},
	}, got.ScanRoots)
}

// TestGetUsageScanRoots_NoMachinesRow_FallsBackToInstallID covers the
// ticket's own explicitly-named edge case: a scan_roots row whose
// install_id has no machines row falls back to showing the raw
// install_id as the hostname, never a blank one.
func TestGetUsageScanRoots_NoMachinesRow_FallsBackToInstallID(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	// Deliberately no seedMachine call for "install-orphan".
	seedScanRoot(t, conn, "install-orphan", "/home/thw-home/.claude", "main", now)

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/scan-roots", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageScanRootList(t, rec)
	require.Len(t, got.ScanRoots, 1)
	assert.Equal(t, "install-orphan", got.ScanRoots[0].Hostname, "must fall back to the raw install_id, never a blank hostname")
	assert.Equal(t, "install-orphan", got.ScanRoots[0].InstallId)
}

// TestGetUsageScanRoots_NoScanRoots_EmptyArray proves the zero-registered
// case returns an empty array on the wire, not null or an error.
func TestGetUsageScanRoots_NoScanRoots_EmptyArray(t *testing.T) {
	router, session, _ := newBFFRouterForUsage(t)

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/scan-roots", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageScanRootList(t, rec)
	assert.Empty(t, got.ScanRoots)
}

func TestGetUsageScanRoots_MissingSession_Unauthorized(t *testing.T) {
	router, _, _ := newBFFRouterForUsage(t)
	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/scan-roots", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func decodeMachineList(t *testing.T, rec *httptest.ResponseRecorder) bffapi.MachineList {
	t.Helper()
	var got bffapi.MachineList
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	return got
}

// --- GET /api/bff/machines (story-3/ticket-1) ---------------------------

// TestGetMachines_ReturnsLifetimeSummaryAcrossMachines_SortedByLastReportedDesc
// is this ticket's own core acceptance test (contract's Verifiable
// section): two machines, seeded usage_events across distinct
// paths/actors/costs, come back with the correct wire shape -- per-machine
// lifetime_cost/lifetime_tokens/collected_paths/collected_actors -- sorted
// last_seen_at descending, matching last_seen_at's own "Last reported"
// semantics (not usage_events' own created_at).
func TestGetMachines_ReturnsLifetimeSummaryAcrossMachines_SortedByLastReportedDesc(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)

	earlierSeen := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	laterSeen := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

	// install-a last reported most recently but is seeded first --
	// proves ordering comes from last_seen_at, not seed/table order.
	seedMachine(t, conn, "install-a", "thw-home", laterSeen)
	seedMachine(t, conn, "install-b", "thw-laptop", earlierSeen)

	// install-a: two events, two distinct paths, two distinct actors.
	seedUsageEvent(t, conn, "event-a1", "freya", "repo-a", "install-a", 100, 1.0, laterSeen.Add(-time.Hour))
	seedUsageEvent(t, conn, "event-a2", "nicole", "repo-b", "install-a", 200, 2.5, laterSeen.Add(-30*time.Minute))
	// install-b: one event, one path, one actor.
	seedUsageEvent(t, conn, "event-b1", "freya", "repo-c", "install-b", 50, 0.5, earlierSeen.Add(-time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/machines", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeMachineList(t, rec)
	require.Len(t, got.Machines, 2)

	// last_seen_at DESC: install-a (later) must come first.
	first := got.Machines[0]
	assert.Equal(t, "install-a", first.InstallId)
	assert.Equal(t, "thw-home", first.Hostname)
	assert.True(t, first.LastSeenAt.Equal(laterSeen))
	assert.InDelta(t, 3.5, first.LifetimeCost, 0.0001)
	assert.Equal(t, int64(300), first.LifetimeTokens, "100+200 input tokens, no other token columns seeded")
	assert.Equal(t, int64(2), first.CollectedPaths)
	assert.Equal(t, int64(2), first.CollectedActors)

	second := got.Machines[1]
	assert.Equal(t, "install-b", second.InstallId)
	assert.Equal(t, "thw-laptop", second.Hostname)
	assert.True(t, second.LastSeenAt.Equal(earlierSeen))
	assert.InDelta(t, 0.5, second.LifetimeCost, 0.0001)
	assert.Equal(t, int64(50), second.LifetimeTokens)
	assert.Equal(t, int64(1), second.CollectedPaths)
	assert.Equal(t, int64(1), second.CollectedActors)
}

// TestGetMachines_MachineWithNoUsageEvents_ZeroedFieldsNotOmitted covers
// the contract's own explicitly-named defensive edge case end to end
// through the real HTTP route: a machines row with no matching
// usage_events rows still comes back as one row with every numeric field
// zeroed, not omitted from the array or erroring.
func TestGetMachines_MachineWithNoUsageEvents_ZeroedFieldsNotOmitted(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	seenAt := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	seedMachine(t, conn, "install-orphan", "thw-orphan", seenAt)

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/machines", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeMachineList(t, rec)
	require.Len(t, got.Machines, 1)
	assert.Equal(t, "install-orphan", got.Machines[0].InstallId)
	assert.Equal(t, float64(0), got.Machines[0].LifetimeCost)
	assert.Equal(t, int64(0), got.Machines[0].LifetimeTokens)
	assert.Equal(t, int64(0), got.Machines[0].CollectedPaths)
	assert.Equal(t, int64(0), got.Machines[0].CollectedActors)
}

// TestGetMachines_NoMachines_EmptyArray proves the zero-machines case
// returns an empty array on the wire, not null or an error.
func TestGetMachines_NoMachines_EmptyArray(t *testing.T) {
	router, session, _ := newBFFRouterForUsage(t)

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/machines", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeMachineList(t, rec)
	assert.Empty(t, got.Machines)
}

func TestGetMachines_MissingSession_Unauthorized(t *testing.T) {
	router, _, _ := newBFFRouterForUsage(t)
	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/machines", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetUsageWindows_NoEvents_AllZero(t *testing.T) {
	router, session, _ := newBFFRouterForUsage(t)
	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/windows", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageWindows(t, rec)
	require.Len(t, got.Windows, 7)
	for _, w := range got.Windows {
		assert.Equal(t, int64(0), w.Turns, w.Window)
		assert.Equal(t, int64(0), w.Tokens, w.Window)
	}
}
