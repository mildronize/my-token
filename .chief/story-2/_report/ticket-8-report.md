# Ticket 8 Report

## Ticket

Registered scan-path naming, write path: collector config becomes
`scan_paths: [{name, path, source_type}]`, two migrations
(`usage_events.scan_root`, new `scan_roots` table), ingestion handler
upserts `scan_roots` on every batch.

## Outcome

done

## Decision

- **Issue:** the ticket names `bff-openapi.yaml` as the schema file for the
  ingestion endpoint's extended body, but `POST /api/v1/usage-events/batch`
  is actually defined in a separate `openapi.yaml` (`bff-openapi.yaml` only
  covers the `/api/bff/*` read-side routes).
  **Chosen:** edited `openapi.yaml` (the correct file), left
  `bff-openapi.yaml` untouched — confirmed correct by `/chief-review-code`'s
  Spec axis. A naming slip in the ticket/contract's own prose, not a real
  ambiguity about which file owns the ingestion contract.
- **Issue:** the contract doesn't say whether `scan_root`/`scan_roots` on
  the wire are required or optional (unlike `source_type` in the collector
  config, which is explicitly optional-with-a-default).
  **Chosen:** required, matching `install_id`/`hostname`'s own existing
  required-field precedent on the same endpoint — there's no independently
  deployed old collector version this story needs to stay wire-compatible
  with (development data, per goal.md's Out of Scope).

## Notes

- PR: https://github.com/mildronize/my-token/pull/3 (branch
  `story-2/ticket-8-scan-root-write-path`, commit `43e6f14`, branched off
  `main` directly — confirmed no overlap with ticket 7's in-flight branch).
- `/chief-review-code` Standards axis flagged two judgment calls, both
  resolved inline: a naming collision (domain type renamed
  `UpsertScanRootInput` to avoid colliding with the generated wire type)
  and a deliberate primitive-params shape kept as-is (mirrors `UpsertMachine`'s
  own existing bare-params signature, which the ticket explicitly said to
  mirror — documented as a kept judgment call, not silently dropped).
- Backend-only, as scoped — confirmed zero frontend/BFF-filter/`GET
  /scan-roots` work leaked in from tickets 9-11.
- `make test` (Go + Vitest) green; ASCII-only `db/queries/*.sql` check and
  this repo's own invariant-coverage test both passing explicitly.
- Frontier after this ticket: 9 (blocked by 8) and 10 (blocked by 7, 8) are
  both now open — 9 has no file overlap with 10; safe to build in either
  order.
