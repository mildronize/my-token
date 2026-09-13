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

// story-2/ticket-14: group_by=actor's key is the session's raw launch cwd
// (ticket 7 removed the `.typ-crews/<name>` regex), so it needs the same
// basename+tooltip shortening `path` already gets. story-2/ticket-16
// further makes the key machine-prefixed (`<machine>:<actor>`) for the
// same cross-machine false-merge reason `path` was fixed for — so these
// tests now mirror the `group_by=path` block below exactly, just over an
// actor value instead of a path.
describe("BreakdownPanel — group_by=actor", () => {
  it("shows the hostname-prefixed, shortened actor as the label and the raw install_id:actor as the tooltip", () => {
    render(
      <BreakdownPanel
        title="By actor"
        groupBy="actor"
        totalsCost={1}
        breakdown={[
          row({ key: "thw-home:/home/thw-home/.typ-crews/naomi", raw_key: "install-a-uuid:/home/thw-home/.typ-crews/naomi", cost: 1, tokens: 100, turns: 1 }),
        ]}
      />,
    );

    const label = screen.getByText("thw-home: naomi");
    expect(label).toBeInTheDocument();
    expect(label).toHaveAttribute("title", "install-a-uuid:/home/thw-home/.typ-crews/naomi");
  });

  it("falls back to key itself as the tooltip when raw_key is absent (no machines row)", () => {
    render(
      <BreakdownPanel
        title="By actor"
        groupBy="actor"
        totalsCost={1}
        breakdown={[row({ key: "install-orphan:/home/thw-home/.typ-crews/naomi", cost: 1, tokens: 100, turns: 1 })]}
      />,
    );

    const label = screen.getByText("install-orphan: naomi");
    expect(label).toHaveAttribute("title", "install-orphan:/home/thw-home/.typ-crews/naomi");
  });

  it("two machines' identical raw actor path render as two distinct rows via their own hostname prefix", () => {
    render(
      <BreakdownPanel
        title="By actor"
        groupBy="actor"
        totalsCost={4}
        breakdown={[
          row({ key: "thw-home:/home/thw-home/.typ-crews/naomi", cost: 3, tokens: 100, turns: 1 }),
          row({ key: "thw-home-openrouter:/home/thw-home/.typ-crews/naomi", cost: 1, tokens: 100, turns: 1 }),
        ]}
      />,
    );

    expect(screen.getByText("thw-home: naomi")).toBeInTheDocument();
    expect(screen.getByText("thw-home-openrouter: naomi")).toBeInTheDocument();
  });

  it("applies basename collision walk-up between two different real actor paths sharing a basename", () => {
    render(
      <BreakdownPanel
        title="By actor"
        groupBy="actor"
        totalsCost={2}
        breakdown={[
          row({ key: "thw-home:/home/a/.typ-crews/naomi", cost: 1, tokens: 100, turns: 1 }),
          row({ key: "thw-home:/home/b/other/.typ-crews/naomi", cost: 1, tokens: 100, turns: 1 }),
        ]}
      />,
    );

    expect(screen.getByText("thw-home: a/.typ-crews/naomi")).toBeInTheDocument();
    expect(screen.getByText("thw-home: other/.typ-crews/naomi")).toBeInTheDocument();
  });

  it("flags the (unknown) sentinel actor as an unknown row, not its hostname prefix", () => {
    render(
      <BreakdownPanel
        title="By actor"
        groupBy="actor"
        totalsCost={1}
        breakdown={[row({ key: "thw-home:(unknown)", cost: 1, tokens: 100, turns: 1 })]}
      />,
    );

    const label = screen.getByText("thw-home: (unknown)");
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

// story-2/ticket-16: มายด์'s own observation — once a machine filter is
// active, every row on screen is already scoped to that one machine, so
// repeating its name as a prefix on every row is pure noise. Applies
// identically to `path` and `actor`, both machine-prefixed keys.
describe("BreakdownPanel — activeMachine suppresses the machine prefix", () => {
  it("drops the machine prefix from By path when a machine filter is active", () => {
    render(
      <BreakdownPanel
        title="By path"
        groupBy="path"
        totalsCost={1}
        activeMachine="install-a"
        breakdown={[row({ key: "thw-home:/home/thw-home/gits/my-task", raw_key: "install-a:/home/thw-home/gits/my-task", cost: 1, tokens: 100, turns: 1 })]}
      />,
    );

    expect(screen.getByText("my-task")).toBeInTheDocument();
    expect(screen.queryByText("thw-home: my-task")).not.toBeInTheDocument();
    // The full raw key is still reachable via the tooltip even with the
    // prefix hidden from the label.
    expect(screen.getByText("my-task")).toHaveAttribute("title", "install-a:/home/thw-home/gits/my-task");
  });

  it("drops the machine prefix from By actor when a machine filter is active", () => {
    render(
      <BreakdownPanel
        title="By actor"
        groupBy="actor"
        totalsCost={1}
        activeMachine="install-a"
        breakdown={[row({ key: "thw-home:/home/thw-home/.typ-crews/naomi", raw_key: "install-a:/home/thw-home/.typ-crews/naomi", cost: 1, tokens: 100, turns: 1 })]}
      />,
    );

    expect(screen.getByText("naomi")).toBeInTheDocument();
    expect(screen.queryByText("thw-home: naomi")).not.toBeInTheDocument();
  });

  it("still shows the machine prefix when no machine filter is active", () => {
    render(
      <BreakdownPanel
        title="By path"
        groupBy="path"
        totalsCost={1}
        breakdown={[row({ key: "thw-home:/home/thw-home/gits/my-task", cost: 1, tokens: 100, turns: 1 })]}
      />,
    );

    expect(screen.getByText("thw-home: my-task")).toBeInTheDocument();
  });

  it("does not affect By machine's own label, which never carries a prefix to begin with", () => {
    render(
      <BreakdownPanel
        title="By machine"
        groupBy="machine"
        totalsCost={1}
        activeMachine="install-a"
        breakdown={[row({ key: "thw-home", raw_key: "install-a", cost: 1, tokens: 100, turns: 1 })]}
      />,
    );

    expect(screen.getByText("thw-home")).toBeInTheDocument();
  });
});
