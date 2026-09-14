# Ticket 14 Report

## Ticket
Console — `GET /api/bff/usage/summary` + `GET /api/bff/usage/windows`, and
the embedded SPA console UI matching the approved mockup and the
contract's "Console display rules".

## Outcome
done

## Decision

- **Issue:** `reporting_installs`'s window-scoped-not-lifetime definition
  was already resolved and written into the ticket file before I started
  ("deciding it here as window-scoped ... Flag for confirmation during
  build if that's wrong"). I did not revisit it — implemented exactly as
  written (`internal/domain/usage/summary.go`'s `Aggregate` computes it
  from whatever event set the caller already filtered to the window, so
  it structurally cannot be lifetime-scoped by construction). Re-flagging
  here only because the ticket file asked for that, not because I found
  a new reason to doubt it.

- **Issue:** the ticket's own mockup spec (and the approved mockup itself,
  which I fetched and read in full) names three summary tiles: "lifetime
  cost", "this-window cost", "total tokens" (lifetime). The contract's API
  surface only defines five `window` values (`5h|24h|today|week|month`),
  none of which is unbounded — there is no way to answer "lifetime cost"
  from any of them.
  **Options considered:** (a) add a sixth `window=lifetime` value,
  additive to `GET /usage/summary`'s own query enum only, leaving
  `GET /usage/windows`' fixed five-row table untouched; (b) approximate
  lifetime with the widest existing window (`month`) and just mislabel
  the tile; (c) drop the lifetime tile entirely and only show two tiles.
  **Chosen:** (a). It's the only option that shows a real number under an
  honest label, it doesn't touch the contract's own fixed five-window
  table at all (`GET /usage/windows` still returns exactly those five, in
  that order — tested via `TestWindows_DoesNotIncludeLifetime`), and it's
  cheap to remove later if this reading turns out wrong. Flagging this
  for the loop orchestrator/มายด์ to confirm — it's a real, if small,
  widening of the contract's own named API surface, done to satisfy the
  ticket's own UI spec rather than silently mismatching it.

- **Issue:** the approved mockup's top bar shows a `host thw-home` pill.
  No hostname is persisted anywhere this endpoint can read — ticket 11's
  own ingestion handler reads `hostname` off
  `POST /api/v1/usage-events/batch`'s request body but explicitly
  discards it ("identify the reporting collector for observability only
  ... read and discarded here", `usage_handler.go`'s own doc comment).
  **Options considered:** (a) omit the host pill, document why; (b) fake
  it by treating `machine` (the install_id) as if it were a hostname; (c)
  add a `hostname` column to `usage_events` now, outside this ticket's
  own contract.
  **Chosen:** (a). (b) would display a UUID as if it meant something it
  doesn't; (c) is a schema change belonging to ticket 11's own contract,
  not something to smuggle into ticket 14. Flagging since it's a visible
  gap from the approved mockup, not a silent omission — worth a follow-up
  ticket against ticket 11/12 if a real hostname is wanted later.

## Notes

**What was built** (`/home/thw-home/gits/my-token`):

- `internal/domain/usage/summary.go`: `Window`/`GroupBy` types,
  `WindowBounds` (5h/24h rolling; today/week/month UTC-calendar-anchored;
  `lifetime` per the decision above), and `Aggregate` — a pure function
  over `[]Event` that computes totals, a cost-sorted breakdown by one
  dimension, and window-scoped `reporting_installs` in one pass, with no
  database or clock dependency.
- `internal/domain/usage/repo.go`: `ListEventsInWindow` (new sqlc query,
  half-open `[start, end)`), and `service.go`: `Summary`/`Windows` methods
  wiring `WindowBounds`→repo→`Aggregate` together.
- `internal/transport/bff/usage_handler.go`: `UsageServer` implementing
  `GetUsageSummary`/`GetUsageWindows`, owner-session-only like every other
  `/api/bff` endpoint, wired into `cmd/server/main.go`'s `bffServer`.
- `bff-openapi.yaml`: the two new paths + `UsageTotals`/
  `UsageBreakdownRow`/`UsageSummary`/`UsageWindowRow`/`UsageWindows`
  schemas, wire field names copied verbatim from the contract including
  its `snake_case` `group_by`/`reporting_installs` (a deliberate break
  from this surface's usual camelCase — the contract's own choice, not
  "corrected" here).
- Frontend (React 19 + Vite + TanStack Query + Tailwind — the stack
  already fully in place in this fork, confirmed by reading `web/`'s
  existing structure before assuming anything; no new dependency added):
  `web/src/lib/usage.ts` (query hooks), `web/src/lib/pathDisplay.ts` (the
  path-shortening pure function), `web/src/lib/format.ts`, and
  `web/src/app/usage/{UsagePage,WindowTabs,SummaryTiles,WindowsTable,
  BreakdownPanel}.tsx`, wired as a new `/usage` route + nav link inside
  the existing `AppLayout`/`AuthGate`. Styling is a new scoped stylesheet
  (`web/src/styles/usage-console.css`, ported class-for-class from the
  fetched mockup HTML/CSS) plus new Sora/IBM Plex Sans/Mono Google Fonts
  links — deliberately not touching `globals.css`'s existing blue theme,
  since the console is its own visual identity per ticket 3.

**Path-shortening implementation note:** it's client-side (`pathDisplay.ts`),
not server-side — the contract's own collision rule is scoped to "the
rendered set, not global" (e.g. one panel's top 8 for one window), and
only the SPA knows what's actually on screen. The literal-identical-path
machine-fallback branch (contract point 4) is implemented and fully unit
tested, but is **not reachable through today's live wiring**: `Aggregate`
keys its breakdown map purely on the raw path string, so two machines
reporting the same literal path always collapse into one row before the
client ever sees a duplicate — exactly what the contract itself says is
the current (acknowledged-wrong, deferred-to-fleet-rollout) behavior. Both
review agents independently confirmed this reading during
`/chief-review-code`.

**Real-data check:** tickets 12/13's own real database run didn't
persist between sessions (ticket 13's report notes its own demo binary
was "built, run, and deleted" without leaving a DB on disk, and no
`my-token` `.db` file exists anywhere on this machine). So I ran the
collector fresh, against the exact same real transcript directory ticket
12/13 used (`/home/thw-home/.claude/projects/-home-thw-home--typ-crews-freya`),
into a scratch database, started the real server, and curled the real
`/api/bff/usage/*` endpoints with a signed session cookie (minted via a
throwaway `cmd/demoticket14` program — same pattern ticket 13's
`cmd/demoticket13`, built, run, deleted, never committed). Actual
responses observed, not fabricated:

- Collector run: 14 transcript files scanned, 2025 deduped rows found and
  inserted, local estimate ~$288.06 (matches the server's own authoritative
  figure below almost exactly, as expected — same pricing table on both
  sides).
- `GET /usage/summary?window=lifetime&group_by=actor`: `totals` =
  {tokens: 634,916,052, cost: $288.0596, turns: 2025}, single breakdown
  row `freya`, `reporting_installs: 1`.
- `GET /usage/summary?window=today&group_by=path`: single breakdown row,
  key `/home/thw-home/gits/my-token`, tokens 131,744,793, cost $47.82,
  turns 600 — matches this ticket's own build session's real path
  resolution.
- `GET /usage/summary?window=24h&group_by=machine`: single row keyed by
  the collector's real `install_id` (a fresh UUID minted for this demo
  run), tokens 187,096,196, cost $69.60, turns 870.
