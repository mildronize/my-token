# Ticket 17 Report

## Ticket

Apply one consistent amber/gold theme across the whole `my-token` fork,
replacing ticket 14's leftover blue "lagoon" theme app-wide (not just
`/usage`), including server-rendered login/error pages in
`internal/transport/bff`.

## Outcome

done

## Decision

- **Issue: how to "reconcile" the theme — merge `usage-console.css`'s
  `--uc-*` tokens and `globals.css`'s `--sea-ink`/`--lagoon`/`--sand`
  tokens into one shared set, or keep two token sets but recolor the
  app-wide one to match?**
  The ticket's own wording explicitly allows the second option:
  "Reconcile into one shared token set rather than two parallel
  stylesheets, if that's cleaner than search-and-replace." Merging
  would mean renaming every `var(--sea-ink)`/`var(--lagoon)`/etc.
  reference across ~15 files (Header, Footer, AuthGate, EmptyState,
  ErrorState, settings pages, every shadcn `ui/*` component) to
  `--uc-*` names, or vice versa — real churn with no functional upside
  once both sets carry the same colors.
  **Chosen:** recolor-in-place. Kept every existing variable name in
  `globals.css` (`--sea-ink`, `--lagoon`, `--lagoon-deep`, `--palm`,
  `--sand`, `--foam`, `--surface*`, `--line`, `--chip-*`, `--hero-*`,
  `--kicker`, `--bg-base`, `--header-bg`, `--link-bg-hover`, plus the
  shadcn `--background`/`--primary`/`--secondary`/`--muted`/`--accent`/
  `--border`/`--input`/`--ring` set consumed via Tailwind's `@theme
  inline`), only changed their hex/oklch *values* to the same
  amber/gold family `usage-console.css` already uses (light: ink
  `#241d12`, accent `#b8790a`/`#8f5c06`; dark: ink `#ede6d8`, accent
  `#f0a202`/`#ffc857`). This rethemed every consumer — all of Header,
  Footer, AuthGate's spinner, settings/ApiKeySettings, and every
  shadcn `ui/*` component (button, badge, dialog, spinner, progress,
  etc., all of which reach these tokens only through Tailwind's
  `bg-primary`/`bg-accent`/etc. classes) — without touching any of
  those files individually. `usage-console.css`'s own `--uc-*` tokens
  were left as their own scoped set (verified their light/dark accent
  values match the recolored app-wide ones exactly); its stale header
  comment (which described the old blue-vs-amber split) was rewritten
  to explain the new reality. Flagged by both the ticket's own
  permissive language and confirmed reasonable by `/chief-review-code`'s
  Spec-axis sub-agent as a fair reading, not a shortcut around it — the
  tradeoff (two token sets must be hand-kept in sync if the palette
  changes again) is called out explicitly in `usage-console.css`'s
  updated comment.

- **Issue: `internal/transport/bff`'s only server-rendered HTML page
  (`renderLoginError`) had zero real styling** — bare
  `<!doctype html><html><body><h1>...`. No CSS custom properties are
  reachable from Go (no shared build step with the SPA), so the
  ticket's own text explicitly allows "inline `<style>` ... for what's
  likely a small number of simple HTML pages — don't over-engineer a
  full build pipeline." Confirmed via repo-wide grep for
  `text/html`/`c.String(http.` across `internal/`/`cmd/` that this is
  genuinely the *only* such handler (GET /login and the success path
  of GET /callback both just issue redirects; nothing else in the
  Go backend renders HTML).
  **Chosen:** a self-contained inline `<style>` block
  (`loginErrorPage` constant in `middleware.go`) hand-carrying the same
  four light/dark hex values `globals.css`'s `:root` blocks use, with
  a comment explaining why these are hand-kept-in-sync literals rather
  than shared tokens. Wrote a red-first regression test
  (`TestRenderLoginError_IsStyledWithAppTheme`) before implementing:
  confirmed it failed against the old unstyled output (`page must
  carry real styling`), then implemented and confirmed green. The test
  also asserts none of the old blue theme's specific hex values
  (`#3b82f6`, `#2563eb`, `#1d4ed8`, `#60a5fa`, `#93bbfd`, `#7daffc`,
  `#bdd4fe`) ever appear in this page's output, and that the existing
  `assert.Contains(rec.Body.String(), "Login failed")` other tests
  depend on still holds.
  **Gotcha caught during implementation:** the new CSS literally
  contains `calc(100% - 2rem)`. `gin.Context.String` runs its format
  argument through `fmt.Sprintf` when any values are passed (here, the
  one escaped user-facing message) — a bare `%` in the format string
  would have been misinterpreted as a broken format verb and corrupted
  the response. Escaped it to `100%%` before ever running the test;
  caught by reasoning about `c.String`'s own implementation, not by a
  test failure (the naive test would have still technically "passed"
  string-containment checks with garbled output nearby, so this was
  worth catching by inspection rather than relying on the test suite
  alone).

