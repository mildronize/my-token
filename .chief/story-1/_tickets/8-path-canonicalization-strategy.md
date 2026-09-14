# 8: Which path-canonicalization strategy should my-token actually use?

Type: wayfinder:grilling
Status: resolved
Blocked by: 7 (resolved)

## Question

Given ticket 7's findings, which strategy collapses a subdirectory session
into its project's canonical `path`?

## Answer

**Revised after ticket 7's drift finding** — git-root walk-up is still right,
but not applied per turn:

`path` is computed **once per session**, from that session's **first**
`cwd`-bearing record (the session's actual launch directory, before any
in-session `cd` drift — see ticket 7), then git-root walk-up
(`git rev-parse --show-toplevel`) on that one value, falling back to the raw
first-`cwd` verbatim if it's not inside a git repo. Every turn in that
session gets the same `path`, regardless of any later `cd` inside the
session.

Rejected: canonicalizing per-turn `cwd`. Confirmed directly (ticket 7) that
raw `cwd` drifts within a single session as the shell state changes — using
it per-turn would misattribute usage to whatever directory the shell happened
to be sitting in at that exact moment, including a wholly unrelated repo.

Still rejected: git-root walk-up as a concept isn't wrong (nearly everything
crews work in is a git repo, confirmed on this machine, one well-tested
command, no new config surface) — the fix is *what* it's applied to
(session-start `cwd`, once) not whether to use it at all. Accepted tradeoff
unchanged: a non-git session root can't be collapsed and is tracked as its
own raw-cwd `path`.

**Further revised — see ticket 10.** `cwd` isn't just driftable (this
ticket's original fix), it can be *permanently wrong for the session's whole
life*: a session whose Bash commands `cd` inline per-command (e.g.
`cd /path && git commit ...`) never updates the harness's session-level
`cwd` field at all — confirmed on a real, large, verified example (luna's
work on my-template: 146 real commits, `cwd` pinned to her own crew home in
100% of records, zero drift, yet the actual work is unambiguously there).
`path` must additionally be derived from every real filesystem path touched
by the session's tool calls (`Read`/`Write`/`Edit`/`Glob` targets, plus
absolute paths in `Bash` command strings — at minimum the `cd <path> &&`
prefix), common-directory-reduced, then git-root-canonicalized — falling
back to `cwd`-based attribution only when no tool call yields a real path.
See ticket 10 for the full method and evidence.
