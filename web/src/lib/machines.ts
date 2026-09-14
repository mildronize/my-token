// story-3/ticket-3: TanStack Query hook for the Machines page's own read
// surface, GET /api/bff/machines (story-3/ticket-1) — same thin-wrapper
// convention as ~/lib/usage.ts's hooks (bffFetch + a query key + the
// openapi-typescript-generated response type, no client-side re-sort
// since the endpoint already returns last_seen_at descending).
import { useQuery } from "@tanstack/react-query";

import { bffFetch } from "~/lib/api/client";
import type { components } from "~/lib/api/bff-schema.gen";

export type Machine = components["schemas"]["Machine"];
export type MachineList = components["schemas"]["MachineList"];

export const machinesQueryKey = ["bff", "machines"] as const;

/**
 * GET /api/bff/machines — every machine's lifetime summary, sorted
 * `last_seen_at` descending (server-side; the Machines overview table
 * and the machine detail page both consume this same response).
 */
export function useMachinesQuery() {
  return useQuery({
    queryKey: machinesQueryKey,
    queryFn: () => bffFetch<MachineList>("/machines"),
  });
}
