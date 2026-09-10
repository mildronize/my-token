package collector

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver (pure Go, no cgo) — same driver internal/platform/db.go already uses for the core service's own database
)

// PathStore persists ticket 13/the contract's git_root_cache and
// session_path_votes tables (plus session_scan_progress, this store's own
// bookkeeping for "how far a session's transcript has already been
// scanned" — the contract's own wording for that concept, ticket 13 step
// 6) in a local SQLite database file. This is the collector's own cache,
// entirely separate from the core service's database
// (internal/platform/db.go): a collector install has no network access to
// that database at all, and shouldn't need any — matching doc.go's own
// design note that this package must stay independently buildable/
// distributable from just its own source tree.
//
// session_scan_progress is keyed by (session_id, file_path) rather than
// session_id alone: one session's transcript is not always one physical
// file — ticket 9's scan scope is a session's main transcript file *plus*
// every <session-uuid>/subagents/agent-*.jsonl file beneath it — so
// "how far has this session been scanned" is really "how far has each of
// its files been scanned," tracked independently per file so a re-run
// only re-parses genuinely new lines in each one.
//
// This is a deliberate deviation from the contract's literal Data model
// wording ("session_path_votes — session_id, git_root, vote_count, PK
// (session_id, git_root), plus a last_scanned_offset per session"), which
// names one offset column per session, not per (session, git_root) row —
// a shape that has no sane single value once a session's transcript spans
// several physical files. A single last_scanned_offset per session_id
// could still work for a session with exactly one file, but silently
// breaks (which file does it mean?) the moment subagents/agent-*.jsonl
// files exist, which ticket 9 established is the common case, not an
// edge case. Recorded here explicitly rather than silently — flag if this
// reading of "per session" is wrong.
type PathStore struct {
	db *sql.DB
}

// pathStoreSchema is applied via CREATE TABLE IF NOT EXISTS on every
// OpenPathStore call — idempotent, no separate migration tool needed for
// what is, on every real install, a small local cache file rather than
// the core service's own schema-managed database.
const pathStoreSchema = `
CREATE TABLE IF NOT EXISTS git_root_cache (
	directory   TEXT PRIMARY KEY,
	git_root    TEXT NOT NULL,
	resolved_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS session_path_votes (
	session_id  TEXT NOT NULL,
	git_root    TEXT NOT NULL,
	vote_count  INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (session_id, git_root)
);
CREATE TABLE IF NOT EXISTS session_scan_progress (
	session_id  TEXT NOT NULL,
	file_path   TEXT NOT NULL,
	line_offset INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (session_id, file_path)
);
`

// OpenPathStore opens (creating if necessary) the SQLite database at path
// and ensures its schema exists.
func OpenPathStore(path string) (*PathStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening path-store %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging path-store %s: %w", path, err)
	}
	if _, err := db.Exec(pathStoreSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing path-store schema %s: %w", path, err)
	}
	return &PathStore{db: db}, nil
}

// Close closes the underlying database connection.
func (s *PathStore) Close() error {
	return s.db.Close()
}

