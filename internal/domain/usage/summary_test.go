package usage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ParseWindow / ParseGroupBy ---------------------------------------

func TestParseWindow_ValidValues(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Window
	}{
		{"5h", Window5h},
		{"24h", Window24h},
		{"today", WindowToday},
		{"week", WindowWeek},
		{"month", WindowMonth},
		{"year", WindowYear},
		{"lifetime", WindowLifetime},
	} {
		got, ok := ParseWindow(tc.in)
		assert.True(t, ok, tc.in)
		assert.Equal(t, tc.want, got, tc.in)
	}
}

func TestParseWindow_InvalidValue(t *testing.T) {
	_, ok := ParseWindow("hour")
	assert.False(t, ok)
	_, ok = ParseWindow("")
	assert.False(t, ok)
}

func TestParseGroupBy_ValidValues(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want GroupBy
	}{
		{"actor", GroupByActor},
		{"path", GroupByPath},
		{"machine", GroupByMachine},
	} {
		got, ok := ParseGroupBy(tc.in)
		assert.True(t, ok, tc.in)
		assert.Equal(t, tc.want, got, tc.in)
	}
}

func TestParseGroupBy_InvalidValue(t *testing.T) {
	_, ok := ParseGroupBy("session")
	assert.False(t, ok)
}

// --- WindowBounds: rolling windows (5h, 24h) ---------------------------

func TestWindowBounds_5h_IsRollingFiveHoursEndingNow(t *testing.T) {
	now := time.Date(2026, 9, 10, 14, 37, 0, 0, time.UTC)
	start, end, err := WindowBounds(Window5h, now)
	require.NoError(t, err)
	assert.Equal(t, now, end)
	assert.Equal(t, now.Add(-5*time.Hour), start)
}

func TestWindowBounds_24h_IsRollingTwentyFourHoursEndingNow(t *testing.T) {
	now := time.Date(2026, 9, 10, 0, 15, 0, 0, time.UTC)
	start, end, err := WindowBounds(Window24h, now)
	require.NoError(t, err)
	assert.Equal(t, now, end)
	assert.Equal(t, now.Add(-24*time.Hour), start)
}

// --- WindowBounds: calendar-anchored windows (today, week, month) ------

func TestWindowBounds_Today_StartsAtUTCMidnightOfNow(t *testing.T) {
	now := time.Date(2026, 9, 10, 23, 59, 59, 0, time.UTC)
	start, end, err := WindowBounds(WindowToday, now)
	require.NoError(t, err)
	assert.Equal(t, now, end)
	assert.Equal(t, time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC), start)
}

func TestWindowBounds_Today_JustAfterMidnightStillAnchorsToTheSameDay(t *testing.T) {
	now := time.Date(2026, 9, 10, 0, 0, 1, 0, time.UTC)
	start, _, err := WindowBounds(WindowToday, now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC), start)
}

// 2026-09-10 is a Thursday — the most recent Monday is 2026-09-07.
func TestWindowBounds_Week_StartsAtMostRecentUTCMonday(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	start, end, err := WindowBounds(WindowWeek, now)
	require.NoError(t, err)
	assert.Equal(t, now, end)
	assert.Equal(t, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Monday, start.Weekday())
}

// A Monday itself is the start of its own week — zero days back, not
// treated as "the previous Monday" (an off-by-one that would otherwise
// silently include an extra week).
func TestWindowBounds_Week_OnAMondayStartsToday(t *testing.T) {
	now := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
	start, _, err := WindowBounds(WindowWeek, now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), start)
}

// A Sunday is 6 days after its week's Monday — the largest offset the
// modulo arithmetic has to get right.
func TestWindowBounds_Week_OnASundayGoesBackSixDays(t *testing.T) {
	now := time.Date(2026, 9, 13, 23, 0, 0, 0, time.UTC) // Sunday
	require.Equal(t, time.Sunday, now.Weekday())
	start, _, err := WindowBounds(WindowWeek, now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), start)
}

func TestWindowBounds_Month_StartsAtTheFirstOfTheCurrentUTCMonth(t *testing.T) {
	now := time.Date(2026, 9, 30, 23, 59, 0, 0, time.UTC)
	start, end, err := WindowBounds(WindowMonth, now)
	require.NoError(t, err)
	assert.Equal(t, now, end)
	assert.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), start)
}

