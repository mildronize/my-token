# Ticket 16 Report

## Ticket

Delete the example `todo` domain entirely from the `my-token` fork
(`internal/domain/todo`, both its transport handlers, its migrations/
queries/openapi content, every registration site, and every stale
reference) — deletion only, per `docs/GETTING-STARTED.md` Steps 8/9,
since ticket 11 built the real `usage` domain independently and never
used `todo` as a template.

## Outcome

done

## Decision

- **Issue: the actual repo had grown `todo` far beyond the ticket's own
  literal file list.** The ticket text describes simple CRUD
  (`title`/`status`/`assigneeId`/`priority`/`dueDate`) with a handful of
  named files. By milestone-4 the real repo had added an event-sourced
  write path (`todo_events`, I15–I19: one write path, append-only,
  owner-only close permission, idempotent-on-`clientRequestId`), a
  `permission.go` policy layer, and a cross-domain "activity" feed
  (`GET /api/bff/activity`, its own OpenAPI schemas, SPA pages, a
  `TimelineEventRow` component) — none of it named in the ticket's
  itemized list.
  **Options considered:** (a) delete only the literally-named files,
  leaving the event/permission/activity machinery in place since the
  ticket didn't name it; (b) delete all of it, since it's still the same
  example domain (`internal/domain/todo/doc.go`'s own comment: "this
  directory is eventually deleted whole") and the ticket's own final bar
  ("zero references to todo/Todo/todos anywhere in internal/ or web/src/")
  is unreachable otherwise.
  **Chosen:** (b). Deleted all of it — domain, both transport handlers
  (including the bff-side `ListActivity`/`toBFFActivityItem` methods
  living inside `todo_handler.go`), migrations, queries, openapi/
  bff-openapi content (`Todo`/`TodoList`/`TodoEvent`/`TodoEventList`/
  `CreateTodoRequest`/`UpdateTodoRequest`/`CreateTodoEventRequest`/
  `ActivityFeed`/`ActivityItem`/`ActivityActor`/`ActivityCursor`/
  `ActivityTodoRef`), and the SPA's `TodoDetailPage`/`ActivityPage`/
  `ActivityList`/`TimelineEventRow`. Retired invariants I15–I19 from
  `.chief/_rules/_contract/INVARIANTS.md` (with a short "retired,
  story-1/ticket-16" note replacing them, numbering gap left
  intentional) and deleted their dedicated architecture test
  (`TestArchitecture_OnlyTodoRepoReferencesTodoEventQueries` +
  `TestI15Floor_CanActuallyFail` + helpers) from
  `internal/architecture_test.go`.

- **Issue: three domain-agnostic test helpers lived inside the doomed
  domain's own files** — `newBFFRouterForOwner` (in the deleted
  `todo_handler_test.go`, used by `users_handler_test.go`,
  `keys_handler_test.go`, `negative_check_test.go`),
  `newBFFRouterForOwnerSharedDB` (in the deleted
  `activity_handler_test.go`, used by `users_handler_test.go` and
  `keys_handler_test.go`), and `doAgentAPIRequest` (same file, used by
  `keys_handler_test.go`) — exactly the trap `docs/GETTING-STARTED.md`'s
  "Two modules, briefly, at once" section warns about (it names
  `bffOwnerID` as the example; these three weren't named there, but are
  the same shape). Also `bffOwnerID` itself and `bffValidationErrorBody`
  (the latter used by `usage_handler.go`, not previously flagged by the
  doc at all).
  **Chosen:** moved all five into neutral shared files
  (`internal/transport/bff/middleware.go` for `bffOwnerID`/
  `bffValidationErrorBody`, next to `ActorFromContext`;
  `internal/transport/bff/bff_testutil_test.go` for the three test
  helpers, next to `newTestRouter`) before deleting their old homes, and
  updated every call site (`newTestRouter`'s signature dropped `todoSvc`;
  `newBFFRouterForOwnerSharedDB` dropped its `*todo.Service` return
  value) — confirmed via `go build`/`go test`, not by inspection.

