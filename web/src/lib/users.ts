// milestone-4: TanStack Query hook for the BFF's user-listing source
// (GET /api/bff/users, _contract/API.md). List only, owner-session only,
// every active user of either role, ordered by handle — mirrors my-task's
// own user.ts router and lib/keys.ts's own minimal shape (one query, no
// mutations: this surface has no create/update/delete, by design).
//
// story-1/ticket-16 deleted this file's only real consumer (the example
// domain module's assignee picker) along with the domain itself —
// nothing in this app currently calls useUsersQuery/assigneeOptions, but
// the endpoint and hook are left in place as a general-purpose
// user-listing utility rather than deleted, since neither is
// domain-specific. Flagged for review, not acted on.
import { useQuery } from "@tanstack/react-query";

import { bffFetch } from "~/lib/api/client";
import type { components } from "~/lib/api/bff-schema.gen";
import type { ComboboxOption } from "~/components/ui/combobox";

export type User = components["schemas"]["User"];

export const usersQueryKey = ["bff", "users"] as const;

/**
 * GET /api/bff/users — every active user, either role, ordered by
 * handle. `enabled` (default true) mirrors my-task's own
 * `NewTaskDialog.tsx` — `usersQuery = api.user.list.useQuery(undefined, {
 * enabled: open })` — so a picker inside a dialog only fetches while
 * that dialog is actually open, not on every page load that happens to
 * render the (closed) dialog's component tree.
 */
export function useUsersQuery(enabled = true) {
  return useQuery({
    queryKey: usersQueryKey,
    queryFn: () => bffFetch<components["schemas"]["UserList"]>("/users").then((r) => r.users),
    enabled,
  });
}

/**
 * The synthetic "no assignee" option value every assignee `<Combobox>` on
 * this surface uses — the literal my-task's own task detail page and
 * `NewTaskDialog` share (`UNASSIGNED = "__unassigned__"`,
 * `~/gits/my-task/src/app/(app)/tasks/[ref]/page.tsx`), kept identical
 * here so a reader who knows one codebase recognizes the other's intent
 * immediately. Radix's `<Select>`/this repo's own `<Combobox>` have no
 * real empty value, so "no assignee" needs one that isn't a real id.
 */
export const UNASSIGNED = "__unassigned__";

/**
 * Builds an assignee `<Combobox>`'s option list: "Unassigned" first, then
 * every active user labeled by handle. The option `value` is the user's
 * **id**, not their handle — a deliberate divergence from my-task's own
 * Combobox (which writes a handle), so the picker's value matches what a
 * write would actually expect, not the display text.
 */
export function assigneeOptions(users: User[]): ComboboxOption[] {
  return [
    { value: UNASSIGNED, label: "Unassigned" },
    ...users.map((u) => ({ value: u.id, label: u.handle })),
  ];
}
