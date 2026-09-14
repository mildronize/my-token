# Ticket 20 Report

## Ticket

Add `year` as a new calendar-anchored `Window` value (Jan 1 00:00 UTC of
the current year → now), and make both `year` and the already-existing
`lifetime` value into real, selectable console tabs — reopening ticket
14's "lifetime is tab-only-tile, never a tab" call. `GET /usage/windows`'
fixed table grows from 5 to 7 rows: 5h/24h/today/week/month/year/lifetime.

## Outcome

done

## Notes

**What was built** (`/home/thw-home/gits/my-token`, branch
`feature/year-lifetime-tabs`, commit `85b456a`):

- `internal/domain/usage/summary.go`: added `WindowYear` — same
  `time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)` calendar-anchoring
  style as `today`/`week`/`month`. `Windows` now lists all seven fixed
  windows in the contract's stated order
  (`5h, 24h, today, week, month, year, lifetime`). Collapsed the old
  `Windows`/`parseableWindows` split from ticket 14 — since every
  parseable window is now a real tab, `ParseWindow` validates against
  `Windows` directly; there's no longer a wider "parseable but not
  tabbed" set to keep separate.
- `bff-openapi.yaml`: `/usage/summary`'s `window` enum widened to include
  `year`; `/usage/windows`'s `UsageWindowRow.window` enum widened to
  include `year` and `lifetime` (previously just the original five).
  Doc comments updated to reflect ticket 14's exclusion being reopened.
- Regenerated `internal/bffapi/bffapi.gen.go` (`oapi-codegen`) and
  `web/src/lib/api/bff-schema.gen.ts` (`openapi-typescript`) from the
  updated spec — both are pure codegen output, no hand-edits.
- `internal/domain/usage/service.go`, `internal/transport/bff/usage_handler.go`:
  doc-comment updates only (both already looped over `Windows`/rendered
  whatever rows came back, so no logic changes were needed there —
  `Service.Windows`'s `len(Windows)`-driven loop and
  `GetUsageWindows`'s pass-through both scale to 7 rows for free).
- Frontend: `web/src/lib/usage.ts`'s `UsageWindow` type and
  `FIXED_WINDOWS` grow to all seven windows.
  `web/src/app/usage/WindowTabs.tsx`: removed the doc comment/reasoning
  that deliberately excluded `lifetime` from the tab set, added `year`
  to `WINDOW_LABELS`. `WindowsTable.tsx`/`UsagePage.tsx`: doc-comment
  updates only — both already render whatever `FIXED_WINDOWS`/the API
  response contains, so no rendering logic changed.

**Tests added:**
- `internal/domain/usage/summary_test.go`: `TestParseWindow_ValidValues`
  extended with `"year"`; four new `WindowBounds` tests for `year`
  mirroring the existing week/month rigor —
  `TestWindowBounds_Year_StartsAtJan1UTCOfTheCurrentYear`,
  `TestWindowBounds_Year_OnJan1stItselfStartsThatSameInstant` (the
  Jan-1st edge case, mirroring the week test's Monday-itself case),
  `TestWindowBounds_Year_OnDec31stStillAnchorsToJan1stOfTheSameYear`
  (the widest-offset case, mirroring the week test's Sunday case), and
  `TestWindowBounds_Year_NonUTCNowNearYearBoundaryIsNormalizedToUTCFirst`
  (a UTC+7 instant that's a different calendar *year* in UTC — asserts
  the start is `2025-01-01`, not `2026-01-01`, if local-time leaked in).
  Replaced the now-obsolete `TestWindows_DoesNotIncludeLifetime` with
  `TestWindows_ListsAllSevenFixedWindowsInOrder`, asserting the exact
  slice contents and order.
- `internal/transport/bff/usage_handler_test.go`: added
  `TestGetUsageWindows_ReturnsSevenRowsInOrder` (this ticket's own
  explicitly-required acceptance test — exactly 7 rows, exact stated
  order over the wire). Updated the two pre-existing tests that
  hardcoded `require.Len(t, got.Windows, 5)` to `7`.
- `web/src/app/usage/UsagePage.test.tsx` (new — no prior
  WindowTabs/UsagePage test existed to otherwise mirror; followed this
  repo's own established fetch-mock + `userEvent.click` + assert-the-
  resulting-request pattern, `ApiKeySettings.test.tsx`): two tests,
  clicking "Year" and clicking "Lifetime". Each asserts the click causes
  a genuinely new `/api/bff/usage/summary?window=<w>&group_by=<dim>`
  fetch for panels that were never fetched with that window before —
  for Year, all three (actor/path/machine, since nothing on the page
  ever fetches `window=year` on initial load); for Lifetime, machine
  specifically (actor/path already share a cache key with the page's
  own always-fetched `lifetimeByActor`/`lifetimeByPath` summary-tile
  queries, so only the machine panel's fetch can prove the *click*
  — not a coincidentally-shared cache key — re-scoped it). This
  distinction is deliberate: a weaker assertion (any fetch to
  `window=lifetime`) would have passed even if clicking the tab did
  nothing, since the lifetime tiles fetch that window unconditionally.
  Also updated `web/src/lib/usage.test.tsx`'s windows-hook test to a
  7-row fixture and added a `window=year` pass-through case, for
  consistency with the widened real shape (not required by the ticket,
  done for accuracy since the old 5-row fixture would otherwise misstate
  what the real endpoint now returns).

**Verification actually run:**
- `go build ./...` — clean.
- `go vet $(go list ./... | grep -v /node_modules/)` — clean.
- `go test $(go list ./... | grep -v /node_modules/)` — every package
  green, including `internal` (architecture/invariants guardrails) and
  the newly extended `internal/domain/usage`/`internal/transport/bff`
  suites.
- `gofmt -l` on every Go file this ticket touched — clean (one
  pre-existing, untouched-by-this-diff drift on
  `internal/collector/path_test.go` was left alone, confirmed via
  `git status`/`git diff --stat` to be unrelated to this change).
- `cd web && npx tsc -b --noEmit` — clean.
- `cd web && npx vitest run` — 6 files, 26 tests, all green (up from 5
  files/23 tests before this ticket).
- `/chief-review-code` ran two parallel review agents before commit.
  **Standards axis:** zero hard violations against
  `.chief/_rules/_standard/ARCHITECTURE.md`'s five dependency rules
  (confirmed no domain module imports gin, transport imports domain in
  the correct direction, generated files are pure regenerations with no
  hand-edits smuggled in). One judgement-call note (the seven-window set
  is independently spelled out across yaml/Go/TS/React — a pre-existing
  shape widened, not a new smell introduced by this diff) left as-is,
  and one positive note: this diff actually *removes* a prior
  Speculative-Generality-shaped split (`parseableWindows`) now that
  every parseable window is a real tab. **Spec axis:** zero
  missing/partial requirements, zero scope creep, zero implementation
  bugs — independently re-verified the year calendar math (including the
  Dec 31/Jan 1 and UTC+7 cross-year-boundary cases) and confirmed the
  7-row order matches end to end across the yaml enum, the `Windows`
  slice, both new tests, and the frontend fixture.

No design ambiguity came up during this ticket — the calendar-anchoring
style, row order, and tab mechanism were all fully specified by the
ticket/contract and the existing today/week/month precedent in
`summary.go`, so nothing here needed a judgement call flagged for review.
