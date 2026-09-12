// story-2/ticket-12: the story's final ticket, replacing the wayfinder's
// original "one real second host" requirement (ticket-2/second-machine-
// proof.md) with a real second collector *process* on this same machine,
// per the loop-readiness review that resolved it. Lives in this package
// (not internal/collector or internal/transport/publicapi) because it is
// the one test in the whole suite that needs BOTH a real /api/v1
// ingestion surface (for collector.Run to POST to) AND a real,
// session-authenticated /api/bff/usage/summary surface (to read the
// result back) against the exact same underlying database — this
// package already owns the session/cookie machinery
// (bff_testutil_test.go's newTestRouter/newFakeIDP/NewSigner) that
// GET /api/bff/usage/summary needs, and already has precedent
// (keys_handler_test.go's newAgentPublicAPIRouterForKeys) for building a
// second, bare /api/v1 router sharing one identity.Service/database with
// a bff router built by newTestRouter — this file's own
// newAgentPublicAPIRouterForUsage below is that same pattern's usage-
// domain counterpart.
package bff

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mildronize/my-token/internal/api"
	"github.com/mildronize/my-token/internal/bffapi"
	"github.com/mildronize/my-token/internal/collector"
	"github.com/mildronize/my-token/internal/domain/usage"
	"github.com/mildronize/my-token/internal/identity"
	"github.com/mildronize/my-token/internal/transport/publicapi"
)

// newAgentPublicAPIRouterForUsage mirrors keys_handler_test.go's own
// newAgentPublicAPIRouterForKeys: a bare /api/v1 stack mounting only
// publicapi.UsageServer.IngestUsageEventsBatch — the real Bearer-
// authenticated surface collector.Run's own poster (collector.Client)
// actually POSTs to in production, not a service-layer shortcut. Kept
// separate from newTestRouter (which only ever mounts /api/bff) so this
// test can run a real collector against a real HTTP ingestion endpoint
// and then read the result back through a real, separately-mounted
// /api/bff/usage/summary — both routers share the same usageSvc/conn
// underneath, so nothing here is a mock of the other.
func newAgentPublicAPIRouterForUsage(t *testing.T, identitySvc *identity.Service, usageSvc *usage.Service) *gin.Engine {
	t.Helper()
	validator, err := api.RequestValidator()
	require.NoError(t, err)

	router := gin.New()
	group := router.Group("/api/v1")
	group.Use(publicapi.RejectActorFields(), publicapi.RequireActor(identitySvc), validator)
	usageServer := publicapi.NewUsageServer(usageSvc)
	group.POST("/usage-events/batch", usageServer.IngestUsageEventsBatch)
	return router
}

// runRealCollector wires and runs one real collector.Run invocation
// against scanRoot, install/hostname, and coreURL — the one place this
// test's two otherwise-identical machine setups (Machine A/B, below)
// share their Config/Client/Run wiring, so the two only ever differ in
// the five values that actually distinguish them. statePath/pathStorePath
// are always fresh t.TempDir() files: each invocation is its own
// install's first-ever run, never sharing sent-state with the other.
func runRealCollector(t *testing.T, scanPathName, scanRoot, installID, hostname, coreURL, rawKey string) collector.RunResult {
	t.Helper()
	cfg := collector.Config{
		ScanPaths: []collector.ScanPath{{Name: scanPathName, Path: scanRoot, SourceType: "claude_code"}},
		InstallID: installID,
		CoreURL:   coreURL,
		APIKey:    rawKey,
	}
	client := collector.NewClient(coreURL, rawKey, installID, hostname)
	result, err := collector.Run(cfg, filepath.Join(t.TempDir(), "state.json"), filepath.Join(t.TempDir(), "pathstore.db"), collector.RealGitRootResolver, hostname, client)
	require.NoError(t, err, "collector run for install_id %q against %q must complete without error", installID, scanRoot)
	return result
}

