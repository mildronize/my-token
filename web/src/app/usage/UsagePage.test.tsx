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
});
