// Command collector is story-1/ticket-12's standalone collector: reads
// its own JSON config (scan_paths, install_id, core_url, api_key —
// contract's Data model), recursively scans every configured path for
// Claude Code *.jsonl transcripts (subagent transcripts included), dedupes
// per-turn usage by message.id (ticket 9), attributes actor/path/machine,
// and POSTs newly-found rows to ticket 11's core service
// (POST /api/v1/usage-events/batch). See internal/collector's own doc.go
// for the full pipeline and why the business logic lives there rather
// than in this file.
//
// Usage:
//
//	go run ./cmd/collector -config /path/to/collector-config.json
//
// The config file's install_id is generated (a fresh UUID) and persisted
// back to that same file on its very first run if absent (ticket 5) —
// every later run against the same config file reuses it. A sibling
// state file (<config>.state.json) tracks which message.ids this install
// has already sent, purely as an optimization (the server is idempotent
// on id regardless — ticket 11) so a re-run doesn't re-upload rows the
// server would just no-op anyway. A second sibling file
// (<config>.pathstore.db, a local SQLite database) is ticket 13's own
// path-attribution cache — every directory's known git-root, and each
// session's running touched-path vote tally — so a re-run only parses
// new transcript lines and never re-resolves a known directory's
// git-root via a subprocess call twice.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mildronize/my-token/internal/collector"
)

func main() {
	configPath := flag.String("config", "", "path to the collector's JSON config file (scan_paths, install_id, core_url, api_key)")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "usage: collector -config <path-to-config.json>")
		os.Exit(2)
	}

	if err := run(*configPath); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// statePathFor derives this run's sent-state file path from the config
// file's own path — a sibling file, not a field in Config itself (the
// contract's own Data model names exactly scan_paths/install_id/
// core_url/api_key for the config; the sent-state file is this ticket's
// own local bookkeeping optimization, not part of that contract).
func statePathFor(configPath string) string {
	return configPath + ".state.json"
}

// pathStorePathFor derives ticket 13's local SQLite path-attribution
// cache (git_root_cache + session_path_votes + session_scan_progress,
// internal/collector/pathstore.go) from the config file's own path — a
// sibling file, the same pattern statePathFor already uses.
func pathStorePathFor(configPath string) string {
	return configPath + ".pathstore.db"
}

func run(configPath string) error {
	cfg, err := collector.LoadOrInitConfig(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}

	client := collector.NewClient(cfg.CoreURL, cfg.APIKey, cfg.InstallID, hostname)

	result, err := collector.Run(cfg, statePathFor(configPath), pathStorePathFor(configPath), collector.RealGitRootResolver, hostname, client)
	if err != nil {
		return fmt.Errorf("running collector: %w", err)
	}

	fmt.Printf("Scanned %d transcript file(s), found %d deduped usage row(s) total (%d already sent).\n",
		result.FilesScanned, result.TotalRowsFound, result.TotalRowsFound-result.NewRowsFound)
	if result.NewRowsFound == 0 {
		fmt.Println("Nothing new to report.")
		return nil
	}
	fmt.Printf("Reported %d new row(s): server received=%d inserted=%d (local cost estimate: $%.4f, not authoritative).\n",
		result.NewRowsFound, result.Received, result.Inserted, result.EstimatedCostUS)

	return nil
}
