# Ticket 7 Report

## Ticket

Generic `actor` derivation (raw launch cwd, no `.typ-crews` regex),
cross-machine `path` identity fix (`machine:path` aggregation key +
hostname substitution), and the path/actor dedup exclusion — all three in
`internal/domain/usage/summary.go`/`service.go` plus `actor.go`, with a
small `BreakdownPanel.tsx` cleanup.

## Outcome

done

## Decision

- **Issue:** the contract says the dedup check should fire when `path`
  "equals that same session's own raw launch `cwd`, **or its git-root**."
  A live git-root resolution against `Actor` inside `Aggregate` is
  architecturally impossible — `Aggregate` is a pure, resolver-free,
  database-free function (contract's own Testing Decisions requirement),
  and it runs on the core server aggregating events that may have come
  from a different machine's filesystem entirely; there is no local
  directory to run `git rev-parse` against for a foreign machine's `Actor`
  value.
  **Options considered:** (a) raw-cwd equality only (`e.Path == e.Actor`),
  computed purely server-side, matching the common case (a crew launching
  from its own crew-home root, itself a git repo, so `path`'s git-rooting
  doesn't move the value); (b) push git-rooting of the actor's own cwd back
  to the *collector* (ingestion time, where the real filesystem exists) as
  a third stored value, compared at aggregation time; (c) drop the dedup
  rule's git-root half from the contract entirely and only test the raw-cwd
  case (what the ticket's own Verifiable bullets already specified).
  **Chosen:** (a), matching (c)'s test scope — raw-cwd equality only. An
  independent Spec-review pass (part of `/chief-review-code`) confirmed this
  is a reasonable, well-justified scope reading given the architectural
  constraint, not a silently broken promise.
  **Residual, accepted gap:** a session launched from a *subdirectory* of a
  git repo that never left it (`Actor` = raw subdirectory, `Path` =
  git-rooted to the repo's top level — two different strings) won't be
  deduped. Not a concern for this fleet's actual usage (crews launch
  exactly at their crew-home root), but a real edge case if a future story
  needs option (b)'s ingestion-time fix.

## Notes

- PR: https://github.com/mildronize/my-token/pull/2 (branch
  `story-2/ticket-7-actor-and-cross-machine-path`, commit `bdf8ec0`).
- `/chief-review-code` flagged (not fixed, out of this ticket's named
  files): `web/src/lib/pathDisplay.ts`'s old machine-disambiguator branch
  is now dead code — its one call site was retired in `BreakdownPanel.tsx`
  by this ticket. Worth a small cleanup ticket later, not blocking.
- `make test` (Go + Vitest) green; `go vet`, `gofmt -l`, `tsc --noEmit` all
  clean.
- Ticket 8 (registered scan-path naming) is independent of this ticket's
  files — no merge-order concern for the next frontier round.
