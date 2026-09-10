package collector

import (
	"encoding/json"
	"regexp"
	"strings"
)

// toolUseBlock is one message.content array element shaped like a
// tool_use block — the only content-block shape ExtractTouchedPaths cares
// about. Other block types (text, thinking, tool_result, …) are silently
// skipped via the Type check in ExtractTouchedPaths; this struct doesn't
// even attempt to model their fields.
type toolUseBlock struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// cdPrefixPattern matches a leading `cd <path> &&` or `cd <path>;`
// pattern inside a Bash command string (ticket 10 §6 / ticket 13's own
// "at minimum" scope) — the dominant, reliably-parseable case found on
// luna's real my-template session (114 of her own `cd ... && git commit`
// invocations use exactly this shape). Deliberately does not attempt to
// parse every other absolute-path argument a command might contain (e.g.
// a `git -C <path>` flag elsewhere in the string) — a fuller pass is
// future work, not this ticket's scope.
var cdPrefixPattern = regexp.MustCompile(`^\s*cd\s+(\S+)\s*(?:&&|;)`)

// ExtractTouchedPaths scans lines — one session's raw JSONL transcript
// lines, from its main transcript file and/or a
// <session-uuid>/subagents/agent-*.jsonl file beneath it (ticket 9's scan
// scope) — for every real filesystem path a tool_use record touched:
// Read/Write/Edit's `file_path` input, Glob's `path` input, and the
// leading `cd <path>` target of a Bash command's `command` input.
//
// Returns every match verbatim, in the order found — no dedup, no
// scratch-filtering, no git-rooting; those are TallyGitRoots'/
// ResolveSessionPathFull's job (ticket 13's aggregation step), kept
// separate so this extraction step stays independently unit-testable
// against constructed content-block fixtures, per the contract's Testing
// Decisions.
//
// A line that isn't valid JSON, has no message.content, or has
// message.content in a shape other than a tool_use-block array (e.g. a
// plain string, which real user-turn records use) yields nothing for
// that line — never an error, matching ExtractUsageRows' own
// skip-don't-fail convention for malformed/irrelevant lines.
func ExtractTouchedPaths(lines []string) []string {
	var out []string
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var rec struct {
			Message *struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil || rec.Message == nil || len(rec.Message.Content) == 0 {
			continue
		}

		var blocks []toolUseBlock
		if err := json.Unmarshal(rec.Message.Content, &blocks); err != nil {
			continue // content wasn't an array of blocks at all — nothing to extract
		}
		for _, b := range blocks {
			if b.Type != "tool_use" {
				continue
			}
			if p, ok := touchedPathFromBlock(b); ok {
				out = append(out, p)
			}
		}
	}
	return out
}

// touchedPathFromBlock extracts the one real filesystem path (if any) a
// single tool_use block names, per tool: Read/Write/Edit's file_path,
// Glob's path, or Bash's leading `cd` target.
func touchedPathFromBlock(b toolUseBlock) (string, bool) {
	switch b.Name {
	case "Read", "Write", "Edit":
		var in struct {
			FilePath string `json:"file_path"`
		}
		if json.Unmarshal(b.Input, &in) == nil && in.FilePath != "" {
			return in.FilePath, true
		}
	case "Glob":
		var in struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(b.Input, &in) == nil && in.Path != "" {
			return in.Path, true
		}
	case "Bash":
		var in struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(b.Input, &in) == nil && in.Command != "" {
			if p, ok := extractCdPath(in.Command); ok {
				return p, true
			}
		}
	}
	return "", false
}

// extractCdPath pulls the leading `cd <path> &&`/`cd <path>;` target out
// of a Bash command string, expanding a leading "~" via scan.go's own
// expandHome — luna's real commands use both `cd /abs/path && ...` and
// `cd ~/gits/my-template && ...` (ticket 10 §6). Returns ok=false when
// the command doesn't start with `cd `, or its target is a bare relative
// path (`cd subdir`) — a relative cd carries no information about where
// the session's shell actually was without knowing its prior directory,
// which this line-local extraction has no access to.
func extractCdPath(command string) (string, bool) {
	m := cdPrefixPattern.FindStringSubmatch(command)
	if m == nil {
		return "", false
	}
	target := strings.Trim(m[1], `"'`)
	if !strings.HasPrefix(target, "/") && !strings.HasPrefix(target, "~") {
		return "", false
	}
	return expandHome(target), true
}

// firstSessionID returns the sessionId shared by every record in a
// transcript file — ticket 9 confirmed a subagent file's every record
// carries its parent session's id, never its own, so any line's
// sessionId (not necessarily the first line's — a summary/no-op line can
// carry no sessionId at all) identifies the whole file. ok is false only
// if no line in the file carries a non-empty sessionId at all (e.g.
// ticket 9's "bare hi" noop-session edge case).
func firstSessionID(lines []string) (string, bool) {
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var rec struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err == nil && rec.SessionID != "" {
			return rec.SessionID, true
		}
	}
	return "", false
}

// isScratchPath reports whether p is scratch-space noise that must be
// dropped outright, before any git-root attempt (ticket 13 step 2 / ticket
// 10 §9.1) — a deny-list check, not a "no git root" case. Even a /tmp
// path that happens to sit inside a real .git working tree still doesn't
// count; scratch space is never "the project."
func isScratchPath(p string) bool {
	return p == "/tmp" || strings.HasPrefix(p, "/tmp/")
}
