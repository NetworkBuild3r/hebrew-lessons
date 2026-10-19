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
		{"today", "/?lang=en", "/?lang=he"},
		{"review", "/review?lang=en", "/review?lang=he"},
		{"progress", "/progress?lang=en", "/progress?lang=he"},
		{"lesson", "/lesson?lang=en", "/lesson?lang=he"},
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
	blank := views.NewPage("", "", "today")
	if blank.Lang != "en" || blank.LangLabel() != "עב" || blank.LangHref() != "/?lang=he" {
		t.Fatalf("empty lang href %q label %q lang %q", blank.LangHref(), blank.LangLabel(), blank.Lang)
	}
}

func TestLangLink(t *testing.T) {
	pages := []struct {
		path  string
		href  string
		lang  string
		dir   string
		label string
	}{
		{"/", "/?lang=he", "he", "rtl", "עב"},
		{"/?lang=en", "/?lang=he", "he", "rtl", "עב"},
		{"/?lang=he", "/?lang=en", "en", "ltr", "EN"},
		{"/review", "/review?lang=he", "he", "rtl", "עב"},
		{"/review?lang=en", "/review?lang=he", "he", "rtl", "עב"},
		{"/review?lang=he", "/review?lang=en", "en", "ltr", "EN"},
		{"/progress", "/progress?lang=he", "he", "rtl", "עב"},
		{"/progress?lang=en", "/progress?lang=he", "he", "rtl", "עב"},
		{"/progress?lang=he", "/progress?lang=en", "en", "ltr", "EN"},
		{"/lesson", "/lesson?lang=he", "he", "rtl", "עב"},
		{"/lesson?lang=en", "/lesson?lang=he", "he", "rtl", "עב"},
		{"/lesson?lang=he", "/lesson?lang=en", "en", "ltr", "EN"},
	}
	h := New(nil, nil)
	for _, page := range pages {
		assertLangLinks(t, get(t, h, page.path), page.href, page.lang, page.dir, page.label)
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
