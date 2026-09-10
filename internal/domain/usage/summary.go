package usage

import (
	"fmt"
	"sort"
	"time"
)

// Window is the fixed set of time ranges the console's own read surface
// offers (story-1/ticket-14, contract's API surface section) — the exact
// five spellings the wire query parameter and JSON `window` values use,
// never a caller-invented sixth.
type Window string

const (
	Window5h    Window = "5h"
	Window24h   Window = "24h"
	WindowToday Window = "today"
	WindowWeek  Window = "week"
	WindowMonth Window = "month"

	// WindowLifetime is additive to the contract's own fixed five
	// (contract's API surface section names exactly
	// 5h|24h|today|week|month) — a build-time decision flagged in
	// ticket 14's own report, not silently added: the ticket's own
	// mockup spec names a "lifetime cost" summary tile, which none of
	// the five contract windows can answer (each has a bounded start).
	// Valid on GET /usage/summary's own `window` query parameter only
	// (bff-openapi.yaml) — deliberately excluded from the Windows slice
	// below, so GET /usage/windows' fixed table stays exactly the five
	// the contract names, in that order, untouched by this addition.
	WindowLifetime Window = "lifetime"
)

// Windows lists every fixed window GET /usage/windows' own table renders,
// in that order (contract: "the fixed 5h/24h/today/week/month table") —
// the one place that order is named, so GetUsageWindows's own handler
// doesn't have to repeat it. WindowLifetime is deliberately not a member
// of this slice (see its own doc comment) — GET /usage/summary accepts it
// as a query value regardless (parseableWindows, below, is the wider set
// ParseWindow actually validates against).
var Windows = []Window{Window5h, Window24h, WindowToday, WindowWeek, WindowMonth}

// parseableWindows is every window value GET /usage/summary's own
// `window` query parameter accepts — Windows (above) plus WindowLifetime.
// A separate slice from Windows on purpose: Windows' own meaning ("the
// fixed table's rows, in order") must stay exactly five long regardless
// of what ParseWindow accepts elsewhere.
var parseableWindows = append(append([]Window{}, Windows...), WindowLifetime)

// ParseWindow validates a wire-supplied window string against every
// value GET /usage/summary accepts (parseableWindows, above — Windows
// plus WindowLifetime). The bff-openapi.yaml request validator already
// rejects an unrecognised value before a handler ever calls this (the
// `enum` on GetUsageSummaryParams.Window) — this exists so
// Service.Summary/Windows has one place to convert the generated wire
// type into this package's own Window, and so WindowBounds below has a
// total function to build on instead of accepting a bare string.
func ParseWindow(s string) (Window, bool) {
	for _, w := range parseableWindows {
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
// names the five windows but not how each one's start is computed, so
// this is this ticket's own call, flagged in the ticket report):
//
//   - 5h/24h are rolling: start is exactly duration before now, so
//     "24h" always covers a full day of activity regardless of what time
//     of day it is right now.
//   - today/week/month are calendar-anchored at UTC: "today" starts at
//     the most recent UTC midnight, "week" at the most recent UTC Monday
//     00:00, "month" at the 1st of the current UTC month at 00:00 — the
//     same "which bucket does this dashboard say I'm in right now"
//     framing a calendar-based tab usually means, not a rolling 7/30-day
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
func (g GroupBy) keyFor(e Event) string {
	switch g {
	case GroupByActor:
		return e.Actor
	case GroupByPath:
		return e.Path
	case GroupByMachine:
		return e.Machine
	default:
		return ""
	}
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
// friendlier display value (group_by=machine: Key becomes
// machines.hostname, RawKey carries the install_id Aggregate originally
// grouped by) — the contract's "machine label" rule requires the raw
// install_id stay reachable (the console's tooltip), the same
// "shortened label, full value still reachable" pattern `path` already
// follows. Aggregate itself never sets this field; it is populated only
// by Service.Summary's post-aggregation substitution pass.
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
