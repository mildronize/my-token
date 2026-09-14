# 10: Backend filter support (machine, scan-root)

Type: implementation
Status: resolved
Blocked by: 7, 8

## Delivers

Per the contract's `/api/bff` section:

- **`bff-openapi.yaml`**: `GET /api/bff/usage/summary` and `GET
  /api/bff/usage/windows` both gain two new **optional** query params:
  `machine` (raw `install_id`) and `scan_root` (composite
  `<install_id>:<scan_root_path>` string — same machine-prefix convention
  ticket 7 established for `path`, for the identical reason: a bare
  scan-root path/name is ambiguous across machines since naming is
  per-install, per contract.md's Data model).
- **`internal/domain/usage/service.go`**: `Summary`/`Windows` (or a shared
  `Filter{Machine, ScanRoot string}` struct both take) pass the filter down
  to the repository query — `ListEventsInWindow` (or its replacement) grows
  optional `machine`/`scan_root` constraints, AND-composed with the existing
  window-bounds `WHERE created_at BETWEEN ...` clause.
- **`internal/transport/bff/usage_handler.go`**: parses the two new query
  params (both optional, absent = no filter), validates `scan_root`'s
  `install_id:scan_root_path` shape, passes through to the service layer.
- When either filter is set, **the whole result is scoped** —
  `totals`/`breakdown`/`reporting_installs` on `/summary`, every row of the
  fixed table on `/windows` — not just one panel's own dimension (contract's
  Console filters section: global, not per-panel).

## Verifiable

- Unit test: `Aggregate`/repo-query-building logic applies both filters
  correctly when both are set together (AND, not OR) — table-driven,
  constructed `Event` slices, no database needed for the pure half.
- HTTP integration test: seed `usage_events` across ≥2 machines and ≥2
  scan-roots, assert `machine=<id>` alone narrows correctly, `scan_root=
  <id>:<path>` alone narrows correctly, and both set together narrow to the
  intersection, on both `GET /usage/summary` and `GET /usage/windows`.
- HTTP integration test: an invalid `scan_root` value (wrong shape, or a
  well-formed but non-existent `install_id:scan_root_path` pair) returns an
  empty/zero result, not an error — mirrors this API's existing tolerance
  for "no matching data" versus "malformed request" (confirm the exact
  400-vs-empty-result line against this repo's own error-envelope
  convention during `/chief-build`, not guessed here).
- `go test ./...` green.
