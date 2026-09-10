package publicapi

import (
	"database/sql"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mildronize/my-template/internal/collector"
	"github.com/mildronize/my-template/internal/domain/usage"
)

// usageLine mirrors internal/collector's own test helper of the same
// name (transcript_test.go) — duplicated rather than imported, since
// that helper is unexported test-only code in a different package. Its
// shape is the real Claude Code transcript record shape confirmed
// against a live transcript on this host: message.id, message.model,
// message.usage's four counters, top-level cwd/sessionId/timestamp.
func usageLine(recordUUID, messageID, model, contentType string, input, output, cacheRead, cacheCreation int64, cwd, sessionID, ts string) string {
	itoa := strconv.FormatInt
	return `{"uuid":"` + recordUUID + `","sessionId":"` + sessionID + `","cwd":"` + cwd + `","timestamp":"` + ts + `",` +
		`"message":{"id":"` + messageID + `","model":"` + model + `",` +
		`"content":[{"type":"` + contentType + `"}],` +
		`"usage":{"input_tokens":` + itoa(input, 10) + `,"output_tokens":` + itoa(output, 10) +
		`,"cache_read_input_tokens":` + itoa(cacheRead, 10) + `,"cache_creation_input_tokens":` + itoa(cacheCreation, 10) + `}}}`
}

// TestCollectorIntegration_RunsAgainstTicket11sRealEndpoint is ticket 12's
// own required "one real integration test": a fixture transcript
// directory (never real crew data), run through the actual collector
// pipeline (internal/collector.Run), POSTed over real HTTP to a live
// instance of ticket 11's own handler stack (newIntegrationRouter — the
// exact same wiring cmd/server's buildHandler assembles, not a hand-rolled
// substitute), and confirmed by querying the real database directly
// afterward — external behavior end to end, matching the contract's
// Testing Decisions ("test external behavior via the API surface").
//
// Lives in this package (not internal/collector) specifically so it can
// reuse newIntegrationRouter/createAgentWithKey — the same real
// integration harness usage_handler_test.go already uses for ticket 11's
// own acceptance tests — rather than re-implementing a second copy of
// "a real router against a real temp SQLite database" elsewhere.
func TestCollectorIntegration_RunsAgainstTicket11sRealEndpoint(t *testing.T) {
	router, conn := newIntegrationRouter(t)
	_, rawKey := createAgentWithKey(t, conn, "collector-integration")

	server := httptest.NewServer(router)
	defer server.Close()

	// A small fixture transcript directory — never real crew data. One
	// session, with the exact multi-line-per-message.id shape ticket 9
	// found (msg_turn1 split across thinking+text+tool_use lines) plus a
	// second, genuinely distinct turn, plus a subagent transcript nested
	// under <session-uuid>/subagents/ (ticket 9/10 — must not be
	// silently dropped).
	scanRoot := t.TempDir()
	sessionID := "fixture-session-1"
	sessionFile := filepath.Join(scanRoot, sessionID+".jsonl")
	writeFixture(t, sessionFile, ""+
		usageLine("r1", "msg_turn1", "claude-sonnet-4-5", "thinking", 100, 50, 10, 5, "/home/thw-home/.typ-crews/freya", sessionID, "2026-09-10T12:00:00Z")+"\n"+
		usageLine("r2", "msg_turn1", "claude-sonnet-4-5", "text", 100, 50, 10, 5, "/home/thw-home/.typ-crews/freya", sessionID, "2026-09-10T12:00:01Z")+"\n"+
		usageLine("r3", "msg_turn1", "claude-sonnet-4-5", "tool_use", 100, 50, 10, 5, "/home/thw-home/.typ-crews/freya", sessionID, "2026-09-10T12:00:02Z")+"\n"+
		usageLine("r4", "msg_turn2", "claude-sonnet-4-5", "text", 200, 80, 0, 0, "/home/thw-home/.typ-crews/freya", sessionID, "2026-09-10T12:01:00Z")+"\n")

	subagentFile := filepath.Join(scanRoot, sessionID, "subagents", "agent-fixture1.jsonl")
	writeFixture(t, subagentFile,
		usageLine("r5", "msg_subagent1", "claude-haiku-4-5", "text", 30, 15, 0, 0, "/home/thw-home/.typ-crews/freya", sessionID, "2026-09-10T12:02:00Z")+"\n")

	cfg := collector.Config{
		ScanPaths: []string{scanRoot},
		InstallID: "install-integration-test",
		CoreURL:   server.URL,
		APIKey:    rawKey,
	}
	client := collector.NewClient(server.URL, rawKey, cfg.InstallID, "test-host")
	statePath := filepath.Join(t.TempDir(), "state.json")

	// Never-a-git-repo resolver — scanRoot is a plain temp dir, so the
	// simple path method's real git-root fallback chain (path.go) is
	// exercised for real here, not stubbed.
	result, err := collector.Run(cfg, statePath, collector.RealGitRootResolver, "test-host", client)
	require.NoError(t, err)

	assert.Equal(t, 3, result.NewRowsFound, "msg_turn1's three content-block lines dedupe to 1, plus msg_turn2, plus the subagent's msg_subagent1 — 3 real turns, not 5 lines")
	assert.Equal(t, int64(3), result.Inserted)

	assert.Equal(t, 3, countUsageEventRows(t, conn), "3 real rows landed in the real usage_events table via a real HTTP POST")

	assertRow(t, conn, "msg_turn1", "freya", int64(100), int64(50), int64(10), int64(5))
	assertRow(t, conn, "msg_turn2", "freya", int64(200), int64(80), int64(0), int64(0))
	assertRow(t, conn, "msg_subagent1", "freya", int64(30), int64(15), int64(0), int64(0))

	// path falls back to the raw cwd (not a git repo): the fixture's own
	// cwd, "/home/thw-home/.typ-crews/freya" — proves the simple method's
	// fallback chain ran for real against a real (non-git) directory, not
	// just against an injected fake.
	var gotPath string
	require.NoError(t, conn.QueryRow(`SELECT path FROM usage_events WHERE id = ?`, "msg_turn1").Scan(&gotPath))
	assert.Equal(t, "/home/thw-home/.typ-crews/freya", gotPath)

	// cost is server-computed (ticket 11 — the collector never sends one),
	// via the exact same pricing table internal/domain/usage.CostForUsage
	// implements. Contract's Testing Decisions explicitly names cost as
	// part of what this end-to-end pipeline test must confirm.
	var gotCost float64
	require.NoError(t, conn.QueryRow(`SELECT cost FROM usage_events WHERE id = ?`, "msg_turn1").Scan(&gotCost))
	assert.InDelta(t, usage.CostForUsage("claude-sonnet-4-5", 100, 50, 10, 5), gotCost, 1e-9)

	// Re-running against the same fixture (and the same server/db) sends
	// nothing new — ticket 12's own local sent-state tracking, on top of
	// (not instead of) the server's own idempotency.
	result2, err := collector.Run(cfg, statePath, collector.RealGitRootResolver, "test-host", client)
	require.NoError(t, err)
	assert.Equal(t, 0, result2.NewRowsFound)
	assert.Equal(t, 3, countUsageEventRows(t, conn), "row count must not grow on a re-run")
}

