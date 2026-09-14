# 20: Add Year and Lifetime as selectable window tabs

Type: implementation
Status: resolved
Blocked by: None (can start immediately)

## Delivers

Per the grilled decision (reopening ticket 14's "lifetime is tab-only-tile,
never a tab" call): `year` and `lifetime` become real, selectable tabs,
same mechanism as the existing 5h/24h/today/week/month tabs — clicking one
re-scopes the actor/path/machine breakdown panels via
`/usage/summary?window=...`, exactly like the existing five.

- **Backend**: extend `Window`/`WindowBounds` (`internal/domain/usage/summary.go`)
  with a `year` case — calendar-anchored (Jan 1 00:00 UTC of the current
  year → now), matching `today`/`week`/`month`'s existing calendar-boundary
  style, not a rolling 365-day window. `lifetime` already exists as a
  `window` value (ticket 14) — no backend change needed there beyond
  making sure it's included wherever `year` is added.
- `GET /api/bff/usage/windows`'s fixed table grows from 5 to 7 rows:
  `5h`/`24h`/`today`/`week`/`month`/`year`/`lifetime`, in that order.
- **Frontend**: `FIXED_WINDOWS` (`web/src/lib/usage.ts`) and `WindowTabs.tsx`
  grow to include `year` and `lifetime` as real tabs (remove
  `WindowTabs.tsx`'s current comment/logic that deliberately excludes
  `lifetime` from the tab set — that exclusion is what's being reversed
  here). `WindowsTable.tsx` renders the two new rows.

Verifiable: unit tests for `year`'s `WindowBounds` (calendar-boundary edge
cases, same style as the existing week/month tests — e.g. Jan 1st itself,
a UTC-vs-local-timezone input near year boundary); `GET /usage/windows`
returns exactly 7 rows in the stated order; both new tabs are clickable
and correctly re-scope the breakdown panels (frontend test, same pattern
as the existing tab tests).
