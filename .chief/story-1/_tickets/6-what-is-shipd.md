# 6: What is "shipd," referenced as the install-location model?

Type: wayfinder:research
Status: resolved
Blocked by: None (can start immediately)

## Question

The source ticket says the CLI should install "in different locations, like
shipd." What is shipd?

## Answer

`shipd` (service name `typ-fleet-shipd`) is typ-fleet's existing per-ship
control-path daemon — one instance runs on each ship/machine (e.g. `thw-home`),
and the Fleet UI drives it for control operations. Source: `~/.typmem/memory/retro/2026-08-12-thw-home-prod-onboarding.md`.

Implication for this story: the ticket is pointing at an existing, proven
pattern (one daemon instance per machine, centrally driven) as the model for
my-token's eventual multi-machine rollout — not asking for something novel.
Confirms the `machine` field (ticket 2) is the right shape for that future,
without this story needing to build any of the rollout/control-path machinery
itself (ticket 1 already scoped that out).
