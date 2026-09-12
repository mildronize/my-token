// story-2/ticket-11: persistence for the console's two new filters
// (contract's "Console filters" section) — "a small hook or utility, not
// a new dependency" per the ticket's own text. Deliberately narrow: this
// hook owns only get/set/persist. Reconciling a restored value against
// the current data (contract: "An invalid/stale stored value ... is
// dropped silently on restore ... falls back to 'no filter'") needs the
// loaded machine/scan-root option lists, which only UsagePage.tsx has —
// see its own useEffect calls into `setMachine`/`setScanRoot`.
import { useState } from "react";

const MACHINE_STORAGE_KEY = "my-token.usage-console.filter.machine";
const SCAN_ROOT_STORAGE_KEY = "my-token.usage-console.filter.scanRoot";

function readStored(key: string): string | undefined {
  try {
    return window.localStorage.getItem(key) ?? undefined;
  } catch {
    // localStorage unavailable (private browsing, disabled, quota) — same
    // "no filter" starting point as a first-ever visit.
    return undefined;
  }
}

function writeStored(key: string, value: string | undefined): void {
  try {
    if (value === undefined) {
      window.localStorage.removeItem(key);
    } else {
      window.localStorage.setItem(key, value);
    }
  } catch {
    // Filter selection still works for the rest of this session — it
    // just won't survive a reload. Never let a storage failure surface
    // as a page error.
  }
}

export interface UsageFiltersState {
  machine: string | undefined;
  scanRoot: string | undefined;
  setMachine: (value: string | undefined) => void;
  setScanRoot: (value: string | undefined) => void;
}

export function useUsageFilters(): UsageFiltersState {
  const [machine, setMachineState] = useState<string | undefined>(() => readStored(MACHINE_STORAGE_KEY));
  const [scanRoot, setScanRootState] = useState<string | undefined>(() => readStored(SCAN_ROOT_STORAGE_KEY));

  const setMachine = (value: string | undefined) => {
    setMachineState(value);
    writeStored(MACHINE_STORAGE_KEY, value);
  };
  const setScanRoot = (value: string | undefined) => {
    setScanRootState(value);
    writeStored(SCAN_ROOT_STORAGE_KEY, value);
  };

  return { machine, scanRoot, setMachine, setScanRoot };
}
