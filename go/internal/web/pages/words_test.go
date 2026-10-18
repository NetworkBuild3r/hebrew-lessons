package pages

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/library"
)

func wordsMux(lib library.Lister) *http.ServeMux {
	mux := http.NewServeMux()
	NewWords(lib).Register(mux)
	return mux
}

func TestWordAndCursorDoNotCollideWithFilterKeys(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	js, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "shared", "words-page.js"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`var FILTER_KEYS = \[([^\]]+)\]`).FindSubmatch(js)
	if m == nil {
		t.Fatal("FILTER_KEYS missing from words-page.js")
	}
	body := string(m[1])
	for _, key := range []string{"word", "cursor"} {
		if strings.Contains(body, "'"+key+"'") || strings.Contains(body, `"`+key+`"`) {
			t.Fatalf("%s collides with FILTER_KEYS: %s", key, body)
		}
	}
	for _, key := range []string{"q", "status", "weak", "pos", "binyan", "tense", "source", "suspended"} {
		if !strings.Contains(body, "'"+key+"'") {
			t.Fatalf("FILTER_KEYS lost %s: %s", key, body)
		}
	}
}

func TestWordsPageHooks(t *testing.T) {
	rec := get(t, wordsMux(library.Demo{}), "/words/", false)
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	html := body(rec)
	checks := []string{
		`class="hebrew-spa has-hebrew-shell words-page"`,
		`data-route-ready="words"`,
		`data-words-state="ready"`,
		`form class="words-filters" id="words-filters"`,
		`data-filter="q"`,
		`data-filter="binyan"`,
		`id="words-filter-binyan"`,
		`data-filter="weak"`,
		`id="words-vowels-btn"`,
		`data-words-review`,
		`data-words-list`,
		`data-words-detail`,
		`class="word-item"`,
		`data-word="שָׁלוֹם"`,
		`data-gloss`,
		`data-speak=`,
		`<bdi class="words-he" lang="he" dir="rtl"`,
		`delay:300ms`,
		`src="/shared/vendor/htmx.min.js"`,
		`responseHandling`,
		`lang="en" dir="ltr"`,
		`root.dir = lang === 'he' ? 'rtl' : 'ltr'`,
		`Learning`,
		`Long-term`,
		`Paused`,
		`data-strength="fragile"`,
		`data-strength="shaky"`,
		`data-strength="steady"`,
		`data-strength="solid"`,
	}
	for _, want := range checks {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(html, "data-words-add") || strings.Contains(html, "data-words-suspend") {
		t.Fatal("write controls shipped in the read-only page")
	}
	if strings.Contains(html, "pr2-badge") || strings.Contains(html, "streak") || strings.Contains(html, "XP") {
		t.Fatal("badge, streak, or XP rendered")
	}
	if strings.Contains(html, ">Due<") || strings.Contains(html, ">לחזרה<") {
		t.Fatal("Due chip rendered")
	}
	// The row button must not wrap the audio button.
	row := html[strings.Index(html, `class="words-row"`):]
	end := strings.Index(row, "</button>")
	if strings.Contains(row[:end], "audio-btn") {
		t.Fatal("audio button is inside the row button")
	}
	if strings.Contains(html, "https://") || strings.Contains(html, "cdn.") {
		t.Fatal("page references a CDN")
	}
}

func TestBinyanFilterSetsTheReviewDeck(t *testing.T) {
	rec := get(t, wordsMux(library.Demo{}), "/words/?binyan=hifil", false)
	html := body(rec)
	if !strings.Contains(html, `data-review-deck="concept:binyan:hifil"`) {
		t.Fatalf("deck missing:\n%s", html)
	}
	if !strings.Contains(html, `href="/review/?deck=concept%3Abinyan%3Ahifil"`) {
		t.Fatal("review href")
	}
	if strings.Contains(html, `data-word="שָׁלוֹם"`) {
		t.Fatal("noun survived the hifil filter")
	}
	if !strings.Contains(html, `data-binyan="hifil"`) {
		t.Fatal("binyan attribute")
	}
	open := get(t, wordsMux(library.Demo{}), "/words/?binyan=hifil&word="+url.QueryEscape("הִסְבִּיר"), false)
	if !strings.Contains(body(open), "not yet checked") || !strings.Contains(body(open), `data-cell="1s"`) {
		t.Fatal("detail paradigm")
	}
}

func TestCanonicalRedirectAndHXPush(t *testing.T) {
	mux := wordsMux(library.Demo{})
	rec := get(t, mux, "/words/?status=&weak=0&q=peace", false)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/words/?q=peace" {
		t.Fatalf("redirect %d %q", rec.Code, rec.Header().Get("Location"))
	}
	kept := get(t, mux, "/words/?suspended=0", false)
	if kept.Code != http.StatusOK {
		t.Fatalf("suspended=0 should stay, got %d %s", kept.Code, kept.Header().Get("Location"))
	}
	hx := get(t, mux, "/words/?binyan=hifil&q=explained", true)
	if hx.Code != http.StatusOK {
		t.Fatal(hx.Code)
	}
	if hx.Header().Get("HX-Push-Url") != "/words/?q=explained&binyan=hifil" {
		t.Fatalf("push %q", hx.Header().Get("HX-Push-Url"))
	}
	if strings.Contains(body(hx), "<!DOCTYPE") {
		t.Fatal("fragment included the document")
	}
	if !strings.Contains(body(hx), `id="words-results"`) {
		t.Fatal("fragment missing the swap root")
	}
	same := get(t, mux, "/words/?q=peace", true)
	if same.Header().Get("HX-Push-Url") != "" {
		t.Fatal("pushed an already canonical URL")
	}
}

