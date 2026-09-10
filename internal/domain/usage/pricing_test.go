package usage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCostForUsage_TableDriven — contract's Testing Decisions: "pure,
// table-driven unit tests — no framework dependency, ported straight
// from the reference collector's own PRICING table."
func TestCostForUsage_TableDriven(t *testing.T) {
	cases := []struct {
		name                     string
		model                    string
		inputTokens              int64
		outputTokens             int64
		cacheReadInputTokens     int64
		cacheCreationInputTokens int64
		want                     float64
	}{
		{
			name:         "sonnet input+output only",
			model:        "claude-sonnet-4-5-20250929",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			want:         3.00 + 15.00,
		},
		{
			name:                     "sonnet with cache read and cache write",
			model:                    "claude-sonnet-4-5-20250929",
			inputTokens:              100_000,
			outputTokens:             10_000,
			cacheReadInputTokens:     1_000_000,
			cacheCreationInputTokens: 1_000_000,
			want:                     100_000*3.00/1e6 + 10_000*15.00/1e6 + 1_000_000*0.30/1e6 + 1_000_000*3.75/1e6,
		},
		{
			name:         "opus 4.5 new tier",
			model:        "claude-opus-4-5-20250101",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			want:         5.00 + 25.00,
		},
		{
			name:         "opus 4.1 legacy tier — 3x the new tier",
			model:        "claude-opus-4-1-20240101",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			want:         15.00 + 75.00,
		},
		{
			name:         "haiku",
			model:        "claude-haiku-4-5-20250101",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			want:         1.00 + 5.00,
		},
		{
			name:         "haiku 3.5 legacy — cheaper than 4.5",
			model:        "claude-haiku-3-5-20240101",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			want:         0.80 + 4.00,
		},
		{
			name:         "fable",
			model:        "claude-fable-5-20260101",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			want:         10.00 + 50.00,
		},
		{
			name:         "unrecognized model defaults to sonnet",
			model:        "some-future-unknown-model",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			want:         3.00 + 15.00,
		},
		{
			name:         "empty model defaults to sonnet",
			model:        "",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			want:         3.00 + 15.00,
		},
		{
			name:  "zero tokens costs zero regardless of model",
			model: "claude-opus-4-5-20250101",
			want:  0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CostForUsage(tc.model, tc.inputTokens, tc.outputTokens, tc.cacheReadInputTokens, tc.cacheCreationInputTokens)
			assert.InDelta(t, tc.want, got, 1e-9)
		})
	}
}

func TestModelFamily_TableDriven(t *testing.T) {
	cases := []struct {
		model string
		want  string
	}{
		{"claude-sonnet-4-5-20250929", "sonnet"},
		{"claude-opus-4-5-20250101", "opus"},
		{"claude-opus-4-1-20240101", "opus_legacy"},
		{"claude-opus-4.0", "opus_legacy"},
		{"claude-haiku-4-5", "haiku"},
		{"claude-haiku-3-5", "haiku_legacy"},
		{"claude-fable-5", "fable"},
		{"mythos-preview", "fable"},
		{"synthetic-model", "haiku"},
		{"totally-unknown", defaultFamily},
		{"", defaultFamily},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			assert.Equal(t, tc.want, modelFamily(tc.model))
		})
	}
}
