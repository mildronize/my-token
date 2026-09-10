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
// repo_test.go instead.
type fakeRepo struct {
	inserted []Event
	// insertedIDs simulates INSERT OR IGNORE's own dedup, so
	// TestService_IngestBatch_ReturnsReceivedAndInserted can exercise the
	// "some already existed" branch without a real database.
	insertedIDs map[string]bool
	err         error

	// events is what ListEventsInWindow serves — Summary/Windows tests
	// seed this directly rather than going through InsertBatch, since
	// they're testing the window-selection wiring (does Service pass the
	// right [start, end) to the repo), not insertion.
	events        []Event
	windowCalls   []windowCall
	listWindowErr error
}

type windowCall struct {
	start, end time.Time
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

// ListEventsInWindow returns every seeded event whose CreatedAt falls in
// [start, end) — a plain in-memory stand-in for repo.go's real SQL
// filter, and records the call so tests can assert Service computed the
// range it meant to.
func (f *fakeRepo) ListEventsInWindow(ctx context.Context, start, end time.Time) ([]Event, error) {
	f.windowCalls = append(f.windowCalls, windowCall{start: start, end: end})
	if f.listWindowErr != nil {
		return nil, f.listWindowErr
	}
	var out []Event
	for _, e := range f.events {
		if !e.CreatedAt.Before(start) && e.CreatedAt.Before(end) {
			out = append(out, e)
		}
	}
	return out, nil
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

// --- Service.Summary / Service.Windows: window-selection wiring --------
//
// summary_test.go already covers WindowBounds' own boundary math and
// Aggregate's own group_by logic exhaustively as pure functions with no
// repo involved at all — these tests exist only to prove Service wires
// the two together correctly: it asks the repo for the range WindowBounds
// actually computed, and hands whatever comes back to Aggregate unmodified.

func TestService_Summary_PassesWindowBoundsRangeToRepo(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	_, err := svc.Summary(context.Background(), Window5h, GroupByActor, now)
	require.NoError(t, err)

	require.Len(t, repo.windowCalls, 1)
	wantStart, wantEnd, _ := WindowBounds(Window5h, now)
	assert.Equal(t, wantStart, repo.windowCalls[0].start)
	assert.Equal(t, wantEnd, repo.windowCalls[0].end)
}

func TestService_Summary_AggregatesOnlyEventsTheRepoReturned(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	repo.events = []Event{
		ev("freya", "p1", "m1", 100, 0, 0, 0, 1.0),
		ev("freya", "p1", "m1", 100, 0, 0, 0, 1.0),
	}
	// Only the second event actually falls inside the window (an hour
	// before now); the first is long before any real window's start.
	repo.events[0].CreatedAt = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	repo.events[1].CreatedAt = now.Add(-1 * time.Hour)

	svc := NewService(repo)
	got, err := svc.Summary(context.Background(), Window24h, GroupByActor, now)
	require.NoError(t, err)
	assert.Equal(t, int64(1), got.Totals.Turns)
}

func TestService_Summary_UnknownWindow_Errors(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	_, err := svc.Summary(context.Background(), Window("fortnight"), GroupByActor, time.Now())
	assert.Error(t, err)
}

func TestService_Summary_RepoError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	repo.listWindowErr = errors.New("db down")
	svc := NewService(repo)
	_, err := svc.Summary(context.Background(), Window24h, GroupByActor, time.Now())
	assert.ErrorIs(t, err, repo.listWindowErr)
}

// TestService_Windows_QueriesEveryFixedWindowInOrder is this method's own
// core contract: one repo call per fixed window (5h, 24h, today, week,
// month — summary.go's package-level Windows slice), each with the range
// WindowBounds computes for that window, returned in that same order.
func TestService_Windows_QueriesEveryFixedWindowInOrder(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	rows, err := svc.Windows(context.Background(), now)
	require.NoError(t, err)

	require.Len(t, rows, len(Windows))
	require.Len(t, repo.windowCalls, len(Windows))
	for i, w := range Windows {
		assert.Equal(t, w, rows[i].Window)
		wantStart, wantEnd, _ := WindowBounds(w, now)
		assert.Equal(t, wantStart, repo.windowCalls[i].start, w)
		assert.Equal(t, wantEnd, repo.windowCalls[i].end, w)
	}
}

func TestService_Windows_TotalsReflectEachWindowsOwnEvents(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	// One event 3 hours ago (inside 5h and every wider window) and one
	// event 10 hours ago (outside 5h, inside 24h and every wider window).
	e1 := ev("freya", "p1", "m1", 100, 0, 0, 0, 1.0)
	e1.CreatedAt = now.Add(-3 * time.Hour)
	e2 := ev("freya", "p1", "m1", 100, 0, 0, 0, 1.0)
	e2.CreatedAt = now.Add(-10 * time.Hour)
	repo.events = []Event{e1, e2}

	svc := NewService(repo)
	rows, err := svc.Windows(context.Background(), now)
	require.NoError(t, err)

	byWindow := map[Window]WindowTotals{}
	for _, r := range rows {
		byWindow[r.Window] = r
	}
	assert.Equal(t, int64(1), byWindow[Window5h].Totals.Turns, "5h should only see the 3-hours-ago event")
	assert.Equal(t, int64(2), byWindow[Window24h].Totals.Turns, "24h should see both")
}

func TestService_Windows_RepoError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	repo.listWindowErr = errors.New("db down")
	svc := NewService(repo)
	_, err := svc.Windows(context.Background(), time.Now())
	assert.ErrorIs(t, err, repo.listWindowErr)
}
