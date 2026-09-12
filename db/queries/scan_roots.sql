-- name: UpsertScanRoot :exec
-- story-2/ticket-8: the reporting collector's own (install_id,
-- scan_root_path) pair, refreshed on every ingestion batch
-- (internal/domain/usage/service.go's UpsertScanRoots, called from
-- internal/transport/publicapi/usage_handler.go's
-- IngestUsageEventsBatch) so a renamed scan root's console label catches
-- up rather than staying stuck on whatever name it was first seen with
-- (contract's Data model: `scan_roots`). INSERT OR REPLACE, same idiom
-- machines.sql already uses -- not "ON CONFLICT(...) DO UPDATE SET ...",
-- which false-positives internal/dbquery/tableisolation.go's table
-- scanner (it reads the SET clause's target list as if it named a table
-- called "set"; see machines.sql's own header for the full explanation).
-- INSERT OR REPLACE has no such clause and remains functionally
-- equivalent here since every column is supplied on every call.
--
-- Note: this file must stay plain ASCII -- bin/sqlc v1.31.1 corrupts its
-- own star-expansion byte offsets on any non-ASCII byte in a
-- db/queries/*.sql file (internal/db_queries_ascii_test.go's own doc
-- comment has the full history). No em dashes, no Thai, no "section" /
-- other non-ASCII punctuation in this file.
INSERT OR REPLACE INTO scan_roots (install_id, scan_root_path, name, source_type, last_seen_at)
VALUES (?, ?, ?, ?, ?);

-- name: ListScanRoots :many
-- story-2/ticket-9: every registered scan_roots row across every
-- reporting install -- GET /api/bff/usage/scan-roots' own read path
-- (internal/domain/usage/service.go's Service.ScanRoots), populating the
-- console's future scan-root filter dropdown (ticket 11). No window
-- filter, no group_by -- this table is not usage_events, it's a small
-- upserted label table (mirrors ListMachines in machines.sql). The
-- install_id -> hostname join happens in Go over this result
-- (Service.ScanRoots, reusing the existing MachineHostnames query), not
-- as a SQL JOIN here -- same reasoning ListMachines' own doc comment
-- gives: reuse the pure aggregation/substitution logic that already
-- exists rather than re-deriving a join in SQL.
SELECT install_id, scan_root_path, name FROM scan_roots;
