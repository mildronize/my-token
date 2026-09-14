# Ticket 4 Report

## Ticket
Frontend: `/machines/:installId` route, `MachineDetailPage` (identity/stats,
scan-roots, "View in Usage dashboard" button), `useUsageFilters.ts` export
for the cross-page filter handoff. Last ticket in story-3.

## Outcome
done — story-3's full frontier is now resolved.

## Decision

- **Issue:** the goal/contract didn't specify what a `MachineDetailPage`
  should render for an `installId` absent from the machines list (there's
  no second single-machine endpoint, per contract — the page finds its row
  client-side from the same list ticket 1 returns).
- **Options considered:** silently render nothing; treat it as a hard
  error state; render an explicit "not found" state distinct from an
  error (since the fetch itself succeeds).
- **Chosen:** explicit not-found state with a link back to `/machines` —
  matches the fetch-succeeded-but-empty-result shape rather than
  conflating it with a real fetch failure.

Also: kept `MACHINE_STORAGE_KEY` module-private, exporting only
`presetMachineFilter(installId)` — the contract allowed either, and
nothing outside the one call site needs the raw key.

## Notes

- Committed directly to `story-3/machines-page` (`30ca5d6`), pushed as a
  clean fast-forward — confirms no interleaving writes from the
  concurrent-process risk flagged on ticket 3.
- Branch/HEAD verified twice (before starting, right before commit) per
  the standing caution from ticket 3's concurrent-checkout incident. One
  more instance of that risk surfaced here too: an uncommitted
  `docs/readme-coauthoring` README.md change appeared and later
  disappeared from the working tree mid-build, untouched by this ticket's
  own commit (`git show --stat HEAD` confirms only the 6 ticket-4 files).
  **Flagging to มายด์**: something else using this same shared checkout
  may have lost in-progress work on a README — worth a direct check with
  whoever owns that branch, not something for me to act on since it isn't
  my branch or my call.
- Tests: `MachineDetailPage.test.tsx` (identity/stats, scan-roots
  filtering, not-found, zero-scan-roots) + the explicit cross-page
  contract test (button click -> fresh `UsagePage` mount's first fetch
  carries `machine=<installId>`, mirroring `UsagePage.test.tsx`'s existing
  pattern). Full suite 78/78, typecheck + build clean.
- `/chief-review-code` clean both axes; two suggested cleanups applied
  before commit (kept `MACHINE_STORAGE_KEY` private per above; removed a
  non-null assertion by hoisting `install_id` after the narrowing check).
- Small unrequested addition, flagged for visibility: a "Back to Machines"
  link in the detail page's topbar — low-risk UX, not itself in the
  contract.
- Scope check against goal.md/contract.md: both pages, nav link, summary
  strip, all six table columns with final labels, row-click nav,
  scan-roots properly laid out (not a table cell), cross-page handoff —
  all present and tested. Did not personally re-audit ticket 1/2's own
  backend test coverage line-by-line (already merged, out of this
  ticket's scope) — worth an independent spot-check before merge if full
  confidence is wanted.

## End-to-end verification (post-ticket-4, whole branch)

Re-ran everything together on the final `story-3/machines-page` HEAD
(30ca5d6), not just ticket 4's own diff: `go build`/`go vet`/`go test ./...`
all green, `tsc --noEmit` clean, full frontend suite 78/78, `npm run build`
clean. `web/dist/.gitkeep` restored after the build deleted it (known
runbook step, `locker/memory/active/my-token-dev-server-restart.md`).
