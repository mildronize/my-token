package bff

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mildronize/my-token/internal/bffapi"
	"github.com/mildronize/my-token/internal/domain/usage"
)

// UsageServer adapts usage.Service's read side to internal/bffapi's
// generated ServerInterface's usage-shaped subset (GetUsageSummary,
// GetUsageWindows) — story-1/ticket-14's console read surface. Mirrors
// KeysServer/UsersServer's own shape: a thin adapter, no business logic
// of its own (WindowBounds/Aggregate, both in internal/domain/usage, do
// all of it). Owner-session only, the same tier as every other endpoint
// on this surface (bffOwnerID below) — there is no publicapi equivalent
// at all (contract's API surface section: only `/api/bff/usage/*`
// exists, ingestion is the only thing `/api/v1` exposes for this
// domain).
type UsageServer struct {
	Service *usage.Service
}

// NewUsageServer builds a UsageServer on top of svc.
func NewUsageServer(svc *usage.Service) *UsageServer {
	return &UsageServer{Service: svc}
}

// toWireTotals converts a domain usage.Totals into the wire UsageTotals
// shape (bff-openapi.yaml) — the one place this file touches the
// generated type's field names for totals.
func toWireTotals(t usage.Totals) bffapi.UsageTotals {
	return bffapi.UsageTotals{
		Tokens: t.Tokens,
		Cost:   t.Cost,
		Turns:  t.Turns,
	}
}

// toWireBreakdown converts a []usage.BreakdownRow into the wire
// []UsageBreakdownRow — Key is the dimension's display value (contract's
// "Console display rules": display shortening beyond that is the SPA's
// own job, never done here). RawKey (story-1/ticket-18) is only set on
// the wire when usage.BreakdownRow.RawKey is non-empty — Service.Summary
// only ever populates it for group_by=machine's hostname substitution, so
// every other group_by's rows carry no raw_key at all on the wire
// (bff-openapi.yaml's own `raw_key` doc comment: "absent for every other
// group_by").
func toWireBreakdown(rows []usage.BreakdownRow) []bffapi.UsageBreakdownRow {
	out := make([]bffapi.UsageBreakdownRow, 0, len(rows))
	for _, r := range rows {
		row := bffapi.UsageBreakdownRow{
			Key:    r.Key,
			Tokens: r.Tokens,
			Cost:   r.Cost,
			Turns:  r.Turns,
		}
		if r.RawKey != "" {
			rawKey := r.RawKey
			row.RawKey = &rawKey
		}
		out = append(out, row)
	}
	return out
}

// GetUsageSummary implements bffapi.ServerInterface —
// GET /api/bff/usage/summary?window=...&group_by=.... window/group_by are
// both required, enum-constrained query parameters (bff-openapi.yaml's
// own `enum` on GetUsageSummaryParams) — the request validator mounted
// ahead of this handler (cmd/server/main.go's wireBFF) already rejects
// anything outside the fixed five/three values before this is ever
// reached, so ParseWindow/ParseGroupBy failing here is unreachable in
// practice; it's still checked, not assumed, the same defensive-fallback
// convention every other handler in this package follows for its own
// request validator.
func (s *UsageServer) GetUsageSummary(c *gin.Context, params bffapi.GetUsageSummaryParams) {
	if _, ok := bffOwnerID(c); !ok {
		return
	}

	window, ok := usage.ParseWindow(string(params.Window))
	if !ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, bffValidationErrorBody("unrecognised window value", "window"))
		return
	}
	groupBy, ok := usage.ParseGroupBy(string(params.GroupBy))
	if !ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, bffValidationErrorBody("unrecognised group_by value", "group_by"))
		return
	}

	result, err := s.Service.Summary(c.Request.Context(), window, groupBy, time.Now())
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, bffapi.UsageSummary{
		Totals:            toWireTotals(result.Totals),
		Breakdown:         toWireBreakdown(result.Breakdown),
		ReportingInstalls: result.ReportingInstalls,
	})
}

// GetUsageWindows implements bffapi.ServerInterface —
// GET /api/bff/usage/windows: the fixed 5h/24h/today/week/month/year/
// lifetime table (story-1/ticket-20 grew this from five to seven rows),
// no group_by (usage.Service.Windows' own doc comment).
func (s *UsageServer) GetUsageWindows(c *gin.Context) {
	if _, ok := bffOwnerID(c); !ok {
		return
	}

	rows, err := s.Service.Windows(c.Request.Context(), time.Now())
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	wire := make([]bffapi.UsageWindowRow, 0, len(rows))
	for _, r := range rows {
		wire = append(wire, bffapi.UsageWindowRow{
			Window: bffapi.UsageWindowRowWindow(r.Window),
			Turns:  r.Totals.Turns,
			Tokens: r.Totals.Tokens,
			Cost:   r.Totals.Cost,
		})
	}
	c.JSON(http.StatusOK, bffapi.UsageWindows{Windows: wire})
}
