package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
	"golang.org/x/text/unicode/norm"
)

type placedUser struct {
	id     int64
	csrf   string
	cookie *http.Cookie
}

func newPlacementRouter(t *testing.T) (http.Handler, *db.Store, placedUser, placedUser) {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedAudio(t, store)
	a := insertPlacedUser(t, store, "a@example.com", "sub-a", "csrf-a", now)
	b := insertPlacedUser(t, store, "b@example.com", "sub-b", "csrf-b", now)
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		CookieSecure:  false,
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{"a@example.com", "b@example.com", "c@example.com", "d@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	deck := placement.New(store, fsrs.New(store, clk), clk)
	return NewWithPlacement(nil, svc, deck, store), store, a, b
}

func insertPlacedUser(t *testing.T, store *db.Store, email, sub, csrf string, now time.Time) placedUser {
	t.Helper()
	user, err := store.CreateUser(context.Background(), email, sub, auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 32)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(id)
	if err := store.InsertSession(context.Background(), hex.EncodeToString(sum[:]), user.ID, csrf, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	raw := base64.RawURLEncoding.EncodeToString(id)
	return placedUser{id: user.ID, csrf: csrf, cookie: &http.Cookie{Name: "hebrew_session", Value: raw}}
}

func seedAudio(t *testing.T, store *db.Store) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "placement", "items.json"))
	if err != nil {
		t.Fatal(err)
	}
	var items []struct {
		Text string `json:"text_nfc"`
	}
	if err := json.Unmarshal(b, &items); err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		sum := sha256.Sum256([]byte("placement-audio:" + it.Text))
		_, err := store.InsertAudio(context.Background(), db.Asset{
			ID:      hex.EncodeToString(sum[:]),
			TextNFC: it.Text,
			Voice:   "test",
			Model:   "legacy",
			Bytes:   1,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func (u placedUser) post(t *testing.T, h http.Handler, path string, form url.Values, token string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(form.Encode())
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Host = "learn.example"
	req.Header.Set("Origin", "https://learn.example")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(u.cookie)
	if token != "" {
		req.Header.Set("X-CSRF-Token", token)
	}
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func hidden(t *testing.T, body, name string) string {
	t.Helper()
	re := regexp.MustCompile(`name="` + regexp.QuoteMeta(name) + `" value="([^"]*)"`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("hidden %s missing in %s", name, body)
	}
	return m[1]
}

func TestPlacementGateAndFragment(t *testing.T) {
	h, _, a, _ := newPlacementRouter(t)
	en := views.PlacementFor("en")

	anon := httptest.NewRecorder()
	h.ServeHTTP(anon, httptest.NewRequest(http.MethodGet, "/placement", nil))
	if anon.Code != http.StatusSeeOther || anon.Header().Get("Location") != "/signin" {
		t.Fatalf("signed-out GET = %d %q", anon.Code, anon.Header().Get("Location"))
	}
	if strings.Contains(anon.Body.String(), en.Heading) {
		t.Fatal("signed-out GET rendered the page")
	}

	hxReq := httptest.NewRequest(http.MethodGet, "/placement", nil)
	hxReq.Header.Set("HX-Request", "true")
	hx := httptest.NewRecorder()
	h.ServeHTTP(hx, hxReq)
	if hx.Code != http.StatusUnauthorized || hx.Header().Get("HX-Redirect") != "/signin" {
		t.Fatalf("htmx GET = %d %q", hx.Code, hx.Header().Get("HX-Redirect"))
	}
	if hx.Header().Get("Location") != "" || strings.Contains(hx.Body.String(), `id="placement"`) {
		t.Fatal("htmx sign-in leaked a fragment or a Location")
	}

	hxHe := httptest.NewRequest(http.MethodGet, "/placement?lang=he", nil)
	hxHe.Header.Set("HX-Request", "true")
	hxHeRec := httptest.NewRecorder()
	h.ServeHTTP(hxHeRec, hxHe)
	if hxHeRec.Code != http.StatusUnauthorized || hxHeRec.Header().Get("HX-Redirect") != "/signin?lang=he" {
		t.Fatalf("htmx he = %d %q", hxHeRec.Code, hxHeRec.Header().Get("HX-Redirect"))
	}

	page := getSignedIn(t, h, a.cookie, "/placement")
	if !strings.Contains(page, `<html lang="en" dir="ltr"`) || !strings.Contains(page, en.Heading) || !strings.Contains(page, en.Start) {
		t.Fatal("signed-in placement intro is not English")
	}
	if strings.Contains(page, `aria-current="page"`) {
		t.Fatal("placement should not mark a dock item current")
	}
	if strings.Count(page, "<script") != 3 {
		t.Fatalf("placement page scripts = %d", strings.Count(page, "<script"))
	}
	if left := hebrewExcept(page, []string{views.AppName, "ע"}); left != "" {
		t.Fatalf("english intro has Hebrew UI %q", left)
	}
	if !strings.Contains(page, `hx-post="/_hx/placement/start"`) || !strings.Contains(page, `hx-select="unset"`) {
		t.Fatal("start form is not an htmx write")
	}

	full := a.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {a.csrf}}, "", false)
	if full.Code != http.StatusOK {
		t.Fatalf("full start = %d %s", full.Code, full.Body.String())
	}
	fullBody := full.Body.String()
	if !strings.Contains(fullBody, "<html") || !strings.Contains(fullBody, `class="dock"`) {
		t.Fatal("non-htmx write did not return a full page")
	}
	assertFront(t, fullBody, en)
}

func TestPlacementFragmentVersusPage(t *testing.T) {
	h, _, _, b := newPlacementRouter(t)
	en := views.PlacementFor("en")
	frag := b.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {b.csrf}}, "", true)
	if frag.Code != http.StatusOK {
		t.Fatalf("fragment start = %d %s", frag.Code, frag.Body.String())
	}
	body := frag.Body.String()
	if strings.Contains(body, "<html") || strings.Contains(body, `class="dock"`) || strings.Contains(body, "<script") {
		t.Fatal("htmx write returned a full page")
	}
	if !strings.Contains(body, `id="placement"`) {
		t.Fatal("fragment is missing #placement")
	}
	assertFront(t, body, en)
}

