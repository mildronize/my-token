// story-1/ticket-14: one ranked breakdown panel (by actor / by path / by
// machine), each row showing a proportional cost-share bar. `path`'s own
// display rule (contract's "Console display rules") is applied here to
// the path portion of `key` (story-2/ticket-7: `key` is machine-prefixed
// for group_by=path) — display shortening beyond the BFF's own hostname
// substitution is entirely this component's own job.
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
  isUnknown: boolean;
}

// UNKNOWN_SENTINEL is the collector's own path-attribution final fallback
// value (contract's ticket 10 §9 fallback chain) — never a real project
// path, flagged with its own "unknown" row styling below.
const UNKNOWN_SENTINEL = "(unknown)";

// splitMachinePrefix splits group_by=path's own machine-prefixed key
// (`<machine>:<path>`, story-2/ticket-7 — either `<hostname>:<path>` once
// Service.Summary has substituted it, or the raw `<install_id>:<path>`
// fallback) into its two parts. Defensive fallback to "no prefix found"
// for anything that doesn't contain a colon — shouldn't happen given
// Aggregate's own keyFor(GroupByPath), which always produces exactly one.
function splitMachinePrefix(key: string): { machine: string | null; path: string } {
  const i = key.indexOf(":");
  if (i < 0) return { machine: null, path: key };
  return { machine: key.slice(0, i), path: key.slice(i + 1) };
}

/**
 * Builds each row's display label. `machine` (story-1/ticket-18) shows
 * the API's `key` (already the hostname, not the raw install_id — the
 * BFF's own substitution, not this component's job) as the label, with
 * `raw_key` (the install_id) as the tooltip when present — same
 * "shortened label, full value still reachable via tooltip" pattern as
 * `path` (contract's "machine label" rule). `raw_key` is absent when the
 * BFF never substituted anything (the contract's own no-machines-row
 * fallback case: `key` is already the raw install_id there), so the
 * tooltip falls back to `key` itself rather than showing nothing.
 * `actor` keys are rendered as their raw value, same as the mockup shows.
 *
 * `path` (story-2/ticket-7): `key` is now machine-prefixed
 * (`<hostname-or-install_id>:<path>`, summary.go's keyFor(GroupByPath) +
 * Service.Summary's substitution pass) — two machines' identical raw
 * paths are already two distinct rows by the time they reach this
 * component, so the machine-as-UI-disambiguator behavior story-1's
 * contract described (shortenPaths' own `machine` fallback, appended in
 * parens only on a literal-path collision) is retired here: it's no
 * longer reachable, since the compound key already guarantees
 * uniqueness before shortenPaths ever runs. Only the path portion is
 * passed through shortenPaths (collision-scope basename/walk-up still
 * matters between two *different* real project paths); the machine
 * portion is prepended to the shortened label as-is. The tooltip shows
 * `raw_key ?? key` — the ground-truth `<install_id>:<path>` — same
 * fallback pattern as `machine`.
 */
function toDisplayRows(groupBy: UsageGroupBy, rows: UsageBreakdownRow[]): DisplayRow[] {
  if (groupBy === "machine") {
    return rows.map((r) => ({
      key: r.key,
      label: r.key,
      title: r.raw_key ?? r.key,
      tokens: r.tokens,
      cost: r.cost,
      isUnknown: r.key === UNKNOWN_SENTINEL,
    }));
  }
  if (groupBy !== "path") {
    return rows.map((r) => ({
      key: r.key,
      label: r.key,
      title: r.key,
      tokens: r.tokens,
      cost: r.cost,
      isUnknown: r.key === UNKNOWN_SENTINEL,
    }));
  }

  const parsed = rows.map((r) => splitMachinePrefix(r.key));
  const shortened = shortenPaths(parsed.map((p) => ({ path: p.path })));
  return rows.map((r, i) => ({
    key: r.key,
    label: parsed[i].machine !== null ? `${parsed[i].machine}: ${shortened[i].label}` : shortened[i].label,
    title: r.raw_key ?? r.key,
    tokens: r.tokens,
    cost: r.cost,
    isUnknown: parsed[i].path === UNKNOWN_SENTINEL,
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
          return (
            <div className={`rank-row${row.isUnknown ? " unknown" : ""}`} key={`${row.key}-${i}`}>
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
