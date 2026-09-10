package bff

import (
	"html"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/mildronize/my-token/internal/identity"
	"github.com/mildronize/my-token/internal/transport/publicapi"
)

// actorContextKey is this package's own gin-context key for the resolved
// owner — deliberately not shared with internal/transport/publicapi's own
// (unexported, package-private) key of the same purpose. ARCHITECTURE.md's
// "no transport/shared package" decision means each transport surface
// owns this small amount of duplication rather than reaching for a shared
// package that would otherwise hold nothing else.
const actorContextKey = "bff.actor"

// secureFromURL reports whether cookies should carry the Secure attribute
// for a service reachable at rawURL — true when rawURL's scheme is
// "https", false when it's "http". This is task-10's fix: Secure is derived from
// cfg.AuthAudience, the same value that already has to be correct for
// OAuth redirect matching to work at all (oauth.go's redirectURL), rather
// than a separately-settable flag someone could ship in the wrong
// position. `http://localhost` (GETTING-STARTED.md's documented local-dev
// setup) now correctly gets a non-Secure cookie — Safari refuses to store
// a Secure cookie over plain http, which is exactly the bug this fixes —
// while any real (always-https) deployment is unaffected.
//
// A rawURL that fails to parse (e.g. a missing scheme separator) fails
// safe to Secure=true, the stricter attribute, rather than silently
// downgrading to a non-Secure cookie on a malformed config. An empty or
// schemeless rawURL parses successfully with an empty Scheme, so it falls
// through to the false branch below like any other non-"https" value —
// in practice this never reaches a real request anyway, since both
// callers (NewLoginHandler, NewCallbackHandler) already refuse to set any
// cookie at all while configured(cfg) is false, which an empty
// cfg.AuthAudience always implies.
func secureFromURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	return u.Scheme == "https"
}

// setCookie centralizes the cookie attributes task-4.md's login-flow spec
// calls for on both cookies this package sets: HttpOnly (never readable
// from JS), Secure (per secureFromURL above), SameSite=Lax. path "/" so
// the cookie is sent back on every route this package registers, not just
// the one that set it.
func setCookie(c *gin.Context, name, value string, maxAge int, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, value, maxAge, "/", "", secure, true)
}

// clearCookie deletes a cookie this package previously set (maxAge<0).
func clearCookie(c *gin.Context, name string, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, "", -1, "/", "", secure, true)
}

// loginErrorPage is the one server-rendered HTML page this whole service
// draws (renderLoginError below is its only caller) — the login/logged-out
// side of story-1/ticket-17's "one consistent amber/gold theme across the
// whole app" (_contract/contract.md's "Frontend theme" section), so a user
// mid-failed-login sees the same palette as every other screen rather than
// bare unstyled markup. The exact hex values below are hand-kept in sync
// with web/src/styles/globals.css's light/dark `:root` blocks and
// web/src/styles/usage-console.css's `--uc-accent`/`--uc-accent-strong` —
// this page has no build step of its own (deliberately: ticket-17's own
// text says "inline <style> ... is fine for what's likely a small number
// of simple HTML pages — don't over-engineer a full build pipeline"), so
// there is no way to share the SPA's actual CSS custom properties here;
// keep these four values (light bg/card/text/accent, dark bg/card/text/
// accent) matching globals.css's `:root` / `:root[data-theme="dark"]`
// blocks if that palette ever changes again.
const loginErrorPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Login failed</title>
<style>
  :root {
    color-scheme: light dark;
    --bg: #f6f0e4;
    --card: #faf7ef;
    --border: rgba(36, 29, 18, 0.14);
    --text: #241d12;
    --text-soft: #7c7261;
    --accent: #b8790a;
    --accent-strong: #8f5c06;
  }
  @media (prefers-color-scheme: dark) {
    :root {
      --bg: #14110d;
      --card: #1e1912;
      --border: rgba(255, 200, 87, 0.16);
      --text: #ede6d8;
      --text-soft: #a89c87;
      --accent: #f0a202;
      --accent-strong: #ffc857;
    }
  }
  * { box-sizing: border-box; }
  body {
    margin: 0;
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg);
    color: var(--text);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  }
  .card {
    max-width: 26rem;
    width: calc(100%% - 2rem);
    margin: 1rem;
    padding: 2rem;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--card);
    box-shadow: 0 18px 34px rgba(36, 29, 18, 0.1);
    text-align: center;
  }
  h1 {
    margin: 0 0 0.75rem;
    font-size: 1.25rem;
    color: var(--accent-strong);
  }
  p {
    margin: 0 0 1.5rem;
    color: var(--text-soft);
    line-height: 1.5;
  }
  a.btn {
    display: inline-block;
    padding: 0.6rem 1.4rem;
    border-radius: 999px;
    background: var(--accent);
    color: #fff;
    font-weight: 600;
    text-decoration: none;
  }
  a.btn:hover {
    background: var(--accent-strong);
  }
