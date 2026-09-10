package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActorFromCwd(t *testing.T) {
	cases := []struct {
		name string
		cwd  string
		want string
		ok   bool
	}{
		{"exact crew home", "/home/thw-home/.typ-crews/freya", "freya", true},
		{"nested under crew home", "/home/thw-home/.typ-crews/freya/workspace/synthing", "freya", true},
		{"a different crew", "/home/thw-home/.typ-crews/nicole", "nicole", true},
		{"no crew-home pattern at all", "/home/thw-home/gits/my-token", "", false},
		{"empty cwd", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ActorFromCwd(tc.cwd)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