## Notes

**Files changed** (commit `d90c0ef`, 5 files, +301/−126):
- `web/src/styles/globals.css` — every blue hex/oklch value in the
  light `:root`, dark `:root[data-theme="dark"]`, and the
  `prefers-color-scheme` duplicate block recolored to amber/gold;
  hardcoded blue values in `body`'s background gradients, `body::before`,
  `a:hover` (both light and dark), `.nav-link::after`'s gradient
  end-stop (switched to `var(--lagoon-deep)` instead of a new magic
  hex, per code review), and `.island-shell`/`.feature-card`'s
  box-shadow tints (recolored from navy `26,46,74` to the new ink's
  brown-black `36,29,18`) all fixed.
- `web/src/components/Header.tsx` — one hardcoded shadow rgba
  (`rgba(26,46,74,0.08)` → `rgba(36,29,18,0.08)`) matching the new ink
  color; everything else in Header/Footer/AuthGate/settings already
  referenced only CSS custom properties, so needed no direct edits —
  verified by reading each file fresh rather than assuming.
- `web/src/styles/usage-console.css` — header comment rewritten (no
  functional change) to describe the new one-theme reality instead of
  the retired blue/amber split.
- `internal/transport/bff/middleware.go` — `renderLoginError` now
  renders a real styled page (`loginErrorPage` constant: centered
  card, amber/gold light+dark via `prefers-color-scheme`, "Try again"
  button) instead of bare unstyled HTML.
- `internal/transport/bff/middleware_test.go` — new
  `TestRenderLoginError_IsStyledWithAppTheme` (red-first).

**Seam tested:** `renderLoginError`'s own HTTP response body — the one
seam this ticket added new behavior at (real styling, previously
none). Everything else is a pure CSS value recolor with no new logic,
so no other seam applied; existing component/API tests were run to
confirm nothing broke, not to drive new behavior.

**Verification:**
- `cd web && npx tsc -b --noEmit` — clean.
- `cd web && npx vitest run` — 4 test files, 21 tests, all passing
  (unchanged from before this ticket — no snapshot/assertion needed
  updating).
- `go build ./...` — green.
- `go vet ./...` — clean.
- `go test ./...` — green, every package, including the new
  `TestRenderLoginError_IsStyledWithAppTheme` (confirmed red against
  the pre-change unstyled output before implementing, green after).
- `gofmt -l` on both changed Go files — clean.
- Repo-wide grep for every retired blue hex value
  (`#3b82f6`/`#2563eb`/`#1d4ed8`/`#60a5fa`/`#93bbfd`/`#7daffc`/
  `#bdd4fe`/`#1a2e4a`/`#4a6080`/`#e8eef6`/`#f0f5fc`) across `web/src`
  and `internal` — zero hits outside the new Go test's own
  negative-assertion list (which exists specifically to keep it that
  way).
- `/chief-review-code` (Standards + Spec axes, parallel sub-agents):
  Standards axis found no documented `.chief/_rules/_standard`
  violations (none exist in this repo) and one self-acknowledged
  baseline smell (the amber palette now lives in four
  independently-maintained places — `globals.css`, `usage-console.css`,
  the Go `loginErrorPage` constant, and the new test's hex lists —
  judged an accepted, architecturally-forced tradeoff given Go can't
  consume the SPA's CSS custom properties, not a fix-now item). Spec
  axis found zero missing/partial requirements, zero scope creep, and
  one minor blemish (the `.nav-link::after` magic hex) — fixed
  immediately (switched to `var(--lagoon-deep)`) before committing.

**Visual verification — honest limitation:** I do not have a real
browser available in this session, so I could not literally see any
screen rendered. What I verified instead: (1) a repo-wide grep
confirms zero occurrences of any specific old blue hex value anywhere
in the shipped CSS/HTML/Go output; (2) traced every SPA screen's own
source (`AuthGate`'s login-prompt/spinner state, `Header`/`Footer`
chrome, `UsagePage`'s loading/error/loaded states, `SettingsPage`,
`ApiKeySettings`) and confirmed each references only CSS custom
properties now pointing at amber/gold values, with no hardcoded colors
of its own; (3) the Go login-error page's new `<style>` block was read
back in full and its hex values cross-checked byte-for-byte against
`globals.css`'s `:root` blocks. This is strong indirect evidence but
not the same as having watched it render — flagging that plainly
rather than claiming otherwise.
