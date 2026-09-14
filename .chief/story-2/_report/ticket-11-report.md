# Ticket 11 Report

## Ticket

Console filter UI: machine + scan-root filter dropdowns wired to tickets
9 & 10, localStorage persistence (filters only, not the window tab).

## Outcome

done

## Decision

(No open ambiguity reached this session — both deviations below were
found and self-corrected inline during `/chief-review-code`, not punted
upward as an open decision.)

- Standards axis found duplicated key-derivation logic between the new
  filter-option builders and `UsagePage.tsx`'s own stale-value
  reconciliation; fixed by having reconciliation reuse the same builder
  functions rather than re-deriving keys separately.
- Spec axis found the ticket's "a stale filter value falls back silently"
  bullet was only tested for the machine filter; added the matching
  scan-root test to cover both, as the ticket's own contract section
  implies (both filters share the same "invalid/stale = drop silently"
  rule).

## Notes

- PR: https://github.com/mildronize/my-token/pull/6, branch
  `story-2/ticket-11-console-filter-ui`, commit `9db7aef` — branched off
  and opened against `story-2/integration`.
- Frontend-only, as scoped — zero Go files touched, confirmed.
- Regenerated `web/src/lib/api/bff-schema.gen.ts` as a necessary
  prerequisite (it was stale — missing the `scan_root`/`machine` params and
  `scan-roots` schema tickets 9/10 already shipped server-side) — not scope
  creep, just catching up the generated client to what the backend already
  contractually supports.
- New files: `web/src/app/usage/useUsageFilters.ts` (localStorage
  persistence hook) and `UsageFilters.tsx` (the two filter controls).
- `make test` green: 53 Vitest tests plus the full Go suite unaffected.
- This was the last ticket before the final smoke test (12) — every piece
  of story-2's goal is now implemented and merged into
  `story-2/integration`; ticket 12 is the real end-to-end proof.
