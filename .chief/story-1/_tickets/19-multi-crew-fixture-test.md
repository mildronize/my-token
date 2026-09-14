# 19: Fixture test for multi-crew scanning

Type: implementation
Status: resolved
Blocked by: None (can start immediately)

## Delivers

A fixture-based test proving `FindTranscriptFiles`/the collector's scan
step correctly handles a scan root containing multiple distinct
crew-home-shaped subdirectories — not verified by any existing test.
This exact scenario (many crews under one scan root) was previously only
checked by hand against real production data this session, which caused
real friction: a stale config file went unnoticed for a full round-trip
because nothing caught it automatically.

- Build a temp directory tree with 2-3 fake crew-home-shaped subdirectories
  (e.g. `<tmp>/.claude/projects/-home-x-.typ-crews-alice/`,
  `.../-home-x-.typ-crews-bob/`, each with its own small fixture
  `*.jsonl` transcript, distinct `actor`-resolvable cwd patterns, and at
  least one real usage-bearing turn).
- Run the collector's scan+extract pipeline against the temp root (not the
  full `Run` pipeline necessarily — whichever seam actually needs this
  coverage; `FindTranscriptFiles` + the per-session actor/path resolution
  is the specific thing that was never exercised this way).
- Assert: all fixture files are found (not just the first alphabetically,
  or just one), and each session's `actor` resolves to its own distinct
  crew name, not collapsed into one.

Verifiable: the new test fails if the scan root argument is accidentally
narrowed to a single subdirectory (confirm this by temporarily narrowing
it and checking the test catches it, then revert) — proving it actually
would have caught this session's real mistake, not just exercising the
happy path.
