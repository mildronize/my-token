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

	// machines is UpsertMachine's own in-memory store (installID ->
	// hostname), read back by MachineHostnames — a plain map stand-in for
	// the real ON CONFLICT DO UPDATE upsert (repo_test.go proves that
	// against a real database).
	machines          map[string]string
	upsertMachineErr  error
	hostnamesErr      error
	upsertMachineCall []upsertMachineCall

	// scanRoots is UpsertScanRoot's own in-memory store, keyed by
	// installID+"|"+scanRootPath — a plain map stand-in for the real
	// INSERT OR REPLACE upsert (repo_test.go proves that against a real
	// database), mirroring machines above.
	scanRoots          map[string]scanRootRecord
	upsertScanRootErr  error
	upsertScanRootCall []upsertScanRootCall
}

type upsertMachineCall struct {
	installID, hostname string
	lastSeenAt          time.Time
}

type scanRootRecord struct {
	name, sourceType string
	lastSeenAt       time.Time
}

type upsertScanRootCall struct {
	installID, scanRootPath, name, sourceType string
	lastSeenAt                                time.Time
}

type windowCall struct {
	start, end time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{insertedIDs: map[string]bool{}, machines: map[string]string{}, scanRoots: map[string]scanRootRecord{}}
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

func (f *fakeRepo) UpsertMachine(ctx context.Context, installID, hostname string, lastSeenAt time.Time) error {
	f.upsertMachineCall = append(f.upsertMachineCall, upsertMachineCall{installID: installID, hostname: hostname, lastSeenAt: lastSeenAt})
	if f.upsertMachineErr != nil {
		return f.upsertMachineErr
	}
	f.machines[installID] = hostname
	return nil
}

func (f *fakeRepo) MachineHostnames(ctx context.Context) (map[string]string, error) {
	if f.hostnamesErr != nil {
		return nil, f.hostnamesErr
	}
	out := make(map[string]string, len(f.machines))
	for k, v := range f.machines {
		out[k] = v
	}
	return out, nil
}

func (f *fakeRepo) UpsertScanRoot(ctx context.Context, installID, scanRootPath, name, sourceType string, lastSeenAt time.Time) error {
	f.upsertScanRootCall = append(f.upsertScanRootCall, upsertScanRootCall{
		installID: installID, scanRootPath: scanRootPath, name: name, sourceType: sourceType, lastSeenAt: lastSeenAt,
	})
	if f.upsertScanRootErr != nil {
		return f.upsertScanRootErr
	}
	f.scanRoots[installID+"|"+scanRootPath] = scanRootRecord{name: name, sourceType: sourceType, lastSeenAt: lastSeenAt}
	return nil
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

// --- Service.UpsertMachine / group_by=machine hostname substitution ----
// story-1/ticket-18.

// TestService_UpsertMachine_DelegatesToRepoWithNow proves Service.
// UpsertMachine is a thin pass-through to Repo.UpsertMachine, carrying
// the caller-supplied `now` straight through as last_seen_at — the real
// upsert-overwrites-not-insert-once behavior against a real database is
// this ticket's own explicitly-called-out test, proven in repo_test.go
// instead (a fake map can't demonstrate a real SQL upsert).
func TestService_UpsertMachine_DelegatesToRepoWithNow(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	now := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)

	err := svc.UpsertMachine(context.Background(), "install-1", "thw-home", now)
	require.NoError(t, err)

	require.Len(t, repo.upsertMachineCall, 1)
	assert.Equal(t, "install-1", repo.upsertMachineCall[0].installID)
	assert.Equal(t, "thw-home", repo.upsertMachineCall[0].hostname)
	assert.Equal(t, now, repo.upsertMachineCall[0].lastSeenAt)
}

func TestService_UpsertMachine_RepoError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	repo.upsertMachineErr = errors.New("db down")
	svc := NewService(repo)

	err := svc.UpsertMachine(context.Background(), "install-1", "thw-home", time.Now())
	assert.ErrorIs(t, err, repo.upsertMachineErr)
}

