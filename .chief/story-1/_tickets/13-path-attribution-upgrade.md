# 13: Collector — full path-attribution method

Type: implementation
Status: resolved
Blocked by: 12

## Delivers

Replaces ticket 12's simple `cwd`-based `path` with ticket 10's full,
verified method:

- Extract every real filesystem path touched by the session's tool calls:
  `Read`/`Write`/`Edit`'s `file_path`/`filePath`, `Glob`'s `path`, and
  absolute paths in `Bash` command strings (at minimum the leading
  `cd <path> &&`/`cd <path>;` pattern).
- Drop `/tmp/...` scratch noise outright, before any git-root attempt.
- Git-root each remaining touched path individually, falling back to the
  raw path itself if genuinely not inside a git repo (not the `/tmp` case —
  a plain non-git directory).
- Take the **most frequent git-root by count** — majority vote, not
  set-intersection (ticket 10's corrected §7 finding: literal common-dir
  collapses to `/` on real multi-tree sessions).
- Fall back to session-`cwd` attribution (ticket 12's method) only when zero
  real candidate paths exist; fall back further to `path = "(unknown)"` if
  even that fails.
- Persist `git_root_cache` (`directory`, `git_root`, `resolved_at`) so a
  directory's git-root is never re-resolved via subprocess once known.
- Persist `session_path_votes` (`session_id`, `git_root`, `vote_count`) with
  a `last_scanned_offset` per session, so re-scans only parse new lines and
  update vote counts incrementally rather than re-deriving from scratch.

Demoable/verifiable on its own: **use a constructed fixture transcript**
that exhibits the same "launch dir ≠ real work dir" pattern ticket 10 found
(a session whose `cwd` never drifts, but whose tool calls' `file_path`/
`Bash` targets consistently point elsewhere) — not luna's real session
files, for the same reason ticket 12 doesn't scan the shared
`~/.claude/projects` directly: this is someone else's real private session
content, and an unattended build has no business reading it just to
construct a test fixture. Confirm `path` correctly reflects the
majority-vote winner instead of the session's launch directory. Contract's
Testing Decisions: unit tests with an injected fake git-root resolver
covering the multi-tree-collision case and both fallback-chain edge cases
(all-scratch, non-git-real-path).
