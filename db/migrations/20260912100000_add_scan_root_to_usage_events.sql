-- +goose Up
-- usage_events.scan_root -- story-2/ticket-8 (contract's Data model): the
-- raw, resolved scan-root path (as configured in the collector's own
-- scan_paths, ScanPath.Path after expandHome) the reporting transcript
-- file was found under -- distinct from `path`, which is git-rooted from
-- touched files (ticket 13), not the transcript's own location.
--
-- NOT NULL per the contract's own schema. DEFAULT '' backfills any
-- pre-existing dev rows written before this column existed -- this is
-- still development data (goal.md's Out of Scope), the same "old rows
-- simply won't merge" precedent already set for `actor`'s own shape
-- change, not an attempt at a real backfill.
ALTER TABLE usage_events ADD COLUMN scan_root TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE usage_events DROP COLUMN scan_root;
