// story-1/ticket-14: TanStack Query hooks for the console's read surface,
// GET /api/bff/usage/summary and GET /api/bff/usage/windows —
// bff-openapi.yaml's own `snake_case` wire shape (`group_by`,
// `reporting_installs`) is kept verbatim here too, same reasoning that
// file's own comment gives: the contract is this ticket's literal input,
// not something to "fix" into this SPA's usual camelCase convention.
import { useQuery } from "@tanstack/react-query";

import { bffFetch } from "~/lib/api/client";
import type { components } from "~/lib/api/bff-schema.gen";

export type UsageWindow = "5h" | "24h" | "today" | "week" | "month" | "year" | "lifetime";
export type UsageGroupBy = "actor" | "path" | "machine";

// FIXED_WINDOWS is exactly GET /usage/windows' own seven rows, in the
// contract's own order — the console's time-window tabs render this set.
// story-1/ticket-20 grew this from five to seven and made both `year`
// and `lifetime` real, selectable tabs, reopening ticket 14's own
// "lifetime is tab-only-tile, never a tab" call (that call's reasoning —
// a "lifetime" tab makes no sense since every other window already
// answers "since when" relative to now — turned out not to be what was
// wanted; see ticket-20's own report).
export const FIXED_WINDOWS: UsageWindow[] = ["5h", "24h", "today", "week", "month", "year", "lifetime"];

export type UsageTotals = components["schemas"]["UsageTotals"];
export type UsageBreakdownRow = components["schemas"]["UsageBreakdownRow"];
export type UsageSummary = components["schemas"]["UsageSummary"];
export type UsageWindowRow = components["schemas"]["UsageWindowRow"];
export type UsageWindows = components["schemas"]["UsageWindows"];
export type UsageScanRoot = components["schemas"]["UsageScanRoot"];
export type UsageScanRoots = components["schemas"]["UsageScanRootList"];

// story-2/ticket-11: the two new optional console filters (contract's
// "Console filters" section) — both AND-composed with `window` and with
// each other, server-side (ticket 10). `scanRoot` is always the composite
// `<install_id>:<scan_root_path>` string, never a bare path/name (same
// machine-prefix convention as the cross-machine `path` fix, since
// scan-root naming is per-install).
export interface UsageFilterParams {
  machine?: string;
  scanRoot?: string;
}

export const usageSummaryQueryKey = (window: UsageWindow, groupBy: UsageGroupBy, filters: UsageFilterParams = {}) =>
  ["bff", "usage", "summary", window, groupBy, filters.machine ?? null, filters.scanRoot ?? null] as const;

export const usageWindowsQueryKey = (filters: UsageFilterParams = {}) =>
  ["bff", "usage", "windows", filters.machine ?? null, filters.scanRoot ?? null] as const;

export const usageScanRootsQueryKey = ["bff", "usage", "scan-roots"] as const;

// appendFilterParams mutates `search` in place, adding `machine`/
// `scan_root` only when set — the shared bit between getUsageSummary and
// getUsageWindows' own two new optional params (bff-openapi.yaml).
function appendFilterParams(search: URLSearchParams, filters: UsageFilterParams) {
  if (filters.machine !== undefined) search.set("machine", filters.machine);
  if (filters.scanRoot !== undefined) search.set("scan_root", filters.scanRoot);
}

/** GET /api/bff/usage/summary?window=...&group_by=...&machine=...&scan_root=... */
export function useUsageSummaryQuery(window: UsageWindow, groupBy: UsageGroupBy, filters: UsageFilterParams = {}) {
  return useQuery({
    queryKey: usageSummaryQueryKey(window, groupBy, filters),
    queryFn: () => {
      const search = new URLSearchParams({ window, group_by: groupBy });
      appendFilterParams(search, filters);
      return bffFetch<UsageSummary>(`/usage/summary?${search.toString()}`);
    },
  });
}

/** GET /api/bff/usage/windows — the fixed 5h/24h/today/week/month/year/lifetime table. */
export function useUsageWindowsQuery(filters: UsageFilterParams = {}) {
  return useQuery({
    queryKey: usageWindowsQueryKey(filters),
    queryFn: () => {
      const search = new URLSearchParams();
      appendFilterParams(search, filters);
      const query = search.toString();
      return bffFetch<UsageWindows>(`/usage/windows${query ? `?${query}` : ""}`);
    },
  });
}

/**
 * GET /api/bff/usage/scan-roots — story-2/ticket-9: every registered scan
 * root across every reporting install, no params ever (populates the
 * scan-root filter's own dropdown; unaffected by any filter so its option
 * list never collapses once a filter is selected).
 */
export function useUsageScanRootsQuery() {
  return useQuery({
    queryKey: usageScanRootsQueryKey,
    queryFn: () => bffFetch<UsageScanRoots>("/usage/scan-roots"),
  });
}
