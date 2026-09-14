# 15: Rename the Go module path and visible service-name strings

Type: implementation
Status: resolved
Blocked by: None (can start immediately)

## Delivers

Per `docs/GETTING-STARTED.md` Steps 2/3/3b, scoped per the grill decision
(this story is local-only — skip docker-compose naming, deploy-doc polish):

- **Module path** (Step 2): `go.mod`'s `module` line,
  `github.com/mildronize/my-template` → `github.com/mildronize/my-token`
  (or whatever the actual intended module path is — confirm before
  committing if unclear). Update every import — `grep -rl
  'github.com/mildronize/my-template' --include='*.go' .` is the doc's own
  source of truth for what's affected, not a number written anywhere.
  `internal/architecture_test.go`'s one doc-comment mention is harmless
  either way (resolves the module path dynamically at runtime).
- **Service name** (Step 3): `openapi.yaml`'s `info.title`
  (`my-template API` → `my-token API`), the key-path env var and directory
  (`~/.my-template/keys/` → `~/.my-token/keys/`,
  `MY_TEMPLATE_CREW` → `MY_TOKEN_CREW` — both, not just one; per the doc's
  own warning, missing this one specifically means two forks silently
  overwrite each other's key files with no error).
- **`.claude/skills/my-template-api/`** if present — rename to
  `my-token-api` (keep the `-api` suffix; `internal/skill_doc_test.go`
  scans for exactly one `*-api` entry). Check whether this directory
  actually exists in this fork before assuming the step applies.
- **`web/package.json`'s `name`** (Step 3b): `my-template-web` → `my-token-web`.

Explicitly out of scope (grill decision): docker-compose service/image
naming, `DEPLOY-REQUIREMENTS.md` polish, `cmd/smoke` (no deploy planned,
no smoke-test requirement in this story's contract).

Verifiable: `go build ./...` and `go test ./...` both green, `grep -rl
'my-template' --include='*.go' .` empty (excluding doc-comment/historical
mentions that don't affect behavior — judgment call, but the actual
`github.com/mildronize/my-template` import path must be fully gone).
