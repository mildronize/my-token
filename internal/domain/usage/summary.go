package usage

import (
	"fmt"
	"sort"
	"time"
)

// Window is the fixed set of time ranges the console's own read surface
// offers (story-1/ticket-14, contract's API surface section, widened by
// story-1/ticket-20) — the exact spellings the wire query parameter and
// JSON `window` values use, never a caller-invented one.
type Window string

const (
	Window5h    Window = "5h"
	Window24h   Window = "24h"
	WindowToday Window = "today"
	WindowWeek  Window = "week"
	WindowMonth Window = "month"

	// WindowYear is story-1/ticket-20's addition — calendar-anchored at
	// Jan 1 00:00 UTC of the current year (WindowBounds, below), matching
	// today/week/month's own calendar-boundary convention, not a rolling
	// 365-day window.
	WindowYear Window = "year"

	// WindowLifetime was originally additive to the contract's own fixed
	// five (contract's API surface section originally named exactly
	// 5h|24h|today|week|month) — a build-time decision flagged in
	// ticket 14's own report: the ticket's own mockup spec names a
	// "lifetime cost" summary tile, which none of the five original
	// windows can answer (each has a bounded start). story-1/ticket-20
	// reopens ticket 14's "lifetime is tab-only-tile, never a tab" call:
	// lifetime (and year, above) are now both real, selectable tabs, and
	// both belong in the Windows slice below, not just as
	// GET /usage/summary query values.
	WindowLifetime Window = "lifetime"
)

// Windows lists every fixed window GET /usage/windows' own table renders,
// in that order (contract: "the fixed 5h/24h/today/week/month/year/
// lifetime table") — the one place that order is named, so
// GetUsageWindows's own handler doesn't have to repeat it. Also the full
// set ParseWindow (below) validates a wire-supplied window string
// against, since story-1/ticket-20 made every member of this slice a
// real, selectable tab — there is no longer a wider "parseable but not
// tabbed" set distinct from this one.
var Windows = []Window{Window5h, Window24h, WindowToday, WindowWeek, WindowMonth, WindowYear, WindowLifetime}

// ParseWindow validates a wire-supplied window string against every
// fixed window (Windows, above). The bff-openapi.yaml request validator
// already rejects an unrecognised value before a handler ever calls this
// (the `enum` on GetUsageSummaryParams.Window) — this exists so
// Service.Summary/Windows has one place to convert the generated wire
// type into this package's own Window, and so WindowBounds below has a
// total function to build on instead of accepting a bare string.
func ParseWindow(s string) (Window, bool) {
	for _, w := range Windows {
		if string(w) == s {
			return w, true
		}
	}
	return "", false
}

// WindowBounds computes the half-open [start, end) range window covers,
// anchored at now. end is always now itself — every window here is "from
// some start up to right now", never a range that ends in the past.
//
// Two different anchoring rules, both real dashboard conventions, chosen
// per window (story-1/ticket-14's own build-time decision — the contract
// names the original five windows but not how each one's start is
// computed, so this is this ticket's own call, flagged in the ticket
// report; story-1/ticket-20 adds `year` to the calendar-anchored group,
// following the exact same style):
//
//   - 5h/24h are rolling: start is exactly duration before now, so
//     "24h" always covers a full day of activity regardless of what time
//     of day it is right now.
//   - today/week/month/year are calendar-anchored at UTC: "today" starts
//     at the most recent UTC midnight, "week" at the most recent UTC
//     Monday 00:00, "month" at the 1st of the current UTC month at
//     00:00, "year" at Jan 1 of the current UTC year at 00:00 — the same
//     "which bucket does this dashboard say I'm in right now" framing a
//     calendar-based tab usually means, not a rolling 30/365-day
//     lookback. now is always treated as UTC (now.UTC()) so this is
//     deterministic regardless of the caller's local time zone — this
//     service has no per-user timezone concept to anchor to instead.
func WindowBounds(w Window, now time.Time) (start, end time.Time, err error) {
	now = now.UTC()
	end = now

	switch w {
	case Window5h:
		start = now.Add(-5 * time.Hour)
	case Window24h:
		start = now.Add(-24 * time.Hour)
	case WindowToday:
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	case WindowWeek:
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		// time.Weekday: Sunday=0 ... Saturday=6. Days since the most
		// recent Monday: Monday->0, Tuesday->1, ..., Sunday->6.
		daysSinceMonday := (int(startOfDay.Weekday()) + 6) % 7
		start = startOfDay.AddDate(0, 0, -daysSinceMonday)
	case WindowMonth:
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	case WindowYear:
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	case WindowLifetime:
		// Comfortably before any real usage_events row can exist (Claude
		// Code postdates this by years) — a fixed, safe left bound rather
		// than time.Time{} (year 1), which risks a driver-specific text
		// serialization quirk this domain has no reason to depend on.
		start = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("usage: unknown window %q", w)
	}
	return start, end, nil
}

