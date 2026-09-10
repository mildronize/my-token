package usage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo is a plain in-memory Repository used only for Service's own
// dispatch-shape tests below (does Service compute Cost/Source correctly
// before delegating, does it pass every field through unchanged) — the
// real idempotent-insert behavior against a real database is proven in
// repo_test.go instead (mirrors internal/domain/todo's own
// fakeRepo/real-repo split).
type fakeRepo struct {
	inserted []Event
	// insertedIDs simulates INSERT OR IGNORE's own dedup, so
	// TestService_IngestBatch_ReturnsReceivedAndInserted can exercise the
	// "some already existed" branch without a real database.
	insertedIDs map[string]bool
	err         error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{insertedIDs: map[string]bool{}}
}

func (f *fakeRepo) InsertBatch(ctx context.Context, batch []Event) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	var newlyInserted int64
	for _, e := range batch {
		if f.insertedIDs[e.ID] {
			continue
		}
		f.insertedIDs[e.ID] = true
		f.inserted = append(f.inserted, e)
		newlyInserted++
	}
	return newlyInserted, nil
}

func TestService_IngestBatch_ComputesCostAndSetsSource_NeverFromCaller(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	ts := time.Date(2026, 9, 10, 8, 30, 0, 0, time.UTC)
	_, _, err := svc.IngestBatch(context.Background(), []IngestEvent{
		{
			ID:                       "msg-1",
			SessionID:                "session-1",
			Actor:                    "freya",
			Path:                     "my-token",
			Machine:                  "install-1",
			Model:                    "claude-sonnet-4-5-20250929",
			InputTokens:              100,
			OutputTokens:             50,
			CacheReadInputTokens:     10,
			CacheCreationInputTokens: 5,
			Timestamp:                ts,
		},
	})
	require.NoError(t, err)
	require.Len(t, repo.inserted, 1)

	got := repo.inserted[0]
	assert.Equal(t, "msg-1", got.ID)
	assert.Equal(t, "session-1", got.SessionID)
	assert.Equal(t, "freya", got.Actor)
	assert.Equal(t, "my-token", got.Path)
	assert.Equal(t, "install-1", got.Machine)
	assert.Equal(t, "claude-sonnet-4-5-20250929", got.Model)
	assert.Equal(t, int64(100), got.InputTokens)
	assert.Equal(t, int64(50), got.OutputTokens)
	assert.Equal(t, int64(10), got.CacheReadInputTokens)
	assert.Equal(t, int64(5), got.CacheCreationInputTokens)
	assert.Equal(t, ts, got.CreatedAt)

	// Server-computed, not client-supplied — IngestEvent has no Cost/
	// Source field at all for a caller to have set in the first place;
	// this asserts the value Service actually computed matches the pure
	// pricing function directly, not just "is nonzero".
	wantCost := CostForUsage("claude-sonnet-4-5-20250929", 100, 50, 10, 5)
	assert.InDelta(t, wantCost, got.Cost, 1e-9)
	assert.Equal(t, "claude_code", got.Source)
}

func TestService_IngestBatch_ReturnsReceivedAndInserted(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	// Pre-seed one id as already-ingested (simulating a prior call), then
	// send a batch of three where one repeats it.
	repo.insertedIDs["msg-existing"] = true

	received, inserted, err := svc.IngestBatch(context.Background(), []IngestEvent{
		{ID: "msg-existing", Model: "sonnet", Timestamp: time.Now()},
		{ID: "msg-new-1", Model: "sonnet", Timestamp: time.Now()},
		{ID: "msg-new-2", Model: "sonnet", Timestamp: time.Now()},
	})
	require.NoError(t, err)
	assert.Equal(t, 3, received)
	assert.Equal(t, 2, inserted)
}

func TestService_IngestBatch_EmptyBatch_NoOp(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	received, inserted, err := svc.IngestBatch(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 0, received)
	assert.Equal(t, 0, inserted)
	assert.Empty(t, repo.inserted)
}

func TestService_IngestBatch_RepoError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	repo.err = errors.New("boom")
	svc := NewService(repo)

	_, _, err := svc.IngestBatch(context.Background(), []IngestEvent{{ID: "msg-1", Model: "sonnet"}})
	assert.ErrorIs(t, err, repo.err)
}
