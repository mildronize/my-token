// story-1/ticket-14: the console's own `path` label shortening
// (contract's "Console display rules" section) — pure, presentation-only
// logic, deliberately implemented client-side rather than server-side.
// The contract's own collision rule is scoped to "the rendered set, not
// global": two labels only need to differ from each other within the
// specific list actually on screen right now (e.g. one panel's top 8 for
// one window), not against every path the core service has ever seen.
// The server has no notion of "what's currently rendered" at all — only
// the SPA does, since it's the one deciding how many rows of a
// window+group_by's breakdown to actually show — so this can only
// correctly live here, over whatever slice of rows the caller is about
// to render.
//
// Algorithm (contract, points 1-5):
//   1. Start every row at its basename (the last path segment).
//   2. Whenever two or more rows in the given list share the same
//      candidate label, walk up one more parent segment for exactly
//      those tied rows (not the whole list) and re-check — repeat until
//      every row's candidate is unique, or a row has expanded to its own
//      full path and still ties with another (which, since two
//      *different* strings can never produce the same "walked up to the
//      end" result, only happens when the two rows' raw paths are the
//      literal same string).
//   3. That literal-identical-string case is real (contract point 4):
//      it means the same path was reported by two different machines
//      (cross-machine path identity is currently just the raw
//      canonicalized string — a known, deliberately deferred limitation,
//      not something this function fixes). When it happens, append the
//      machine as a final, UI-only disambiguator.
//   4. The full canonical path is always available via `title`,
//      regardless of whether shortening needed to collide-resolve at
//      all (contract point 5).

export interface PathDisplayInput {
  /** The raw, full canonicalized path — always what `title` echoes back. */
  path: string;
  /**
   * Which machine (install_id) reported this row, if known — only
   * consulted as the last-resort disambiguator (step 3, above), when two
   * rows' `path` strings are still literally identical after walking up
   * to their own full length.
   */
  machine?: string;
}

export interface PathDisplayResult {
  path: string;
  machine?: string;
  /** The shortened label to render. */
  label: string;
  /** Always the full canonical path — for the label's `title` attribute. */
  title: string;
}

/** Splits a path into its non-empty segments (`/a/b/c` and `a/b/c` both -> `["a","b","c"]`). */
function segmentsOf(path: string): string[] {
  return path.split("/").filter((s) => s.length > 0);
}

/**
 * The last `depth` segments of path, joined back with `/` — depth is
 * clamped to the path's own segment count (so it never exceeds "the
 * whole path"). A path with no `/` at all (e.g. the literal fallback
 * value `"(unknown)"`, contract's own path-attribution fallback chain)
 * has one "segment" (itself) at any depth.
 */
function candidateAt(path: string, depth: number): string {
  const segs = segmentsOf(path);
  if (segs.length === 0) return path;
  const d = Math.min(depth, segs.length);
  return segs.slice(-d).join("/");
}

/**
 * Applies the contract's path-shortening rule to exactly the rows given
 * — callers must pass exactly the rendered set (e.g. one panel's current
 * top-N for one window), never a wider "every path ever seen" list, per
 * the contract's own collision-scope rule. Rows are returned in the same
 * order they were given.
 */
export function shortenPaths(rows: PathDisplayInput[]): PathDisplayResult[] {
  const depth = rows.map(() => 1);
  const maxDepth = rows.map((r) => Math.max(1, segmentsOf(r.path).length));

  // Rows whose raw `path` is a literal duplicate of another row's are
  // "locked" at the basename — walking either of them up further can
  // never disambiguate them from their twin (they're the same string at
  // every depth), so escalating them would only produce a needlessly
  // long label before falling back to the machine suffix anyway
  // (contract's own example is `my-template (thw-home)`, a basename,
  // not a fully expanded path). A locked row can still be told apart
  // from an unrelated *different*-string row that happens to share its
  // basename — that other row is free to escalate on its own.
  const rawPathCounts = new Map<string, number>();
  for (const r of rows) rawPathCounts.set(r.path, (rawPathCounts.get(r.path) ?? 0) + 1);
  const locked = rows.map((r) => (rawPathCounts.get(r.path) ?? 0) > 1);

  // Iteratively escalate depth only for non-locked rows still tied with
  // at least one other row at their current candidate — converges
  // because every round either resolves a tie or every escalatable tied
  // row is already at its own max depth.
  let changed = true;
  while (changed) {
    changed = false;
    const groups = new Map<string, number[]>();
    rows.forEach((r, i) => {
      const cand = candidateAt(r.path, depth[i]);
      const list = groups.get(cand);
      if (list) list.push(i);
      else groups.set(cand, [i]);
    });
    for (const idxs of groups.values()) {
      if (idxs.length <= 1) continue;
      for (const i of idxs) {
        if (!locked[i] && depth[i] < maxDepth[i]) {
          depth[i] += 1;
          changed = true;
        }
      }
    }
  }

  const labels = rows.map((r, i) => candidateAt(r.path, depth[i]));

  // Anything still tied at this point is tied because the two rows' raw
  // `path` strings are themselves identical (contract point 4) — either
  // both are locked at their basename, or (an unescalatable non-locked
  // row) both have reached maxDepth, which recovers the full raw path.
  // Disambiguate with `machine`, when available.
  const finalGroups = new Map<string, number[]>();
  labels.forEach((label, i) => {
    const list = finalGroups.get(label);
    if (list) list.push(i);
    else finalGroups.set(label, [i]);
  });
  for (const idxs of finalGroups.values()) {
    if (idxs.length <= 1) continue;
    for (const i of idxs) {
      if (rows[i].machine) {
        labels[i] = `${labels[i]} (${rows[i].machine})`;
      }
      // No machine to disambiguate with: leave as-is. A genuinely
      // unresolvable duplicate (same path, same/no machine) is an
      // accepted edge case — there is nothing left in this function's
      // own input to tell the two rows apart.
    }
  }

  return rows.map((r, i) => ({
    path: r.path,
    machine: r.machine,
    label: labels[i],
    title: r.path,
  }));
}
