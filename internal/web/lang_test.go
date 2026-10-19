package web

import (
	"regexp"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func TestLangHrefAndLabel(t *testing.T) {
	screens := []struct {
		current string
		heHref  string
		enHref  string
	}{
		{"today", "/?lang=en", "/"},
		{"review", "/review?lang=en", "/review"},
		{"progress", "/progress?lang=en", "/progress"},
		{"lesson", "/lesson?lang=en", "/lesson"},
	}
	for _, screen := range screens {
		he := views.NewPage("he", "", screen.current)
		if he.LangHref() != screen.heHref {
			t.Errorf("he %s href %q, want %q", screen.current, he.LangHref(), screen.heHref)
		}
		if he.LangLabel() != "EN" {
			t.Errorf("he %s label %q", screen.current, he.LangLabel())
		}
		en := views.NewPage("en", "", screen.current)
		if en.LangHref() != screen.enHref {
			t.Errorf("en %s href %q, want %q", screen.current, en.LangHref(), screen.enHref)
		}
		if en.LangLabel() != "עב" {
			t.Errorf("en %s label %q", screen.current, en.LangLabel())
		}
	}
}

func TestLangLink(t *testing.T) {
	pages := []struct {
		hePath string
		enPath string
		heHref string
		enHref string
	}{
		{"/", "/?lang=en", "/?lang=en", "/"},
		{"/review", "/review?lang=en", "/review?lang=en", "/review"},
		{"/progress", "/progress?lang=en", "/progress?lang=en", "/progress"},
		{"/lesson", "/lesson?lang=en", "/lesson?lang=en", "/lesson"},
	}
	h := New(nil, nil)
	for _, page := range pages {
		assertLangLinks(t, get(t, h, page.hePath), page.heHref, "en", "ltr", "EN")
		assertLangLinks(t, get(t, h, page.enPath), page.enHref, "he", "rtl", "עב")
	}
}

func assertLangLinks(t *testing.T, body, href, lang, dir, label string) {
	t.Helper()
	header := regexp.MustCompile(regexp.QuoteMeta(
		`<a class="lang-switch" href="` + href + `"><bdi lang="` + lang + `" dir="` + dir + `">` + label + `</bdi></a>`,
	))
	rail := regexp.MustCompile(regexp.QuoteMeta(
		`<a class="lang-switch rail-lang" href="` + href + `"><bdi lang="` + lang + `" dir="` + dir + `">` + label + `</bdi></a>`,
	))
	if !header.MatchString(body) {
		t.Fatalf("header lang link missing for href %s label %s", href, label)
	}
	if !rail.MatchString(body) {
		t.Fatalf("rail lang link missing for href %s", href)
	}
}