func TestPlacementCSRF(t *testing.T) {
	h, _, a, _ := newPlacementRouter(t)

	missing := a.post(t, h, "/_hx/placement/start", url.Values{}, "", false)
	if missing.Code != http.StatusForbidden || !strings.Contains(missing.Body.String(), "csrf") {
		t.Fatalf("missing token = %d %s", missing.Code, missing.Body.String())
	}
	bad := a.post(t, h, "/_hx/placement/start", url.Values{}, "not-the-token", false)
	if bad.Code != http.StatusForbidden {
		t.Fatalf("bad token = %d", bad.Code)
	}
	foreign := httptest.NewRequest(http.MethodPost, "/_hx/placement/start", strings.NewReader(""))
	foreign.Host = "learn.example"
	foreign.Header.Set("Origin", "https://evil.example")
	foreign.Header.Set("X-CSRF-Token", a.csrf)
	foreign.AddCookie(a.cookie)
	foreignRec := httptest.NewRecorder()
	h.ServeHTTP(foreignRec, foreign)
	if foreignRec.Code != http.StatusForbidden {
		t.Fatalf("bad origin = %d", foreignRec.Code)
	}
	if strings.Contains(getSignedIn(t, h, a.cookie, "/"), "Resume placement") {
		t.Fatal("rejected writes opened a run")
	}

	form := a.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {a.csrf}}, "", true)
	if form.Code != http.StatusOK {
		t.Fatalf("form token = %d %s", form.Code, form.Body.String())
	}
	runID := hidden(t, form.Body.String(), "run_id")
	itemID := hidden(t, form.Body.String(), "item_id")
	header := a.post(t, h, "/_hx/placement/rate", url.Values{
		"run_id":  {runID},
		"item_id": {itemID},
		"rating":  {"3"},
	}, a.csrf, true)
	if header.Code != http.StatusOK || !strings.Contains(header.Body.String(), `class="wb__he heword"`) {
		t.Fatalf("header token = %d %s", header.Code, header.Body.String())
	}
}

