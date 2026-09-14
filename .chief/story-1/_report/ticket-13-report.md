# Ticket 13 Report

## Ticket
Collector — full path-attribution method: extract every real filesystem
path a session's tool calls touched (Read/Write/Edit/Glob/Bash), drop
`/tmp` scratch, git-root each individually, majority-vote by count,
persist `git_root_cache`/`session_path_votes` so re-runs are incremental,
fall back to ticket 12's simple cwd method, then `"(unknown)"`.

## Outcome
done

## Decision

- **Schema deviation (flagged, not silent):** the contract names
  `session_path_votes` as carrying `session_id, git_root, vote_count` plus
  "a `last_scanned_offset` per session." A single offset column keyed only
  by `session_id` has no sane value once a session's transcript spans more
  than one physical file — which ticket 9 established is the common case,
  not an edge case (main transcript + every
  `<session-uuid>/subagents/agent-*.jsonl` beneath it). Built a separate
  `session_scan_progress(session_id, file_path, line_offset)` table
  instead, keyed per file. Documented explicitly in
  `internal/collector/pathstore.go`'s own doc comment as a deliberate
  reading of "per session," not left for someone to rediscover — flag if
  this reading is wrong.
- **`dirForGitRoot`'s file-vs-directory heuristic:** ticket 13 says
  git-root "its directory, or the file's parent directory if the path is
  a file," but doesn't say how to tell the two apart from a bare string.
  Implemented as: real `os.Stat` when the path still exists on this
  machine (production's common case), falling back to a
  has-a-real-extension heuristic on the bare filename when it doesn't
  (already deleted, or a constructed test fixture). Known blind spot:
  a no-extension real file (`Makefile`) that no longer exists on disk
  would be misclassified as a directory — accepted, since the `os.Stat`
  branch handles it correctly whenever the file is still there, which is
  the real-world common case.
- **`/tmp`-scratch exclusion vs. test fixtures using `t.TempDir()`:** the
  new touched-paths integration test initially built its "real, non-scratch"
  temp git repo via `t.TempDir()`, which itself resolves under `/tmp` on
  this host — `isScratchPath` correctly (if inconveniently) dropped it.
  Fixed by building that one fixture repo as a sibling of the test file
  instead (`os.MkdirTemp(".", ...)`, cleaned up via `t.Cleanup`), not by
  weakening the scratch filter.

## Notes

- `chief-review-code`'s two parallel review agents (Standards + Spec) both
  independently caught the same real issue before commit: `path.go`'s
  `ResolveSessionPathFull`/`ResolveSessionPathTouched` were fully
  implemented and unit-tested but never actually called —
  `collector.go`'s `scanSessionPaths` reimplemented the identical
  touched-votes → cwd → `"(unknown)"` fallback chain inline against
  persisted vote counts instead. Fixed by factoring the shared fallback
  logic into one function (`resolvePathFromVoteCounts`) that both
  `ResolveSessionPathFull` (what `path_test.go`'s unit tests exercise
  directly) and `scanSessionPaths` (production wiring, reading persisted
  votes instead of re-tallying fresh) now call — the fallback chain can no
  longer drift between the tested copy and the shipped one.
- **Real-data re-run against freya's own transcripts**
  (`/home/thw-home/.claude/projects/-home-thw-home--typ-crews-freya`, the
  same scope ticket 12 used): 1858 rows across 2 sessions (grew from
  ticket 12's 1735 since more work has happened in this same session).
  Both sessions' `path` now resolve to a real git root
  (`/home/thw-home/gits/my-task`) instead of ticket 12's raw-cwd fallback
  (`/home/thw-home/.typ-crews/freya`) for every row — the touched-paths
  method found real signal ticket 12's simple method structurally
  couldn't see. Vote breakdown for the current build session
  (`2502d73d-...`) shows the exact multi-tree pattern ticket 10 predicted:
  456 votes `my-task`, 352 `my-token` (this ticket's own build work), 124
  freya's own crew home (raw, non-git), plus small counts across
  `.agents`, `my-template`, `.typmem`, and others — `my-task` wins the
  majority vote despite five-plus other real trees being touched in the
  same session, matching ticket 10's own real luna evidence shape. 62
  distinct directories resolved into `git_root_cache` across the run.
  (Demoed via a throwaway `cmd/demoticket13` program, built, run, and
  deleted — never committed, since it exists only to print `path` per
  session for this report, not as ticket deliverable code.)
- Test suite: `go build ./...`, `go vet ./...`, and `go test ./...` all
  pass, including `internal`'s architecture/invariants tests (unaffected —
  `internal/collector` sits outside `ARCHITECTURE.md`'s five dependency
  rules, confirmed by both review agents) and the two new/updated
  `internal/transport/publicapi` integration tests (one real temp git
  repo + real `git rev-parse` subprocess call each).
- Committed: `1485035`.
