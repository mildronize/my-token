# 7: Generic actor derivation + cross-machine path identity + path/actor dedup

Type: implementation
Status: resolved
Blocked by: None (can start immediately)

## Delivers

Per the contract's "Cross-machine path identity" and "Path/actor dedup"
sections (both **Supersede** story-1 clauses — see contract.md), plus the
`actor` row of the Data model table:

- **`internal/collector/actor.go`**: remove `crewHomePattern` and its regex
  match entirely. `ActorFromCwd` becomes `func(cwd string) string` (drops
  the `ok bool` return — there is no "unmatched" case anymore, any non-empty
  cwd is a valid actor value). `internal/collector/collector.go`'s
  `actorForSession` keeps its own existing `unknownActor` fallback exactly
  as-is — that fallback already lives at the `FirstCwdBearingRow`-`ok=false`
  level (no cwd-bearing row found at all for a session), not inside
  `ActorFromCwd`, so it needs no change; only the call site's use of
  `ActorFromCwd`'s old two-return signature needs updating.
- **`internal/domain/usage/summary.go`**: `GroupBy.keyFor`'s `GroupByPath`
  case becomes `e.Machine + ":" + e.Path` (was: bare `e.Path`). Add the
  path/actor dedup rule to `Aggregate`: for a session whose `Path` equals
  that session's own launch cwd (or its git-root — same equality test
  `path.go`'s existing `ResolveSessionPath` would produce, computed once per
  session), exclude that event from the `group_by=path` breakdown entirely.
  This exclusion applies before ranking/truncation, so it can never occupy
  a top-N slot a real project path should have.
- **`internal/domain/usage/service.go`**: extend the existing
  `substituteMachineHostnames` pass (today scoped to `GroupByMachine` only)
  to also run for `GroupByPath`: `BreakdownRow.Key` becomes
  `<hostname>:<path>` (through the console's existing `shortenPaths`
  display logic — no display-layer change needed here, just the raw value
  it now receives), `RawKey` carries `<install_id>:<path>` for the tooltip.
- **`web/src/app/usage/BreakdownPanel.tsx`**: retire the machine-as-UI-
  disambiguator behavior story-1's contract described for identical-path
  collisions — no longer needed now that the aggregation key itself is
  machine-prefixed, so two machines' identical raw paths are already two
  distinct rows before display shortening ever runs.

## Verifiable

- Unit test: `ActorFromCwd("/home/thw-home/gits/some-project")` returns
  `"/home/thw-home/gits/some-project"` verbatim — no pattern requirement,
  no special-casing of `.typ-crews`.
- Unit test (table-driven, `Aggregate`): two `Event`s with identical `Path`
  but different `Machine` produce two distinct `group_by=path` breakdown
  rows keyed `machine:path`, not one merged row — the direct regression
  test for the bug this ticket fixes.
- Unit test (table-driven, `Aggregate`): an `Event` whose `Path` equals its
  own session's launch cwd is excluded from `group_by=path`'s breakdown; a
  sibling `Event` in the same session whose `Path` differs from that cwd is
  unaffected.
- Integration/service test: `Service.Summary` with `group_by=path` returns
  `Key`/`RawKey` in the `<hostname>:<path>` / `<install_id>:<path>` shape,
  mirroring the existing `group_by=machine` substitution test.
- `go test ./...` and the frontend suite (`make test`) green.
