## Destination

my-token supports real multi-machine reporting — not just the data model
shaped for it, but proven with a second machine (simulated for iteration,
one real second host for a final smoke test) and cross-machine `path`
identity actually fixed, not just UI-disambiguated. The console gets a
machine filter and a registered-scan-path-name filter, both persisted in
localStorage, and the By path breakdown/filter stops double-counting a
session against its own By actor row when it never left where it started.
This is the fleet-rollout follow-up story-1's map explicitly deferred.

All five of มายด์'s scope points ship together — nothing graduates out of
this story unless a ticket surfaces a reason to (see ticket 1).

## Notes

- Source: มายด์'s message in conversation, following freya's memory recap of
  story-1's status. Tracked under the same ENG-23 my-task ticket as story-1
  — not filed as a new ENG number.
- Story-1's full context is ground truth here, not re-derived: `../story-1/
  _goal/goal.md`, `../story-1/_contract/contract.md`, ticket 5 (why
  `machine` is my-token's own opaque `install_id`, not typ-fleet's ship
  identity), ticket 10 (the `path` derivation method and its documented
  cross-machine weaknesses), ticket 18 (`machines` table, hostname display).
- Real code: `/home/thw-home/gits/my-token` — `internal/collector/
  {config,scan}.go` (scan_paths, unnamed today), `web/src/app/usage/
  {UsagePage,BreakdownPanel}.tsx` (today's unfiltered three-panel console),
  `bff-openapi.yaml` (group_by query surface), `db/migrations/` (usage_events,
  machines tables).
- Real fleet ships available for the second-machine smoke test:
  `~/.typ-fleet/fleet/ships.json` on thw-home lists `thw-home`,
  `thw-home-prod` (ruled out — don't conflate with prod deploy), and
  `thw-home-gpu` (natural non-prod candidate).
- Ticket 3's revision leaned on the original reference collector story-1
  adapted from, `~/tmp/claude-token-tracker/scan_claude_usage.py`
  (`decode_project_path`/`short_project_name`) — proof that "raw launch cwd,
  basename for display" is a working, generic method, not a new invention.
- Ownership per freya's standing role: app/API/contract is mine to decide.
  Anything touching real prod deploy is มายด์'s call routed through hestia —
  out of scope for this story regardless (see story-1's goal, still true
  here: no external-reachable deploy).

## Decisions so far

- [1: done bar](_tickets/1-done-bar.md): all five scope points fully built,
  nothing deferred by default.
- [2: second-machine proof](_tickets/2-second-machine-proof.md): revised
  after a loop-readiness review — build/iterate against a simulated second
  `install_id`, then prove it for real via a second collector *process* on
  this same machine (`~/.claude-openrouter`'s real transcripts, distinct
  `install_id`), not a separate real host. Cross-host deployment stays
  deferred to the fleet-wide rollout follow-up story, unchanged.
- [3: path/agent dedup meaning](_tickets/3-path-agent-dedup-meaning.md):
  revised away from `.typ-crews` pattern-matching to a fully generic rule —
  `actor` derivation itself is simplified to the session's raw launch `cwd`
  (no regex, matching the proven reference tool's own `project` concept),
  and a `path` row is excluded from By path whenever it equals that same
  session's own raw cwd/git-root — the session never left where it started.
  Fixes a pre-existing genericity gap in already-shipped story-1 code
  (`internal/collector/actor.go`'s `crewHomePattern` regex), not deferred.
  Migration explicitly skipped (dev data, clean cutover, มายด์'s call).
- [4: cross-machine path identity fix](_tickets/4-cross-machine-path-identity-fix.md):
  aggregation keys `group_by=path` on `machine + ":" + path` instead of bare
  `path` — no schema change, `machine` is already on every event. Display
  substitutes hostname the same way `group_by=machine` already does
  (`<hostname>:<path>`, raw install_id in the tooltip). Fixes the false-merge
  bug for good; the same-project-different-local-path bug stays unmerged by
  design, left for a possible later display-level rollup.
- [5: registered path naming shape](_tickets/5-registered-path-naming-shape.md):
  mirrors the `machines` pattern — collector config becomes `[{name, path}]`,
  new `usage_events.scan_root` (raw path) + new `scan_roots(install_id,
  scan_root_path, name, source, last_seen_at)` table upserted every batch,
  display substitutes name via the same Key/RawKey pattern. Naming is
  per-install, not shared/global. `scan_roots.source` added now (defaulted
  `"claude_code"`) for forward-compat with a future non-Claude-Code source.
- [6: filter mechanics](_tickets/6-filter-mechanics.md): server-side —
  optional `machine`/`scan_root` query params on both `GET /usage/summary`
  and `GET /usage/windows`, filters scope the whole dashboard (all panels),
  AND-composed with the window tab. localStorage persists the two filters
  only, not the window tab. Also settles where ticket 3's dedup exclusion
  lives: server-side, before the By path top-8 cutoff.

## Not yet specified

(none — fog is clear)

## Out of scope

- Full fleet-wide collector rollout to every real machine in the
  organization — still the *next* follow-up story past this one; this story
  proves multi-machine correctness with two installs, not universal
  coverage.
- Deploying the core service/console anywhere externally reachable — still
  out of scope, same as story-1. A local/dev-run core plus one real second
  collector install is sufficient to prove this story's destination.
- Correlating `install_id` with typ-fleet's ship registry automatically —
  story-1 ticket 5 already ruled this out; the optional `fleet_ship_name`
  field stays structurally allowed, unpopulated, not this story's job either.
