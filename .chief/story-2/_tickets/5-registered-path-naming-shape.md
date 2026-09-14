# 5: Registered path naming — what does it mean end to end?

Type: wayfinder:grilling
Status: resolved
Blocked by: None (can start immediately)

## Question

Today the collector's config is `scan_paths: string[]` (unnamed,
`internal/collector/config.go`) — plain directory strings, no identity beyond
the path itself. No schema anywhere stores which scan root an event came
from; `usage_events` only carries the git-rooted `path` per touched file
(story-1 ticket 10), not the top-level scan root it was found under.

มายด์ wants scan roots nameable at registration (his example: label
`~/.claude` "main", `~/.claude-local` something else) and the console's path
filter to filter/display by that registered name.

## Answer

**Mirrors the `machines` pattern exactly — the same "raw identifier on every
event, friendly name in a separate table, substituted at display time" shape
story-1 ticket 18 already established for hostnames, and ticket 4 just reused
for path's machine-prefix.** Confirmed this is genuinely new information —
the scan root a transcript came from is not derivable from the existing
`path` column (which comes from git-rooting a session's *touched files*, a
different thing entirely from which top-level directory the transcript
*itself* was found under) — must be captured at collection time.

- **Collector config:** `Config.ScanPaths []string` becomes
  `ScanPaths []{Name, Path}` — each entry named at registration.
- **Ingestion:** each reported event carries the resolved scan-root path it
  came from (the collector already knows this during its directory walk).
- **Schema:** new `usage_events.scan_root` column, raw path value (same
  treatment as `machine` storing the raw `install_id` — no name baked into
  the event row itself). New `scan_roots(install_id, scan_root_path, name,
  source, last_seen_at)` table, PK `(install_id, scan_root_path)`, upserted
  on every ingestion batch exactly like `machines` — a rename takes effect
  immediately for all past and future rows, same "renamed machine's display
  catches up" property `machines` already has.
- **Display:** reuses the same `Key`/`RawKey` substitution pattern
  (`Service.Summary`) already used for machine hostnames and (per ticket 4)
  path's machine-prefix — friendly name shown, raw scan-root path in the
  tooltip.
- **Naming scope: per-install, not shared/global.** Each machine's
  `scan_roots` rows are scoped to its own `install_id` — two machines can
  each register a root named "main" independently, no cross-machine
  coordination or shared registry. Matches how `machine`/`install_id` itself
  already works (story-1 ticket 5: no stable/global machine identity exists
  to borrow) and avoids inventing fleet-wide coordination this story's goal
  doesn't need. Concrete example worked through with มายด์: thw-home and
  thw-home-gpu can each have their own "main", and the console's filter
  lists them as separate entries (`thw-home: main`, `thw-home-gpu: main`),
  never merged just because the label string matches.
- **Forward-compat for non-Claude-Code sources:** `scan_roots` gets a
  `source` column now, defaulted to `"claude_code"` (mirroring
  `usage_events.source`, which already exists for exactly this reason —
  story-1's goal already names non-Claude-Code sources as explicitly future/
  out-of-scope, not unplanned). Decided over the leaner "add it later when a
  second source actually exists" alternative — มายด์'s call, accepting the
  small cost now to avoid a migration + backfill-source-of-truth question
  later.

Whether this new dimension becomes its own breakdown panel ("By scan root")
or stays filter-only (no dedicated panel) is ticket 6's job, not decided
here.
