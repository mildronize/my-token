// story-1/ticket-14: the fixed 5h/24h/today/week/month table, backed by
// GET /api/bff/usage/windows.
import { formatCost, formatInt } from "~/lib/format";
import type { UsageWindowRow } from "~/lib/usage";
import { WINDOW_LABELS } from "./WindowTabs";

interface WindowsTableProps {
  rows: UsageWindowRow[];
}

export function WindowsTable({ rows }: WindowsTableProps) {
  return (
    <section aria-label="Usage by time window">
      <div className="section-head">
        <h2>By time window</h2>
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Window</th>
              <th className="num">Turns</th>
              <th className="num">Tokens</th>
              <th className="num">Cost</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.window}>
                <td className="window">{WINDOW_LABELS[row.window]}</td>
                <td className="num mono">{formatInt(row.turns)}</td>
                <td className="num mono">{formatInt(row.tokens)}</td>
                <td className="num cost mono">{formatCost(row.cost)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
