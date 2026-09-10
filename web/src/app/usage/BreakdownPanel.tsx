// story-1/ticket-14: one ranked breakdown panel (by actor / by path / by
// machine), each row showing a proportional cost-share bar. `path`'s own
// display rule (contract's "Console display rules") is applied here,
// and only here — the API's `key` is always the raw canonicalized value
// (bff-openapi.yaml's UsageBreakdownRow.key doc comment), display
// shortening is entirely this component's own job.
import { formatCost, formatTokensCompact } from "~/lib/format";
import { shortenPaths } from "~/lib/pathDisplay";
import type { UsageBreakdownRow, UsageGroupBy } from "~/lib/usage";

// TOP_N mirrors the contract's own worked example of "the rendered set"
// ("one panel's top 8 for one window") — the exact cap the path-label
// collision rule is scoped against.
export const BREAKDOWN_TOP_N = 8;

interface BreakdownPanelProps {
  title: string;
  groupBy: UsageGroupBy;
  breakdown: UsageBreakdownRow[];
  totalsCost: number;
}

interface DisplayRow {
  key: string;
  label: string;
  title: string;
  tokens: number;
  cost: number;
}

/**
 * Builds each row's display label. `path` gets the contract's own
 * shortening treatment (basename, collision walk-up, machine fallback).
 * `machine` (story-1/ticket-18) shows the API's `key` (already the
 * hostname, not the raw install_id — the BFF's own substitution, not
 * this component's job) as the label, with `raw_key` (the install_id)
 * as the tooltip when present — same "shortened label, full value still
 * reachable via tooltip" pattern as `path` (contract's "machine label"
 * rule). `raw_key` is absent when the BFF never substituted anything
 * (the contract's own no-machines-row fallback case: `key` is already
 * the raw install_id there), so the tooltip falls back to `key` itself
 * rather than showing nothing. `actor` keys are rendered as their raw
 * value, same as the mockup shows.
 */
function toDisplayRows(groupBy: UsageGroupBy, rows: UsageBreakdownRow[]): DisplayRow[] {
  if (groupBy === "machine") {
    return rows.map((r) => ({ key: r.key, label: r.key, title: r.raw_key ?? r.key, tokens: r.tokens, cost: r.cost }));
  }
  if (groupBy !== "path") {
    return rows.map((r) => ({ key: r.key, label: r.key, title: r.key, tokens: r.tokens, cost: r.cost }));
  }
  // group_by=path's breakdown never carries a `machine` per row (it's
  // aggregated across every machine that reported that literal path
  // string) — shortenPaths' own machine-fallback branch is exercised by
  // its unit tests directly, not reachable through this live wiring
  // today (contract point 4's own note: this story never has a second
  // machine to test cross-machine collision against for real).
  const shortened = shortenPaths(rows.map((r) => ({ path: r.key })));
  return rows.map((r, i) => ({
    key: r.key,
    label: shortened[i].label,
    title: shortened[i].title,
    tokens: r.tokens,
    cost: r.cost,
  }));
}

export function BreakdownPanel({ title, groupBy, breakdown, totalsCost }: BreakdownPanelProps) {
  const top = breakdown.slice(0, BREAKDOWN_TOP_N);
  const displayRows = toDisplayRows(groupBy, top);

  return (
    <div className="panel">
      <h3>{title}</h3>
      {displayRows.length === 0 ? (
        <p className="empty-note">No usage in this window.</p>
      ) : (
        displayRows.map((row, i) => {
          const share = totalsCost > 0 ? Math.max(0, Math.min(100, (row.cost / totalsCost) * 100)) : 0;
          const isUnknown = row.key === "(unknown)";
          return (
            <div className={`rank-row${isUnknown ? " unknown" : ""}`} key={`${row.key}-${i}`}>
              <span className="n">{i + 1}</span>
              <div className="who">
                <span className="name" title={row.title}>
                  {row.label}
                </span>
                <span className="bar">
                  <span style={{ width: `${share}%` }} />
                </span>
              </div>
              <div className="figs">
                <span className="cost">{formatCost(row.cost)}</span>
                <span className="tokens">{formatTokensCompact(row.tokens)} tok</span>
              </div>
            </div>
          );
        })
      )}
    </div>
  );
}