// GroupBy is the fixed set of dimensions GET /usage/summary can break a
// window's totals down by (contract's API surface section).
type GroupBy string

const (
	GroupByActor   GroupBy = "actor"
	GroupByPath    GroupBy = "path"
	GroupByMachine GroupBy = "machine"
)

// ParseGroupBy validates a wire-supplied group_by string, mirroring
// ParseWindow above (same reasoning: the request validator already
// rejects anything outside this set, this just gives Service a typed
// value to switch on).
func ParseGroupBy(s string) (GroupBy, bool) {
	switch GroupBy(s) {
	case GroupByActor, GroupByPath, GroupByMachine:
		return GroupBy(s), true
	default:
		return "", false
	}
}

// keyFor reads whichever field of e the requested dimension names — the
// one place Aggregate touches Event's individual dimension fields, so
// adding a future group_by option is a one-line change here plus the
// wire enum, not a change scattered across Aggregate's own logic.
//
// GroupByPath and GroupByActor's keys are both machine-prefixed
// (`<machine>:<path>`/`<machine>:<actor>`, story-2/ticket-7 for path,
// ticket-16 for actor — supersedes each one's own former bare field):
// two machines reporting the exact same literal path *or* actor string
// are two distinct breakdown rows, not one silently merged row. Actor
// needed the identical fix path already had once ticket-7 redefined
// `actor` to be a raw directory path too (the session's launch cwd) —
// the exact same identical-string collision risk `path` was fixed
// against now applies equally to `actor` (มายด์ caught this by testing
// two collector installs sharing one filesystem, where an identical
// raw actor path from both is a real, not hypothetical, case). No schema
// change either time; `machine` already exists on every event.
// Service.Summary substitutes each prefix for a display hostname
// afterward (service.go's substituteMachineHostnamesInPathKeys/
// InActorKeys), the same "aggregate on the raw join key, substitute for
// display after" pattern GroupByMachine's own hostname substitution
// already uses.
func (g GroupBy) keyFor(e Event) string {
	switch g {
	case GroupByActor:
		return e.Machine + ":" + e.Actor
	case GroupByPath:
		return e.Machine + ":" + e.Path
	case GroupByMachine:
		return e.Machine
	default:
		return ""
	}
}

// excludedFromPathBreakdown implements the contract's "Path/actor dedup"
// rule (story-2/ticket-7, new — no story-1 precedent): a session whose
// `path` equals that same session's own raw launch cwd never touched
// anything outside where it started, so `path` and `actor` are the same
// underlying fact for it — By path must not show it as a second,
// separate thing (goal.md point 4). Since story-2/ticket-7 also redefines
// `actor` to be exactly that raw launch cwd (actor.go's ActorFromCwd),
// the raw-cwd half of the contract's equality test ("path equals cwd, or
// its git-root") reduces to this direct field comparison — the only half
// this ticket's own Verifiable section actually names and tests. The
// git-root half is deliberately NOT independently recomputed here:
// Aggregate is a pure function with no resolver/database dependency
// (contract's Testing Decisions), and it runs on the core server against
// events that may have been reported by a different machine entirely, so
// live git-root resolution against Actor at aggregation time is not just
// out of scope but structurally impossible (the path may not even exist
// on this machine). One accepted residual gap this leaves, relative to
// goal.md point 4's plain-English intent: a session launched from a
// subdirectory of a git repo that never touched anything outside that
// repo has Path (git-rooted, per the collector's own resolution chain)
// != Actor (the raw, un-rooted launch cwd) — that case is NOT deduped by
// this check. Fixing it would need a new field/scope the contract doesn't
// grant here, not a bug in what this ticket actually asked for. Scoped to
// group_by=path only — this event's contribution to every other group_by
// dimension,
// and to Totals/ReportingInstalls, is unaffected.
func excludedFromPathBreakdown(e Event) bool {
	return e.Path == e.Actor
}

// Totals is GET /usage/summary's `totals` object and one fixed window's
// row in GET /usage/windows' table — both are "turns/tokens/cost for
// some set of events", just scoped differently by their own caller.
type Totals struct {
	Tokens int64
	Cost   float64
	Turns  int64
}

