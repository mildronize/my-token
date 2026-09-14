# Ticket 10 Report

## Ticket

Backend filter support: optional `machine=`/`scan_root=` query params on
`GET /api/bff/usage/summary` and `GET /api/bff/usage/windows`, AND-composed,
scoping the whole result.

## Outcome

done

## Decision

- **Issue:** the build session that produced this ticket's first commit
  was killed by a Claude session rate limit right after committing, before
  confirming `/chief-review-code` had run. The commit itself
  (`a1790af`) was intact and all tests were green, but the review
  confirmation step was lost.
  **Chosen:** re-verified `go build`/`vet`/`test` green on the orphaned
  commit myself, then ran `/chief-review-code` independently against it
  before allowing it to proceed to push/PR — treated the interrupted
  session as "unreviewed," not "reviewed and lost the receipt."
- **Issue:** that review found one real gap against the ticket's own
  Verifiable bullet — HTTP-level `scan_root`-alone narrowing and
  invalid-`scan_root`-returns-empty tests existed for `GET /usage/summary`
  but not `GET /usage/windows`, even though the ticket explicitly said "on
  both" endpoints. The underlying repo/service logic is shared and already
  covered at that layer, so this was a coverage gap, not a suspected
  correctness bug.
  **Options considered:** accept the gap (functionally low-risk, logic
  already proven elsewhere) vs. close it (matches what the ticket literally
  asked for).
  **Chosen:** closed it — added the two missing `/usage/windows` HTTP tests
  (plus a third for full symmetry: a well-formed-but-nonexistent
  `scan_root` pair), amended into the same commit rather than deferred.

## Notes

- PR: https://github.com/mildronize/my-token/pull/5, branch
  `story-2/ticket-10-backend-filter-support`, final commit `078ae1f`
  (amended after the review found the coverage gap) — branched off and
  opened against `story-2/integration`.
- `/chief-review-code` Standards axis: zero hard violations against
  `ARCHITECTURE.md`'s dependency rules; three cosmetic judgment calls left
  as-is (a raw composite-string filter value mirroring ticket 7's own
  established convention rather than a typed struct; a test-double
  reimplementing a small split helper rather than importing it; verbose
  repeated contract-citation comments across several files — flagged as
  worth pruning at a retro, not blocking).
- One prose/implementation mismatch noted, not a functional bug: the
  ticket's own text says the *handler* validates `scan_root`'s shape, but
  that logic actually lives in `repo.go` — behavior is identical either way
  (malformed input still yields an empty result, never a 400).
- `go test ./...` green (before and after the amendment); `gofmt -l`,
  `go vet` clean.
- Next: merge into `story-2/integration` (same pattern as 7/8/9) before
  starting ticket 11, which is blocked on this ticket and ticket 9 together.
