package collector

// ActorFromCwd derives a session's `actor` from its cwd (or any other
// path known to be that session's launch directory): the raw cwd value,
// verbatim, no pattern matching, no crew-home convention baked in
// anywhere (story-2/ticket-7 — supersedes tickets 7/8/12's original
// `.typ-crews/<name>` pattern match, a real gap against my-token's own
// stated design goal of being usable by a non-typ-fleet user too).
// Matches the reference collector's (`~/tmp/claude-token-tracker`) own
// `project` derivation method: the session's own launch cwd, unmodified.
//
// There is no "unmatched" case anymore — any non-empty cwd is a valid
// actor value, so this has no ok bool to return. collector.go's own
// actorForSession keeps its existing unknownActor fallback exactly as it
// was: that fallback lives at the FirstCwdBearingRow-ok=false level (no
// cwd-bearing row found at all for a session), not here.
func ActorFromCwd(cwd string) string {
	return cwd
}
