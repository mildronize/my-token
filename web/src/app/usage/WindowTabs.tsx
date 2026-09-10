// story-1/ticket-14: the top bar's time-window tabs — the fixed five
// windows GET /usage/windows' own table covers (~/lib/usage.ts's
// FIXED_WINDOWS). "lifetime" is deliberately not a tab: it only exists
// as one summary tile's own unbounded range (SummaryTiles.tsx), not a
// window a user picks to re-scope every panel by.
import { FIXED_WINDOWS, type UsageWindow } from "~/lib/usage";

const WINDOW_LABELS: Record<UsageWindow, string> = {
  "5h": "5h",
  "24h": "24h",
  today: "Today",
  week: "Week",
  month: "Month",
  lifetime: "Lifetime",
};

interface WindowTabsProps {
  value: UsageWindow;
  onChange: (window: UsageWindow) => void;
}

export function WindowTabs({ value, onChange }: WindowTabsProps) {
  return (
    <div className="windowtabs" role="tablist" aria-label="Time window">
      {FIXED_WINDOWS.map((w) => (
        <button
          key={w}
          type="button"
          role="tab"
          aria-current={w === value}
          aria-selected={w === value}
          onClick={() => onChange(w)}
        >
          {WINDOW_LABELS[w]}
        </button>
      ))}
    </div>
  );
}

export { WINDOW_LABELS };