func TestWindowBounds_NonUTCNowIsNormalizedToUTCFirst(t *testing.T) {
	loc := time.FixedZone("UTC+7", 7*60*60)
	// 2026-09-10 01:00 +07:00 is 2026-09-09 18:00 UTC — a different
	// calendar day. "today" must anchor to the UTC day, not the input's
	// own local day.
	now := time.Date(2026, 9, 10, 1, 0, 0, 0, loc)
	start, end, err := WindowBounds(WindowToday, now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC), start)
	assert.Equal(t, now.UTC(), end)
}

// --- WindowBounds: calendar-anchored window (year) ---------------------
// story-1/ticket-20: same calendar-boundary style as today/week/month
// above, anchored at Jan 1 00:00 UTC of the current year rather than a
// rolling 365-day lookback.

func TestWindowBounds_Year_StartsAtJan1UTCOfTheCurrentYear(t *testing.T) {
	now := time.Date(2026, 9, 10, 23, 59, 59, 0, time.UTC)
	start, end, err := WindowBounds(WindowYear, now)
	require.NoError(t, err)
	assert.Equal(t, now, end)
	assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), start)
}

// Jan 1st itself is the start of its own year — zero days back, not
// treated as "last year's Jan 1" (an off-by-one that would otherwise
// silently include an extra year), mirroring
// TestWindowBounds_Week_OnAMondayStartsToday's own reasoning for week.
func TestWindowBounds_Year_OnJan1stItselfStartsThatSameInstant(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC)
	start, _, err := WindowBounds(WindowYear, now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), start)
}

// A moment right at the very end of December must not leak into next
// year's bucket — the widest possible offset "year" has to get right,
// mirroring TestWindowBounds_Week_OnASundayGoesBackSixDays' own reasoning
// for week.
func TestWindowBounds_Year_OnDec31stStillAnchorsToJan1stOfTheSameYear(t *testing.T) {
	now := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	start, _, err := WindowBounds(WindowYear, now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), start)
}

// Mirrors TestWindowBounds_NonUTCNowIsNormalizedToUTCFirst, but the
// UTC-vs-local mismatch straddles a year boundary, not just a day
// boundary: 2026-01-01 01:00 +07:00 is 2025-12-31 18:00 UTC — a different
// calendar *year*. "year" must anchor to the UTC year, not the input's
// own local year, or this would wrongly start at 2026-01-01 instead of
// 2025-01-01.
func TestWindowBounds_Year_NonUTCNowNearYearBoundaryIsNormalizedToUTCFirst(t *testing.T) {
	loc := time.FixedZone("UTC+7", 7*60*60)
	now := time.Date(2026, 1, 1, 1, 0, 0, 0, loc)
	start, end, err := WindowBounds(WindowYear, now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), start)
	assert.Equal(t, now.UTC(), end)
}

// TestWindowBounds_Lifetime_HasNoLeftBoundWithinPlausibleRange proves
// WindowLifetime's start is comfortably before any real usage_events row
// could exist — the additive tile-only window (see WindowLifetime's own
// doc comment) — without hardcoding the exact sentinel value.
func TestWindowBounds_Lifetime_HasNoLeftBoundWithinPlausibleRange(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	start, end, err := WindowBounds(WindowLifetime, now)
	require.NoError(t, err)
	assert.Equal(t, now, end)
	assert.True(t, start.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)))
}

// TestWindows_ListsAllSevenFixedWindowsInOrder proves GET /usage/windows'
// own fixed table is exactly the contract's seven windows (story-1/
// ticket-20 grew this from five to seven — year and lifetime are now
// both real, selectable tabs, reopening ticket 14's "lifetime is
// tab-only-tile, never a tab" call), in the exact stated order.
func TestWindows_ListsAllSevenFixedWindowsInOrder(t *testing.T) {
	assert.Equal(t, []Window{
		Window5h, Window24h, WindowToday, WindowWeek, WindowMonth, WindowYear, WindowLifetime,
	}, Windows)
}

func TestWindowBounds_UnknownWindow_Errors(t *testing.T) {
	_, _, err := WindowBounds(Window("fortnight"), time.Now())
	assert.Error(t, err)
}