// BreakdownRow is one entry in GET /usage/summary's `breakdown` array —
// Key is the dimension's stored value (an actor name, a canonicalized
// path, or — story-1/ticket-18 — a machine's hostname once
// Service.Summary substitutes it in for group_by=machine); display
// shortening (contract's "Console display rules") is the SPA's own job
// over this value, never done here.
//
// RawKey is empty except when Service.Summary has substituted Key for a
// friendlier display value:
//   - group_by=machine (story-1/ticket-18): Key becomes machines.hostname,
//     RawKey carries the install_id Aggregate originally grouped by.
//   - group_by=path (story-2/ticket-7): Key becomes `<hostname>:<path>`,
//     RawKey carries the ground-truth `<install_id>:<path>` Aggregate's
//     own keyFor(GroupByPath) originally produced.
//   - group_by=actor (story-2/ticket-16): same shape, Key becomes
//     `<hostname>:<actor>`, RawKey carries `<install_id>:<actor>`.
//
// All three cases exist for the same reason (the contract's "machine
// label"/"Cross-machine path identity" rules, and ticket-16's identical
// extension of the latter to actor): the raw join key must stay
// reachable somewhere (the console's tooltip) even once Key itself shows
// a friendlier value. Aggregate itself never sets this field; it is
// populated only by Service.Summary's post-aggregation substitution pass.
type BreakdownRow struct {
	Key    string
	RawKey string
	Tokens int64
	Cost   float64
	Turns  int64
}

// SummaryResult is Service.Summary's return value — Totals across every
// event in the window, Breakdown split by the requested GroupBy (cost
// descending, ties broken by Key for a deterministic order tests can
// assert on), and ReportingInstalls (distinct machines in the window,
// computed from the same event set regardless of which dimension
// Breakdown is grouped by — contract: "reporting_installs ... window-
// scoped ... this is the only definition that always matches what the
// displayed totals actually cover").
type SummaryResult struct {
	Totals            Totals
	Breakdown         []BreakdownRow
	ReportingInstalls int64
}

// tokensFor sums every one of an event's four raw token counters into
// one display number — distinct from ticket 9's "never summed into each
// other" rule, which is about what gets *stored* per field (input vs.
// output vs. the two cache counters must never be added together and
// re-stored as a single column); this is a read-side display total
// computed fresh on every request, the stored columns are untouched.
func tokensFor(e Event) int64 {
	return e.InputTokens + e.OutputTokens + e.CacheReadInputTokens + e.CacheCreationInputTokens
}

// Aggregate is the pure function behind both Service.Summary (with a
// real GroupBy) and Service.Windows (called once per fixed window with
// events already filtered to that window — Windows has no group_by at
// all, so it only ever reads the returned Totals, not Breakdown).
// Receives exactly the events already inside the target window — it has
// no notion of time itself, which is what makes it testable with plain
// constructed Event slices, no clock/database involved.
func Aggregate(events []Event, groupBy GroupBy) SummaryResult {
	var totals Totals
	machines := map[string]struct{}{}
	byKey := map[string]*BreakdownRow{}
	var order []string

	for _, e := range events {
		tok := tokensFor(e)
		totals.Tokens += tok
		totals.Cost += e.Cost
		totals.Turns++

		if e.Machine != "" {
			machines[e.Machine] = struct{}{}
		}

		if groupBy == GroupByPath && excludedFromPathBreakdown(e) {
			// Path/actor dedup (contract's "Path/actor dedup" section):
			// still counted in Totals/ReportingInstalls above, just left
			// out of group_by=path's own breakdown — applied here, before
			// ranking/truncation, so it can never occupy a top-N slot a
			// real project path should have.
			continue
		}

		key := groupBy.keyFor(e)
		row, ok := byKey[key]
		if !ok {
			row = &BreakdownRow{Key: key}
			byKey[key] = row
			order = append(order, key)
		}
		row.Tokens += tok
		row.Cost += e.Cost
		row.Turns++
	}

	breakdown := make([]BreakdownRow, 0, len(order))
	for _, key := range order {
		breakdown = append(breakdown, *byKey[key])
	}
	sort.SliceStable(breakdown, func(i, j int) bool {
		if breakdown[i].Cost != breakdown[j].Cost {
			return breakdown[i].Cost > breakdown[j].Cost
		}
		return breakdown[i].Key < breakdown[j].Key
	})

	return SummaryResult{
		Totals:            totals,
		Breakdown:         breakdown,
		ReportingInstalls: int64(len(machines)),
	}
}
