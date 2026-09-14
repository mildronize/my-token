# 7: How does Claude Code record working directory, and what does that mean for classifying `path`?

Type: wayfinder:research
Status: resolved
Blocked by: None (can start immediately)

## Question

If an agent runs in `~/gits/my-project` in one session and `~/gits/my-project/api`
in another, should `path` treat these as the same project? What does the raw
data actually give us to work with?

## Answer

Two findings, checked directly against this machine's own `~/.claude/projects/`:

1. **The transcript *directory name* is a lossy encoding, not usable as-is.**
   Claude Code names each project's transcript folder by slugging the absolute
   cwd (`/` → `-`). But `-` in a real path segment slugs identically, so
   `my-project-api` and `my-project/api` produce indistinguishable directory
   names — confirmed a real collision on this machine (`-home-thw-home-gits-drawnloop`
   vs. `-home-thw-home-gits-drawnloop-4`; can't tell from the name alone whether
   `4` is a sibling dir or a subpath). The reference collector's own
   `decode_project_path()` has this exact bug baked in.

2. **Each JSONL record instead carries an exact, verbatim `cwd` field** —
   confirmed on a live transcript (`cwd: /home/thw-home/.typ-crews/freya`).
   This is the field to read, not the directory name.

3. **But `cwd` is per-record, not session-stable — it drifts.** Only certain
   record types carry it at all (`assistant`, `user`, `system`, `attachment` —
   not housekeeping types like `mode`/`file-history-snapshot`/etc., which
   don't matter since those never carry `usage` either). Checked a live
   transcript end to end (4285 lines, 3042 carrying `cwd`): 3 distinct `cwd`
   values appear across ONE session, in this order — the session's actual
   root first (line 3), then two unrelated directories later (lines 3628,
   4191) that exactly match two points in that same session where a `cd` was
   run into a temp clone of a different repo entirely, and back. **Per-turn
   `cwd` reflects the harness's live, mutable shell state, not a fixed
   session-level project.** Canonicalizing per-turn `cwd` (even via git-root)
   would misattribute those turns' cost to whatever unrelated repo the shell
   happened to be sitting in at that exact moment — a real bug, not a
   hypothetical one.

   The stable signal instead: the **first** `cwd`-bearing record in a given
   session/transcript file reflects the session's actual launch directory
   (matches the `~/.claude/projects/<slug>` folder's own encoded path, before
   any in-session `cd` drift). `path` should be a session-level attribute —
   computed once from that first value — not recomputed per turn.

But exact `cwd` alone doesn't answer the actual question: a session run from
`~/gits/my-project` and one run from `~/gits/my-project/api` produce two
different (both individually correct) `cwd` values. Treating them as the same
`path` needs a canonicalization step on top of the raw `cwd`, not just reading
it verbatim.

Surveyed approaches:
- **Git-root walk-up** — `git rev-parse --show-toplevel` from the recorded
  `cwd`. Confirmed directly: run from this repo's `.chief/` subdirectory, it
  still resolves to `/home/thw-home/.typ-crews/freya`. Simple, well-tested,
  works for any git repo regardless of depth.
- **Marker-file walk-up** — same idea, generalized to non-git markers
  (`package.json`, `go.mod`, etc.) for the rare non-git working tree.
- **Explicit configured root list** — match `cwd` against a list of known
  project roots (longest-prefix match), config-file driven.
- **No canonicalization at ingestion** — store raw `cwd` as-is; let the
  console group/roll up by prefix at query time instead of collapsing at
  collection time.
- **Prior art already in hand**: the reference collector's `cowork_project_root()`
  solves an analogous problem for Cowork sessions — computing the common
  directory across every file path a session actually touched, snapping only
  through generic containers, never past a real subfolder boundary. Same
  spirit as git-root walk-up, just derived from touched files instead of `.git`.

This is a real design choice with tradeoffs (see ticket 8), not something to
settle unilaterally here — recording the facts, not picking a strategy.
