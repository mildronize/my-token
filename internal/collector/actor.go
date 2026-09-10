package collector

import "regexp"

// crewHomePattern matches a `.typ-crews/<name>` segment anywhere in a
// path (contract's Data model: "derived from a `.typ-crews/<name>`-
// equivalent launch-directory pattern"; ticket 12's own text: a pattern
// like `/home/*/.typ-crews/<name>`). Deliberately not anchored to
// `/home/*` specifically — the crew-home convention is what matters, not
// which filesystem root it sits under on a given host — but is anchored
// so `<name>` is exactly the path segment immediately following
// `.typ-crews/`, not some later segment two or three deep.
var crewHomePattern = regexp.MustCompile(`\.typ-crews/([^/]+)`)

// ActorFromCwd extracts the crew name from a session's cwd (or any other
// path known to be under a crew home) by matching the first
// `.typ-crews/<name>` segment. Returns ok=false when cwd doesn't match
// the pattern at all — callers decide what "unknown actor" means for
// their own purpose (collector.go falls back to "(unknown)", mirroring
// the console's own path-label fallback convention — contract's Console
// display rules).
func ActorFromCwd(cwd string) (actor string, ok bool) {
	m := crewHomePattern.FindStringSubmatch(cwd)
	if m == nil {
		return "", false
	}
	return m[1], true
}
