package collector

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// scannedPaths extracts just the discovered file paths from a
// []ScannedFile result, sorted, for tests that don't care about
// scan-root attribution.
func scannedPaths(t *testing.T, files []ScannedFile) []string {
	t.Helper()
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	sort.Strings(paths)
	return paths
}

// TestFindTranscriptFiles_IncludesNestedSubagentTranscripts is ticket 9/10's
// own regression: "a non-recursive scan silently drops subagent usage" —
// asserts a *.jsonl file nested under <session-uuid>/subagents/agent-*.jsonl
// is found, not just top-level session-*.jsonl files.
func TestFindTranscriptFiles_IncludesNestedSubagentTranscripts(t *testing.T) {
	root := t.TempDir()
	sessionFile := filepath.Join(root, "session-1.jsonl")
	subagentFile := filepath.Join(root, "session-1", "subagents", "agent-abc123.jsonl")
	nonJSONL := filepath.Join(root, "session-1", "subagents", "agent-abc123.meta.json")

	writeFile(t, sessionFile, "{}\n")
	writeFile(t, subagentFile, "{}\n")
	writeFile(t, nonJSONL, "{}\n")

	got, err := FindTranscriptFiles([]ScanPath{{Name: "main", Path: root, SourceType: "claude_code"}})
	require.NoError(t, err)
	paths := scannedPaths(t, got)

	require.Len(t, paths, 2, "must find both the top-level session file and the nested subagent file, and skip the .meta.json")
	require.Contains(t, paths, sessionFile)
	require.Contains(t, paths, subagentFile)
	for _, f := range got {
		assert.Equal(t, root, f.ScanRoot)
		assert.Equal(t, "main", f.Name)
		assert.Equal(t, "claude_code", f.SourceType)
	}
}

// TestFindTranscriptFiles_MultipleScanPaths covers the goal's own
// requirement — multiple scan roots (e.g. ~/.claude and ~/.claude-local)
// on one install, not just one hardcoded root.
func TestFindTranscriptFiles_MultipleScanPaths(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	fileA := filepath.Join(rootA, "a.jsonl")
	fileB := filepath.Join(rootB, "nested", "b.jsonl")
	writeFile(t, fileA, "{}\n")
	writeFile(t, fileB, "{}\n")

	got, err := FindTranscriptFiles([]ScanPath{
		{Name: "main", Path: rootA, SourceType: "claude_code"},
		{Name: "backup-install", Path: rootB, SourceType: "claude_code"},
	})
	require.NoError(t, err)
	paths := scannedPaths(t, got)

	require.Len(t, paths, 2)
	require.Contains(t, paths, fileA)
	require.Contains(t, paths, fileB)

	byPath := map[string]ScannedFile{}
	for _, f := range got {
		byPath[f.Path] = f
	}
	assert.Equal(t, "main", byPath[fileA].Name, "story-2/ticket-8: each discovered file is attributed to its own ScanPath entry")
	assert.Equal(t, rootA, byPath[fileA].ScanRoot)
	assert.Equal(t, "backup-install", byPath[fileB].Name)
	assert.Equal(t, rootB, byPath[fileB].ScanRoot)
}

// TestFindTranscriptFiles_MissingScanPathIsNotFatal — a configured scan
// path that doesn't exist (e.g. ~/.claude-local never installed) must
// not abort scanning the other configured paths.
func TestFindTranscriptFiles_MissingScanPathIsNotFatal(t *testing.T) {
	root := t.TempDir()
	fileA := filepath.Join(root, "a.jsonl")
	writeFile(t, fileA, "{}\n")

	got, err := FindTranscriptFiles([]ScanPath{
		{Name: "missing", Path: filepath.Join(root, "does-not-exist"), SourceType: "claude_code"},
		{Name: "main", Path: root, SourceType: "claude_code"},
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, fileA, got[0].Path)
}

// TestFindTranscriptFiles_ExpandsLeadingTilde — the goal and ticket 12
// both give scan_paths examples as `["~/.claude", "~/.claude-local"]`;
// FindTranscriptFiles must actually scan the caller's home directory for
// such a path, not silently find nothing because os.Stat/filepath.Walk
// never expand `~` themselves. $HOME is overridden to a temp dir for
// this test — never touches a developer's real home directory.
func TestFindTranscriptFiles_ExpandsLeadingTilde(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	fileUnderHome := filepath.Join(fakeHome, ".claude", "projects", "a.jsonl")
	writeFile(t, fileUnderHome, "{}\n")

	got, err := FindTranscriptFiles([]ScanPath{{Name: "main", Path: "~/.claude", SourceType: "claude_code"}})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, fileUnderHome, got[0].Path)
	assert.Equal(t, filepath.Join(fakeHome, ".claude"), got[0].ScanRoot, "ScanRoot is the entry's *resolved* Path — expandHome already applied")
}

// TestFindTranscriptFiles_DefaultsMissingSourceType proves the defensive
// default in FindTranscriptFiles itself (config.go's LoadOrInitConfig
// already applies the same default when loading from disk — this covers
// a caller that builds a ScanPath by hand, e.g. this test, without going
// through LoadOrInitConfig).
func TestFindTranscriptFiles_DefaultsMissingSourceType(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.jsonl"), "{}\n")

	got, err := FindTranscriptFiles([]ScanPath{{Name: "main", Path: root}})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, DefaultSourceType, got[0].SourceType)
}

func TestExpandHome(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	assert.Equal(t, fakeHome, expandHome("~"))
	assert.Equal(t, filepath.Join(fakeHome, ".claude"), expandHome("~/.claude"))
	assert.Equal(t, "/absolute/path", expandHome("/absolute/path"), "a path with no leading ~ is returned unchanged")
	assert.Equal(t, "relative/path", expandHome("relative/path"), "a relative path with no leading ~ is returned unchanged")
}