</style>
</head>
<body>
  <div class="card">
    <h1>Login failed</h1>
    <p>%s</p>
    <a class="btn" href="/login">Try again</a>
  </div>
</body>
</html>`

// renderLoginError is the one error page GET /callback ever writes,
// regardless of which specific check failed (unrecognized state, expired
// state cookie, token-exchange failure, unrecognized sub, wrong role) —
// mirrors I5's "401 never leaks why" applied to this surface's own error
// shape (_contract/API.md's BFF conventions: "bff returns HTML ... not
// this JSON shape"). The specific reason is logged server-side only,
// exactly like identity.Service.unauthorized's own pattern.
func renderLoginError(c *gin.Context, logger *slog.Logger, reason string) {
	if logger != nil {
		logger.Warn("bff: login failed", "reason", reason)
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusUnauthorized, loginErrorPage,
		html.EscapeString("Something went wrong signing you in. Please try again."))
}

// ActorFromContext returns the identity.User RequireJSONSession
// (json_middleware.go) resolved for this request — every handler in this
// package (me_handler.go, keys_handler.go, usage_handler.go, and any
// other domain handler this surface grows) reads it through this one
// function instead of ever querying users itself (I4).
func ActorFromContext(c *gin.Context) (identity.User, bool) {
	v, ok := c.Get(actorContextKey)
	if !ok {
		return identity.User{}, false
	}
	user, ok := v.(identity.User)
	return user, ok
}

// bffOwnerID reads the actor RequireJSONSession already resolved onto the
// gin context (I4: this package never queries users itself for this
// purpose) and writes the standard 401 body if none is present. A
// domain-agnostic helper (every handler on this surface needs it, not
// just one domain's) — lives here, next to ActorFromContext, rather than
// inside any one domain-specific handler file, precisely so deleting a
// domain's handler file never takes this down with it (story-1/ticket-16
// moved this out of the deleted example domain's own bff handler for
// exactly that reason). The !ok branch is defensive, mirroring
// handleBFFMe: it should be unreachable given the intended middleware
// order (RequireJSONSession before any handler that calls this), and only
// guards a route ever wired without it.
func bffOwnerID(c *gin.Context) (string, bool) {
	user, ok := ActorFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, jsonUnauthorizedBody)
		return "", false
	}
	return user.ID, true
}

// bffValidationErrorBody builds a validation_error-coded ErrorEnvelope
// with a hint naming the offending field — reuses
// internal/transport/publicapi.ErrorEnvelope/NewErrorEnvelope directly
// (same "bff-openapi.yaml reuses publicapi's envelope" decision
// json_middleware.go's jsonUnauthorizedBody already follows). A
// domain-agnostic helper (usage_handler.go, and any other domain handler
// this surface grows, all need this same shape for their own request-time
// validation) — lives here rather than inside any one domain-specific
// handler file, same reasoning as bffOwnerID above (story-1/ticket-16
// moved this out of the deleted example domain's own bff handler).
func bffValidationErrorBody(message, hint string) publicapi.ErrorEnvelope {
	return publicapi.NewErrorEnvelope("validation_error", message, hint)
}