// TestCollectorIntegration_PathResolvesToRealGitRoot is the contract's
// Testing Decisions' other named integration shape: "a temp git repo +
// a fixture transcript." The first test above deliberately exercises the
// simple method's *fallback* branch (a non-git scan directory); this one
// exercises its *success* branch against a real `git` subprocess (via
// collector.RealGitRootResolver, not an injected fake) — proving
// git-root canonicalization actually happens end to end, not just that
// the fallback works when it doesn't.
func TestCollectorIntegration_PathResolvesToRealGitRoot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	router, conn := newIntegrationRouter(t)
	_, rawKey := createAgentWithKey(t, conn, "collector-git-integration")
	server := httptest.NewServer(router)
	defer server.Close()

	repoRoot := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", repoRoot, "init", "-q").Run())
	// The session's cwd is a subdirectory of the repo, not the repo root
	// itself — proves this resolves to the repo's toplevel, not just
	// echoes cwd back unchanged.
	nestedCwd := filepath.Join(repoRoot, "internal", "collector")
	require.NoError(t, os.MkdirAll(nestedCwd, 0o755))

	scanRoot := t.TempDir()
	sessionID := "fixture-session-git"
	writeFixture(t, filepath.Join(scanRoot, sessionID+".jsonl"),
		usageLine("r1", "msg_git1", "claude-sonnet-4-5", "text", 10, 5, 0, 0, nestedCwd, sessionID, "2026-09-10T12:00:00Z")+"\n")

	cfg := collector.Config{
		ScanPaths: []string{scanRoot},
		InstallID: "install-git-integration-test",
		CoreURL:   server.URL,
		APIKey:    rawKey,
	}
	client := collector.NewClient(server.URL, rawKey, cfg.InstallID, "test-host")
	statePath := filepath.Join(t.TempDir(), "state.json")

	result, err := collector.Run(cfg, statePath, collector.RealGitRootResolver, "test-host", client)
	require.NoError(t, err)
	assert.Equal(t, 1, result.NewRowsFound)

	wantRoot, err := collector.RealGitRootResolver(nestedCwd)
	require.NoError(t, err, "sanity check: the temp dir this test just `git init`'d must itself resolve as a git repo")

	var gotPath string
	require.NoError(t, conn.QueryRow(`SELECT path FROM usage_events WHERE id = ?`, "msg_git1").Scan(&gotPath))
	assert.Equal(t, wantRoot, gotPath, "path must be the git toplevel, not the raw nested cwd")
	assert.NotEqual(t, nestedCwd, gotPath, "a real git repo must NOT fall back to the raw cwd")
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func assertRow(t *testing.T, conn *sql.DB, id, wantActor string, wantInput, wantOutput, wantCacheRead, wantCacheCreation int64) {
	t.Helper()
	var actor string
	var input, output, cacheRead, cacheCreation int64
	var createdAt time.Time
	require.NoError(t, conn.QueryRow(
		`SELECT actor, input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens, created_at FROM usage_events WHERE id = ?`,
		id,
	).Scan(&actor, &input, &output, &cacheRead, &cacheCreation, &createdAt))
	assert.Equal(t, wantActor, actor, "id=%s actor", id)
	assert.Equal(t, wantInput, input, "id=%s input_tokens", id)
	assert.Equal(t, wantOutput, output, "id=%s output_tokens", id)
	assert.Equal(t, wantCacheRead, cacheRead, "id=%s cache_read_input_tokens", id)
	assert.Equal(t, wantCacheCreation, cacheCreation, "id=%s cache_creation_input_tokens", id)
	assert.False(t, createdAt.IsZero(), "id=%s created_at must reflect the collector-supplied timestamp, not a zero value", id)
}
