package collector

import (
	"os/exec"
	"strings"
)

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
