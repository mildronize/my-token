# 14: Console — bff endpoints + UI

Type: implementation
Status: resolved
Blocked by: 11 (schema/API must exist; real demoable data wants 12 done too)

## Delivers

- `GET /api/bff/usage/summary?window=5h|24h|today|week|month&group_by=actor|path|machine`
  → `{ totals: {tokens, cost, turns}, breakdown: [{key, tokens, cost, turns}], reporting_installs: N }`.
  `reporting_installs` = distinct `machine`s with at least one event inside
  the selected window, not lifetime — this was left open when the contract
  was written (mild didn't confirm it); deciding it here as window-scoped
  since that's the only definition that always matches what the displayed
  totals actually cover. Flag for confirmation during build if that's wrong.
- `GET /api/bff/usage/windows` → the fixed 5h/24h/today/week/month table.
- Embedded SPA console UI (amber/gold theme, per ticket 3 and the approved
  mockup — https://claude.ai/code/artifact/c1382132-8662-490a-9e7a-8d7d8f20ea6d):
  - Top bar: title, "N install(s) reporting" pill, hostname, time-window
    tabs.
  - Summary tiles: lifetime cost, this-window cost, total tokens.
  - Time-window table.
  - Three breakdown panels (by actor / by path / by machine), each a ranked
    list with a proportional cost-share bar.
  - **Path display rule** (contract's "Console display rules" section):
    basename only; on collision within the same rendered list, walk up
    parent segments until unique (no fixed cap); if two rows still share the
    literal identical path string (same path, different `machine`), append
    the machine as a last-resort disambiguator — this is UI-only, it does
    not change what's aggregated; full canonical path always available via
    a tooltip on the label, whether or not it needed shortening.
  - Footer: the partial-view caveat, in plain words (goal's "Note on `path`
    totals").

Demoable/verifiable on its own: with tickets 11/12 (and ideally 13) done,
open the console against this machine's own real collected data and confirm
the numbers and breakdowns match what querying the DB directly shows.
