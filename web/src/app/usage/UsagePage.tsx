// story-1/ticket-14: the console — bff endpoints (~/lib/usage.ts) + this
// UI, matching the approved mockup
// (https://claude.ai/code/artifact/c1382132-8662-490a-9e7a-8d7d8f20ea6d)
// and the contract's "Console display rules" section.
//
// Query plan (six requests per load/window change, all independently
// cacheable by TanStack Query — see ~/lib/usage.ts's query keys):
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
//   - windowsTable (GET /usage/windows): the fixed five-row table.
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
import { useState } from "react";

import "~/styles/usage-console.css";
import { ErrorState } from "~/components/ErrorState";
import { Spinner } from "~/components/ui/spinner";
import {
  useUsageSummaryQuery,
  useUsageWindowsQuery,
  type UsageWindow,
} from "~/lib/usage";
import { WindowTabs } from "./WindowTabs";
import { SummaryTiles } from "./SummaryTiles";
import { WindowsTable } from "./WindowsTable";
import { BreakdownPanel } from "./BreakdownPanel";

const DEFAULT_WINDOW: UsageWindow = "today";

export default function UsagePage() {
  const [selectedWindow, setSelectedWindow] = useState<UsageWindow>(DEFAULT_WINDOW);

  const lifetimeByActor = useUsageSummaryQuery("lifetime", "actor");
  const lifetimeByPath = useUsageSummaryQuery("lifetime", "path");
  const windowByActor = useUsageSummaryQuery(selectedWindow, "actor");
  const windowByPath = useUsageSummaryQuery(selectedWindow, "path");
  const windowByMachine = useUsageSummaryQuery(selectedWindow, "machine");
  const windowsTable = useUsageWindowsQuery();

  const queries = [lifetimeByActor, lifetimeByPath, windowByActor, windowByPath, windowByMachine, windowsTable];
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
  if (firstError || !lifetimeByActor.data || !lifetimeByPath.data || !windowByActor.data || !windowByPath.data || !windowByMachine.data || !windowsTable.data) {
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
          />
          <BreakdownPanel
            title="By path"
            groupBy="path"
            breakdown={windowByPath.data.breakdown}
            totalsCost={windowByPath.data.totals.cost}
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
