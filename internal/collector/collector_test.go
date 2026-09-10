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
	pathStorePath := filepath.Join(t.TempDir(), "pathstore.db")
	poster := &fakePoster{}

	cfg := Config{ScanPaths: []string{root}, InstallID: "install-abc"}
	result, err := Run(cfg, statePath, pathStorePath, fakeResolver("/home/thw-home/gits/my-token"), "test-host", poster)
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
	result2, err := Run(cfg, statePath, pathStorePath, fakeResolver("/home/thw-home/gits/my-token"), "test-host", poster2)
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
	_, err := Run(cfg, filepath.Join(t.TempDir(), "state.json"), filepath.Join(t.TempDir(), "pathstore.db"), fakeResolver("/home/thw-home/gits/some-repo"), "test-host", poster)
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
	_, err := Run(cfg, filepath.Join(t.TempDir(), "state.json"), filepath.Join(t.TempDir(), "pathstore.db"), fakeResolver(""), "test-host", poster)
	require.NoError(t, err)
	require.Len(t, poster.posted, 1)
	assert.Equal(t, "/home/thw-home/.typ-crews/freya", poster.posted[0].Path)
}

// TestRun_TouchedPathsMajorityVoteBeatsSessionCwd is ticket 13's own
// demoable/verifiable-on-its-own scenario, per the ticket's own text: "a
// session whose cwd never drifts, but whose tool calls' file_path/Bash
// targets consistently point elsewhere" (the exact luna/my-template
// pattern ticket 10 found) — confirms Run's own `path` reflects the
// majority-vote winner over the touched paths, not the session's launch
// directory, end to end through the real pipeline (scan -> extract ->
// tally -> resolve), not just the pure path.go functions in isolation.
func TestRun_TouchedPathsMajorityVoteBeatsSessionCwd(t *testing.T) {
	root := t.TempDir()
	sessionID := "session-luna-like"
	sessionFile := filepath.Join(root, sessionID+".jsonl")

	// cwd never drifts from the crew home — but three Edit calls and one
	// Bash `cd` all target a different real project tree.
	writeFile(t, sessionFile, ""+
		usageLine("r1", "msg_1", "claude-sonnet-4-5", "text", 10, 5, 0, 0, "/home/thw-home/.typ-crews/luna", sessionID, "2026-09-10T12:00:00Z")+"\n"+
		toolUseLine(t, sessionID, "Edit", map[string]any{"file_path": "/home/thw-home/gits/my-template/AGENTS.md", "old_string": "a", "new_string": "b"})+"\n"+
		toolUseLine(t, sessionID, "Edit", map[string]any{"file_path": "/home/thw-home/gits/my-template/README.md", "old_string": "a", "new_string": "b"})+"\n"+
		toolUseLine(t, sessionID, "Bash", map[string]any{"command": "cd /home/thw-home/gits/my-template && git commit -m \"work\""})+"\n")

	poster := &fakePoster{}
	cfg := Config{ScanPaths: []string{root}, InstallID: "install-abc"}
	resolver := fakeGitRootResolver(map[string]string{
		"/home/thw-home/gits/my-template": "/home/thw-home/gits/my-template",
	})

	result, err := Run(cfg, filepath.Join(t.TempDir(), "state.json"), filepath.Join(t.TempDir(), "pathstore.db"), resolver, "test-host", poster)
	require.NoError(t, err)
	assert.Equal(t, 1, result.NewRowsFound)
	require.Len(t, poster.posted, 1)
	assert.Equal(t, "/home/thw-home/gits/my-template", poster.posted[0].Path, "touched-path majority vote must win over the session's own never-drifting cwd")
	assert.Equal(t, "luna", poster.posted[0].Actor, "actor stays cwd-derived — ticket 13 only changes path")
}
