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

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/gate"
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
		`מילים — Words · דֶּרֶךְ עִבְרִית`,
		`role="status"`,
		`aria-atomic="true"`,
		`aria-busy="false"`,
		`hx-on::send-error`,
		`hx-on::before-swap="wordsFail(event)"`,
		`id="words-error-network"`,
		`id="words-error-csrf"`,
		`wordsFail`,
		`/login/?next=`,
		`=== 403`,
		`השינוי לא נשמר. רעננו את הדף ונסו שוב.`,
		`type="submit" form="words-filters"`,
		`icon-close`,
		`Loading…`,
		`[\u0591-\u05BD\u05BF\u05C1\u05C2\u05C4\u05C5\u05C7]`,
		`· Solid`,
		`class="words-row__chips"`,
	}
	for _, want := range checks {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s", want)
		}
	}
	status := html[strings.Index(html, `id="words-status"`):]
	statusEnd := strings.Index(status, "</p>")
	if strings.Contains(status[:statusEnd], `role="alert"`) {
		t.Fatal("result counter is an alert")
	}
	if strings.Contains(html, `[\u0591-\u05C7]`) {
		t.Fatal("vowel strip still deletes the maqaf")
	}
	if strings.Contains(html, `words-skeleton words-list`) || strings.Contains(html, "wordsNetError") {
		t.Fatal("skeleton still uses the list grid, or the old network script is back")
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
	if strings.Contains(html, "onclick=") || strings.Contains(html, "wordsOffline") {
		t.Fatal("inline onclick or the old offline helper")
	}
	if !strings.Contains(html, `class="empty-title"`) || !strings.Contains(html, `class="empty-he"`) {
		t.Fatal("error title lost its Hebrew line or its bilingual title")
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
	detail := body(open)
	if open.Code != http.StatusOK || !strings.Contains(detail, "Hebrew → English") || !strings.Contains(detail, `data-binyan="hifil"`) {
		t.Fatalf("detail %d", open.Code)
	}
	if !strings.Contains(detail, "Selected word") || !strings.Contains(detail, "How well you know it") || !strings.Contains(detail, ">Shaky<") {
		t.Fatal("detail kicker, heading, or strength casing")
	}
	if strings.Contains(detail, ">not yet checked<") {
		t.Fatal("duplicate unchecked label")
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
	if cur.Code != http.StatusFound || cur.Header().Get("Location") != "/words/" {
		t.Fatalf("bad cursor %d %q", cur.Code, cur.Header().Get("Location"))
	}
	hx := get(t, mux, "/words/?cursor="+url.QueryEscape("%%%"), true)
	if hx.Code != http.StatusOK || hx.Header().Get("HX-Push-Url") != "/words/" {
		t.Fatalf("hx cursor %d %q", hx.Code, hx.Header().Get("HX-Push-Url"))
	}
	if !strings.Contains(body(hx), `data-word="שָׁלוֹם"`) || strings.Contains(body(hx), `data-words-error`) {
		t.Fatal("bad cursor did not show page 1")
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
	solid := "solid"
	rec := &recordLister{items: []library.Item{{
		Word: "שָׁלוֹם", GlossEN: "peace", HeEN: &library.Direction{Stage: "new", StrengthBucket: &solid},
		StrengthBucket: &solid,
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

func TestSuspendAndResume(t *testing.T) {
	const key = "שָׁלוֹם"
	demo := library.NewDemo()
	mux := http.NewServeMux()
	NewWords(demo).Register(mux)

	open := body(get(t, mux, "/words/?word="+url.QueryEscape(key), false))
	if !strings.Contains(open, `data-words-suspend`) || !strings.Contains(open, ">Pause reviews<") {
		t.Fatal("suspend control missing")
	}
	if strings.Contains(open, "data-words-add") || strings.Contains(open, "name=\"gloss\"") {
		t.Fatal("add or gloss edit shipped")
	}

	paused := postSuspend(t, mux, key, "1", true, "")
	if paused.Code != http.StatusOK {
		t.Fatalf("suspend %d %s", paused.Code, paused.Body.String())
	}
	html := paused.Body.String()
	if paused.Header().Get("HX-Push-Url") != "/words/?word="+url.QueryEscape(key) {
		t.Fatalf("push %q", paused.Header().Get("HX-Push-Url"))
	}
	if !strings.Contains(html, ">Resume reviews<") || !strings.Contains(html, `data-suspended="true"`) || !strings.Contains(html, ">Paused<") {
		t.Fatalf("paused card:\n%s", html)
	}
	if !strings.Contains(html, "Paused — it won’t come up in reviews") || !strings.Contains(html, "מושהית — לא תופיע בחזרות") {
		t.Fatal("paused detail line")
	}
	if !strings.Contains(html, `data-words-suspend`) || strings.Contains(html, `lookup-card__he" lang="he" dir="rtl" tabindex="-1" autofocus`) {
		t.Fatal("focus did not stay on the pause button")
	}
	resumed := postSuspend(t, mux, key, "0", true, "")
	if resumed.Code != http.StatusOK || !strings.Contains(resumed.Body.String(), ">Pause reviews<") {
		t.Fatalf("resume %d", resumed.Code)
	}
	row := rowSnippet(resumed.Body.String(), key)
	if strings.Contains(row, `data-suspended="true"`) || !strings.Contains(row, `data-suspended="false"`) {
		t.Fatalf("resume left the row paused: %s", row)
	}
}

func rowSnippet(html, key string) string {
	idx := strings.Index(html, `data-word="`+key+`"`)
	if idx < 0 {
		return ""
	}
	end := strings.Index(html[idx:], "</button>")
	if end < 0 {
		return html[idx:]
	}
	return html[idx : idx+end]
}

func TestSuspendWithoutJSRedirects(t *testing.T) {
	const key = "סֵפֶר"
	mux := http.NewServeMux()
	NewWords(library.NewDemo()).Register(mux)
	rec := postSuspend(t, mux, key, "1", false, "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/words/?word="+url.QueryEscape(key) {
		t.Fatalf("location %q", rec.Header().Get("Location"))
	}
}

func TestSuspendStaysOpenUnderActiveOnly(t *testing.T) {
	const key = "בַּיִת"
	mux := http.NewServeMux()
	NewWords(library.NewDemo()).Register(mux)
	form := url.Values{}
	form.Set("pause", "1")
	form.Set("suspended", "0")
	req := httptest.NewRequest(http.MethodPost, "/_hx/words/"+url.PathEscape(key)+"/suspend", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d %s", rr.Code, rr.Body.String())
	}
	html := rr.Body.String()
	if rowSnippet(html, key) != "" {
		t.Fatal("active-only list still showed the paused row")
	}
	if !strings.Contains(html, "Paused — it won’t come up in reviews") || !strings.Contains(html, ">Resume reviews<") {
		t.Fatal("detail closed when the row left the list")
	}
}

func TestExpiredSessionIsUnauthorized(t *testing.T) {
	h := NewWords(library.NewDemo())
	h.user = func(*http.Request) (int64, error) { return 0, nil }
	mux := http.NewServeMux()
	h.Register(mux)
	rec := postSuspend(t, mux, "סֵפֶר", "1", true, "")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"code":"no_session"`) {
		t.Fatalf("session %d %s", rec.Code, rec.Body.String())
	}
}

func TestSuspendNotFoundIsDisabled(t *testing.T) {
	const key = "אֹכֶל"
	demo := library.NewDemo()
	mux := http.NewServeMux()
	NewWords(demo).Register(mux)
	rec := postSuspend(t, mux, key, "1", true, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
	html := rec.Body.String()
	if !strings.Contains(html, `data-words-suspend-state="disabled"`) || !strings.Contains(html, "nothing to pause") {
		t.Fatalf("disabled state missing:\n%s", html)
	}
	if !strings.Contains(html, "עוד לא תרגלתם את המילה הזאת, אז אין מה להשהות.") || strings.Contains(html, "data-words-suspend disabled") {
		t.Fatal("can't-pause note")
	}
	if !strings.Contains(html, "disabled") || strings.Contains(html, `name="pause"`) {
		t.Fatal("paused control was still a form")
	}
	items, _, err := demo.List(context.Background(), library.DemoUserID, library.Filter{}, "", library.PageSize)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Word == key && item.Suspended {
			t.Fatal("not_found still paused the word")
		}
	}
}

func TestSuspendCSRFAndSession(t *testing.T) {
	const key = "שָׁלוֹם"
	demo := library.NewDemo()
	h := NewWords(demo)
	h.UseDevGate()
	mux := http.NewServeMux()
	h.Register(mux)

	page := body(get(t, mux, "/words/?word="+url.QueryEscape(key), false))
	if !strings.Contains(page, gate.DevToken) || !strings.Contains(page, "X-CSRF-Token") {
		t.Fatal("csrf header was not rendered")
	}
	denied := postSuspend(t, mux, key, "1", true, "")
	if denied.Code != http.StatusForbidden || !strings.Contains(denied.Body.String(), `"code":"csrf"`) {
		t.Fatalf("csrf %d %s", denied.Code, denied.Body.String())
	}
	ok := postSuspend(t, mux, key, "1", true, gate.DevToken)
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), ">Resume reviews<") {
		t.Fatalf("accepted %d %s", ok.Code, ok.Body.String())
	}

	bare := NewWords(library.NewDemo())
	bare.Require = gate.RequireSession
	session := http.NewServeMux()
	bare.Register(session)
	hx := postSuspend(t, session, key, "1", true, gate.DevToken)
	if hx.Code != http.StatusUnauthorized || !strings.Contains(hx.Body.String(), `"code":"no_session"`) {
		t.Fatalf("session %d %s", hx.Code, hx.Body.String())
	}
	req := httptest.NewRequest(http.MethodPost, "/_hx/words/"+url.PathEscape(key)+"/suspend", strings.NewReader("pause=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	rr := httptest.NewRecorder()
	session.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound || !strings.HasPrefix(rr.Header().Get("Location"), "/login/?next=") {
		t.Fatalf("login redirect %d %q", rr.Code, rr.Header().Get("Location"))
	}
}

func postSuspend(t *testing.T, mux *http.ServeMux, key, pause string, hx bool, token string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("pause", pause)
	req := httptest.NewRequest(http.MethodPost, "/_hx/words/"+url.PathEscape(key)+"/suspend", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if hx {
		req.Header.Set("HX-Request", "true")
		req.Header.Set("Accept", "text/html")
	}
	if token != "" {
		req.Header.Set("X-CSRF-Token", token)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
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
