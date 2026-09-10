// story-1/ticket-14: TanStack Query hook tests for the usage read
// surface — a fetch-mock pattern (asserts the right URL/query string is
// called and the right shape comes back), not a re-test of the server's
// own aggregation logic (covered server-side in internal/domain/usage).
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

import { useUsageSummaryQuery, useUsageWindowsQuery } from "./usage";

function makeWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

function newQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

describe("useUsageSummaryQuery", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    global.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it("GETs /api/bff/usage/summary with window and group_by in the query string", async () => {
    let calledUrl: string | undefined;
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      calledUrl = typeof input === "string" ? input : input.toString();
      return new Response(
        JSON.stringify({
          totals: { tokens: 100, cost: 1.5, turns: 2 },
          breakdown: [{ key: "freya", tokens: 100, cost: 1.5, turns: 2 }],
          reporting_installs: 1,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    });
    global.fetch = fetchMock as unknown as typeof fetch;

    const { result } = renderHook(() => useUsageSummaryQuery("24h", "actor"), {
      wrapper: makeWrapper(newQueryClient()),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(calledUrl).toBe("/api/bff/usage/summary?window=24h&group_by=actor");
    expect(result.current.data?.totals.tokens).toBe(100);
    expect(result.current.data?.reporting_installs).toBe(1);
    expect(result.current.data?.breakdown[0].key).toBe("freya");
  });

  it("passes group_by=path and window=lifetime through unchanged", async () => {
    let calledUrl: string | undefined;
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      calledUrl = typeof input === "string" ? input : input.toString();
      return new Response(
        JSON.stringify({
          totals: { tokens: 0, cost: 0, turns: 0 },
          breakdown: [],
          reporting_installs: 0,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    });
    global.fetch = fetchMock as unknown as typeof fetch;

    const { result } = renderHook(() => useUsageSummaryQuery("lifetime", "path"), {
      wrapper: makeWrapper(newQueryClient()),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(calledUrl).toBe("/api/bff/usage/summary?window=lifetime&group_by=path");
  });
});

describe("useUsageWindowsQuery", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    global.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it("GETs /api/bff/usage/windows and returns the fixed table", async () => {
    let calledUrl: string | undefined;
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      calledUrl = typeof input === "string" ? input : input.toString();
      return new Response(
        JSON.stringify({
          windows: [
            { window: "5h", turns: 1, tokens: 10, cost: 0.1 },
            { window: "24h", turns: 2, tokens: 20, cost: 0.2 },
            { window: "today", turns: 2, tokens: 20, cost: 0.2 },
            { window: "week", turns: 2, tokens: 20, cost: 0.2 },
            { window: "month", turns: 2, tokens: 20, cost: 0.2 },
          ],
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    });
    global.fetch = fetchMock as unknown as typeof fetch;

    const { result } = renderHook(() => useUsageWindowsQuery(), {
      wrapper: makeWrapper(newQueryClient()),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(calledUrl).toBe("/api/bff/usage/windows");
    expect(result.current.data?.windows).toHaveLength(5);
    expect(result.current.data?.windows[0].window).toBe("5h");
  });
});
