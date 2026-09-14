# My Token

Track Claude Code token usage and cost across every machine you run it on.

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## About

Most token-usage tools assume one machine and one project. My Token
doesn't: a lightweight collector runs on each machine, reads Claude Code's
own session transcripts, and reports usage to a shared core service — so
usage and cost roll up across every machine you've installed it on, not
just the one you're looking at.

It tracks two things per event, not one:

- **Actor** — *who* did the work (the identity attached to the API key
  that reported it)
- **Path** — *where* the work actually happened (the working directory
  the agent was in, resolved to that directory's git root)

Actor and Path are usually the same story — one person, one project. They
diverge when an agent is asked to work outside its own directory: pointed
at someone else's repo, or running on a shared machine alongside other
agents. My Token itself is generic and doesn't know anything about any
particular multi-agent setup — but that divergent case (an actor working
outside its own path) is why Path exists as its own tracked dimension
instead of being inferred from the actor alone.

## How it works

```
Claude Code transcripts (*.jsonl)
        │
        ▼
  collector (cmd/collector)   — scans configured paths, dedupes by
        │                        message.id, resolves each session's
        │                        git root, attributes actor + path + machine
        ▼
  core service (cmd/server)   — computes cost from a pricing table,
        │                        stores usage_events
        ▼
  web dashboard                — usage by year/lifetime, a machines page,
                                  API key management
```

## Features

- **Two auth paths, one identity model** — SSO session for a human owner,
  long-lived API key (`Authorization: Bearer <key>`) for agents and
  collectors. Both resolve to the same actor concept.
- **OpenAPI-first HTTP layer** — two hand-authored specs:
  - `openapi.yaml` — Bearer-token public API (collector ingestion, key
    management). Go server interfaces only (`oapi-codegen`).
  - `bff-openapi.yaml` — session-authenticated BFF for the web UI.
    Generates *both* Go server interfaces and the frontend's TypeScript
    types (`oapi-codegen` + `openapi-typescript`, same file).
- **Typed, generated DB access** — `sqlc` generates Go from hand-written
  SQL; `goose` manages migrations.
- **A standalone collector** (`cmd/collector`) — recursively scans
  configured directories for Claude Code transcripts, dedupes per-turn
  usage by `message.id`, and POSTs new rows to the core service. Safe to
  re-run: idempotent server-side, and keeps a local cache so a re-run only
  parses new lines.
- **A minimal React SPA** — usage (year/lifetime tabs), a machines page,
  and settings (API key management). Talks to the BFF surface only.

### Tech stack

**Backend**
- Go
- [gin](https://github.com/gin-gonic/gin) — HTTP router
- [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) + `gin-middleware` — OpenAPI codegen + request validation
- [sqlc](https://sqlc.dev/) — typed SQL → Go
- [goose](https://github.com/pressly/goose) — migrations
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) — SQLite driver
- [lestrrat-go/jwx](https://github.com/lestrrat-go/jwx) — JWT/JWKS (SSO)
- `log/slog` + [tint](https://github.com/lmittmann/tint) — logging
- [testify](https://github.com/stretchr/testify) — testing

**Frontend** (`web/`)
- React + Vite
- TanStack Query — data fetching
- React Router
- Radix UI + Tailwind — components/styling
- [openapi-typescript](https://openapi-ts.dev/) — TS types from `bff-openapi.yaml`

## Architecture

Module-first layout, not layer-first: each domain owns its own
repo/service code together, instead of being split across parallel
`handlers/`, `services/`, `repos/` trees.

- Request flow: handler → service → repo
- Only `repo.go` may import the sqlc-generated package — domain types
  stay independent of the DB schema by design, not by convention

```
internal/
  domain/
    usage/       # usage events, pricing, per-actor/machine summaries
  collector/     # the scan-and-report pipeline cmd/collector runs
  identity/      # auth: SSO session handling, API key issuance/verification
  platform/      # config, DB connection, migrations
  transport/
    publicapi/   # Bearer-token surface, for the collector and other agents
    bff/         # session-authenticated surface, for the web SPA
web/             # React SPA (Vite), talks to internal/transport/bff only
db/
  migrations/    # goose
  queries/       # sqlc source
```

## Local development

### Requirements

- Go 1.26+
- Node.js 22+ (only needed to build/embed the web frontend)
- A registered OAuth2/OIDC client for the SSO login path locally

### Setup

```sh
make tools      # installs pinned sqlc/goose/oapi-codegen into ./bin
make generate   # sqlc + oapi-codegen, from db/ and the two openapi specs
make build      # builds the web SPA, then the Go binary (in that order)
make run        # go run ./cmd/server
```

Or with Docker:

```sh
docker compose up
```

SSO is unconfigured by default (`SSO_ISSUER`/`SSO_CLIENT_ID`/
`SSO_CLIENT_SECRET` unset):
- The service runs on API-key auth alone.
- The owner login path shows a "not configured" page until you set them.

### Testing

```sh
make test         # go test ./... + the web test suite
make vet          # go vet ./...
make smoke        # exercises a running instance (does not start one)
```

Browser end-to-end tests live in `e2e/` — its own `package.json`, brings
up a local OIDC issuer and a real instance of the service, tears both
down. See `e2e/README.md`.

## Running the collector

The collector is a separate binary from the server — install it wherever
you want usage tracked, point it at a running core service, and run it
(by hand or on a schedule; it isn't a daemon itself):

```sh
go run ./cmd/collector -config /path/to/collector-config.json
```

Config file (`scan_paths`, `install_id`, `core_url`, `api_key`):

```json
{
  "scan_paths": ["/home/you/projects"],
  "core_url": "https://your-instance.example.com",
  "api_key": "tpl_..."
}
```

- `install_id` is generated and written back to the config file on first
  run if absent — later runs reuse it.
- A key is issued host-side: `go run ./cmd/issue-key <handle>`.
- Safe to re-run on a schedule (cron, systemd timer): dedupes locally, and
  the server is idempotent on each event's own id regardless.

## Docs

| Doc | Covers |
| --- | --- |
| [`docs/GETTING-STARTED.md`](docs/GETTING-STARTED.md) | ⚠️ predates this fork's specialization — still written as a generic fork checklist, due for a rewrite |
| [`docs/DEPLOY-REQUIREMENTS.md`](docs/DEPLOY-REQUIREMENTS.md) | ⚠️ same — what a real deployment needs, not yet updated for this fork |
| [`.claude/skills/my-token-api/SKILL.md`](.claude/skills/my-token-api/SKILL.md) | The `/api/v1` REST surface — auth, endpoints, the collector's own ingestion call |
| [`e2e/README.md`](e2e/README.md) | Running the browser end-to-end suite |

## License

MIT — see [`LICENSE`](LICENSE).
