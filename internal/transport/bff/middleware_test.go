package bff

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestSecureFromURL_FollowsConfiguredScheme is task-10's regression test:
// the cookie Secure attribute must be derived from cfg.AuthAudience's
// scheme, not hardcoded. This directly
// covers the bug that broke มายด์'s own first login attempt — an
// http://localhost AuthAudience (GETTING-STARTED.md's documented local-dev
// setup) must produce Secure=false, since Safari refuses to store a Secure
// cookie over plain http. A real (always-https) deployment must still get
// Secure=true, unaffected by this change.
func TestSecureFromURL_FollowsConfiguredScheme(t *testing.T) {
	cases := []struct {
		name   string
		url    string
		secure bool
	}{
		{"http scheme is never secure", "http://localhost:8080", false},
		{"http scheme, no port", "http://localhost", false},
		{"https scheme is always secure", "https://app.example.test", true},
		{"https scheme, local-style host", "https://localhost:8443", true},
		// A URL that fails to parse outright fails safe to Secure=true
		// (the stricter attribute) rather than silently downgrading
		// cookies on a malformed config — see secureFromURL's own doc
		// comment. An empty value parses successfully with an empty
		// scheme, so it lands on the same false branch as any other
		// non-"https" scheme (never reachable in practice: both callers
		// refuse to set any cookie while configured(cfg) is false, which
		// an empty AuthAudience always implies).
		{"unparseable URL fails safe to secure", "://not-a-url", true},
		{"empty URL is not https, not secure", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := secureFromURL(tc.url)
			if got != tc.secure {
				t.Errorf("secureFromURL(%q) = %v, want %v", tc.url, got, tc.secure)
			}
		})
	}
}

// TestRenderLoginError_IsStyledWithAppTheme is story-1/ticket-17's
// regression test for the server-rendered login/error page: this used to
// be bare, unstyled HTML (a bug this ticket fixes), and before that it
// must never carry the app's old blue "lagoon" theme (globals.css,
// replaced app-wide by ticket-17's amber/gold palette). Asserts the page
// actually carries real CSS (not just a color swap on unstyled markup)
// using the same amber/gold hex values web/src/styles/globals.css and
// usage-console.css use, and that none of the specific blue hex values
// the old theme used anywhere survive in this response.
func TestRenderLoginError_IsStyledWithAppTheme(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/callback", nil)

	renderLoginError(c, nil, "some internal reason never shown to the user")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}

	body := w.Body.String()

	if !strings.Contains(body, "Login failed") {
		t.Fatalf("body must still contain %q (existing behavior other tests rely on), got: %s", "Login failed", body)
	}
	if strings.Contains(body, "some internal reason never shown to the user") {
		t.Fatalf("the specific internal reason must never leak into the rendered page, got: %s", body)
	}
	if !strings.Contains(body, "<style") {
		t.Fatalf("page must carry real styling (a <style> block), not just unstyled HTML, got: %s", body)
	}

	// Same amber/gold hex values the frontend's own theme tokens use
	// (web/src/styles/globals.css's light :root block / usage-console.css) —
	// at least one must appear, proving this page draws from the same
	// palette rather than an unrelated ad-hoc color choice.
	amberHexen := []string{"#b8790a", "#8f5c06", "#f0a202", "#ffc857"}
	foundAmber := false
	for _, hex := range amberHexen {
		if strings.Contains(strings.ToLower(body), strings.ToLower(hex)) {
			foundAmber = true
			break
		}
	}
	if !foundAmber {
		t.Fatalf("body must use the app's amber/gold theme colors (one of %v), got: %s", amberHexen, body)
	}

	// None of the old blue "lagoon" theme's specific hex values (formerly
	// in globals.css, before ticket-17 recolored them) may appear.
	blueHexen := []string{"#3b82f6", "#2563eb", "#1d4ed8", "#60a5fa", "#93bbfd", "#7daffc", "#bdd4fe"}
	for _, hex := range blueHexen {
		if strings.Contains(strings.ToLower(body), strings.ToLower(hex)) {
			t.Fatalf("body must not contain old blue-theme color %q, got: %s", hex, body)
		}
	}
}
