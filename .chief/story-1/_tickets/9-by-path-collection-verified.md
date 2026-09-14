# 9: How does "by path" usage collection actually work, verified — not hand-computed inline?

Type: wayfinder:research
Status: resolved
Blocked by: 7, 8 (both resolved, but their turn/token counting was never
independently verified — this ticket exists because it should have been a
proper research pass from the start, not ad hoc scripts in the main thread)

## Question

Reliably determine, from Claude Code session transcript files under
`~/.claude/projects/*/`, how to correctly count usage-bearing turns and their
tokens per session, so that "by path" aggregation (ticket 8's model: one
`path` per session, summed across sessions) produces numbers that are
actually correct — not just plausible-looking.

Specific gaps my own inline attempt did not verify:
- Whether subagent/sidechain transcript files (separate `.jsonl` files
  alongside the main session file, per the reference collector's own
  handling) need to be included and were being counted correctly.
- Whether a `memory/` subdirectory (found sitting inside a project's
  transcript folder) is something that needs handling, or is unrelated noise.
- Whether the turn/token counts my inline script produced for a real session
  are actually correct, cross-checked against another independent method,
  not just "the script ran without an error."

## Answer

The inline script's numbers were wrong. Verified against the raw files, cause
identified, and it is NOT a plausible ratio for a real session — it's a
counting bug, plus one real, previously-unknown structural fact this
verification pass turned up.

**1. Manual line-by-line read of both `my-template` files:**

`5057e847-...jsonl` (11 lines): lines 1-6 are housekeeping/attachments (no
`usage`). Lines 7 and 8 both have `"message":{"role":"assistant", "id":
"gen-1787067973-4fAGaRgR3i3Vq4VgugSZ", ...}` — **the same message id**, one
carrying a `thinking` content block, the other the final `text` block. Both
carry an **identical** `usage` dict: `input_tokens:33452,
cache_creation_input_tokens:0, cache_read_input_tokens:0, output_tokens:34`.
Lines 9-11 are housekeeping. So this file has 2 raw usage-bearing *lines* but
only **1 real LLM call**.

`c3696973-...jsonl` (9 lines): user "hi", agent/skill-listing attachments, a
`plan_mode_exit` attachment, `last-prompt`. **Zero** lines with
`message.usage` — no assistant reply was ever generated in this session.

Naive per-line summation gives `input=33452*2=66904, output=34*2=68` across
the two files — exactly what the inline script reported. **That is not two
turns, it's one turn counted twice.** The correct total for these two files
is **1 turn, input=33452, output=34, cache_read=0, cache_creation=0.**

**2. The `memory/` subdirectory:** empty (`stat` confirms 0 entries, mtime ==
session-creation time). It is noise, and it's irrelevant for a structural
reason, not a special-cased one: any glob restricted to `*.jsonl` skips it
automatically since it holds nothing at all, let alone a `.jsonl` file. No
exclusion logic needed beyond "match `*.jsonl`".

**3. Subagent transcripts ARE separate files — confirmed directly, and it's
not where ticket 9 guessed.** No `isSidechain:true` record exists anywhere in
either `my-template` file or in 86 checked `.jsonl` files project-wide — every
top-level record under `~/.claude/projects/*/*.jsonl` is `isSidechain:false`.
But subagent transcripts do exist on this machine as **separate files nested
one level deeper**: `<project>/<main-session-uuid>/subagents/agent-<hash>.jsonl`
(with a sibling `<hash>.meta.json` carrying `agentType`, `description`,
`toolUseId`, `spawnDepth`), plus a `<main-session-uuid>/tool-results/*.txt`
folder for large raw tool output (not `.jsonl`, not usage-bearing, ignorable).
Caught this live: this very research task's own subagent transcript is
sitting at
`.../-home-thw-home--typ-crews-freya/2502d73d-.../subagents/agent-a85192ba6a15e61e9.jsonl`,
`meta.json` reading `{"description":"Research by-path usage collection
correctness",...}` — every one of its 98 records is `isSidechain:true`, 46
carry `usage`. A found third example (`typ-fleet`, `nicole` projects) confirms
this is the general layout, not a fluke.
Consequence: a scan that only reads `<project>/*.jsonl` (non-recursive)
**undercounts** — it silently drops all subagent usage. A recursive walk
(`rglob("*.jsonl")`, as the reference `claude-token-tracker`'s `walk_jsonl`
does) picks these up correctly, and does **not** double-count them: the main
file's own record only carries the small parent-turn cost of *issuing* the
Task/Agent tool call, never a copy of the subagent's internal usage.
Subagent files carry the parent session's own first `cwd` (verified — matches
exactly), so they should inherit the parent session's `path`, not compute
their own.

