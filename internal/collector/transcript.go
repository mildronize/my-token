package collector

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"time"
)

// rawRecord is one JSONL line's shape, trimmed to exactly the fields
// this collector reads. Real Claude Code transcripts carry a great deal
// more (tool results, thinking signatures, git branch, …) — this
// intentionally only names what ticket 12 needs, so an unrelated field
// Claude Code adds later never breaks parsing here.
type rawRecord struct {
	UUID      string      `json:"uuid"`
	SessionID string      `json:"sessionId"`
	Cwd       string      `json:"cwd"`
	Timestamp string      `json:"timestamp"`
	Message   *rawMessage `json:"message"`
}

type rawMessage struct {
	ID    string    `json:"id"`
	Model string    `json:"model"`
	Usage *rawUsage `json:"usage"`
}

// rawUsage is message.usage, trimmed to the four counters the contract's
// Data model actually stores — never summed into each other (ticket 9).
type rawUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

// UsageRow is one deduped turn — the collector's own intermediate shape,
// carrying everything ExtractUsageRows can determine from the transcript
// lines alone. Actor/Path/Machine/Cost are filled in by later pipeline
// stages (actor.go, path.go, pricing.go, collector.go), not here:
// ExtractUsageRows has no access to any session-wide state (the "first
// cwd-bearing record" path.go needs is a session-level fact, not a
// per-row one) and no config (install_id).
type UsageRow struct {
	MessageID                string
	SessionID                string
	Cwd                      string
	Model                    string
	InputTokens              int64
	OutputTokens             int64
	CacheReadInputTokens     int64
	CacheCreationInputTokens int64
	Timestamp                time.Time
}

// ExtractUsageRows parses each line as JSON and groups records where
// message.usage is a dict by message.id (never the record's own
// top-level uuid — ticket 9's counting-bug fix: one real LLM call is
// often split across several JSONL lines, one per content-block type
// such as thinking/text/tool_use, all sharing one message.id and an
// identical usage dict). The first line seen for a given message.id
// supplies that row's cwd/timestamp/model/usage — later lines sharing
// the id are pure duplicates for this purpose and are skipped, not
// merged or re-summed.
//
// A line that isn't valid JSON, or is valid JSON with no message.usage
// dict (user turns, tool-result records, etc.), is silently skipped —
// not an error. Order among the returned rows follows first-occurrence
// order of each message.id in lines.
func ExtractUsageRows(lines []string) ([]UsageRow, error) {
	seen := make(map[string]bool)
	rows := make([]UsageRow, 0)

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var rec rawRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue // malformed line — skip, don't fail the whole scan
		}
		if rec.Message == nil || rec.Message.Usage == nil || rec.Message.ID == "" {
			continue
		}
		if seen[rec.Message.ID] {
			continue
		}
		seen[rec.Message.ID] = true

		ts, _ := time.Parse(time.RFC3339, rec.Timestamp) // zero time if unparsable/absent — never fatal

		rows = append(rows, UsageRow{
			MessageID:                rec.Message.ID,
			SessionID:                rec.SessionID,
			Cwd:                      rec.Cwd,
			Model:                    rec.Message.Model,
			InputTokens:              rec.Message.Usage.InputTokens,
			OutputTokens:             rec.Message.Usage.OutputTokens,
			CacheReadInputTokens:     rec.Message.Usage.CacheReadInputTokens,
			CacheCreationInputTokens: rec.Message.Usage.CacheCreationInputTokens,
			Timestamp:                ts,
		})
	}

	return rows, nil
}

// ExtractUsageRowsFromFile reads path line by line and delegates to
// ExtractUsageRows — the file-reading counterpart used by the real scan
// pipeline (collector.go); ExtractUsageRows itself stays file-agnostic
// so its own tests (transcript_test.go) never touch disk.
func ExtractUsageRowsFromFile(path string) ([]UsageRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	// Transcript lines (especially ones carrying large tool outputs) can
	// exceed bufio.Scanner's 64KiB default token size — grow the buffer
	// generously rather than silently truncating/erroring on a long line.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 16*1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, err
	}

	return ExtractUsageRows(lines)
}
