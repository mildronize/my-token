// story-1/ticket-14: unit tests for the contract's "Console display
// rules" path-shortening logic, as a pure function — no rendering, no
// React, just shortenPaths' own input/output contract.
import { describe, it, expect } from "vitest";
import { shortenPaths } from "./pathDisplay";

describe("shortenPaths — basename-only, no collisions", () => {
  it("shows the basename when every path in the list is already unique", () => {
    const got = shortenPaths([
      { path: "/home/thw-home/gits/my-template" },
      { path: "/home/thw-home/gits/my-task" },
    ]);
    expect(got.map((r) => r.label)).toEqual(["my-template", "my-task"]);
  });

  it("always sets title to the full canonical path, even with no collision", () => {
    const got = shortenPaths([{ path: "/home/thw-home/gits/my-template" }]);
    expect(got[0].title).toBe("/home/thw-home/gits/my-template");
    expect(got[0].label).toBe("my-template");
  });

  it("a path with no slashes at all (e.g. the attribution fallback) renders as itself", () => {
    const got = shortenPaths([{ path: "(unknown)" }, { path: "/gits/my-task" }]);
    expect(got[0].label).toBe("(unknown)");
    expect(got[0].title).toBe("(unknown)");
  });
});

describe("shortenPaths — basename collision, walk up parent segments", () => {
  it("walks up exactly one parent segment when that's enough to disambiguate", () => {
    const got = shortenPaths([
      { path: "/home/a/gits/my-template" },
      { path: "/home/b/other/my-template" },
    ]);
    expect(got.map((r) => r.label)).toEqual(["gits/my-template", "other/my-template"]);
  });

  it("keeps walking up with no fixed cap when one parent segment isn't enough", () => {
    const got = shortenPaths([
      { path: "/home/x/shared/gits/my-template" },
      { path: "/home/y/shared/other/my-template" },
    ]);
    // Both share "shared/.../my-template" and "gits"/"other" — after
    // one level up they're already unique (gits/my-template vs
    // other/my-template), so it should NOT need to walk all the way to
    // the full path.
    expect(got.map((r) => r.label)).toEqual(["gits/my-template", "other/my-template"]);
  });

  it("walks all the way to the full path when nothing shorter disambiguates", () => {
    const got = shortenPaths([
      { path: "/a/x/my-template" },
      { path: "/b/x/my-template" },
    ]);
    // basename ties ("my-template"), one level up ties again ("x/my-template")
    // — need the full path (three segments each) to differ.
    expect(got.map((r) => r.label)).toEqual(["a/x/my-template", "b/x/my-template"]);
  });

  it("only escalates the rows that actually collide, leaving unrelated rows at basename", () => {
    const got = shortenPaths([
      { path: "/home/a/gits/my-template" },
      { path: "/home/b/other/my-template" },
      { path: "/home/c/gits/my-task" },
    ]);
    expect(got.map((r) => r.label)).toEqual([
      "gits/my-template",
      "other/my-template",
      "my-task",
    ]);
  });

  it("resolves three-way collisions, not just pairs", () => {
    const got = shortenPaths([
      { path: "/one/x/app" },
      { path: "/two/x/app" },
      { path: "/three/y/app" },
    ]);
    expect(got.map((r) => r.label)).toEqual(["one/x/app", "two/x/app", "y/app"]);
  });
});

describe("shortenPaths — collision scope is exactly the given list", () => {
  it("the same path can shorten differently across two separate calls (different rendered sets) — not a bug", () => {
    const alone = shortenPaths([{ path: "/home/a/gits/my-template" }]);
    const withCollision = shortenPaths([
      { path: "/home/a/gits/my-template" },
      { path: "/home/b/other/my-template" },
    ]);
    expect(alone[0].label).toBe("my-template");
    expect(withCollision[0].label).toBe("gits/my-template");
  });
});

describe("shortenPaths — literal identical path string (cross-machine), machine disambiguator", () => {
  it("appends the machine when two rows share the exact same path string", () => {
    const got = shortenPaths([
      { path: "/home/thw-home/gits/my-task", machine: "install-a" },
      { path: "/home/thw-home/gits/my-task", machine: "install-b" },
    ]);
    expect(got.map((r) => r.label)).toEqual([
      "my-task (install-a)",
      "my-task (install-b)",
    ]);
    // Aggregation is untouched — both still report their own real path.
    expect(got.map((r) => r.path)).toEqual([
      "/home/thw-home/gits/my-task",
      "/home/thw-home/gits/my-task",
    ]);
    expect(got.every((r) => r.title === "/home/thw-home/gits/my-task")).toBe(true);
  });

  it("leaves the label as-is (still tied) when a machine disambiguator isn't available", () => {
    const got = shortenPaths([
      { path: "/home/thw-home/gits/my-task" },
      { path: "/home/thw-home/gits/my-task" },
    ]);
    expect(got.map((r) => r.label)).toEqual(["my-task", "my-task"]);
  });

  it("does not append a machine to rows that never collided in the first place", () => {
    const got = shortenPaths([
      { path: "/home/a/my-template", machine: "install-a" },
      { path: "/home/b/my-task", machine: "install-b" },
    ]);
    expect(got.map((r) => r.label)).toEqual(["my-template", "my-task"]);
  });
});

describe("shortenPaths — order and pass-through fields", () => {
  it("returns rows in the same order given, with machine passed through", () => {
    const got = shortenPaths([
      { path: "/z/app", machine: "m1" },
      { path: "/a/app", machine: "m2" },
    ]);
    expect(got[0].path).toBe("/z/app");
    expect(got[0].machine).toBe("m1");
    expect(got[1].path).toBe("/a/app");
    expect(got[1].machine).toBe("m2");
  });

  it("an empty list returns an empty list", () => {
    expect(shortenPaths([])).toEqual([]);
  });
});