// --- Aggregate: group_by dimensions -------------------------------------

func ev(actor, path, machine string, in, out, cacheRead, cacheCreate int64, cost float64) Event {
	return Event{
		Actor:                    actor,
		Path:                     path,
		Machine:                  machine,
		InputTokens:              in,
		OutputTokens:             out,
		CacheReadInputTokens:     cacheRead,
		CacheCreationInputTokens: cacheCreate,
		Cost:                     cost,
	}
}

func TestAggregate_EmptyInput_ZeroTotalsNoBreakdownNoInstalls(t *testing.T) {
	got := Aggregate(nil, GroupByActor)
	assert.Equal(t, Totals{}, got.Totals)
	assert.Empty(t, got.Breakdown)
	assert.Equal(t, int64(0), got.ReportingInstalls)
}

// story-2/ticket-16 makes GroupByActor's key machine-prefixed too
// (`<machine>:<actor>`), same reason and shape as GroupByPath's own
// ticket-7 fix — all three events here share the same machine, so the
// prefix doesn't change which rows collapse together, just the key
// string itself.
func TestAggregate_ByActor_SumsTokensCostTurnsPerActor(t *testing.T) {
	events := []Event{
		ev("freya", "p1", "m1", 100, 50, 10, 5, 1.0), // tokens=165
		ev("freya", "p2", "m1", 200, 0, 0, 0, 2.0),   // tokens=200
		ev("nicole", "p1", "m1", 10, 10, 0, 0, 0.1),  // tokens=20
	}
	got := Aggregate(events, GroupByActor)

	assert.Equal(t, int64(385), got.Totals.Tokens)
	assert.InDelta(t, 3.1, got.Totals.Cost, 1e-9)
	assert.Equal(t, int64(3), got.Totals.Turns)

	require.Len(t, got.Breakdown, 2)
	// cost descending: freya (3.0) before nicole (0.1)
	assert.Equal(t, "m1:freya", got.Breakdown[0].Key)
	assert.Equal(t, int64(365), got.Breakdown[0].Tokens)
	assert.InDelta(t, 3.0, got.Breakdown[0].Cost, 1e-9)
	assert.Equal(t, int64(2), got.Breakdown[0].Turns)

	assert.Equal(t, "m1:nicole", got.Breakdown[1].Key)
	assert.Equal(t, int64(20), got.Breakdown[1].Tokens)
	assert.InDelta(t, 0.1, got.Breakdown[1].Cost, 1e-9)
	assert.Equal(t, int64(1), got.Breakdown[1].Turns)
}

// TestAggregate_ByPath_GroupsOnPathNotActor: story-2/ticket-7 makes
// GroupByPath's key machine-prefixed (`<machine>:<path>`, was bare
// `path`) — every event here shares the same machine ("m1"), so the
// prefix doesn't change how many rows result, just their Key shape.
func TestAggregate_ByPath_GroupsOnPathNotActor(t *testing.T) {
	events := []Event{
		ev("freya", "/gits/my-task", "m1", 100, 0, 0, 0, 1.0),
		ev("nicole", "/gits/my-task", "m1", 100, 0, 0, 0, 1.0),
		ev("freya", "/gits/my-token", "m1", 100, 0, 0, 0, 5.0),
	}
	got := Aggregate(events, GroupByPath)

	require.Len(t, got.Breakdown, 2)
	assert.Equal(t, "m1:/gits/my-token", got.Breakdown[0].Key) // higher cost first
	assert.InDelta(t, 5.0, got.Breakdown[0].Cost, 1e-9)
	assert.Equal(t, "m1:/gits/my-task", got.Breakdown[1].Key)
	assert.InDelta(t, 2.0, got.Breakdown[1].Cost, 1e-9)
	assert.Equal(t, int64(2), got.Breakdown[1].Turns)
}

