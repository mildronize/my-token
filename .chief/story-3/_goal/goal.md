# Goal

my-token's console gets a second page — **Machines** — a fleet-overview
list, complementing the existing `/usage` dashboard's time-sliced,
single-panel "By machine" ranking with a dedicated, lifetime-scoped view
of every machine that has ever reported.

Concretely, this story delivers:

1. **A new route, `/machines`**, reachable via a "Machines" link in the
   header nav (next to "Usage").

2. **A summary strip at the top**: total machine count and combined
   lifetime cost across all of them — the same "at a glance" framing the
   `/usage` dashboard's own top-bar pill already uses.

3. **One table, one row per machine, sorted by "Last reported" descending**
   (most recently reported first — an "is my fleet alive" framing, not a
   cost-ranking one; `/usage`'s own panels already rank by cost, this page
   deliberately answers a different question). Each row shows:
   - Hostname (install_id reachable via tooltip)
   - **Last reported** — the last time this machine's collector
     successfully sent a batch (`machines.last_seen_at`, stamped at
     ingestion time by the server's own clock). Deliberately *not* the
     timestamp of the machine's most recent real Claude Code activity
     (`MAX(usage_events.created_at)`) — that's a different, "last seen by
     the user" question this story isn't answering. Named "Last reported"
     rather than "Last seen" specifically to avoid that ambiguity.
   - Lifetime cost
   - Lifetime tokens
   - **Collected Paths** — count of distinct paths/projects this machine
     has reported (was originally deferred, added back in after reviewing
     the draft design)
   - **Collected Actors** — count of distinct actors this machine has
     reported

   "Collected" names both count columns consistently — chosen over
   "Distinct"/"Unique" (read as programmer jargon) and "Detected" (implies
   an inference step that isn't happening; this is a plain count).

   Lifetime-scoped throughout, deliberately with no time-window tabs —
   this page is a fleet inventory, not a trend view; time-slicing already
   has a home on `/usage`. No "Scan roots" column here — a machine can have
   several, and a table cell is the wrong place to show them (see point 5).

4. **Clicking a row navigates to that machine's detail page** (point 5),
   not directly to `/usage` — reversing this story's own original plan
   after the draft design showed a machine with multiple scan roots
   couldn't be shown decently in one table cell.

5. **A new machine detail page, `/machines/:installId`**, showing:
   - The same identity/lifetime stats as the overview row (hostname,
     install_id, last reported, lifetime cost/tokens, collected
     paths/actors)
   - **Scan roots**, properly laid out (not cramped into a table cell) —
     each one's name, its registered path, and its source type, exactly
     as configured in that machine's own collector config
   - A **"View in Usage dashboard" button** that does what this story
     originally planned for the overview row itself: writes this
     machine's install_id into the exact `localStorage` key the "Filter by
     machine" dropdown already uses, then navigates to `/usage` —
     reusing story-2's filter mechanism as-is, no new plumbing.

## Out of Scope

- Most-used model per machine — a real, derivable column deliberately left
  out to ship a tight page first; can be added later if it turns out to be
  missed.
- Any time-window scoping on this page (5h/24h/Today/etc.) — lifetime
  totals only, by design.
- A URL-query-param mechanism for the machine filter (e.g.
  `/usage?machine=...`) — considered and rejected in favor of reusing the
  existing `localStorage`-only filter mechanism as-is, since a
  machine-scoped dashboard view isn't something that needs to be
  shareable/bookmarkable as its own link.
- Backfilling old `usage_events` rows that predate `scan_root` tracking —
  unaffected by and unrelated to this story.
- A shared/global machine registry or any cross-machine identity
  correlation — story-1 (ticket 5) and story-2 already ruled this out;
  each machine's `install_id` stays its own opaque, per-install identity.
