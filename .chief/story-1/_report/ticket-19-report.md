# Ticket 19 Report

## Ticket

Add a fixture-based test proving the collector's scan+actor-resolution
pipeline correctly handles a scan root containing multiple distinct
crew-home-shaped subdirectories in one pass — this exact scenario was
only ever checked by hand this session, letting a stale config go
unnoticed for a full round-trip.

## Outcome

done

## Notes

**What was built** (commit `706f8c6`, 1 file, +51):

- `internal/collector/collector_test.go` —
  `TestRun_MultipleCrewHomesUnderOneScanRoot_AllFoundAndDistinctlyAttributed`.
  Builds a `t.TempDir()` root with 3 fake crew-home-shaped subdirectories
  (`<root>/.claude/projects/-home-x-.typ-crews-alice/session-alice.jsonl`,
  same shape for `bob`/`carol`), mirroring the real Claude Code
  project-directory naming convention (cwd with `/` replaced by `-`).
  Each fixture has one real-shaped, usage-bearing JSONL line via the
  package's existing `usageLine` helper, with `cwd` set to
  `/home/x/.typ-crews/<crew>` so `ActorFromCwd` has something real to
  resolve.

**Seam chosen:** the full `Run()` pipeline (scan -> extract -> attribute
-> post), not `FindTranscriptFiles` called standalone. The ticket
explicitly licensed this ("not the full `Run` pipeline necessarily —
whichever seam actually needs this coverage"), and `Run()` is the exact
seam collector_test.go's other `TestRun_*` tests already use for
full-pipeline coverage (e.g. `TestRun_TouchedPathsMajorityVoteBeatsSessionCwd`),
so this follows the package's own established convention rather than
introducing a new test shape. Using `Run()` also means the test proves
the actual thing that broke in production — a scan-root config value
fed into the real pipeline — not just one internal function in
isolation.

**Assertions:**
- `result.FilesScanned == 3` — all 3 fixture files found, not just the
  first alphabetically or just one.
- `poster.posted` has all 3 sessions, each mapped to its own actor via
  `actorBySession`, individually checked against `"alice"`/`"bob"`/`"carol"`.
- A `seenActors` distinctness loop, asserting no actor value repeats
  across the 3 sessions and that exactly 3 distinct actors resulted —
  directly mirrors the contract/ticket's own explicit wording ("not
  collapsed into one... not just the first one repeated") as its own
  traceable assertion.

**Verification (step 5's own bar — actually performed, not just
described):**
1. Ran the new test as written (full multi-crew root) — passed.
2. Temporarily edited `cfg.ScanPaths` to point only at alice's
   subdirectory (`filepath.Join(root, ".claude", "projects",
   "-home-x-.typ-crews-alice")`), re-ran the test:
   ```
   Error: Not equal:
     expected: 3
     actual  : 1
   Messages: must find all 3 crews' transcript files under the one
             scan root, not just the first alphabetically or just one
   --- FAIL
   ```
   Confirms the test genuinely would have caught this session's real
   mistake — it fails loudly and specifically, not silently passing.
3. Reverted `cfg.ScanPaths` back to the full temp root, re-ran — passed
   again. The reverted state is what's committed; the narrow-and-fail
   step was never committed.

**Full verification:**
- `go build ./...` — clean.
- `go vet ./...` — clean.
- `go test ./...` — all packages green, including the new test.
- `gofmt -l internal/collector/collector_test.go` — clean.
- `git diff --stat` confirmed the change is scoped to exactly one file.

**`/chief-review-code` (Standards + Spec axes, parallel sub-agents):**
- Spec axis: zero missing/partial requirements, zero scope creep (using
  `Run()` over standalone `FindTranscriptFiles` explicitly licensed by
  the ticket's own text), zero implementation bugs — independently
  confirmed the assertion structure is regression-proof by tracing
  `FindTranscriptFiles`'s `filepath.Walk` recursion and `ActorFromCwd`'s
  regex against the real source.
- Standards axis: zero hard violations (`ARCHITECTURE.md`'s five
  dependency rules are scoped to `internal/domain/*`, `internal/identity`,
  `internal/transport/*`, `internal/platform` — confirmed `internal/collector`
  sits outside that layering by checking `internal/architecture_test.go`
  never mentions `collector`). One minor judgement-call note (mild
  Duplicated-Code smell: the `seenActors` distinctness loop re-proves
  what the three literal `assert.Equal` calls already guarantee).
  **Kept as-is** — it directly mirrors the ticket's own explicit
  acceptance wording as its own traceable assertion, a deliberate
  belt-and-suspenders choice rather than an oversight.
