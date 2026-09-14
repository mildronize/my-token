// story-3/ticket-3: the Machines page — goal's points 1-4, a
// lifetime-scoped fleet-overview list complementing /usage's own
// time-sliced "By machine" ranking. Reuses usage-console.css's scoped
// class set (topbar/pill/table-wrap/section-head) rather than inventing a
// second stylesheet: this page is /usage's own sibling in framing (goal:
// "same 'at a glance' framing as /usage's own top-bar pill"), and those
// classes are already generic layout, not usage-specific despite the
// file's name (see UsagePage.tsx's own use of the same import).
//
// Data source: GET /api/bff/machines (story-3/ticket-1, ~/lib/machines.ts)
// — one request, already sorted `last_seen_at` descending server-side, so
// this component does no client-side re-sort of its own.
import { useNavigate } from "react-router-dom";

import "~/styles/usage-console.css";
import { ErrorState } from "~/components/ErrorState";
import { Spinner } from "~/components/ui/spinner";
import { useMachinesQuery, type Machine } from "~/lib/machines";
import { formatCost, formatDateTime, formatInt } from "~/lib/format";

export default function MachinesPage() {
  const navigate = useNavigate();
  const machinesQuery = useMachinesQuery();

  if (machinesQuery.isLoading) {
    return (
      <div className="usage-console">
        <div className="flex min-h-[300px] items-center justify-center">
          <Spinner size="lg" />
        </div>
      </div>
    );
  }

  if (machinesQuery.isError || !machinesQuery.data) {
    return (
      <div className="usage-console">
        <ErrorState message="Could not load machines." onRetry={() => void machinesQuery.refetch()} />
      </div>
    );
  }

  const machines = machinesQuery.data.machines;
  const combinedLifetimeCost = machines.reduce((sum, m) => sum + m.lifetime_cost, 0);

  function goToDetail(m: Machine) {
    navigate(`/machines/${m.install_id}`);
  }

  return (
    <div className="usage-console">
      <div className="topbar">
        <div className="id">
          <h1>my-token</h1>
          <span className="tag">machines</span>
        </div>
        {/* goal point 2: total machine count + combined lifetime cost,
            same "at a glance" pill framing /usage's own topbar uses. */}
        <div className="meta" aria-label="Machines summary">
          <span className="pill">
            <span className="dot" />
            <strong>{machines.length}</strong> machine{machines.length === 1 ? "" : "s"}
          </span>
          <span className="pill">
            <strong className="mono">{formatCost(combinedLifetimeCost)}</strong> lifetime cost
          </span>
        </div>
      </div>

      <section aria-label="Every reporting machine">
        <div className="section-head">
          <h2>Fleet</h2>
          <span className="note">sorted by last reported, most recent first</span>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Hostname</th>
                <th>Last reported</th>
                <th className="num">Lifetime cost</th>
                <th className="num">Lifetime tokens</th>
                <th className="num">Collected Paths</th>
                <th className="num">Collected Actors</th>
              </tr>
            </thead>
            <tbody>
              {machines.map((m) => (
                <tr
                  key={m.install_id}
                  onClick={() => goToDetail(m)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      goToDetail(m);
                    }
                  }}
                  tabIndex={0}
                  className="clickable-row"
                >
                  {/* goal point 3: hostname displayed, install_id reachable via tooltip. */}
                  <td title={m.install_id}>{m.hostname}</td>
                  <td className="mono">{formatDateTime(m.last_seen_at)}</td>
                  <td className="num mono">{formatCost(m.lifetime_cost)}</td>
                  <td className="num mono">{formatInt(m.lifetime_tokens)}</td>
                  <td className="num mono">{formatInt(m.collected_paths)}</td>
                  <td className="num mono">{formatInt(m.collected_actors)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
