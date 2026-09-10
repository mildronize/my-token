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
		"scan_paths": ["/home/thw-home/.typ-crews/freya"],
		"core_url": "http://localhost:8080",
		"api_key": "tpl_test"
	}`), 0o644))

	cfg, err := LoadOrInitConfig(path)
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.InstallID, "a missing install_id must be generated on first run")
	assert.Equal(t, []string{"/home/thw-home/.typ-crews/freya"}, cfg.ScanPaths)
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
		"scan_paths": ["/x"],
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
