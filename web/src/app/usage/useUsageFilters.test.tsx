// story-2/ticket-11: localStorage persistence for the console's two new
// filters (contract's "Console filters" section — selected values persist
// to localStorage, restored on mount; the window tab is deliberately NOT
// persisted, unaffected here). Reconciling a stale/invalid restored value
// against the current data ("falls back to 'no filter' ... never
// surfaced as an error") is UsagePage.tsx's own job, tested there against
// real query responses — this file covers only the persistence seam
// itself: get/set/round-trip.
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

  it("starts with no filter selected on a first-ever visit (nothing in localStorage)", () => {
    const { result } = renderHook(() => useUsageFilters());
    expect(result.current.machine).toBeUndefined();
    expect(result.current.scanRoot).toBeUndefined();
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