**4. The 66904:68 ratio is not a real ratio, and it's not a
cache-token mix-up either — checked and ruled out.** `cache_read_input_tokens`
and `cache_creation_input_tokens` are both `0` on this record, so the
"cache tokens miscounted as input" hypothesis doesn't apply here. It's
exactly a 2x duplicate: one real API call's usage dict, written to two
JSONL lines (one per content block: `thinking` + `text`), summed as if it
were two calls. The *actual* per-call ratio, 33452 input : 34 output, is
completely ordinary for a `hi` prompt that pulled in a huge system context
(this session's own agent-listing + skill-listing attachments, ~33K tokens
worth, both loaded before the model ever answered) and got a two-word reply.
Scaled up: the real `freya` session file
(`87e1d4f3-...jsonl`, 4285 lines) has **970 raw usage-bearing lines but only
581 unique `message.id` values** — 241 ids appear 2-5 times each (one per
tool_use/text content block in a multi-block turn), all with byte-identical
`usage`. Naive per-line summation there inflates the true totals by **1.67x**
(measured: naive input=1798 vs correct=1090; naive output=951228 vs
correct=515812; naive cache_read=448.8M vs correct=272.0M) — the same bug,
just far larger blast radius on a real multi-tool-call session than on the
2-turn toy example.

Notably, this exact bug is **already present in the reference collector**
(`claude-token-tracker`'s `scan_claude_usage.py`, found at
`~/tmp/claude-token-tracker/`): `_insert_record()` dedupes on the JSONL
record's own top-level `uuid` (`INSERT OR IGNORE ... PRIMARY KEY(uuid)`), and
each content-block record has its own distinct `uuid` even when it shares one
`message.id` — so that `uuid`-based PRIMARY KEY does *not* collapse the
duplicate lines; it only protects against re-ingesting the same line twice on
a rescan. It is not a safe template to copy verbatim for this specific
concern.

**5. Corrected counting method:**

- **Which files to scan:** recursively glob `<project-dir>/**/*.jsonl` (not
  just the top-level two files) — this is required to pick up
  `<session-uuid>/subagents/agent-*.jsonl`, not optional. Non-`.jsonl`
  content (`memory/`, `tool-results/*.txt`) is naturally skipped by the
  extension filter; no special-case exclusion list is needed.
- **What counts as one "turn":** one unique `message.id` (the field inside
  the record's `"message"` object, e.g. `gen-...` or `msg_...`) that carries a
  `usage` dict — **not** one per JSONL line/record `uuid`. A single LLM call
  can be split across several lines (one per content block: `thinking`,
  `text`, `tool_use`, ...), all sharing one `message.id` and one byte-identical
  `usage` snapshot for that call. Group by `message.id` first, take the
  `usage` dict once per group (they're identical across the group, so first
  or last both work), then sum.
- **Tokens per turn:** read `input_tokens`, `output_tokens`,
  `cache_read_input_tokens`, `cache_creation_input_tokens` directly off that
  one deduped `usage` dict per turn — do not add cache fields into
  `input_tokens` (they're billed at different rates; keep them as separate
  columns, matching how the reference collector's own `PRICING` table already
  treats them).
- **Session vs subagent attribution:** every `.jsonl` found under a given
  `<session-uuid>/` tree (the main file plus its `subagents/agent-*.jsonl`
  files) belongs to that one session and inherits that session's `path`
  (ticket 8's model: first `cwd`-bearing record of the *main* file,
  git-root-canonicalized) — do not compute `path` per subagent file from its
  own first `cwd`, even though empirically it currently matches.
- **Verified corrected totals for the two real `my-template` files:** turns=1,
  input=33452, output=34, cache_read=0, cache_creation=0 (not
  turns=2/input=66904/output=68 as originally reported).
