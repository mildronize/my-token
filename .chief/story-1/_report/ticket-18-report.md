# Ticket 18 Report

## Ticket

Persist a `machines(install_id, hostname, last_seen_at)` table, upserted
on every `POST /api/v1/usage-events/batch`, and use it to show a real
hostname (not the raw `install_id` UUID) as the "By machine" breakdown's
label — raw `install_id` still reachable via tooltip.

## Outcome

done

## Decision

- **Issue: `ON CONFLICT(install_id) DO UPDATE SET ...` (the natural
  SQLite upsert form) trips a known blind spot in this repo's own
  `internal/dbquery` table-isolation scanner** — its keyword scanner
  reads `SET`'s target list after `DO UPDATE` as if it named a table
  called `set`, and its own doc comment explicitly says it deliberately
  will not be widened to handle more SQL forms ("an arms race against
  forms nobody has enumerated yet").
  **Chosen:** `INSERT OR REPLACE INTO machines (...) VALUES (...)`
  instead — sidesteps the scanner entirely (no `UPDATE`/`SET` keyword at
  all) and is semantically equivalent here since every column is
  supplied on every call (no partial-update case this query ever needs;
  no FK/trigger for REPLACE's delete+reinsert to disturb). Matches the
  existing `usage_events.sql`'s own `INSERT OR IGNORE` convention rather
  than introducing a new SQL idiom. Verified directly: the `ON CONFLICT`
  form was tried first and reproduced the exact "undeclared table `set`"
  failure before switching.

- **Issue: the contract's "machine label" rule requires "the raw
  `install_id` available via tooltip," but `UsageBreakdownRow`
  (`bff-openapi.yaml`) has `additionalProperties: false` and its only
  field, `key`, becomes the hostname once substituted — there is no
  schema-legal channel left to carry the raw value to the console.**
  **Chosen:** added a new optional `raw_key` field to `UsageBreakdownRow`
  (present only when `group_by=machine` actually substituted a hostname;
  absent for every other `group_by` and for the no-machines-row fallback
  case, where `key` is already the raw value). This is a small,
  necessary wire-schema addition, not an invented feature — flagging it
  here since it's the one place this ticket's work reached slightly
  beyond the ticket's own literal bullet list (which named "the console
  shows hostname, raw install_id via tooltip" as an outcome, not the
  specific wire mechanism). `/chief-review-code`'s Spec axis
  independently reviewed this and agreed it's contract-mandated, not
  scope creep.

## Notes

**What was built** (commit `1473082`, 20 files, +922/-102):

- `db/migrations/20260910140000_create_machines.sql` — new `machines`
  table (`install_id TEXT PRIMARY KEY`, `hostname TEXT NOT NULL`,
  `last_seen_at TIMESTAMP NOT NULL`; `NOT NULL` added on the latter two
  since the upsert always supplies both — a stricter-than-literal but
  non-contradicting reading of the ticket's own schema line).
- `db/queries/machines.sql` — `UpsertMachine` (`INSERT OR REPLACE`,
  see Decision above) and `ListMachines` (`install_id -> hostname`, read
  by `Service.Summary`'s hostname-substitution pass rather than a SQL
  `JOIN` — reuses `Aggregate`'s existing pure group_by logic instead of
  re-deriving it in SQL). Kept plain ASCII throughout per this repo's own
  `internal/db_queries_ascii_test.go` (a real, previously-hit sqlc v1.31.1
  corruption bug on any non-ASCII byte in `db/queries/*.sql` — I hit it
  myself on the first draft, which used em dashes, and caught it via that
  exact test before it ever reached a build).
- `internal/dbquery/tableisolation.go` — one new `TableOwnership` entry
  (`"machines": "usage"` — same domain module as `usage_events`, not a
  separate one).
- `internal/domain/usage/{repo,service,summary}.go` — `Repository`
  gained `UpsertMachine`/`MachineHostnames`; `Service.UpsertMachine`
  (thin pass-through, `now` supplied by the caller); `Service.Summary`
  gained a post-`Aggregate` substitution pass for `GroupByMachine` only;
  `BreakdownRow` gained `RawKey` (empty unless a substitution happened).
- `internal/transport/publicapi/usage_handler.go` — `IngestUsageEventsBatch`
  now calls `Service.UpsertMachine(installId, hostname, time.Now())`
  before/alongside the existing `IngestBatch` call; additive, no change
  to the existing ingestion path.
- `internal/transport/bff/usage_handler.go` — `toWireBreakdown` sets
  `RawKey` on the wire only when non-empty.
- `bff-openapi.yaml` — `UsageBreakdownRow.key`'s doc comment updated to
  describe the substitution; new optional `raw_key` field (see Decision).
- `web/src/app/usage/BreakdownPanel.tsx` — `groupBy === "machine"` now
  renders `key` as the label and `raw_key ?? key` as the tooltip (falls
  back to `key` itself when no substitution happened, so the tooltip is
  never blank).
- Checked `web/src/lib/pathDisplay.ts` per the ticket's own instruction:
  its `machine` field is only ever used as an opaque disambiguator
  suffix appended verbatim, no UUID-shape assumption (no truncation, no
  length check) — confirmed no change needed there.

**Tests** (all new, all passing):
- `internal/domain/usage/repo_test.go`: `TestRepo_UpsertMachine_FirstCall_Inserts`,
  `TestRepo_UpsertMachine_SecondCallWithChangedHostname_UpdatesStoredValue`
  (the ticket's own explicitly-named acceptance test — a changed
  hostname on a resend updates the stored value, row count stays 1),
  `TestRepo_UpsertMachine_DifferentInstallIDs_BothLand`,
  `TestRepo_MachineHostnames_NoRows_EmptyMap`, `TestI4_MachinesRepoOnlyQueriesMachinesTable`.
- `internal/domain/usage/service_test.go`: delegation + error-propagation
  tests for `UpsertMachine`, plus `TestService_Summary_GroupByMachine_SubstitutesHostname`,
  `..._NoMachinesRow_FallsBackToInstallID`, `..._GroupByActor_NeverConsultsMachineHostnames`
  (proves the substitution pass is scoped to `group_by=machine` only),
  `..._HostnamesRepoError_Propagates`.
- `internal/transport/publicapi/usage_handler_test.go`:
  `TestHandler_IngestUsageEventsBatch_UpsertsMachine`,
  `..._SecondBatchWithChangedHostname_UpdatesMachine` — real HTTP
  integration tests against a real SQLite-backed router, confirming
  `usage_events.machine` stays the raw `install_id` throughout.
- `internal/transport/bff/usage_handler_test.go`:
  `TestGetUsageSummary_GroupByMachine_ReturnsHostnamesNotUUIDs` (the
  ticket's own explicit acceptance test — seeded `usage_events` +
  `machines` rows, confirms `group_by=machine` returns hostnames, never
  the raw install_id, as `key`), `..._NoMachinesRow_FallsBackToInstallID`,
  `..._GroupByActor_RowsCarryNoRawKey`.
- `web/src/app/usage/BreakdownPanel.test.tsx` (new file, no prior
  component-test convention existed for this component — added one):
  hostname-as-label/install_id-as-tooltip, and the no-`raw_key` fallback
  case.

**End-to-end confirmation, without a live server:** confirmed by reading
`internal/collector/client.go` (`Body.InstallID`/`Body.Hostname`, ticket
12's existing design) that the collector already sends both fields at
the batch's top level with zero changes needed on that side — this
ticket's server-side change alone is sufficient to pick it up. Per the
ticket's own "test-level confirmation ... is sufficient and preferred
over standing up a live demo," this was proven via the HTTP integration
tests above rather than an actual running collector+server+browser
session.

**Verification:**
- `go build ./...`, `go vet ./...`, `go test ./...` — all green (every
  package, including every new test above).
- `gofmt -l` on every changed `.go` file — clean.
- `cd web && npx tsc -b --noEmit` — clean.
- `cd web && npx vitest run` — 5 test files, 23 tests, all passing (21
  pre-existing + 2 new `BreakdownPanel` tests).
- `/chief-review-code` (Standards + Spec axes, parallel sub-agents):
  Standards axis found zero hard `ARCHITECTURE.md` violations and only
  minor, non-blocking judgement-call notes (a mild Primitive-Obsession
  observation on `UpsertMachine`'s three bare params, judged consistent
  with this repo's existing param-style convention rather than a
  divergence). Spec axis found zero missing/partial requirements, zero
  unwarranted scope creep (confirmed `raw_key` is contract-mandated, per
  Decision above), and zero implementation bugs — the fallback logic was
  checked specifically against the ticket's own wording and confirmed to
  never produce a blank/null key.
