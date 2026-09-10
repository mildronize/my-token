package collector

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// unknownPath is the final sentinel `path` falls back to when neither
// ticket 13's touched-paths method nor ticket 12's simple cwd method
// produces anything usable at all (ticket 10 §9's "final fallback").
// Token usage must never be silently dropped just because path
// attribution failed — mirrors collector.go's unknownActor convention for
// the same reason.
const unknownPath = "(unknown)"

// GitRootResolver resolves dir's git root (as `git rev-parse
// --show-toplevel` would), or returns an error if dir is not inside a
// git working tree. A function type, not a concrete implementation
// directly, so tests can inject a fake table-driven resolver instead of
// shelling out to a real `git` subprocess (contract's Testing
// Decisions).
type GitRootResolver func(dir string) (string, error)

// RealGitRootResolver shells out to `git -C dir rev-parse
// --show-toplevel` — the production GitRootResolver. Trims the trailing
// newline git always prints.
func RealGitRootResolver(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ResolveSessionPath implements tickets 7/8's SIMPLE path-attribution
// method (the only method in scope for this ticket — ticket 13 upgrades
// this in place to the full touched-paths method): given a session's
// first cwd-bearing record's cwd, run resolver on it and use the
// git-root if it succeeds, else fall back to the raw cwd value itself.
func ResolveSessionPath(cwd string, resolver GitRootResolver) string {
	if root, err := resolver(cwd); err == nil && root != "" {
		return root
	}
	return cwd
}

// FirstCwdBearingRow returns the cwd of the earliest row (in rows' own
// order) belonging to sessionID that carries a non-empty cwd — "the
// session's first cwd", the input ResolveSessionPath's simple method
// needs. ok is false if sessionID has no row with a non-empty cwd at
// all.
func FirstCwdBearingRow(rows []UsageRow, sessionID string) (cwd string, ok bool) {
	for _, r := range rows {
		if r.SessionID != sessionID {
			continue
		}
		if r.Cwd != "" {
			return r.Cwd, true
		}
	}
	return "", false
}

// GitRootCache is an in-memory directory -> git-root cache used to wrap a
// GitRootResolver so a directory is never re-resolved via a real `git`
// subprocess call twice within one resolution pass (contract's
// git_root_cache table; ticket 13 step 4). It deliberately holds no
// persistence logic of its own — PathStore (pathstore.go) is what
// survives across collector runs; GitRootCache is just the in-memory
// speed layer CachedGitRootResolver checks first. Keeping persistence out
// of this type is what keeps it (and CachedGitRootResolver) free of any
// database dependency for path_test.go's unit tests.
type GitRootCache struct {
	entries map[string]string
}

// NewGitRootCache returns an empty GitRootCache.
func NewGitRootCache() *GitRootCache {
	return &GitRootCache{entries: make(map[string]string)}
}

// Seed pre-populates the cache from persisted entries (PathStore's own
// git_root_cache table, loaded once per collector run) so an
// already-known directory never reaches the wrapped resolver at all, even
// on this process's very first lookup of it.
func (c *GitRootCache) Seed(entries map[string]string) {
	for dir, root := range entries {
		c.entries[dir] = root
	}
}

// Snapshot returns every directory->git-root entry currently in the
// cache, Seed-loaded ones included. Callers persist this back to
// PathStore after a run so newly-learned entries survive to the next one
// — re-writing an already-known entry is a harmless idempotent upsert,
// not a correctness concern.
func (c *GitRootCache) Snapshot() map[string]string {
	out := make(map[string]string, len(c.entries))
	for k, v := range c.entries {
		out[k] = v
	}
	return out
}

// CachedGitRootResolver wraps resolver so a directory already present in
// cache (via Seed, or from an earlier call within this same run) never
// reaches resolver again. This is the actual "a directory is never
// re-resolved via a git rev-parse subprocess call twice" behavior
// contract's git_root_cache exists for (ticket 13 step 4) — resolver
// itself (real or fake) is never told the difference between a cache hit
// and a fresh call.
func CachedGitRootResolver(resolver GitRootResolver, cache *GitRootCache) GitRootResolver {
	return func(dir string) (string, error) {
		if root, ok := cache.entries[dir]; ok {
			return root, nil
		}
		root, err := resolver(dir)
		if err != nil {
			return "", err
		}
		cache.entries[dir] = root
		return root, nil
	}
}

// hasRealExtension reports whether base (a bare filename, no directory
// component) looks like it names a file rather than a directory: a "."
// that is neither the first character (hidden-file/dotdir names like
// ".claude" or ".typmem" must not count) nor the last (a trailing bare
// dot). Used only as dirForGitRoot's fallback heuristic when a real
// os.Stat isn't available (path doesn't exist on this machine, or a
// unit-test fixture) — a no-extension real file (e.g. "Makefile") is a
// known, accepted blind spot of the heuristic-only path; the production
// os.Stat branch handles that case correctly whenever the file still
// exists on disk.
func hasRealExtension(base string) bool {
	idx := strings.LastIndex(base, ".")
	return idx > 0 && idx < len(base)-1
}

// dirForGitRoot returns the directory that should be passed to a
// GitRootResolver for touchedPath (ticket 13 step 3: "git rev-parse
// --show-toplevel on its directory, or the file's parent directory if
// the path is a file"). Prefers a real os.Stat when the path still exists
// on this machine (production's common case — a path a tool call touched
// during this or an earlier session is usually still there); falls back
// to hasRealExtension's filename heuristic when it doesn't (already
// deleted, or a constructed test fixture that was never meant to exist on
// disk).
func dirForGitRoot(touchedPath string) string {
	if info, err := os.Stat(touchedPath); err == nil {
		if info.IsDir() {
			return touchedPath
		}
		return filepath.Dir(touchedPath)
	}
	if hasRealExtension(filepath.Base(touchedPath)) {
		return filepath.Dir(touchedPath)
	}
	return touchedPath
}

// TallyGitRoots implements ticket 13 steps 2-3's core aggregation: drop
// any /tmp/... scratch path outright (isScratchPath, touchedpaths.go),
// then git-root each remaining touchedPath individually via resolver. A
// path resolver genuinely can't place inside a git repo (a real,
// non-scratch directory that just isn't one) falls back to its own
// directory as its "root" for voting purposes (ticket 10 §9.2) — it can
// still win the majority vote as a raw, non-canonicalized path. Returns
// the resulting per-root vote counts; MajorityGitRoot picks the winner.
func TallyGitRoots(touchedPaths []string, resolver GitRootResolver) map[string]int64 {
	counts := make(map[string]int64)
	for _, p := range touchedPaths {
		if p == "" || isScratchPath(p) {
			continue
		}
		dir := dirForGitRoot(p)
		root, err := resolver(dir)
		if err != nil || root == "" {
			root = dir
		}
		counts[root]++
	}
	return counts
}

// MajorityGitRoot picks the git-root with the highest vote count — ticket
// 10 §7's corrected aggregation ("most frequent git-root by count", not
// set-intersection: a literal common-directory reduction across every
// touched path was tried against real data and collapses to "/" the
// moment a session legitimately touches more than one unrelated tree,
// which is normal). Ties break on the lexicographically smaller root,
// purely for determinism — Go map iteration order is random, and vote
// counts loaded back from session_path_votes across multiple runs have no
// other natural ordering; real ties are not expected to matter in
// practice. ok is false for a nil/empty counts map (nothing to vote on at
// all).
func MajorityGitRoot(counts map[string]int64) (root string, ok bool) {
	var best int64
	for r, c := range counts {
		if c > best || (c == best && c > 0 && r < root) {
			root, best = r, c
		}
	}
	return root, best > 0
}

// ResolveSessionPathTouched is ticket 13's touched-paths half of the full
// method: tally every touchedPath's git-root and return the majority
// winner. ok is false when touchedPaths yields no real (non-scratch)
// candidate at all — callers fall back to the session-cwd method
// (ResolveSessionPath) in that case, per ticket 10 §9.3.
func ResolveSessionPathTouched(touchedPaths []string, resolver GitRootResolver) (path string, ok bool) {
	return MajorityGitRoot(TallyGitRoots(touchedPaths, resolver))
}

// resolvePathFromVoteCounts is ticket 13's whole fallback chain, factored
// out so there is exactly one implementation of it: the majority-vote
// winner over an already-tallied set of git-root vote counts; if that
// finds no real candidate at all, ticket 12's simple cwd method (only
// when a cwd-bearing record actually exists); if even that produces
// nothing usable, the "(unknown)" sentinel. Token usage must never be
// dropped just because path attribution failed (ticket 10 §9's final
// fallback).
//
// Both ResolveSessionPathFull (a single-pass, non-persisted call — what
// path_test.go's unit tests exercise directly against fresh vote counts)
// and collector.go's scanSessionPaths (which reads its vote counts back
// from PathStore's persisted, incrementally-updated tally instead of
// re-tallying from raw touchedPaths every run) share this one function,
// so the fallback-chain logic itself can never drift between the two
// callers.
func resolvePathFromVoteCounts(voteCounts map[string]int64, cwd string, cwdOK bool, resolver GitRootResolver) string {
	if winner, ok := MajorityGitRoot(voteCounts); ok {
		return winner
	}
	if cwdOK {
		return ResolveSessionPath(cwd, resolver)
	}
	return unknownPath
}

// ResolveSessionPathFull is ticket 13's whole fallback chain for a
// session's `path`, given a fresh (not yet tallied) list of touchedPaths:
// the touched-paths majority vote first, then the simple cwd method, then
// the "(unknown)" sentinel — see resolvePathFromVoteCounts for the shared
// fallback logic itself.
func ResolveSessionPathFull(touchedPaths []string, cwd string, cwdOK bool, resolver GitRootResolver) string {
	return resolvePathFromVoteCounts(TallyGitRoots(touchedPaths, resolver), cwd, cwdOK, resolver)
}
