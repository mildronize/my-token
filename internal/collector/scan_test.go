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

	got, err := FindTranscriptFiles([]string{root})
	require.NoError(t, err)
	sort.Strings(got)

	require.Len(t, got, 2, "must find both the top-level session file and the nested subagent file, and skip the .meta.json")
	require.Contains(t, got, sessionFile)
	require.Contains(t, got, subagentFile)
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

	got, err := FindTranscriptFiles([]string{rootA, rootB})
	require.NoError(t, err)
	sort.Strings(got)

	require.Len(t, got, 2)
	require.Contains(t, got, fileA)
	require.Contains(t, got, fileB)
}

// TestFindTranscriptFiles_MissingScanPathIsNotFatal — a configured scan
// path that doesn't exist (e.g. ~/.claude-local never installed) must
// not abort scanning the other configured paths.
func TestFindTranscriptFiles_MissingScanPathIsNotFatal(t *testing.T) {
	root := t.TempDir()
	fileA := filepath.Join(root, "a.jsonl")
	writeFile(t, fileA, "{}\n")

	got, err := FindTranscriptFiles([]string{filepath.Join(root, "does-not-exist"), root})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Contains(t, got, fileA)
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

	got, err := FindTranscriptFiles([]string{"~/.claude"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Contains(t, got, fileUnderHome)
}

func TestExpandHome(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	assert.Equal(t, fakeHome, expandHome("~"))
	assert.Equal(t, filepath.Join(fakeHome, ".claude"), expandHome("~/.claude"))
	assert.Equal(t, "/absolute/path", expandHome("/absolute/path"), "a path with no leading ~ is returned unchanged")
	assert.Equal(t, "relative/path", expandHome("relative/path"), "a relative path with no leading ~ is returned unchanged")
}
