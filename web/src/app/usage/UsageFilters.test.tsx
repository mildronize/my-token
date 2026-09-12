// story-2/ticket-11: pure rendering test for the two filter dropdowns —
// same "render + assert options/selection" pattern as
// BreakdownPanel.test.tsx (a mocked-props unit test, not a network test;
// UsagePage.test.tsx covers the wiring into real queries).
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { UsageFilters, machineOptionsFromBreakdown, scanRootOptionsFromList } from "./UsageFilters";
import type { UsageBreakdownRow, UsageScanRoot } from "~/lib/usage";

afterEach(cleanup);

describe("machineOptionsFromBreakdown", () => {
  it("uses raw_key (install_id) as the option value and key (hostname) as the label", () => {
    const rows: UsageBreakdownRow[] = [
      { key: "thw-home", raw_key: "install-a-uuid", tokens: 1, cost: 1, turns: 1 },
    ];
    expect(machineOptionsFromBreakdown(rows)).toEqual([{ value: "install-a-uuid", label: "thw-home" }]);
  });

  it("falls back to key as the value when raw_key is absent (no machines row)", () => {
    const rows: UsageBreakdownRow[] = [{ key: "install-orphan", tokens: 1, cost: 1, turns: 1 }];
    expect(machineOptionsFromBreakdown(rows)).toEqual([{ value: "install-orphan", label: "install-orphan" }]);
  });
});

describe("scanRootOptionsFromList", () => {
  it("builds the composite install_id:scan_root_path value per the contract's machine-prefix convention", () => {
    const rows: UsageScanRoot[] = [
      { install_id: "install-a", hostname: "thw-home", scan_root_path: "~/.claude", name: "main" },
    ];
    const options = scanRootOptionsFromList(rows);
    expect(options).toHaveLength(1);
    expect(options[0].value).toBe("install-a:~/.claude");
    expect(options[0].label).toContain("thw-home");
    expect(options[0].label).toContain("main");
  });
});

describe("UsageFilters", () => {
  it("renders 'All machines'/'All scan roots' plus every option, selected value reflects props", () => {
    render(
      <UsageFilters
        machineOptions={[{ value: "install-a", label: "thw-home" }]}
        scanRootOptions={[{ value: "install-a:~/.claude", label: "thw-home — main" }]}
        selectedMachine="install-a"
        selectedScanRoot={undefined}
        onMachineChange={() => {}}
        onScanRootChange={() => {}}
      />,
    );

    const machineSelect = screen.getByRole("combobox", { name: "Filter by machine" }) as HTMLSelectElement;
    expect(machineSelect.value).toBe("install-a");
    expect(screen.getByRole("option", { name: "thw-home" })).toBeInTheDocument();

    const scanRootSelect = screen.getByRole("combobox", { name: "Filter by scan root" }) as HTMLSelectElement;
    expect(scanRootSelect.value).toBe("");
    expect(screen.getByRole("option", { name: "thw-home — main" })).toBeInTheDocument();
  });

  it("calls onMachineChange(undefined) when 'All machines' is (re)selected", () => {
    const onMachineChange = vi.fn();
    render(
      <UsageFilters
        machineOptions={[{ value: "install-a", label: "thw-home" }]}
        scanRootOptions={[]}
        selectedMachine="install-a"
        selectedScanRoot={undefined}
        onMachineChange={onMachineChange}
        onScanRootChange={() => {}}
      />,
    );

    fireEvent.change(screen.getByRole("combobox", { name: "Filter by machine" }), { target: { value: "" } });
    expect(onMachineChange).toHaveBeenCalledWith(undefined);
  });

  it("calls onScanRootChange with the composite value when a scan root is selected", () => {
    const onScanRootChange = vi.fn();
    render(
      <UsageFilters
        machineOptions={[]}
        scanRootOptions={[{ value: "install-a:~/.claude", label: "thw-home — main" }]}
        selectedMachine={undefined}
        selectedScanRoot={undefined}
        onMachineChange={() => {}}
        onScanRootChange={onScanRootChange}
      />,
    );

    fireEvent.change(screen.getByRole("combobox", { name: "Filter by scan root" }), {
      target: { value: "install-a:~/.claude" },
    });
    expect(onScanRootChange).toHaveBeenCalledWith("install-a:~/.claude");
  });
});
