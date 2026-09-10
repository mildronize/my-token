// Package collector is story-1/ticket-12's standalone collector: scans
// Claude Code transcript files on disk, dedupes per-turn usage by
// message.id (ticket 9), attributes each turn to an actor/path/machine,
// estimates a local cost, and reports batches to ticket 11's core
// service (POST /api/v1/usage-events/batch).
//
// This lives under internal/ (not as a domain module — see
// .chief/_rules/_standard/ARCHITECTURE.md) because it is not part of the
// core HTTP service's request path at all: it never imports gin, is
// never wired into cmd/server, and has its own entrypoint
// (cmd/collector/main.go) that runs as a wholly separate process/binary,
// typically not even on the same machine as the core service. It is
// still part of this repo (ticket 12's own scoping note: "part of the
// same fork/repo, in whatever location/package structure makes sense
// for a Go CLI") because this story never requires the collector to be
// independently distributable — that's a real, but different, follow-up
// concern (goal.md's "installing the collector everywhere" is out of
// scope for this story).
//
// Deliberately does NOT import internal/domain/usage for its pricing
// function even though this is the same Go module and could: this
// package's CostForUsage (pricing.go) is its own ported copy, computing
// only a local, non-authoritative estimate — the server (internal/domain
// /usage.CostForUsage) is what actually gets written to usage_events.cost
// (ticket 11: the server never trusts a client-supplied cost). Keeping
// two independent copies, each with its own table-driven tests, matches
// how a real deployment would actually work (the collector installed on
// an end-user's machine, potentially built from a copy of just this
// package, with no access to the core service's own source tree) and
// avoids this package accidentally depending on internal/domain/usage's
// unrelated Repository/Service machinery just to reach one pure
// function.
package collector
