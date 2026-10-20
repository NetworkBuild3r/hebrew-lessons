package web

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// cspPages are the GET routes that render a document. Today, the lesson
// player, Review, and Progress are the ones named in the shell; the rest are
// the other learner GET routes that return a page without a signed-in learner.
var cspPages = []string{
	"/",
	"/lesson",
	"/review",
	"/progress",
	"/lessons",
	"/me",
	"/placement",
	"/signin",
}

// cspOtherGETs are learner GET routes that do not render a full page on the
// ungated router (redirect or not found). They still must not ship an inline
// script or an event handler.
var cspOtherGETs = []string{
	"/placement/review",
	"/lessons/1/report",
	"/lesson/missing/1",
	"/healthz",
	"/audio/not-a-clip.mp3",
}

const themeScriptTag = `<script src="/static/js/theme.js"></script>`

var (
	scriptTagRe = regexp.MustCompile(`(?i)<script\b([^>]*)>`)
	srcAttrRe   = regexp.MustCompile(`(?i)(?:^|\s)src\s*=`)
	handlerRe   = regexp.MustCompile(`(?i)(?:^|[\s"'/<])on[a-z]+\s*=`)
	hxOnRe      = regexp.MustCompile(`(?i)hx-on(?:[:][\w:-]+)?\s*=`)
)

func TestCSPPagesHaveNoInlineScripts(t *testing.T) {
	h := New(nil, nil)
	for _, path := range cspPages {
		t.Run(path, func(t *testing.T) {
			body := get(t, h, path)
			assertNoInlineScriptOrHandler(t, body)
		})
	}
	for _, path := range cspOtherGETs {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			assertNoInlineScriptOrHandler(t, rec.Body.String())
		})
	}
}

func TestCSPThemeScriptBeforeStylesheet(t *testing.T) {
	h := New(nil, nil)
	for _, path := range cspPages {
		t.Run(path, func(t *testing.T) {
			body := get(t, h, path)
			headStart := strings.Index(body, "<head>")
			headEnd := strings.Index(body, "</head>")
			if headStart < 0 || headEnd < headStart {
				t.Fatal("head missing")
			}
			head := body[headStart:headEnd]
			scriptAt := strings.Index(head, themeScriptTag)
			if scriptAt < 0 {
				t.Fatalf("head missing %s", themeScriptTag)
			}
			if strings.Contains(head[scriptAt:scriptAt+len(themeScriptTag)], "defer") ||
				strings.Contains(head[scriptAt:scriptAt+len(themeScriptTag)], "async") {
				t.Fatal("theme.js must be a blocking classic script")
			}
			// A second tag that adds defer, async, or type=module would still
			// leave the exact tag above in place. Reject any other theme tag.
			for _, tag := range scriptTagRe.FindAllStringSubmatch(head, -1) {
				attrs := tag[1]
				if !strings.Contains(attrs, "/static/js/theme.js") {
					continue
				}
				if strings.Contains(attrs, "defer") || strings.Contains(attrs, "async") || strings.Contains(strings.ToLower(attrs), "module") {
					t.Fatalf("theme script is not a blocking classic script: <script%s>", attrs)
				}
			}
			cssAt := strings.Index(head, `rel="stylesheet"`)
			if cssAt < 0 || scriptAt > cssAt {
				t.Fatal("theme.js must precede the first stylesheet")
			}
		})
	}
}

func TestCSPHeaderScriptSrcSelfOnly(t *testing.T) {
	h := New(nil, nil)
	for _, path := range cspPages {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d", rec.Code)
			}
			csp := rec.Header().Get("Content-Security-Policy")
			src, ok := cspDirective(csp, "script-src")
			if !ok {
				t.Fatalf("CSP missing script-src: %s", csp)
			}
			if src != "'self'" {
				t.Fatalf("script-src = %q, want 'self'", src)
			}
			for _, bad := range []string{"sha256-", "nonce-", "'unsafe-inline'", "'unsafe-eval'"} {
				if strings.Contains(csp, bad) {
					t.Fatalf("CSP contains %s: %s", bad, csp)
				}
			}
		})
	}
}

func cspDirective(csp, name string) (string, bool) {
	for _, part := range strings.Split(csp, ";") {
		part = strings.TrimSpace(part)
		if part == name || strings.HasPrefix(part, name+" ") {
			return strings.TrimSpace(strings.TrimPrefix(part, name)), true
		}
	}
	return "", false
}

func TestCSPThemeScriptServed(t *testing.T) {
	h := New(nil, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/js/theme.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "javascript") {
		t.Fatalf("Content-Type = %q, want a JavaScript type", ct)
	}
	if strings.TrimRight(rec.Body.String(), "\n") != views.ThemeBoot {
		t.Fatal("theme.js bytes are not the theme boot")
	}
}

func assertNoInlineScriptOrHandler(t *testing.T, body string) {
	t.Helper()
	for _, tag := range scriptTagRe.FindAllStringSubmatch(body, -1) {
		if !srcAttrRe.MatchString(tag[1]) {
			t.Fatalf("script element without src: <script%s>", tag[1])
		}
	}
	if m := handlerRe.FindString(body); m != "" {
		t.Fatalf("inline event handler %q", m)
	}
	if m := hxOnRe.FindString(body); m != "" {
		t.Fatalf("hx-on attribute %q", m)
	}
}
