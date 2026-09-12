package collector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Event is one usage row on the wire, matching openapi.yaml's
// UsageEventInput field-for-field (snake_case, per the contract's own
// wire shape — ticket 11). No Cost/Source fields at all: the server
// computes and stores both itself and rejects a client that tries to
// send either (additionalProperties: false).
type Event struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Actor     string `json:"actor"`
	Path      string `json:"path"`
	Machine   string `json:"machine"`
	Model     string `json:"model"`
	// ScanRoot is the resolved ScanPath.Path this event's transcript file
	// was found under (story-2/ticket-8, contract's Data model:
	// usage_events.scan_root) — distinct from Path, which is git-rooted
	// from touched files, not the transcript's own location.
	ScanRoot                 string    `json:"scan_root"`
	InputTokens              int64     `json:"input_tokens"`
	OutputTokens             int64     `json:"output_tokens"`
	CacheReadInputTokens     int64     `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64     `json:"cache_creation_input_tokens"`
	Timestamp                time.Time `json:"timestamp"`
}

// ScanRootReport is one entry of the batch's top-level scan_roots array
// (story-2/ticket-8, contract's API surface: "scan_roots: [{path, name,
// source_type}] — NEW — batch-level, upserted like hostname already
// is"). Path is the same resolved value as the events' own ScanRoot
// field; Name/SourceType are the owning ScanPath config entry's own
// declared values.
type ScanRootReport struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	SourceType string `json:"source_type"`
}

// batchRequest is POST /api/v1/usage-events/batch's request body
// (contract's API surface: "{ install_id, hostname, scan_roots: [...],
// events: [...] }" — scan_roots added story-2/ticket-8).
type batchRequest struct {
	InstallID string           `json:"install_id"`
	Hostname  string           `json:"hostname"`
	ScanRoots []ScanRootReport `json:"scan_roots"`
	Events    []Event          `json:"events"`
}

// BatchResult is the endpoint's response shape (ticket 11's report:
// "{received, inserted}").
type BatchResult struct {
	Received int64 `json:"received"`
	Inserted int64 `json:"inserted"`
}

// Client posts batches of usage Events to ticket 11's core service.
type Client struct {
	baseURL    string
	apiKey     string
	installID  string
	hostname   string
	httpClient *http.Client
}

// NewClient builds a Client against coreURL (its base — "/api/v1/..." is
// appended per-call, so coreURL should be just the scheme+host[:port],
// e.g. "http://localhost:8080"), authenticating with apiKey as a Bearer
// credential (ticket 11: reuses my-template's existing API-key
// middleware — nothing collector-specific about the auth mechanism
// itself). installID/hostname identify this reporting collector
// (contract API surface's top-level batch fields), not any individual
// event.
func NewClient(coreURL, apiKey, installID, hostname string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(coreURL, "/"),
		apiKey:     apiKey,
		installID:  installID,
		hostname:   hostname,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// PostBatch POSTs events (and the scan_roots array they were attributed
// to, story-2/ticket-8) to /api/v1/usage-events/batch. A nil/empty
// events slice is a no-op (returns a zero BatchResult, makes no HTTP
// request at all) — collector.go already filters to newly-found rows
// before calling this, so an empty batch means "nothing new to report",
// not an error condition.
func (c *Client) PostBatch(scanRoots []ScanRootReport, events []Event) (BatchResult, error) {
	if len(events) == 0 {
		return BatchResult{}, nil
	}

	body, err := json.Marshal(batchRequest{
		InstallID: c.installID,
		Hostname:  c.hostname,
		ScanRoots: scanRoots,
		Events:    events,
	})
	if err != nil {
		return BatchResult{}, fmt.Errorf("encoding usage-events batch: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/v1/usage-events/batch", bytes.NewReader(body))
	if err != nil {
		return BatchResult{}, fmt.Errorf("building usage-events batch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return BatchResult{}, fmt.Errorf("POST %s/api/v1/usage-events/batch: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return BatchResult{}, fmt.Errorf("POST %s/api/v1/usage-events/batch: unexpected status %d", c.baseURL, resp.StatusCode)
	}

	var result BatchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return BatchResult{}, fmt.Errorf("decoding usage-events batch response: %w", err)
	}
	return result, nil
}
