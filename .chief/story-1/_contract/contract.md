# Contract

## Data model

**`usage_events`** — one row per turn (one unique `message.id` carrying a
`usage` dict — see ticket 9, not one per JSONL line):

| field | type | notes |
|---|---|---|
| `id` | PK | = `message.id`, the idempotency key (not the JSONL record's own `uuid` — ticket 9) |
| `session_id` | string | |
| `actor` | string | derived from the scanned session's launch directory matching a known crew-home pattern (`.typ-crews/<name>` or equivalent) — the same directory-naming convention every crew already has, not auto-detected some other way |
| `path` | string | ticket 10's full method: extract touched paths → drop scratch → git-root each (raw-path fallback if ungit-rooted) → majority vote → session-`cwd` fallback → `"(unknown)"` final fallback |
| `machine` | string | = collector's own `install_id` (UUID, minted on first run, persisted — ticket 5) |
| `model` | string | |
| `input_tokens`, `output_tokens`, `cache_read_input_tokens`, `cache_creation_input_tokens` | int | read directly off the deduped `usage` dict, never summed into each other (ticket 9) |
| `cost` | numeric | computed via the ported pricing table (pure function) |
| `source` | string | fixed `"claude_code"` for this story (goal: non-Claude-Code sources out of scope) |
| `created_at` | timestamp | |

**`git_root_cache`** — `directory` (PK), `git_root`, `resolved_at` (ticket 10 §8).

**`session_path_votes`** — `session_id`, `git_root`, `vote_count`, PK`(session_id, git_root)`, plus a `last_scanned_offset` per session for incremental re-scans (ticket 10 §8).

**Collector's own local config** (JSON, per goal): `scan_paths: string[]` (e.g. `["~/.claude", "~/.claude-local"]`), `install_id` (generated, persisted), `core_url`, `api_key`.

**`machines`** — `install_id` (PK), `hostname`, `last_seen_at`. Added on top of ticket 11's original design, which read `hostname` off the ingestion payload and discarded it. `usage_events.machine` (the `install_id`) is unchanged and stays the real identity/join key — `machines` exists purely so the console has a readable label to show instead of a raw UUID. Upserted on every ingestion batch (`hostname`/`last_seen_at` refreshed each time), so a renamed machine's display catches up rather than staying stuck on whatever it was first seen as.

## Console display rules

**`path` label shortening**, grilled after the goal/contract were drafted:

1. Show the path's basename only (e.g. `/home/thw-home/gits/my-template` →
   `my-template`).
