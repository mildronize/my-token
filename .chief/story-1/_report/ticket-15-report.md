# Ticket 15 Report

## Ticket
Rename the fork's Go module path (`github.com/mildronize/my-template` →
`github.com/mildronize/my-token`) and every visible service-name string
(`openapi.yaml` title, key-path env var + directory, the `-api` skill
directory, `web/package.json`'s name) per `docs/GETTING-STARTED.md`
Steps 2/3/3b, scoped local-only per the grill decision.

## Outcome
done

## Decision

- **Issue:** the ticket's four explicit deliverables (go.mod+imports,
  `openapi.yaml` title, key-path env var+directory, skill dir,
  `web/package.json` name) don't mention `bff-openapi.yaml` (a second,
  structurally identical OpenAPI spec for the BFF surface, added by
  milestone-3 after `docs/GETTING-STARTED.md`'s Step 3 text was last
  written) or `e2e/package.json`/its lockfile (the same kind of `name`
  field as `web/package.json`, in a sibling directory `docs/
  GETTING-STARTED.md` Step 3b doesn't enumerate either). Both still said
  `my-template` after the ticket's literal scope was done, and both were
  independently flagged by the `/chief-review-code` Standards and Spec
  sub-agents as "the exact same stale-name pattern this ticket exists to
  fix."
  **Options considered:** (a) leave them, since the ticket's literal text
  doesn't name them; (b) fix them now, treating them as an oversight in
  the ticket's own enumeration rather than a deliberate exclusion (unlike
  `cmd/smoke`/docker-compose/`DEPLOY-REQUIREMENTS.md`, which the ticket
  names explicitly as out of scope).
  **Chosen:** (b). Renamed `bff-openapi.yaml`'s `info.title` to
  `my-token BFF API` and `e2e/package.json` + `e2e/package-lock.json`'s
  `name` field to `my-token-e2e`. Low-risk, single-line/field string
  changes, no code or test impact (`e2e/` has no Go build dependency and
  isn't exercised by `go build`/`go test`). Flagging for review since it's
  a small widening beyond the ticket's literal text, done to avoid
  shipping a "fork rename" ticket that leaves an identical bug sitting
  right next to the files it explicitly fixed.

- **Issue:** whether `cmd/smoke/main.go`'s own `~/.my-template` key-path
  reference (used only by the smoke script's own key lookup, unrelated to
  its Step-10 domain-literal rename which the ticket explicitly excludes)
  should be renamed along with the rest of the key-path env var/directory
  sweep, since the ticket's own key-path bullet says "search for
  `MY_TEMPLATE_CREW` and `~/.my-template` — rename both occurrences" with
  no file-scoping.
  **Options considered:** (a) rename it too, since it's the same literal
  string the key-path bullet targets; (b) leave it, since the ticket's own
  "Explicitly out of scope" list names `cmd/smoke` without qualification.
  **Chosen:** (b). `cmd/smoke` has zero imports of the module path (build
  unaffected either way) and the ticket's out-of-scope list reads as
  "don't touch `cmd/smoke`" as a whole category, not "don't touch its
  domain literals specifically." Flagged this explicitly to the
  `/chief-review-code` sub-agents; both agreed it's a reasonable scope
  read, not an oversight.

## Notes

**What changed** (`/home/thw-home/gits/my-token`, commit `b400bbe`):

- `go.mod`'s `module` line + every `.go` file's import of the old path
  (54 files touched total, found via `grep -rl 'github.com/mildronize/
  my-template' --include='*.go' .` per the doc's own instruction, not a
  stated count).
- `openapi.yaml` + `bff-openapi.yaml`'s `info.title` → `my-token API` /
  `my-token BFF API`.
- `cmd/issue-key/keyfile.go`, `main.go`, `keyfile_test.go`:
  `MY_TEMPLATE_CREW` → `MY_TOKEN_CREW` and `~/.my-template` →
  `~/.my-token`, everywhere including the embedded shell resolver
  script's own text (usage/error messages, the `CREW=` fallback chain),
  and the `myTemplateBaseDir` Go function renamed to `myTokenBaseDir` for
  consistency.
- `.claude/skills/my-template-api/` → `.claude/skills/my-token-api/`
  (`git mv`, `-api` suffix kept), `SKILL.md`'s front-matter `name:`,
  description, and body key-path references updated to match.
- `web/package.json` + `e2e/package.json`/`e2e/package-lock.json`'s
  `name` fields → `my-token-web` / `my-token-e2e`.

**Verification:**
- `go build ./...` — green.
- `go test ./...` (all packages, including `internal`'s
  `skill_doc_test.go`, which dynamically discovers the `*-api` skill
  directory by suffix rather than a hardcoded name — confirmed it still
  finds exactly one match after the rename) — green.
- `go vet ./...` — clean.
- `grep -rl 'github.com/mildronize/my-template' --include='*.go' .` —
  empty.
- `grep -rn 'my-template' --include='*.go' .` — remaining matches are all
  harmless: doc-comment prose in `internal/skill_doc_test.go` and
  `internal/transport/publicapi/usage_handler.go`/
  `collector_integration_test.go` (historical "ticket 11" mentions);
  unrelated filesystem-path test fixtures in `internal/collector/*_test.go`
  (a real directory on this machine literally named `my-template`, used
  as example data for path-attribution logic — nothing to do with the Go
  module); and `cmd/smoke/main.go`'s explicitly-out-of-scope key-path
  reference (see Decision above).

**`/chief-review-code`** ran two parallel review agents before commit.
**Standards axis:** no ARCHITECTURE.md dependency-rule violations (pure
string substitution, same import graph before/after); no smells
introduced (the `myTemplateBaseDir`→`myTokenBaseDir` rename was called out
as a correct Mysterious-Name fix, not a smell). Flagged `bff-openapi.yaml`
and `e2e/package.json`/lockfile as stale siblings — addressed, see
Decision above. **Spec axis:** confirmed all four ticket deliverables
fully implemented with no gaps and no scope creep (docker-compose,
`DEPLOY-REQUIREMENTS.md`, `cmd/smoke` all absent from the diff); spot-checked
the env-var/directory pairing specifically (the pattern the ticket warns
about) and found both halves renamed together everywhere. Independently
ran `go build`/`go test`/`go vet` and the import-path grep itself,
confirmed green/empty. Same `bff-openapi.yaml` gap flagged as a "minor
observation, not a ticket violation" — addressed anyway per the Decision
above, then re-verified green after.

Committed: `b400bbe`.
