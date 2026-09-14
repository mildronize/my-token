# 2: How do we prove multi-machine reporting for real?

Type: wayfinder:grilling
Status: resolved
Blocked by: None (can start immediately)

## Question

This fleet has real registered ships (`~/.typ-fleet/fleet/ships.json` on
thw-home): `thw-home`, `thw-home-prod`, `thw-home-gpu`. Should the second
"machine" this story proves against be a real second host, a simulated
second `install_id` on this same machine, or both?

## Answer

**Revised after loop-readiness review — originally required a real second
host, later replaced with a same-machine real second process once a
concrete, safe way to do that surfaced.**

First pass (superseded): build/iterate against a simulated `install_id`,
but require one real second **host** (not `thw-home-prod`, likely
`thw-home-gpu`) round-tripping a real report before calling the story done.
Flagged during a loop-readiness review as unsafe for unattended
`/chief-loop`/`/chief-autopilot`: installing/running something on a
separate real host is infra-crossing, outside what a code-only ticket
should attempt without a human's hand (มายด์'s own standing rule: infra
beyond this repo routes through him/hestia).

**Resolved instead: build/iterate against a simulated `install_id` as
before, and replace the real-second-host requirement with a real second
collector *process* on this same machine** — `~/.claude-openrouter`
(confirmed real: a separate Claude config directory with its own real
`projects/*.jsonl` transcripts, not fabricated fixtures) becomes a second
collector install's scan root, given its own distinct `install_id`,
reporting to the same real core service as the first. This is a real
binary, real transcripts, real HTTP POST, real auth — a materially
stronger proof than simulated fixture data — while staying entirely inside
this machine's own environment, so it's safe for an unattended
`/chief-build` session to run and assert against itself.

**Known, accepted gap:** this does not prove genuine cross-host behavior —
real network reachability between two different physical/logical hosts, or
whether the collector builds/runs cleanly in a different host's own
environment (OS, Go version, filesystem layout). Explicitly ruled
acceptable, มายด์'s call: real cross-host rollout stays deferred to the
already-planned fleet-wide rollout follow-up story (goal.md's Out of
Scope), same boundary as before — this story only needed to prove
multi-machine *correctness*, not deployment.

`thw-home-prod` was never a candidate either way — using the prod ship for
this story's own testing would conflate dev work with the separate, later
"deploy to prod" decision.
