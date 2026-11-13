package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The Verbs pages teach the seven patterns from the signed course list, and the
// customs reference covers what Isaiah leans on. INIT-040.

func TestVerbsIndexTeachesTheSevenPatterns(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-verbs", now)
	body := getLearner(t, h, l, "/learn/verbs")

	for _, want := range []string{
		"How Hebrew verbs work", "The seven patterns", "The three roots trick",
		"pa&#39;al", "nif&#39;al", "pi&#39;el", "hitpa&#39;el", "hif&#39;il",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("verbs index missing %q", want)
		}
	}
	if n := strings.Count(body, `class="binyan-card"`); n != 7 {
		t.Errorf("%d binyan cards, want 7", n)
	}
	// Every pattern card names a real course verb and says how many there are.
	if !strings.Contains(body, "in your course") {
		t.Error("no pattern says how many verbs it has")
	}
}

// A verb page shows forms that the course row holds, and the rule tables.
func TestVerbPageShowsRealForms(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-verb", now)
	body := getLearner(t, h, l, "/learn/verbs/word/כתב")

	for _, want := range []string{"to write", "Past tense", "Present tense", "Future tense", "verb-form"} {
		if !strings.Contains(body, want) {
			t.Errorf("verb page missing %q", want)
		}
	}
	// A form the course row holds.
	if !strings.Contains(body, "כתבתי") {
		t.Error("verb page does not show the form the course holds")
	}
	// The page says plainly where the forms come from.
	if !strings.Contains(body, "course list holds") {
		t.Error("verb page does not say where the forms come from")
	}
}

func TestVerbPageNotFound(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-verb404", now)
	// A noun is not a verb, so its page does not exist.
	rec := httptest.NewRequest(http.MethodGet, "/learn/verbs/word/שלום", nil)
	rec.AddCookie(l.cookie)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, rec)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("a noun was served as a verb page: %d", rr.Code)
	}
}

// The customs reference is grouped, is reference only, and never grades.
func TestCustomsPageCoversIsaiahsWorld(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-customs", now)
	body := getLearner(t, h, l, "/learn/customs")

	for _, want := range []string{
		"The ways of the Jews", "Time", "The Temple", "The Prophets", "Words",
		"Shabbat", "Pesaḥ", "Tish&#39;a b&#39;Av", "The Temple", "Navi", "Tzedek",
		"Nothing here is graded",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("customs page missing %q", want)
		}
	}
	if n := strings.Count(body, `class="custom-card"`); n < 15 {
		t.Errorf("%d customs, want at least 15", n)
	}
	// A practice that Isaiah does not use must not be invented as one.
	if strings.Contains(body, "In Isaiah: .") {
		t.Error("a custom claims Isaiah with no reason")
	}
}

// Every Learn guide the index lists can be opened.
func TestLearnIndexLinksTheNewGuides(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-learn", now)
	body := getLearner(t, h, l, "/learn")
	for _, href := range []string{`href="/learn/alefbet"`, `href="/learn/vowels"`, `href="/learn/verbs"`, `href="/learn/customs"`} {
		if !strings.Contains(body, href) {
			t.Errorf("learn index missing %s", href)
		}
	}
	for _, path := range []string{"/learn/alefbet", "/learn/vowels", "/learn/verbs", "/learn/customs"} {
		page := getLearner(t, h, l, path)
		if len(page) < 500 {
			t.Errorf("%s is nearly empty", path)
		}
	}
}