// TestService_Summary_GroupByMachine_SubstitutesHostname is this ticket's
// own core acceptance criterion for the read side: group_by=machine's
// breakdown key comes back as machines.hostname, not the raw install_id
// Aggregate originally grouped by — and RawKey carries that install_id so
// the console can still show it (contract's "machine label" rule: raw
// install_id reachable via tooltip).
func TestService_Summary_GroupByMachine_SubstitutesHostname(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	repo.events = []Event{ev("freya", "p1", "install-1", 100, 0, 0, 0, 1.0)}
	repo.events[0].CreatedAt = now.Add(-1 * time.Hour)
	repo.machines["install-1"] = "thw-home"

	svc := NewService(repo)
	got, err := svc.Summary(context.Background(), Window24h, GroupByMachine, now)
	require.NoError(t, err)

	require.Len(t, got.Breakdown, 1)
	assert.Equal(t, "thw-home", got.Breakdown[0].Key, "key must be the hostname, not the raw install_id")
	assert.Equal(t, "install-1", got.Breakdown[0].RawKey, "the raw install_id must still be reachable")
}

// TestService_Summary_GroupByMachine_NoMachinesRow_FallsBackToInstallID
// covers the contract's own explicitly-named edge case: a machine with
// usage_events rows but no corresponding machines row (shouldn't happen
// given upsert-on-every-batch, but must never show a blank/null key).
func TestService_Summary_GroupByMachine_NoMachinesRow_FallsBackToInstallID(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	repo.events = []Event{ev("freya", "p1", "install-orphan", 100, 0, 0, 0, 1.0)}
	repo.events[0].CreatedAt = now.Add(-1 * time.Hour)
	// Deliberately no repo.machines["install-orphan"] entry.

	svc := NewService(repo)
	got, err := svc.Summary(context.Background(), Window24h, GroupByMachine, now)
	require.NoError(t, err)

	require.Len(t, got.Breakdown, 1)
	assert.Equal(t, "install-orphan", got.Breakdown[0].Key, "must fall back to the raw install_id, never a blank key")
	assert.Empty(t, got.Breakdown[0].RawKey, "no substitution happened, so there is no separate raw value")
}

// TestService_Summary_GroupByActor_NeverConsultsMachineHostnames proves
// the substitution pass is scoped to group_by=machine only — a
// group_by=actor request must not even call MachineHostnames, let alone
// let it affect the breakdown keys.
func TestService_Summary_GroupByActor_NeverConsultsMachineHostnames(t *testing.T) {
	repo := newFakeRepo()
	repo.hostnamesErr = errors.New("must not be called")
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	repo.events = []Event{ev("freya", "p1", "install-1", 100, 0, 0, 0, 1.0)}
	repo.events[0].CreatedAt = now.Add(-1 * time.Hour)

	svc := NewService(repo)
	got, err := svc.Summary(context.Background(), Window24h, GroupByActor, now)
	require.NoError(t, err)
	assert.Equal(t, "freya", got.Breakdown[0].Key)
}

func TestService_Summary_GroupByMachine_HostnamesRepoError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	repo.hostnamesErr = errors.New("db down")
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	repo.events = []Event{ev("freya", "p1", "install-1", 100, 0, 0, 0, 1.0)}
	repo.events[0].CreatedAt = now.Add(-1 * time.Hour)

	svc := NewService(repo)
	_, err := svc.Summary(context.Background(), Window24h, GroupByMachine, now)
	assert.ErrorIs(t, err, repo.hostnamesErr)
}

// --- Service.Summary group_by=path: hostname substitution in the
// machine-prefixed key (story-2/ticket-7) -------------------------------
// Mirrors the group_by=machine substitution tests above — the contract's
// own "Integration/service test" acceptance criterion: Service.Summary
// with group_by=path returns Key/RawKey in the <hostname>:<path> /
// <install_id>:<path> shape.

// TestService_Summary_GroupByPath_SubstitutesHostnameInKeyPrefix is this
// ticket's own core acceptance criterion for the read side: group_by=path's
// breakdown key comes back as `<hostname>:<path>`, not the raw
// `<install_id>:<path>` Aggregate originally grouped by — and RawKey
// carries that raw key so the console can still show it via tooltip.
func TestService_Summary_GroupByPath_SubstitutesHostnameInKeyPrefix(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	repo.events = []Event{ev("freya", "/gits/my-token", "install-1", 100, 0, 0, 0, 1.0)}
	repo.events[0].CreatedAt = now.Add(-1 * time.Hour)
	repo.machines["install-1"] = "thw-home"

	svc := NewService(repo)
	got, err := svc.Summary(context.Background(), Window24h, GroupByPath, now)
	require.NoError(t, err)

	require.Len(t, got.Breakdown, 1)
	assert.Equal(t, "thw-home:/gits/my-token", got.Breakdown[0].Key, "key's install_id prefix must be substituted for the hostname, path suffix untouched")
	assert.Equal(t, "install-1:/gits/my-token", got.Breakdown[0].RawKey, "the raw install_id:path must still be reachable")
}

