// story-1/ticket-14: the console — bff endpoints (~/lib/usage.ts) + this
// UI, matching the approved mockup
// (https://claude.ai/code/artifact/c1382132-8662-490a-9e7a-8d7d8f20ea6d)
// and the contract's "Console display rules" section.
//
// Query plan (six requests per load/window/filter change, all
// independently cacheable by TanStack Query — see ~/lib/usage.ts's query
// keys), plus two more added by story-2/ticket-11 (below):
//   - lifetimeByActor (window=lifetime, group_by=actor): tile 1's cost +
//     actor count, tile 3's total tokens.
//   - lifetimeByPath (window=lifetime, group_by=path): tile 1's path
//     count only.
//   - windowByActor/windowByPath/windowByMachine (the selected window,
//     one call per breakdown panel): each panel's own `breakdown`, and
//     `reporting_installs`/tile 2's cost off whichever of the three is
//     convenient (they're identical across group_by for a fixed window —
//     internal/domain/usage.Aggregate computes reporting_installs from
//     the whole window's event set, independent of the grouping
//     dimension, verified by TestAggregate_ReportingInstalls_
//     IsIndependentOfGroupByDimension).
//   - windowsTable (GET /usage/windows): the fixed seven-row table
//     (story-1/ticket-20: grew from five rows to seven — year and
//     lifetime are now both real, selectable tabs too).
//
// story-2/ticket-11 (console filters) additions:
//   - Every one of the six requests above grows two new optional params,
//     `machine`/`scanRoot` (contract's "Console filters" section: global
//     scoping, not per-panel) — see useUsageFilters.ts for their
//     localStorage persistence.
//   - machineOptionsQuery (window=selectedWindow, group_by=machine,
//     scanRoot only — never `machine`) and scanRootsQuery (GET
//     /usage/scan-roots, no params ever) exist purely to populate the two
//     filter dropdowns' own option lists; see UsageFilters.tsx.
//
// Known gap, flagged rather than worked around: the approved mockup's
// top bar also shows a "host" pill (e.g. `host thw-home`). No hostname
// is available to this endpoint to show — ticket 11's own ingestion
// handler reads `hostname` off POST /api/v1/usage-events/batch's request
// body but never persists it (usage_handler.go's own doc comment: "read
// and discarded here"). Adding a column to store it is a schema change
// outside this ticket's own scope (ticket 11's contract, not ticket
// 14's) — the host pill is omitted here rather than fabricating a value
// or silently repurposing `machine` (the install_id) as a stand-in for
// a hostname, which it isn't. See ticket-14-report.md's Decision section.
import { useEffect } from "react";

import "~/styles/usage-console.css";
import { ErrorState } from "~/components/ErrorState";
import { Spinner } from "~/components/ui/spinner";
import { useUsageSummaryQuery, useUsageWindowsQuery, useUsageScanRootsQuery } from "~/lib/usage";
import { useUsageFilters } from "./useUsageFilters";
import { UsageFilters, machineOptionsFromBreakdown, scanRootOptionsFromList } from "./UsageFilters";
import { WindowTabs } from "./WindowTabs";
import { SummaryTiles } from "./SummaryTiles";
import { WindowsTable } from "./WindowsTable";
import { BreakdownPanel } from "./BreakdownPanel";