func TestPlacementOwnership(t *testing.T) {
	h, _, a, b := newPlacementRouter(t)
	started := a.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {a.csrf}}, "", true)
	if started.Code != http.StatusOK {
		t.Fatalf("start = %d %s", started.Code, started.Body.String())
	}
	body := started.Body.String()
	runID := hidden(t, body, "run_id")
	itemID := hidden(t, body, "item_id")
	word := regexp.MustCompile(`<bdi class="wb__he heword" lang="he" dir="rtl"[^>]*>([^<]*)</bdi>`).FindStringSubmatch(body)
	if word == nil {
		t.Fatalf("word missing: %s", body)
	}

	stolen := b.post(t, h, "/_hx/placement/rate", url.Values{
		"run_id":  {runID},
		"item_id": {itemID},
		"rating":  {"3"},
		"_csrf":   {b.csrf},
	}, "", true)
	if stolen.Code != http.StatusForbidden {
		t.Fatalf("other user rate = %d %s", stolen.Code, stolen.Body.String())
	}
	if strings.Contains(stolen.Body.String(), word[1]) {
		t.Fatal("forbidden response included the card")
	}
	flipped := b.post(t, h, "/_hx/placement/flip", url.Values{
		"run_id":  {runID},
		"item_id": {itemID},
		"_csrf":   {b.csrf},
	}, "", true)
	if flipped.Code != http.StatusForbidden || strings.Contains(flipped.Body.String(), word[1]) {
		t.Fatalf("other user flip = %d %s", flipped.Code, flipped.Body.String())
	}
	keptGoing := b.post(t, h, "/_hx/placement/continue", url.Values{
		"run_id": {runID},
		"_csrf":  {b.csrf},
	}, "", true)
	if keptGoing.Code != http.StatusForbidden || strings.Contains(keptGoing.Body.String(), word[1]) {
		t.Fatalf("other user continue = %d %s", keptGoing.Code, keptGoing.Body.String())
	}
	finished := b.post(t, h, "/_hx/placement/finish", url.Values{
		"run_id": {runID},
		"_csrf":  {b.csrf},
	}, "", true)
	if finished.Code != http.StatusForbidden {
		t.Fatalf("other user finish = %d", finished.Code)
	}
	resumed := b.post(t, h, "/_hx/placement/start", url.Values{
		"run_id": {runID},
		"_csrf":  {b.csrf},
	}, "", true)
	if resumed.Code != http.StatusForbidden {
		t.Fatalf("other user start = %d %s", resumed.Code, resumed.Body.String())
	}
	if strings.Contains(getSignedIn(t, h, b.cookie, "/"), "Resume placement") {
		t.Fatal("rejected start opened a run for the other user")
	}

	missing := a.post(t, h, "/_hx/placement/rate", url.Values{
		"run_id":  {"999999"},
		"item_id": {itemID},
		"rating":  {"3"},
		"_csrf":   {a.csrf},
	}, "", true)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing run = %d", missing.Code)
	}

	kept := a.post(t, h, "/_hx/placement/rate", url.Values{
		"run_id":  {runID},
		"item_id": {itemID},
		"rating":  {"3"},
		"_csrf":   {a.csrf},
	}, "", true)
	if kept.Code != http.StatusOK {
		t.Fatalf("owner rate after rejection = %d %s", kept.Code, kept.Body.String())
	}
	if hidden(t, kept.Body.String(), "item_id") == itemID {
		t.Fatal("owner rating did not advance the deck")
	}
	hx := httptest.NewRequest(http.MethodGet, "/_hx/placement/start", nil)
	hx.AddCookie(a.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, hx)
	if rec.Code != http.StatusMethodNotAllowed || strings.Contains(rec.Body.String(), word[1]) {
		t.Fatalf("GET /_hx/placement/start = %d", rec.Code)
	}
}

func TestPlacementRateWritesOneCard(t *testing.T) {
	h, store, a, b := newPlacementRouter(t)
	started := a.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {a.csrf}}, "", true)
	if started.Code != http.StatusOK {
		t.Fatal(started.Body.String())
	}
	runID := hidden(t, started.Body.String(), "run_id")
	itemID := hidden(t, started.Body.String(), "item_id")
	if cards, logs := learnerMemory(t, store, a.id); cards != 0 || logs != 0 {
		t.Fatalf("start wrote memory: %d cards, %d logs", cards, logs)
	}

	stolen := b.post(t, h, "/_hx/placement/rate", url.Values{
		"run_id":  {runID},
		"item_id": {itemID},
		"rating":  {"3"},
		"_csrf":   {b.csrf},
	}, "", true)
	if stolen.Code != http.StatusForbidden {
		t.Fatalf("other user rate = %d", stolen.Code)
	}
	if cards, logs := learnerMemory(t, store, a.id); cards != 0 || logs != 0 {
		t.Fatalf("forbidden rate wrote the owner's memory: %d cards, %d logs", cards, logs)
	}
	if cards, logs := learnerMemory(t, store, b.id); cards != 0 || logs != 0 {
		t.Fatalf("forbidden rate wrote the other user: %d cards, %d logs", cards, logs)
	}

	rated := a.post(t, h, "/_hx/placement/rate", url.Values{
		"run_id":  {runID},
		"item_id": {itemID},
		"rating":  {"3"},
		"_csrf":   {a.csrf},
	}, "", true)
	if rated.Code != http.StatusOK {
		t.Fatalf("rate = %d %s", rated.Code, rated.Body.String())
	}
	if cards, logs := learnerMemory(t, store, a.id); cards != 1 || logs != 1 {
		t.Fatalf("one rating wrote %d cards and %d logs", cards, logs)
	}
}