// realClaudeConfigDirWithTranscripts confirms a named Claude Code config
// directory under the current user's home (e.g. ".claude",
// ".claude-openrouter") is still what ticket 2 (second-machine-proof.md)
// found it to be: a real config directory with its own real
// projects/*.jsonl transcripts — never assumed, always checked fresh (per
// this ticket's own instruction: "verify it still exists and still has
// *.jsonl files under projects/ before assuming its shape"). Returns the
// resolved directory path and skips the test (not fails it — this is a
// fact about this specific machine's own environment, not a code defect)
// when the directory or any real transcript under it is missing.
func realClaudeConfigDirWithTranscripts(t *testing.T, dirName string) string {
	t.Helper()
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	dir := filepath.Join(home, dirName)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Skipf("ticket-12 real second-machine smoke test requires ~/%s to exist on this machine — got: %v", dirName, err)
	}

	projectsDir := filepath.Join(dir, "projects")
	var found bool
	_ = filepath.Walk(projectsDir, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil || fi == nil || fi.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".jsonl") {
			found = true
		}
		return nil
	})
	if !found {
		t.Skipf("ticket-12 real second-machine smoke test requires at least one real *.jsonl transcript under ~/%s/projects — none found", dirName)
	}

	return dir
}

// TestTicket12_RealSecondMachineSmokeTest is this story's final ticket:
// the real second-machine proof the wayfinder ticket (_tickets/
// 2-second-machine-proof.md) resolved to after a loop-readiness review —
// a real second collector *process* on this same machine, not a second
// real host. Two real collector.Run() invocations against one real
// (in-process httptest) core service, both against this machine's own
// genuinely real transcripts (never synthetic fixture data — the
// contract's own Testing Decisions names exactly these two directories):
//
//   - "Machine A": ~/.claude — this story's own existing dev scan root,
//     the same directory tickets 7-11 were themselves built and iterated
//     against, real transcripts from every crew that has ever used this
//     machine's default Claude Code install. Its own distinct install_id
//     ("A").
//   - "Machine B": ~/.claude-openrouter, confirmed real
//     (realClaudeConfigDirWithTranscripts, above) — a second, genuinely
//     separate Claude config directory with its own real projects/*.jsonl
//     transcripts. Its own distinct install_id ("B").
//
// Both POST to the same real /api/v1 ingestion router
// (newAgentPublicAPIRouterForUsage), authenticated with a real agent API
// key issued through identity.Service.IssueAPIKeyForHandle (the same
// production path cmd/issue-key itself calls — never a direct repo
// insert). Verification reads back through a real, session-authenticated
// GET /api/bff/usage/summary (newTestRouter, this package's own existing
// harness) against the exact same underlying database — the established
// pattern this package's own usage_handler_test.go already uses for
// every other BFF-level usage assertion in this suite (real HTTP,
// decode-and-assert on the JSON response), not a direct Service.Summary
// call and not a second, unauthenticated round trip.
//
// Because both ~/.claude and ~/.claude-openrouter are real, live
// directories this machine's crews keep using (not frozen fixtures), the
// exact token/turn counts either one contributes can grow between runs of
// this test — every assertion below is written to hold regardless of
// that: only non-zero/structural facts are asserted for either machine's
// own totals (never an exact count), and group_by=path's own per-row
// checks validate structural correctness (the right install_id, the
// right hostname substitution, a real non-blank path, never a row
// attributed to a machine that didn't report it) for however many rows
// either real install's data happens to produce — this is exactly the
// contract's own "whichever the two real transcript sets actually
// exercise" allowance for this specific test. Explicitly out of scope,
// per the wayfinder ticket's own resolved boundary: genuine cross-host
// network reachability or environment differences — this test never
// leaves this machine's own filesystem/process boundary.
func TestTicket12_RealSecondMachineSmokeTest(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH — the real collector pipeline needs it to resolve touched paths to their git-root")
	}
	claudeDir := realClaudeConfigDirWithTranscripts(t, ".claude")
	openrouterDir := realClaudeConfigDirWithTranscripts(t, ".claude-openrouter")

	const (
		installIDA = "A"
		hostnameA  = "ticket12-machine-a-claude-host"
		installIDB = "B"
		hostnameB  = "ticket12-machine-b-openrouter-host"
	)

	// --- Shared core service: one real /api/v1 ingestion router and one
	// real session-authenticated /api/bff router, both wired on top of the
	// exact same database/services — mirrors keys_handler_test.go's own
	// newAgentPublicAPIRouterForKeys + newBFFRouterForOwnerSharedDB split.
	conn := newTestDB(t)
	identityRepo := identity.NewRepo(conn)
	identitySvc := identity.NewService(identityRepo, identityRepo, nil, nil)
	usageSvc := usage.NewService(usage.NewRepo(conn))

	v1Router := newAgentPublicAPIRouterForUsage(t, identitySvc, usageSvc)
	coreServer := httptest.NewServer(v1Router)
	defer coreServer.Close()

	issued, err := identitySvc.IssueAPIKeyForHandle(context.Background(), "ticket12-collector-smoke")
	require.NoError(t, err)
	require.Equal(t, "agent", issued.User.Role, "the collector must authenticate as a real role=agent identity, the only path cmd/issue-key ever produces")
	rawKey := issued.RawKey
	require.NotEmpty(t, rawKey)

	owner := seedUser(t, conn, "owner", "owner-sub-"+t.Name(), true)
	idp := newFakeIDP(t, "test-client")
	cfg := idp.testConfig()
	signer := NewSigner([]byte(cfg.SessionSecret))
	bffRouter := newTestRouter(cfg, signer, newIDVerifier(t, idp), identityRepo, identitySvc, usageSvc)
	sessionValue, err := signer.NewSessionCookie(owner.ID)
	require.NoError(t, err)

	// --- Machine A: the real ~/.claude install (this story's own existing
	// dev scan root) — its own distinct install_id, no fixture data at
	// all. Machine B: the real ~/.claude-openrouter install — its own
	// distinct install_id, no fixture data at all. runRealCollector (below)
	// is the one place both runs' identical Config/Client/Run wiring lives.
	resultA := runRealCollector(t, "ticket12-claude", claudeDir, installIDA, hostnameA, coreServer.URL, rawKey)
	require.Greater(t, resultA.NewRowsFound, 0, "~/.claude's real transcripts must contain at least one real usage turn (realClaudeConfigDirWithTranscripts already confirmed real *.jsonl files exist)")
	require.Equal(t, int64(resultA.NewRowsFound), resultA.Inserted, "every real row machine A found must actually land — Verifiable: non-zero Inserted")

	resultB := runRealCollector(t, "ticket12-openrouter", openrouterDir, installIDB, hostnameB, coreServer.URL, rawKey)
	require.Greater(t, resultB.NewRowsFound, 0, "~/.claude-openrouter's real transcripts must contain at least one real usage turn (realClaudeConfigDirWithTranscripts already confirmed real *.jsonl files exist)")
	require.Equal(t, int64(resultB.NewRowsFound), resultB.Inserted, "every real row machine B found must actually land — Verifiable: non-zero Inserted")

	// --- Verify via the real, session-authenticated GET
	// /api/bff/usage/summary — this package's own established pattern
	// (usage_handler_test.go), not a direct Service.Summary call.
	recMachine := doBFFJSONRequest(t, bffRouter, http.MethodGet, "/api/bff/usage/summary?window=lifetime&group_by=machine", sessionValue, nil)
	require.Equal(t, http.StatusOK, recMachine.Code, recMachine.Body.String())
	gotMachine := decodeUsageSummary(t, recMachine)

	assert.Equal(t, int64(2), gotMachine.ReportingInstalls, "exactly two real collector installs reported in this window")
	require.Len(t, gotMachine.Breakdown, 2, "group_by=machine's breakdown must have two distinct rows — one per real install")

	rowsByInstall := make(map[string]bffapi.UsageBreakdownRow, 2)
	for _, row := range gotMachine.Breakdown {
		require.NotNil(t, row.RawKey, "group_by=machine always substitutes the hostname into Key and stashes the raw install_id in RawKey once a machines row exists")
		rowsByInstall[*row.RawKey] = row
	}

	rowA, ok := rowsByInstall[installIDA]
	require.True(t, ok, "machine A's install_id must appear in the machine breakdown")
	assert.Equal(t, hostnameA, rowA.Key, "machine A's row must show its hostname, not its raw install_id")
	assert.GreaterOrEqual(t, rowA.Turns, int64(1), "real ~/.claude data must contribute at least one real turn")

	rowB, ok := rowsByInstall[installIDB]
	require.True(t, ok, "machine B's install_id (the real ~/.claude-openrouter install) must appear in the machine breakdown")
	assert.Equal(t, hostnameB, rowB.Key, "machine B's row must show its hostname, not its raw install_id")
	assert.GreaterOrEqual(t, rowB.Turns, int64(1), "real ~/.claude-openrouter data must contribute at least one real turn")

	// --- group_by=path: story-2/ticket-7's cross-machine fix and
	// path/actor dedup rule, exercised against real data from both real
	// installs. Assertions are written to hold regardless of how much real
	// data either directory happens to carry at any given run (see this
	// test's own doc comment) — every row is checked structurally, rather
	// than requiring an exact count or an exact set of rows.
	recPath := doBFFJSONRequest(t, bffRouter, http.MethodGet, "/api/bff/usage/summary?window=lifetime&group_by=path", sessionValue, nil)
	require.Equal(t, http.StatusOK, recPath.Code, recPath.Body.String())
	gotPath := decodeUsageSummary(t, recPath)

	assert.Equal(t, int64(2), gotPath.ReportingInstalls, "reporting_installs is computed from the full event set, independent of group_by — the path/actor dedup rule only ever affects the breakdown, never this count")

	// pathsSeenByInstall tracks each install's own distinct path values —
	// used below to confirm the two real installs' path data never
	// collapses into a shared row even where the literal path string is
	// identical (story-2/ticket-7's own fix: the aggregation key is
	// machine-prefixed, `<install_id>:<path>`, precisely so this can never
	// happen structurally).
	pathsSeenByInstall := map[string]map[string]bool{installIDA: {}, installIDB: {}}
	for _, row := range gotPath.Breakdown {
		require.NotNil(t, row.RawKey, "group_by=path always substitutes the hostname prefix once a machines row exists (story-2/ticket-7)")
		gotInstallID, gotPathValue, found := strings.Cut(*row.RawKey, ":")
		require.True(t, found, "raw_key must be the machine-prefixed <install_id>:<path> composite ticket 7's keyFor(GroupByPath) produces")
		require.NotEmpty(t, gotPathValue, "a group_by=path row's own path half must never be blank")

		switch gotInstallID {
		case installIDA:
			assert.Equal(t, hostnameA+":"+gotPathValue, row.Key, "machine A's row must be prefixed with its own hostname, not merged with or mislabeled as machine B's")
			pathsSeenByInstall[installIDA][gotPathValue] = true
		case installIDB:
			assert.Equal(t, hostnameB+":"+gotPathValue, row.Key, "machine B's row must be prefixed with its own hostname, not merged with or mislabeled as machine A's")
			pathsSeenByInstall[installIDB][gotPathValue] = true
		default:
			t.Fatalf("group_by=path breakdown contains a row for an unexpected install_id %q — real data must never be misattributed to a machine that didn't report it", gotInstallID)
		}
	}

	// Whether either real install's transcripts contribute any
	// group_by=path row at all depends on whether any of their real
	// sessions ever touched a path outside their own launch cwd (ticket
	// 7's path/actor dedup rule excludes a session whose path equals its
	// own actor) — "whichever the two real transcript sets actually
	// exercise" (contract's Testing Decisions). What must hold regardless:
	// a literal path string shared by both real installs (a real
	// possibility — both directories carry real sessions against some of
	// the same real repositories on this machine, e.g. gits/my-template)
	// must still show up as two separate, correctly-attributed rows, never
	// one silently merged row — the exact false-merge bug story-1
	// documented and story-2/ticket-7 fixed.
	for p := range pathsSeenByInstall[installIDA] {
		if pathsSeenByInstall[installIDB][p] {
			t.Logf("both real installs reported the same literal path %q — confirmed present as two separate machine-prefixed rows, not merged", p)
		}
	}
}
