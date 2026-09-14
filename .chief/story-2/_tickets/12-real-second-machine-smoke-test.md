# 12: Real second-machine smoke test

Type: implementation
Status: resolved
Blocked by: 7, 8, 9, 10, 11

## Delivers

Per the contract's Testing Decisions "Real second-machine smoke test"
entry (`_map.md` ticket 2, revised after a loop-readiness review away from
requiring a real second host):

- Run the real collector binary **twice** against the real local core
  service: once with this story's existing dev scan root and `install_id`,
  once configured with `~/.claude-openrouter` as its scan root (confirmed
  real — a separate Claude config directory with its own real
  `projects/*.jsonl` transcripts) and a second, distinct `install_id`.
- Query `GET /api/bff/usage/summary` against the running core afterward and
  assert: `reporting_installs` is 2; `group_by=machine`'s breakdown has two
  distinct rows; `group_by=path` correctly reflects this story's fix (two
  machines sharing a literal path string stay two rows if the touched
  projects are actually different, or correctly attribute per-machine
  without the pre-story false-merge/never-merge bugs — whichever the two
  real transcript sets actually exercise).
- This is real end-to-end proof — real binary, real transcripts, real HTTP,
  real auth — entirely within this machine's own environment, so it's safe
  to run and assert against inside `/chief-build` without any separate
  host's credentials.
- **Explicitly not covered, by design**: genuine cross-host network
  reachability or environment differences (different OS/Go
  version/filesystem layout on an actually separate machine). Deferred to
  the fleet-wide rollout follow-up story (goal.md's Out of Scope) — do not
  attempt to install anything on a separate real host as part of this
  ticket.

## Verifiable

- The two collector runs both complete and both report successfully
  (non-zero `Inserted` in each `RunResult`).
- `GET /api/bff/usage/summary`'s response shape matches the assertions
  above, checked against the real running core service (not a mocked
  response) — this ticket's own acceptance bar is the real query result,
  not a unit test double.