func learnerMemory(t *testing.T, store *db.Store, userID int64) (cards, logs int) {
	t.Helper()
	var err error
	cards, err = store.CountCards(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	err = conn.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM review_log
		WHERE card_id IN (SELECT id FROM cards WHERE user_id = ?)`, userID).Scan(&logs)
	if err != nil {
		t.Fatal(err)
	}
	return cards, logs
}

func TestPlacementReadDoesNotDeal(t *testing.T) {
	h, _, a, _ := newPlacementRouter(t)
	started := a.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {a.csrf}}, "", false)
	if started.Code != http.StatusOK {
		t.Fatalf("start = %d %s", started.Code, started.Body.String())
	}
	itemID := hidden(t, started.Body.String(), "item_id")
	page := getSignedIn(t, h, a.cookie, "/placement")
	if strings.Contains(page, `name="item_id"`) || strings.Contains(page, itemID) {
		t.Fatal("GET /placement dealt or repeated the open card")
	}
	if !strings.Contains(page, views.PlacementFor("en").ResumeTitle) || !strings.Contains(page, `hx-post="/_hx/placement/start"`) {
		t.Fatal("open run should offer resume")
	}
	hx := httptest.NewRequest(http.MethodGet, "/_hx/placement/rate", nil)
	hx.AddCookie(a.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, hx)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /_hx/placement/rate = %d", rec.Code)
	}
}

func TestPlacementFinishAndBreak(t *testing.T) {
	h, _, a, _ := newPlacementRouter(t)
	en := views.PlacementFor("en")
	cur := a.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {a.csrf}}, "", true)
	if cur.Code != http.StatusOK {
		t.Fatalf("start = %d %s", cur.Code, cur.Body.String())
	}
	runID := hidden(t, cur.Body.String(), "run_id")
	done := a.post(t, h, "/_hx/placement/finish", url.Values{
		"run_id": {runID},
		"_csrf":  {a.csrf},
	}, "", false)
	if done.Code != http.StatusOK {
		t.Fatalf("finish = %d %s", done.Code, done.Body.String())
	}
	body := done.Body.String()
	if !strings.Contains(body, "<html") || !strings.Contains(body, en.Results) || !strings.Contains(body, en.StartHere) {
		t.Fatal("finish did not render the ladder")
	}
	if !strings.Contains(body, `href="/"`) || !strings.Contains(body, "added to your practice") || !strings.Contains(body, en.StartOver) {
		t.Fatal("results are missing the start unit, the practice line, or the way back")
	}
	if strings.Contains(body, `name="rating"`) {
		t.Fatal("results still show grade buttons")
	}
	today := getSignedIn(t, h, a.cookie, "/")
	if strings.Contains(today, en.TakeTitle) || strings.Contains(today, "Resume placement") {
		t.Fatal("a finished run still offers placement on Today")
	}
	again := getSignedIn(t, h, a.cookie, "/placement")
	if !strings.Contains(again, en.Results) || !strings.Contains(again, en.StartHere) {
		t.Fatal("GET after finish should show the stored summary")
	}
}

func TestPlacementSittingBreak(t *testing.T) {
	h, _, a, _ := newPlacementRouter(t)
	en := views.PlacementFor("en")
	cur := a.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {a.csrf}}, "", true)
	if cur.Code != http.StatusOK {
		t.Fatal(cur.Body.String())
	}
	for i := 0; i < 20; i++ {
		body := cur.Body.String()
		if strings.Contains(body, en.BreakTitle) {
			t.Fatalf("break arrived on card %d", i)
		}
		next := a.post(t, h, "/_hx/placement/rate", url.Values{
			"run_id":  {hidden(t, body, "run_id")},
			"item_id": {hidden(t, body, "item_id")},
			"rating":  {"3"},
			"_csrf":   {a.csrf},
		}, "", true)
		if next.Code != http.StatusOK {
			t.Fatalf("rate %d = %d %s", i, next.Code, next.Body.String())
		}
		cur = next
	}
	body := cur.Body.String()
	if !strings.Contains(body, html.EscapeString(en.BreakTitle)) || !strings.Contains(body, en.DoneToday) || !strings.Contains(body, en.KeepGoing) {
		t.Fatalf("sitting did not break: %s", body)
	}
	if !strings.Contains(body, `href="/"`) || strings.Contains(body, `hx-post="/_hx/placement/finish"`) {
		t.Fatal("done for today must leave the run open")
	}
	if strings.Contains(getSignedIn(t, h, a.cookie, "/"), "Take the placement check") || !strings.Contains(getSignedIn(t, h, a.cookie, "/"), "Resume placement") {
		t.Fatal("a break should leave the run in progress")
	}
	kept := a.post(t, h, "/_hx/placement/continue", url.Values{
		"run_id": {hidden(t, body, "run_id")},
		"_csrf":  {a.csrf},
	}, "", true)
	if kept.Code != http.StatusOK || !strings.Contains(kept.Body.String(), `class="wb__he heword"`) {
		t.Fatalf("continue = %d %s", kept.Code, kept.Body.String())
	}
}

func TestTodayPlacementCards(t *testing.T) {
	h, _, a, b := newPlacementRouter(t)
	en := views.PlacementFor("en")

	none := getSignedIn(t, h, a.cookie, "/")
	if !strings.Contains(none, `class="today-place today-place--open"`) || !strings.Contains(none, en.TakeTitle) {
		t.Fatal("empty placement should offer the check")
	}
	if !strings.Contains(none, `href="/placement"`) || strings.Contains(none, `href="/placement?lang=`) {
		t.Fatal("english placement link should be bare")
	}
	if !strings.Contains(none, `href="/review"`) || !strings.Contains(none, `class="ink-card`) {
		t.Fatal("placement card replaced Today")
	}
	if strings.Contains(none, "Resume placement") {
		t.Fatal("a new learner is offered resume")
	}

	heReq := httptest.NewRequest(http.MethodGet, "/", nil)
	heReq.AddCookie(b.cookie)
	heReq.AddCookie(&http.Cookie{Name: langCookieName, Value: "he"})
	heRec := httptest.NewRecorder()
	h.ServeHTTP(heRec, heReq)
	if heRec.Code != http.StatusOK {
		t.Fatalf("hebrew today = %d", heRec.Code)
	}
	he := heRec.Body.String()
	heCopy := views.PlacementFor("he")
	if !strings.Contains(he, heCopy.TakeTitle) || !strings.Contains(he, `href="/placement?lang=he"`) {
		t.Fatal("hebrew shell did not translate the placement card")
	}
	if strings.Contains(he, en.TakeTitle) {
		t.Fatal("hebrew card still uses the English label")
	}

	started := a.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {a.csrf}}, "", true)
	if started.Code != http.StatusOK {
		t.Fatal(started.Body.String())
	}
	progress := getSignedIn(t, h, a.cookie, "/")
	if !strings.Contains(progress, `class="today-place today-place--resume"`) || !strings.Contains(progress, "Resume placement") {
		t.Fatal("open run should offer resume")
	}
	if strings.Contains(progress, en.TakeTitle) {
		t.Fatal("open run still offers a new check")
	}
	if !strings.Contains(progress, `href="/review"`) {
		t.Fatal("resume card gates Today")
	}

	a.post(t, h, "/_hx/placement/finish", url.Values{
		"run_id": {hidden(t, started.Body.String(), "run_id")},
		"_csrf":  {a.csrf},
	}, "", true)
	done := getSignedIn(t, h, a.cookie, "/")
	if strings.Contains(done, "today-place") || strings.Contains(done, en.TakeTitle) || strings.Contains(done, "Resume placement") {
		t.Fatal("finished placement should leave Today alone")
	}
	if !strings.Contains(done, `class="today"`) || !strings.Contains(done, `href="/review"`) {
		t.Fatal("finished placement removed Today")
	}
}

