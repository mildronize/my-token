package collector

import "strings"

// modelRate is one model family's $/1M-token rates — ported verbatim
// (same field shape and rates) from internal/domain/usage/pricing.go,
// ticket 11's own server-side copy, which itself ports the reference
// collector's PRICING table. See doc.go for why this package keeps its
// own independent copy rather than importing that one.
type modelRate struct {
	Input        float64
	Output       float64
	CacheWrite5m float64
	CacheRead    float64
}

var pricingTable = map[string]modelRate{
	// Fable 5 — Anthropic's most capable widely released model (above Opus tier).
	"fable": {Input: 10.00, Output: 50.00, CacheWrite5m: 12.50, CacheRead: 1.00},
	// Opus 4.5+ (4.5/4.6/4.7/4.8) — new lower pricing tier.
	"opus": {Input: 5.00, Output: 25.00, CacheWrite5m: 6.25, CacheRead: 0.50},
	// Opus 4.1 and earlier (legacy, deprecated).
	"opus_legacy":  {Input: 15.00, Output: 75.00, CacheWrite5m: 18.75, CacheRead: 1.50},
	"sonnet":       {Input: 3.00, Output: 15.00, CacheWrite5m: 3.75, CacheRead: 0.30},
	"haiku":        {Input: 1.00, Output: 5.00, CacheWrite5m: 1.25, CacheRead: 0.10},
	"haiku_legacy": {Input: 0.80, Output: 4.00, CacheWrite5m: 1.00, CacheRead: 0.08},
}

const defaultFamily = "sonnet"

// modelFamily maps a model id to a pricingTable key — ported verbatim
// from the same source internal/domain/usage/pricing.go already ported
// from (the reference collector's model_family()).
func modelFamily(m string) string {
	if m == "" {
		return defaultFamily
	}
	lower := strings.ToLower(m)
	switch {
	case strings.Contains(lower, "fable") || strings.Contains(lower, "mythos"):
		return "fable"
	case strings.Contains(lower, "opus"):
		for _, v := range []string{"4-5", "4-6", "4-7", "4-8", "4.5", "4.6", "4.7", "4.8"} {
			if strings.Contains(lower, v) {
				return "opus"
			}
		}
		for _, v := range []string{"4-0", "4-1", "4.0", "4.1"} {
			if strings.Contains(lower, v) {
				return "opus_legacy"
			}
		}
		return "opus" // future Opus models default to the new tier.
	case strings.Contains(lower, "sonnet"):
		return "sonnet"
	case strings.Contains(lower, "haiku"):
		for _, v := range []string{"3-5", "3.5"} {
			if strings.Contains(lower, v) {
				return "haiku_legacy"
			}
		}
		return "haiku"
	case strings.Contains(lower, "synthetic"):
		return "haiku"
	default:
		return defaultFamily
	}
}

// CostForUsage computes a turn's LOCAL ESTIMATED cost from its deduped
// usage dict's four token counts (ticket 9: read directly off the dict,
// never summed into each other) and its model id — a pure function, no
// I/O. This is the collector's own display/estimate figure only; the
// core service recomputes cost itself and never trusts this value
// (ticket 11, doc.go).
func CostForUsage(model string, inputTokens, outputTokens, cacheReadInputTokens, cacheCreationInputTokens int64) float64 {
	rate := pricingTable[modelFamily(model)]
	return float64(inputTokens)*rate.Input/1e6 +
		float64(outputTokens)*rate.Output/1e6 +
		float64(cacheReadInputTokens)*rate.CacheRead/1e6 +
		float64(cacheCreationInputTokens)*rate.CacheWrite5m/1e6
}
