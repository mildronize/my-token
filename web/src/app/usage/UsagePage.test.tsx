// story-1/ticket-20: proves `year` and `lifetime` are real, clickable
// tabs (WindowTabs.tsx) that re-scope the actor/path/machine breakdown
// panels' query exactly like the original five (`5h`/`24h`/`today`/
// `week`/`month`) — a real render against a mocked
// `/api/bff/usage/*` fetch, same fetch-mock + userEvent pattern as
// ApiKeySettings.test.tsx (this repo's own established "render + click +
// assert the resulting request" convention; no prior WindowTabs/
// UsagePage test existed to otherwise mirror).
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactElement } from "react";
import userEvent from "@testing-library/user-event";

import UsagePage from "./UsagePage";

function renderWithClient(ui: ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

function emptySummaryBody() {
  return JSON.stringify({
    totals: { tokens: 0, cost: 0, turns: 0 },
    breakdown: [],
    reporting_installs: 0,
  });
}

function windowsTableBody() {
  return JSON.stringify({
    windows: ["5h", "24h", "today", "week", "month", "year", "lifetime"].map((window) => ({
      window,
      turns: 0,
      tokens: 0,
      cost: 0,
    })),
  });
}

function scanRootsBody() {
  return JSON.stringify({ scan_roots: [] });
}

// Every /api/bff/usage/* call this page can make resolves with an empty
// (but well-shaped) body — the point of these tests is which *URLs* got
// called when a tab is clicked, not what the aggregated numbers are
// (that's summary_test.go/service_test.go's job, server-side).
function mockUsageFetch() {
  const calledUrls: string[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input.toString();
    calledUrls.push(url);
    if (url.startsWith("/api/bff/usage/windows")) {
      return new Response(windowsTableBody(), { status: 200, headers: { "Content-Type": "application/json" } });
    }
    if (url.startsWith("/api/bff/usage/summary")) {
      return new Response(emptySummaryBody(), { status: 200, headers: { "Content-Type": "application/json" } });
    }
    // story-2/ticket-11: the scan-root filter's own option list — every
    // load fetches this once, unfiltered.
    if (url.startsWith("/api/bff/usage/scan-roots")) {
      return new Response(scanRootsBody(), { status: 200, headers: { "Content-Type": "application/json" } });
    }
    throw new Error(`UsagePage.test.tsx: unexpected fetch to ${url}`);
  });
  global.fetch = fetchMock as unknown as typeof fetch;
  return { fetchMock, calledUrls };
}

describe("UsagePage — window tabs re-scope the breakdown panels' query", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    cleanup();
    global.fetch = originalFetch;
    vi.restoreAllMocks();
    // story-2/ticket-15: the window tab now persists to localStorage
    // (useUsageFilters) — each test here assumes it starts from the
    // default "today" window, which only holds if the previous test's
    // tab click didn't leak into this one via real jsdom localStorage.
    window.localStorage.clear();
  });

  it("clicking the Year tab is possible and re-fetches every breakdown panel with window=year", async () => {
    const { calledUrls } = mockUsageFetch();
    renderWithClient(<UsagePage />);

    // The default window ("today") must finish loading before the tabs
    // are on screen at all (UsagePage's own isLoading gate).
    const yearTab = await screen.findByRole("tab", { name: "Year" });
    expect(calledUrls).toContain("/api/bff/usage/summary?window=today&group_by=actor");

    await userEvent.click(yearTab);

    // None of these three exact URLs were ever called for the default
    // "today" window, so their presence can only mean the click actually
    // re-scoped all three breakdown panels' own queries to window=year —
    // not just the tab's own visual selected state.
    await waitFor(() => {
      expect(calledUrls).toContain("/api/bff/usage/summary?window=year&group_by=actor");
      expect(calledUrls).toContain("/api/bff/usage/summary?window=year&group_by=path");
      expect(calledUrls).toContain("/api/bff/usage/summary?window=year&group_by=machine");
    });

    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Year" })).toHaveAttribute("aria-selected", "true");
    });
  });

  it("clicking the Lifetime tab is possible and re-fetches the machine breakdown panel with window=lifetime", async () => {
    const { calledUrls } = mockUsageFetch();
    renderWithClient(<UsagePage />);

    const lifetimeTab = await screen.findByRole("tab", { name: "Lifetime" });
    expect(calledUrls).toContain("/api/bff/usage/summary?window=today&group_by=actor");
    // The page's own lifetime summary tiles already fetch window=lifetime
    // for actor/path on initial load (UsagePage.tsx's lifetimeByActor/
    // lifetimeByPath) — group_by=machine is the one dimension that is
    // never fetched with window=lifetime until the tab itself is
    // selected, so it is the one assertion that can only pass if the
    // click genuinely re-scoped the machine breakdown panel's own query,
    // not merely reused an already-cached lifetime query.
    expect(calledUrls).not.toContain("/api/bff/usage/summary?window=lifetime&group_by=machine");

    await userEvent.click(lifetimeTab);

    await waitFor(() => {
      expect(calledUrls).toContain("/api/bff/usage/summary?window=lifetime&group_by=machine");
    });

    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Lifetime" })).toHaveAttribute("aria-selected", "true");
    });
  });

  // story-2/ticket-15: มายด์'s own report — "when i mean filter, it also
  // mean 5h/24h/Today/Week/Month/Year/Lifetime too." The window tab is
  // now persisted the same way machine/scan-root already are.
  it("a selected window tab survives a remount — restored from localStorage, applied to the very first fetch", async () => {
    mockUsageFetch();
    const { unmount } = renderWithClient(<UsagePage />);

    const yearTab = await screen.findByRole("tab", { name: "Year" });
    await userEvent.click(yearTab);
    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Year" })).toHaveAttribute("aria-selected", "true");
    });

    unmount();
    cleanup();

    const { calledUrls } = mockUsageFetch();
    renderWithClient(<UsagePage />);

    // The remounted page's very first request already carries the
    // persisted window — proving it came from localStorage, not the
    // "today" default, and not a user re-clicking the tab.
    await waitFor(() => {
      expect(calledUrls).toContain("/api/bff/usage/summary?window=year&group_by=actor");
    });
    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Year" })).toHaveAttribute("aria-selected", "true");
    });
  });

  it("a stored window value that isn't a real fixed window falls back to the default 'today'", async () => {
    window.localStorage.setItem("my-token.usage-console.filter.window", "not-a-real-window");
    const { calledUrls } = mockUsageFetch();
    renderWithClient(<UsagePage />);

    await waitFor(() => {
      expect(calledUrls).toContain("/api/bff/usage/summary?window=today&group_by=actor");
    });
    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Today" })).toHaveAttribute("aria-selected", "true");
    });
  });
});

