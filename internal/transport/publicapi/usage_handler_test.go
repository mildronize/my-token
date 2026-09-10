package publicapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mildronize/my-template/internal/api"
)

// decodeIngestResponse decodes an api.IngestUsageEventsBatchResponse-shaped
// body — usage-specific, mirrors todo_handler_test.go's own decodeTodo.
func decodeIngestResponse(t *testing.T, rec *httptest.ResponseRecorder) api.IngestUsageEventsBatchResponse {
	t.Helper()
	var got api.IngestUsageEventsBatchResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	return got
}

// countUsageEventRows returns usage_events' current row count — used
// below the same way todo_handler_test.go's countTodoEventRows is: a
// status-code assertion alone can't tell "exactly one row landed" apart
// from "two rows landed and the response just reported one".
func countUsageEventRows(t *testing.T, conn *sql.DB) int {
	t.Helper()
	var n int
	require.NoError(t, conn.QueryRow(`SELECT COUNT(*) FROM usage_events`).Scan(&n))
	return n
}

func usageEventBody(id string) map[string]any {
	return map[string]any{
		"id":                          id,
		"session_id":                  "session-1",
		"actor":                       "freya",
		"path":                        "my-token",
		"machine":                     "install-1",
		"model":                       "claude-sonnet-4-5-20250929",
		"input_tokens":                100,
		"output_tokens":               50,
		"cache_read_input_tokens":     10,
		"cache_creation_input_tokens": 5,
		"timestamp":                   "2026-09-10T12:00:00Z",
	}
}

// TestHandler_IngestUsageEventsBatch_DuplicateIDLandsExactlyOnce is
// ticket 11's own acceptance test, verbatim: "POST a batch of synthetic
// events including a deliberately repeated `id`, confirm exactly one row
// lands per unique `id`."
func TestHandler_IngestUsageEventsBatch_DuplicateIDLandsExactlyOnce(t *testing.T) {
	router, conn := newIntegrationRouter(t)
	_, rawKey := createAgentWithKey(t, conn, "collector-1")

	rec := doJSONRequest(t, router, http.MethodPost, "/api/v1/usage-events/batch", rawKey, map[string]any{
		"install_id": "install-1",
		"hostname":   "thw-home",
		"events": []map[string]any{
			usageEventBody("msg-1"),
			usageEventBody("msg-2"),
			usageEventBody("msg-1"), // deliberately repeated id
		},
	})
	require.Equal(t, http.StatusCreated, rec.Code)

	resp := decodeIngestResponse(t, rec)
	assert.Equal(t, int64(3), resp.Received, "the batch named 3 events, duplicates included")
	assert.Equal(t, int64(2), resp.Inserted, "only the 2 distinct ids were newly inserted")
	assert.Equal(t, 2, countUsageEventRows(t, conn), "exactly one row per unique id, not one per event in the batch")
}

// TestHandler_IngestUsageEventsBatch_ResendAcrossCalls_ChangesNothing
// covers the contract's own idempotency wording directly: "a resend of
// an already-ingested batch changes nothing" — a second call with the
// exact same batch, not just a duplicate id inside one call.
func TestHandler_IngestUsageEventsBatch_ResendAcrossCalls_ChangesNothing(t *testing.T) {
	router, conn := newIntegrationRouter(t)
	_, rawKey := createAgentWithKey(t, conn, "collector-1")

	body := map[string]any{
		"install_id": "install-1",
		"hostname":   "thw-home",
		"events":     []map[string]any{usageEventBody("msg-1"), usageEventBody("msg-2")},
	}

	first := doJSONRequest(t, router, http.MethodPost, "/api/v1/usage-events/batch", rawKey, body)
	require.Equal(t, http.StatusCreated, first.Code)
	assert.Equal(t, int64(2), decodeIngestResponse(t, first).Inserted)
	require.Equal(t, 2, countUsageEventRows(t, conn))

	// Resend the exact same batch.
	second := doJSONRequest(t, router, http.MethodPost, "/api/v1/usage-events/batch", rawKey, body)
	require.Equal(t, http.StatusCreated, second.Code)
	resp := decodeIngestResponse(t, second)
	assert.Equal(t, int64(2), resp.Received)
	assert.Equal(t, int64(0), resp.Inserted, "a resent batch must insert nothing new")
	assert.Equal(t, 2, countUsageEventRows(t, conn), "row count must not grow on a resend")
}

