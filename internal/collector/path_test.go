package collector

import (
	"errors"
	"strings"
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

// ticket10MultiTreeFixture reconstructs (at greatly reduced scale) the
// exact shape of ticket 10 §7's real luna evidence: one dominant tree plus
// several minor ones, none of which should ever be able to outvote the
// dominant tree. Real counts were 1465/33/13/9/7 (94.8% dominant); this
// fixture keeps the same relative ordering and a similarly lopsided split
// while staying small enough to read at a glance.
func ticket10MultiTreeFixture() []string {
	var paths []string
	add := func(dir string, n int) {
		for i := 0; i < n; i++ {
			paths = append(paths, dir+"/file.go")
		}
	}
	add("/home/thw-home/gits/my-template", 95) // dominant — must win
	add("/home/thw-home/.typ-crews/luna", 33)
	add("/home/thw-home/gits/my-task", 13)
	add("/home/thw-home/.typmem", 9)
	add("/home/thw-home/gits/prod-thw-home", 7)
	return paths
}

// gitRootByPrefixResolver is a fake GitRootResolver (no real subprocess,
// per the contract's Testing Decisions) that resolves any directory
// underneath one of roots to that root — enough to exercise
// TallyGitRoots/MajorityGitRoot against realistic multi-file touched-path
// fixtures without needing a real git repo per root.
func gitRootByPrefixResolver(roots ...string) GitRootResolver {
	return func(dir string) (string, error) {
		for _, root := range roots {
			if dir == root || strings.HasPrefix(dir, root+"/") {
				return root, nil
			}
		}
		return "", errors.New("not a git repository (fake resolver)")
	}
}

// TestResolveSessionPathTouched_MajorityVoteWinsOverFourOtherRealTrees is
// ticket 10 §7's corrected aggregation, verified: a session that
// genuinely touches five different real (non-scratch) trees at once still
// correctly attributes to the dominant one, by vote count — not a
// literal common-directory/intersection across all of them (which ticket
// 10 found collapses to "/" on exactly this kind of fixture).
func TestResolveSessionPathTouched_MajorityVoteWinsOverFourOtherRealTrees(t *testing.T) {
	resolver := gitRootByPrefixResolver(
		"/home/thw-home/gits/my-template",
		"/home/thw-home/.typ-crews/luna",
		"/home/thw-home/gits/my-task",
		"/home/thw-home/.typmem",
		"/home/thw-home/gits/prod-thw-home",
	)

	path, ok := ResolveSessionPathTouched(ticket10MultiTreeFixture(), resolver)
	require.True(t, ok)
	assert.Equal(t, "/home/thw-home/gits/my-template", path, "the dominant tree must win the majority vote despite four other real trees being touched in the same session")
}

// TestTallyGitRoots_DropsTmpScratchBeforeGitRooting is ticket 13 step 2:
// /tmp paths are excluded outright, before any git-root attempt at all —
// even one that (in this fake resolver) would resolve to a real root if
// it were ever asked.
func TestTallyGitRoots_DropsTmpScratchBeforeGitRooting(t *testing.T) {
	resolver := fakeGitRootResolver(map[string]string{
		"/tmp/claude-1000/scratchpad": "/should/never/be/counted",
		"/home/thw-home/gits/my-template": "/home/thw-home/gits/my-template",
	})

	counts := TallyGitRoots([]string{
		"/tmp/claude-1000/scratchpad/notes.html",
		"/home/thw-home/gits/my-template/file.go",
	}, resolver)

	assert.Equal(t, map[string]int64{"/home/thw-home/gits/my-template": 1}, counts)
}

// TestResolveSessionPathFull_AllScratchFallsBackToCwdMethod is the
// "all-scratch" fallback-chain edge case (ticket 10 §9.3 / the outer
// task's own wording): every touched path was scratch (or there were none
// at all) — ResolveSessionPathTouched must find no real candidate, and
// ResolveSessionPathFull must fall back to ticket 12's simple cwd method,
// not "(unknown)".
func TestResolveSessionPathFull_AllScratchFallsBackToCwdMethod(t *testing.T) {
	resolver := fakeGitRootResolver(map[string]string{
		"/home/thw-home/.typ-crews/freya": "/home/thw-home/.typ-crews/freya-repo",
	})

	touched := []string{"/tmp/a.txt", "/tmp/b/c.txt"}
	path := ResolveSessionPathFull(touched, "/home/thw-home/.typ-crews/freya", true, resolver)
	assert.Equal(t, "/home/thw-home/.typ-crews/freya-repo", path, "no real touched-path candidate — must fall back to the cwd method, which here resolves to a real git root")
}

// TestResolveSessionPathFull_NoTouchedPathsAtAllFallsBackToCwdMethod
// covers the "no filesystem-touching tool calls in the session" half of
// ticket 10 §9.3 — an empty touchedPaths slice, not just an all-scratch
// one.
func TestResolveSessionPathFull_NoTouchedPathsAtAllFallsBackToCwdMethod(t *testing.T) {
	resolver := fakeGitRootResolver(map[string]string{})
	path := ResolveSessionPathFull(nil, "/home/thw-home/.typ-crews/freya", true, resolver)
	assert.Equal(t, "/home/thw-home/.typ-crews/freya", path, "no touched paths and not a git repo — raw cwd is the fallback")
}

// TestResolveSessionPathFull_RealNonGitTouchedPathUsesRawPathNotCwd is the
// "a real non-git path" fallback-chain edge case: exactly one real,
// non-scratch touched path, which resolver genuinely can't place inside a
// git repo — ResolveSessionPathTouched must still succeed (ok=true), using
// the touched path's own directory as its root, and ResolveSessionPathFull
// must return that — never falling through to the session's (different)
// cwd.
func TestResolveSessionPathFull_RealNonGitTouchedPathUsesRawPathNotCwd(t *testing.T) {
	resolver := fakeGitRootResolver(map[string]string{}) // resolves nothing — a plain non-git directory

	touched := []string{"/home/thw-home/data/plain-install/config.json"}
	path := ResolveSessionPathFull(touched, "/home/thw-home/.typ-crews/freya", true, resolver)
	assert.Equal(t, "/home/thw-home/data/plain-install", path, "a real, non-git touched path must win via its own directory, not fall through to the session's unrelated cwd")
}

// TestResolveSessionPathFull_NoTouchedPathsAndNoCwdFallsBackToUnknown is
// ticket 10 §9's final fallback: neither method produces anything usable
// at all.
func TestResolveSessionPathFull_NoTouchedPathsAndNoCwdFallsBackToUnknown(t *testing.T) {
	resolver := fakeGitRootResolver(map[string]string{})
	path := ResolveSessionPathFull(nil, "", false, resolver)
	assert.Equal(t, "(unknown)", path)
}

// TestCachedGitRootResolver_NeverCallsResolverTwiceForTheSameDirectory is
// contract's git_root_cache behavior (ticket 13 step 4): a directory is
// never re-resolved via a git rev-parse subprocess call twice.
func TestCachedGitRootResolver_NeverCallsResolverTwiceForTheSameDirectory(t *testing.T) {
	calls := map[string]int{}
	resolver := func(dir string) (string, error) {
		calls[dir]++
		return "/home/thw-home/gits/my-template", nil
	}

	cache := NewGitRootCache()
	cached := CachedGitRootResolver(resolver, cache)

	for i := 0; i < 3; i++ {
		root, err := cached("/home/thw-home/gits/my-template/internal/collector")
		require.NoError(t, err)
		assert.Equal(t, "/home/thw-home/gits/my-template", root)
	}

	assert.Equal(t, 1, calls["/home/thw-home/gits/my-template/internal/collector"], "the underlying resolver must be called at most once per directory")
}

// TestCachedGitRootResolver_SeedPreventsAnyCallAtAll proves Seed (loaded
// from PathStore's persisted git_root_cache at the start of a run) skips
// the wrapped resolver entirely for an already-known directory, even on
// the very first lookup in a fresh process.
func TestCachedGitRootResolver_SeedPreventsAnyCallAtAll(t *testing.T) {
	calls := 0
	resolver := func(dir string) (string, error) {
		calls++
		return "should-not-be-used", nil
	}

	cache := NewGitRootCache()
	cache.Seed(map[string]string{"/home/thw-home/gits/my-template/internal": "/home/thw-home/gits/my-template"})
	cached := CachedGitRootResolver(resolver, cache)

	root, err := cached("/home/thw-home/gits/my-template/internal")
	require.NoError(t, err)
	assert.Equal(t, "/home/thw-home/gits/my-template", root)
	assert.Equal(t, 0, calls, "a seeded directory must never reach the wrapped resolver")
}

// TestMajorityGitRoot_EmptyCountsIsNotOk covers MajorityGitRoot's own
// contract in isolation, independent of TallyGitRoots.
func TestMajorityGitRoot_EmptyCountsIsNotOk(t *testing.T) {
	_, ok := MajorityGitRoot(map[string]int64{})
	assert.False(t, ok)
}
