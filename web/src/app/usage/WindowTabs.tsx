// story-1/ticket-14: the top bar's time-window tabs — the fixed windows
// GET /usage/windows' own table covers (~/lib/usage.ts's FIXED_WINDOWS).
// story-1/ticket-20 reopens ticket 14's "lifetime is deliberately not a
// tab" call: `year` and `lifetime` are now both real, selectable tabs
// like every other window here — clicking either re-scopes the
// actor/path/machine breakdown panels via `/usage/summary?window=...`,
// exactly like the original five.
import { FIXED_WINDOWS, type UsageWindow } from "~/lib/usage";

const WINDOW_LABELS: Record<UsageWindow, string> = {
  "5h": "5h",
  "24h": "24h",
  today: "Today",
  week: "Week",
  month: "Month",
  year: "Year",
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
