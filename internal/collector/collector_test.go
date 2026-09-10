package collector

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePoster is a BatchPoster test double — collector_test.go's own unit
// tests must never make a real HTTP call (client_test.go already covers
// Client's own wire behavior against a real httptest.Server).
type fakePoster struct {
	posted []Event
	result BatchResult
	err    error
}

func (f *fakePoster) PostBatch(events []Event) (BatchResult, error) {
	f.posted = append(f.posted, events...)
	if f.err != nil {
		return BatchResult{}, f.err
	}
	if f.result.Received == 0 {
		f.result = BatchResult{Received: int64(len(events)), Inserted: int64(len(events))}
	}
	return f.result, nil
}

func fakeResolver(gitRoot string) GitRootResolver {
	return func(dir string) (string, error) {
		if gitRoot == "" {
			return "", assert.AnError
		}
		return gitRoot, nil
	}
}

func TestRun_ScansExtractsAttributesAndPosts_ThenTracksSentState(t *testing.T) {
	root := t.TempDir()
	sessionFile := filepath.Join(root, "session-1.jsonl")
	writeFile(t, sessionFile, ""+
		usageLine("r1", "msg_1", "claude-sonnet-4-5", "text", 100, 50, 10, 5, "/home/thw-home/.typ-crews/freya", "session-1", "2026-09-10T12:00:00Z")+"\n"+
		// same message.id, second content-block line — must dedup to 1 row
		usageLine("r2", "msg_1", "claude-sonnet-4-5", "thinking", 100, 50, 10, 5, "/home/thw-home/.typ-crews/freya", "session-1", "2026-09-10T12:00:01Z")+"\n"+
		usageLine("r3", "msg_2", "claude-sonnet-4-5", "text", 200, 80, 0, 0, "/home/thw-home/.typ-crews/freya", "session-1", "2026-09-10T12:01:00Z")+"\n")

	statePath := filepath.Join(t.TempDir(), "state.json")
	poster := &fakePoster{}

	cfg := Config{ScanPaths: []string{root}, InstallID: "install-abc"}
	result, err := Run(cfg, statePath, fakeResolver("/home/thw-home/gits/my-token"), "test-host", poster)
	require.NoError(t, err)

	assert.Equal(t, 2, result.NewRowsFound, "msg_1's two content-block lines must dedup to one row")
	assert.Equal(t, int64(2), result.Inserted)
	require.Len(t, poster.posted, 2)

	byID := map[string]Event{}
	for _, e := range poster.posted {
		byID[e.ID] = e
	}
	evt1 := byID["msg_1"]
	assert.Equal(t, "session-1", evt1.SessionID)
	assert.Equal(t, "freya", evt1.Actor, "cwd matches .typ-crews/freya")
	assert.Equal(t, "/home/thw-home/gits/my-token", evt1.Path, "resolved via the injected git-root resolver")
	assert.Equal(t, "install-abc", evt1.Machine)
	assert.Equal(t, int64(100), evt1.InputTokens)

	// A second run with the same fixture must find nothing new to send —
	// the local sent-state file already has both message.ids.
	poster2 := &fakePoster{}
	result2, err := Run(cfg, statePath, fakeResolver("/home/thw-home/gits/my-token"), "test-host", poster2)
	require.NoError(t, err)
	assert.Equal(t, 0, result2.NewRowsFound)
	assert.Empty(t, poster2.posted)
}

func TestRun_ActorUnknownFallsBackToUnknownMarker(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "session-1.jsonl"),
		usageLine("r1", "msg_1", "claude-sonnet-4-5", "text", 10, 5, 0, 0, "/home/thw-home/gits/some-repo", "session-1", "2026-09-10T12:00:00Z")+"\n")

	poster := &fakePoster{}
	cfg := Config{ScanPaths: []string{root}, InstallID: "install-abc"}
	_, err := Run(cfg, filepath.Join(t.TempDir(), "state.json"), fakeResolver("/home/thw-home/gits/some-repo"), "test-host", poster)
	require.NoError(t, err)
	require.Len(t, poster.posted, 1)
	assert.Equal(t, "(unknown)", poster.posted[0].Actor)
}

func TestRun_NotAGitRepoFallsBackToRawCwdForPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "session-1.jsonl"),
		usageLine("r1", "msg_1", "claude-sonnet-4-5", "text", 10, 5, 0, 0, "/home/thw-home/.typ-crews/freya", "session-1", "2026-09-10T12:00:00Z")+"\n")

	poster := &fakePoster{}
	cfg := Config{ScanPaths: []string{root}, InstallID: "install-abc"}
	_, err := Run(cfg, filepath.Join(t.TempDir(), "state.json"), fakeResolver(""), "test-host", poster)
	require.NoError(t, err)
	require.Len(t, poster.posted, 1)
	assert.Equal(t, "/home/thw-home/.typ-crews/freya", poster.posted[0].Path)
}