2. **Collision scope is the rendered set, not global**: two labels only need
   to differ from each other within the same list actually on screen right
   now (e.g. one panel's top 8 for one window) — not against every path the
   core service has ever seen. A label can legitimately render differently
   in a different window/list; that's an accepted tradeoff, not a bug.
3. **On collision, walk up parent segments one at a time until unique**
   (`gits/my-template` vs `other/my-template`, or deeper if needed) — no
   fixed cap at one parent segment.
4. **If two rows still show the literal identical path string** (a real,
   separately-flagged case, not the same thing as a basename collision):
   that means the same path string was reported by two *different machines*.
   Cross-machine `path` identity is currently just the raw canonicalized
   path (ticket 10) — known to be wrong in two ways (coincidentally-same-path
   unrelated projects on different machines get merged; the same real
   project cloned to different local paths on different machines does NOT
   get merged) — **deliberately deferred to the fleet-rollout follow-up
   story**, since this story never has a second machine reporting to test
   against. Until that's fixed, the console adds the machine as a final,
   UI-only disambiguator (e.g. `my-template (thw-home)`) purely so an
   identical-string collision isn't silently indistinguishable on screen —
   this does **not** change what's aggregated together underneath.
5. **The full canonical path is always available via a tooltip** (`title`
   attribute) on the shortened label, regardless of whether shortening
   collided with anything — never hide the ground-truth value behind a
   shortened display form.

**`machine` label**: the "By machine" breakdown shows `machines.hostname`
(joined on `usage_events.machine` = `machines.install_id`), not the raw
`install_id` — same "shortened label, full value still reachable" pattern
as `path`, with the raw `install_id` available via tooltip. Falls back to
the raw `install_id` itself if a machine has sent events but `machines`
somehow has no row for it (should not happen given the upsert-on-every-batch
design, but never show a blank label).

## Frontend theme

One consistent amber/gold theme across the **whole** app, not scoped to
`/usage` — the original split (ticket 14: new theme for `/usage` only,
leftover blue theme everywhere else) was an unreviewed judgment call that
turned out not to be what was wanted. This includes the server-rendered
login/error pages (`renderLoginError` and friends) — no screen a user
actually sees, before or after login, should still carry the old theme.

## API surface (my-template's two-surface split, ticket 4)

**`/api/v1`** (Bearer API-key, collector-facing):
- `POST /api/v1/usage-events/batch` — collector POSTs new events since its last successful send. Idempotent on `id` (`message.id`) — a resend of an already-ingested batch changes nothing. Body: `{ install_id, hostname, events: [{ id, session_id, actor, path, machine, model, input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens, timestamp }] }`. Server computes `cost` and `source`, doesn't trust the client for those. `hostname` is now upserted into `machines` (see Data model) instead of being discarded.

**`/api/bff`** (SSO session, console-facing):
- `GET /api/bff/usage/summary?window=5h|24h|today|week|month|year|lifetime&group_by=actor|path|machine` → `{ totals: {tokens, cost, turns}, breakdown: [{key, tokens, cost, turns}], reporting_installs: N }` — `reporting_installs` exists specifically to surface the goal's partial-view caveat (console must show it, not imply completeness), and is scoped to the selected window, not lifetime, so it always matches what `totals` actually covers. `group_by=machine`'s breakdown `key` is now `hostname` (joined via `machines`), not the raw `install_id`. `year` is calendar-anchored (Jan 1 00:00 UTC of the current year → now), matching `today`/`week`/`month`'s existing calendar-boundary precedent — not a rolling 365-day window.
- `GET /api/bff/usage/windows` → the fixed table for the console's time-window view — now 7 rows: `5h`/`24h`/`today`/`week`/`month`/`year`/`lifetime`. `lifetime` and `year` were grilled in after ticket 14 (which had deliberately kept `lifetime` tab-only, tile-only) — both are now real, selectable tabs that also re-scope the breakdown panels, so both belong in the fixed table too for consistency between what's clickable and what's listed.

## Testing Decisions

- **Pricing function**: pure, table-driven unit tests — no framework dependency, ported straight from the reference collector's `PRICING` table.
- **Turn-counting regression test**: feed a fixture transcript with the exact multi-line-per-`message.id` shape ticket 9 found (one call split across `thinking`+`text` lines) and assert the ingested count is 1 turn, not 2 — this is a direct regression test for the bug that was actually caught this session, not a hypothetical.
- **Path-attribution unit tests**: inject a fake git-root resolver (no real subprocess) and test the extraction → scratch-exclusion → majority-vote → fallback-chain logic against constructed touched-path lists, including the exact multi-tree case found in ticket 10 (94.8%-vs-noise split) and the all-scratch / all-fallback edge cases from ticket 10 §9.
- **One real integration test**: a temp git repo + a fixture transcript, asserting the whole pipeline (extract → cache → vote → ingest) produces the right `path`, `id`-deduped turn count, and cost — external behavior end to end, not internals.
- Test external behavior via the API surface (`/api/v1` ingestion, `/api/bff` reads), mirroring my-template's own `todo` domain module's test shape — exact test tooling/commands to confirm against my-template directly during `/chief-build`, not guessed here.
- **Multi-crew scan fixture test**: a scan root containing 2-3 distinct crew-home-shaped subdirectories, each with its own small fixture transcript and a distinct `actor` pattern, asserting the collector finds and correctly attributes all of them separately — not verified by any existing test. This exact scenario was only ever checked by hand against real production data this session, which caused real friction (a stale config file went unnoticed for a full round-trip before being caught).
- **Fork-rename verification bar**: `go build ./...` and `go test ./...` green with the `todo` domain fully removed (not just renamed) — per `docs/GETTING-STARTED.md` step 8's own warning, `go build` passing alone is not sufficient evidence (a stale test-only reference, e.g. `compositeServer`'s embed, only surfaces via `go test`/`go vet`). Both must be run and checked, not just the first.