// TestAggregate_ByPath_CrossMachineIdenticalPath_ProducesDistinctRows is
// the contract's own direct regression test for the false-merge bug
// story-1 documented and deferred (goal.md point 1, contract's
// "Cross-machine path identity" section): two Events with identical Path
// but different Machine must produce two distinct group_by=path
// breakdown rows, keyed `machine:path`, never one merged row.
func TestAggregate_ByPath_CrossMachineIdenticalPath_ProducesDistinctRows(t *testing.T) {
	events := []Event{
		ev("freya", "/gits/my-task", "install-a", 100, 0, 0, 0, 1.0),
		ev("nicole", "/gits/my-task", "install-b", 100, 0, 0, 0, 3.0),
	}
	got := Aggregate(events, GroupByPath)

	require.Len(t, got.Breakdown, 2, "the exact same literal path from two machines must not silently merge into one row")
	assert.Equal(t, "install-b:/gits/my-task", got.Breakdown[0].Key, "higher cost first")
	assert.InDelta(t, 3.0, got.Breakdown[0].Cost, 1e-9)
	assert.Equal(t, "install-a:/gits/my-task", got.Breakdown[1].Key)
	assert.InDelta(t, 1.0, got.Breakdown[1].Cost, 1e-9)
}

// TestAggregate_ByActor_CrossMachineIdenticalActor_ProducesDistinctRows is
// story-2/ticket-16's own direct regression test for the actor-side false-
// merge bug, mirroring TestAggregate_ByPath_CrossMachineIdenticalPath_
// ProducesDistinctRows exactly: two machines reporting the identical raw
// actor string (a real, not hypothetical, case for two collector installs
// sharing one filesystem — มายด์'s own finding) must not silently merge.
func TestAggregate_ByActor_CrossMachineIdenticalActor_ProducesDistinctRows(t *testing.T) {
	events := []Event{
		ev("/home/thw-home/.typ-crews/naomi", "p1", "install-a", 100, 0, 0, 0, 1.0),
		ev("/home/thw-home/.typ-crews/naomi", "p2", "install-b", 100, 0, 0, 0, 3.0),
	}
	got := Aggregate(events, GroupByActor)

	require.Len(t, got.Breakdown, 2, "the exact same literal actor path from two machines must not silently merge into one row")
	assert.Equal(t, "install-b:/home/thw-home/.typ-crews/naomi", got.Breakdown[0].Key, "higher cost first")
	assert.InDelta(t, 3.0, got.Breakdown[0].Cost, 1e-9)
	assert.Equal(t, "install-a:/home/thw-home/.typ-crews/naomi", got.Breakdown[1].Key)
	assert.InDelta(t, 1.0, got.Breakdown[1].Cost, 1e-9)
}

// --- Aggregate: Path/actor dedup (story-2/ticket-7, contract's
// "Path/actor dedup" section, new — no story-1 precedent) --------------

// TestAggregate_ByPath_ActorPathDedup_ExcludesSessionThatNeverLeftLaunchCwd
// is the contract's own table-driven acceptance test: an Event whose
// Path equals its own session's launch cwd (== Actor, per story-2/
// ticket-7's actor redefinition) is excluded from group_by=path's
// breakdown entirely; a sibling Event in the same session whose Path
// differs from that cwd is unaffected. Both events still count toward
// Totals/ReportingInstalls — the dedup rule only scopes group_by=path's
// own breakdown.
func TestAggregate_ByPath_ActorPathDedup_ExcludesSessionThatNeverLeftLaunchCwd(t *testing.T) {
	events := []Event{
		// This session's path never left its own launch cwd — path ==
		// actor is the dedup signal, must be excluded from the breakdown.
		ev("/home/thw-home/gits/my-token", "/home/thw-home/gits/my-token", "install-a", 100, 0, 0, 0, 1.0),
		// A sibling event in a DIFFERENT session whose path genuinely
		// differs from its own actor — unaffected, must still appear.
		ev("/home/thw-home/.typ-crews/freya", "/home/thw-home/gits/other-project", "install-a", 100, 0, 0, 0, 2.0),
	}
	got := Aggregate(events, GroupByPath)

	require.Len(t, got.Breakdown, 1, "the never-left-launch-cwd session must be excluded entirely")
	assert.Equal(t, "install-a:/home/thw-home/gits/other-project", got.Breakdown[0].Key)

	// Totals/ReportingInstalls are unaffected — the dedup rule only scopes
	// group_by=path's own breakdown, per the contract's own wording
	// ("excluded from group_by=path entirely", not from the event set).
	assert.Equal(t, int64(2), got.Totals.Turns)
	assert.Equal(t, int64(1), got.ReportingInstalls)
}

