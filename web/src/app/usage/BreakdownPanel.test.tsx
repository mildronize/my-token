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

// story-2/ticket-14: group_by=actor's key is now the session's raw
// launch cwd (ticket 7 removed the `.typ-crews/<name>` regex), so it
// gets the same basename+tooltip shortening as `path` — a bare crew
// name (pre-story-2 data) is already its own basename, so this is a
// pure additive fix with no visible change for old rows.
describe("BreakdownPanel — group_by=actor", () => {
  it("shortens a raw launch cwd to its basename, full value in the tooltip", () => {
    render(
      <BreakdownPanel
        title="By actor"
        groupBy="actor"
        totalsCost={1}
        breakdown={[row({ key: "/home/thw-home/.typ-crews/naomi", cost: 1, tokens: 100, turns: 1 })]}
      />,
    );

    const label = screen.getByText("naomi");
    expect(label).toBeInTheDocument();
    expect(label).toHaveAttribute("title", "/home/thw-home/.typ-crews/naomi");
  });

  it("leaves an already-short pre-story-2 actor value (a bare crew name) unchanged", () => {
    render(
      <BreakdownPanel
        title="By actor"
        groupBy="actor"
        totalsCost={1}
        breakdown={[row({ key: "hestia", cost: 1, tokens: 100, turns: 1 })]}
      />,
    );

    const label = screen.getByText("hestia");
    expect(label).toHaveAttribute("title", "hestia");
  });

  it("applies basename collision walk-up between two different real actor paths sharing a basename", () => {
    render(
      <BreakdownPanel
        title="By actor"
        groupBy="actor"
        totalsCost={2}
        breakdown={[
          row({ key: "/home/thw-home/.typ-crews/naomi", cost: 1, tokens: 100, turns: 1 }),
          row({ key: "/home/other-host/.typ-crews/naomi", cost: 1, tokens: 100, turns: 1 }),
        ]}
      />,
    );

    expect(screen.getByText("thw-home/.typ-crews/naomi")).toBeInTheDocument();
    expect(screen.getByText("other-host/.typ-crews/naomi")).toBeInTheDocument();
  });

  it("flags the (unknown) sentinel actor as an unknown row", () => {
    render(
      <BreakdownPanel
        title="By actor"
        groupBy="actor"
        totalsCost={1}
        breakdown={[row({ key: "(unknown)", cost: 1, tokens: 100, turns: 1 })]}
      />,
    );

    const label = screen.getByText("(unknown)");
    expect(label.closest(".rank-row")).toHaveClass("unknown");
  });
});

// story-2/ticket-7: group_by=path's key is now machine-prefixed
// (`<hostname>:<path>` once the BFF substitutes it,
// `<install_id>:<path>` raw) — internal/domain/usage/summary.go's
// keyFor(GroupByPath) + service.go's substituteMachineHostnamesInPathKeys.
describe("BreakdownPanel — group_by=path", () => {
  it("shows the hostname-prefixed, shortened path as the label and the raw install_id:path as the tooltip", () => {
    render(
      <BreakdownPanel
        title="By path"
        groupBy="path"
        totalsCost={1}
        breakdown={[
          row({ key: "thw-home:/home/thw-home/gits/my-task", raw_key: "install-a-uuid:/home/thw-home/gits/my-task", cost: 1, tokens: 100, turns: 1 }),
        ]}
      />,
    );

    const label = screen.getByText("thw-home: my-task");
    expect(label).toBeInTheDocument();
    expect(label).toHaveAttribute("title", "install-a-uuid:/home/thw-home/gits/my-task");
  });

  it("falls back to key itself as the tooltip when raw_key is absent (no machines row)", () => {
    render(
      <BreakdownPanel
        title="By path"
        groupBy="path"
        totalsCost={1}
        breakdown={[row({ key: "install-orphan:/home/thw-home/gits/my-task", cost: 1, tokens: 100, turns: 1 })]}
      />,
    );

    const label = screen.getByText("install-orphan: my-task");
    expect(label).toHaveAttribute("title", "install-orphan:/home/thw-home/gits/my-task");
  });

  it("retires the machine-as-UI-disambiguator: two machines' identical raw paths render as two distinct rows via their own hostname prefix, not a '(hostname)' suffix", () => {
    render(
      <BreakdownPanel
        title="By path"
        groupBy="path"
        totalsCost={4}
        breakdown={[
          row({ key: "thw-home:/home/thw-home/gits/my-task", cost: 3, tokens: 100, turns: 1 }),
          row({ key: "thw-laptop:/home/thw-home/gits/my-task", cost: 1, tokens: 100, turns: 1 }),
        ]}
      />,
    );

    expect(screen.getByText("thw-home: my-task")).toBeInTheDocument();
    expect(screen.getByText("thw-laptop: my-task")).toBeInTheDocument();
    // The old story-1 disambiguator format ("my-task (thw-home)") must
    // not appear — the compound key already made these two rows distinct
    // before shortenPaths ever ran, so its own machine-fallback branch is
    // never reached from this call site anymore.
    expect(screen.queryByText(/\(thw-home\)/)).not.toBeInTheDocument();
    expect(screen.queryByText(/\(thw-laptop\)/)).not.toBeInTheDocument();
  });

  it("still applies basename collision walk-up between two different real paths sharing a basename", () => {
    render(
      <BreakdownPanel
        title="By path"
        groupBy="path"
        totalsCost={2}
        breakdown={[
          row({ key: "thw-home:/home/a/gits/my-template", cost: 1, tokens: 100, turns: 1 }),
          row({ key: "thw-home:/home/b/other/my-template", cost: 1, tokens: 100, turns: 1 }),
        ]}
      />,
    );

    expect(screen.getByText("thw-home: gits/my-template")).toBeInTheDocument();
    expect(screen.getByText("thw-home: other/my-template")).toBeInTheDocument();
  });

  it("flags the (unknown) sentinel path as an unknown row, not its hostname prefix", () => {
    render(
      <BreakdownPanel
        title="By path"
        groupBy="path"
        totalsCost={1}
        breakdown={[row({ key: "thw-home:(unknown)", cost: 1, tokens: 100, turns: 1 })]}
      />,
    );

    const label = screen.getByText("thw-home: (unknown)");
    expect(label.closest(".rank-row")).toHaveClass("unknown");
  });
});
