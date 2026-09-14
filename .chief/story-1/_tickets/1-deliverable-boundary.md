# 1: What's this story's deliverable boundary?

Type: wayfinder:grilling
Status: resolved
Blocked by: None (can start immediately)

## Question

Does this story deliver the my-token service itself (working single-machine), the
service plus its rollout across every ship, or just a design with no working code?
Separately: does "done" require a deployed/reachable console, or is a local/dev
console enough?

## Answer

Single-machine scope: this story delivers a working my-token collector CLI +
console UI, runnable on one machine, proving the actor/path/machine data model.
A working local/dev console is enough — it does not need to be deployed or
reachable to count as done. Fleet-wide rollout (installing on every ship,
typ-fleet's multi-ship architecture, "shipd"-style multi-location install) is a
separate follow-up story, not this one.

Rationale: matches the ticket's own "use low effort design if possible" note;
fleet rollout depends on tiana's multi-ship architecture input, which the source
ticket itself defers ("ask tiana"); a single-machine version is the cheapest way
to validate the data model before taking on deploy/rollout decisions.
