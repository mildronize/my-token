# 6: Filter mechanics — server-side query params or client-side?

Type: wayfinder:grilling
Status: resolved
Blocked by: 4, 5

## Question

The console today (`UsagePage.tsx`) has no filters at all — it fires six
fixed queries per load (lifetime/window × actor/path/machine) and renders
three always-all-rows breakdown panels. Adding a machine filter and a
registered-path-name filter (point 2/3 of มายด์'s brief) needs a mechanism:
server-side query params or client-side filtering; how filters compose with
each other and the window tabs; and whether a filter scopes the whole
dashboard or just its own panel.

## Answer

**Server-side, global-scoping, filters-only persistence** — confirmed with
มายด์ across three sub-questions:

1. **Mechanism: server-side query params.** `GET /usage/summary` already
   takes `window` and `group_by` as required query params (`bff-openapi.yaml`)
   — filtering is already a server-side concept in this API. Add optional
   `machine` (install_id) and `scan_root` (name or id, per ticket 5) query
   params to both `GET /usage/summary` **and** `GET /usage/windows` (which
   has zero params today — needs them added so the fixed windows table also
   respects an active filter), applied as additional constraints before
   `Aggregate()` runs. Rejected client-side filtering: would mean shipping
   the full unfiltered dataset to the browser just to discard most of it,
   which gets worse as more machines/scan-roots report — directly against
   this story's own point.

2. **Composition: global, not per-panel.** Selecting a machine or scan-root
   filter narrows every query on the page — By actor and By path show only
   that machine/scan-root's contribution, not just the By machine panel's own
   rows. Filters AND together with each other and with the active window.
   Known, accepted cosmetic consequence: filtering to one machine makes the
   By machine panel itself trivial (one row, 100% share) — not hidden, just
   thin; no special-casing requested.

3. **localStorage scope: the two new filters only, not the window tab.**
   Machine and scan-root filter selections persist across reloads; the
   window tab (`today`/`week`/etc.) keeps resetting to `DEFAULT_WINDOW =
   "today"` on every load, unchanged from today's behavior. Explicitly not
   expanded to a full "restore where I left off" experience — sticking to
   the literal ask rather than growing scope.

**Resolves the last open fog item too** (map's "Not yet specified": where
ticket 3's dedup exclusion logic lives). Since filtering is now confirmed
server-side, ticket 3's exclusion belongs there too, not in
`BreakdownPanel.tsx` — it must apply before the top-8 cutoff (`BREAKDOWN_TOP_N`
in `BreakdownPanel.tsx`) so an excluded redundant-path row never occupies a
slot a real project path should have. `BreakdownPanel.tsx`'s own
display-shortening logic (basename, collision walk-up) stays client-side,
unaffected — that's purely a label transform over rows the server already
decided to include. (Ticket 3 itself was later revised away from any
`.typ-crews`-specific pattern match to a generic per-session structural
comparison — this ticket's own conclusion about *where* the check lives is
unaffected by that revision.)