// TestHandler_IngestUsageEventsBatch_CostAndSourceAreServerComputed is
// ticket 11's other acceptance criterion: "confirm cost and source are
// server-computed not client-supplied." UsageEventInput declares no
// cost/source properties at all and additionalProperties: false
// (openapi.yaml), so a client that sends them gets rejected by the
// openapi request validator before the handler ever runs — this test
// proves both halves: (1) a well-formed request with no cost/source at
// all still gets a real, correctly-computed cost persisted, and (2) a
// request that tries to smuggle a client-chosen cost/source in gets a
// validation_error, not a request that silently succeeds using the
// client's numbers.
func TestHandler_IngestUsageEventsBatch_CostAndSourceAreServerComputed(t *testing.T) {
	router, conn := newIntegrationRouter(t)
	_, rawKey := createAgentWithKey(t, conn, "collector-1")

	rec := doJSONRequest(t, router, http.MethodPost, "/api/v1/usage-events/batch", rawKey, map[string]any{
		"install_id": "install-1",
		"hostname":   "thw-home",
		"events":     []map[string]any{usageEventBody("msg-1")},
	})
	require.Equal(t, http.StatusCreated, rec.Code)

	var gotCost float64
	var gotSource string
	require.NoError(t, conn.QueryRow(`SELECT cost, source FROM usage_events WHERE id = ?`, "msg-1").Scan(&gotCost, &gotSource))
	assert.Equal(t, "claude_code", gotSource)
	// 100 input + 50 output + 10 cache_read + 5 cache_creation tokens at
	// sonnet's own $/1M rates (pricing_test.go's own table) — a real,
	// nonzero, model-and-token-dependent value, not a placeholder.
	assert.InDelta(t, 100*3.00/1e6+50*15.00/1e6+10*0.30/1e6+5*3.75/1e6, gotCost, 1e-9)

	// Attempting to smuggle a client-chosen cost/source is rejected
	// outright (additionalProperties: false) — not silently accepted and
	// ignored, and definitely not used.
	withClientValues := usageEventBody("msg-2")
	withClientValues["cost"] = 999999.0
	withClientValues["source"] = "totally_fake"
	badRec := doJSONRequest(t, router, http.MethodPost, "/api/v1/usage-events/batch", rawKey, map[string]any{
		"install_id": "install-1",
		"hostname":   "thw-home",
		"events":     []map[string]any{withClientValues},
	})
	assert.Equal(t, http.StatusBadRequest, badRec.Code)
	assert.Equal(t, 1, countUsageEventRows(t, conn), "the rejected request must not have written anything")
}

func TestHandler_IngestUsageEventsBatch_Unauthenticated_Returns401(t *testing.T) {
	router, _ := newIntegrationRouter(t)

	rec := doJSONRequest(t, router, http.MethodPost, "/api/v1/usage-events/batch", "", map[string]any{
		"install_id": "install-1",
		"hostname":   "thw-home",
		"events":     []map[string]any{usageEventBody("msg-1")},
	})
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestHandler_IngestUsageEventsBatch_TimestampBecomesCreatedAt proves the
// client-supplied `timestamp` (unlike cost/source) is a legitimate input
// that flows straight through to created_at, not overwritten with
// time.Now() server-side.
func TestHandler_IngestUsageEventsBatch_TimestampBecomesCreatedAt(t *testing.T) {
	router, conn := newIntegrationRouter(t)
	_, rawKey := createAgentWithKey(t, conn, "collector-1")

	rec := doJSONRequest(t, router, http.MethodPost, "/api/v1/usage-events/batch", rawKey, map[string]any{
		"install_id": "install-1",
		"hostname":   "thw-home",
		"events":     []map[string]any{usageEventBody("msg-1")},
	})
	require.Equal(t, http.StatusCreated, rec.Code)

	var gotCreatedAt time.Time
	require.NoError(t, conn.QueryRow(`SELECT created_at FROM usage_events WHERE id = ?`, "msg-1").Scan(&gotCreatedAt))
	assert.True(t, gotCreatedAt.Equal(time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)))
}
