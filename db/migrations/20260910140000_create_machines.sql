-- +goose Up
-- machines — story-1/ticket-18: the console's own hostname-label lookup
-- for usage_events.machine (contract's Data model: "machines exists
-- purely so the console has a readable label to show instead of a raw
-- UUID"). install_id is the same string usage_events.machine already
-- stores (the collector's own install_id, ticket 5) and stays the real
-- join key — this table never replaces it, only adds a display-only
-- hostname alongside it.
--
-- Upserted on every ingestion batch (internal/domain/usage/service.go's
-- UpsertMachine, called from internal/transport/publicapi/usage_handler.go
-- on every POST /api/v1/usage-events/batch), hostname/last_seen_at
-- refreshed each time — a plain TEXT PRIMARY KEY on install_id is enough
-- for `INSERT OR REPLACE` (db/queries/machines.sql) to mean exactly
-- that, the same reasoning usage_events.sql's own header gives for why
-- INSERT OR IGNORE needs no separate unique index there.
--
-- No FOREIGN KEY to usage_events: this table can legitimately be written
-- before any usage_events row referencing the same install_id exists
-- (the ingestion handler upserts machines and inserts usage_events in the
-- same request, but nothing enforces an ordering between the two at the
-- schema level), and a domain module never needs referential integrity
-- against its own append-only event log to serve a label lookup.
CREATE TABLE machines (
    install_id     TEXT PRIMARY KEY,
    hostname       TEXT NOT NULL,
    last_seen_at   TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE machines;
