# `/api/v1` errors

**The envelope and I-numbered rules below are domain-agnostic — `key`
mentions are identity, not this fork's own domain.** The codes table
below reflects this fork's real domain (story-1/ticket-11's
`usage_events`), which has no permission-refusal case the way a
richer example domain might — update this file again if this fork's own
`/api/v1` surface grows a domain module with one.

Every failure that reaches a handler — mapped or not — comes back as this
envelope (`_rules/_contract/API.md`):

```jsonc
{ "error": {
    "code": "validation_error",
    "message": "install_id is required",
    "hint": null
} }
```

`code` and `message` are always present. `hint` appears only where the
table below says so — do not require it, and where it's present, treat it
as best-effort: it comes from pattern-matching the underlying validator's
own error text, not a guaranteed structured field.

## Codes

| Code | HTTP | When | Carries |
| --- | --- | --- | --- |
| `unauthorized` | 401 | any credential failure at all (I2, I5, I9) | — |
| `not_found` | 404 | a key id that exists but isn't the caller's own, or never existed (I3 scopes keys — absence, not `403`) | — |
| `actor_field_present` | 400 | request tried to declare an actor (I1) | — |
| `validation_error` | 400 | a malformed or missing field on `POST /usage-events/batch` (e.g. a missing `install_id`/`hostname`/required event field, or a stray `cost`/`source` on an event, `additionalProperties: false`), caught either by the OpenAPI request validator or the handler's own fallback check | `hint` names the field, when the underlying error names exactly one |

There is no `internal_error` code documented in the contract — an
unmapped failure is a bug in the service, not a designed response; report
it rather than retrying into it.

## Reading them

**401 tells you nothing about why, on purpose (I5).** Missing, malformed,
expired, revoked, inactive-user, and owner-role credentials are all
indistinguishable in the response — the real reason is logged server-side
only. Do not branch on it, and see the main `SKILL.md`'s "The
indistinguishable-401 trap" before assuming the credential itself is what
failed — an empty or unset key produces the exact same response as a
genuinely wrong one.

**404 only ever means a key.** A key id that exists but belongs to a
different caller returns exactly the same `not_found` a nonexistent id
would (I3, unchanged) — deliberate, since a `403` would leak that the row
exists. There is no other resource on this surface that a caller could
get a 404 for: `usage_events` has no per-id lookup endpoint at all, only
batch ingestion.

**400 `actor_field_present` is a loud refusal, not a dropped field.** The
guard checks the `X-Actor` header, and `actor` / `actorId` / `ownerId` in
the body **and the query string**. Identity comes from the credential
only; there is no free-text identity sub-label on this surface the way
`my-task-api`'s `X-Actor-Detail` is (that surface's own escape hatch does
not exist here).

**400 `validation_error` can come from two different places, and they
don't always carry `hint`.** Most of the time it's `openapi.yaml`'s
request validator, running before any handler code — that's the case
`hint` is most likely to be populated for, when the underlying validation
error names exactly one field or parameter. A handler's own defensive
`ShouldBindJSON` fallback (present in case a route is ever wired without
the validator ahead of it) returns the same code with no `hint` at all —
treat `hint`'s absence as "re-check your request body against
`references/endpoints.md`," not as a signal about which path produced the
error.

## What does not error

**Resending a `POST /usage-events/batch` never errors and never writes an
already-ingested event twice.** Idempotent on each event's own `id`
(`message.id`): a resend just reports a lower `inserted` count than
`received` for the ids it had already seen — a `200`-shaped success, not
a `409` or any other conflict code. Retrying a batch you're unsure landed
is always safe on this surface.

Revoking an already-revoked key is `404 not_found`, the same as any other
unknown id — naturally idempotent from the caller's side with no
special-casing needed, but that idempotency shows up as a repeat 404, not
a repeat success.

`GET /keys` lists an expired-but-unrevoked key without erroring or
filtering it out — expiry is checked only when that key is actually
presented for auth (I9), not when listing. Seeing a key here does not mean
it still authenticates.
