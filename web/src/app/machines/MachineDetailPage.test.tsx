// story-3/ticket-4: MachineDetailPage — the identity/stats block + scan
// roots section (found by `installId` route param from the same
// GET /api/bff/machines response MachinesPage.tsx already renders, no
// second single-machine endpoint per the contract), plus the "View in
// Usage dashboard" button's own cross-page contract with UsagePage
// (silent glue via a shared localStorage key — worth a direct test rather
// than trusting it works, same reasoning UsagePage.test.tsx's own
// "a selected machine filter survives a remount" test gives, just
// triggered from this page instead of the dropdown).
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactElement } from "react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import MachineDetailPage from "./MachineDetailPage";
import UsagePage from "~/app/usage/UsagePage";

function renderWithProviders(ui: ReactElement, initialPath: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/machines/:installId" element={ui} />
          <Route path="/machines" element={<div>machines overview page</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
}

const MACHINE_A = {
  install_id: "install-a",
  hostname: "thw-home",
  last_seen_at: "2026-09-13T13:31:00Z",
  lifetime_cost: 4430.3,
  lifetime_tokens: 89_200_000,
  collected_paths: 340,
  collected_actors: 12,
};

const MACHINE_B = {
  install_id: "install-b",
  hostname: "thw-laptop",
  last_seen_at: "2026-09-01T09:00:00Z",
  lifetime_cost: 12.5,
  lifetime_tokens: 1000,
  collected_paths: 5,
  collected_actors: 3,
};

const SCAN_ROOT_A1 = {
  install_id: "install-a",
  hostname: "thw-home",
  scan_root_path: "~/.claude",
  name: "main",
  source_type: "claude_code",
};

const SCAN_ROOT_A2 = {
  install_id: "install-a",
  hostname: "thw-home",
  scan_root_path: "~/work/other-tool",
  name: "side project",
  source_type: "codex",
};

// A scan root belonging to a *different* install — proves the section
// filters client-side to this page's own installId rather than rendering
// every row the endpoint returns.
const SCAN_ROOT_B1 = {
  install_id: "install-b",
  hostname: "thw-laptop",
  scan_root_path: "~/.claude",
  name: "main",
  source_type: "claude_code",
};

function mockDetailFetch(options: { machines?: unknown[]; scanRoots?: unknown[] } = {}) {
  const calledUrls: string[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const raw = typeof input === "string" ? input : input.toString();
    calledUrls.push(raw);
    const url = new URL(raw, "http://localhost");
    if (url.pathname === "/api/bff/machines") {
      return jsonResponse({ machines: options.machines ?? [MACHINE_A, MACHINE_B] });
    }
    if (url.pathname === "/api/bff/usage/scan-roots") {
      return jsonResponse({ scan_roots: options.scanRoots ?? [SCAN_ROOT_A1, SCAN_ROOT_A2, SCAN_ROOT_B1] });
    }
    throw new Error(`MachineDetailPage.test.tsx: unexpected fetch to ${raw}`);
  });
  global.fetch = fetchMock as unknown as typeof fetch;
  return { fetchMock, calledUrls };
}

