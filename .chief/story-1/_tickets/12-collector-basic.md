# 12: Collector — scan, dedupe, price, report (simple path attribution)

Type: implementation
Status: resolved
Blocked by: 11

## Delivers

A standalone collector (CLI) that:

- Reads its JSON config: `scan_paths` (e.g. `["~/.claude", "~/.claude-local"]`),
  `install_id` (generated on first run if absent, then persisted — ticket 5),
  `core_url`, `api_key`.
- Recursively finds every session transcript under each scan path,
  **including `<session-uuid>/subagents/agent-*.jsonl`** (ticket 9/10 — a
  non-recursive scan silently drops subagent usage).
- Extracts one usage row per unique `message.id` carrying a `usage` dict —
  **not** per JSONL line/record `uuid` (ticket 9's counting-bug fix; include
  the regression test from the contract's Testing Decisions: the
  `thinking`+`text` multi-line-per-call fixture must count as 1 turn).
- Computes `cost` via the pricing table ported from
  `~/tmp/claude-token-tracker/scan_claude_usage.py` as a pure function
  (contract's Testing Decisions: table-driven unit tests). Note: ticket 11
  already ported its own server-side copy of this same table (since the
  server, not the client, is the source of truth for `cost` — ticket 11's
  ingestion endpoint never trusts a client-supplied cost). The collector's
  own copy here is for whatever local display/estimate it wants to do before
  sending, not the authoritative figure.
- Determines `actor` from the session's launch directory matching a known
  crew-home pattern (`.typ-crews/<name>`).
- Sets `machine` = the collector's own `install_id`.
- Determines `path` via the **simple** method only for this ticket: session's
  first `cwd`, git-root-canonicalized, raw `cwd` if not a git repo (tickets
  7/8 — drift-corrected, but not yet the full touched-paths method). Ticket
  13 upgrades this in place.
- Batches new events and `POST`s them to ticket 11's
  `/api/v1/usage-events/batch`, tracking what's already been sent so re-runs
  don't resend (incremental scan, per-session scan-position tracking —
  contract's `session_path_votes`-adjacent bookkeeping, simplified for this
  ticket since full path-voting isn't built yet).

Demoable/verifiable on its own: run the collector configured with
`scan_paths: ["/home/thw-home/.claude/projects/-home-thw-home--typ-crews-freya"]`
**only** — this is where freya's own transcripts actually live (a slugged
encoding of her crew-home path under `~/.claude/projects`, not the crew-home
directory itself — an earlier version of this ticket pointed at the wrong
one, `/home/thw-home/.typ-crews/freya`, which contains no transcripts at
all) — not the whole
shared `~/.claude/projects` — since this dev host's `~/.claude/projects` is
shared across every crew here (confirmed earlier this session), and an
unattended demo has no business reading other crews' private session
content just to prove the pipeline works. Confirm real rows land in ticket
11's service via its own API (query them back), with correct deduped turn
counts and costs. (This scoping is a property of *this demo on this
particular shared host*, not of the collector's design — a real single-user
deployment target would correctly point `scan_paths` at its own
`~/.claude`.)

Out of scope for this ticket: the full touched-paths `path` method (13), the
console (14).
