// story-1/ticket-14: TanStack Query hook tests for the usage read
// surface — a fetch-mock pattern (asserts the right URL/query string is
// called and the right shape comes back), not a re-test of the server's
// own aggregation logic (covered server-side in internal/domain/usage).
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

import { useUsageSummaryQuery, useUsageWindowsQuery, useUsageScanRootsQuery } from "./usage";

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

  it("passes group_by=actor and window=year through unchanged", async () => {
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

    const { result } = renderHook(() => useUsageSummaryQuery("year", "actor"), {
      wrapper: makeWrapper(newQueryClient()),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(calledUrl).toBe("/api/bff/usage/summary?window=year&group_by=actor");
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

  // story-2/ticket-11: the query-building layer grows optional
  // machine/scanRoot params (contract's "Console filters" section) —
  // included in the URL only when set, and included in the cache key
  // (asserted below) so a filter change triggers a genuine refetch
  // rather than serving a previous filter's cached response.
  it("includes machine= in the query string when a machine filter is set", async () => {
    let calledUrl: string | undefined;
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      calledUrl = typeof input === "string" ? input : input.toString();
      return new Response(
        JSON.stringify({ totals: { tokens: 0, cost: 0, turns: 0 }, breakdown: [], reporting_installs: 0 }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    });
    global.fetch = fetchMock as unknown as typeof fetch;

    const { result } = renderHook(() => useUsageSummaryQuery("today", "actor", { machine: "install-a" }), {
      wrapper: makeWrapper(newQueryClient()),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(calledUrl).toBe("/api/bff/usage/summary?window=today&group_by=actor&machine=install-a");
  });

  it("includes both machine= and scan_root= when both filters are set, AND-composed", async () => {
    let calledUrl: string | undefined;
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      calledUrl = typeof input === "string" ? input : input.toString();
      return new Response(
        JSON.stringify({ totals: { tokens: 0, cost: 0, turns: 0 }, breakdown: [], reporting_installs: 0 }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    });
    global.fetch = fetchMock as unknown as typeof fetch;

    const { result } = renderHook(
      () => useUsageSummaryQuery("today", "path", { machine: "install-a", scanRoot: "install-a:~/.claude" }),
      { wrapper: makeWrapper(newQueryClient()) },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const [path, query] = (calledUrl ?? "").split("?");
    expect(path).toBe("/api/bff/usage/summary");
    const params = new URLSearchParams(query);
    expect(params.get("window")).toBe("today");
    expect(params.get("group_by")).toBe("path");
    expect(params.get("machine")).toBe("install-a");
    expect(params.get("scan_root")).toBe("install-a:~/.claude");
  });

  it("a filter change produces a different cache key, not a stale-cache hit", async () => {
    let callCount = 0;
    const fetchMock = vi.fn(async () => {
      callCount += 1;
      return new Response(
        JSON.stringify({ totals: { tokens: 0, cost: 0, turns: 0 }, breakdown: [], reporting_installs: 0 }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    });
    global.fetch = fetchMock as unknown as typeof fetch;
    const queryClient = newQueryClient();

    const { result, rerender } = renderHook(
      ({ machine }: { machine: string | undefined }) => useUsageSummaryQuery("today", "actor", { machine }),
      { wrapper: makeWrapper(queryClient), initialProps: { machine: undefined as string | undefined } },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(callCount).toBe(1);

    rerender({ machine: "install-a" });
    await waitFor(() => expect(callCount).toBe(2));
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
            { window: "year", turns: 2, tokens: 20, cost: 0.2 },
            { window: "lifetime", turns: 2, tokens: 20, cost: 0.2 },
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
    expect(result.current.data?.windows).toHaveLength(7);
    expect(result.current.data?.windows[0].window).toBe("5h");
    expect(result.current.data?.windows[5].window).toBe("year");
    expect(result.current.data?.windows[6].window).toBe("lifetime");
  });

  it("includes machine=/scan_root= in the query string when filters are set", async () => {
    let calledUrl: string | undefined;
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      calledUrl = typeof input === "string" ? input : input.toString();
      return new Response(JSON.stringify({ windows: [] }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    });
    global.fetch = fetchMock as unknown as typeof fetch;

    const { result } = renderHook(() => useUsageWindowsQuery({ machine: "install-a", scanRoot: "install-a:~/.claude" }), {
      wrapper: makeWrapper(newQueryClient()),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const [path, query] = (calledUrl ?? "").split("?");
    expect(path).toBe("/api/bff/usage/windows");
    const params = new URLSearchParams(query);
    expect(params.get("machine")).toBe("install-a");
    expect(params.get("scan_root")).toBe("install-a:~/.claude");
  });

  it("omits both params (bare URL, same as before) when no filters are set", async () => {
    let calledUrl: string | undefined;
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      calledUrl = typeof input === "string" ? input : input.toString();
      return new Response(JSON.stringify({ windows: [] }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    });
    global.fetch = fetchMock as unknown as typeof fetch;

    const { result } = renderHook(() => useUsageWindowsQuery({}), {
      wrapper: makeWrapper(newQueryClient()),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(calledUrl).toBe("/api/bff/usage/windows");
  });
});

describe("useUsageScanRootsQuery", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    global.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it("GETs /api/bff/usage/scan-roots and returns the list, no params ever", async () => {
    let calledUrl: string | undefined;
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      calledUrl = typeof input === "string" ? input : input.toString();
      return new Response(
        JSON.stringify({
          scan_roots: [
            { install_id: "install-a", hostname: "thw-home", scan_root_path: "~/.claude", name: "main" },
          ],
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    });
    global.fetch = fetchMock as unknown as typeof fetch;

    const { result } = renderHook(() => useUsageScanRootsQuery(), {
      wrapper: makeWrapper(newQueryClient()),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(calledUrl).toBe("/api/bff/usage/scan-roots");
    expect(result.current.data?.scan_roots).toHaveLength(1);
    expect(result.current.data?.scan_roots[0].name).toBe("main");
  });
});
