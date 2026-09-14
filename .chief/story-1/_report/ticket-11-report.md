# Ticket 11 Report

## Ticket
Core service — `usage_events` schema + `/api/v1/usage-events/batch` ingestion
endpoint, idempotent on `message.id`.

## Outcome
done

## Decision

- **Issue:** Should a real GitHub repo (`mildronize/my-token`) be created now
  for the fork, or should the build stay local-only?
- **Options considered:** (a) create the repo now via `gh repo create`, so
  the fork has a real remote from the start; (b) stay local-only
  (`/home/thw-home/gits/my-token`, `upstream` pointing at the real
  `my-template`, no `origin` yet) until a later ticket/story actually needs
  one.
- **Chosen:** (b), local-only. The goal document itself already settles
  this — this story explicitly puts "deploying the core service/console
  anywhere externally reachable" out of scope, and never requires a hosted
  or tracked repo; it's proving the collector → core → console architecture
  locally, on one machine. Creating a real, externally-visible GitHub repo
  is a semi-irreversible action with no requirement behind it right now.
  Decided directly from the goal's own scope rather than spawning a
  decision-support agent — the goal document already answered this clearly
  enough that generating alternative options wouldn't have added anything.

## Notes

- Fork lives at `/home/thw-home/gits/my-token`, cloned from my-template's
  real `main` (2dc2dd3 — confirmed Gin/oapi-codegen, not `main.v2`'s
  unmerged huma migration, per ticket 4). `upstream` remote points at
  `github.com/mildronize/my-template.git`.
- `usage_events` (goose migration), `internal/domain/usage/`
  (Repository+Service, shaped like `internal/domain/todo`, idempotent
  `INSERT OR IGNORE` on `id`), a ported server-side pricing table (pure
  function, table-driven tests), and `POST /api/v1/usage-events/batch`
  (reuses existing Bearer API-key middleware, untouched) are all in place.
  `cost`/`source` have no client-facing schema fields at all
  (`additionalProperties: false`) — a client sending them gets
  `validation_error`, never a silent override.
- Response shape is `{received, inserted}` — not specified in the contract,
  added so the ticket's own acceptance test (repeated `id` → exactly one row)
  is checkable via the API response itself, not just a direct DB query.
  Accepted as a reasonable in-scope judgment call; cheap to change later if
  a different shape is wanted.
- Fixed a stale cross-reference found in ticket 12 (it cited "ticket 13" for
  the pricing table; ticket 13 is path-attribution, not pricing) — corrected
  in place, and noted that ticket 11 already has its own server-side copy of
  the pricing table, since the server (not the client) is authoritative for
  `cost`.
- My-template's full fork-rename checklist (module path, service name,
  Hydra client, key-path env var) was **not** run — correctly out of this
  ticket's scope per the builder's own judgment. Left for whichever ticket
  actually productionizes this fork; worth a reminder at that point, not now.
- Tests: repo-layer, service-layer (fake repo), HTTP integration including
  the literal acceptance test (repeated `id` in one batch → one row; a
  resend across separate calls inserts nothing new). `go build`, `go vet`,
  `gofmt -l`, and the repo's own architecture/invariant guardrail tests are
  all clean.