// story-2/ticket-11: the console's two new filters (contract's "Console
// filters" section). A single mock capable of responding differently per
// group_by/machine/scan_root combination, so a test can assert both which
// URLs got re-fetched AND that the rendered rows actually changed — not
// just that a request happened.
function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
}

const UNFILTERED_MACHINE_ROW = { key: "thw-home", raw_key: "install-a", tokens: 100, cost: 1, turns: 2 };
const UNFILTERED_ACTOR_ROW = { key: "freya", tokens: 100, cost: 1, turns: 2 };
const UNFILTERED_PATH_ROW = { key: "thw-home:/home/thw-home/gits/my-task", raw_key: "install-a:/home/thw-home/gits/my-task", tokens: 100, cost: 1, turns: 2 };

const NARROWED_ACTOR_ROW = { key: "narrowed-actor", tokens: 5, cost: 0.5, turns: 1 };
const NARROWED_PATH_ROW = { key: "thw-home:/home/narrowed-project", raw_key: "install-a:/home/narrowed-project", tokens: 5, cost: 0.5, turns: 1 };

function mockFilterableUsageFetch(options: { scanRoots?: unknown[] } = {}) {
  const calledUrls: string[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const raw = typeof input === "string" ? input : input.toString();
    calledUrls.push(raw);
    const url = new URL(raw, "http://localhost");

    if (url.pathname === "/api/bff/usage/windows") {
      return jsonResponse({
        windows: ["5h", "24h", "today", "week", "month", "year", "lifetime"].map((window) => ({
          window,
          turns: 0,
          tokens: 0,
          cost: 0,
        })),
      });
    }
    if (url.pathname === "/api/bff/usage/scan-roots") {
      return jsonResponse({ scan_roots: options.scanRoots ?? [] });
    }
    if (url.pathname === "/api/bff/usage/summary") {
      const groupBy = url.searchParams.get("group_by");
      const machine = url.searchParams.get("machine");
      if (groupBy === "machine") {
        // Deliberately unaffected by `machine` itself (see UsagePage.tsx's
        // own comment) — this response also backs the filter dropdown's
        // own option list, which must stay complete once a machine is
        // selected, not collapse to just the selected row.
        return jsonResponse({ totals: { tokens: 100, cost: 1, turns: 2 }, breakdown: [UNFILTERED_MACHINE_ROW], reporting_installs: 1 });
      }
      if (groupBy === "actor") {
        const rows = machine ? [NARROWED_ACTOR_ROW] : [UNFILTERED_ACTOR_ROW];
        return jsonResponse({ totals: { tokens: rows[0].tokens, cost: rows[0].cost, turns: rows[0].turns }, breakdown: rows, reporting_installs: 1 });
      }
      if (groupBy === "path") {
        const rows = machine ? [NARROWED_PATH_ROW] : [UNFILTERED_PATH_ROW];
        return jsonResponse({ totals: { tokens: rows[0].tokens, cost: rows[0].cost, turns: rows[0].turns }, breakdown: rows, reporting_installs: 1 });
      }
    }
    throw new Error(`UsagePage.test.tsx: unexpected fetch to ${raw}`);
  });
  global.fetch = fetchMock as unknown as typeof fetch;
  return { fetchMock, calledUrls };
}

