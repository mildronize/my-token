-- +goose Up
-- scan_roots -- story-2/ticket-8: the console's own scan-root-label
-- lookup (contract's Data model), mirroring `machines`' own shape and
-- lifecycle (story-1/ticket-18). install_id + scan_root_path is the
-- composite PK -- naming is per-install (goal.md's Out of Scope: "a
-- shared/global scan-root naming registry across machines"), so two
-- installs can each register a root under the same name independently,
-- with no cross-machine link implied.
--
-- Upserted on every ingestion batch (internal/domain/usage/service.go's
-- UpsertScanRoots, called from internal/transport/publicapi/
-- usage_handler.go on every POST /api/v1/usage-events/batch),
-- name/source_type/last_seen_at refreshed each time -- a rename takes
-- effect for all past and future rows immediately, same as `machines`.
--
-- No FOREIGN KEY to usage_events, for the same reason machines has none:
-- this table can legitimately be written before any usage_events row
-- referencing the same (install_id, scan_root_path) exists, and this
-- domain module never needs referential integrity against its own
-- append-only event log to serve a label lookup.
CREATE TABLE scan_roots (
    install_id      TEXT NOT NULL,
    scan_root_path  TEXT NOT NULL,
    name            TEXT NOT NULL,
    source_type     TEXT NOT NULL DEFAULT 'claude_code',
    last_seen_at    TIMESTAMP NOT NULL,
    PRIMARY KEY (install_id, scan_root_path)
);

-- +goose Down
DROP TABLE scan_roots;
