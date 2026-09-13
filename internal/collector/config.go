package collector

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
)

// ScanPath is one registered transcript root (story-2/ticket-8's
// contract "Collector config" section — supersedes story-1's bare
// `scan_paths: string[]`): named at registration (Name), the directory
// to scan (Path), and a declared category (SourceType) — distinct from
// usage_events.source (a fixed per-event fact stamped by the server,
// story-1) since a scan root's own source_type is a settable,
// config-time value the operator (or a future non-Claude-Code source)
// controls (contract's Data model: scan_roots.source_type).
type ScanPath struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// SourceType defaults to DefaultSourceType (scan.go) when the config
	// JSON omits it — every scan root in practice is "claude_code" for
	// this story, but the field is real and settable, matching the
	// goal's own forward-compat intent for a future non-Claude-Code
	// source.
	SourceType string `json:"source_type,omitempty"`
}

// Config is the collector's own local config file (contract's Data
// model: "Collector's own local config (JSON, per goal): scan_paths,
// install_id (generated, persisted), core_url, api_key").
type Config struct {
	// ScanPaths are the named transcript roots to recursively scan for
	// *.jsonl transcripts (goal: "one or more transcript root paths ...
	// not hardcoded to a single default", e.g. ["~/.claude",
	// "~/.claude-local"]) — story-2/ticket-8 supersedes the old bare
	// `[]string` shape with `[]ScanPath{Name, Path, SourceType}`; a
	// config file still using the old shape fails to unmarshal here with
	// a clear Go json type error ("cannot unmarshal string into Go
	// struct field ... of type collector.ScanPath") rather than silently
	// misparsing — existing dev config files using the old shape are not
	// migrated (development data, goal.md's Out of Scope).
	ScanPaths []ScanPath `json:"scan_paths"`
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

	// story-2/ticket-8: source_type is optional in the config JSON,
	// defaulting to DefaultSourceType (scan.go) when omitted — applied
	// here, once, so every later reader of cfg.ScanPaths (scan.go,
	// collector.go) always sees a real, non-empty value rather than
	// having to re-apply the same default itself.
	for i := range cfg.ScanPaths {
		if cfg.ScanPaths[i].SourceType == "" {
			cfg.ScanPaths[i].SourceType = DefaultSourceType
		}
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