func TestPlacementHebrewCardKeepsGloss(t *testing.T) {
	h, _, _, b := newPlacementRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/_hx/placement/start", strings.NewReader(url.Values{"_csrf": {b.csrf}}.Encode()))
	req.Host = "learn.example"
	req.Header.Set("Origin", "https://learn.example")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.AddCookie(b.cookie)
	req.AddCookie(&http.Cookie{Name: langCookieName, Value: "he"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("hebrew start = %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	he := views.PlacementFor("he")
	assertFront(t, body, he)
	if strings.Contains(body, ">Again<") || strings.Contains(body, he.Again) {
		t.Fatal("hebrew front shows the grade pad")
	}
	backReq := httptest.NewRequest(http.MethodPost, "/_hx/placement/flip", strings.NewReader(url.Values{
		"run_id":  {hidden(t, body, "run_id")},
		"item_id": {hidden(t, body, "item_id")},
		"_csrf":   {b.csrf},
	}.Encode()))
	backReq.Host = "learn.example"
	backReq.Header.Set("Origin", "https://learn.example")
	backReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	backReq.Header.Set("HX-Request", "true")
	backReq.AddCookie(b.cookie)
	backReq.AddCookie(&http.Cookie{Name: langCookieName, Value: "he"})
	back := httptest.NewRecorder()
	h.ServeHTTP(back, backReq)
	if back.Code != http.StatusOK {
		t.Fatalf("hebrew flip = %d %s", back.Code, back.Body.String())
	}
	flipped := back.Body.String()
	if !strings.Contains(flipped, he.Again) || !strings.Contains(flipped, he.Good) || strings.Contains(flipped, ">Again<") {
		t.Fatal("hebrew grades did not replace the English labels")
	}
	assertBack(t, flipped, he)
}

func assertFront(t *testing.T, body string, copy views.PlacementCopy) {
	t.Helper()
	he := regexp.MustCompile(`<bdi class="wb__he heword" lang="he" dir="rtl"[^>]*>([^<]*)</bdi>`).FindStringSubmatch(body)
	tr := regexp.MustCompile(`<bdi class="wb__tr" lang="he-Latn" dir="ltr">([^<]*)</bdi>`).FindStringSubmatch(body)
	if he == nil || tr == nil || he[1] == "" || tr[1] == "" {
		t.Fatalf("front word block missing: %s", body)
	}
	if !norm.NFC.IsNormalString(he[1]) {
		t.Fatalf("hebrew word %q", he[1])
	}
	if strings.Contains(body, `class="wb__gl"`) || strings.Contains(body, `class="ans__main"`) {
		t.Fatal("front shows the gloss")
	}
	if !strings.Contains(body, copy.Flip) || !strings.Contains(body, copy.DontKnow) || !strings.Contains(body, `name="rating" value="0"`) {
		t.Fatal("flip or I don't know missing")
	}
	if strings.Contains(body, `name="rating" value="1"`) {
		t.Fatal("grades are on the front")
	}
	if !strings.Contains(body, `hx-post="/_hx/placement/flip"`) || !strings.Contains(body, `role="progressbar"`) || !strings.Contains(body, "/20") {
		t.Fatal("flip or sitting bar missing")
	}
	if strings.Contains(body, "from:body") || strings.Contains(body, "hx-on") || strings.Contains(body, `class="listen"`) {
		t.Fatal("front still has a key shortcut or a button around the audio")
	}
	if !strings.Contains(body, "<audio controls") || !strings.Contains(body, `preload="auto"`) || !strings.Contains(body, `src="/audio/`) || !strings.Contains(body, `aria-label="`) {
		t.Fatal("front audio missing")
	}
	if strings.Contains(body, "overflow:hidden") || strings.Contains(body, "overflow: hidden") {
		t.Fatal("card clips overflow")
	}
}

func assertBack(t *testing.T, body string, copy views.PlacementCopy) {
	t.Helper()
	if !strings.Contains(body, `class="ans__main"`) || !strings.Contains(body, `lang="he-Latn"`) {
		t.Fatal("back is missing the gloss or the transliteration")
	}
	if !strings.Contains(body, "<audio autoplay controls") || !strings.Contains(body, `preload="auto"`) || strings.Contains(body, "hx-on") || strings.Contains(body, "0.75") {
		t.Fatal("back audio missing")
	}
	for _, label := range []string{copy.Again, copy.Hard, copy.Good, copy.Easy} {
		if !strings.Contains(body, label) {
			t.Fatalf("grade %q missing", label)
		}
	}
	for _, value := range []string{"1", "2", "3", "4"} {
		if !strings.Contains(body, `name="rating" value="`+value+`"`) {
			t.Fatalf("rating %s missing", value)
		}
	}
	if strings.Contains(body, "key==") || strings.Contains(body, "from:body") {
		t.Fatal("grades still advertise a key shortcut")
	}
	if strings.Contains(body, "overflow:hidden") || strings.Contains(body, "overflow: hidden") {
		t.Fatal("card clips overflow")
	}
}
