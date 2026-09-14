# 3: Machines overview page

Type: implementation
Status: resolved
Blocked by: 1

## Delivers

Per the contract's Frontend section, built on `story-2/integration`:

- **`web/src/App.tsx`**: new route `/machines` -> `MachinesPage`, added to
  the existing `<Routes>` block exactly as `/usage`/`/settings` are
  already declared.
- **`web/src/components/Header.tsx`**: `NAV_LINKS` gains `{ href:
  "/machines", label: "Machines" }`.
- **`MachinesPage`** component:
  - Summary strip: total machine count + combined lifetime cost across
    all of them, same "at a glance" framing as `/usage`'s own top-bar
    pill.
  - One table, one row per machine, sorted by "Last reported" descending
    (server already returns this order — no client re-sort needed):
    Hostname (install_id via tooltip), Last reported, Lifetime cost,
    Lifetime tokens, Collected Paths, Collected Actors.
  - Row click navigates to `/machines/:installId` (react-router
    `useNavigate` or a `Link` wrapping the row).
  - Data source: ticket 1's `GET /api/bff/machines`.

## Verifiable

- `MachinesPage` component test: renders the summary strip and table from
  a mocked `GET /api/bff/machines` response; clicking a row navigates to
  that row's `/machines/:installId`.
- `npm run build` and existing frontend test suite green.