func TestInvalidFilterAndLoadError(t *testing.T) {
	mux := wordsMux(library.Demo{})
	bad := get(t, mux, "/words/?status=nope", false)
	if bad.Code != http.StatusBadRequest || !strings.Contains(body(bad), `data-words-error="invalid_filter"`) {
		t.Fatalf("invalid %d", bad.Code)
	}
	if strings.Contains(body(bad), "Try again") {
		t.Fatal("invalid filter offered Try again")
	}
	if !strings.Contains(body(bad), "Clear filters") {
		t.Fatal("clear missing")
	}
	broken := wordsMux(errLister{})
	fail := get(t, broken, "/words/", false)
	if fail.Code != http.StatusInternalServerError || !strings.Contains(body(fail), `data-words-error="load"`) {
		t.Fatalf("load %d\n%s", fail.Code, body(fail))
	}
	if !strings.Contains(body(fail), "Try again") {
		t.Fatal("load error missing Try again")
	}
	cur := get(t, mux, "/words/?cursor="+url.QueryEscape("%%%"), false)
	if cur.Code != http.StatusBadRequest || !strings.Contains(body(cur), `data-words-error="load"`) {
		t.Fatalf("cursor %d", cur.Code)
	}
}

func TestEmptyAndNoResults(t *testing.T) {
	empty := body(get(t, wordsMux(emptyLister{}), "/words/", false))
	if !strings.Contains(empty, `data-words-empty="library"`) || !strings.Contains(empty, "No words yet") {
		t.Fatal("empty library")
	}
	if strings.Contains(empty, "data-words-review") {
		t.Fatal("review link on an empty library")
	}
	if !strings.Contains(empty, "empty-words.svg") {
		t.Fatal("illustration")
	}
	none := body(get(t, wordsMux(library.Demo{}), "/words/?q=zzzz", false))
	if !strings.Contains(none, `data-words-empty="filter"`) || !strings.Contains(none, "<bdi>zzzz</bdi>") {
		t.Fatal("no results")
	}
	if !strings.Contains(none, "Clear filters") {
		t.Fatal("clear filters")
	}
}

