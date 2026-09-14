# Contract

Extends story-1's contract (`../story-1/_contract/contract.md`) for the same
project. Sections below are additive unless marked **Supersedes** — a
superseding section replaces the named story-1 clause outright; story-1's
file itself is left as the historical record of what was decided then, not
edited.

## Data model

**`usage_events`** — two changes to the existing table (story-1's schema
otherwise unchanged):

| field | type | notes |
|---|---|---|
| `actor` | string | **Supersedes** story-1: derived from the session's raw launch `cwd`, verbatim — no `.typ-crews/<name>` pattern match, no fallback needed for a non-matching cwd (there is no such thing anymore; any non-empty cwd is a valid actor value). Matches `~/tmp/claude-token-tracker`'s own `project` derivation method. |
| `scan_root` | string, NOT NULL | **New.** The raw scan-root path (as configured in the collector's `scan_paths`) the reporting transcript file was found under — distinct from `path`, which is git-rooted from *touched files*, not the transcript's own location. |

`path` itself (the stored column) is **unchanged** — still ticket 10's
touched-paths method. What changes is how `group_by=path` aggregates it (see
"Cross-machine path identity," below) — a query-time behavior, not a schema
change.

**`scan_roots`** — new table, mirrors `machines`' own shape and lifecycle:

| field | type | notes |
|---|---|---|
| `install_id` | PK (composite) | |
| `scan_root_path` | PK (composite) | the raw path, same value `usage_events.scan_root` stores |
| `name` | string | operator-assigned label (e.g. "main") — per-install, no shared/global naming registry across machines |
| `source_type` | string | set from the collector config's own `scan_paths[].source_type` (defaults to `"claude_code"` if the config omits it) — a declared category at config time, distinct from `usage_events.source` (a fixed per-event fact, unchanged from story-1) — kept as two differently-named fields on purpose so a settable, config-time value is never confused with the fixed, stamped-per-event one |
| `last_seen_at` | timestamp | |

Upserted on every ingestion batch, `name`/`last_seen_at` refreshed each time
— identical lifecycle to `machines`' own upsert-on-every-batch design (a
rename takes effect for all past and future rows immediately).

## Cross-machine path identity — Supersedes story-1 contract's rule 4

Story-1's "same literal path from two machines → append machine as a
UI-only disambiguator" rule is **retired**. Replaced with a real fix at the
aggregation layer:

- `GroupBy.keyFor`'s `GroupByPath` case becomes `e.Machine + ":" + e.Path`
  (was: bare `e.Path`) — `internal/domain/usage/summary.go`. No schema
  change; `machine` already exists on every event.
- Display: `Service.Summary`'s existing hostname-substitution pass (today
  scoped to `group_by=machine` only) extends to also cover `group_by=path`:
  `BreakdownRow.Key` becomes `<hostname>:<path-basename-etc, per existing
  shortenPaths rules>`, `RawKey` carries the ground-truth `<install_id>:<path>`
  for the tooltip.
- The same real project cloned to a different local path on a second
  machine still does not merge automatically — accepted, out of scope (see
  goal.md).

## Path/actor dedup — new rule, no story-1 precedent

For a given session, if `path`'s resolved value equals that same session's
own raw launch `cwd` (or its git-root, via `path.go`'s existing
`ResolveSessionPath`) — the session never touched anything outside where it
started — that session's contribution is **excluded from `group_by=path`
entirely** (both the breakdown and, once filters exist, the filter's own
option list). Applied server-side, before the console's top-8 cutoff
(`BREAKDOWN_TOP_N`) — never in `BreakdownPanel.tsx`, which stays a pure
label-shortening layer over rows the server already decided to include.

Computed once per session (mirroring how `path` itself is already computed
once per session, not once per row).

## Collector config — Supersedes story-1 contract's `scan_paths: string[]`

```json
{
  "scan_paths": [
    {"name": "main", "path": "~/.claude", "source_type": "claude_code"},
    {"name": "backup-install", "path": "~/.claude-local", "source_type": "claude_code"}
  ],
  "install_id": "...",
  "core_url": "...",
  "api_key": "..."
}
```

