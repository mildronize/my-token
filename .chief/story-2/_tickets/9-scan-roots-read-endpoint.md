# 9: GET /api/bff/usage/scan-roots — read path

Type: implementation
Status: resolved
Blocked by: 8

## Delivers

Per the contract's API surface section:

- **`bff-openapi.yaml`**: new `GET /api/bff/usage/scan-roots` operation, no
  parameters, SSO-session-auth'd like every other `/api/bff` route. Response
  schema: array of `{install_id, hostname, scan_root_path, name}` — joins
  `scan_roots` to `machines` on `install_id` for the hostname, same join
  style `Service.Summary`'s existing hostname substitution already uses
  (read via a repo method, not a SQL `JOIN`, per that same precedent).
- **`internal/domain/usage/repo.go`/`service.go`**: new `Repository` method
  to list every `scan_roots` row joined with its machine's hostname; new
  `Service.ScanRoots(ctx)` wrapping it.
- **`internal/transport/bff/usage_handler.go`**: new handler wiring the
  above to the OpenAPI-generated response type.

This endpoint exists purely to populate the console's future scan-root
filter dropdown (ticket 11) — there is no `group_by=scan_root` breakdown
panel in this story's scope to derive the list from otherwise.

## Verifiable

- Unit test: `Service.ScanRoots` returns every `scan_roots` row with its
  joined hostname; a `scan_roots` row whose `install_id` has no `machines`
  row falls back to showing no hostname (or the raw `install_id`, matching
  the same "never a blank label" precedent `machines`' own fallback sets) —
  confirm against contract.md if this exact fallback shape isn't already
  obvious from ticket 8's schema.
- HTTP integration test: seed `machines` + `scan_roots` rows across two
  install_ids, assert `GET /api/bff/usage/scan-roots` returns all of them
  with correct hostname joins.
- `go test ./...` green.
