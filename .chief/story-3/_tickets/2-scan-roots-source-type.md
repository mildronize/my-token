# 2: GET /api/bff/usage/scan-roots — expose source_type

Type: implementation
Status: resolved
Blocked by: None

## Delivers

Per the contract's API surface section, built on `story-2/integration`:

- **`bff-openapi.yaml`**: `UsageScanRoot` schema gains `source_type`
  (string, required) — already a real column on `scan_roots`
  (story-2/ticket-8), just never exposed since story-2/ticket-9 only
  needed the endpoint for the filter dropdown, which doesn't display it.
  Additive only: no existing consumer (the scan-root filter dropdown)
  needs to change.
- **`internal/domain/usage/repo.go`/`service.go`**: extend the existing
  scan-roots read path to also select/pass through `source_type`.
- **`internal/transport/bff/usage_handler.go`**: include the field in the
  response mapping.

This is deliberately additive to story-2/ticket-9's own endpoint, not a
new one — story-3's machine detail page (ticket 4) reuses it, client-side
filtered to one `install_id`, same "reuse the list" reasoning the
contract lays out for the `/machines` endpoint itself.

## Verifiable

- Extend story-2/ticket-9's existing scan-roots tests (unit + HTTP
  integration) to also assert `source_type` is present and correct — no
  existing assertion should need to change behavior, only gain one.
- `go test ./...` green.