- **Issue: `internal/dbquery/tableisolation_test.go`'s own fixtures
  piggybacked on the real `"todos"`/`"todo_events"` entries in
  production `TableOwnership`/`ReadOnlyGrants`**, which this ticket
  removes. Four tests (`GrantedReadOnlyReferencePasses`,
  `UngrantedCrossModuleReferenceFails`,
  `GrantIsReadOnlyByMechanismNotJustIntent`,
  `CatchesAnUnusedGrant`) would otherwise fail (two for the wrong
  reason, `require.GreaterOrEqualf`-shaped) or become silently
  vacuous once those table names no longer resolve. Rewrote all four
  (plus two generic regex tests using synthetic `"todos"`/`"widgets"`
  fixture names, swapped for a placeholder) to use `usage_events`/
  `usage` — a table that still really exists — with a new
  `withTemporaryReadOnlyGrant(t, grant)` helper that mutates the
  real package-level `ReadOnlyGrants` for one test and restores it via
  `t.Cleanup`. Flagged during `/chief-review-code`: this mutation is
  safe only because nothing in this package calls `t.Parallel` (true
  repo-wide today) — documented that assumption explicitly in the
  helper's own doc comment rather than leaving it implicit.

- **Issue: `go run ./cmd/agentic` (via `sqlc generate`) corrupted an
  unrelated query file after a routine comment edit.** While removing a
  stale `"todo_events.sql"` reference from `db/queries/api_keys.sql`'s
  own comment, introducing a single em dash (`—`) triggered a known,
  already-regression-tested `bin/sqlc` v1.31.1 bug
  (`internal/db_queries_ascii_test.go`'s `TestDBQueriesFilesAreASCIIOnly`
  exists specifically for this — any non-ASCII byte in a
  `db/queries/*.sql` comment corrupts sqlc's star-expansion byte offsets
  and produces a misleading "mismatched input" parse error at the wrong
  line). Caught immediately by `sqlc generate` failing loudly (would
  also have been caught by `go test` regardless, since the ASCII-only
  check is a real, already-existing regression test — not something
  this ticket added). Fixed by using a plain hyphen instead, matching
  the rest of that file's own convention. Updated that test file's own
  doc comment (it pointed at `todo_events.sql`'s comment for "the
  original, narrower report of this bug" — a now-dead pointer) to cite
  its own package doc instead, and added this incident as a second,
  independently-found repro to that history.

- **Issue: `.claude/skills/my-token-api/`'s agent-facing skill doc
  (`SKILL.md` + `references/endpoints.md`/`references/errors.md`) still
  fully documented the deleted `/todos` surface — 60+ literal
  `todo`/`Todo`/`todos` mentions — and `internal/skill_doc_test.go`
  (inside `internal/`, so technically inside the ticket's own "zero
  references... anywhere in internal/" bar) had two live, passing tests
  that actively *required* the doc to keep describing it
  (`TestDoneWhen6_SkillDocDocumentsTheNewEventEndpoints` asserted the
  doc must contain `/todos/:id/events`, `status_changed`, etc.).
  Initially judged this out of scope (ticket's final verification
  command list only names `go build`/`go test`, and the doc isn't named
  in the ticket's file list) and left it untouched through the first
  full pass. **`/chief-review-code`'s Spec-axis sub-agent disagreed**,
  correctly: a test living in `internal/` mandating stale content puts
  this inside the ticket's own literal bar, not outside it, and the
  doc is agent-facing — an agent calling this fork by following the
  skill instead of reading `openapi.yaml` directly gets real 404s
  (`docs/GETTING-STARTED.md`'s own stated risk for exactly this).
  **Reversed the decision**: rewrote all three doc files to describe
  this fork's real remaining `/api/v1` surface
  (`POST /usage-events/batch`, plus `GET /me`/`GET /keys`/
  `DELETE /keys/:id` unchanged), and replaced the two milestone-4-
  specific `TestDoneWhen6_*` tests (whose whole premise — verifying
  that milestone's own todo-domain doc rewrite — no longer applies)
  with a symmetric pair: `TestSkillDocDoesNotDocumentTheDeletedExampleDomain`
  (absence) and `TestSkillDocDocumentsTheRealDomain` (presence, so a doc
  that deleted the old content and wrote nothing in its place still
  fails). Verified both pass against the rewritten doc.

- **Issue: still out of scope, flagged for a follow-up ticket rather
  than fixed here** — three more consumers of the deleted domain that
  `/chief-review-code` confirmed are reasonable to leave, since the
  ticket's own verification bar (`go build`/`go test`, and "internal/
  or web/src/") doesn't reach them and fixing them is real,
  separate-scope work:
  - `cmd/smoke/main.go` is now functionally broken (every check probes
    `/todos`, which 404s) — no test file covers it, so `go build`/
    `go test` both stay green regardless (`docs/GETTING-STARTED.md`
    Step 10's own explicit warning about this exact trap).
  - `e2e/specs/login.spec.ts` (asserts a "Todos" heading after login)
    and `e2e/specs/assignee-picker-real-click.spec.ts` (drives the
    deleted assignee picker via `/api/bff/todos` and `/todos/:id`) are
    now broken — not covered by `make test`, only by the separate,
    heavier `make e2e`.
  - `web/src/lib/users.ts` (`useUsersQuery`, `assigneeOptions`,
    `UNASSIGNED`) and the backend `internal/transport/bff`
    `UsersServer`/`ListUsers` it calls are now orphaned (their only real
    consumer, the deleted assignee picker, is gone) but contain no
    "todo" text and aren't domain-specific, so left in place rather than
    deleted — flagged in both files' own comments.
  - `web/src/components/Markdown.tsx` is likewise now unused (its only
    consumer, the deleted `TimelineEventRow.tsx`, is gone) but left in
    place for the same reason.

- **Minor, fixed during verification, not flagged for follow-up:** a
  `make build` run during verification (`npm run build`) deleted the
  tracked `web/dist/.gitkeep` placeholder (Vite empties its `outDir`
  before writing, and doesn't regenerate an empty file it didn't
  produce) — restored it from the prior commit before committing, since
  a fresh clone's `go build ./...`/`go test ./...` depend on it existing
  to satisfy `web/embed.go`'s `//go:embed` directive.

## Notes

**Full scope of what was deleted/changed** (`/home/thw-home/gits/my-token`,
commit `c311ccd`, 102 files changed, +927/−12494):

- `internal/domain/todo/` (whole directory: `doc.go`, `permission.go`
  + test, `repo.go` + test, `service.go` + test, `todo_testutil_test.go`).
- `internal/transport/publicapi/todo_handler.go` + test;
  `internal/transport/bff/todo_handler.go` + test (which also carried
  `ListActivity`, the bff-only cross-domain feed handler) +
  `internal/transport/bff/activity_handler_test.go`.
- `db/migrations/20260812190000_create_todos.sql`,
  `db/migrations/20260813100000_todo_activity_log.sql`;
  `db/queries/todos.sql`, `db/queries/todo_events.sql`.
- `openapi.yaml`/`bff-openapi.yaml`: every `/todos*`/`/activity` path
  and every `Todo`/`Activity`-prefixed schema, by hand, before
  `make generate` (which only auto-cleans `internal/db`'s stale sqlc
  output, never openapi.yaml itself, per the doc's own warning).
- `cmd/server/main.go`: `todoSvc`, `*publicapi.TodoServer`,
  `*bff.TodoServer` removed from `buildHandler`/`apiServer`/`bffServer`/
  `wirePublicAPI`/`wireBFF`, all call sites updated.
- `internal/transport/publicapi/publicapi_testutil_test.go`'s
  `compositeServer` (embed + `newIntegrationRouter` composite literal —
  two separate edits, both done).
- `internal/invariants_test.go`'s `perDomainModuleScopePackages` and
  `domainScopePackageNames` maps, `.chief/_rules/_contract/
  INVARIANTS.md`'s I15–I19 (retired) and I3/I4 prose (todo-specific
  carve-outs removed).
- `internal/dbquery/tableisolation.go`'s `TableOwnership`
  (`"todos"`/`"todo_events"` entries removed) and `ReadOnlyGrants`
  (both entries removed — now empty; documented as such, not just
  silently emptied).
- SPA: `TodosPage`/`TodosList`/`TodoRow`/`NewTodoDialog`/
  `TodoDetailPage`/`ActivityPage`/`ActivityList`/`TimelineEventRow`
  (+ every one's own test) and `lib/todos.ts`/`lib/activity.ts` (+
  tests) deleted; `App.tsx` now routes `"/"` to `UsagePage` (previously
  `TodosPage`; the usage console is this app's only real remaining
  screen) alongside `"/usage"`; `Header.tsx`'s nav trimmed to just
  "Usage", logo icon swapped from `ListChecks` to `Gauge`.
- `.claude/skills/my-token-api/` (`SKILL.md` +
  `references/endpoints.md`/`references/errors.md`) rewritten for the
  real `/api/v1` surface; `internal/skill_doc_test.go`'s two
  milestone-4-specific tests replaced with a symmetric absence/presence
  pair for the current domain.
- sqlc query-name collision check (ticket's own item 9): verified no
  collision between `usage_events.sql` and anything `todos.sql` used to
  define — confirmed via `grep -h "^-- name:" db/queries/*.sql | sort |
  uniq -c`, all 13 remaining query names unique.
- Roughly 40 more files touched only for comment-level cleanup (stale
  `"mirrors internal/domain/todo's own X"` cross-references, mostly in
  `internal/identity/*`, `internal/domain/usage/*`, `internal/transport/
  {bff,publicapi}/*`, plus `docker-compose.yml`'s volume comment and
  `db/migrations/20260910120000_create_usage_events.sql`'s own prose)
  — not functional changes.

**Verification:**
- `go build ./...` — green.
- `go vet ./...` — clean.
- `go test ./...` — green, every package (`internal`, `internal/dbquery`,
  `internal/domain/usage`, `internal/identity`, `internal/platform`,
  `internal/transport/bff`, `internal/transport/publicapi`, `cmd/*`).
  Per the doc's own explicit warning, checked this is the command that
  actually caught the stale-reference class of bug: it's what caught
  `internal/architecture_test.go`'s `TestArchitecture_
  OnlyTodoRepoReferencesTodoEventQueries` needing deletion (a floor
  check that would otherwise fail forever at "found 0 functions, want
  ≥3") and `internal/dbquery/tableisolation_test.go`'s four grant tests
  needing their fixtures rewritten — `go build` alone stayed green
  through both.
- `make generate` — clean (sqlc + both oapi-codegen runs), after fixing
  the em-dash/ASCII-corruption incident above.
- `make build` (full production build: `npm run build` + `go build`)
  — green.
- `cd web && npx tsc -b --noEmit` — clean.
- `cd web && npm test` — 4 test files, 21 tests, all passing.
- `gofmt -l` against every changed `.go` file — clean (the bare `make
  fmt-check` run shows spurious `lstat: no such file` errors for files
  this ticket deletes, since `git ls-files` still lists them
  pre-commit; resolved once committed, and confirmed separately with
  the actually-changed-file list).
- Final grep sweep, both required and beyond: zero `todo`/`Todo`/`todos`
  matches (case-insensitive) in `internal/` or `web/src/`, with two
  narrow, deliberate exceptions both already reasoned above:
  `internal/architecture_test.go:73`'s one historical citation of
  milestone-1's real, literal `{internal/todo, internal/identity}`
  hardcoded list, and `internal/skill_doc_test.go`'s own explanatory
  prose plus the literal string `"todo"` the absence-check test itself
  greps for (functionally required, not stale).

Committed: `c311ccd`.
