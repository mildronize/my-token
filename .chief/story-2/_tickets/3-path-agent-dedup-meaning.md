# 3: What exactly does "ignore By path if already existing in By agent" mean?

Type: wayfinder:grilling
Status: resolved
Blocked by: None (can start immediately)

## Question

มายด์'s point 5, in his own words: "ignore 'By path', if already existing in
'by agent' e.g. hestia can be both place, so, we should not duplicate it." The
console (`web/src/app/usage/UsagePage.tsx`) currently renders three
independent breakdown panels — By actor, By path, By machine — each a
separate ranked dimension of the same underlying cost. What counts as a
"duplicate" between By path and By actor specifically?

## Answer

**Revised after grilling — first pass (below, superseded) hardcoded a
`.typ-crews/<name>` pattern match, which mayd correctly challenged: my-token
is meant to be generic, not typ-fleet-specific (story-1's own stated design
goal), and `internal/collector/actor.go`'s existing `crewHomePattern` regex
already violated that goal even before this ticket. Fixed properly instead
of inherited as-is.**

**Grounded against a real, working reference** (`~/tmp/claude-token-tracker`,
story-1's own adaptation source): its `project` field is just the session's
raw launch `cwd` (decoded straight from Claude Code's own
`~/.claude/projects/<encoded-cwd>/` folder naming) — no git-rooting, no
pattern matching, nothing fleet-specific. Its display shortening
(`short_project_name`) is one line: split the path, take the last segment.
No new mechanism needed anywhere — every piece already exists in my-token.

- **`actor` derivation, simplified:** becomes the session's raw launch `cwd`
  directly (already the exact input `ActorFromCwd` reads today) — the
  `crewHomePattern` regex and its `ok=false`/`"(unknown)"` fallback are
  removed entirely; a session's `cwd` value is used as-is, un-pattern-matched.
  This touches already-shipped story-1 code
  (`internal/collector/actor.go`) — a deliberate fix within this story, not
  deferred, per มายด์'s explicit call.
- **Display:** reuses the existing `shortenPaths` logic
  (`web/src/lib/pathDisplay.ts`) already built for `path` (basename +
  parent-segment walk-up on collision, strictly more capable than the
  reference tool's plain-basename approach) — no new frontend code.
- **Migration: explicitly skipped, มายด์'s call** — this is still
  development data. Existing stored `actor` values (bare crew names, e.g.
  `"hestia"`) and new ones (full raw cwd, e.g.
  `"/home/thw-home/.typ-crews/hestia"`) won't merge in `By actor` across the
  cutover; accepted as a clean-break, not backfilled.
- **The actual dedup rule, restated with zero fleet-specific knowledge:**
  for a given session, if `path`'s resolved value (ticket 10's existing
  git-root/touched-files method) equals that same session's own raw launch
  `cwd` (or its git-root, via `path.go`'s existing `ResolveSessionPath`) —
  i.e. the session never touched anything outside where it started — exclude
  that `path` row from the By path breakdown/filter. It's the same
  underlying fact `By actor` already shows, not a second, distinct thing.
  Structural equality per session, computed once (mirroring how `path`
  itself is already computed once per session, not once per row) — never a
  string/pattern match against any specific directory-naming convention.
