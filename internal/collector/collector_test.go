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
	posted          []Event
	postedScanRoots []ScanRootReport
	result          BatchResult
	err             error
}

func (f *fakePoster) PostBatch(scanRoots []ScanRootReport, events []Event) (BatchResult, error) {
	f.posted = append(f.posted, events...)
	f.postedScanRoots = append(f.postedScanRoots, scanRoots...)
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

	cfg := Config{ScanPaths: []ScanPath{{Name: "main", Path: root, SourceType: "claude_code"}}, InstallID: "install-abc"}
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
	assert.Equal(t, root, evt1.ScanRoot, "story-2/ticket-8: ScanRoot is the configured ScanPath entry's own resolved Path, not the git-rooted `path`")

	require.Len(t, poster.postedScanRoots, 1, "story-2/ticket-8: the batch's own deduped scan_roots array, one entry per distinct scan root actually used this run")
	assert.Equal(t, ScanRootReport{Path: root, Name: "main", SourceType: "claude_code"}, poster.postedScanRoots[0])

	// A second run with the same fixture must find nothing new to send —
	// the local sent-state file already has both message.ids.
	poster2 := &fakePoster{}
	result2, err := Run(cfg, statePath, pathStorePath, fakeResolver("/home/thw-home/gits/my-token"), "test-host", poster2)
	require.NoError(t, err)
	assert.Equal(t, 0, result2.NewRowsFound)
	assert.Empty(t, poster2.posted)
}

// TestRun_TwoScanPathEntries_EventsAttributedToTheRightScanRoot is
// story-2/ticket-8's own required unit test: a fixture scan across two
// configured ScanPath entries correctly attributes each found
// transcript's events to the right ScanRoot value (and carries that
// entry's own Name/SourceType through to the batch's scan_roots array),
// not just whichever root happened to be scanned first.
func TestRun_TwoScanPathEntries_EventsAttributedToTheRightScanRoot(t *testing.T) {
	rootMain := t.TempDir()
	rootBackup := t.TempDir()
	writeFile(t, filepath.Join(rootMain, "session-main.jsonl"),
		usageLine("r1", "msg_main", "claude-sonnet-4-5", "text", 10, 5, 0, 0, "/home/thw-home/.typ-crews/freya", "session-main", "2026-09-10T12:00:00Z")+"\n")
	writeFile(t, filepath.Join(rootBackup, "session-backup.jsonl"),
		usageLine("r1", "msg_backup", "claude-sonnet-4-5", "text", 20, 10, 0, 0, "/home/thw-home/.typ-crews/freya", "session-backup", "2026-09-10T12:00:00Z")+"\n")

	poster := &fakePoster{}
	cfg := Config{
		ScanPaths: []ScanPath{
			{Name: "main", Path: rootMain, SourceType: "claude_code"},
			{Name: "backup-install", Path: rootBackup, SourceType: "claude_code"},
		},
		InstallID: "install-abc",
	}
	result, err := Run(cfg, filepath.Join(t.TempDir(), "state.json"), filepath.Join(t.TempDir(), "pathstore.db"), fakeResolver(""), "test-host", poster)
	require.NoError(t, err)
	assert.Equal(t, 2, result.NewRowsFound)
	require.Len(t, poster.posted, 2)

	byID := map[string]Event{}
	for _, e := range poster.posted {
		byID[e.ID] = e
	}
	assert.Equal(t, rootMain, byID["msg_main"].ScanRoot, "msg_main's transcript was found under rootMain")
	assert.Equal(t, rootBackup, byID["msg_backup"].ScanRoot, "msg_backup's transcript was found under rootBackup")

	require.Len(t, poster.postedScanRoots, 2, "both configured scan roots produced an event this run")
	byPath := map[string]ScanRootReport{}
	for _, r := range poster.postedScanRoots {
		byPath[r.Path] = r
	}
	assert.Equal(t, ScanRootReport{Path: rootMain, Name: "main", SourceType: "claude_code"}, byPath[rootMain])
	assert.Equal(t, ScanRootReport{Path: rootBackup, Name: "backup-install", SourceType: "claude_code"}, byPath[rootBackup])
}

