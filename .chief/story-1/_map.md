## Destination

A working, single-machine version of **my-token** (a token/cost tracking
service): a collector CLI + a console UI, proving out the `actor`/`path`/
`machine` data model. `machine` is an opaque `install_id` my-token mints
itself, not borrowed from typ-fleet (ticket 5) — see Decisions. Deployment
and fleet-wide multi-ship rollout are a separate follow-up story, not this
one.

## Notes

- Source ticket: ENG-23 (my-task, project ENG), assigned to freya, opened by
  มายด์.
- Build against my-template's actual current `main` — PR #11 (huma migration)
  is still open/unmerged, so `main` today is oapi-codegen+Gin, not huma.
- Example collector to adapt from: `~/tmp/claude-token-tracker` — a Windows
  desktop app that scans `~/.claude/projects/*.jsonl`, prices usage per-model
  family, stores it in SQLite (`usage.db`, keyed by message uuid so re-scans
  are idempotent), and optionally POSTs new records to a remote aggregator.
  Read in full during charting — see `scan_claude_usage.py`'s `PRICING` table,
  `ingest()`, and `aggregate_from_db()` for the shape worth reusing.
- Vocabulary is deliberately generic, not typ-fleet-specific, since my-token
  should make sense to a non-typ-fleet user too: `actor` (not "crew"), `path`
  (not "project"), `machine`.
- Fleet-wide rollout, "shipd"-style multi-location install, and typ-fleet's
  multi-ship architecture are explicitly deferred to a later story. This
  story's schema stays forward-compatible with them (the `machine` field
  exists for this reason) without taking on their design now.
- `shipd` (ticket 6) is typ-fleet's existing per-ship control daemon — the
  rollout model this ticket's "install... like shipd" phrase is pointing at.
- Reopened during `/chief-plan` Phase 1 (goal review): `actor` and `machine`
  are clear on how to collect them (Claude Code's own session pattern; install
  time, respectively), but `path` was not — see tickets 7 and 8.

## Decisions so far

- [1: deliverable boundary](_tickets/1-deliverable-boundary.md): single-machine,
  local/dev-runnable is enough to count as done; fleet rollout is a follow-up
  story.
- [2: data model dimensions](_tickets/2-data-model-dimensions.md): three
  first-class fields — `actor`, `path`, `machine` — generic terms, no
  typ-fleet jargon.
- [3: theme color](_tickets/3-theme-color.md): amber/gold accent on a dark
  neutral background.
- [6: what is shipd](_tickets/6-what-is-shipd.md): typ-fleet's existing
  per-ship control daemon; confirms `machine` is the right shape for future
  rollout without building rollout machinery now.
- [5: multi-ship architecture](_tickets/5-multi-ship-architecture.md): typ-fleet
  has no stable/global machine identity to borrow (ship names are per-host,
  registry-relative, not canonical — `crates/fleet/src/registry.rs`). `machine`
  is my-token's own opaque `install_id` (UUID, minted on first run, persisted),
  never `ship_id`/`ship_name`; hostname is a separate display field. Follow-up
  rollout story can add an optional `fleet_ship_name` correlation field later.
