// story-3/ticket-4: the machine detail page — goal's point 5, reachable
// from MachinesPage's own row click (ticket-3). Reuses usage-console.css's
// scoped class set (topbar/pill/tiles/section-head), same "this page is
// /usage's own sibling in framing" reasoning MachinesPage.tsx already
// gives for doing so.
//
// Data sources, both reused rather than duplicated (contract's API
// surface section is explicit that neither gets a second, single-machine
// endpoint):
//   - GET /api/bff/machines (ticket-1, ~/lib/machines.ts) — same response
//     MachinesPage's own table renders, found here by `installId` route
//     param instead of mapped over.
//   - GET /api/bff/usage/scan-roots (ticket-2, ~/lib/usage.ts) — same
//     response the /usage console's scan-root filter dropdown already
//     fetches, filtered client-side to this page's own `installId`.
import { Link, useNavigate, useParams } from "react-router-dom";

import "~/styles/usage-console.css";
import { ErrorState } from "~/components/ErrorState";
import { Spinner } from "~/components/ui/spinner";
import { Button } from "~/components/ui/button";
import { useMachinesQuery } from "~/lib/machines";
import { useUsageScanRootsQuery } from "~/lib/usage";
import { formatCost, formatDateTime, formatInt } from "~/lib/format";
import { presetMachineFilter } from "~/app/usage/useUsageFilters";

export default function MachineDetailPage() {
  const { installId } = useParams<{ installId: string }>();
  const navigate = useNavigate();
  const machinesQuery = useMachinesQuery();
  const scanRootsQuery = useUsageScanRootsQuery();

  const isLoading = machinesQuery.isLoading || scanRootsQuery.isLoading;
  const isError = machinesQuery.isError || scanRootsQuery.isError || !machinesQuery.data || !scanRootsQuery.data;

  if (isLoading) {
    return (
      <div className="usage-console">
        <div className="flex min-h-[300px] items-center justify-center">
          <Spinner size="lg" />
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="usage-console">
        <ErrorState
          message="Could not load this machine."
          onRetry={() => {
            void machinesQuery.refetch();
            void scanRootsQuery.refetch();
          }}
        />
      </div>
    );
  }

  const machine = machinesQuery.data.machines.find((m) => m.install_id === installId);

  // No second, single-machine endpoint (contract) means an unknown
  // installId — a stale bookmark, a typo, a machine that's since dropped
  // out of the response for some other reason — has nowhere left to
  // resolve to. Not a fetch error (the request itself succeeded), so this
  // renders as its own state rather than routing through ErrorState.
  if (!machine) {
    return (
      <div className="usage-console">
        <p className="empty-note">
          No machine found for <span className="mono">{installId}</span>.{" "}
          <Link to="/machines">Back to Machines</Link>.
        </p>
      </div>
    );
  }

  const scanRoots = scanRootsQuery.data.scan_roots.filter((r) => r.install_id === installId);

  // Hoisted out of the closure below so viewInUsageDashboard doesn't need
  // a non-null assertion on `machine` — TypeScript's narrowing from the
  // `if (!machine) return` above doesn't carry into a nested function.
  const machineInstallId = machine.install_id;

  function viewInUsageDashboard() {
    // story-3/ticket-4: this page never mounts useUsageFilters itself (it
    // lives outside UsagePage's own tree) — presetMachineFilter writes the
    // exact same localStorage key directly, then a plain navigate. When
    // UsagePage mounts, its own lazy-initializer reads that value back
    // exactly as it already does for a reload — no new mechanism.
    presetMachineFilter(machineInstallId);
    navigate("/usage");
  }

  return (
    <div className="usage-console">
      <div className="topbar">
        <div className="id">
          <h1>{machine.hostname}</h1>
          <span className="tag mono" title={machine.install_id}>
            {machine.install_id}
          </span>
        </div>
        <div className="meta">
          <Link to="/machines" className="mono">
            &larr; Back to Machines
          </Link>
          <Button onClick={viewInUsageDashboard}>View in Usage dashboard</Button>
        </div>
      </div>

      <section aria-label="Machine stats">
        <div className="tiles">
          <div className="tile">
            <span className="label">Last reported</span>
            <span className="value mono">{formatDateTime(machine.last_seen_at)}</span>
          </div>
          <div className="tile">
            <span className="label">Lifetime cost</span>
            <span className="value accent mono">{formatCost(machine.lifetime_cost)}</span>
          </div>
          <div className="tile">
            <span className="label">Lifetime tokens</span>
            <span className="value mono">{formatInt(machine.lifetime_tokens)}</span>
          </div>
          <div className="tile">
            <span className="label">Collected Paths</span>
            <span className="value mono">{formatInt(machine.collected_paths)}</span>
          </div>
          <div className="tile">
            <span className="label">Collected Actors</span>
            <span className="value mono">{formatInt(machine.collected_actors)}</span>
          </div>
        </div>
      </section>

      <section aria-label="Scan roots">
        <div className="section-head">
          <h2>Scan roots</h2>
          <span className="note">as configured in this machine&rsquo;s own collector config</span>
        </div>
        {scanRoots.length === 0 ? (
          <p className="empty-note">No scan roots reported by this machine.</p>
        ) : (
          <div className="scanroots-list">
            {scanRoots.map((root) => (
              <div className="scanroot-card" key={`${root.install_id}:${root.scan_root_path}`}>
                <div className="scanroot-card-main">
                  <span className="scanroot-card-name">{root.name}</span>
                  <span className="scanroot-card-path mono">{root.scan_root_path}</span>
                </div>
                <span className="scanroot-card-type mono">{root.source_type}</span>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
