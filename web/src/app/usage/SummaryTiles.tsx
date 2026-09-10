// story-1/ticket-14: the three summary tiles (approved mockup) — lifetime
// cost, this-window cost, and lifetime total tokens.
import { formatCost, formatTokensCompact } from "~/lib/format";
import type { UsageSummary, UsageWindow } from "~/lib/usage";
import { WINDOW_LABELS } from "./WindowTabs";

interface SummaryTilesProps {
  window: UsageWindow;
  lifetimeByActor: UsageSummary;
  lifetimeByPath: UsageSummary;
  windowByActor: UsageSummary;
}

export function SummaryTiles({ window, lifetimeByActor, lifetimeByPath, windowByActor }: SummaryTilesProps) {
  return (
    <section aria-label="Summary totals">
      <div className="tiles">
        <div className="tile">
          <span className="label">Lifetime cost</span>
          <span className="value accent mono">{formatCost(lifetimeByActor.totals.cost)}</span>
          <span className="sub">
            across {lifetimeByActor.breakdown.length} actor{lifetimeByActor.breakdown.length === 1 ? "" : "s"},{" "}
            {lifetimeByPath.breakdown.length} path{lifetimeByPath.breakdown.length === 1 ? "" : "s"}
          </span>
        </div>
        <div className="tile">
          <span className="label">This {WINDOW_LABELS[window]}</span>
          <span className="value mono">{formatCost(windowByActor.totals.cost)}</span>
          <span className="sub">{formatTokensCompact(windowByActor.totals.tokens)} tokens</span>
        </div>
        <div className="tile">
          <span className="label">Total tokens</span>
          <span className="value mono">{formatTokensCompact(lifetimeByActor.totals.tokens)}</span>
          <span className="sub">this install, lifetime</span>
        </div>
      </div>
    </section>
  );
}
