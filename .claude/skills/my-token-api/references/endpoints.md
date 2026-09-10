# `/api/v1` endpoints

Derived from `openapi.yaml` (repo root) and `.chief/_rules/_contract/
API.md` — where a fork's actual code and this file might ever disagree,
`openapi.yaml` is the source of truth, since every request is validated
against it before a handler runs at all (see "Request validation" below).

**`/usage-events/batch` below is this fork's own real domain
(story-1/ticket-11), not a template placeholder** — `GET /me`, `GET
/keys`, and `DELETE /keys/:id` are identity, unrelated to it.

Every path below is relative to `$BASE_URL`. Every request carries
`Authorization: Bearer <credential>`.

---

## `GET /me`

No parameters. Confirms the key and states who it belongs to.

```jsonc
{ "handle": "luna", "role": "agent", "active": true }
```

`role` is always `agent` in practice — an owner's credential is refused
with 401 on this whole surface (I2), so a successful response can never
carry `owner`.

---

## `POST /usage-events/batch` → **201**

story-1/ticket-11's core ingestion endpoint — a running collector
installation POSTs new events since its last successful send. Idempotent
on each event's own `id` (`message.id`, ticket 9's dedup key — never the
source JSONL record's own `uuid`): a resend of an already-ingested id
changes nothing.

```jsonc
{
  "install_id": "6f1e2a3b-…",   // the collector's own install_id (UUID)
  "hostname": "thw-home",
  "events": [
    {
      "id": "msg_01AbC…",                    // message.id — the idempotency key
      "session_id": "sess_…",
      "actor": "freya",                      // which crew-home produced this session
      "path": "/home/thw-home/gits/my-token",
      "machine": "6f1e2a3b-…",                // = install_id
      "model": "claude-sonnet-4-5-20250929",
      "input_tokens": 1200,
      "output_tokens": 340,
      "cache_read_input_tokens": 800,
      "cache_creation_input_tokens": 0,
      "timestamp": "2026-09-10T09:00:00Z"     // when the turn actually happened
    }
  ]
}
```

`cost` and `source` have no properties on `UsageEventInput` at all
(`additionalProperties: false`) — sending either is a `400
validation_error`, not silently dropped. The server computes `cost`
itself (the ported pricing table) and always sets `source:
"claude_code"`. `install_id`/`hostname` identify the reporting collector
for observability only; they are not threaded into the stored event rows.

```jsonc
{ "received": 1, "inserted": 1 }
```

`received` is how many events the batch named, duplicates included.
`inserted` is how many were newly written — `received - inserted` is
exactly how many ids this batch named were already ingested by a prior
call.

---

## `GET /keys`

No parameters. Lists the caller's own **non-revoked** keys
(`revoked_at IS NULL`), regardless of expiry — an expired-but-unrevoked
key still shows up here so the caller can see it needs rotating; this is a
separate check from the auth-time expiry check (I9), not a relaxation of
it.

```jsonc
{ "keys": [
    { "id": "8b1e4f2a-…", "prefix": "tpl_a1b2c3d4",
      "createdAt": "2026-05-01T00:00:00Z", "expiresAt": "2026-07-30T00:00:00Z" }
] }
```

`prefix` is the `tpl_` literal plus the first 8 characters of the random
portion — stored in the clear specifically so a listed key is
identifiable without exposing the raw value (I8: the raw key exists only
once, at issuance, and is never stored or re-derivable).

---

## `DELETE /keys/:id` → **204**

Sets `revoked_at`. Owner-scoped, same 404 rule as every other owner-scoped
resource on this surface (I3). There is
deliberately no `POST /api/v1/keys` and no rotation endpoint — see the
main `SKILL.md`'s "Endpoints" section for why rotation specifically can't
be a safe HTTP call.

---

## Request validation

Every request is checked against `openapi.yaml`'s declared shape
(`gin-middleware`'s OpenAPI request validator) **before** it reaches any
handler above — a missing required field or a wrong type never reaches
application code at all, and comes back as `400 validation_error` (see
`errors.md`) with `hint` naming the field, on a best-effort basis. Auth is
deliberately **not** declared as an OpenAPI `security` scheme — credential
resolution is entirely `internal/identity`'s job (I1, I2, I5), enforced as
separate middleware that runs before the OpenAPI validator, not encoded in
the spec.

## Not on this surface

No endpoint issues or rotates a key, and none creates a second domain
resource type beyond what this fork's own `openapi.yaml` documents — a
service forked from this template that adds its own domain module gets
its own endpoints added to `openapi.yaml` directly; check that file first
if this reference and a running instance ever disagree.
