package publicapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mildronize/my-template/internal/api"
	"github.com/mildronize/my-template/internal/domain/usage"
)

// UsageServer adapts usage.Service to internal/api's generated
// ServerInterface's usage-shaped subset (IngestUsageEventsBatch) —
// story-1/ticket-11's core ingestion endpoint. Mirrors TodoServer's own
// shape (todo_handler.go): a thin adapter over the domain Service, no
// business logic of its own.
type UsageServer struct {
	Service *usage.Service
}

// NewUsageServer builds a UsageServer on top of svc.
func NewUsageServer(svc *usage.Service) *UsageServer {
	return &UsageServer{Service: svc}
}

// toIngestEvent converts one wire-shaped api.UsageEventInput into the
// domain's own usage.IngestEvent — the one place this file touches the
// generated type's field names. Cost/Source have no wire representation
// at all (UsageEventInput carries no such properties, additionalProperties:
// false — openapi.yaml), so there is nothing here for a client value to
// flow through even before usage.Service computes them itself.
func toIngestEvent(e api.UsageEventInput) usage.IngestEvent {
	return usage.IngestEvent{
		ID:                       e.Id,
		SessionID:                e.SessionId,
		Actor:                    e.Actor,
		Path:                     e.Path,
		Machine:                  e.Machine,
		Model:                    e.Model,
		InputTokens:              e.InputTokens,
		OutputTokens:             e.OutputTokens,
		CacheReadInputTokens:     e.CacheReadInputTokens,
		CacheCreationInputTokens: e.CacheCreationInputTokens,
		Timestamp:                e.Timestamp,
	}
}

// IngestUsageEventsBatch implements api.ServerInterface —
// POST /api/v1/usage-events/batch. Requires a resolved actor (any
// Bearer-authenticated caller — the collector's own API key, per
// ticket 11's "reusing my-template's existing auth middleware" decision;
// there is no further per-caller restriction the way I3's ownership
// scoping applies to todos, since this domain has none — see
// internal/domain/usage/repo_test.go's own TestI3_ test). `install_id`/
// `hostname` (api.IngestUsageEventsBatchRequest's own top-level fields)
// are required by the contract's own wire shape (contract's API surface
// section: "Body: { install_id, hostname, events: [...] }") — this
// handler does not invent them speculatively. They identify the
// reporting collector for observability only; ticket 11's domain layer
// has no use for them yet (a future console/collector-health ticket
// might), so they are read and discarded here rather than threaded
// through to usage.Service for no purpose it currently has.
func (s *UsageServer) IngestUsageEventsBatch(c *gin.Context) {
	if _, ok := ActorFromContext(c); !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, unauthorizedBody)
		return
	}

	var req api.IngestUsageEventsBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// openapi.yaml's request validator (internal/api.RequestValidator)
		// already rejects a malformed body, a missing required field, or
		// one declaring `cost`/`source` (additionalProperties: false on
		// UsageEventInput) before this middleware chain reaches the
		// handler at all — this is a defensive fallback, the same
		// convention todo_handler.go's own CreateTodo/UpdateTodo follow.
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	events := make([]usage.IngestEvent, 0, len(req.Events))
	for _, e := range req.Events {
		events = append(events, toIngestEvent(e))
	}

	received, inserted, err := s.Service.IngestBatch(c.Request.Context(), events)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusCreated, api.IngestUsageEventsBatchResponse{
		Received: int64(received),
		Inserted: int64(inserted),
	})
}
