-- name: InsertUsageEventIgnoreDuplicate :execrows
-- story-2/ticket-8 adds scan_root (the raw, resolved scan-root path the
-- reporting transcript file was found under -- distinct from `path`,
-- which is git-rooted from touched files).
INSERT OR IGNORE INTO usage_events (id, session_id, actor, path, machine, model, scan_root, input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens, cost, source, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListUsageEventsInWindow :many
-- story-1/ticket-14: every event whose created_at falls between the two
-- bound parameters below, range_start inclusive, range_end exclusive.
-- The raw rows behind the console's own read surface --
-- internal/domain/usage/summary.go's Aggregate does the group_by/window
-- math in Go, pure and unit-testable without sqlc/a real database; this
-- query's only job is the window filter itself.
SELECT id, session_id, actor, path, machine, model, scan_root, input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens, cost, source, created_at
FROM usage_events
WHERE created_at >= sqlc.arg(range_start) AND created_at < sqlc.arg(range_end);
