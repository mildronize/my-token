-- +goose Up
-- usage_events — one row per turn (one unique `message.id` carrying a
-- `usage` dict — ticket 9, not one per JSONL line). `id` IS `message.id`,
-- the idempotency key: a collector re-POSTing an already-ingested id must
-- change nothing (INSERT OR IGNORE, internal/domain/usage/repo.go), which
-- is exactly what a plain TEXT PRIMARY KEY gives us here for free — no
-- separate unique index needed, unlike todo_events' client_request_id
-- (that table's own primary key is a fresh generated id per row; here the
-- caller-supplied dedup key IS the primary key, so ON CONFLICT(id) DO
-- NOTHING is enough).
--
-- No FOREIGN KEY to users/api_keys: this domain is reported by a
-- collector authenticating as one machine-wide API key, not owned by (or
-- scoped to) the human/agent identity that key belongs to — `actor` below
-- is a plain string naming which crew-home produced the underlying
-- session (ticket 9/contract's data model), not a users.id reference.
-- Ownership-scoping (I3) does not apply to this domain for the same
-- reason it stopped applying to `todos` (GOAL.md's Ownership model
-- decision) — see internal/domain/usage/repo_test.go's own TestI3_ test.
CREATE TABLE usage_events (
    id                           TEXT PRIMARY KEY,
    session_id                   TEXT NOT NULL,
    actor                        TEXT NOT NULL,
    path                         TEXT NOT NULL,
    machine                      TEXT NOT NULL,
    model                        TEXT NOT NULL,
    input_tokens                 INTEGER NOT NULL,
    output_tokens                INTEGER NOT NULL,
    cache_read_input_tokens      INTEGER NOT NULL,
    cache_creation_input_tokens  INTEGER NOT NULL,
    cost                         REAL NOT NULL,
    source                       TEXT NOT NULL,
    created_at                   TIMESTAMP NOT NULL
);

-- Every dimension ticket 14's console groups/filters by (contract's BFF
-- section: group_by=actor|path|machine, window filters on created_at).
CREATE INDEX usage_events_actor_idx ON usage_events (actor);
CREATE INDEX usage_events_path_idx ON usage_events (path);
CREATE INDEX usage_events_machine_idx ON usage_events (machine);
CREATE INDEX usage_events_created_at_idx ON usage_events (created_at);

-- +goose Down
DROP TABLE usage_events;
