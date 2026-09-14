# Contract

**Build target note (not a schema/API concern, but critical):** this
story depends on story-2's filter mechanism (`useUsageFilters.ts`, the
`machine`/`scan_root` query params, `GET /api/bff/usage/scan-roots`) —
verified these exist on `story-2/integration` but **not yet on `main`**
(tickets 9-11 are still awaiting review). Build this story on top of
`story-2/integration`, the same way tickets 9-17 did, not on `main`,
until story-2's remaining PRs land.

## Data model

No new tables, no migrations. Everything this story needs already
exists (`machines`, `scan_roots`, `usage_events`) — this is a read-only
story over data story-2 already collects.

**New query** (`db/queries/machines.sql`, `ListMachineSummaries :many`,
plain ASCII per this repo's own sqlc constraint):

```sql
SELECT
  m.install_id,
  m.hostname,
  m.last_seen_at,
  COALESCE(SUM(u.cost), 0) AS lifetime_cost,
  COALESCE(SUM(u.input_tokens + u.output_tokens + u.cache_read_input_tokens + u.cache_creation_input_tokens), 0) AS lifetime_tokens,
  COUNT(DISTINCT u.path) AS collected_paths,
  COUNT(DISTINCT u.actor) AS collected_actors
FROM machines m
LEFT JOIN usage_events u ON u.machine = m.install_id
GROUP BY m.install_id, m.hostname, m.last_seen_at
ORDER BY m.last_seen_at DESC;
```

One query, one round trip, already sorted the way the page needs it
(`last_seen_at DESC` — "Last reported" descending, per the goal). `LEFT
JOIN`/`COALESCE` are defensive, not load-bearing: today a `machines` row
is only ever created alongside at least one `usage_events` row (the
collector never contacts the server when it has zero new events —
`internal/collector/collector.go`'s own `if len(events) == 0 { return
result, nil }` early-return, before `PostBatch` is ever called) — but a
`LEFT JOIN` means this query still returns a sane zeroed row rather than
silently omitting a machine, or erroring, if that ever stops being true.

`last_seen_at` is `machines.last_seen_at` verbatim — stamped by the
server's own clock at ingestion time (story-1 ticket 18), **not**
`MAX(usage_events.created_at)`. This is a deliberate distinction settled
in conversation: "Last reported" answers "is this machine's collector
still alive," which only the ingestion timestamp answers — the
most-recent-event timestamp would instead answer "when did a person last
actually use it," a different question this story isn't asking.

## API surface

**New**: `GET /api/bff/machines` — SSO-session-auth'd like every other
`/api/bff` route, no parameters.

```
GET /api/bff/machines
->
{
  "machines": [
    {
      "install_id": "...",
      "hostname": "...",
      "last_seen_at": "2026-09-13T13:31:00Z",
      "lifetime_cost": 4430.30,
      "lifetime_tokens": 89200000,
      "collected_paths": 340,
      "collected_actors": 12
    },
    ...
  ]
}
```

Array is pre-sorted `last_seen_at` descending (matches
`ListMachineSummaries`'s own `ORDER BY` — no client-side re-sort needed).
Wrapped as `{"machines": [...]}`, matching this same OpenAPI file's own
established convention for a top-level array (`UsageWindows`,
`UsageScanRootList`, `ApiKeyList`, `UserList` all wrap the same way).

This single endpoint backs **both** the overview table (point 3) and the
detail page's own identity/stats block (point 5) — the detail page finds
its one row by `install_id` from the same response rather than a second,
single-machine endpoint. Reasonable at this fleet's current and
foreseeable scale (a handful of machines); would need revisiting if the
fleet ever grew into the hundreds.

**Extended**: `GET /api/bff/usage/scan-roots` (story-2/ticket-9) gains one
field on `UsageScanRoot` — `source_type` (already a real column in
`scan_roots`, just never exposed by this endpoint since ticket 9 only
needed it for the filter dropdown, which doesn't display it). Additive,
`required` list grows to include it; no existing consumer (the scan-root
filter dropdown) needs to change. The detail page's own scan-roots
section reuses this same endpoint, filtered client-side to the one
`install_id` the page is showing — same "reuse the list, filter
client-side" reasoning as the machines endpoint above, same fleet-scale
caveat.

## Frontend

**Routing** (`web/src/App.tsx`, extending the existing `<Routes>` block
exactly as `/usage`/`/settings` are already declared):
- `/machines` -> `MachinesPage` (the overview table)
- `/machines/:installId` -> `MachineDetailPage`

**Nav**: `Header.tsx`'s `NAV_LINKS` gains `{ href: "/machines", label:
"Machines" }`.

**The "View in Usage dashboard" button's mechanism** (goal point 5):
`useUsageFilters.ts` exports its `MACHINE_STORAGE_KEY` constant (or a
small wrapping helper, e.g. `presetMachineFilter(installId: string):
void`, same try/catch-swallow convention `writeStored` already uses) so
`MachineDetailPage` — which does not itself mount `useUsageFilters`, since
that hook lives inside `UsagePage`'s own tree — can write the machine
filter's stored value directly, then `navigate("/usage")` (react-router's
`useNavigate`). When `UsagePage` mounts, its own `useUsageFilters()`
lazy-initializer reads that value back exactly as it already does for a
reload — no new mechanism, just a new caller of the existing one.

**Column labels, final** (settled across several rounds — recorded here
so a fresh `/chief-build` session doesn't re-litigate them): "Last
reported" (not "Last seen" — see Data model above), "Collected Paths" and
"Collected Actors" (not "Distinct"/"Unique" — reads as programmer jargon
for this audience — and not "Detected" — implies an inference step that
isn't happening; both are plain counts, named consistently with each
other around the product's own "collector" vocabulary).

## Testing Decisions

- **`ListMachineSummaries` unit/integration test**: seed `usage_events`
  across >=2 machines with distinct paths/actors/costs, assert correct
  per-machine `SUM`/`COUNT(DISTINCT ...)` values and that rows come back
  sorted `last_seen_at` descending.
- **Zero-events edge case**: a `machines` row with no matching
  `usage_events` rows (defensive — see Data model's note that this
  shouldn't currently be reachable) returns zeroed numeric fields, not a
  null/error.
- **`GET /api/bff/machines` HTTP integration test**: seed real rows,
  assert the wire shape and sort order match the contract above.
- **`UsageScanRoot.source_type` addition**: existing scan-roots endpoint
  tests (story-2/ticket-9) extended to also assert the new field is
  present and correct; no existing test should need to change behavior,
  only add an assertion.
- **`MachinesPage` component test**: renders the summary strip and table
  from a mocked `GET /api/bff/machines` response; clicking a row
  navigates to that row's `/machines/:installId`.
- **`MachineDetailPage` component test**: renders the identity/stats
  block (found by `installId` route param from the same mocked machines
  list) and the scan-roots section (from a mocked, client-filtered
  scan-roots response, including `source_type`).
- **The button's cross-page contract, tested explicitly** (this is silent
  glue — no shared component, just a shared `localStorage` key
  convention, exactly the kind of thing worth a direct test rather than
  trusting it works): clicking "View in Usage dashboard" on the detail
  page, then rendering `UsagePage` fresh, asserts `UsagePage`'s very
  first fetch already carries `machine=<that install_id>` — mirroring the
  existing "a selected machine filter survives a remount" test's own
  pattern (`UsagePage.test.tsx`), just triggered from the other page
  instead of the dropdown.
