package collector

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
)

// Config is the collector's own local config file (contract's Data
// model: "Collector's own local config (JSON, per goal): scan_paths,
// install_id (generated, persisted), core_url, api_key").
type Config struct {
	// ScanPaths are the directories to recursively scan for *.jsonl
	// transcripts (goal: "one or more transcript root paths ... not
	// hardcoded to a single default", e.g. ["~/.claude", "~/.claude-local"]).
	ScanPaths []string `json:"scan_paths"`
	// InstallID is a self-minted UUID identifying this collector
	// install — never derived from any external registry (ticket 5).
	// Generated on first run if absent, then persisted back to this same
	// file so every later run reuses it.
	InstallID string `json:"install_id,omitempty"`
	CoreURL   string `json:"core_url"`
	APIKey    string `json:"api_key"`
}

// LoadOrInitConfig reads the JSON config at path. If it has no
// install_id yet, one is generated (a fresh UUID, ticket 5) and
// persisted back to path before returning — every later call against the
// same file sees the same install_id. A missing or malformed config file
// is an error: this collector has nothing to run against without one,
// and inventing a config from nothing would silently hide a real
// misconfiguration (a typo'd path, a config that was never actually
// deployed).
func LoadOrInitConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}

	if cfg.InstallID != "" {
		return cfg, nil
	}

	cfg.InstallID = uuid.NewString()
	if err := writeConfig(path, cfg); err != nil {
		return Config{}, fmt.Errorf("persisting generated install_id to %s: %w", path, err)
	}
	return cfg, nil
}

// writeConfig serializes cfg back to path, pretty-printed so an operator
// who opens the file by hand (e.g. to add a second scan path) sees
// readable JSON, matching the shape they'd have hand-written it in.
func writeConfig(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}
