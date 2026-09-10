// Package usage is the usage_events domain module (story-1/ticket-11): a
// collector-reported, append-only, idempotent-on-id ledger of per-turn
// Claude Code token usage. Modeled on internal/domain/todo's own
// Repository+Service shape (docs/GETTING-STARTED.md's worked example),
// but deliberately simpler — there is no per-actor ownership scoping (I3
// does not apply here, the same way it stopped applying to `todos` —
// GOAL.md's Ownership model decision, this domain's own
// TestI3_..._ScopingDoesNotApplyToThisDomain), no multi-step permission
// dispatch, and only one write operation (IngestBatch), not an
// append-only event log with several distinct event types.
//
// It holds no transport code: the HTTP adapter that exposes
// POST /api/v1/usage-events/batch lives in
// internal/transport/publicapi/usage_handler.go instead, outside every
// domain module (ARCHITECTURE.md — "Why transport is not inside a domain
// module anymore").
package usage
