# 11: Core service — usage_events schema + ingestion endpoint

Type: implementation
Status: resolved
Blocked by: None (can start immediately)

## Delivers

Fork my-template's current `main` (Go/Gin, oapi-codegen — not the unmerged
huma PR #11, per ticket 4). Add:

- `usage_events` migration (goose), matching the contract's field list:
  `id` (PK = `message.id`), `session_id`, `actor`, `path`, `machine`,
  `model`, `input_tokens`, `output_tokens`, `cache_read_input_tokens`,
  `cache_creation_input_tokens`, `cost`, `source`, `created_at`.
- Repository + Service layer over it, shaped like `internal/domain/todo`
  (ticket 4).
- `POST /api/v1/usage-events/batch` on the existing `/api/v1` surface
  (Bearer API-key). Idempotent insert on `id` — a resent batch changes
  nothing. Server computes `cost` (ported pricing table, see ticket 13) and
  sets `source = "claude_code"` itself; never trusts the client for those
  two fields.

Demoable/verifiable on its own: POST a batch of synthetic events (including
a deliberately repeated `id`) directly via curl/integration test, confirm
exactly one row lands per unique `id`, `cost` and `source` are server-computed
not client-supplied.

Out of scope for this ticket: the collector that produces real events (12),
the console that reads them (14).
