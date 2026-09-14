# 17: Apply the amber/gold theme app-wide

Type: implementation
Status: resolved
Blocked by: None (can start immediately)

## Delivers

Per the contract's "Frontend theme" section: one consistent amber/gold
theme across the whole app, replacing ticket 14's scoped-to-`/usage`
split entirely.

- Extend the amber/gold token set (`web/src/styles/usage-console.css`,
  ported from the approved mockup) to cover the rest of the SPA — replace
  `globals.css`'s existing blue theme rather than letting the two coexist.
  Reconcile into one shared token set rather than two parallel stylesheets,
  if that's cleaner than search-and-replace.
- Server-rendered login/error pages (`renderLoginError` and any other
  `internal/transport/bff` HTML responses outside the SPA) — these
  currently have minimal/no real styling; give them a real (if simple)
  styled pass using the same token set, not just a color swap on unstyled
  HTML.
- Nav/header and any other SPA chrome outside `/usage` specifically
  (`web/src/components/Header.tsx` etc.) — confirm nothing still renders in
  the old blue.

Verifiable: visually, no screen (logged out, mid-login-error, logged in,
`/usage`) shows the old blue theme anywhere. `npx tsc -b --noEmit` and
`npx vitest run` stay green (styling changes shouldn't break existing
component tests, but confirm rather than assume).