describe("UsagePage — machine/scan-root filters (story-2/ticket-11)", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    cleanup();
    global.fetch = originalFetch;
    window.localStorage.clear();
    vi.restoreAllMocks();
  });

  it("selecting a machine filter narrows all three breakdown panels' rendered rows, not just 'By machine'", async () => {
    const { calledUrls } = mockFilterableUsageFetch();
    renderWithClient(<UsagePage />);

    // Unfiltered state first: "By actor" shows the unfiltered row.
    await screen.findByText("freya");
    expect(screen.getByTitle("install-a:/home/thw-home/gits/my-task")).toBeInTheDocument();

    const machineSelect = await screen.findByRole("combobox", { name: "Filter by machine" });
    await userEvent.selectOptions(machineSelect, "install-a");

    // "By actor" and "By path" — the two panels that aren't "By
    // machine" itself — both re-fetched with machine=install-a.
    await waitFor(() => {
      expect(calledUrls.some((u) => u.includes("group_by=actor") && u.includes("machine=install-a"))).toBe(true);
      expect(calledUrls.some((u) => u.includes("group_by=path") && u.includes("machine=install-a"))).toBe(true);
    });

    // And their rendered rows actually changed to the narrowed data —
    // not just that a request was fired.
    await screen.findByText("narrowed-actor");
    expect(screen.queryByText("freya")).not.toBeInTheDocument();
    expect(screen.getByTitle("install-a:/home/narrowed-project")).toBeInTheDocument();
    expect(screen.queryByTitle("install-a:/home/thw-home/gits/my-task")).not.toBeInTheDocument();
  });

  it("a selected machine filter survives a remount — restored from localStorage, applied to the very first fetch", async () => {
    const first = mockFilterableUsageFetch();
    const { unmount } = renderWithClient(<UsagePage />);

    const machineSelect = await screen.findByRole("combobox", { name: "Filter by machine" });
    await userEvent.selectOptions(machineSelect, "install-a");
    await screen.findByText("narrowed-actor");

    unmount();
    cleanup();

    const second = mockFilterableUsageFetch();
    renderWithClient(<UsagePage />);

    // The remounted page's very first "By actor" request already carries
    // the persisted filter — proving it came from localStorage, not a
    // user re-selecting it.
    await waitFor(() => {
      expect(second.calledUrls.some((u) => u.includes("group_by=actor") && u.includes("machine=install-a"))).toBe(true);
    });
    await screen.findByText("narrowed-actor");

    void first;
  });

  it("a stored machine value not present in the current data falls back to 'no filter' without an error state", async () => {
    window.localStorage.setItem("my-token.usage-console.filter.machine", "install-stale-and-gone");
    const { calledUrls } = mockFilterableUsageFetch();
    renderWithClient(<UsagePage />);

    // The very first render optimistically includes the stale value —
    // reconciliation only happens once the machine options list has
    // actually loaded.
    await screen.findByRole("combobox", { name: "Filter by machine" });

    // Once the (unfiltered-by-machine) options list loads and doesn't
    // contain the stale install_id, the filter silently resets: the
    // dropdown falls back to "All machines" and every panel re-fetches
    // unfiltered — never an error state.
    await waitFor(() => {
      const select = screen.getByRole("combobox", { name: "Filter by machine" }) as HTMLSelectElement;
      expect(select.value).toBe("");
    });
    await screen.findByText("freya");
    expect(screen.queryByText("Could not load usage data.")).not.toBeInTheDocument();
    expect(screen.queryByText("narrowed-actor")).not.toBeInTheDocument();
    expect(window.localStorage.getItem("my-token.usage-console.filter.machine")).toBeNull();

    void calledUrls;
  });

  it("a stored scan-root value not present in the current data falls back to 'no filter' without an error state", async () => {
    window.localStorage.setItem("my-token.usage-console.filter.scanRoot", "install-stale:~/.gone");
    // A real, distinct scan root exists — it's simply not the stale one —
    // so reconciliation has something concrete to compare against.
    mockFilterableUsageFetch({
      scanRoots: [{ install_id: "install-a", hostname: "thw-home", scan_root_path: "~/.claude", name: "main" }],
    });
    renderWithClient(<UsagePage />);

    await screen.findByRole("combobox", { name: "Filter by scan root" });

    // Once the scan-roots list loads and doesn't contain the stale
    // composite value, the filter silently resets: the dropdown falls
    // back to "All scan roots" — never an error state.
    await waitFor(() => {
      const select = screen.getByRole("combobox", { name: "Filter by scan root" }) as HTMLSelectElement;
      expect(select.value).toBe("");
    });
    await screen.findByText("freya");
    expect(screen.queryByText("Could not load usage data.")).not.toBeInTheDocument();
    expect(window.localStorage.getItem("my-token.usage-console.filter.scanRoot")).toBeNull();
  });
});
