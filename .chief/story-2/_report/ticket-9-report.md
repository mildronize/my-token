# Ticket 9 Report

## Ticket

`GET /api/bff/usage/scan-roots` read endpoint — new BFF handler + OpenAPI
entry + service/repo method, populating the console's future scan-root
filter dropdown.

## Outcome

done

## Decision

- **Issue:** the contract's prose describes the response as a bare array,
  `[{install_id, hostname, scan_root_path, name}]`.
  **Chosen:** wrapped as `{"scan_roots": [...]}` instead, matching this
  same `bff-openapi.yaml`'s own established convention — `UsageWindows`,
  `ApiKeyList`, and `UserList` all wrap their arrays the same way. Both
  `/chief-review-code` axes flagged this as worth recording rather than
  silently deviating; documented in the commit/PR. Same class of issue as
  ticket 8's `openapi.yaml`/`bff-openapi.yaml` naming slip — the contract's
  prose under-specified the exact wire shape, and the existing codebase
  convention settled it.

## Notes

- PR: https://github.com/mildronize/my-token/pull/4, branch
  `story-2/ticket-9-scan-roots-endpoint`, commit `bd562e5` — branched off
  `story-2/integration` (not `main`), since ticket 8's `scan_roots`
  table/repo methods aren't in `main` yet (PRs #2/#3 await human review).
  Opened against `story-2/integration` as base, matching that.
- Backend-only, as scoped — confirmed no collector or console frontend
  changes.
- `/chief-review-code` raised two judgment calls, both left as-is (no fix
  needed): minor doc-duplication, and the hostname-fallback logic being
  inlined rather than a shared helper — consistent with the exact
  pre-existing pattern it mirrors (`group_by=machine`/`group_by=path`);
  refactoring that shared pattern is out of this ticket's scope.
- `make test` green; ASCII-only `db/queries/scan_roots.sql` check passing.
- Next: merging this into `story-2/integration` locally (same as tickets
  7+8) before starting ticket 10, since both touch overlapping files
  (`bff-openapi.yaml`, `service.go`, `usage_handler.go`).