func TestRun_ActorUnknownFallsBackToUnknownMarker(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "session-1.jsonl"),
		usageLine("r1", "msg_1", "claude-sonnet-4-5", "text", 10, 5, 0, 0, "/home/thw-home/gits/some-repo", "session-1", "2026-09-10T12:00:00Z")+"\n")

	poster := &fakePoster{}
	cfg := Config{ScanPaths: []ScanPath{{Name: "main", Path: root, SourceType: "claude_code"}}, InstallID: "install-abc"}
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
	cfg := Config{ScanPaths: []ScanPath{{Name: "main", Path: root, SourceType: "claude_code"}}, InstallID: "install-abc"}
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
	cfg := Config{ScanPaths: []ScanPath{{Name: "main", Path: root, SourceType: "claude_code"}}, InstallID: "install-abc"}
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

// TestRun_MultipleCrewHomesUnderOneScanRoot_AllFoundAndDistinctlyAttributed
// is ticket 19's own regression test: earlier this session, a real
// production collector run was manually pointed at a scan root
// containing many crews' subdirectories, and a stale config file went
// unnoticed for a full round-trip because nothing in this suite ever
// exercised "one scan root containing multiple distinct crew-home
// subdirectories" — every fixture before this one only ever covered one
// session/crew at a time. Directory shape mirrors the real Claude Code
// project-directory naming convention (cwd with "/" replaced by "-":
// /home/x/.typ-crews/alice -> -home-x-.typ-crews-alice), with 3 distinct
// crews nested under one single scan root (not 3 separate ScanPaths
// entries — that multi-root case is already covered by
// TestFindTranscriptFiles_MultipleScanPaths).
func TestRun_MultipleCrewHomesUnderOneScanRoot_AllFoundAndDistinctlyAttributed(t *testing.T) {
	root := t.TempDir()
	crews := []string{"alice", "bob", "carol"}
	for _, crew := range crews {
		projectDir := filepath.Join(root, ".claude", "projects", "-home-x-.typ-crews-"+crew)
		cwd := "/home/x/.typ-crews/" + crew
		sessionID := "session-" + crew
		writeFile(t, filepath.Join(projectDir, sessionID+".jsonl"),
			usageLine("r1", "msg_"+crew, "claude-sonnet-4-5", "text", 10, 5, 0, 0, cwd, sessionID, "2026-09-10T12:00:00Z")+"\n")
	}

	poster := &fakePoster{}
	cfg := Config{ScanPaths: []ScanPath{{Name: "main", Path: root, SourceType: "claude_code"}}, InstallID: "install-abc"}
	result, err := Run(cfg, filepath.Join(t.TempDir(), "state.json"), filepath.Join(t.TempDir(), "pathstore.db"), fakeResolver(""), "test-host", poster)
	require.NoError(t, err)

	require.Equal(t, 3, result.FilesScanned, "must find all 3 crews' transcript files under the one scan root, not just the first alphabetically or just one")
	require.Len(t, poster.posted, 3)

	actorBySession := map[string]string{}
	for _, e := range poster.posted {
		actorBySession[e.SessionID] = e.Actor
	}
	require.Len(t, actorBySession, 3, "all 3 sessions must be present")
	assert.Equal(t, "alice", actorBySession["session-alice"])
	assert.Equal(t, "bob", actorBySession["session-bob"])
	assert.Equal(t, "carol", actorBySession["session-carol"])

	// Each actor must be its own distinct value — not collapsed into one
	// shared actor, and not just the first one repeated for all three.
	seenActors := map[string]bool{}
	for _, a := range actorBySession {
		assert.False(t, seenActors[a], "actor %q must not repeat across distinct crew sessions", a)
		seenActors[a] = true
	}
	assert.Len(t, seenActors, 3, "3 sessions must resolve to 3 distinct actors")
}
