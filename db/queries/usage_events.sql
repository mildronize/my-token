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
-- query's own job is the window/machine/scan_root filters.
--
-- story-2/ticket-10 adds two optional filters, AND-composed with the
-- window bound and with each other: machine (the raw install_id) and
-- the scan_root pair (scan_root_install_id + scan_root_path together --
-- internal/domain/usage/repo.go decomposes the wire's own
-- <install_id>:<scan_root_path> composite into these two arguments
-- before this query ever runs, same machine-prefix convention ticket 7
-- established for group_by=path, contract's API surface section). Each
-- narg is NULL when its own filter was not requested --
-- "sqlc.narg(x) IS NULL OR column = sqlc.narg(x)" is the standard
-- optional-filter idiom, so one query text covers "no filter"/"one
-- filter"/"both filters" without a query-builder assembling different
-- SQL per case.
SELECT id, session_id, actor, path, machine, model, scan_root, input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens, cost, source, created_at
FROM usage_events
WHERE created_at >= sqlc.arg(range_start) AND created_at < sqlc.arg(range_end)
  AND (sqlc.narg(machine) IS NULL OR machine = sqlc.narg(machine))
  AND (sqlc.narg(scan_root_install_id) IS NULL OR machine = sqlc.narg(scan_root_install_id))
  AND (sqlc.narg(scan_root_path) IS NULL OR scan_root = sqlc.narg(scan_root_path));
