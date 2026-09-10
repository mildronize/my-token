// story-1/ticket-18: BreakdownPanel's own "By machine" rendering — the
// component's job (per its own toDisplayRows doc comment) is to show the
// API's `key` (already the hostname, not the raw install_id, once the
// BFF substitutes it — internal/domain/usage/service.go) as the label,
// with `raw_key` (the install_id) reachable via the row's `title`
// tooltip attribute (contract's "machine label" rule: same
// "shortened label, full value still reachable" pattern as `path`).
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { BreakdownPanel } from "./BreakdownPanel";
import type { UsageBreakdownRow } from "~/lib/usage";

afterEach(cleanup);

function row(overrides: Partial<UsageBreakdownRow>): UsageBreakdownRow {
  return { key: "key", tokens: 0, cost: 0, turns: 0, ...overrides };
}

describe("BreakdownPanel — group_by=machine", () => {
  it("shows the hostname as the label and the raw install_id as the tooltip", () => {
    render(
      <BreakdownPanel
        title="By machine"
        groupBy="machine"
        totalsCost={1}
        breakdown={[row({ key: "thw-home", raw_key: "install-a-uuid", cost: 1, tokens: 100, turns: 1 })]}
      />,
    );

    const label = screen.getByText("thw-home");
    expect(label).toBeInTheDocument();
    expect(label).toHaveAttribute("title", "install-a-uuid");
    expect(screen.queryByText("install-a-uuid")).not.toBeInTheDocument();
  });

  it("falls back to the hostname itself as the tooltip when raw_key is absent (no machines row)", () => {
    render(
      <BreakdownPanel
        title="By machine"
        groupBy="machine"
        totalsCost={1}
        breakdown={[row({ key: "install-orphan", cost: 1, tokens: 100, turns: 1 })]}
      />,
    );

    const label = screen.getByText("install-orphan");
    expect(label).toHaveAttribute("title", "install-orphan");
  });
});