// AllGitRoots returns every persisted directory->git_root mapping, for
// seeding a fresh GitRootCache at the start of a run (collector.go) — the
// step that makes "a directory is never re-resolved via a git rev-parse
// subprocess call twice" true across runs, not just within one.
func (s *PathStore) AllGitRoots() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT directory, git_root FROM git_root_cache`)
	if err != nil {
		return nil, fmt.Errorf("loading git_root_cache: %w", err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var dir, root string
		if err := rows.Scan(&dir, &root); err != nil {
			return nil, fmt.Errorf("scanning git_root_cache row: %w", err)
		}
		out[dir] = root
	}
	return out, rows.Err()
}

// SaveGitRoots upserts every directory->git_root entry in snapshot
// (GitRootCache.Snapshot's output) into git_root_cache — called once at
// the end of a run so entries this run learned survive to the next one.
// Re-writing an already-known entry (loaded via AllGitRoots at the start
// of the same run) is a harmless idempotent upsert.
func (s *PathStore) SaveGitRoots(snapshot map[string]string) error {
	if len(snapshot) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning git_root_cache transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit succeeds

	stmt, err := tx.Prepare(`
		INSERT INTO git_root_cache (directory, git_root, resolved_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(directory) DO UPDATE SET git_root = excluded.git_root, resolved_at = excluded.resolved_at`)
	if err != nil {
		return fmt.Errorf("preparing git_root_cache upsert: %w", err)
	}
	defer stmt.Close()

	for dir, root := range snapshot {
		if _, err := stmt.Exec(dir, root); err != nil {
			return fmt.Errorf("upserting git_root_cache row %q: %w", dir, err)
		}
	}
	return tx.Commit()
}

// AddVotes increments session_path_votes' vote_count for sessionID, one
// git-root at a time, by delta — ticket 13 step 6's "increments vote
// counts incrementally rather than re-deriving from scratch every time."
// A delta of 0 is skipped (no-op row, never written).
func (s *PathStore) AddVotes(sessionID string, deltas map[string]int64) error {
	nonZero := make(map[string]int64, len(deltas))
	for root, delta := range deltas {
		if delta != 0 {
			nonZero[root] = delta
		}
	}
	if len(nonZero) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning session_path_votes transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit succeeds

	stmt, err := tx.Prepare(`
		INSERT INTO session_path_votes (session_id, git_root, vote_count)
		VALUES (?, ?, ?)
		ON CONFLICT(session_id, git_root) DO UPDATE SET vote_count = vote_count + excluded.vote_count`)
	if err != nil {
		return fmt.Errorf("preparing session_path_votes upsert: %w", err)
	}
	defer stmt.Close()

	for root, delta := range nonZero {
		if _, err := stmt.Exec(sessionID, root, delta); err != nil {
			return fmt.Errorf("adding vote for session %q root %q: %w", sessionID, root, err)
		}
	}
	return tx.Commit()
}

// VoteCounts returns sessionID's current running vote tally — a cheap
// read, not a recomputation from raw transcript data (ticket 13 step 6 /
// ticket 10 §8's "path for that session is a cheap read — whichever
// git_root has the highest vote_count").
func (s *PathStore) VoteCounts(sessionID string) (map[string]int64, error) {
	rows, err := s.db.Query(`SELECT git_root, vote_count FROM session_path_votes WHERE session_id = ?`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("loading session_path_votes for %q: %w", sessionID, err)
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var root string
		var count int64
		if err := rows.Scan(&root, &count); err != nil {
			return nil, fmt.Errorf("scanning session_path_votes row for %q: %w", sessionID, err)
		}
		out[root] = count
	}
	return out, rows.Err()
}

// ScanOffset returns how many lines of (sessionID, filePath) have already
// been scanned for touched paths — 0 if this file/session pair has never
// been scanned before, not an error. Callers only need to parse
// filePath's lines from this offset onward (ticket 13 step 6).
func (s *PathStore) ScanOffset(sessionID, filePath string) (int64, error) {
	var offset int64
	err := s.db.QueryRow(
		`SELECT line_offset FROM session_scan_progress WHERE session_id = ? AND file_path = ?`,
		sessionID, filePath,
	).Scan(&offset)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("loading scan offset for session %q file %q: %w", sessionID, filePath, err)
	}
	return offset, nil
}

// SetScanOffset records how many lines of (sessionID, filePath) have now
// been scanned, so the next run picks up from exactly there.
func (s *PathStore) SetScanOffset(sessionID, filePath string, offset int64) error {
	_, err := s.db.Exec(`
		INSERT INTO session_scan_progress (session_id, file_path, line_offset)
		VALUES (?, ?, ?)
		ON CONFLICT(session_id, file_path) DO UPDATE SET line_offset = excluded.line_offset`,
		sessionID, filePath, offset)
	if err != nil {
		return fmt.Errorf("setting scan offset for session %q file %q: %w", sessionID, filePath, err)
	}
	return nil
}
