# 1: GET /api/bff/machines — machine summaries endpoint

Type: implementation
Status: resolved
Blocked by: None

## Delivers

Per the contract's Data model and API surface sections, built on top of
`story-2/integration` (this story's build target until story-2's own
branch reaches `main` — see contract.md's Build target note):

- **`db/queries/machines.sql`**: new `ListMachineSummaries :many` query —
  left-joins `machines` to `usage_events` on `machine`, groups by
  `install_id`/`hostname`/`last_seen_at`, aggregates `lifetime_cost`
  (`SUM(cost)`), `lifetime_tokens` (`SUM` of all four token columns),
  `collected_paths` (`COUNT(DISTINCT path)`), `collected_actors`
  (`COUNT(DISTINCT actor)`), ordered `last_seen_at DESC`. Exact SQL is in
  contract.md — copy it verbatim, plain ASCII per this repo's sqlc
  constraint.
- **`bff-openapi.yaml`**: new `GET /api/bff/machines` operation, no
  parameters, SSO-session-auth'd like every other `/api/bff` route.
  Response wrapped `{"machines": [...]}`, matching this file's own
  established top-level-array convention (`UsageWindows`,
  `UsageScanRootList`, etc.).
- **`internal/domain/*/repo.go`/`service.go`**: repo method wrapping the
  new sqlc query, service method assembling the response.
- **`internal/transport/bff/*_handler.go`**: new handler wiring the above
  to the OpenAPI-generated response type.

## Verifiable

- Query-level test: seed `usage_events` across >=2 machines with distinct
  paths/actors/costs, assert correct per-machine `SUM`/`COUNT(DISTINCT
  ...)` and that rows come back sorted `last_seen_at` descending.
- Zero-events edge case test: a `machines` row with no matching
  `usage_events` rows returns zeroed numeric fields, not a null/error
  (defensive per contract.md — not currently reachable via the collector,
  but the `LEFT JOIN` must hold if that ever changes).
- HTTP integration test: seed real rows, assert `GET /api/bff/machines`'s
  wire shape and sort order match the contract.
- `go test ./...` green.
