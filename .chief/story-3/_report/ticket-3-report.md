# Ticket 3 Report

## Ticket
Frontend: `/machines` route, nav link, `MachinesPage` — summary strip +
table wired to ticket 1's `GET /api/bff/machines`, row click navigates to
the (not-yet-built) detail route.

## Outcome
done

## Notes

- Committed directly onto the shared `story-3/machines-page` branch
  (`3633480`), no new branch, no PR — per มายด์'s "one PR per story"
  correction.
- Pre-flight confirmed ticket 1's endpoint was actually present on the
  branch before building against it, rather than assuming.
- Had to regenerate the frontend's generated API types
  (`bff-schema.gen.ts`) — tickets 1/2 updated `bff-openapi.yaml` but never
  regenerated the frontend side, so `Machine`/`MachineList` and
  `source_type` weren't consumable yet. One stale test fixture
  (`UsageFilters.test.tsx`, missing the now-required `source_type`) fell
  out of that and got a one-line fix — no behavior change.
- New `formatDateTime` formatter (not `formatDate`) for "Last reported" —
  date-only would hide same-day staleness, which matters for the "is my
  fleet alive" framing the goal calls out.
- `/chief-review-code` clean on Spec; two minor Standards notes, one fixed
  (inline `cursor: pointer` → `.clickable-row` CSS class), one left as a
  judgment call (duplicate onClick/onKeyDown handler wiring, flagged by
  the reviewer as a future-divergence seed, not a present problem).
- Tests: new `MachinesPage.test.tsx` 4/4, full suite 71/71, typecheck +
  build clean.
- **Operational flag**: mid-build, the shared repo checkout got switched
  to an unrelated branch (`docs/readme-coauthoring`) by something else
  using the same directory concurrently. Caught via a diff-stat sanity
  check before any writes, recovered cleanly. No damage, but this
  checkout isn't exclusively held during a session — worth a branch/HEAD
  check before each ticket build going forward, not just the first.
