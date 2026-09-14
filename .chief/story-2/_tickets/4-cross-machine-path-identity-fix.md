# 4: How should cross-machine `path` identity actually be fixed?

Type: wayfinder:grilling
Status: resolved
Blocked by: None (can start immediately)

## Question

Story-1's contract.md documented the current gap explicitly: `path` identity
is just the raw canonicalized directory string, known wrong two ways —
(a) two unrelated projects on different machines that happen to share a
canonicalized path string get wrongly merged, and (b) the same real project
cloned to a different local path on a second machine does NOT get merged with
its first machine's rows. This story's done-bar (ticket 1) requires this
actually fixed, not just UI-disambiguated.

Needs a real design decision, not just an implementation: what makes two
paths on different machines "the same project" for real? Also: does fixing
this require a schema/migration change to `usage_events`/`git_root_cache`, or
can it be layered on top via a new join table?

## Answer

**Rejected git-remote-URL and content-fingerprint approaches (มายด์'s call) —
resolved instead on machine-prefixing the identity, which turns out to need
no schema change at all.**

Verified against the real code (`internal/domain/usage/summary.go`,
`service.go`): `group_by=path`'s aggregation currently keys purely off
`e.Path` (`GroupBy.keyFor`, summary.go:154) while `e.Machine` (the
`install_id`) already exists on every `Event` — that gap alone is the entirety
of bug (a).

- **Storage/aggregation fix:** `keyFor`'s `GroupByPath` case becomes
  `e.Machine + ":" + e.Path` instead of bare `e.Path`. No migration, no new
  column — `machine` is already there on every row.
- **Display fix:** `Service.Summary` already substitutes a machine's raw
  `install_id` for its `hostname` on `group_by=machine` breakdown rows
  (`Key`/`RawKey`, story-1 ticket 18's pattern, `service.go` lines ~127-149).
  Extend that exact substitution pass to `group_by=path`: `Key` becomes
  `<hostname>:<path>`, `RawKey` carries the ground-truth `<install_id>:<path>`
  for the tooltip — same convention, no new pattern.
- **Bug (b) stays unmerged, by design** — the same real project cloned to a
  different local path on a second machine still won't merge automatically.
  Explicitly accepted as this ticket's scope boundary: solving (a) for good
  (false merges are actively misleading) matters more than solving (b) (which
  is only an undercount, visible and explainable). A later display-level
  rollup (matching same trailing path segment/basename across machines) is
  possible future work, not this ticket's or this story's job.
- **Cleanup this enables:** contract.md's existing "same literal path from
  two machines → append machine as a UI-only disambiguator" hack becomes
  unnecessary — two machines' identical raw paths now land as two naturally
  distinct breakdown rows (already labeled by hostname), so that special-case
  collision code can be retired. The same-machine basename-collision
  walk-up-parent-segments rule (contract's other collision case) is untouched.
- **Interaction with ticket 3 (updated after ticket 3's revision):** ticket
  3's dedup check (does this session's `path` equal its own raw launch
  `cwd`?) must run against the raw `path` value alone, before this ticket's
  `machine:` prefix is applied for the group-by key — flagged here so
  whoever implements both together doesn't compare a prefixed key against
  an unprefixed cwd by mistake.
