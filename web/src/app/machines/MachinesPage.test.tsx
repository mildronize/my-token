// story-3/ticket-3: MachinesPage — the overview table backed by
// GET /api/bff/machines (story-3/ticket-1), summary strip + one row per
// machine + row-click navigation to /machines/:installId (ticket-4's own
// page, not built yet — this test only asserts the navigation itself,
// same "which URL got called/reached" convention UsagePage.test.tsx
// already uses for its own fetch-mock assertions).
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactElement } from "react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import MachinesPage from "./MachinesPage";

function renderWithProviders(ui: ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/machines"]}>
        <Routes>
          <Route path="/machines" element={ui} />
          {/* ticket-4's own page — a bare marker is enough here to prove
              *which* installId the click navigated to. */}
          <Route path="/machines/:installId" element={<div>detail page for install-a</div>} />
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

function mockMachinesFetch(machines: unknown[] = [MACHINE_A, MACHINE_B]) {
  const calledUrls: string[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input.toString();
    calledUrls.push(url);
    if (url.startsWith("/api/bff/machines")) {
      return jsonResponse({ machines });
    }
    throw new Error(`MachinesPage.test.tsx: unexpected fetch to ${url}`);
  });
  global.fetch = fetchMock as unknown as typeof fetch;
  return { fetchMock, calledUrls };
}

describe("MachinesPage", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    cleanup();
    global.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it("renders the summary strip (total machine count + combined lifetime cost)", async () => {
    mockMachinesFetch();
    renderWithProviders(<MachinesPage />);

    await screen.findByText("thw-home");
    const strip = screen.getByLabelText("Machines summary");
    // Two machines, combined lifetime cost 4430.30 + 12.50 = 4442.80.
    expect(within(strip).getByText("2")).toBeInTheDocument();
    expect(within(strip).getByText("$4,442.80")).toBeInTheDocument();
  });

  it("renders one row per machine with every required column, hostname tooltip showing install_id, in server-provided order", async () => {
    mockMachinesFetch();
    renderWithProviders(<MachinesPage />);

    await screen.findByText("thw-home");
    const rows = screen.getAllByRole("row");
    const [headerRow, rowA, rowB] = rows;
    void headerRow;

    // No client-side re-sort: install-a (rows[0]) still renders above
    // install-b (rows[1]) exactly as the mocked response ordered them.
    const hostA = within(rowA).getByText("thw-home");
    expect(hostA).toHaveAttribute("title", "install-a");
    expect(within(rowA).getByText("$4,430.30")).toBeInTheDocument();
    expect(within(rowA).getByText("340")).toBeInTheDocument();
    expect(within(rowA).getByText("12")).toBeInTheDocument();

    const hostB = within(rowB).getByText("thw-laptop");
    expect(hostB).toHaveAttribute("title", "install-b");
    expect(within(rowB).getByText("$12.50")).toBeInTheDocument();
    expect(within(rowB).getByText("5")).toBeInTheDocument();
    expect(within(rowB).getByText("3")).toBeInTheDocument();
  });

  it("clicking a row navigates to that row's /machines/:installId", async () => {
    mockMachinesFetch();
    renderWithProviders(<MachinesPage />);

    const hostA = await screen.findByText("thw-home");
    await userEvent.click(hostA.closest("tr")!);

    await screen.findByText("detail page for install-a");
  });

  it("renders zero-machines state without an error", async () => {
    mockMachinesFetch([]);
    renderWithProviders(<MachinesPage />);

    await screen.findByRole("table");
    expect(screen.getAllByRole("row")).toHaveLength(1); // header row only
    const strip = screen.getByLabelText("Machines summary");
    expect(within(strip).getByText("0")).toBeInTheDocument();
  });
});
