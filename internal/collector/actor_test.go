package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestActorFromCwd_ReturnsRawCwdVerbatim replaces the old crewHomePattern
// test (story-2/ticket-7, contract's Testing Decisions: "ActorFromCwd-
// equivalent returns the raw cwd verbatim for an arbitrary directory —
// no .typ-crews needed, no ok=false/unknown case reachable for a
// non-empty cwd"). There is no pattern requirement and no special-casing
// of `.typ-crews` anymore — any non-empty cwd, typ-fleet-shaped or not,
// is returned exactly as given.
func TestActorFromCwd_ReturnsRawCwdVerbatim(t *testing.T) {
	cases := []struct {
		name string
		cwd  string
		want string
	}{
		{"a typ-crews-shaped cwd, no longer special", "/home/thw-home/.typ-crews/freya", "/home/thw-home/.typ-crews/freya"},
		{"an arbitrary non-typ-fleet directory", "/home/thw-home/gits/my-token", "/home/thw-home/gits/my-token"},
		{"a nested directory under a crew home", "/home/thw-home/.typ-crews/freya/workspace/synthing", "/home/thw-home/.typ-crews/freya/workspace/synthing"},
		{"empty cwd — caller's own concern, not this function's", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ActorFromCwd(tc.cwd)
			assert.Equal(t, tc.want, got)
		})
	}
}
