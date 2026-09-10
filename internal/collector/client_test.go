package collector

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_PostBatch_SendsBearerAuthAndCorrectBody(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"received": 1, "inserted": 1})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tpl_test_key", "install-123", "test-host")
	events := []Event{{
		ID:                       "msg_1",
		SessionID:                "session-1",
		Actor:                    "freya",
		Path:                     "/home/thw-home/gits/my-token",
		Machine:                  "install-123",
		Model:                    "claude-sonnet-4-5",
		InputTokens:              10,
		OutputTokens:             20,
		CacheReadInputTokens:     0,
		CacheCreationInputTokens: 0,
		Timestamp:                time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC),
	}}

	result, err := client.PostBatch(events)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Received)
	assert.Equal(t, int64(1), result.Inserted)

	assert.Equal(t, "Bearer tpl_test_key", gotAuth)
	assert.Equal(t, "install-123", gotBody["install_id"])
	assert.Equal(t, "test-host", gotBody["hostname"])
	gotEvents, ok := gotBody["events"].([]any)
	require.True(t, ok)
	require.Len(t, gotEvents, 1)
	evt := gotEvents[0].(map[string]any)
	assert.Equal(t, "msg_1", evt["id"])
	assert.Equal(t, "freya", evt["actor"])
}

func TestClient_PostBatch_EmptyBatchIsNoOp(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	client := NewClient(server.URL, "tpl_test_key", "install-123", "test-host")
	result, err := client.PostBatch(nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Received)
	assert.False(t, called, "an empty batch should not make a request at all")
}

func TestClient_PostBatch_NonSuccessStatusIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-key", "install-123", "test-host")
	_, err := client.PostBatch([]Event{{ID: "msg_1"}})
	assert.Error(t, err)
}
