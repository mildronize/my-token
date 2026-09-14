package collector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadOrInitConfig_GeneratesAndPersistsInstallIDOnFirstRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"scan_paths": [{"name": "main", "path": "/home/thw-home/.typ-crews/freya"}],
		"core_url": "http://localhost:8080",
		"api_key": "tpl_test"
	}`), 0o644))

	cfg, err := LoadOrInitConfig(path)
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.InstallID, "a missing install_id must be generated on first run")
	assert.Equal(t, []ScanPath{{Name: "main", Path: "/home/thw-home/.typ-crews/freya", SourceType: "claude_code"}}, cfg.ScanPaths,
		"story-2/ticket-8: source_type defaults to claude_code when the config JSON omits it")
	assert.Equal(t, "http://localhost:8080", cfg.CoreURL)
	assert.Equal(t, "tpl_test", cfg.APIKey)

	// Persisted back to disk — ticket 5's requirement, not just held in
	// memory for this one process run.
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var onDisk Config
	require.NoError(t, json.Unmarshal(raw, &onDisk))
	assert.Equal(t, cfg.InstallID, onDisk.InstallID)
}

func TestLoadOrInitConfig_ReusesExistingInstallIDOnSubsequentRuns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"scan_paths": [{"name": "main", "path": "/x"}],
		"install_id": "fixed-install-id-123",
		"core_url": "http://localhost:8080",
		"api_key": "tpl_test"
	}`), 0o644))

	cfg, err := LoadOrInitConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "fixed-install-id-123", cfg.InstallID, "an existing install_id must never be regenerated")

	// Loading again must still see the same value — nothing overwrote it.
	cfg2, err := LoadOrInitConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "fixed-install-id-123", cfg2.InstallID)
}

func TestLoadOrInitConfig_MissingFileIsAnError(t *testing.T) {
	_, err := LoadOrInitConfig(filepath.Join(t.TempDir(), "does-not-exist.json"))
	assert.Error(t, err, "a collector with no config file at all has nothing to run against — this should not silently invent one")
}

func TestLoadOrInitConfig_MalformedJSONIsAnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`not json`), 0o644))

	_, err := LoadOrInitConfig(path)
	assert.Error(t, err)
}

// TestLoadOrInitConfig_OldBareStringScanPathsShapeErrorsClearly is
// story-2/ticket-8's own required regression: a config file still using
// story-1's old bare-string `scan_paths: string[]` shape must fail to
// load with a clear error, not silently misparse (goal.md's Out of
// Scope: existing dev config files using the old shape are not
// migrated). json.Unmarshal itself already provides this for free — a
// JSON string can't unmarshal into the new ScanPath struct — so this
// test is this behavior's own regression guard, not new production code.
func TestLoadOrInitConfig_OldBareStringScanPathsShapeErrorsClearly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"scan_paths": ["~/.claude", "~/.claude-local"],
		"core_url": "http://localhost:8080",
		"api_key": "tpl_test"
	}`), 0o644))

	_, err := LoadOrInitConfig(path)
	require.Error(t, err, "the old bare-string scan_paths shape must not silently misparse")
	assert.Contains(t, err.Error(), "parsing config", "the error must clearly point at config parsing, not some unrelated failure")
}

// TestLoadOrInitConfig_ExplicitSourceTypeIsNotOverridden proves the
// default (DefaultSourceType) only fills in a *missing* source_type — an
// entry that explicitly declares one (even a future non-"claude_code"
// value, per the goal's own forward-compat intent) is left exactly as
// configured.
func TestLoadOrInitConfig_ExplicitSourceTypeIsNotOverridden(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"scan_paths": [{"name": "main", "path": "~/.claude", "source_type": "future_source"}],
		"install_id": "fixed-install-id-123",
		"core_url": "http://localhost:8080",
		"api_key": "tpl_test"
	}`), 0o644))

	cfg, err := LoadOrInitConfig(path)
	require.NoError(t, err)
	require.Len(t, cfg.ScanPaths, 1)
	assert.Equal(t, "future_source", cfg.ScanPaths[0].SourceType)
}

func TestResolveHostname_ConfiguredValueWins_OSHostnameNeverCalled(t *testing.T) {
	called := false
	osHostname := func() (string, error) {
		called = true
		return "real-os-hostname", nil
	}

	got, err := ResolveHostname(Config{Hostname: "thw-home-openrouter"}, osHostname)
	require.NoError(t, err)
	assert.Equal(t, "thw-home-openrouter", got)
	assert.False(t, called, "a configured Hostname must short-circuit before osHostname is ever invoked")
}

func TestResolveHostname_EmptyConfig_FallsBackToOSHostname(t *testing.T) {
	got, err := ResolveHostname(Config{}, func() (string, error) { return "real-os-hostname", nil })
	require.NoError(t, err)
	assert.Equal(t, "real-os-hostname", got)
}

func TestResolveHostname_EmptyConfig_OSHostnameErrors_PropagatesError(t *testing.T) {
	wantErr := assert.AnError
	_, err := ResolveHostname(Config{}, func() (string, error) { return "", wantErr })
	assert.ErrorIs(t, err, wantErr)
}