// TestService_Summary_GroupByPath_NoMachinesRow_LeavesRawInstallIDPrefix
// mirrors the group_by=machine no-machines-row fallback: a machine with
// usage_events rows but no corresponding machines row must never produce
// a blank/null key — the raw `install_id:path` key stays exactly as
// Aggregate produced it.
func TestService_Summary_GroupByPath_NoMachinesRow_LeavesRawInstallIDPrefix(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	repo.events = []Event{ev("freya", "/gits/my-token", "install-orphan", 100, 0, 0, 0, 1.0)}
	repo.events[0].CreatedAt = now.Add(-1 * time.Hour)
	// Deliberately no repo.machines["install-orphan"] entry.

	svc := NewService(repo)
	got, err := svc.Summary(context.Background(), Window24h, GroupByPath, now)
	require.NoError(t, err)

	require.Len(t, got.Breakdown, 1)
	assert.Equal(t, "install-orphan:/gits/my-token", got.Breakdown[0].Key, "must fall back to the raw install_id:path, never a blank key")
	assert.Empty(t, got.Breakdown[0].RawKey, "no substitution happened, so there is no separate raw value")
}

// TestService_Summary_GroupByActor_NeverConsultsMachineHostnames (above)
// already proves the substitution pass doesn't run for group_by=actor;
// this proves group_by=path's own MachineHostnames call errors propagate,
// the same as group_by=machine's own equivalent test does.
func TestService_Summary_GroupByPath_HostnamesRepoError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	repo.hostnamesErr = errors.New("db down")
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	repo.events = []Event{ev("freya", "/gits/my-token", "install-1", 100, 0, 0, 0, 1.0)}
	repo.events[0].CreatedAt = now.Add(-1 * time.Hour)

	svc := NewService(repo)
	_, err := svc.Summary(context.Background(), Window24h, GroupByPath, now)
	assert.ErrorIs(t, err, repo.hostnamesErr)
}

// --- Service.UpsertScanRoots (story-2/ticket-8) -------------------------

// TestService_UpsertScanRoots_DelegatesEachEntryToRepoWithNow proves
// UpsertScanRoots is a thin pass-through to Repo.UpsertScanRoot, once per
// entry of the batch's own scan_roots array, carrying the caller-supplied
// `now` straight through as last_seen_at for every entry — mirrors
// TestService_UpsertMachine_DelegatesToRepoWithNow. The real
// upsert-overwrites-not-insert-once behavior against a real database is
// this ticket's own explicitly-called-out test, proven in repo_test.go
// instead (a fake map can't demonstrate a real SQL upsert).
func TestService_UpsertScanRoots_DelegatesEachEntryToRepoWithNow(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	now := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)

	err := svc.UpsertScanRoots(context.Background(), "install-1", []UpsertScanRootInput{
		{Path: "/home/thw-home/.claude", Name: "main", SourceType: "claude_code"},
		{Path: "/home/thw-home/.claude-local", Name: "backup-install", SourceType: "claude_code"},
	}, now)
	require.NoError(t, err)

	require.Len(t, repo.upsertScanRootCall, 2)
	assert.Equal(t, upsertScanRootCall{
		installID: "install-1", scanRootPath: "/home/thw-home/.claude", name: "main", sourceType: "claude_code", lastSeenAt: now,
	}, repo.upsertScanRootCall[0])
	assert.Equal(t, upsertScanRootCall{
		installID: "install-1", scanRootPath: "/home/thw-home/.claude-local", name: "backup-install", sourceType: "claude_code", lastSeenAt: now,
	}, repo.upsertScanRootCall[1])
}

func TestService_UpsertScanRoots_EmptyArray_NoOp(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	err := svc.UpsertScanRoots(context.Background(), "install-1", nil, time.Now())
	require.NoError(t, err)
	assert.Empty(t, repo.upsertScanRootCall)
}

func TestService_UpsertScanRoots_RepoError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	repo.upsertScanRootErr = errors.New("db down")
	svc := NewService(repo)

	err := svc.UpsertScanRoots(context.Background(), "install-1", []UpsertScanRootInput{{Path: "/x", Name: "main", SourceType: "claude_code"}}, time.Now())
	assert.ErrorIs(t, err, repo.upsertScanRootErr)
}