describe("MachineDetailPage", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    cleanup();
    global.fetch = originalFetch;
    window.localStorage.clear();
    vi.restoreAllMocks();
  });

  it("renders the identity/stats block for the machine matching the installId route param", async () => {
    mockDetailFetch();
    renderWithProviders(<MachineDetailPage />, "/machines/install-a");

    await screen.findByText("thw-home");
    expect(screen.getByText("install-a")).toBeInTheDocument();
    expect(screen.getByText("$4,430.30")).toBeInTheDocument();
    expect(screen.getByText("89,200,000")).toBeInTheDocument();
    expect(screen.getByText("340")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();

    // Not install-b's own data.
    expect(screen.queryByText("thw-laptop")).not.toBeInTheDocument();
    expect(screen.queryByText("$12.50")).not.toBeInTheDocument();
  });

  it("renders the scan-roots section, client-filtered to this installId, with each root's name/path/source_type", async () => {
    mockDetailFetch();
    renderWithProviders(<MachineDetailPage />, "/machines/install-a");

    await screen.findByText("thw-home");
    const section = screen.getByLabelText("Scan roots");

    expect(within(section).getByText("main")).toBeInTheDocument();
    expect(within(section).getByText("~/.claude")).toBeInTheDocument();
    expect(within(section).getByText("claude_code")).toBeInTheDocument();

    expect(within(section).getByText("side project")).toBeInTheDocument();
    expect(within(section).getByText("~/work/other-tool")).toBeInTheDocument();
    expect(within(section).getByText("codex")).toBeInTheDocument();

    // install-b's own scan root never renders on install-a's page.
    expect(within(section).queryAllByText("main")).toHaveLength(1);
  });

  it("renders a not-found state for an installId absent from the machines list, without an error state", async () => {
    mockDetailFetch();
    renderWithProviders(<MachineDetailPage />, "/machines/install-does-not-exist");

    await screen.findByText(/No machine found/);
    expect(screen.queryByText("Could not load this machine.")).not.toBeInTheDocument();
  });

  it("renders zero-scan-roots state without an error", async () => {
    mockDetailFetch({ scanRoots: [] });
    renderWithProviders(<MachineDetailPage />, "/machines/install-a");

    await screen.findByText("thw-home");
    expect(screen.getByText("No scan roots reported by this machine.")).toBeInTheDocument();
  });
});

// ---- cross-page contract: MachineDetailPage's button -> UsagePage's own
// localStorage-restored machine filter (contract's Testing Decisions,
// mirroring UsagePage.test.tsx's "a selected machine filter survives a
// remount" test but triggered from this page's button instead of the
// dropdown).
function emptySummaryBody() {
  return JSON.stringify({ totals: { tokens: 0, cost: 0, turns: 0 }, breakdown: [], reporting_installs: 0 });
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

function mockUsageConsoleFetch() {
  const calledUrls: string[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const raw = typeof input === "string" ? input : input.toString();
    calledUrls.push(raw);
    const url = new URL(raw, "http://localhost");
    if (url.pathname === "/api/bff/usage/windows") {
      return jsonResponse(JSON.parse(windowsTableBody()));
    }
    if (url.pathname === "/api/bff/usage/scan-roots") {
      return jsonResponse({ scan_roots: [] });
    }
    if (url.pathname === "/api/bff/usage/summary") {
      return jsonResponse(JSON.parse(emptySummaryBody()));
    }
    throw new Error(`MachineDetailPage.test.tsx (cross-page): unexpected fetch to ${raw}`);
  });
  global.fetch = fetchMock as unknown as typeof fetch;
  return { fetchMock, calledUrls };
}

describe("MachineDetailPage -> UsagePage cross-page contract (story-3/ticket-4)", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    cleanup();
    global.fetch = originalFetch;
    window.localStorage.clear();
    vi.restoreAllMocks();
  });

  it("clicking 'View in Usage dashboard', then rendering UsagePage fresh, carries machine=<installId> on its very first fetch", async () => {
    mockDetailFetch();
    renderWithProviders(<MachineDetailPage />, "/machines/install-a");

    const button = await screen.findByRole("button", { name: "View in Usage dashboard" });
    await userEvent.click(button);

    // The button navigates within its own <Routes> tree to a stub
    // "/machines" element — the assertion that matters is what got
    // written to localStorage, not this test app's own navigation, which
    // a real render of UsagePage below exercises for real.
    expect(window.localStorage.getItem("my-token.usage-console.filter.machine")).toBe("install-a");

    cleanup();

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { calledUrls } = mockUsageConsoleFetch();
    render(
      <QueryClientProvider client={queryClient}>
        <UsagePage />
      </QueryClientProvider>,
    );

    // UsagePage's own lazy-initializer picks the preset value back up on
    // mount exactly as it already does for a reload — its very first "By
    // actor" fetch already carries the filter, before any user
    // interaction with this fresh render.
    await waitFor(() => {
      expect(calledUrls.some((u) => u.includes("group_by=actor") && u.includes("machine=install-a"))).toBe(true);
    });
  });
});
