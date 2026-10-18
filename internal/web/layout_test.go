package web

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestTodayLayout(t *testing.T) {
	const token = `tok"en<script>`
	body := get(t, newMux(nil, CSRFFunc(func(*http.Request) string { return token })), "/")

	if !strings.Contains(body, `<html lang="he" dir="rtl"`) {
		t.Fatalf("html lang/dir missing:\n%s", body[:min(400, len(body))])
	}
	if !strings.Contains(body, `href="#main"`) || !strings.Contains(body, `id="main"`) {
		t.Fatal("skip link or main landmark missing")
	}
	for _, href := range []string{`href="/"`, `href="/review"`, `href="/progress"`} {
		if !strings.Contains(body, href) {
			t.Fatalf("nav href %s missing", href)
		}
	}
	tag := dockHref(body, "/")
	if !strings.Contains(tag, `aria-current="page"`) {
		t.Fatalf("today link is not current: %s", tag)
	}
	if strings.Contains(dockHref(body, "/review"), `aria-current="page"`) {
		t.Fatal("review link is current on Today")
	}
	if !strings.Contains(body, `name="csrf-token"`) || !strings.Contains(body, `content="tok&#34;en&lt;script&gt;"`) {
		t.Fatal("csrf meta was not escaped")
	}
	if !strings.Contains(body, "X-CSRF-Token") || !strings.Contains(body, "hx-headers") {
		t.Fatal("hx-headers missing X-CSRF-Token")
	}
	if !strings.Contains(body, `tok&#34;en&lt;script&gt;`) {
		t.Fatal("token was not escaped into hx-headers")
	}
	if strings.Contains(body, "<script>") && strings.Contains(body, token) {
		t.Fatal("raw token broke out of an attribute")
	}
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") || strings.Contains(body, "url(//") {
		t.Fatal("rendered page has an external URL")
	}
	for _, src := range []string{`href="/static/css/tokens.css"`, `href="/static/css/base.css"`, `src="/static/js/htmx.min.js"`, `/static/icons/sprite.svg#icon-today`} {
		if !strings.Contains(body, src) {
			t.Fatalf("missing local asset %s", src)
		}
	}
	if strings.Contains(body, "icon-streak") || strings.Contains(body, "icon-xp") {
		t.Fatal("streak or xp icon is in the shell")
	}
	if regexp.MustCompile(`(?i)\b(streak|xp)\b`).MatchString(body) {
		t.Fatal("streak or xp copy is in the shell")
	}
	if !regexp.MustCompile(`<bdi lang="en" dir="ltr">[^<]*<bdi lang="he" dir="rtl">`).MatchString(body) {
		t.Fatal("Hebrew inside English is not isolated")
	}
	if !strings.Contains(body, `<bdi lang="en" dir="ltr">My morning</bdi>`) {
		t.Fatal("English inside Hebrew is not isolated")
	}
	if !strings.Contains(body, `class="he-display he-proof"`) || !strings.Contains(body, "שָׁלוֹם עֲלֵיכֶם") {
		t.Fatal("long pointed line missing")
	}
	scriptAt := strings.Index(body, "localStorage")
	cssAt := strings.Index(body, "/static/css/tokens.css")
	if scriptAt < 0 || cssAt < 0 || scriptAt > cssAt {
		t.Fatal("theme script must run before the stylesheet")
	}
	if !strings.Contains(body, `href="/lesson"`) {
		t.Fatal("Today does not link to the lesson player")
	}
}

func TestNavCurrent(t *testing.T) {
	h := newMux(nil, CSRFFunc(func(*http.Request) string { return "t" }))
	for _, path := range []string{"/review", "/progress"} {
		body := get(t, h, path)
		tag := dockHref(body, path)
		if !strings.Contains(tag, `aria-current="page"`) {
			t.Fatalf("%s is not current: %s", path, tag)
		}
		if !strings.Contains(body, "X-CSRF-Token") {
			t.Fatalf("%s missing csrf header wiring", path)
		}
	}
	lesson := get(t, h, "/lesson")
	if strings.Contains(lesson, `aria-current="page"`) {
		t.Fatal("lesson player should not mark a nav item current")
	}
}

func TestStaticHasNoExternalURL(t *testing.T) {
	h := New(nil)
	for _, path := range []string{
		"/static/css/tokens.css",
		"/static/css/base.css",
		"/static/js/htmx.min.js",
		"/static/icons/sprite.svg",
		"/static/fonts/noto-sans-hebrew-hebrew.woff2",
		"/static/fonts/noto-sans-hebrew-latin.woff2",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status %d", path, rec.Code)
		}
		text := strings.ReplaceAll(rec.Body.String(), "http://www.w3.org/2000/svg", "")
		if strings.Contains(text, "http://") || strings.Contains(text, "https://") {
			t.Fatalf("%s contains an external URL", path)
		}
	}
	css := get(t, h, "/static/css/tokens.css")
	if !strings.Contains(css, "oklch(") || !strings.Contains(css, "--space-9: 96px") {
		t.Fatal("tokens missing oklch or the 96px step")
	}
	if !strings.Contains(css, "light-dark(") || !strings.Contains(css, `color-scheme: light dark`) || !strings.Contains(css, `[data-theme="dark"]`) {
		t.Fatal("tokens missing scheme override")
	}
	if !strings.Contains(css, "--lh-he-body: 1.6") || !strings.Contains(css, "--lh-he-display: 1.3") {
		t.Fatal("pointed line-height tokens missing")
	}
	sprite := get(t, h, "/static/icons/sprite.svg")
	if strings.Contains(sprite, "icon-streak") || strings.Contains(sprite, "icon-xp") {
		t.Fatal("sprite still ships streak or xp")
	}
}

func dockHref(body, href string) string {
	return regexp.MustCompile(`<a\b[^>]*href="` + regexp.QuoteMeta(href) + `"[^>]*class="dock__link"[^>]*>|<a\b[^>]*class="dock__link"[^>]*href="` + regexp.QuoteMeta(href) + `"[^>]*>`).FindString(body)
}

func get(t *testing.T, h http.Handler, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s status %d", path, rec.Code)
	}
	return rec.Body.String()
}
