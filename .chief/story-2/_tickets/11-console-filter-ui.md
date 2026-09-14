# 11: Console filter UI

Type: implementation
Status: resolved
Blocked by: 9, 10

## Delivers

Per the contract's "Console filters" section:

- **`web/src/app/usage/UsagePage.tsx`**: two new filter controls (machine,
  scan-root) alongside the existing window tabs. Machine filter's options
  come from the existing `group_by=machine` breakdown response
  (`key`=hostname, `raw_key`=install_id — no new endpoint needed).
  Scan-root filter's options come from ticket 9's new `GET
  /api/bff/usage/scan-roots`. Selecting either re-fetches every panel's
  query (all six: lifetime/window × actor/path/machine, plus the windows
  table) with the new param attached — global scoping, not per-panel, per
  the contract.
- **localStorage persistence**: selected `machine`/`scan_root` filter
  values persist across reloads (a small hook or utility, not a new
  dependency). The window tab is **not** persisted — stays reset to
  `DEFAULT_WINDOW = "today"` on every load, unchanged from story-1. An
  invalid/stale stored value (e.g. a machine that stopped reporting) is
  dropped silently on restore — falls back to "no filter," same as a
  first-ever visit, never surfaced as an error.
- **`web/src/lib/usage.ts`**: the query-building layer (`useUsageSummaryQuery`
  and friends) grows optional `machine`/`scanRoot` params, included in each
  query's cache key so a filter change correctly triggers a refetch rather
  than serving stale cached data for the previous filter state.

## Verifiable

- Frontend test: selecting a machine filter narrows all three breakdown
  panels' rendered rows (not just "By machine") — asserted against mocked
  query responses.
- Frontend test: a selected filter survives a remount (localStorage
  round-trip) — exact test tooling/commands to confirm against this repo's
  existing frontend test setup (`vitest`, per `web/package.json`) during
  `/chief-build`.
- Frontend test: a stored filter value referencing a machine/scan-root not
  present in the current data falls back to "no filter" without an error
  state rendering.
- `make test` (both `go test ./...` and the Vitest suite) green.
