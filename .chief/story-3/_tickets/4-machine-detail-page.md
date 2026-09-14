# 4: Machine detail page + usage-dashboard handoff

Type: implementation
Status: resolved
Blocked by: 1, 2

## Delivers

Per the contract's Frontend section, built on `story-2/integration`:

- **`web/src/App.tsx`**: new route `/machines/:installId` ->
  `MachineDetailPage`.
- **`MachineDetailPage`** component:
  - Identity/stats block: same fields as the overview row (hostname,
    install_id, last reported, lifetime cost/tokens, collected
    paths/actors) — found by `installId` route param from ticket 1's
    machines list response (no second single-machine endpoint, per
    contract).
  - Scan roots section, properly laid out (not a table cell): each root's
    name, registered path, and source type — from ticket 2's extended
    `GET /api/bff/usage/scan-roots`, client-filtered to this page's
    `installId`.
  - "View in Usage dashboard" button: writes this machine's `install_id`
    into `useUsageFilters.ts`'s machine-filter storage key, then
    `navigate("/usage")`.
- **`web/src/hooks/useUsageFilters.ts`**: export `MACHINE_STORAGE_KEY` (or
  a small wrapping helper, e.g. `presetMachineFilter(installId: string):
  void`, matching `writeStored`'s existing try/catch-swallow convention)
  so `MachineDetailPage` — which doesn't itself mount the hook, since it
  lives inside `UsagePage`'s own tree — can write the stored value
  directly. `UsagePage`'s own lazy-initializer picks it up on mount
  exactly as it already does for a reload; no new mechanism.
- Column labels are final per contract.md — "Last reported", "Collected
  Paths", "Collected Actors" — don't re-litigate naming in this ticket.

## Verifiable

- `MachineDetailPage` component test: renders the identity/stats block
  (found by `installId` from a mocked machines list) and the scan-roots
  section (from a mocked, client-filtered scan-roots response including
  `source_type`).
- Cross-page contract test (this is silent glue — no shared component,
  just a shared `localStorage` key convention, worth a direct test):
  clicking "View in Usage dashboard" on the detail page, then rendering
  `UsagePage` fresh, asserts `UsagePage`'s very first fetch already
  carries `machine=<that install_id>` — mirroring the existing "a
  selected machine filter survives a remount" test's own pattern
  (`UsagePage.test.tsx`), triggered from the other page instead of the
  dropdown.
- `npm run build` and existing frontend test suite green.
