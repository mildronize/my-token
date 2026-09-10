package usage

import "strings"

// modelRate is one model family's $/1M-token rates.
type modelRate struct {
	Input        float64
	Output       float64
	CacheWrite5m float64
	CacheRead    float64
}

// pricingTable is ported verbatim (rates and family-detection logic) from
// the reference collector's own PRICING table
// (~/tmp/claude-token-tracker/scan_claude_usage.py, `PRICING`/
// `model_family`) — the contract's "ported pricing table (pure function)"
// requirement (Testing Decisions: table-driven unit tests, no framework
// dependency). Ticket 11's own text points at "ticket 13" for this table,
// which is a stale cross-reference — ticket 13 (path-attribution-upgrade)
// never mentions pricing at all; ticket 12 is the one that actually
// describes porting this table (as the collector's own pure function,
// for a client-side estimate). This is a SEPARATE, server-side copy: the
// core service must compute `cost` itself and never trust a
// client-supplied value (this ticket's own text), so the rates need to
// live here too, not only in ticket 12's future collector. See this
// package's own report/escalation note for the cross-reference mismatch.
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
// from the reference collector's model_family(): "Versions matter — Opus
// 4.5+ is 3x cheaper than Opus 4.1, and Haiku 3.5 is slightly cheaper
// than 4.5."
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

// CostForUsage computes a turn's cost from its deduped usage dict's four
// token counts (ticket 9: read directly off the dict, never summed into
// each other) and its model id — a pure function, no I/O, matching the
// contract's Testing Decisions.
//
// cacheCreationInputTokens is billed at the 5m cache-write rate: the
// reference collector's own cost_for_usage() only does this as a
// fallback when the granular 5m/1h breakdown (usage.cache_creation, a
// nested dict Anthropic's API adds separately from the legacy flat
// cache_creation_input_tokens total) is unavailable — this domain's own
// schema (contract's Data model) has only the single flat
// cache_creation_input_tokens column, never the granular breakdown, so
// that fallback is this function's only path, not a special case.
func CostForUsage(model string, inputTokens, outputTokens, cacheReadInputTokens, cacheCreationInputTokens int64) float64 {
	rate := pricingTable[modelFamily(model)]
	return float64(inputTokens)*rate.Input/1e6 +
		float64(outputTokens)*rate.Output/1e6 +
		float64(cacheReadInputTokens)*rate.CacheRead/1e6 +
		float64(cacheCreationInputTokens)*rate.CacheWrite5m/1e6
}
