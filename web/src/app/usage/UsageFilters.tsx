// story-2/ticket-11: the console's two new filter controls (machine,
// scan-root) — contract's "Console filters" section. Plain native
// <select> elements, matching WindowTabs.tsx's own choice of plain DOM
// controls over this repo's Radix-based ui/select.tsx and ui/combobox.tsx
// (neither is wired into any real screen yet) — introducing a
// Portal-based listbox onto this hand-styled usage-console.css screen
// would add real test complexity (async open/close, jsdom pointer-capture
// stubs) for no benefit here.
import type { UsageBreakdownRow, UsageScanRoot } from "~/lib/usage";

export interface FilterOption {
  value: string;
  label: string;
}

// machineOptionsFromBreakdown mirrors BreakdownPanel.tsx's own "By
// machine" display rule (raw_key — the install_id — is the value the BFF
// filter actually takes; key — the hostname, or the raw install_id when
// no `machines` row exists — is what a human reads).
export function machineOptionsFromBreakdown(rows: UsageBreakdownRow[]): FilterOption[] {
  return rows.map((r) => ({ value: r.raw_key ?? r.key, label: r.key }));
}

// scanRootOptionsFromList builds the composite `<install_id>:<scan_root_path>`
// value the contract's `scan_root` query param requires (same
// machine-prefix convention as the cross-machine `path` fix — a bare
// scan-root path/name is ambiguous across machines since naming is
// per-install).
export function scanRootOptionsFromList(rows: UsageScanRoot[]): FilterOption[] {
  return rows.map((r) => ({
    value: `${r.install_id}:${r.scan_root_path}`,
    label: `${r.hostname} — ${r.name}`,
  }));
}

const ALL_VALUE = "";

interface UsageFiltersProps {
  machineOptions: FilterOption[];
  scanRootOptions: FilterOption[];
  selectedMachine: string | undefined;
  selectedScanRoot: string | undefined;
  onMachineChange: (value: string | undefined) => void;
  onScanRootChange: (value: string | undefined) => void;
}

export function UsageFilters({
  machineOptions,
  scanRootOptions,
  selectedMachine,
  selectedScanRoot,
  onMachineChange,
  onScanRootChange,
}: UsageFiltersProps) {
  return (
    <div className="usagefilters">
      <label className="usagefilters-field">
        <span className="usagefilters-label">Machine</span>
        <select
          aria-label="Filter by machine"
          value={selectedMachine ?? ALL_VALUE}
          onChange={(e) => onMachineChange(e.target.value === ALL_VALUE ? undefined : e.target.value)}
        >
          <option value={ALL_VALUE}>All machines</option>
          {machineOptions.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </label>
      <label className="usagefilters-field">
        <span className="usagefilters-label">Scan root</span>
        <select
          aria-label="Filter by scan root"
          value={selectedScanRoot ?? ALL_VALUE}
          onChange={(e) => onScanRootChange(e.target.value === ALL_VALUE ? undefined : e.target.value)}
        >
          <option value={ALL_VALUE}>All scan roots</option>
          {scanRootOptions.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </label>
    </div>
  );
}
