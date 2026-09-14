# Ticket 12 Report

## Ticket
Collector — scan configured paths (incl. subagent files), `message.id`-deduped
turns, pricing, `actor`/`machine`, simple `cwd`-based `path`, POST to core.

## Outcome
done

## Decision

- **Issue:** Ticket 12's own demo instruction told the builder to scan
  `/home/thw-home/.typ-crews/freya` — freya's crew-home directory. That
  directory holds no transcripts at all; Claude Code writes session
  transcripts to `~/.claude/projects/<slugged-cwd>`, a different location
  entirely (just an encoding of the same underlying path). This was a
  mistake in how I wrote the ticket, not something the builder introduced.
- **Options considered:** (a) accept the builder's substitution — scan the
  real transcript directory
  (`/home/thw-home/.claude/projects/-home-thw-home--typ-crews-freya`)
  instead, which still only contains freya's own sessions (Claude Code names
  each project directory by its own cwd, so this directory structurally
  cannot hold anyone else's data) — preserves the actual safety intent while
  getting real numbers; (b) run the demo literally as written and accept
  zero real rows.
- **Chosen:** (a). The safety constraint was "don't read other crews' data,"
  not "use this exact literal path" — the corrected path satisfies the
  actual intent better than the literal instruction did. Ticket text fixed
  in place so this doesn't get re-discovered by whoever reads it next.
- Separately, the builder noted ticket 11 has no read endpoint yet (that's
  ticket 14), so verification used a direct SQLite query instead of an API
  round-trip — correct, not a real ambiguity, just a sequencing fact.

## Notes

- Real run against freya's actual transcripts: **1735 rows, 1735 distinct
  `message.id`** — confirms ticket 9's dedup fix holds on real data at scale,
  not just the fixture. Single actor (`freya`), single machine, `path`
  fell back to raw cwd for all of them (none of these sessions ran inside a
  git repo) — expected, since this ticket only implements the simple
  `cwd`-based method; ticket 13 is what adds git-root canonicalization's
  real payoff. ~$267.56 local cost estimate (non-authoritative, client-side
  only). Re-run correctly sent only the 1 new turn produced since.
- `chief-review-code` caught two real issues before commit: a hand-rolled
  int→string conversion duplicated across two test files (should be
  `strconv`), and missing `~` expansion for `scan_paths` despite the
  goal/ticket's own documented example (`~/.claude`, `~/.claude-local`) —
  both fixed pre-commit.
- Built at `internal/collector/` (business logic) + `cmd/collector/main.go`
  (entrypoint) in the same `/home/thw-home/gits/my-token` fork. Sent
  `message.id`s tracked in a local JSON state file so re-runs don't resend
  (the server is idempotent regardless — this is an optimization, not a
  correctness requirement).
- Committed: `3c5badc`.