- [4: my-template detail](_tickets/4-my-template-detail.md): fork my-template
  `main` (Go/Gin, oapi-codegen — not the unmerged huma PR #11). Reuse its
  `/api/v1` + `/api/bff` two-surface split for collector-ingestion vs.
  console-UI auth, copy the `todo` domain module's Repository+Service shape
  for a new `usage_events` module (uuid-keyed, INSERT-OR-IGNORE idempotent),
  and its embedded-SPA single-binary deploy. Port the reference collector's
  pricing table as a pure function. NOT from my-template: the collector/CLI
  itself (tiana's domain, ticket 5) and the `path` field's population (the
  collector must detect and send its own working directory — my-template's
  identity model doesn't track this).
- [7: path collection research](_tickets/7-path-collection-research.md):
  Claude Code's transcript *directory name* is a lossy slug encoding (a real
  collision found on this machine); each JSONL record's `cwd` field is exact
  and verbatim instead. But `cwd` is per-record and **drifts within a
  session** as the shell's actual working directory changes (confirmed live
  on this session's own transcript) — it is not a stable session-level
  signal. `path` must be derived from the session's *first* `cwd`, not
  recomputed per turn (see ticket 8).
- [8: path canonicalization strategy](_tickets/8-path-canonicalization-strategy.md):
  `path` is computed **once per session**, from that session's first
  `cwd`-bearing record (before any in-session drift — see ticket 7), then
  git-root walk-up (`git rev-parse --show-toplevel`) on that one value,
  falling back to the raw value if not inside a git repo. Every turn in the
  session shares that one `path`; never recomputed per turn.
- [9: by-path collection, verified](_tickets/9-by-path-collection-verified.md):
  An inline hand-check (not a proper research pass) had produced 2x-inflated
  numbers, caught by มายด์ as implausible. Root cause, confirmed by an actual
  research agent against the raw files: one LLM call can be split across
  several JSONL lines (one per content block — `thinking`, `text`, `tool_use`),
  each carrying an identical `usage` dict under one shared `message.id`.
  Summing per-line double/multi-counts. **Correct unit is one unique
  `message.id`, not one line/`uuid`** — confirmed this inflates a real
  session's totals by 1.67x if done wrong. This bug is *already present* in
  the reference collector (`~/tmp/claude-token-tracker`), which dedupes on
  line `uuid`, not `message.id` — **do not copy that dedup key** (this also
  corrects ticket 4's "uuid-keyed... INSERT-OR-IGNORE" note — the idempotency
  key for a usage row must be `message.id`, not the JSONL record's own
  `uuid`). Also confirmed: must recursively scan `<session-uuid>/subagents/agent-*.jsonl`
  (not just the top-level session file) or subagent usage is silently
  dropped; those files inherit the parent session's `path`, not their own.

- **Correction #1, caught by มายด์:** a single machine's `path` total is a
  *partial* slice, not a project's real total. A path's true total cost can
  only come from the core service summing every machine's collector report
  for that path over its whole lifetime — never from one install alone.
  Worth stating explicitly in the goal/contract, not leaving implicit.
- [10: path attribution, real source](_tickets/10-path-attribution-real-source.md):
  **Correction #2, caught by มายด์, then verified by research** — the "zero
  commits from this machine" read in Correction #1 was itself wrong. The real
  my-template work (146 commits, 60,853 insertions) IS on this machine, done
  by luna. `cwd`-based `path` (tickets 7/8) missed it because luna's Bash
  commands `cd` inline per-command (`cd /path && git commit ...`); the
  harness's session-level `cwd` field only reflects the session's *launch*
  directory and never updates for a `cd` inside a command string — so it
  stayed pinned to her own crew home for 100% of her records, across all 5 of
  her sessions, with zero drift. Confirmed via exact (~1-2s) timestamp
  matches between her `git commit` Bash calls and real commit timestamps, for
  specific commit hashes. `path` now needs a second signal on top of `cwd`:
  extract every real filesystem path touched by the session's tool calls
  (`Read`/`Write`/`Edit`/`Glob` targets, absolute paths in `Bash` command
  strings), drop `/tmp/...` scratch noise (deny-listed outright, before
  git-rooting is even attempted — scratch space never counts even if it
  happens to contain a `.git`), git-root **each remaining path
  individually** (falling back to the raw path itself if genuinely not a
  git repo), then take the **most frequent git-root by count** (majority
  vote — literal common-directory-of-everything was tried and collapses to
  `/`, since a real session touches several unrelated trees at once, not
  just the one it's "about"). Verified on real data: 94.8% vote share
  correctly picked `my-template` over 4 other trees the same session
  touched. Falls back to session-level `cwd` attribution (tickets 7/8) if
  zero real candidate paths exist, then to an explicit `path = "(unknown)"`
  sentinel if even that fails — usage/cost is never silently dropped just
  because attribution failed. Directory→git-root results and each session's
  running vote tally both persist in the collector's own SQLite DB (same DB
  as `usage_events`), so a periodic re-scan only parses new lines and never
  redoes a `git rev-parse` for a directory already resolved.

## Not yet specified

- Nothing sensed. Frontier is empty — see below.

## Out of scope

- Fleet-wide rollout / installing on every ship (see ticket 1). Not ruled out
  forever — it's the natural follow-up story — just not this one's job.

## Frontier

Empty — all 10 decision-tickets resolved. The fog is clear; `/chief-plan` can
resume from where it paused (Phase 1, goal review) without re-grilling any of
this.