export default function UsagePage() {
  // story-2/ticket-11: the console's two new filters (contract's "Console
  // filters" section) — persisted to localStorage (useUsageFilters), AND
  // composed with each other and with `window`, applied globally below
  // (every panel's own query grows the same two optional params, not just
  // one panel's).
  //
  // story-2/ticket-15: the window tab now persists too (useUsageFilters
  // owns all three — machine/scanRoot/window are the same kind of state,
  // same get/set/persist shape).
  const { machine, scanRoot, window: selectedWindow, setMachine, setScanRoot, setWindow: setSelectedWindow } = useUsageFilters();
  const filters = { machine, scanRoot };

  const lifetimeByActor = useUsageSummaryQuery("lifetime", "actor", filters);
  const lifetimeByPath = useUsageSummaryQuery("lifetime", "path", filters);
  const windowByActor = useUsageSummaryQuery(selectedWindow, "actor", filters);
  const windowByPath = useUsageSummaryQuery(selectedWindow, "path", filters);
  const windowByMachine = useUsageSummaryQuery(selectedWindow, "machine", filters);
  const windowsTable = useUsageWindowsQuery(filters);

  // machineOptionsQuery backs the machine filter's own dropdown (contract:
  // "options come from the existing group_by=machine breakdown response")
  // — deliberately NOT scoped by `machine` itself, so the option list
  // stays complete once a machine is selected instead of collapsing to a
  // single row. It IS scoped by `scanRoot`, same as every panel above.
  // When no machine is selected this is the exact same request as
  // windowByMachine (TanStack Query dedupes identical cache keys) — the
  // two only diverge, and only then fetch separately, once a machine
  // filter is actually active.
  const machineOptionsQuery = useUsageSummaryQuery(selectedWindow, "machine", { scanRoot });
  const scanRootsQuery = useUsageScanRootsQuery();

  // Reconcile a value restored from localStorage against the real data
  // once it has actually loaded (contract: "An invalid/stale stored value
  // ... is dropped silently on restore ... falls back to 'no filter' the
  // same way a first-ever visit would"). There's no way to validate a
  // restored value before this data exists, so the very first fetch after
  // a reload still optimistically includes it — these effects are what
  // clear it once the option list proves it's gone. Validity is checked
  // against the very same `.value` the dropdown's own options use
  // (machineOptionsFromBreakdown/scanRootOptionsFromList) rather than
  // re-deriving the key shape here — one definition of "what counts as
  // this filter's identity," not two that could quietly drift apart.
  useEffect(() => {
    if (!machineOptionsQuery.data || machine === undefined) return;
    const options = machineOptionsFromBreakdown(machineOptionsQuery.data.breakdown);
    if (!options.some((o) => o.value === machine)) setMachine(undefined);
  }, [machineOptionsQuery.data, machine, setMachine]);

  useEffect(() => {
    if (!scanRootsQuery.data || scanRoot === undefined) return;
    const options = scanRootOptionsFromList(scanRootsQuery.data.scan_roots);
    if (!options.some((o) => o.value === scanRoot)) setScanRoot(undefined);
  }, [scanRootsQuery.data, scanRoot, setScanRoot]);

  const queries = [
    lifetimeByActor,
    lifetimeByPath,
    windowByActor,
    windowByPath,
    windowByMachine,
    windowsTable,
    machineOptionsQuery,
    scanRootsQuery,
  ];
  const isLoading = queries.some((q) => q.isLoading);
  const firstError = queries.find((q) => q.isError);

  if (isLoading) {
    return (
      <div className="usage-console">
        <div className="flex min-h-[300px] items-center justify-center">
          <Spinner size="lg" />
        </div>
      </div>
    );
  }

  // Checked per-variable (not `queries.every((q) => q.data)`) so
  // TypeScript actually narrows each named query's own `.data` to
  // non-undefined below — a array-based check can't narrow the
  // individually-named bindings the JSX further down reads directly.
  if (
    firstError ||
    !lifetimeByActor.data ||
    !lifetimeByPath.data ||
    !windowByActor.data ||
    !windowByPath.data ||
    !windowByMachine.data ||
    !windowsTable.data ||
    !machineOptionsQuery.data ||
    !scanRootsQuery.data
  ) {
    return (
      <div className="usage-console">
        <ErrorState
          message="Could not load usage data."
          onRetry={() => queries.forEach((q) => void q.refetch())}
        />
      </div>
    );
  }

  const reportingInstalls = windowByActor.data.reporting_installs;

  return (
    <div className="usage-console">
      <div className="topbar">
        <div className="id">
          <h1>my-token</h1>
          <span className="tag">console</span>
        </div>
        <div className="meta">
          <span className="pill">
            <span className="dot" />
            <strong>{reportingInstalls}</strong> install{reportingInstalls === 1 ? "" : "s"} reporting
          </span>
          <UsageFilters
            machineOptions={machineOptionsFromBreakdown(machineOptionsQuery.data.breakdown)}
            scanRootOptions={scanRootOptionsFromList(scanRootsQuery.data.scan_roots)}
            selectedMachine={machine}
            selectedScanRoot={scanRoot}
            onMachineChange={setMachine}
            onScanRootChange={setScanRoot}
          />
          <WindowTabs value={selectedWindow} onChange={setSelectedWindow} />
        </div>
      </div>

      <SummaryTiles
        window={selectedWindow}
        lifetimeByActor={lifetimeByActor.data}
        lifetimeByPath={lifetimeByPath.data}
        windowByActor={windowByActor.data}
      />

      <WindowsTable rows={windowsTable.data.windows} />

      <section aria-label="Breakdowns">
        <div className="section-head">
          <h2>By actor / path / machine</h2>
          <span className="note">bars are share of this window&rsquo;s cost</span>
        </div>
        <div className="breakdowns">
          <BreakdownPanel
            title="By actor"
            groupBy="actor"
            breakdown={windowByActor.data.breakdown}
            totalsCost={windowByActor.data.totals.cost}
            activeMachine={machine}
          />
          <BreakdownPanel
            title="By path"
            groupBy="path"
            breakdown={windowByPath.data.breakdown}
            totalsCost={windowByPath.data.totals.cost}
            activeMachine={machine}
          />
          <BreakdownPanel
            title="By machine"
            groupBy="machine"
            breakdown={windowByMachine.data.breakdown}
            totalsCost={windowByMachine.data.totals.cost}
          />
        </div>
      </section>

      <footer>
        <span>
          Every figure above is this <strong>one install&rsquo;s</strong> own sessions — not a
          project&rsquo;s whole cost. A path&rsquo;s true total needs every reporting install
          summed at the core.
        </span>
      </footer>
    </div>
  );
}
