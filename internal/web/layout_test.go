package web

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
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
	if strings.Contains(body, "good morning") || strings.Contains(body, "שָׁלוֹם") || strings.Contains(body, "he-proof") {
		t.Fatal("sample paragraph or gloss is still on Today")
	}
	if !strings.Contains(body, views.UnitTitle) || !strings.Contains(body, `class="unit-title"`) {
		t.Fatal("unit title missing")
	}
	if !strings.Contains(body, `<bdi lang="en" dir="ltr">Getting ready in the morning.</bdi>`) {
		t.Fatal("English situation is not isolated")
	}
	if strings.Count(body, "btn-primary") != 1 || !strings.Contains(body, `class="btn btn-primary"`) {
		t.Fatal("due Today should have one filled primary")
	}
	if !strings.Contains(body, `class="btn btn-tonal"`) {
		t.Fatal("lesson CTA should be tonal while reviews are due")
	}
	for _, word := range []string{"בֹּקֶר", "קָפֶה", "קָם"} {
		if !strings.Contains(body, `class="chip" lang="he">`+word) {
			t.Fatalf("chip %s missing", word)
		}
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
	for _, token := range []string{"--step-4:", "--fs-eyebrow: var(--step-1)", "--fs-chip: var(--step-3)", "--step-3: 1.25rem", "--shell-max: 1040px", "--bp-desk: 1024px", "--tap-btn: 48px", "--fw-semibold: 600"} {
		if !strings.Contains(css, token) {
			t.Fatalf("tokens missing %s", token)
		}
	}
	sprite := get(t, h, "/static/icons/sprite.svg")
	if strings.Contains(sprite, "icon-streak") || strings.Contains(sprite, "icon-xp") {
		t.Fatal("sprite still ships streak or xp")
	}
}

func TestPairSeparatesRuns(t *testing.T) {
	h := New(nil)
	he := get(t, h, "/?due=0")
	assertPair(t, he, "אֵין חֲזָרוֹת עַכְשָׁיו", "Nothing due", "he")
	en := get(t, h, "/?lang=en&due=0")
	assertPair(t, en, "אֵין חֲזָרוֹת עַכְשָׁיו", "Nothing due", "en")
	if strings.Contains(he, "good morning") || strings.Contains(en, "good morning") {
		t.Fatal("gloss line is still rendered")
	}
}

func TestZeroDueAndNoLesson(t *testing.T) {
	h := New(nil)
	zero := get(t, h, "/?due=0")
	if strings.Contains(zero, "ink-card") || strings.Contains(zero, "הַתְחֵל") {
		t.Fatal("zero due still shows the review hero")
	}
	if strings.Count(zero, "btn-primary") != 1 || !strings.Contains(zero, `class="btn btn-primary"`) {
		t.Fatal("the lesson CTA should be the only primary when nothing is due")
	}
	if strings.Contains(zero, "btn-tonal") {
		t.Fatal("tonal CTA remains when nothing is due")
	}
	none := get(t, h, "/?next=0")
	if strings.Contains(none, `href="/lesson"`) || strings.Contains(none, views.UnitTitle) {
		t.Fatal("missing lesson still renders the lesson card")
	}
	assertPair(t, none, "אֵין שִׁיעוּר בַּתּוֹר", "No lesson queued", "he")
	if strings.Count(none, "btn-primary") != 1 {
		t.Fatal("start should stay the only primary when no lesson is queued")
	}
}

func TestEnglishShell(t *testing.T) {
	body := get(t, New(nil), "/?lang=en")
	if !strings.Contains(body, `<html lang="en" dir="ltr"`) {
		t.Fatal("english shell lang/dir missing")
	}
	if !strings.Contains(body, `<span class="dock__label" lang="en">Today</span>`) {
		t.Fatal("english nav label missing")
	}
	if !strings.Contains(body, `href="/review?lang=en"`) || !strings.Contains(body, `href="/?lang=en"`) {
		t.Fatal("english shell dropped lang on a link")
	}
	if !strings.Contains(body, `<bdi lang="he" dir="rtl">`+views.AppName+`</bdi>`) {
		t.Fatal("hebrew wordmark is not isolated in the english shell")
	}
	if !strings.Contains(body, views.UnitTitle) {
		t.Fatal("unit title left the english shell")
	}
}

func assertPair(t *testing.T, body, he, en, first string) {
	t.Helper()
	m := regexp.MustCompile(`(?s)<span class="pair">(.*?)</span>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatal("pair component missing")
	}
	inner := m[1]
	gap := regexp.MustCompile(`</bdi>(.*?)<bdi`).FindStringSubmatch(inner)
	if gap == nil || strings.TrimSpace(gap[1]) != "" {
		t.Fatalf("pair runs are not separated by a gap: %s", inner)
	}
	heAt := strings.Index(inner, `lang="he"`)
	enAt := strings.Index(inner, `lang="en"`)
	if heAt < 0 || enAt < 0 || !strings.Contains(inner, he) || !strings.Contains(inner, en) {
		t.Fatalf("pair runs missing: %s", inner)
	}
	if first == "en" && enAt > heAt {
		t.Fatal("english shell pair does not lead with English")
	}
	if first == "he" && heAt > enAt {
		t.Fatal("hebrew shell pair does not lead with Hebrew")
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
