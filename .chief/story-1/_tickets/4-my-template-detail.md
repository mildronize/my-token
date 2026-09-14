# 4: What should my-token build on from my-template?

Type: wayfinder:task
Status: resolved
Blocked by: None (can start immediately)

## Question

The source ticket says "i want new service from my-template (main branch) ask
luna for detail." What does my-template actually provide (stack, scaffolding,
conventions), and what specifically should my-token reuse from it?

## Answer

**Correction to the ticket's own phrasing first:** build against my-template's
actual current `main`, not a `main.v2` — PR #11 (huma migration) is still
open/unmerged, so `main` today is oapi-codegen+Gin, not huma.

What my-template (`main`) provides (per luna):
- Go + Gin, two API surfaces in one binary/deploy: `/api/v1` (Bearer API-key,
  agent/script-facing) and `/api/bff` (SSO session, human-facing, serves an
  embedded React SPA).
- SQLite + sqlc (typed queries) + goose migrations.
- Dual auth already built: API-key issuance/verification (`internal/identity`)
  for machine callers, SSO session for a human owner.
- A worked example domain (`internal/domain/todo`) — the repo's own
  Repository-Pattern + Service-layer shape, including an append-only activity
  log (`todo_events`) — the closest existing precedent to "ingest append-only
  usage events."
- `INVARIANTS.md`: I1 (identity never comes from the request body, only the
  resolved credential) and I4 (one seam resolves identity) — relevant if
  per-source attribution should be trustworthy rather than self-reported in a
  payload.
- Error envelope convention, fork checklist (`docs/GETTING-STARTED.md`), deploy
  scaffolding (`docs/DEPLOY-REQUIREMENTS.md`).

What to actually reuse for my-token:
- The two-surface split maps onto my-token's two audiences directly:
  `/api/v1`-equivalent = the ingestion endpoint each install's collector POSTs
  to (API-key authed, no new auth design needed); `/api/bff`-equivalent = what
  the console UI calls (session-authed, human viewing dashboards).
- Copy the `todo` domain module's shape for a new usage/token-events module: a
  `usage_events` migration, one row per turn, keyed by the transcript's own
  uuid, INSERT-OR-IGNORE for idempotent re-ingestion (same dedup idea as
  my-task's own Idempotency-Key convention) — then Repository + Service layer
  over it, same as `todo`.
- Embedded SPA + single-binary deploy already matches the "1 console UI"
  requirement as-is.
- Port the reference collector's pricing table (per-model $/1M-token rates) as
  a pure function in the new domain's business logic — no framework
  dependency, testable standalone.

What's explicitly NOT from my-template — new design needed:
- The actual collector/CLI that runs on each install location and walks local
  transcript files is a new, separate lightweight component — my-template
  doesn't scaffold this. Per ENG-23 pointing at tiana for the multi-ship/shipd
  distribution architecture: "how the collector finds/reports to the central
  console across locations" is her domain (see ticket 5); "what the central
  console's API/DB/UI look like" is the my-template-derived half.
- The `path` field (ticket 2) needs to be an explicit field on the ingestion
  payload, populated by the collector detecting its own working directory —
  nothing in my-template's identity model tracks a caller's working directory
  today.
