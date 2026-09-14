# 5: Does typ-fleet's multi-ship architecture affect this story's schema?

Type: wayfinder:task
Status: resolved
Blocked by: None (can start immediately)

## Question

The source ticket says "ask tiana the multi ship architecture" regarding
multi-agent install locations (e.g. typ-fleet, 3 machines today). Fleet-wide
rollout is a follow-up story (see ticket 1), but does tiana's architecture
constrain how `machine` (ticket 2) should be named/shaped now, to avoid a
mismatch when rollout happens later?

## Answer

No stable/global machine identifier exists in typ-fleet today to borrow —
don't shape the field around it. Grounded in `crates/fleet/src/registry.rs`
(per tiana):

- `ShipEntry` is just `{ name, url, joined_at }` (registry.rs:17-19).
  `Registry::join()` (line 75) only checks name/url uniqueness within one
  host's own `ships.json` — no UUID, no hardware fingerprint.
- That registry is per-host, not synced across the fleet (see fleet-wide
  learning `fleet-ship-registry-is-per-host-not-shared`) — `~/.typ-fleet/fleet/ships.json`
  is local state populated only by `fleet join` run on that host. Two hosts'
  registries are independent files.
- Consequence: "ship name" is registry-relative, not a canonical machine
  identity — the same physical machine could get different names across
  registries, or a new name after `leave`+rejoin.

Decision: my-token's `machine` field (ticket 2) is an opaque per-install
identifier my-token mints and owns itself — a UUID generated on first run,
persisted to disk. Named `install_id` (or `machine_id`), deliberately not
`ship_id`/`ship_name`, so it's never confused with typ-fleet's own concept.
Any human-readable hostname is a separate display field, decoupled from the
identity key — this also means my-token works standalone on a machine that
never runs `fleet join`.

For the deferred fleet-rollout follow-up story: leave room for an *optional*
correlation field added later (e.g. `fleet_ship_name: Option<String>`) an
operator can populate by hand to link an install to a known ship name.
Additive, not required, and not derivable automatically today — there's no
typ-fleet API that hands out a stable canonical name.
