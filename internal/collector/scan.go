package collector

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultSourceType is the source_type value a ScanPath entry gets when
// the collector config's JSON omits it (contract's "Collector config"
// section: "optional in the config, defaults to 'claude_code' if
// omitted — every scan root in practice is 'claude_code' for this
// story, but the field is real and settable, not hardcoded").
const DefaultSourceType = "claude_code"

// ScannedFile pairs one discovered transcript file with the ScanPath
// config entry that produced it (story-2/ticket-8) — the loop below
// already walks each configured root separately, so this is wiring, not
// new discovery logic. ScanRoot/Name/SourceType are what collector.go's
// Run carries through to each of this file's events' own ScanRoot field
// and to the batch payload's scan_roots array (client.go).
type ScannedFile struct {
	// Path is the discovered *.jsonl file's own path.
	Path string
	// ScanRoot is the owning ScanPath entry's *resolved* Path —
	// expandHome already applied, matching what ends up in
	// usage_events.scan_root (contract: "the entry's resolved Path").
	ScanRoot string
	// Name is the owning ScanPath entry's Name.
	Name string
	// SourceType is the owning ScanPath entry's SourceType, defaulted to
	// DefaultSourceType if empty (defensive — LoadOrInitConfig already
	// applies this same default when loading from disk, but this keeps
	// FindTranscriptFiles correct for a caller that builds a ScanPath by
	// hand without going through LoadOrInitConfig, e.g. tests).
	SourceType string
}

// FindTranscriptFiles recursively finds every *.jsonl transcript file
// under each of scanPaths — including nested
// <session-uuid>/subagents/agent-*.jsonl files (ticket 9/10: a
// non-recursive scan silently drops subagent usage). filepath.Walk
// already recurses into every subdirectory by construction, so no
// special-casing of the "subagents" directory name is needed — the only
// filter applied is the *.jsonl suffix, which also correctly excludes
// each subagent's own *.meta.json sidecar file.
//
// A scan path that doesn't exist (e.g. a configured ~/.claude-local that
// was never actually installed on this machine) is skipped rather than
// treated as fatal — the goal's own multi-path design means a missing
// one of several configured roots shouldn't stop the others from being
// scanned.
//
// Each scanPaths entry's Path is passed through expandHome first: the
// goal and ticket 12 both give scan_paths examples with a literal
// leading `~` (["~/.claude", "~/.claude-local"]) — neither os.Stat nor
// filepath.Walk expand that themselves, so without this a config written
// exactly as documented would silently scan nothing.
func FindTranscriptFiles(scanPaths []ScanPath) ([]ScannedFile, error) {
	var found []ScannedFile

	for _, sp := range scanPaths {
		root := expandHome(sp.Path)
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			continue
		}

		sourceType := sp.SourceType
		if sourceType == "" {
			sourceType = DefaultSourceType
		}

		err = filepath.Walk(root, func(path string, fi os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if fi.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".jsonl") {
				found = append(found, ScannedFile{Path: path, ScanRoot: root, Name: sp.Name, SourceType: sourceType})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return found, nil
}

// expandHome expands a leading "~" (exactly "~", or "~/...") to the
// current user's home directory, via os.UserHomeDir() — the same
// resolution every shell does, needed because scan_paths is plain JSON
// config data, never passed through a shell that would expand "~"
// itself. A path with no leading "~", or one shaped like "~otheruser"
// (a different user's home — not supported, and rare enough in a
// single-user collector config not to be worth the extra
// os/user.Lookup dependency), is returned unchanged.
func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path // can't resolve home — leave it as-is; the subsequent os.Stat will just report it missing
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/"))
}
