package collector

import (
	"encoding/json"
	"fmt"
	"os"
)

// SentState tracks which message.ids this install has already
// successfully POSTed to the core service — ticket 12's own bookkeeping
// note: "tracking what's already been sent ... so re-running the
// collector doesn't resend duplicates — though the server is idempotent
// anyway, so this is an optimization not a correctness requirement." A
// small local JSON file (a set of message.ids), not SQLite: this state
// is nothing more than a set membership check, and a real deployment's
// collector shouldn't need a database dependency of its own just to
// avoid re-uploading rows the server would silently no-op anyway.
type SentState struct {
	sentIDs map[string]bool
}

// sentStateFile is SentState's on-disk shape.
type sentStateFile struct {
	SentMessageIDs []string `json:"sent_message_ids"`
}

// LoadSentState reads path's set of already-sent message.ids. A missing
// file is treated as "nothing sent yet" (a fresh install, or the state
// file's first run), not an error — only a file that exists but is
// malformed is an error.
func LoadSentState(path string) (*SentState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SentState{sentIDs: make(map[string]bool)}, nil
		}
		return nil, fmt.Errorf("reading sent-state %s: %w", path, err)
	}

	var f sentStateFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parsing sent-state %s: %w", path, err)
	}

	ids := make(map[string]bool, len(f.SentMessageIDs))
	for _, id := range f.SentMessageIDs {
		ids[id] = true
	}
	return &SentState{sentIDs: ids}, nil
}

// Sent reports whether messageID has already been recorded as sent.
func (s *SentState) Sent(messageID string) bool {
	return s.sentIDs[messageID]
}

// MarkSent records messageID as sent — callers call this only after a
// successful POST (collector.go), never speculatively before one.
func (s *SentState) MarkSent(messageID string) {
	s.sentIDs[messageID] = true
}

// FilterUnsent returns the subset of rows whose MessageID has not
// already been marked sent — what collector.go actually batches and
// POSTs on a given run.
func (s *SentState) FilterUnsent(rows []UsageRow) []UsageRow {
	out := make([]UsageRow, 0, len(rows))
	for _, r := range rows {
		if !s.Sent(r.MessageID) {
			out = append(out, r)
		}
	}
	return out
}

// Save persists the current set of sent message.ids to path.
func (s *SentState) Save(path string) error {
	ids := make([]string, 0, len(s.sentIDs))
	for id := range s.sentIDs {
		ids = append(ids, id)
	}
	data, err := json.MarshalIndent(sentStateFile{SentMessageIDs: ids}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}
