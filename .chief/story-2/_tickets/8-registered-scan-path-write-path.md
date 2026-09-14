# 8: Registered scan-path naming — write path

Type: implementation
Status: resolved
Blocked by: None (can start immediately)

## Delivers

Per the contract's "Collector config" (Supersedes story-1's
`scan_paths: string[]`) and Data model/API surface sections:

- **`internal/collector/config.go`**: `Config.ScanPaths` changes from
  `[]string` to a new `[]ScanPath{Name, Path, SourceType}` type.
  `SourceType` defaults to `"claude_code"` when the config JSON omits it
  (mirrors `usage_events.source`'s fixed value elsewhere, but this field is
  real and settable per the contract). Config load/persist logic
  (`LoadOrInitConfig`/`writeConfig`) updated for the new shape; existing
  dev config files with the old bare-string `scan_paths` are not migrated
  (development data, per goal.md's Out of Scope) — a config using the old
  shape is expected to error clearly on load, not silently misparse.
- **`internal/collector/scan.go`**: `FindTranscriptFiles` (or its caller)
  tracks which `ScanPath` entry produced each discovered transcript file —
  the collector already walks each configured root separately, so this is
  wiring, not new discovery logic. Each resulting `Event` gets a new
  `ScanRoot` field (the entry's resolved `Path`) and carries its `Name`/
  `SourceType` through to the batch payload's `scan_roots` array.
- **Two new migrations** (`db/migrations/`): `usage_events.scan_root TEXT
  NOT NULL` (new column on the existing table); new `scan_roots` table
  (`install_id`, `scan_root_path` composite PK, `name`, `source_type`
  defaulted `'claude_code'`, `last_seen_at`) — schema exactly as contract.md
  specifies, same `INSERT OR REPLACE` upsert idiom `machines` already uses
  (see story-1 ticket 18's report for why: sidesteps this repo's own
  `dbquery` table-isolation scanner's `ON CONFLICT ... DO UPDATE SET`
  blind spot).
- **`internal/transport/publicapi/usage_handler.go`**: `POST
  /api/v1/usage-events/batch` handler upserts every entry in the request's
  new `scan_roots: [{path, name, source_type}]` array into `scan_roots`
  (once per batch, mirroring the existing `hostname`→`machines` upsert),
  and persists each event's new `scan_root` field onto its `usage_events`
  row.
- **`bff-openapi.yaml`**: request body schema for the ingestion endpoint
  gains `scan_roots` (array) and each event gains `scan_root` (string),
  matching the contract's API surface section exactly.

## Verifiable

- Unit test: collector config load rejects/errors clearly on the old
  bare-string `scan_paths` shape (no silent misparse).
- Unit test: a fixture scan across two configured `ScanPath` entries
  correctly attributes each found transcript's events to the right
  `ScanRoot` value.
- Unit test (`scan_roots` upsert, mirroring `machines`' own existing test):
  a second batch with a renamed scan root updates the stored `name`, not
  just the first-seen value.
- HTTP integration test: `POST /api/v1/usage-events/batch` with a
  `scan_roots` array and events carrying `scan_root` persists both the new
  `usage_events.scan_root` column and the `scan_roots` table row correctly.
- `go test ./...` green.
