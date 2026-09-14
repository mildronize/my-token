# Goal

my-token's collector → core → console path, proven single-machine by story-1,
now proves out **multiple** machines reporting the same core, and the
console gets the filtering a multi-machine dashboard actually needs.

Concretely, this story delivers:

1. **Multi-machine correctness, not just multi-machine capacity.** A second
   collector install (simulated `install_id` while iterating, then a real
   second collector *process* on this same machine — pointed at
   `~/.claude-openrouter`'s real transcripts, its own distinct `install_id`
   — for a final proof against real data and a real network round-trip)
   reports alongside the first. Cross-machine `path` identity stops being
   wrong: two machines reporting the exact same project path no longer
   silently merge
   into one row (the false-merge bug story-1 documented and deferred). The
   same real project cloned to a different local path on a second machine is
   accepted as still not merging automatically — an honest undercount, left
   for a possible future display-level rollup, not solved here.

2. **A generic identity fix, not a fleet-specific patch.** Story-1 shipped
   `actor` derivation hardcoded to typ-fleet's own `.typ-crews/<name>`
   directory convention — a real gap against my-token's own stated design
   goal of being usable by a non-typ-fleet user too. This story fixes it:
   `actor` becomes the session's raw launch directory, the same general
   method the reference collector (`~/tmp/claude-token-tracker`) already
   proved works, with no fleet-specific knowledge anywhere in the derivation.

3. **The console gets two new filters** — by machine, and by a named,
   registered scan root (e.g. an operator labels `~/.claude` "main" and
   `~/.claude-local` something else at collector-config time) — both
   persisted in the browser across reloads. Selecting either narrows the
   whole dashboard (every breakdown panel), not just one panel.

4. **The By path breakdown stops double-counting a session against its own
   By actor row.** When a session never touched anything outside where it
   started, its `path` and `actor` are the same underlying fact — By path no
   longer shows it as a second, separate thing.

All four of these ship together — nothing here is meant to graduate out of
this story into a later one; see `_map.md`'s ticket 1 for why.

## Out of Scope

- Full fleet-wide collector rollout to every real machine in the
  organization — still a further follow-up story; this story proves
  multi-machine correctness with two installs, not universal coverage.
- Deploying the core service/console anywhere externally reachable — a
  local/dev-run core plus one real second collector install is sufficient
  proof for this story, same as story-1.
- Correlating `install_id` with typ-fleet's ship registry automatically —
  story-1 ticket 5 already ruled this out; unaffected here.
- Solving the same-project-different-local-path-per-machine gap — accepted
  as still unmerged (see point 1 above); a possible later display-level
  rollup, not this story's job.
- Backfilling/migrating existing `usage_events` rows to the new `actor`
  shape (raw cwd instead of a bare crew name) — this is still development
  data; old and new rows simply won't merge in By actor across the cutover.
- A shared/global scan-root naming registry across machines — naming stays
  per-install (`_map.md` ticket 5); two machines can each register a root
  with the same name independently, with no cross-machine link implied.
- Non-Claude-Code sources — unaffected by this story, though the new
  `scan_roots` table gets a `source` column now (defaulted `"claude_code"`)
  so a future source doesn't need a migration to add one.