func TestLoadMoreFragmentAndRecordedListCall(t *testing.T) {
	rec := &recordLister{items: []library.Item{{
		Word: "שָׁלוֹם", GlossEN: "peace", HeEn: &library.Direction{Stage: "new", StrengthBucket: "solid"},
		StrengthBucket: "solid",
	}}, next: "opaque-cursor"}
	mux := wordsMux(rec)
	page := body(get(t, mux, "/words/", false))
	if !strings.Contains(page, `data-words-more`) || !strings.Contains(page, "fragment=rows") {
		t.Fatal("load more link")
	}
	if rec.limit != library.PageSize || rec.user != library.DemoUserID || rec.cursor != "" {
		t.Fatalf("list call limit %d user %d cursor %q", rec.limit, rec.user, rec.cursor)
	}
	frag := body(get(t, mux, "/words/?cursor=opaque-cursor&fragment=rows", true))
	if strings.Contains(frag, "<!DOCTYPE") || strings.Contains(frag, `id="words-filters"`) {
		t.Fatal("rows fragment included the page")
	}
	if !strings.Contains(frag, `data-word="שָׁלוֹם"`) {
		t.Fatal(frag)
	}
	if rec.cursor != "opaque-cursor" {
		t.Fatalf("cursor not passed through: %q", rec.cursor)
	}
}

func TestChipSyncReturnsTheFilterBar(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/words/?weak=very_weak", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("X-Words-Sync", "1")
	rr := httptest.NewRecorder()
	wordsMux(library.Demo{}).ServeHTTP(rr, req)
	html := rr.Body.String()
	if strings.Count(html, `id="words-filters"`) != 1 || !strings.Contains(html, `hx-swap-oob="outerHTML"`) {
		t.Fatal("filter bar was not returned out of band")
	}
	if !strings.Contains(html, `value="very_weak"`) {
		t.Fatal("weak value did not round-trip")
	}
}

func TestStageChipUsesTheWeakerDirection(t *testing.T) {
	html := body(get(t, wordsMux(library.Demo{}), "/words/", false))
	// בַּיִת is learning in he_en and new in en_he, so the chip is New and data-stage stays he_en.
	idx := strings.Index(html, `data-word="בַּיִת"`)
	if idx < 0 {
		t.Fatal("bayit")
	}
	end := strings.Index(html[idx:], "</button>")
	if end < 0 {
		t.Fatal("row button")
	}
	snippet := html[idx : idx+end]
	if !strings.Contains(snippet, `data-stage="learning"`) || !strings.Contains(snippet, ">New<") {
		t.Fatalf("stage chip: %s", snippet)
	}
}

func TestHtmxPin(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "shared", "vendor")
	ver, err := os.ReadFile(filepath.Join(root, "htmx.version"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(ver)) != "2.0.10" {
		t.Fatalf("version %q", ver)
	}
	js, err := os.ReadFile(filepath.Join(root, "htmx.min.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), "2.0.10") || !strings.Contains(string(js), "responseHandling") {
		t.Fatal("vendored htmx is not 2.0.10")
	}
	lic, err := os.ReadFile(filepath.Join(root, "htmx.LICENSE"))
	if err != nil || !strings.Contains(string(lic), "Zero-Clause BSD") {
		t.Fatal("license missing")
	}
}

type emptyLister struct{}

func (emptyLister) List(context.Context, int64, library.Filter, string, int) ([]library.Item, string, error) {
	return nil, "", nil
}

type errLister struct{}

func (errLister) List(context.Context, int64, library.Filter, string, int) ([]library.Item, string, error) {
	return nil, "", errors.New("fixture down")
}

type recordLister struct {
	items  []library.Item
	next   string
	user   int64
	cursor string
	limit  int
}

func (r *recordLister) List(_ context.Context, userID int64, _ library.Filter, cursor string, limit int) ([]library.Item, string, error) {
	r.user = userID
	r.cursor = cursor
	r.limit = limit
	return r.items, r.next, nil
}
