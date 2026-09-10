// Package internal — the `*-api` skill doc (`.claude/skills/my-token-api/`)
// is the only thing an agent reading this repo actually consults to call
// the API — a doc that still describes a deleted domain sends an agent
// straight into 404s (docs/GETTING-STARTED.md's own warning). This file
// is that doc's own coverage check: story-1/ticket-16 retired this
// file's original milestone-4-specific checks (which verified that
// milestone's own todo-domain rewrite of the doc) along with the domain
// they verified, and replaced them with the equivalent pair for this
// fork's own real domain — an absence check (the deleted domain must not
// still be documented) and a presence check (the real one must be).
package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skillDocFiles is the exact set of files this check reads — not a glob,
// so a file silently added to (or removed from) the skill directory
// without updating this list is visible in a diff, not swallowed by a
// pattern match.
var skillDocFiles = []string{
	filepath.Join("SKILL.md"),
	filepath.Join("references", "endpoints.md"),
	filepath.Join("references", "errors.md"),
}

// skillDocDir resolves the agent-facing API skill's own directory,
// relative to the repo root, by scanning .claude/skills/ for exactly one
// "*-api" directory rather than hardcoding "my-template-api" — a fork
// renames that directory per GETTING-STARTED.md Step 3, and a hardcoded
// name here broke this file's own coverage check the moment a fork did
// what the doc told it to (found by the first real fork; domainModuleNames
// in architecture_test.go is the same discover-don't-hardcode shape, one
// level down). Failing loudly on zero or on more than one match, the same
// as require.NoErrorf below does for a missing file — an ambiguous match
// is not the same claim as a resolved one.
func skillDocDir(t *testing.T, root string) string {
	t.Helper()
	skillsDir := filepath.Join(root, ".claude", "skills")
	entries, err := os.ReadDir(skillsDir)
	require.NoErrorf(t, err, "reading %s", skillsDir)

	var matches []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), "-api") {
			matches = append(matches, entry.Name())
		}
	}
	require.Lenf(t, matches, 1, "expected exactly one *-api directory under %s, found %v", skillsDir, matches)
	return filepath.Join(skillsDir, matches[0])
}

// readSkillDocs reads every skillDocFiles entry and returns their
// combined content plus each file's own individual content, failing
// loudly (not skipping) if the directory or any named file is missing —
// the same "a missing directory is not the same claim as scanned and
// found nothing" distinction I20's own static check draws.
func readSkillDocs(t *testing.T, root string) (combined string, perFile map[string]string) {
	t.Helper()
	dir := skillDocDir(t, root)

	info, err := os.Stat(dir)
	require.NoErrorf(t, err, "%s must exist for this check to mean anything", dir)
	require.Truef(t, info.IsDir(), "%s exists but is not a directory", dir)

	perFile = make(map[string]string, len(skillDocFiles))
	for _, rel := range skillDocFiles {
		path := filepath.Join(dir, rel)
		data, err := os.ReadFile(path)
		require.NoErrorf(t, err, "reading %s", path)
		content := string(data)
		require.GreaterOrEqualf(t, len(content), 200,
			"%s is suspiciously short (%d bytes) — a doc this thin was probably not actually "+
				"loaded/found, not genuinely reviewed and trimmed; the absence check below would pass "+
				"trivially against near-empty content", path, len(content))
		perFile[rel] = content
		combined += content
	}
	return combined, perFile
}

// TestSkillDocDoesNotDocumentTheDeletedExampleDomain is story-1/
// ticket-16's own check, replacing this file's former milestone-4
// Done-when-6 checks (retired along with the domain those verified —
// see below): the skill doc no longer tells an agent to call `/todos`
// endpoints that no longer exist. A doc that still described a deleted
// domain would send an agent following it straight into real 404s
// (docs/GETTING-STARTED.md's own warning about exactly this).
func TestSkillDocDoesNotDocumentTheDeletedExampleDomain(t *testing.T) {
	root := repoRoot(t)
	combined, _ := readSkillDocs(t, root)

	assert.NotContainsf(t, strings.ToLower(combined), "todo",
		"the skill doc must not reference the deleted example domain — "+
			"an agent following this skill instead of reading the real openapi.yaml "+
			"would get 404s against paths that don't exist (story-1/ticket-16)")
}

// TestSkillDocDocumentsTheRealDomain is the positive half: an absence
// check alone cannot see "a doc that removed the old domain and
// documented nothing in its place" — this fork's own real domain
// (story-1/ticket-11's usage-events ingestion) must actually be there.
func TestSkillDocDocumentsTheRealDomain(t *testing.T) {
	root := repoRoot(t)
	combined, _ := readSkillDocs(t, root)

	for _, must := range []string{
		"/api/v1/usage-events/batch",
		"install_id",
		"received",
		"inserted",
	} {
		assert.Containsf(t, combined, must,
			"the skill doc must document %q — this fork's own real domain "+
				"(story-1/ticket-11's usage-events ingestion)", must)
	}
}
