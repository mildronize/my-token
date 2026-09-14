# 2: What are the core dimensions of a usage record?

Type: wayfinder:grilling
Status: resolved
Blocked by: None (can start immediately)

## Question

The source ticket's feature notes pull in two directions — "count at the crew
level only" vs. "the UI should support crew-level filter" plus project-level
tracking by working-directory path. What's the actual data model, and should the
vocabulary be typ-fleet-specific ("crew") or generic (since my-token should make
sense to a non-typ-fleet user too)? Separately: does the design need a third
field for "which machine this ran on," given the source ticket calls out
multi-location install as a design requirement even though this story is
single-machine only?

## Answer

Three independent, first-class fields on every usage record, all generic
(no typ-fleet jargon):

- **`actor`** — the identity dimension (was "crew"). Reuses my-task's own
  `actor` vocabulary rather than coining something new.
- **`path`** — the literal filesystem working-directory path the work happened
  in (was "project"). Named literally so it reads plainly to someone outside
  typ-fleet.
- **`machine`** — which host/install this ran on. Added now even though this
  story is single-machine, because retrofitting it later would mean a schema
  migration plus backfilling every existing record with a guessed value; the
  example collector (`~/tmp/claude-token-tracker`) already captures the
  equivalent of this (`socket.gethostname()`), so it's not new engineering.

Explicitly rejected: naming the identity field "project" (collides with the
path field), and naming it "agent" (works, but "actor" was preferred for fleet
vocabulary consistency).
