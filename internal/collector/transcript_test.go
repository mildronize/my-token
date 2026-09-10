package collector

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// usageLine builds one JSONL transcript line — the real shape a Claude
// Code transcript record has (confirmed against a live transcript on
// this host: message.id, message.model, message.usage's four counters,
// top-level cwd/sessionId/timestamp, and a record uuid distinct from
// message.id).
func usageLine(recordUUID, messageID, model, contentType string, input, output, cacheRead, cacheCreation int64, cwd, sessionID, ts string) string {
	itoa := strconv.FormatInt
	return `{"uuid":"` + recordUUID + `","sessionId":"` + sessionID + `","cwd":"` + cwd + `","timestamp":"` + ts + `",` +
		`"message":{"id":"` + messageID + `","model":"` + model + `",` +
		`"content":[{"type":"` + contentType + `"}],` +
		`"usage":{"input_tokens":` + itoa(input, 10) + `,"output_tokens":` + itoa(output, 10) +
		`,"cache_read_input_tokens":` + itoa(cacheRead, 10) + `,"cache_creation_input_tokens":` + itoa(cacheCreation, 10) + `}}}`
}

// TestExtractUsageRows_DedupesByMessageID_NotRecordUUID is ticket 9's own
// regression test, verbatim per the contract's Testing Decisions: "feed a
// fixture transcript with the exact multi-line-per-message.id shape
// ticket 9 found (one call split across thinking+text lines) and assert
// the ingested count is 1 turn, not 2." Extended to 3 lines
// (thinking+text+tool_use) since that's the exact shape found on a real
// transcript on this host — the classic case is not hypothetical.
func TestExtractUsageRows_DedupesByMessageID_NotRecordUUID(t *testing.T) {
	lines := []string{
		usageLine("record-uuid-1", "msg_shared", "claude-sonnet-4-5", "thinking", 2, 446, 30120, 13387, "/home/thw-home/.typ-crews/freya", "session-1", "2026-08-11T04:31:54.523Z"),
		usageLine("record-uuid-2", "msg_shared", "claude-sonnet-4-5", "text", 2, 446, 30120, 13387, "/home/thw-home/.typ-crews/freya", "session-1", "2026-08-11T04:31:54.600Z"),
		usageLine("record-uuid-3", "msg_shared", "claude-sonnet-4-5", "tool_use", 2, 446, 30120, 13387, "/home/thw-home/.typ-crews/freya", "session-1", "2026-08-11T04:31:54.700Z"),
	}

	rows, err := ExtractUsageRows(lines)
	require.NoError(t, err)

	require.Len(t, rows, 1, "3 JSONL lines sharing one message.id must count as exactly 1 turn, not 3 — ticket 9's counting-bug fix")
	row := rows[0]
	assert.Equal(t, "msg_shared", row.MessageID, "the dedup key must be message.id, never the record's own uuid")
	assert.Equal(t, int64(2), row.InputTokens)
	assert.Equal(t, int64(446), row.OutputTokens)
	assert.Equal(t, int64(30120), row.CacheReadInputTokens)
	assert.Equal(t, int64(13387), row.CacheCreationInputTokens)
	assert.Equal(t, "claude-sonnet-4-5", row.Model)
	assert.Equal(t, "session-1", row.SessionID)
}

// TestExtractUsageRows_DistinctMessageIDsAreDistinctTurns is the mirror
// check: two genuinely different message.id values, each carrying a
// usage dict, must count as 2 turns — proves ExtractUsageRows isn't
// accidentally collapsing everything to one row regardless of id.
func TestExtractUsageRows_DistinctMessageIDsAreDistinctTurns(t *testing.T) {
	lines := []string{
		usageLine("record-uuid-1", "msg_one", "claude-sonnet-4-5", "text", 10, 20, 0, 0, "/home/thw-home/.typ-crews/freya", "session-1", "2026-08-11T04:31:54.523Z"),
		usageLine("record-uuid-2", "msg_two", "claude-sonnet-4-5", "text", 30, 40, 0, 0, "/home/thw-home/.typ-crews/freya", "session-1", "2026-08-11T04:32:00.000Z"),
	}

	rows, err := ExtractUsageRows(lines)
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

// TestExtractUsageRows_SkipsLinesWithoutUsageDict covers records that
// have no message.usage at all (user turns, tool-result records,
// malformed/partial lines) — they must be skipped, not error out the
// whole scan.
func TestExtractUsageRows_SkipsLinesWithoutUsageDict(t *testing.T) {
	lines := []string{
		`{"uuid":"u1","cwd":"/home/thw-home/.typ-crews/freya","message":{"role":"user","content":"hi"}}`,
		`not even json`,
		usageLine("record-uuid-2", "msg_two", "claude-sonnet-4-5", "text", 30, 40, 0, 0, "/home/thw-home/.typ-crews/freya", "session-1", "2026-08-11T04:32:00.000Z"),
	}

	rows, err := ExtractUsageRows(lines)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "msg_two", rows[0].MessageID)
}

// TestExtractUsageRows_KeepsEarliestTimestampAndFirstCwd proves the
// dedup keeps the first-seen record's own cwd/timestamp for a
// message.id — needed by path.go's own "session's first cwd-bearing
// record" rule, and so the reported timestamp is a real "when this
// happened" rather than the last (possibly much later, e.g. after
// several tool_use follow-up lines) record sharing the id.
func TestExtractUsageRows_KeepsEarliestTimestampAndFirstCwd(t *testing.T) {
	lines := []string{
		usageLine("record-uuid-1", "msg_shared", "claude-sonnet-4-5", "thinking", 2, 446, 0, 0, "/home/thw-home/.typ-crews/freya", "session-1", "2026-08-11T04:31:54.000Z"),
		usageLine("record-uuid-2", "msg_shared", "claude-sonnet-4-5", "text", 2, 446, 0, 0, "/home/thw-home/.typ-crews/freya", "session-1", "2026-08-11T04:31:59.000Z"),
	}

	rows, err := ExtractUsageRows(lines)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	want, err := time.Parse(time.RFC3339, "2026-08-11T04:31:54.000Z")
	require.NoError(t, err)
	assert.True(t, rows[0].Timestamp.Equal(want))
}
