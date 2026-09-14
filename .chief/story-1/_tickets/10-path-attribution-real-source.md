# 10: Where did my-template's real work actually happen, and why did cwd-based `path` miss it?

Type: wayfinder:research
Status: resolved
Blocked by: 7, 8 (both resolved — this ticket finds a case where their
`cwd`-based model produces a false negative, not just drift)

## Question

`my-template` has 146 real commits / 60,853 insertions, Aug 12 - Sep 8 2026,
all under the shared git identity "Thada Wangthammang" (uninformative per
this fleet's convention). มายด์ insists the work happened on this machine via
luna. But the only two session files directly under
`~/.claude/projects/-home-thw-home-gits-my-template/*.jsonl` are both tiny
Aug-18 no-ops (11 and 9 raw lines, zero commits between them), and a first
pass over luna's own project directory
(`~/.claude/projects/-home-thw-home--typ-crews-luna/`) found her `cwd` field
pinned to `/home/thw-home/.typ-crews/luna` in every record, in all 5 of her
session files, never drifting to my-template. Ticket 8's whole `path` model
is keyed on `cwd` (session-first-value, git-root-canonicalized) — so on that
model this work has no home on this machine at all. Is that actually true, or
is `cwd` blind to something real happening here?

## Answer

**The work is on this machine. `cwd` is blind to it because luna's Bash tool
calls `cd` into `/home/thw-home/gits/my-template` inline, per-command, while
her *session-level* `cwd` field (the harness's own launch directory) never
moves from `/home/thw-home/.typ-crews/luna` — confirmed with exact
timestamp-to-commit matches, not just plausible correlation.**

**1. Luna's Read/Write/Edit/Bash target paths reference my-template
extensively, despite `cwd` never drifting.**

Grepping all 5 of luna's top-level session files for the literal string
`my-template`:

| file | lines | `my-template` hits |
|---|---|---|
| `f510ee4a-...jsonl` | 16399 | 4329 |
| `40222c55-...jsonl` | 1519 | 292 |
| `1734ed04-...jsonl` | 1414 | 98 |
| `3b878c43-...jsonl` | 3290 | 9 |
| `d020321d-...jsonl` | 597 | 8 |

Confirmed separately that **every** `cwd`-bearing record across all 5 files
is `"cwd":"/home/thw-home/.typ-crews/luna"` — no exception, exactly as the
task's premise stated. But `file_path`/`filePath` targets inside those same
records are absolute paths under `/home/thw-home/gits/my-template`, e.g.
(sample from `f510ee4a-...jsonl`):

```
"file_path":"/home/thw-home/gits/my-template/.agents/agents/builder-agent.md"
"file_path":"/home/thw-home/gits/my-template/AGENTS.md"
"file_path":"/home/thw-home/gits/my-template/bff-openapi.yaml"
"file_path":"/home/thw-home/gits/my-template/.chief/milestone-1/_contract/API.md"
"file_path":"/home/thw-home/gits/my-template/.chief/milestone-2/_goal/GOAL.md"
```

Tool-name breakdown of records in `f510ee4a-...jsonl` whose content
references `my-template`: `Bash` 1039, `Edit` 349, `Write` 77, `Read` 320,
`Agent` 42 (delegated subagent spawns — see §3), plus `Skill`/`SendMessage`/
`TaskCreate`/`AskUserQuestion`. This is a session doing real file-editing
work inside my-template, with its shell's *recorded* cwd never reflecting it
because Claude Code's Bash tool doesn't update the session-level `cwd` field
when a command itself contains a `cd` — it only updates `cwd` when the
*harness's own* working directory changes (e.g. a session literally launched
from that directory). Luna's Bash commands do their own `cd` inline instead,
e.g.:

```
cd /home/thw-home/gits/my-template && git commit -m "$(cat <<'EOF'
plan: milestone-1 goal, contract, and architecture standard for TPL-1
...
```
```
cd ~/gits/my-template && git add internal/db_queries_ascii_test.go && git commit -m "$(cat <<'EOF'
docs(internal): state PR #7's removal condition, not just its rationale
...
```

**2. Exact timestamp-to-commit matches — checked against real commit
hashes, not just "plausible":**

| commit hash | git commit timestamp | matching Bash tool_use timestamp (luna, `f510ee4a-...jsonl`) | delta |
|---|---|---|---|
| `b988194` "plan: milestone-1 goal, contract, and architecture standard for TPL-1" | 2026-08-12T17:40:46Z | 2026-08-12T17:40:44.175Z | ~2s |
| `d4ba812` "chore(milestone-1): mark task-2 done, add task-2 report" | 2026-08-12T18:21:38Z | 2026-08-12T18:21:36.192Z | ~2s |
| `f62682f` "chore(milestone-2): mark task-2 done, add task-2 report" | 2026-08-13T02:31:51Z | 2026-08-13T02:31:50.039Z | ~1s |
| `8bd1bfa` "docs(internal): state PR #7's removal condition, not just its rationale" | 2026-08-14T15:43:20Z | 2026-08-14T15:43:18.276Z | ~2s |

Each match is the actual `git commit -m "..."` Bash command whose heredoc
body is byte-identical (checked past the truncated preview above) to that
commit's real message, timestamped 1-2 seconds before git's own commit
timestamp (command-issued-then-committed lag, exactly as expected — the tool
call necessarily precedes the commit it produces). This is a hard link, not
correlation: 88 separate `git commit` invocations found in
`f510ee4a-...jsonl` alone (22 more in `3b878c43-...jsonl`, 2 each in the
other two files with commits), spanning the *whole* Aug 12 - Sep 8 commit
range of the real repo (`f510ee4a-...jsonl` alone runs 2026-08-10 to
2026-09-02; luna's 5 files together span 2026-08-10 through 2026-09-09,
fully covering my-template's 2026-08-12 - 2026-09-08 commit history with no
gap).

**3. Delegated subagent work also targets my-template, and also never
drifts `cwd`:** `f510ee4a-...jsonl` and `40222c55-...jsonl` together have 60
subagent transcripts under `<session>/subagents/agent-*.jsonl` (per ticket
9's structural finding); 58 of those 60 reference `my-template`. Every
subagent file's `cwd` is also pinned to
`/home/thw-home/.typ-crews/luna` — never their own value, consistent with
ticket 9's note that subagent files inherit the parent's `cwd`/`path`. Sample
`meta.json`: `{"agentType":"general-purpose","description":"Build task-7:
fix second blind-fork-test findings",...}` — matches a real commit message
family (`8e6327c "chore(milestone-1): record task-7 — fix findings from
second blind fork test"`).

**4. Broadened search: no other crew's session files reference
`my-template` at all.** Grepped every crew home under
`~/.claude/projects/-home-thw-home--typ-crews-*/*.jsonl` (23 crew
directories) for the literal string `my-template`: zero hits outside luna's.
This machine's real my-template work is attributable specifically to luna,
not "some other crew" — the task's hedge in point 3 doesn't apply here.

**5. The two tiny direct-cwd session files are confirmed unrelated noise,**
not luna under a different guise: their session UUIDs
(`5057e847-...`, `c3696973-...`) don't appear anywhere in luna's 5 files or
her subagent trees, and (per ticket 9) they're a bare "hi" and an empty
plan-mode exit — not associated with luna's crew-home-anchored, `cd`-per-command
workflow at all.

**Conclusion: this machine genuinely holds the real work.** `cwd`-based
`path` (tickets 7/8) misses it not because the work is elsewhere, but because
it measures the wrong signal for a session whose shell state the harness
never re-observes after launch, while the actual commands inside that shell
freely `cd` elsewhere. This is a distinct failure mode from ticket 7's
already-known "cwd drifts mid-session" bug: here `cwd` doesn't drift *at
all* — because it's never given the chance to. Bash's inline `cd` never
touches the field that would need to change.

**6. Corrected `path`-attribution method — files-touched, not `cwd`:**

`cwd` (first-value or otherwise) cannot be the *sole* signal. The reference
collector's `cowork_project_root()`
(`~/tmp/claude-token-tracker/scan_claude_usage.py:261-279`, unzipped and read
in full) already solves the analogous problem for Cowork-mode sessions,
where the sandbox's own cwd is never the real project either: it collects
every real on-disk path a session actually touched
(`_real_paths_in_record()`, scraping `toolUseResult.filePath`/`.file.filePath`/
`.filenames` and `attachment.attachment.{path,displayPath}`, explicitly
skipping sandbox-marker paths), takes their longest common directory
(`_common_dir()` — plain per-component prefix matching, case-insensitive),
and only "de-collapses" back up to a named project segment when that common
dir sits under a known generic container (`_trim_project_root()`); otherwise
the common dir *is* the project, no matter how deep.

For Claude Code (non-Cowork) sessions specifically, the adapted method:

- Per session (main file's records, plus every `subagents/agent-*.jsonl`
  under it — ticket 9's scan scope), collect every real path from
  `tool_use` records targeting the filesystem: `Read`/`Write`/`Edit`'s
  `file_path`/`filePath` input, `Glob`'s `path` input, and — the case this
  ticket is built on — **absolute paths embedded in `Bash`'s `command`
  string** (at minimum, the `cd <path> &&` / `cd <path>;` prefix pattern
  luna's commands use; a fuller pass could also parse other absolute-path
  arguments inside the command, e.g. file paths passed to `git -C <path>`,
  but the leading `cd` is the dominant, reliably-parseable case seen here —
  88+22+2+2 = 114 of luna's own `git commit` invocations alone use it).
- Take the common directory across all of them (same `_common_dir()`-style
  longest-shared-prefix logic, POSIX `/`-separated instead of `\`).
- Git-root-canonicalize *that* common directory (ticket 8's existing
  git-root walk-up step), not the raw `cwd`.
- **Fall back to `cwd`-based attribution (tickets 7/8, unchanged) only when
  no tool call yields a real touched path** — i.e. keep the existing model as
  the default/simple case (it's still correct for the common case where a
  session actually is launched inside the project it works on — matches
  every crew's project directory *except* luna's my-template pattern), and
  layer the touched-paths signal on top specifically to catch a session like
  luna's where the shell's `cd`s never surface through `cwd` at all.
- This does **not** invalidate tickets 7/8's `cwd`-drift finding or ticket
  9's counting-unit finding — it adds a second, complementary signal for a
  case those tickets' `cwd`-only model structurally cannot see (a session
  whose declared cwd and worked-on directory are different, and stay
  different, for the session's entire life, rather than drifting between
  them mid-session).
- One-line implication for the collector plan (ticket 4): the `path`
  extraction step needs read access to each record's `tool_use.input`
  fields and raw `Bash` command strings, not just the `cwd` field — a
  slightly larger per-record parse than tickets 7/8 assumed, but no new
  data source; it's already sitting in the same JSONL lines.

**7. Correction to §6's aggregation step, verified against real data:**
literal common-directory reduction across *every* touched path (as
described above, copying the Cowork `_common_dir()` pattern directly) was
tried against luna's real `f510ee4a-...jsonl` and **collapses to `/`** —
because one real session legitimately touches several unrelated trees at
once (her own crew notes at `/home/thw-home/.typ-crews/luna`, shared fleet
memory at `/home/thw-home/.typmem`, even a second unrelated repo
`/home/thw-home/gits/my-task`), not just the one it's "about." Cowork's
version gets away with pure intersection because its sessions are
sandbox-scoped to one project already; a general crew session is not.

Corrected aggregation: git-root **each** touched path individually (not the
literal path — the path after canonicalization), then take the **most
frequent git-root by count** — majority vote, not set-intersection. Verified
on the same real file (1545 absolute, non-scratch touched paths across
1607 raw candidates, 62 dropped as `/tmp/...` scratch noise):

```
1465  /home/thw-home/gits/my-template     <- winner, 94.8%
  33  /home/thw-home/.typ-crews/luna
  13  /home/thw-home/gits/my-task
   9  /home/thw-home/.typmem
   7  /home/thw-home/gits/prod-thw-home
```

Correctly and robustly picks `my-template` despite four other real
(non-scratch) trees being touched in the same session. This is the final
form of the method: extract candidate paths → drop scratch/tmp → git-root
each individually → majority vote by count → that's `path`; fall back to
`cwd`-based attribution (tickets 7/8) only when zero real candidate paths
exist.

**8. Caching/persistence, so a collector doesn't rescan everything on every
run.** Two different things, cached separately:

- **Directory → git-root mapping** — stable, rarely changes. Persist as
  `git_root_cache(directory, git_root, resolved_at)` in the same local
  SQLite DB the collector already needs (ticket 4/9). Once a directory
  resolves once, it never needs another `git rev-parse` subprocess call —
  reused across every future session that touches anything under it, not
  just within one session. Measured: 1545 touched paths in one session
  reduce to only 75 unique directories before any git call, and with this
  cache the number of *actual* subprocess calls across a session's life
  approaches the number of distinct repos touched (5, here), not 75.
- **Per-session running vote tally** — needs incremental updates, not full
  rescans. Persist as `session_path_votes(session_id, git_root,
  vote_count)`. Track how far a session's transcript has already been
  scanned (same idea as ticket 9's `message.id` dedup, applied to line
  position instead), and on each periodic run only parse *new* lines since
  last scan, incrementing vote counts for whatever git-roots those new
  lines touch. `path` for that session is a cheap read — whichever
  `git_root` has the highest `vote_count` — not a recomputation from raw
  transcript data. This also means an ongoing session's `path` stays
  correctly up to date as new tool calls come in.

**9. What happens when a touched path never resolves to a git root** (e.g.
`/tmp`) — three fallback layers, in order:

1. **Known scratch patterns (`/tmp/...`) are excluded outright, before
   git-rooting is even attempted** — a deny-list check, not a "no git root"
   case. Even if something under `/tmp` happened to have a `.git` in it, it
   still shouldn't count; scratch space is never "the project."
2. **A real, non-scratch path where git-root genuinely fails** (a plain
   non-git directory — a data dir, an un-versioned install location, etc.):
   falls back to **the path itself** as its own root for voting purposes —
   the same single-value fallback rule ticket 8 already established for
   `cwd` ("fall back to raw `cwd` verbatim if not inside a git repo"),
   applied per touched-path instead of only to `cwd`. It can still win the
   majority vote as a raw, non-canonicalized path.
3. **Nothing real survives at all** (every touched path was scratch, or
   there were no filesystem-touching tool calls in the session): falls back
   to session-level `cwd` attribution (tickets 7/8, unchanged) — first
   `cwd`, git-rooted, or raw `cwd` if that's not git either.

**Final fallback, if even that produces nothing usable:** `path =
"(unknown)"`, an explicit sentinel bucket. Token usage still happened and
still costs money — it must never be silently dropped just because
attribution failed. Aggregate totals stay correct even when per-session
`path` attribution genuinely can't be determined.
