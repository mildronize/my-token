# 18: Persist and display machine hostname instead of raw UUID

Type: implementation
Status: resolved
Blocked by: None (can start immediately)

## Delivers

Per the contract's `machines` table and "machine label" display rule:

- New migration: `machines(install_id TEXT PRIMARY KEY, hostname TEXT,
  last_seen_at TIMESTAMP)`.
- `POST /api/v1/usage-events/batch`'s handler: upsert into `machines`
  (`install_id`, `hostname`, `last_seen_at`) on every batch, instead of
  reading and discarding `hostname` as it does today. Keep
  `usage_events.machine` unchanged — still the `install_id`, still the
  real join key.
- `GET /api/bff/usage/summary?group_by=machine`: join `usage_events.machine`
  → `machines.install_id`, return `hostname` as the breakdown `key`
  instead of the raw `install_id`. Fall back to the raw `install_id` if a
  machine has events but somehow no `machines` row (shouldn't happen given
  upsert-on-every-batch, but don't show a blank label if it does).
- Console: "By machine" panel shows the hostname; raw `install_id`
  available via tooltip (same shortened-label-plus-tooltip pattern as
  `path`).

Verifiable: unit test for the upsert (a second batch with a changed
hostname updates the stored value, not just the first-seen one); HTTP
integration test seeding `usage_events` + `machines` rows, confirming
`group_by=machine` returns hostnames not UUIDs; a quick real run (the
collector already reports `hostname` in its payload per ticket 12 — this
ticket's server-side change should pick it up with no collector changes
needed) confirming the console shows a real hostname instead of the raw
`install_id` string currently visible.
