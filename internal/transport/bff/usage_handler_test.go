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

// TestGetUsageSummary_GroupsAndTotalsMatchSeededRows is this endpoint's
// own core acceptance test: three events across two actors, all inside
// the requested window, group_by=actor.
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
	assert.Equal(t, "freya", got.Breakdown[0].Key, "freya has the higher cost (3.0), sorted first")
	assert.InDelta(t, 3.0, got.Breakdown[0].Cost, 1e-9)
	assert.Equal(t, int64(300), got.Breakdown[0].Tokens)
	assert.Equal(t, int64(2), got.Breakdown[0].Turns)
	assert.Equal(t, "nicole", got.Breakdown[1].Key)
	assert.InDelta(t, 0.5, got.Breakdown[1].Cost, 1e-9)
}

// TestGetUsageSummary_GroupByPath proves group_by actually switches the
// breakdown dimension, not just the query string.
func TestGetUsageSummary_GroupByPath(t *testing.T) {
	router, session, conn := newBFFRouterForUsage(t)
	now := time.Now().UTC()

	seedUsageEvent(t, conn, "e1", "freya", "/gits/my-task", "install-a", 100, 1.0, now.Add(-1*time.Hour))
	seedUsageEvent(t, conn, "e2", "nicole", "/gits/my-task", "install-a", 100, 1.0, now.Add(-1*time.Hour))

	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/summary?window=24h&group_by=path", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageSummary(t, rec)
	require.Len(t, got.Breakdown, 1, "both events share the same path — group_by=path collapses them into one row")
	assert.Equal(t, "/gits/my-task", got.Breakdown[0].Key)
	assert.Equal(t, int64(2), got.Breakdown[0].Turns)
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
	require.Len(t, got.Windows, 5)

	byWindow := map[string]bffapi.UsageWindowRow{}
	for _, w := range got.Windows {
		byWindow[string(w.Window)] = w
	}

	assert.Equal(t, int64(1), byWindow["5h"].Turns, "only the 3-hours-ago event falls inside 5h")
	assert.Equal(t, int64(100), byWindow["5h"].Tokens)
	assert.Equal(t, int64(2), byWindow["24h"].Turns, "both events fall inside a rolling 24h window")
	assert.Equal(t, int64(300), byWindow["24h"].Tokens)
}

func TestGetUsageWindows_MissingSession_Unauthorized(t *testing.T) {
	router, _, _ := newBFFRouterForUsage(t)
	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/windows", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetUsageWindows_NoEvents_AllZero(t *testing.T) {
	router, session, _ := newBFFRouterForUsage(t)
	rec := doBFFJSONRequest(t, router, http.MethodGet, "/api/bff/usage/windows", session, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := decodeUsageWindows(t, rec)
	require.Len(t, got.Windows, 5)
	for _, w := range got.Windows {
		assert.Equal(t, int64(0), w.Turns, w.Window)
		assert.Equal(t, int64(0), w.Tokens, w.Window)
	}
}
