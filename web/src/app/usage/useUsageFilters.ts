// story-2/ticket-11: persistence for the console's two new filters
// (contract's "Console filters" section) — "a small hook or utility, not
// a new dependency" per the ticket's own text. Deliberately narrow: this
// hook owns only get/set/persist. Reconciling a restored value against
// the current data (contract: "An invalid/stale stored value ... is
// dropped silently on restore ... falls back to 'no filter'") needs the
// loaded machine/scan-root option lists, which only UsagePage.tsx has —
// see its own useEffect calls into `setMachine`/`setScanRoot`.
//
// story-2/ticket-15: the window tab (5h/24h/Today/Week/Month/Year/
// Lifetime) is also "a filter" from มายด์'s own point of view — the
// contract originally scoped persistence to just machine/scan-root and
// left the window tab always resetting to "today", a call มายด์ approved
// at planning time but that turned out not to match what he actually
// wanted once he tested it live. Added here rather than left as a
// separate concern, since it's the exact same get/set/persist shape.
import { useState } from "react";

import { FIXED_WINDOWS, type UsageWindow } from "~/lib/usage";

const MACHINE_STORAGE_KEY = "my-token.usage-console.filter.machine";
const SCAN_ROOT_STORAGE_KEY = "my-token.usage-console.filter.scanRoot";
const WINDOW_STORAGE_KEY = "my-token.usage-console.filter.window";
const DEFAULT_WINDOW: UsageWindow = "today";

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

// readStoredWindow validates the stored value against FIXED_WINDOWS
// itself (a fixed, static enum — unlike machine/scan-root, this never
// needs to wait on server data to know whether a stored value is still
// valid). Covers both "nothing stored yet" and "a stale/corrupted value"
// with the same fallback, same "silently drop to the default" rule the
// contract already sets for the other two filters.
function readStoredWindow(): UsageWindow {
  const raw = readStored(WINDOW_STORAGE_KEY);
  if (raw !== undefined && (FIXED_WINDOWS as string[]).includes(raw)) return raw as UsageWindow;
  return DEFAULT_WINDOW;
}

// story-3/ticket-4: the "View in Usage dashboard" button's own mechanism
// (contract's Frontend section) — a small wrapping helper around
// writeStored, same try/catch-swallow convention as every other write in
// this file, rather than exporting writeStored itself: MachineDetailPage
// only ever needs to *set* this one key, never read or clear it, so the
// narrower helper is the honest seam.
export function presetMachineFilter(installId: string): void {
  writeStored(MACHINE_STORAGE_KEY, installId);
}

export interface UsageFiltersState {
  machine: string | undefined;
  scanRoot: string | undefined;
  window: UsageWindow;
  setMachine: (value: string | undefined) => void;
  setScanRoot: (value: string | undefined) => void;
  setWindow: (value: UsageWindow) => void;
}

export function useUsageFilters(): UsageFiltersState {
  const [machine, setMachineState] = useState<string | undefined>(() => readStored(MACHINE_STORAGE_KEY));
  const [scanRoot, setScanRootState] = useState<string | undefined>(() => readStored(SCAN_ROOT_STORAGE_KEY));
  const [windowValue, setWindowState] = useState<UsageWindow>(readStoredWindow);

  const setMachine = (value: string | undefined) => {
    setMachineState(value);
    writeStored(MACHINE_STORAGE_KEY, value);
  };
  const setScanRoot = (value: string | undefined) => {
    setScanRootState(value);
    writeStored(SCAN_ROOT_STORAGE_KEY, value);
  };
  const setWindow = (value: UsageWindow) => {
    setWindowState(value);
    writeStored(WINDOW_STORAGE_KEY, value);
  };

  return { machine, scanRoot, window: windowValue, setMachine, setScanRoot, setWindow };
}
