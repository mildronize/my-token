package collector

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGitRootResolver builds a GitRootResolver from a fixed lookup table,
// for unit tests that must never shell out to a real `git` subprocess
// (contract's Testing Decisions: "inject a fake ... resolver (no real
// subprocess)").
func fakeGitRootResolver(table map[string]string) GitRootResolver {
	return func(dir string) (string, error) {
		root, ok := table[dir]
		if !ok {
			return "", errors.New("not a git repository (fake resolver)")
		}
		return root, nil
	}
}

func TestResolveSessionPath_GitRootSucceeds(t *testing.T) {
	resolver := fakeGitRootResolver(map[string]string{
		"/home/thw-home/gits/my-token/internal/collector": "/home/thw-home/gits/my-token",
	})

	got := ResolveSessionPath("/home/thw-home/gits/my-token/internal/collector", resolver)
	assert.Equal(t, "/home/thw-home/gits/my-token", got)
}

func TestResolveSessionPath_FallsBackToRawCwdWhenNotAGitRepo(t *testing.T) {
	resolver := fakeGitRootResolver(map[string]string{}) // resolves nothing

	got := ResolveSessionPath("/home/thw-home/.typ-crews/freya", resolver)
	assert.Equal(t, "/home/thw-home/.typ-crews/freya", got, "not a git repo — raw cwd is the fallback, per tickets 7/8's simple method")
}

// TestFirstCwdBearingRow_UsesEarliestRowWithNonEmptyCwd is the "session's
// first cwd" half of the simple method (tickets 7/8): the session's
// first cwd-bearing record, not the last, and not one from a different
// session.
func TestFirstCwdBearingRow_UsesEarliestRowWithNonEmptyCwd(t *testing.T) {
	rows := []UsageRow{
		{SessionID: "s1", Cwd: "", MessageID: "m0"},
		{SessionID: "s1", Cwd: "/home/thw-home/.typ-crews/freya", MessageID: "m1"},
		{SessionID: "s1", Cwd: "/home/thw-home/.typ-crews/freya/workspace", MessageID: "m2"},
		{SessionID: "s2", Cwd: "/home/thw-home/gits/other-repo", MessageID: "m3"},
	}

	cwd, ok := FirstCwdBearingRow(rows, "s1")
	require.True(t, ok)
	assert.Equal(t, "/home/thw-home/.typ-crews/freya", cwd)
}

func TestFirstCwdBearingRow_NoMatchingSessionOrNoCwdAtAll(t *testing.T) {
	rows := []UsageRow{
		{SessionID: "s1", Cwd: "", MessageID: "m0"},
	}
	_, ok := FirstCwdBearingRow(rows, "s1")
	assert.False(t, ok)

	_, ok = FirstCwdBearingRow(rows, "unknown-session")
	assert.False(t, ok)
}
