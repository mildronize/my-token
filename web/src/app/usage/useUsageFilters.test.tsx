// story-2/ticket-11: localStorage persistence for the console's two new
// filters (contract's "Console filters" section — selected values persist
// to localStorage, restored on mount). Reconciling a stale/invalid
// restored machine/scan-root value against the current data ("falls back
// to 'no filter' ... never surfaced as an error") is UsagePage.tsx's own
// job, tested there against real query responses — this file covers only
// the persistence seam itself: get/set/round-trip.
//
// story-2/ticket-15: the window tab persists too now (originally scoped
// out at planning time, added after มายด์ found that gap by testing the
// live console). Unlike machine/scan-root, an invalid stored window can
// be validated synchronously against the fixed FIXED_WINDOWS enum — no
// server round-trip needed — so this hook itself owns that fallback,
// tested directly below rather than in UsagePage.tsx.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { renderHook, act } from "@testing-library/react";

import { useUsageFilters } from "./useUsageFilters";

describe("useUsageFilters", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });
  afterEach(() => {
    window.localStorage.clear();
  });

  it("starts with no filter selected and window defaulted to 'today' on a first-ever visit (nothing in localStorage)", () => {
    const { result } = renderHook(() => useUsageFilters());
    expect(result.current.machine).toBeUndefined();
    expect(result.current.scanRoot).toBeUndefined();
    expect(result.current.window).toBe("today");
  });

  it("setWindow persists to localStorage and updates the returned value", () => {
    const { result } = renderHook(() => useUsageFilters());

    act(() => result.current.setWindow("year"));

    expect(result.current.window).toBe("year");
    expect(window.localStorage.getItem("my-token.usage-console.filter.window")).toBe("year");
  });

  it("a selected window survives a remount — a fresh hook instance reads the same persisted value back", () => {
    const first = renderHook(() => useUsageFilters());
    act(() => first.result.current.setWindow("lifetime"));
    first.unmount();

    const second = renderHook(() => useUsageFilters());
    expect(second.result.current.window).toBe("lifetime");
  });

  it("a stored window value that isn't one of the fixed windows falls back to 'today', not a crash", () => {
    window.localStorage.setItem("my-token.usage-console.filter.window", "not-a-real-window");
    const { result } = renderHook(() => useUsageFilters());
    expect(result.current.window).toBe("today");
  });

  it("setMachine persists to localStorage and updates the returned value", () => {
    const { result } = renderHook(() => useUsageFilters());

    act(() => result.current.setMachine("install-a"));

    expect(result.current.machine).toBe("install-a");
    expect(window.localStorage.getItem("my-token.usage-console.filter.machine")).toBe("install-a");
  });

  it("setScanRoot persists to localStorage and updates the returned value", () => {
    const { result } = renderHook(() => useUsageFilters());

    act(() => result.current.setScanRoot("install-a:~/.claude"));

    expect(result.current.scanRoot).toBe("install-a:~/.claude");
    expect(window.localStorage.getItem("my-token.usage-console.filter.scanRoot")).toBe("install-a:~/.claude");
  });

  it("a selected filter survives a remount — a fresh hook instance reads the same persisted value back", () => {
    const first = renderHook(() => useUsageFilters());
    act(() => first.result.current.setMachine("install-a"));
    first.unmount();

    const second = renderHook(() => useUsageFilters());
    expect(second.result.current.machine).toBe("install-a");
  });

  it("setMachine(undefined) clears the stored value, not just the in-memory one", () => {
    const { result } = renderHook(() => useUsageFilters());
    act(() => result.current.setMachine("install-a"));
    act(() => result.current.setMachine(undefined));

    expect(result.current.machine).toBeUndefined();
    expect(window.localStorage.getItem("my-token.usage-console.filter.machine")).toBeNull();
  });

  it("tolerates localStorage being unavailable (e.g. private browsing) — filter state still works in-memory", () => {
    const original = window.localStorage.setItem.bind(window.localStorage);
    window.localStorage.setItem = () => {
      throw new DOMException("blocked");
    };
    try {
      const { result } = renderHook(() => useUsageFilters());
      expect(() => act(() => result.current.setMachine("install-a"))).not.toThrow();
      expect(result.current.machine).toBe("install-a");
    } finally {
      window.localStorage.setItem = original;
    }
  });
});
