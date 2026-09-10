package collector

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// toolUseLine builds one raw JSONL transcript line carrying a single
// tool_use content block — the fixture shape ExtractTouchedPaths reads.
// Marshaled via encoding/json (rather than hand-built strings, as
// usageLine does elsewhere) because a Bash `command` input routinely
// contains quotes/newlines that would be error-prone to hand-escape.
func toolUseLine(t *testing.T, sessionID, toolName string, input map[string]any) string {
	t.Helper()
	inputJSON, err := json.Marshal(input)
	require.NoError(t, err)

	rec := map[string]any{
		"sessionId": sessionID,
		"message": map[string]any{
			"content": []map[string]any{
				{"type": "tool_use", "name": toolName, "input": json.RawMessage(inputJSON)},
			},
		},
	}
	line, err := json.Marshal(rec)
	require.NoError(t, err)
	return string(line)
}

func TestExtractTouchedPaths_ReadWriteEditUseFilePathInput(t *testing.T) {
	lines := []string{
		toolUseLine(t, "s1", "Read", map[string]any{"file_path": "/home/thw-home/gits/my-template/AGENTS.md"}),
		toolUseLine(t, "s1", "Write", map[string]any{"file_path": "/home/thw-home/gits/my-template/bff-openapi.yaml", "content": "..."}),
		toolUseLine(t, "s1", "Edit", map[string]any{"file_path": "/home/thw-home/gits/my-template/internal/collector/path.go", "old_string": "a", "new_string": "b"}),
	}

	got := ExtractTouchedPaths(lines)
	assert.Equal(t, []string{
		"/home/thw-home/gits/my-template/AGENTS.md",
		"/home/thw-home/gits/my-template/bff-openapi.yaml",
		"/home/thw-home/gits/my-template/internal/collector/path.go",
	}, got)
}

func TestExtractTouchedPaths_GlobUsesPathInput(t *testing.T) {
	lines := []string{
		toolUseLine(t, "s1", "Glob", map[string]any{"path": "/home/thw-home/gits/my-template", "pattern": "**/*.go"}),
	}

	got := ExtractTouchedPaths(lines)
	assert.Equal(t, []string{"/home/thw-home/gits/my-template"}, got)
}

func TestExtractTouchedPaths_BashLeadingCdAndAmpersand(t *testing.T) {
	lines := []string{
		toolUseLine(t, "s1", "Bash", map[string]any{"command": "cd /home/thw-home/gits/my-template && git commit -m \"plan: milestone-1\""}),
	}

	got := ExtractTouchedPaths(lines)
	assert.Equal(t, []string{"/home/thw-home/gits/my-template"}, got)
}

func TestExtractTouchedPaths_BashLeadingCdAndSemicolon(t *testing.T) {
	lines := []string{
		toolUseLine(t, "s1", "Bash", map[string]any{"command": "cd /home/thw-home/gits/my-task; go test ./..."}),
	}

	got := ExtractTouchedPaths(lines)
	assert.Equal(t, []string{"/home/thw-home/gits/my-task"}, got)
}

func TestExtractTouchedPaths_BashLeadingCdExpandsTilde(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	lines := []string{
		toolUseLine(t, "s1", "Bash", map[string]any{"command": "cd ~/gits/my-template && git add internal/db_queries_ascii_test.go && git commit -m \"docs\""}),
	}

	got := ExtractTouchedPaths(lines)
	require.Len(t, got, 1)
	assert.Equal(t, fakeHome+"/gits/my-template", got[0])
}

func TestExtractTouchedPaths_BashWithoutLeadingCdYieldsNothing(t *testing.T) {
	lines := []string{
		toolUseLine(t, "s1", "Bash", map[string]any{"command": "git status"}),
		toolUseLine(t, "s1", "Bash", map[string]any{"command": "echo cd /not/a/leading/cd"}),
	}

	got := ExtractTouchedPaths(lines)
	assert.Empty(t, got)
}

func TestExtractTouchedPaths_BashRelativeCdYieldsNothing(t *testing.T) {
	lines := []string{
		toolUseLine(t, "s1", "Bash", map[string]any{"command": "cd subdir && ls"}),
	}

	got := ExtractTouchedPaths(lines)
	assert.Empty(t, got, "a bare relative cd target carries no information about where the shell actually was")
}

func TestExtractTouchedPaths_NonToolUseAndNonTargetToolsAreIgnored(t *testing.T) {
	lines := []string{
		`{"sessionId":"s1","message":{"role":"user","content":"hi"}}`, // content is a plain string, not a block array
		`not even json`,
		toolUseLine(t, "s1", "Skill", map[string]any{"skill": "typmem-recall"}),
		toolUseLine(t, "s1", "TaskCreate", map[string]any{"title": "do the thing"}),
	}

	got := ExtractTouchedPaths(lines)
	assert.Empty(t, got)
}

func TestExtractTouchedPaths_ScratchPathsAreExtractedVerbatim(t *testing.T) {
	// Extraction itself does not filter /tmp scratch noise — that's
	// TallyGitRoots' job (ticket 13 step 2), kept as a separate step so
	// this extraction stays independently testable.
	lines := []string{
		toolUseLine(t, "s1", "Edit", map[string]any{"file_path": "/tmp/claude-1000/scratchpad/notes.html", "old_string": "a", "new_string": "b"}),
	}

	got := ExtractTouchedPaths(lines)
	assert.Equal(t, []string{"/tmp/claude-1000/scratchpad/notes.html"}, got)
}

func TestIsScratchPath(t *testing.T) {
	assert.True(t, isScratchPath("/tmp"))
	assert.True(t, isScratchPath("/tmp/foo/bar.txt"))
	assert.False(t, isScratchPath("/tmpfoo/bar.txt"), "must not match a directory that merely starts with the string tmp")
	assert.False(t, isScratchPath("/home/thw-home/gits/my-template"))
}