- `GET /usage/windows`: 5h and today are identical (600 turns/$47.82 —
  everything happened within the last 5 hours), 24h is larger (870
  turns/$69.60), week and month are identical to each other and larger
  still (892 turns/$70.29) — internally consistent with all of this
  session's real activity having happened within the current UTC day/
  week/month.
- Confirmed `GET /usage` (the SPA route) returns 200 with the real
  `index.html` shell through the same running server — the embedded
  build/serve path works end to end. I did not drive an actual browser
  against it (no browser-automation tool was in scope for this run), so
  rendered-page fidelity to the mockup is verified by the unit/integration
  test suite plus manual code comparison against the fetched mockup HTML,
  not by a screenshot.

Demo artifacts (scratch DB, collector config, the throwaway
`cmd/demoticket14` program) were all deleted after the check — nothing
from this demo is in the commit.

**Tests:** `internal/domain/usage/summary_test.go` (new) — pure unit
tests for `ParseWindow`/`ParseGroupBy`, `WindowBounds` for all six window
values including calendar-boundary edge cases (Monday-start-of-week,
Sunday-six-days-back, non-UTC input normalization), and `Aggregate` for
all three `group_by` dimensions, tie-breaking, and window-scoped
`reporting_installs` (including its independence from `group_by` and its
exclusion of machines outside the window). `service_test.go` (extended)
— `Service.Summary`/`Service.Windows` wiring, via a fake repo that
records exactly what range each call asked for. `repo_test.go` (extended)
— a real-SQLite half-open-range test and a full-column round-trip test.
`internal/transport/bff/usage_handler_test.go` (new) — HTTP integration
tests seeding real `usage_events` rows through the real middleware chain,
covering both endpoints' grouping/totals/window-exclusion/
reporting_installs/auth/validation. `web/src/lib/pathDisplay.test.ts` (new,
14 cases) — every rule in the contract's "Console display rules" section,
including the cross-machine machine-fallback case. `web/src/lib/usage.test.tsx`
(new) — the query hooks' URL/query-string construction. All Go tests
(`go build`/`go vet`/`go test ./...`, including the repo's own
architecture/invariants guardrails) and all frontend tests
(`npx tsc -b --noEmit`, `npx vitest run` — 44 tests across 9 files,
including the 3 pre-existing suites this ticket didn't touch) pass.
`npm run build` (the real `tsc -b && vite build` production build) and a
full `go build ./...` with the resulting `web/dist` embedded both succeed.

`/chief-review-code` ran two parallel review agents before commit.
**Standards axis:** no hard rule violations against
`.chief/_rules/_standard/ARCHITECTURE.md`'s five dependency rules; found
one real doc-drift bug (`cmd/server/main.go`'s `wirePublicAPI` doc comment
still said "usageSvc has no bff-surface counterpart" after this diff added
exactly that) — fixed before commit. Three cosmetic judgement calls: two
were fixed (an unnecessary type-cast-plus-fallback in `WindowsTable.tsx`
that turned out to be dead defensive code once I checked the generated
TS type is already a literal union, and merging two duplicate type-only
imports); one (reusing the `queries` array for the loading-state check
instead of naming each query) I left as-is and noted why in a code
comment — collapsing it into an array-based check would lose TypeScript's
per-variable null-narrowing for the JSX that reads each query's `.data`
directly further down, which is worse than the minor repetition.
**Spec axis:** no missing requirements, no scope creep; independently
hand-traced the path-shortening algorithm and the week-window boundary
math and confirmed both match the contract; confirmed the `lifetime`
window addition is genuinely additive (the five-window table is
untouched) and the hostname omission is real and documented.

Committed: `233faa1`.
