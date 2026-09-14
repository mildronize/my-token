# Goal

my-token is a lightweight **collector** installable on each machine, reporting
to a **core service** over the network, which exposes a **console UI**
showing usage across every machine that has reported in — sliced by `actor`,
`path`, and `machine`, with a time-window breakdown (5h/24h/today/week/month)
and per-dimension totals.

The collector scans one or more transcript root paths on the machine it runs
on — not hardcoded to a single default — configured via a JSON config file,
so a machine with multiple Claude installs (e.g. `~/.claude` and
`~/.claude-local`) gets usage from all of them, not just one.

This story delivers that architecture working end-to-end: at least one
collector installation, configured with one or more scan paths, actually
reporting real usage data to the core service, and the console displaying it
correctly. It does not require installing the collector on every machine in
an organization — that operational rollout is a follow-up story. What this
story must prove is the collector → core → console *path* itself (including
multi-path scanning on a single install), not full coverage.

**Note on `path` totals:** any single install's `path` total reflects only
that machine's own sessions — never a project's whole cost. A path's true
total requires the core service summing every install's reports for that
path, over its whole lifetime. The console should make this partial-view
nature visible (e.g. label totals as "from N reporting installs"), not
imply completeness it can't have with fewer than all real installs
reporting.

## Out of Scope

- Installing/rolling the collector out everywhere — follow-up story; this
  story proves the reporting path works, not full coverage.
- Deploying the core service/console anywhere externally reachable — a
  local/dev-run core is sufficient for this story.
- Correlating `install_id` with any external fleet/registry system — an
  optional correlation field is structurally allowed for later, not
  implemented or populated here.
- Non-Claude-Code sources (Cowork mode, other assistants).
- Auto-discovering non-default install paths — this story requires them to be
  named explicitly in the config file, not detected automatically.
