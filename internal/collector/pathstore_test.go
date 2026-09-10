package collector

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openTestPathStore opens a fresh PathStore backed by a real SQLite file
// in a t.TempDir() — real SQLite, not faked, consistent with how every
// other repo/store in this repo (e.g. internal/domain/usage's own
// testutil) tests its persistence layer against a real temp database. The
// contract's "inject a fake ... resolver (no real subprocess)" testing
// decision is specifically about GitRootResolver/`git`, not about this
// store's own SQLite usage.
func openTestPathStore(t *testing.T) *PathStore {
	t.Helper()
	store, err := OpenPathStore(filepath.Join(t.TempDir(), "pathstore.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestPathStore_GitRootCache_RoundTrips(t *testing.T) {
	store := openTestPathStore(t)

	got, err := store.AllGitRoots()
	require.NoError(t, err)
	assert.Empty(t, got, "a fresh store has no cached entries yet")

	require.NoError(t, store.SaveGitRoots(map[string]string{
		"/home/thw-home/gits/my-template/internal/collector": "/home/thw-home/gits/my-template",
		"/home/thw-home/.typ-crews/luna":                     "/home/thw-home/.typ-crews/luna",
	}))

	got, err = store.AllGitRoots()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"/home/thw-home/gits/my-template/internal/collector": "/home/thw-home/gits/my-template",
		"/home/thw-home/.typ-crews/luna":                     "/home/thw-home/.typ-crews/luna",
	}, got)
}

func TestPathStore_SaveGitRoots_UpsertsExistingDirectory(t *testing.T) {
	store := openTestPathStore(t)

	require.NoError(t, store.SaveGitRoots(map[string]string{"/a/b": "/a"}))
	require.NoError(t, store.SaveGitRoots(map[string]string{"/a/b": "/a/new-root"}))

	got, err := store.AllGitRoots()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"/a/b": "/a/new-root"}, got)
}

func TestPathStore_Votes_AddIncrementsRatherThanReplaces(t *testing.T) {
	store := openTestPathStore(t)

	require.NoError(t, store.AddVotes("session-1", map[string]int64{
		"/home/thw-home/gits/my-template": 5,
		"/home/thw-home/.typ-crews/luna":  1,
	}))
	require.NoError(t, store.AddVotes("session-1", map[string]int64{
		"/home/thw-home/gits/my-template": 3, // a second, later scan pass finding more touched paths
	}))

	counts, err := store.VoteCounts("session-1")
	require.NoError(t, err)
	assert.Equal(t, map[string]int64{
		"/home/thw-home/gits/my-template": 8,
		"/home/thw-home/.typ-crews/luna":  1,
	}, counts)
}

func TestPathStore_Votes_AreIsolatedPerSession(t *testing.T) {
	store := openTestPathStore(t)

	require.NoError(t, store.AddVotes("session-1", map[string]int64{"/repo-a": 10}))
	require.NoError(t, store.AddVotes("session-2", map[string]int64{"/repo-b": 4}))

	countsForSessionThatHasNoVotesYet, err := store.VoteCounts("session-3")
	require.NoError(t, err)
	assert.Empty(t, countsForSessionThatHasNoVotesYet)

	counts1, err := store.VoteCounts("session-1")
	require.NoError(t, err)
	assert.Equal(t, map[string]int64{"/repo-a": 10}, counts1)
}

func TestPathStore_ScanOffset_DefaultsToZeroThenPersists(t *testing.T) {
	store := openTestPathStore(t)

	offset, err := store.ScanOffset("session-1", "/path/to/session-1.jsonl")
	require.NoError(t, err)
	assert.Equal(t, int64(0), offset, "never-scanned file/session pair starts at offset 0")

	require.NoError(t, store.SetScanOffset("session-1", "/path/to/session-1.jsonl", 42))

	offset, err = store.ScanOffset("session-1", "/path/to/session-1.jsonl")
	require.NoError(t, err)
	assert.Equal(t, int64(42), offset)

	// A second file for the same session tracks its own independent
	// offset — a session's transcript can be more than one physical file
	// (main + subagents/agent-*.jsonl, ticket 9's scan scope).
	otherOffset, err := store.ScanOffset("session-1", "/path/to/session-1/subagents/agent-x.jsonl")
	require.NoError(t, err)
	assert.Equal(t, int64(0), otherOffset)
}
