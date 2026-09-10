-- name: InsertUsageEventIgnoreDuplicate :execrows
INSERT OR IGNORE INTO usage_events (id, session_id, actor, path, machine, model, input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens, cost, source, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
