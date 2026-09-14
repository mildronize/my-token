# 16: Delete the example `todo` domain entirely

Type: implementation
Status: resolved
Blocked by: None (can start immediately)

## Delivers

Per `docs/GETTING-STARTED.md` Steps 8/9 — deletion only, not the
copy-then-modify dance in Steps 1-7 (this story's real domain, `usage`,
was already built directly by ticket 11 without ever touching `todo` as a
template; `todo` is now pure leftover cruft, not a coexisting reference
implementation still needed for anything):

- `rm -rf internal/domain/todo` **and** remove
  `internal/transport/publicapi/todo_handler.go` (+ its test) — both, not
  just the dramatic one.
- Remove its migration (`db/migrations/*_create_todos.sql`), its sqlc
  queries (`db/queries/todos.sql`), and its `openapi.yaml` content (the
  `/todos` paths and `Todo`/`TodoList`/`CreateTodoRequest`/
  `UpdateTodoRequest` schemas) — `make generate` only auto-cleans
  `internal/db`'s stale sqlc output, never `openapi.yaml` itself; that edit
  is manual and must happen *before* re-running `make generate`.
- Remove `todo`'s registration from `cmd/server/main.go` (`buildHandler`'s
  `todoSvc` line, the `*publicapi.TodoServer` field on `apiServer`, its
  entry in `wirePublicAPI`'s composite literal).
- Remove every `TodoServer` reference in
  `internal/transport/publicapi/publicapi_testutil_test.go`'s
  `compositeServer` (the embed AND the `newIntegrationRouter` entry — two
  separate edits, easy to only catch one).
- **`internal/transport/bff/todo_handler.go`** (+ its test,
  `bff_testutil_test.go`'s `todoSvc`/`seedTodo`) — this is the third place
  that imports the domain directly (the session-authenticated
  `/api/bff/todos` surface), easy to miss because it isn't under either of
  the two locations above. Delete it and its `cmd/server/main.go` `wireBFF`
  wiring. Watch for `bffOwnerID` (defined in this file but used by
  `keys_handler.go`/`users_handler.go` — a domain-agnostic helper that
  happens to live in a domain-specific file; move it somewhere neutral
  before deleting the file it currently lives in, don't just delete it).
- SPA side: `web/src/app/{TodosPage,TodosList,TodoRow,NewTodoDialog}.tsx`,
  `web/src/lib/todos.ts`, and their test files
  (`TodosList.test.tsx`, `todos.test.tsx`) — remove entirely, along with
  whatever nav link points at the todos page.
- **Invariants**: `internal/invariants_test.go`'s `perDomainModuleScopePackages`
  (I3/I4) and `domainScopePackageNames` (I15-I19) both likely have a stale
  `"todo"` entry pointing at a now-deleted package — remove those entries
  (this domain never needs those invariants once it doesn't exist).
- **`internal/dbquery/tableisolation.go`'s `TableOwnership` map** — remove
  the `"todos"`/`"todo_events"` entries (and any `ReadOnlyGrants` naming
  `todos.sql`/`todo_events.sql`) — nothing checks this automatically, per
  the doc's own explicit warning; don't rely on the test suite to catch it.
- **sqlc query name collision**: if `usage`'s own query file happens to
  reuse a name `todos.sql` also used (unlikely at this point since `usage`
  was built independently, but check), rename permanently rather than
  planning to revert after deletion.

Verifiable — **both commands, not just the first** (per the doc's own
explicit warning that `go build` passing is not sufficient evidence — a
stale test-only reference like `compositeServer`'s embed only surfaces via
`go test`/`go vet`):

```
go build ./...
go test ./...
```

Zero references to `todo`/`Todo`/`todos` anywhere in `internal/` or
`web/src/` when done (excluding this ticket file's own text and historical
doc/commit-message mentions).
