package usage

import (
	"context"
	"errors"
	"strings"
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

	// listScanRootsResult/listScanRootsErr back ListScanRoots directly
	// (story-2/ticket-9) — a plain stand-in for the real SQL read
	// (repo_test.go proves that against a real database), mirroring how
	// f.events stands in for ListEventsInWindow above. Kept separate from
	// f.scanRoots (UpsertScanRoot's own in-memory store) so
	// Service.ScanRoots tests can seed exactly the rows they want without
	// going through UpsertScanRoot first.
	listScanRootsResult []ScanRootRecord
	listScanRootsErr    error

	// machineSummariesResult/machineSummariesErr back ListMachineSummaries
	// directly (story-3/ticket-1) — a plain stand-in for the real SQL
	// aggregation (repo_test.go proves that against a real database),
	// mirroring listScanRootsResult above. Service.MachineSummaries is a
	// thin pass-through (service.go's own doc comment), so these tests
	// only need to prove that pass-through, not re-prove the aggregation
	// itself.
	machineSummariesResult []MachineSummary
	machineSummariesErr    error
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
	filter     Filter
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
// filter, and records the call (including the filter it was given) so
// tests can assert Service computed the range/filter it meant to.
//
// story-2/ticket-10: filter.Machine/filter.ScanRoot are applied the same
// way repo.go's real SQL AND-composes them — filter.ScanRoot's composite
// is split on ":" here too (a plain in-memory stand-in, not a reuse of
// repo.go's own unexported splitScanRootFilter, mirroring how this fake
// already stands in for the real SQL window filter above rather than
// calling into repo.go itself).
func (f *fakeRepo) ListEventsInWindow(ctx context.Context, start, end time.Time, filter Filter) ([]Event, error) {
	f.windowCalls = append(f.windowCalls, windowCall{start: start, end: end, filter: filter})
	if f.listWindowErr != nil {
		return nil, f.listWindowErr
	}

	var scanRootInstallID, scanRootPath string
	scanRootOK := true
	if filter.ScanRoot != "" {
		var found bool
		scanRootInstallID, scanRootPath, found = strings.Cut(filter.ScanRoot, ":")
		scanRootOK = found
	}
	if !scanRootOK {
		// Malformed shape -- mirrors repo.go's own real return value
		// (an empty slice, never a bare nil) for the same case, contract's
		// own tolerance ("returns an empty/zero result, not an error").
		return []Event{}, nil
	}

	var out []Event
	for _, e := range f.events {
		if !e.CreatedAt.Before(start) && e.CreatedAt.Before(end) &&
			(filter.Machine == "" || e.Machine == filter.Machine) &&
			(filter.ScanRoot == "" || (e.Machine == scanRootInstallID && e.ScanRoot == scanRootPath)) {
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

func (f *fakeRepo) ListScanRoots(ctx context.Context) ([]ScanRootRecord, error) {
	if f.listScanRootsErr != nil {
		return nil, f.listScanRootsErr
	}
	return f.listScanRootsResult, nil
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

func (f *fakeRepo) ListMachineSummaries(ctx context.Context) ([]MachineSummary, error) {
	if f.machineSummariesErr != nil {
		return nil, f.machineSummariesErr
	}
	return f.machineSummariesResult, nil
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

	_, err := svc.Summary(context.Background(), Window5h, GroupByActor, Filter{}, now)
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
	got, err := svc.Summary(context.Background(), Window24h, GroupByActor, Filter{}, now)
	require.NoError(t, err)
	assert.Equal(t, int64(1), got.Totals.Turns)
}

// TestService_Summary_PassesFilterToRepoUnmodified is story-2/ticket-10's
// own wiring test: Summary hands whatever Filter it was given straight
// through to Repo.ListEventsInWindow, unmodified — the actual AND/OR
// composition and scan_root-composite decomposition are the repo layer's
// own job (repo_test.go's TestRepo_ListEventsInWindow_* tests), not
// Service's.
func TestService_Summary_PassesFilterToRepoUnmodified(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	filter := Filter{Machine: "install-a", ScanRoot: "install-a:/home/thw-home/.claude"}

	_, err := svc.Summary(context.Background(), Window24h, GroupByActor, filter, now)
	require.NoError(t, err)

	require.Len(t, repo.windowCalls, 1)
	assert.Equal(t, filter, repo.windowCalls[0].filter)
}

// TestService_Summary_MachineFilter_NarrowsBreakdownAndTotals is this
// ticket's own core acceptance test at the Service layer: when Filter.
// Machine is set, totals/breakdown are scoped to events the (fake) repo
// itself already narrowed to that machine — the exact same "the whole
// result is scoped, not just one panel" contract requirement, exercised
// one layer below the real HTTP route (usage_handler_test.go covers the
// HTTP route itself end to end).
func TestService_Summary_MachineFilter_NarrowsBreakdownAndTotals(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	e1 := ev("freya", "p1", "install-a", 100, 0, 0, 0, 1.0)
	e1.CreatedAt = now.Add(-1 * time.Hour)
	e2 := ev("nicole", "p1", "install-b", 200, 0, 0, 0, 5.0)
	e2.CreatedAt = now.Add(-1 * time.Hour)
	repo.events = []Event{e1, e2}

	svc := NewService(repo)
	got, err := svc.Summary(context.Background(), Window24h, GroupByActor, Filter{Machine: "install-a"}, now)
	require.NoError(t, err)

	assert.Equal(t, int64(1), got.Totals.Turns, "totals must be scoped to the filtered machine, not every event")
	assert.Equal(t, int64(100), got.Totals.Tokens)
	require.Len(t, got.Breakdown, 1)
	assert.Equal(t, "install-a:freya", got.Breakdown[0].Key, "story-2/ticket-16: group_by=actor's key is machine-prefixed too, no machines row seeded here")
	assert.Equal(t, int64(1), got.ReportingInstalls, "reporting_installs must also be scoped, not lifetime/unfiltered")
}

func TestService_Summary_UnknownWindow_Errors(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	_, err := svc.Summary(context.Background(), Window("fortnight"), GroupByActor, Filter{}, time.Now())
	assert.Error(t, err)
}

func TestService_Summary_RepoError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	repo.listWindowErr = errors.New("db down")
	svc := NewService(repo)
	_, err := svc.Summary(context.Background(), Window24h, GroupByActor, Filter{}, time.Now())
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

	rows, err := svc.Windows(context.Background(), Filter{}, now)
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

// TestService_Windows_PassesFilterToEveryRepoCall is story-2/ticket-10's
// own wiring test for Windows: the same Filter must reach every one of
// the seven per-window repo calls, not just the first — mirroring the
// contract's own "every row of the fixed table" scoping requirement.
func TestService_Windows_PassesFilterToEveryRepoCall(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	filter := Filter{Machine: "install-a", ScanRoot: "install-a:/home/thw-home/.claude"}

	_, err := svc.Windows(context.Background(), filter, now)
	require.NoError(t, err)

	require.Len(t, repo.windowCalls, len(Windows))
	for i, call := range repo.windowCalls {
		assert.Equal(t, filter, call.filter, "window index %d", i)
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
	rows, err := svc.Windows(context.Background(), Filter{}, now)
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
	_, err := svc.Windows(context.Background(), Filter{}, time.Now())
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
	got, err := svc.Summary(context.Background(), Window24h, GroupByMachine, Filter{}, now)
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
	got, err := svc.Summary(context.Background(), Window24h, GroupByMachine, Filter{}, now)
	require.NoError(t, err)

	require.Len(t, got.Breakdown, 1)
	assert.Equal(t, "install-orphan", got.Breakdown[0].Key, "must fall back to the raw install_id, never a blank key")
	assert.Empty(t, got.Breakdown[0].RawKey, "no substitution happened, so there is no separate raw value")
}

func TestService_Summary_GroupByMachine_HostnamesRepoError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	repo.hostnamesErr = errors.New("db down")
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	repo.events = []Event{ev("freya", "p1", "install-1", 100, 0, 0, 0, 1.0)}
	repo.events[0].CreatedAt = now.Add(-1 * time.Hour)

	svc := NewService(repo)
	_, err := svc.Summary(context.Background(), Window24h, GroupByMachine, Filter{}, now)
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
	got, err := svc.Summary(context.Background(), Window24h, GroupByPath, Filter{}, now)
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
	got, err := svc.Summary(context.Background(), Window24h, GroupByPath, Filter{}, now)
	require.NoError(t, err)

	require.Len(t, got.Breakdown, 1)
	assert.Equal(t, "install-orphan:/gits/my-token", got.Breakdown[0].Key, "must fall back to the raw install_id:path, never a blank key")
	assert.Empty(t, got.Breakdown[0].RawKey, "no substitution happened, so there is no separate raw value")
}

// TestService_Summary_GroupByActor_SubstitutesHostnameInKeyPrefix (below)
// proves group_by=actor's own equivalent substitution; this proves
// group_by=path's own MachineHostnames call errors propagate, the same
// as group_by=machine's own equivalent test does.
func TestService_Summary_GroupByPath_HostnamesRepoError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	repo.hostnamesErr = errors.New("db down")
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	repo.events = []Event{ev("freya", "/gits/my-token", "install-1", 100, 0, 0, 0, 1.0)}
	repo.events[0].CreatedAt = now.Add(-1 * time.Hour)

	svc := NewService(repo)
	_, err := svc.Summary(context.Background(), Window24h, GroupByPath, Filter{}, now)
	assert.ErrorIs(t, err, repo.hostnamesErr)
}

// --- Service.Summary group_by=actor: hostname substitution in the
// machine-prefixed key (story-2/ticket-16) -------------------------------
// Mirrors group_by=path's own substitution tests above exactly — actor's
// key shape and substitution logic are identical, just over the actor
// field instead of path.

// TestService_Summary_GroupByActor_SubstitutesHostnameInKeyPrefix is this
// ticket's own core acceptance criterion for the read side: group_by=actor's
// breakdown key comes back as `<hostname>:<actor>`, not the raw
// `<install_id>:<actor>` Aggregate originally grouped by — and RawKey
// carries that raw key so the console can still show it via tooltip.
func TestService_Summary_GroupByActor_SubstitutesHostnameInKeyPrefix(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	repo.events = []Event{ev("freya", "/gits/my-token", "install-1", 100, 0, 0, 0, 1.0)}
	repo.events[0].CreatedAt = now.Add(-1 * time.Hour)
	repo.machines["install-1"] = "thw-home"

	svc := NewService(repo)
	got, err := svc.Summary(context.Background(), Window24h, GroupByActor, Filter{}, now)
	require.NoError(t, err)

	require.Len(t, got.Breakdown, 1)
	assert.Equal(t, "thw-home:freya", got.Breakdown[0].Key, "key's install_id prefix must be substituted for the hostname, actor suffix untouched")
	assert.Equal(t, "install-1:freya", got.Breakdown[0].RawKey, "the raw install_id:actor must still be reachable")
}

// TestService_Summary_GroupByActor_NoMachinesRow_LeavesRawInstallIDPrefix
// mirrors the group_by=path no-machines-row fallback: a machine with
// usage_events rows but no corresponding machines row must never produce
// a blank/null key — the raw `install_id:actor` key stays exactly as
// Aggregate produced it.
func TestService_Summary_GroupByActor_NoMachinesRow_LeavesRawInstallIDPrefix(t *testing.T) {
	repo := newFakeRepo()
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	repo.events = []Event{ev("freya", "/gits/my-token", "install-orphan", 100, 0, 0, 0, 1.0)}
	repo.events[0].CreatedAt = now.Add(-1 * time.Hour)
	// Deliberately no repo.machines["install-orphan"] entry.

	svc := NewService(repo)
	got, err := svc.Summary(context.Background(), Window24h, GroupByActor, Filter{}, now)
	require.NoError(t, err)

	require.Len(t, got.Breakdown, 1)
	assert.Equal(t, "install-orphan:freya", got.Breakdown[0].Key, "must fall back to the raw install_id:actor, never a blank key")
	assert.Empty(t, got.Breakdown[0].RawKey, "no substitution happened, so there is no separate raw value")
}

func TestService_Summary_GroupByActor_HostnamesRepoError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	repo.hostnamesErr = errors.New("db down")
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	repo.events = []Event{ev("freya", "/gits/my-token", "install-1", 100, 0, 0, 0, 1.0)}
	repo.events[0].CreatedAt = now.Add(-1 * time.Hour)

	svc := NewService(repo)
	_, err := svc.Summary(context.Background(), Window24h, GroupByActor, Filter{}, now)
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

// --- Service.ScanRoots (story-2/ticket-9) -------------------------------
// Backs GET /api/bff/usage/scan-roots (ticket 9's Verifiable section):
// every scan_roots row, each joined with its machine's hostname the same
// way group_by=machine/group_by=path's own substitution already does —
// two separate repo reads combined in Go, not a SQL JOIN.

// TestService_ScanRoots_JoinsHostnameFromMachines is this ticket's own
// core acceptance test: a scan_roots row whose install_id has a
// corresponding machines row comes back with that row's hostname.
func TestService_ScanRoots_JoinsHostnameFromMachines(t *testing.T) {
	repo := newFakeRepo()
	repo.listScanRootsResult = []ScanRootRecord{
		{InstallID: "install-1", ScanRootPath: "/home/thw-home/.claude", Name: "main"},
	}
	repo.machines["install-1"] = "thw-home"

	svc := NewService(repo)
	got, err := svc.ScanRoots(context.Background())
	require.NoError(t, err)

	require.Len(t, got, 1)
	assert.Equal(t, ScanRootWithHostname{
		InstallID:    "install-1",
		Hostname:     "thw-home",
		ScanRootPath: "/home/thw-home/.claude",
		Name:         "main",
	}, got[0])
}

// TestService_ScanRoots_NoMachinesRow_FallsBackToInstallID covers the
// ticket's own explicitly-named edge case ("Verifiable" section): a
// scan_roots row whose install_id has no machines row falls back to
// showing the raw install_id as the hostname — the same "never a blank
// label" precedent machines' own fallback (substituteMachineHostnames)
// already sets.
func TestService_ScanRoots_NoMachinesRow_FallsBackToInstallID(t *testing.T) {
	repo := newFakeRepo()
	repo.listScanRootsResult = []ScanRootRecord{
		{InstallID: "install-orphan", ScanRootPath: "/home/thw-home/.claude", Name: "main"},
	}
	// Deliberately no repo.machines["install-orphan"] entry.

	svc := NewService(repo)
	got, err := svc.ScanRoots(context.Background())
	require.NoError(t, err)

	require.Len(t, got, 1)
	assert.Equal(t, "install-orphan", got[0].Hostname, "must fall back to the raw install_id, never a blank hostname")
}

// TestService_ScanRoots_ReturnsEveryRowAcrossInstalls proves ScanRoots
// doesn't scope to a single install — every registered scan_roots row
// across every reporting machine comes back.
func TestService_ScanRoots_ReturnsEveryRowAcrossInstalls(t *testing.T) {
	repo := newFakeRepo()
	repo.listScanRootsResult = []ScanRootRecord{
		{InstallID: "install-1", ScanRootPath: "/home/thw-home/.claude", Name: "main"},
		{InstallID: "install-2", ScanRootPath: "/home/thw-home/.claude", Name: "main"},
	}
	repo.machines["install-1"] = "thw-home"
	repo.machines["install-2"] = "thw-laptop"

	svc := NewService(repo)
	got, err := svc.ScanRoots(context.Background())
	require.NoError(t, err)

	assert.ElementsMatch(t, []ScanRootWithHostname{
		{InstallID: "install-1", Hostname: "thw-home", ScanRootPath: "/home/thw-home/.claude", Name: "main"},
		{InstallID: "install-2", Hostname: "thw-laptop", ScanRootPath: "/home/thw-home/.claude", Name: "main"},
	}, got)
}

// TestService_ScanRoots_NoRows_EmptySlice proves the zero-registered-roots
// case returns an empty slice, not an error, and never consults
// MachineHostnames at all (nothing to join).
func TestService_ScanRoots_NoRows_EmptySlice(t *testing.T) {
	repo := newFakeRepo()
	repo.hostnamesErr = errors.New("must not be called when there are no scan roots")

	svc := NewService(repo)
	got, err := svc.ScanRoots(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestService_ScanRoots_ListScanRootsRepoError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	repo.listScanRootsErr = errors.New("db down")

	svc := NewService(repo)
	_, err := svc.ScanRoots(context.Background())
	assert.ErrorIs(t, err, repo.listScanRootsErr)
}

func TestService_ScanRoots_MachineHostnamesRepoError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	repo.listScanRootsResult = []ScanRootRecord{
		{InstallID: "install-1", ScanRootPath: "/home/thw-home/.claude", Name: "main"},
	}
	repo.hostnamesErr = errors.New("db down")

	svc := NewService(repo)
	_, err := svc.ScanRoots(context.Background())
	assert.ErrorIs(t, err, repo.hostnamesErr)
}

// --- MachineSummaries: GET /api/bff/machines (story-3/ticket-1) --------
// Service.MachineSummaries is a thin pass-through over Repo.
// ListMachineSummaries (service.go's own doc comment) — these tests only
// prove that pass-through (result and error both flow through unchanged);
// the real aggregation/ordering SQL is proven against a real database in
// repo_test.go, and the HTTP wire shape end to end in bff/usage_handler_
// test.go.

// TestService_MachineSummaries_ReturnsRepoResultUnchanged is this
// method's own core acceptance test: whatever Repo.ListMachineSummaries
// returns comes back from Service.MachineSummaries exactly as given, no
// re-sort, no field transform.
func TestService_MachineSummaries_ReturnsRepoResultUnchanged(t *testing.T) {
	repo := newFakeRepo()
	want := []MachineSummary{
		{InstallID: "install-1", Hostname: "thw-home", LastSeenAt: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC), LifetimeCost: 4.5, LifetimeTokens: 1000, CollectedPaths: 3, CollectedActors: 2},
		{InstallID: "install-2", Hostname: "thw-laptop", LastSeenAt: time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC), LifetimeCost: 0, LifetimeTokens: 0, CollectedPaths: 0, CollectedActors: 0},
	}
	repo.machineSummariesResult = want

	svc := NewService(repo)
	got, err := svc.MachineSummaries(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestService_MachineSummaries_NoMachines_EmptySlice proves the
// zero-machines case returns whatever Repo itself returns for "nothing
// registered" (an empty/nil slice), not an error.
func TestService_MachineSummaries_NoMachines_EmptySlice(t *testing.T) {
	repo := newFakeRepo()

	svc := NewService(repo)
	got, err := svc.MachineSummaries(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

// TestService_MachineSummaries_RepoError_Propagates proves a real repo
// failure surfaces to the caller unchanged, mirroring every other
// Service method's own error-propagation test on this surface.
func TestService_MachineSummaries_RepoError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	repo.machineSummariesErr = errors.New("db down")

	svc := NewService(repo)
	_, err := svc.MachineSummaries(context.Background())
	assert.ErrorIs(t, err, repo.machineSummariesErr)
}