Each `scan_paths` entry is named at registration, and now also declares its
`source_type` explicitly (optional in the config, defaults to
`"claude_code"` if omitted — every scan root in practice is `"claude_code"`
for this story, but the field is real and settable, not hardcoded, matching
the goal's own forward-compat intent for a future non-Claude-Code source).
The collector already knows which entry produced a given transcript file
during its directory walk (`scan.go`) — that entry's resolved path and
`source_type` become the event's `scan_root` and that scan root's
`scan_roots.source_type` value.

## API surface

**`/api/v1`** — `POST /api/v1/usage-events/batch` body, extended:

```
{
  install_id, hostname,
  scan_roots: [{ path, name, source_type }],   // NEW — batch-level, upserted like hostname already is
  events: [{
    id, session_id, actor, path, machine, model,
    scan_root,                    // NEW — per event, the raw scan-root path
    input_tokens, output_tokens, cache_read_input_tokens,
    cache_creation_input_tokens, timestamp
  }]
}
```

`scan_roots` is upserted into the new `scan_roots` table the same way
`hostname` is already upserted into `machines` — once per batch, not once
per event.

**`/api/bff`**:

- `GET /api/bff/usage/summary?window=...&group_by=actor|path|machine
  &machine=<install_id>&scan_root=<install_id>:<scan_root_path>` — two new
  **optional** query params. Both filters AND together with each other and
  with `window`; when either is set, the whole result (`totals`,
  `breakdown`, `reporting_installs`) is scoped to matching events, not just
  a single panel's own dimension. `scan_root`'s value is the composite
  `<install_id>:<scan_root_path>` string (same machine-prefix convention as
  the cross-machine `path` fix, for the same reason: a bare scan-root path
  or bare name is ambiguous across machines since naming is per-install).
- `GET /api/bff/usage/windows?machine=<install_id>&scan_root=<install_id>:
  <scan_root_path>` — same two new optional params, added so the fixed
  windows table also respects an active filter (story-1 shipped this
  endpoint with zero params).
- `GET /api/bff/usage/scan-roots` — **new endpoint**, no params. Returns
  `[{install_id, hostname, scan_root_path, name}]` for every registered
  scan root across every reporting machine — exists purely to populate the
  console's scan-root filter dropdown (there is no `group_by=scan_root`
  breakdown panel to derive this list from otherwise, per this story's own
  scope: filter-only, no new panel).

## Console filters

- Two new filter controls (machine, scan-root) alongside the existing
  window tabs. Selecting either re-fetches every panel's query with the new
  param attached (global scoping, not per-panel).
- Machine filter's options come from the existing `group_by=machine`
  breakdown response (already has `key`=hostname, `raw_key`=install_id — no
  new endpoint needed). Scan-root filter's options come from the new
  `GET /api/bff/usage/scan-roots`.
- Selected filter values persist to `localStorage`, restored on mount. The
  window tab is **not** persisted (unchanged from story-1: always resets to
  `DEFAULT_WINDOW = "today"`). An invalid/stale stored value (e.g. a machine
  that stopped reporting) is dropped silently on restore, not surfaced as an
  error — falls back to "no filter" the same way a first-ever visit would.

## Testing Decisions

- **Actor-derivation unit test**: `ActorFromCwd`-equivalent returns the raw
  cwd verbatim for an arbitrary directory (no `.typ-crews` needed, no
  `ok=false`/unknown case reachable for a non-empty cwd) — direct regression
  test replacing the old `crewHomePattern` test.
- **Cross-machine path aggregation test**: two `Event`s with identical
  `Path` but different `Machine` produce two distinct `group_by=path`
  breakdown rows, not one merged row — direct regression test for the bug
  this story fixes (`Aggregate`, table-driven, no database needed).
- **Path/actor dedup unit test**: an `Event` whose `Path` equals its own
  session's launch cwd/git-root is excluded from `group_by=path`'s
  breakdown; an `Event` whose `Path` differs from its launch cwd is
  unaffected — both cases in one table-driven test.
- **`scan_roots` upsert test**: mirrors `machines`' own existing upsert
  test — a second batch with a renamed scan root updates the stored `name`,
  not just the first-seen value.
- **Filter query param integration tests**: HTTP-level tests seeding
  `usage_events` across ≥2 machines/scan-roots, asserting `machine=`/
  `scan_root=` on `GET /usage/summary` and `GET /usage/windows` narrow the
  result to matching rows only, and that both filters AND-compose correctly
  when set together.
- **localStorage persistence**: a frontend test asserting a selected filter
  survives a remount — exact tooling/commands to confirm against this
  repo's existing frontend test setup during `/chief-build`, not guessed
  here (story-1 left the same kind of note for its own frontend work).
- **Real second-machine smoke test** (`_map.md` ticket 2, revised after a
  loop-readiness review): not a table-driven unit test, but still fully
  automatable and safe to run inside `/chief-build` — no separate host or
  credentials needed. Run the real collector binary twice against the real
  local core service: once against this story's existing dev scan root with
  the usual `install_id`, once against `~/.claude-openrouter` (confirmed
  real, separate transcripts) with a second, distinct `install_id`. Then
  query `GET /api/bff/usage/summary` and assert: `reporting_installs` is 2,
  `group_by=machine`'s breakdown has two distinct rows, and `group_by=path`
  correctly merges/splits paths across the two real installs per this
  story's own fix (not the pre-story false-merge/never-merge bugs). Genuine
  cross-host network/environment behavior is explicitly not covered by this
  test — deferred to the fleet-wide rollout follow-up story (goal.md's Out
  of Scope).