// TestAggregate_ByPath_ActorPathDedup_OnlyAppliesToGroupByPath proves the
// exclusion is scoped to group_by=path only — the same dedup-eligible
// event still appears normally under every other group_by dimension.
func TestAggregate_ByPath_ActorPathDedup_OnlyAppliesToGroupByPath(t *testing.T) {
	events := []Event{
		ev("/home/thw-home/gits/my-token", "/home/thw-home/gits/my-token", "install-a", 100, 0, 0, 0, 1.0),
	}

	byActor := Aggregate(events, GroupByActor)
	require.Len(t, byActor.Breakdown, 1)
	assert.Equal(t, "install-a:/home/thw-home/gits/my-token", byActor.Breakdown[0].Key, "story-2/ticket-16: GroupByActor's key is machine-prefixed too")

	byMachine := Aggregate(events, GroupByMachine)
	require.Len(t, byMachine.Breakdown, 1)
	assert.Equal(t, "install-a", byMachine.Breakdown[0].Key)

	byPath := Aggregate(events, GroupByPath)
	assert.Empty(t, byPath.Breakdown, "excluded from group_by=path's breakdown only")
}

func TestAggregate_ByMachine_GroupsOnMachine(t *testing.T) {
	events := []Event{
		ev("freya", "p1", "install-a", 100, 0, 0, 0, 1.0),
		ev("freya", "p1", "install-b", 100, 0, 0, 0, 3.0),
	}
	got := Aggregate(events, GroupByMachine)

	require.Len(t, got.Breakdown, 2)
	assert.Equal(t, "install-b", got.Breakdown[0].Key)
	assert.Equal(t, "install-a", got.Breakdown[1].Key)
}

func TestAggregate_BreakdownTieBrokenByKeyAscending(t *testing.T) {
	events := []Event{
		ev("zeta", "p", "m", 10, 0, 0, 0, 1.0),
		ev("alpha", "p", "m", 10, 0, 0, 0, 1.0),
	}
	got := Aggregate(events, GroupByActor)
	require.Len(t, got.Breakdown, 2)
	assert.Equal(t, "m:alpha", got.Breakdown[0].Key)
	assert.Equal(t, "m:zeta", got.Breakdown[1].Key)
}

// --- Aggregate: reporting_installs (window-scoped, not lifetime) -------

func TestAggregate_ReportingInstalls_CountsDistinctMachinesInGivenEventsOnly(t *testing.T) {
	events := []Event{
		ev("freya", "p1", "install-a", 10, 0, 0, 0, 1.0),
		ev("freya", "p2", "install-a", 10, 0, 0, 0, 1.0), // same machine again
		ev("nicole", "p1", "install-b", 10, 0, 0, 0, 1.0),
	}
	got := Aggregate(events, GroupByActor)
	assert.Equal(t, int64(2), got.ReportingInstalls)
}

// This is the whole point of "window-scoped, not lifetime": Aggregate has
// no notion of "every machine that ever reported" at all — it can only
// ever count machines present in the slice it's handed, which
// Service.Summary/Windows already filtered to the requested window
// (repo.go's ListEventsInWindow). A machine with events entirely outside
// the window is structurally invisible here, not filtered out by a
// separate step that could be forgotten.
func TestAggregate_ReportingInstalls_IsIndependentOfGroupByDimension(t *testing.T) {
	events := []Event{
		ev("freya", "p1", "install-a", 10, 0, 0, 0, 1.0),
		ev("freya", "p1", "install-b", 10, 0, 0, 0, 1.0),
	}
	byActor := Aggregate(events, GroupByActor)
	byPath := Aggregate(events, GroupByPath)
	byMachine := Aggregate(events, GroupByMachine)

	assert.Equal(t, int64(2), byActor.ReportingInstalls)
	assert.Equal(t, int64(2), byPath.ReportingInstalls)
	assert.Equal(t, int64(2), byMachine.ReportingInstalls)
}

func TestAggregate_ReportingInstalls_IgnoresEmptyMachineString(t *testing.T) {
	events := []Event{ev("freya", "p1", "", 10, 0, 0, 0, 1.0)}
	got := Aggregate(events, GroupByActor)
	assert.Equal(t, int64(0), got.ReportingInstalls)
}
