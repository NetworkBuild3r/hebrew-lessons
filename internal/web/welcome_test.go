package web

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// The first-run welcome and the Progress level card. The store does not have
// migration 014 yet, so these tests pass a *memOnboard. Finishing a run on
// that fake sets onboarded_at and appends history. Abandon does neither.
// The HTTP finish handler must not call SetOnboarded itself.

var welcomeNow = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

func openWelcome(t *testing.T, clk clock.Clock, mem *memOnboard) (*db.Store, http.Handler) {
	t.Helper()
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         clk,
		Store:         store,
		AllowedEmails: shellEmails(),
	})
	if err != nil {
		t.Fatal(err)
	}
	study := session.New(store, clk, 1, session.WithGate(func(context.Context, int64) (activity.Gate, error) {
		return activity.Gate{LettersPassed: false}, nil
	}))
	h := NewOptions(nil, svc, Options{Store: store, Runs: store, study: study, Onboard: mem})
	return store, h
}

func welcomeGet(t *testing.T, h http.Handler, l learnerFix, path string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if l.cookie != nil {
		req.AddCookie(l.cookie)
	}
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func assertWelcome(t *testing.T, rec *httptest.ResponseRecorder, path string) {
	t.Helper()
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/welcome" {
		t.Errorf("GET %s = %d %q, want 303 /welcome", path, rec.Code, rec.Header().Get("Location"))
	}
}

func assertNotWelcome(t *testing.T, rec *httptest.ResponseRecorder, path string) {
	t.Helper()
	if rec.Code == http.StatusSeeOther && rec.Header().Get("Location") == "/welcome" {
		t.Errorf("GET %s still goes to /welcome", path)
	}
}

func welcomeLandings() []string {
	return []string{"/", "/lessons", "/review", "/progress", "/me", "/lesson", "/lesson/1/0", "/progress/placements"}
}

// todayPlacementEntry is the copy Today must not carry. The check-in posts
// to /_hx/placement/recheck and the article may keep data-placement=.
func todayPlacementEntry(body string) string {
	for _, bad := range []string{
		"Take the placement check",
		"Take placement",
		"Resume placement",
		"Find your level",
		`href="/placement"`,
		`data-action="start_placement"`,
		`data-action="resume_placement"`,
		"בְּדִיקַת רָמָה",
	} {
		if strings.Contains(body, bad) {
			return bad
		}
	}
	return ""
}

func TestNewLearnerIsSentToWelcome(t *testing.T) {
	mem := newMemOnboard()
	store, h := openWelcome(t, fixedClock(welcomeNow), mem)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)

	for _, path := range welcomeLandings() {
		assertWelcome(t, welcomeGet(t, h, l, path, false), path)
	}
	assertWelcome(t, welcomeGet(t, h, l, "/", true), "HX /")

	page := welcomeGet(t, h, l, "/welcome", false)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), views.WelcomeTitle) {
		t.Fatalf("GET /welcome = %d", page.Code)
	}

	// Placement itself, a lesson report, and static files stay open.
	for _, path := range []string{"/placement", "/lessons/1/report", "/static/css/tokens.css"} {
		assertNotWelcome(t, welcomeGet(t, h, l, path, false), path)
	}
	anon := welcomeGet(t, h, learnerFix{}, "/", false)
	if anon.Code != http.StatusSeeOther || anon.Header().Get("Location") != "/signin" {
		t.Errorf("signed-out GET / = %d %q", anon.Code, anon.Header().Get("Location"))
	}
}

func TestDeclinerIsNotGatedAgain(t *testing.T) {
	mem := newMemOnboard()
	store, h := openWelcome(t, fixedClock(welcomeNow), mem)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)

	rec := formPost(t, h, l, "/onboarding/basics", url.Values{"_csrf": {l.csrf}}, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("basics = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if mem.SetCalls(l.user.ID) != 1 {
		t.Fatalf("SetOnboarded calls = %d, want 1", mem.SetCalls(l.user.ID))
	}
	at, err := mem.OnboardedAt(context.Background(), l.user.ID)
	if err != nil || at == nil || !at.Equal(welcomeNow) {
		t.Fatalf("onboarded_at = %v %v", at, err)
	}

	for _, path := range welcomeLandings() {
		got := welcomeGet(t, h, l, path, false)
		assertNotWelcome(t, got, path)
		if path == "/" && got.Code != http.StatusOK {
			t.Errorf("Today after decline = %d", got.Code)
		}
	}
	again := welcomeGet(t, h, l, "/welcome", false)
	if again.Code != http.StatusSeeOther || again.Header().Get("Location") != "/" {
		t.Errorf("GET /welcome after decline = %d %q", again.Code, again.Header().Get("Location"))
	}

	home := welcomeGet(t, h, l, "/", false)
	hx := welcomeGet(t, h, l, "/", true)
	if hx.Code != http.StatusOK {
		t.Fatalf("HX Today after decline = %d", hx.Code)
	}
	for _, body := range []string{home.Body.String(), hx.Body.String()} {
		if bad := todayPlacementEntry(body); bad != "" {
			t.Errorf("Today after decline has %q", bad)
		}
	}
}

func TestFinishedPlacementOpensTheAppAndShowsHistory(t *testing.T) {
	mem := newMemOnboard()
	store, h := openWelcome(t, fixedClock(welcomeNow), mem)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	older := welcomeNow.AddDate(0, 0, -14)
	newer := welcomeNow.AddDate(0, 0, -2)
	mem.RecordFinish(l.user.ID, PlacementRun{ID: 1, FinishedAt: older, StartUnit: 2, StartUnitTitle: "First words", WordsKnown: 40})
	mem.RecordFinish(l.user.ID, PlacementRun{ID: 2, FinishedAt: newer, StartUnit: 3, StartUnitTitle: "At home", WordsKnown: 80})

	home := welcomeGet(t, h, l, "/", false)
	if home.Code != http.StatusOK {
		t.Fatalf("Today after a finish = %d %s", home.Code, home.Header().Get("Location"))
	}
	hx := welcomeGet(t, h, l, "/", true)
	if hx.Code != http.StatusOK {
		t.Fatalf("HX Today after a finish = %d", hx.Code)
	}
	for _, body := range []string{home.Body.String(), hx.Body.String()} {
		if bad := todayPlacementEntry(body); bad != "" {
			t.Errorf("Today after a finish has %q", bad)
		}
	}

	card := section(t, welcomeGet(t, h, l, "/progress", false).Body.String(), "c-card pplace", "")
	rows := regexp.MustCompile(`(?s)<li class="pplace__row">.*?</li>`).FindAllString(card, -1)
	if len(rows) != 2 {
		t.Fatalf("%d rows, want 2:\n%s", len(rows), card)
	}
	for _, want := range []string{"Retake placement", views.RetakeKeep, "Past results", "Latest", "Placed at Unit 3, Oct 16", "Unit 3: At home", "Unit 2: First words", "Oct 4"} {
		if !strings.Contains(card, want) {
			t.Errorf("history card missing %q", want)
		}
	}
	if strings.Contains(rows[1], "Latest") || !strings.Contains(rows[0], "Latest") || !strings.Contains(rows[0], "Oct 16") {
		t.Errorf("Latest is not only on the newest row:\n%s", card)
	}
	if strings.Contains(card, "At home,") || strings.Contains(card, "80") || strings.Contains(card, "40") || strings.Contains(card, "Show all") {
		t.Errorf("status, word count, or Show all leaked:\n%s", card)
	}
	if strings.Count(card, "Latest") != 1 {
		t.Errorf("Latest appears %d times", strings.Count(card, "Latest"))
	}
}

func TestLessonAttemptsSkipTheWelcome(t *testing.T) {
	mem := newMemOnboard()
	store, h := openWelcome(t, fixedClock(welcomeNow), mem)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	mem.MarkLesson(l.user.ID)

	for _, path := range welcomeLandings() {
		assertNotWelcome(t, welcomeGet(t, h, l, path, false), path)
	}
	home := welcomeGet(t, h, l, "/", true)
	if home.Code != http.StatusOK {
		t.Fatalf("HX Today = %d", home.Code)
	}
	if bad := todayPlacementEntry(home.Body.String()); bad != "" {
		t.Errorf("Today with only lesson attempts has %q", bad)
	}
	card := section(t, welcomeGet(t, h, l, "/progress", false).Body.String(), "c-card pplace", "")
	for _, want := range []string{"No placement yet", "Take placement", `href="/placement"`, "No results yet."} {
		if !strings.Contains(card, want) {
			t.Errorf("unplaced progress missing %q", want)
		}
	}
	if strings.Contains(card, views.RetakeKeep) || strings.Contains(card, "Retake placement") || strings.Contains(card, "Show all") || strings.Contains(card, "Latest") {
		t.Errorf("unplaced progress offers a retake:\n%s", card)
	}
}

func TestAbandonedRunStillGates(t *testing.T) {
	mem := newMemOnboard()
	store, h := openWelcome(t, fixedClock(welcomeNow), mem)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	mem.Abandon(l.user.ID)

	for _, path := range welcomeLandings() {
		assertWelcome(t, welcomeGet(t, h, l, path, false), path)
	}
	n, err := mem.PlacementHistoryCount(context.Background(), l.user.ID)
	if err != nil || n != 0 {
		t.Fatalf("abandoned run counted as history: %d %v", n, err)
	}
	at, err := mem.OnboardedAt(context.Background(), l.user.ID)
	if err != nil || at != nil {
		t.Fatalf("abandoned run set onboarded_at %v", at)
	}
	if mem.Abandoned(l.user.ID) != 1 {
		t.Fatal("the abandoned run was not recorded")
	}
}

func TestBasicsPostRequiresCSRF(t *testing.T) {
	mem := newMemOnboard()
	store, h := openWelcome(t, fixedClock(welcomeNow), mem)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	rec := formPost(t, h, l, "/onboarding/basics", url.Values{}, nil)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "csrf") {
		t.Fatalf("missing CSRF = %d %s", rec.Code, rec.Body.String())
	}
	if mem.SetCalls(l.user.ID) != 0 {
		t.Fatal("a rejected post called SetOnboarded")
	}
	assertWelcome(t, welcomeGet(t, h, l, "/", false), "/")
}

func TestWelcomeWorksWithoutScript(t *testing.T) {
	mem := newMemOnboard()
	store, h := openWelcome(t, fixedClock(welcomeNow), mem)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	body := welcomeGet(t, h, l, "/welcome", false).Body.String()

	primary := regexp.MustCompile(`(?s)<a class="choice-card choice-card--primary".*?</a>`).FindString(body)
	if primary == "" || !strings.Contains(primary, `href="/placement"`) || !strings.Contains(primary, "Find your level") || !strings.Contains(primary, "Start placement") {
		t.Fatalf("primary card:\n%s", primary)
	}
	if strings.Contains(primary, "<button") || strings.Contains(primary, "choice-cta") && !strings.Contains(primary, "<span") {
		t.Fatal("the gold line is not a span inside the card link")
	}
	if !strings.Contains(primary, `class="choice-cta"`) || strings.Contains(primary, "<button") {
		t.Fatal("Start placement is not a styled span")
	}
	if strings.Contains(primary, "10") || strings.Contains(primary, "min") || strings.Contains(primary, "About") {
		t.Fatal("the meta row estimates a time the flow does not have")
	}
	if !strings.Contains(primary, "Listening") || !strings.Contains(primary, "Reading") {
		t.Fatal("the meta row dropped a skill")
	}
	form := regexp.MustCompile(`(?s)<form method="post" action="/onboarding/basics".*?</form>`).FindString(body)
	if form == "" || !strings.Contains(form, `hx-boost="false"`) || !strings.Contains(form, `name="_csrf"`) || !strings.Contains(form, "Start from the basics") {
		t.Fatalf("basics form:\n%s", form)
	}
	if !strings.Contains(form, "<button") {
		t.Fatal("the secondary choice is not a submit button")
	}
	if strings.Contains(body, "hx-on") || strings.Contains(body, "onclick") || strings.Contains(body, "autofocus") {
		t.Fatal("welcome needs a script or an autofocus")
	}
	note := regexp.MustCompile(`(?s)<p class="welcome__note">.*?</p>`).FindString(body)
	if !strings.Contains(note, views.WelcomeNote) || strings.Contains(note, "<a ") || strings.Contains(note, "tabindex") {
		t.Fatalf("footer:\n%s", note)
	}
	if strings.Contains(body, `class="dock"`) || strings.Contains(body, `class="rail"`) {
		t.Fatal("welcome uses the app shell")
	}
}

func TestSetOnboardedKeepsTheFirstTime(t *testing.T) {
	mem := newMemOnboard()
	store, h := openWelcome(t, fixedClock(welcomeNow), mem)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	first := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := mem.SetOnboarded(context.Background(), l.user.ID, first); err != nil {
		t.Fatal(err)
	}
	rec := formPost(t, h, l, "/onboarding/basics", url.Values{"_csrf": {l.csrf}}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("second basics = %d", rec.Code)
	}
	at, err := mem.OnboardedAt(context.Background(), l.user.ID)
	if err != nil || at == nil || !at.Equal(first) {
		t.Fatalf("onboarded_at = %v, want the first value %v", at, first)
	}
	if mem.SetCalls(l.user.ID) != 2 {
		t.Fatalf("calls = %d, want 2", mem.SetCalls(l.user.ID))
	}
}

func TestPlacementHistoryShapes(t *testing.T) {
	clk := fixedClock(welcomeNow)
	cases := []struct {
		name  string
		n     int
		title string
	}{
		{"none", 0, ""},
		{"one", 1, "At home"},
		{"three", 3, "At home"},
		{"five", 5, "At home"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mem := newMemOnboard()
			store, h := openWelcome(t, clk, mem)
			l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
			if tc.n == 0 {
				if err := mem.SetOnboarded(context.Background(), l.user.ID, welcomeNow); err != nil {
					t.Fatal(err)
				}
			}
			for i := 1; i <= tc.n; i++ {
				mem.RecordFinish(l.user.ID, PlacementRun{
					ID:             int64(i),
					FinishedAt:     time.Date(2026, 10, i, 8, 0, 0, 0, time.UTC),
					StartUnit:      i,
					StartUnitTitle: tc.title,
					WordsKnown:     100 + i,
				})
			}
			body := welcomeGet(t, h, l, "/progress?lang=he&skill=read", false).Body.String()
			card := section(t, body, "c-card pplace", "")
			rows := regexp.MustCompile(`(?s)<li class="pplace__row">.*?</li>`).FindAllString(card, -1)
			wantRows := tc.n
			if wantRows > 3 {
				wantRows = 3
			}
			if len(rows) != wantRows {
				t.Fatalf("%d rows, want %d:\n%s", len(rows), wantRows, card)
			}
			if tc.n == 0 {
				for _, want := range []string{"No placement yet", "Take placement", "No results yet."} {
					if !strings.Contains(card, want) {
						t.Errorf("empty card missing %q", want)
					}
				}
				if strings.Contains(card, views.RetakeKeep) || strings.Contains(card, "Retake") || strings.Contains(card, "Show all") || strings.Contains(card, "Latest") {
					t.Errorf("empty card:\n%s", card)
				}
				return
			}
			if strings.Count(card, "Latest") != 1 || !strings.Contains(rows[0], "Latest") {
				t.Errorf("Latest is not only the newest row:\n%s", card)
			}
			for i := 1; i < len(rows); i++ {
				if strings.Contains(rows[i], "Latest") {
					t.Errorf("row %d is marked Latest", i)
				}
			}
			if !strings.Contains(card, views.RetakeKeep) || !strings.Contains(card, "Retake placement") {
				t.Error("a placed learner has no retake line")
			}
			newestDay := fmt.Sprintf("Oct %d", tc.n)
			if !strings.Contains(rows[0], newestDay) {
				t.Errorf("newest row is not %s:\n%s", newestDay, rows[0])
			}
			if tc.n > 3 {
				if !strings.Contains(card, "Show all (5)") || !strings.Contains(card, `/progress/placements?lang=he&amp;skill=read`) {
					t.Errorf("Show all:\n%s", card)
				}
				all := welcomeGet(t, h, l, "/progress/placements?lang=he&skill=read", false).Body.String()
				allCard := section(t, all, "c-card pplace", "")
				allRows := regexp.MustCompile(`(?s)<li class="pplace__row">.*?</li>`).FindAllString(allCard, -1)
				if len(allRows) != 5 {
					t.Fatalf("full history has %d rows:\n%s", len(allRows), allCard)
				}
				if strings.Contains(allCard, "Show all") {
					t.Error("the full history offers Show all")
				}
				if !strings.Contains(allRows[0], "Oct 5") || !strings.Contains(allRows[4], "Oct 1") {
					t.Errorf("full history is not newest first:\n%s", allCard)
				}
			} else if strings.Contains(card, "Show all") {
				t.Error("Show all with three or fewer runs")
			}
		})
	}
}

func TestPlacementHistoryYear(t *testing.T) {
	mem := newMemOnboard()
	store, h := openWelcome(t, fixedClock(welcomeNow), mem)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	mem.RecordFinish(l.user.ID, PlacementRun{ID: 1, FinishedAt: time.Date(2025, 1, 2, 9, 0, 0, 0, time.UTC), StartUnit: 1, StartUnitTitle: "Alef"})
	mem.RecordFinish(l.user.ID, PlacementRun{ID: 2, FinishedAt: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC), StartUnit: 2, StartUnitTitle: "Home"})
	card := section(t, welcomeGet(t, h, l, "/progress", false).Body.String(), "c-card pplace", "")
	if !strings.Contains(card, "Jan 2, 2025") {
		t.Errorf("a prior year dropped the year:\n%s", card)
	}
	if strings.Contains(card, "Oct 2, 2026") || !strings.Contains(card, "Oct 2") {
		t.Errorf("this year printed a year:\n%s", card)
	}
	if !strings.Contains(card, "Placed at Unit 2, Oct 2") {
		t.Errorf("status:\n%s", card)
	}
}

func TestPlacementHistoryEmptyUnitTitle(t *testing.T) {
	mem := newMemOnboard()
	store, h := openWelcome(t, fixedClock(welcomeNow), mem)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	mem.RecordFinish(l.user.ID, PlacementRun{ID: 1, FinishedAt: welcomeNow.AddDate(0, 0, -1), StartUnit: 4, WordsKnown: 12})
	card := section(t, welcomeGet(t, h, l, "/progress", false).Body.String(), "c-card pplace", "")
	if !strings.Contains(card, ">Unit 4</bdi>") {
		t.Errorf("empty title:\n%s", card)
	}
	if strings.Contains(card, "Unit 4:") {
		t.Errorf("empty title still has a colon:\n%s", card)
	}
	if strings.Contains(card, "12") {
		t.Errorf("words known is on the row:\n%s", card)
	}

	mem.RecordFinish(l.user.ID, PlacementRun{ID: 2, FinishedAt: welcomeNow, StartUnit: 4, StartUnitTitle: "At home"})
	card = section(t, welcomeGet(t, h, l, "/progress", false).Body.String(), "c-card pplace", "")
	if !strings.Contains(card, "Unit 4: At home") {
		t.Errorf("titled unit:\n%s", card)
	}
	line := regexp.MustCompile(`(?s)<p class="pplace__line">.*?</p>`).FindString(card)
	if strings.Contains(line, "At home") || !strings.Contains(line, "Placed at Unit 4, Oct 18") {
		t.Errorf("status line carries the unit name:\n%s", line)
	}
}

func TestFinishHandlerDoesNotSetOnboarded(t *testing.T) {
	mem := newMemOnboard()
	now := welcomeNow
	clk := fixedClock(now)
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedAudio(t, store)
	a := insertPlacedUser(t, store, "a@example.com", "sub-a", "csrf-a", now)
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{"a@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := placement.NewEngine(store, evidence.New(store, fsrs.New(store, clk), clk), clk, placement.WithSource(constSource(3)))
	h := NewOptions(nil, svc, Options{Store: store, Deck: engine, Runs: store, Onboard: mem})
	d := newDriver(t, h, store, a)
	d.run(plScript{letters: 2, typing: "skip", pick: func(db.PlacementItem, int) string { return "wrong" }})
	// The letters-failed path finishes inside the engine, from Next. Posting
	// finish on that run still hits POST /_hx/placement/finish, which must
	// not call SetOnboarded either. The store's own finish transaction is
	// what sets onboarded_at.
	run, err := store.LatestRun(context.Background(), a.id)
	if err != nil {
		t.Fatal(err)
	}
	d.post("/_hx/placement/finish", url.Values{"run_id": {fmt.Sprintf("%d", run.ID)}})
	if mem.SetCalls(a.id) != 0 {
		t.Fatalf("finish called SetOnboarded %d times", mem.SetCalls(a.id))
	}
	at, err := mem.OnboardedAt(context.Background(), a.id)
	if err != nil || at != nil {
		t.Fatalf("finish set onboarded_at %v %v", at, err)
	}
	who := learnerFix{cookie: a.cookie, csrf: a.csrf}
	rec := formPost(t, h, who, "/onboarding/basics", url.Values{"_csrf": {a.csrf}}, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("result basics = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if mem.SetCalls(a.id) != 1 {
		t.Fatalf("the result path called SetOnboarded %d times", mem.SetCalls(a.id))
	}
}

func TestRetakeLeavesCardsAttemptsAndLessons(t *testing.T) {
	clk := fixedClock(welcomeNow)
	store, _ := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	ctx := context.Background()
	rec := evidence.New(store, fsrs.New(store, clk), clk)
	first := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	second := addLexeme(t, store, "בַּיִת", "house", "bayit", "m", "")
	attempt := func(lexeme int64, skill string, correct bool) {
		t.Helper()
		if _, err := rec.Attempt(ctx, evidence.Attempt{
			UserID: l.user.ID, LexemeID: lexeme, Skill: skill, Activity: "listen_pick",
			Source: evidence.SourceLesson, Prompt: "p", Response: "r", Correct: correct,
		}); err != nil {
			t.Fatal(err)
		}
	}
	attempt(first, "listen", true)
	attempt(first, "listen", false)
	attempt(second, "read", true)

	unit := shellUnit(t, store, "Basics", 1, "shared", 1)
	run, err := store.StartPlayerRun(ctx, l.user.ID, unit, 0)
	if err != nil {
		t.Fatal(err)
	}
	finished, applied, err := store.ContinuePlayerRun(ctx, l.user.ID, run.ID, 0, 1)
	if err != nil || !applied || !finished.Finished {
		t.Fatalf("lesson finish applied=%v finished=%v err=%v", applied, finished.Finished, err)
	}

	beforeCards := dumpQuery(t, store, `SELECT lexeme_id, skill, state, printf('%.6f', stability), printf('%.6f', difficulty), reps, lapses, due, COALESCE(last_review, ''), suspended FROM cards WHERE user_id = ? ORDER BY id`, l.user.ID)
	beforeReviews := dumpQuery(t, store, `SELECT card_id, rating, reviewed_at, elapsed_days, scheduled_days FROM review_log ORDER BY id`)
	beforeLessons := dumpQuery(t, store, `SELECT unit_id, lesson_ix, started_at, finished_at, score FROM lesson_runs WHERE user_id = ? ORDER BY id`, l.user.ID)
	lessonAttempts := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ? AND source = 'lesson'`, l.user.ID)
	allAttempts := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, l.user.ID)
	if lessonAttempts != 3 || allAttempts != 3 || len(beforeCards) != 2 || len(beforeLessons) != 1 {
		t.Fatalf("fixture: attempts %d/%d cards %d lessons %d", lessonAttempts, allAttempts, len(beforeCards), len(beforeLessons))
	}

	for i := 0; i < 2; i++ {
		id, err := store.StartPlacementV3(ctx, l.user.ID, welcomeNow.Add(time.Duration(i)*time.Hour), int64(i+1), "initial", "", 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.FinishPlacement(ctx, id, welcomeNow.Add(time.Duration(i+1)*time.Hour), 2, "cap_cards", `{"estimated_known_words":9}`); err != nil {
			t.Fatal(err)
		}
	}

	if got := dumpQuery(t, store, `SELECT lexeme_id, skill, state, printf('%.6f', stability), printf('%.6f', difficulty), reps, lapses, due, COALESCE(last_review, ''), suspended FROM cards WHERE user_id = ? ORDER BY id`, l.user.ID); strings.Join(got, "\n") != strings.Join(beforeCards, "\n") {
		t.Fatalf("cards changed\nbefore:\n%s\nafter:\n%s", strings.Join(beforeCards, "\n"), strings.Join(got, "\n"))
	}
	if got := dumpQuery(t, store, `SELECT card_id, rating, reviewed_at, elapsed_days, scheduled_days FROM review_log ORDER BY id`); strings.Join(got, "\n") != strings.Join(beforeReviews, "\n") {
		t.Fatalf("review log changed\nbefore:\n%s\nafter:\n%s", strings.Join(beforeReviews, "\n"), strings.Join(got, "\n"))
	}
	if got := dumpQuery(t, store, `SELECT unit_id, lesson_ix, started_at, finished_at, score FROM lesson_runs WHERE user_id = ? ORDER BY id`, l.user.ID); strings.Join(got, "\n") != strings.Join(beforeLessons, "\n") {
		t.Fatalf("lesson runs changed\nbefore:\n%s\nafter:\n%s", strings.Join(beforeLessons, "\n"), strings.Join(got, "\n"))
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ? AND source = 'lesson'`, l.user.ID); n != lessonAttempts {
		t.Fatalf("lesson attempts %d, want %d", n, lessonAttempts)
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, l.user.ID); n != allAttempts {
		t.Fatalf("attempts %d, want %d (finish alone must not write one)", n, allAttempts)
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM placement_runs WHERE user_id = ? AND finished_at IS NOT NULL`, l.user.ID); n != 2 {
		t.Fatalf("finished runs = %d, want 2", n)
	}
}

func dumpQuery(t *testing.T, store *db.Store, query string, args ...any) []string {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := conn.QueryContext(context.Background(), query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		parts := make([]string, len(cols))
		for i, v := range vals {
			switch x := v.(type) {
			case nil:
				parts[i] = ""
			case []byte:
				parts[i] = string(x)
			case time.Time:
				parts[i] = x.UTC().Format(time.RFC3339Nano)
			default:
				parts[i] = fmt.Sprint(x)
			}
		}
		out = append(out, strings.Join(parts, "|"))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPlacementAnswerShowsNoVerdict(t *testing.T) {
	h, store, a, b := newPlacementRouter(t)
	check := func(body string) {
		t.Helper()
		for _, bad := range []string{`data-state="correct"`, `data-state="wrong"`, "c-answer__mark", `class="fb`, "feedback-right", "feedback-wrong", "--go", "--stop"} {
			if strings.Contains(body, bad) {
				t.Errorf("verdict markup %q", bad)
			}
		}
	}
	right := newDriver(t, h, store, a)
	right.post("/_hx/placement/start", url.Values{})
	ask := right.body
	if !strings.Contains(ask, views.SkipLabel) || !strings.Contains(ask, views.SkipHint) {
		t.Fatal("the question is missing the skip line")
	}
	if strings.Contains(ask, "I don’t know this one") || strings.Contains(ask, "Skip this word") {
		t.Fatal("the old skip copy is still on the question")
	}
	if !strings.Contains(ask, "Placement, ") || !strings.Contains(ask, " of ") {
		t.Fatal("the header is not Placement, i of n")
	}
	_, option, _ := right.open()
	picked := right.post("/_hx/placement/answer", url.Values{"run_id": {right.runID()}, "item": {hidden(t, ask, "item")}, "option": {option}})
	check(picked)
	if !strings.Contains(picked, `data-state="selected"`) {
		t.Fatal("a tap did not select the option")
	}

	wrong := newDriver(t, h, store, b)
	wrong.post("/_hx/placement/start", url.Values{})
	_, option, opts := wrong.open()
	var other string
	for _, o := range opts {
		if o.ID != option {
			other = o.ID
			break
		}
	}
	missed := wrong.post("/_hx/placement/answer", url.Values{"run_id": {wrong.runID()}, "item": {hidden(t, wrong.body, "item")}, "option": {other}})
	check(missed)
	if !strings.Contains(missed, `data-state="selected"`) {
		t.Fatal("a wrong tap was not the same selected state")
	}
}

func TestKnewNothingResultHasNoFailureLanguage(t *testing.T) {
	res := placement.RunResult{
		RunID:        7,
		ReadingReady: true,
		StartUnit:    1,
		Listening:    placement.SkillResult{Skill: placement.SkillListen, Tested: true, Level: 0, Estimate: 0},
		Reading:      placement.SkillResult{Skill: placement.SkillRead, Tested: true, Level: 0, Estimate: 0},
	}
	out := resultView(res)
	if !out.KnewNothing || out.LettersFirst || out.Headline != views.KnewNothingH || out.Sub != "" {
		t.Fatalf("knew-nothing view: %+v", out)
	}
	page := views.PlacementView{Page: views.NewPage("en", "csrf-k", "placement"), Screen: views.PlaceResults, Result: out}
	var buf bytes.Buffer
	if err := views.Placement(page).Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	for _, want := range []string{views.KnewNothingH, ">" + views.ResultStart + "<", views.ResultBasics, `action="/onboarding/basics"`, `data-kind="primary"`} {
		if !strings.Contains(body, want) {
			t.Errorf("results missing %q", want)
		}
	}
	for _, bad := range []string{"Letters first", "Not tested yet", "Not yet", "missed", "We’ll review", "You missed", "Start my first lesson", views.ResultLede} {
		if strings.Contains(body, bad) {
			t.Errorf("failure language %q", bad)
		}
	}
	if strings.Contains(body, "pl-meter") || strings.Contains(body, "pl-path") {
		t.Error("knew-nothing still shows meters or a path")
	}
}

func TestResultHeadlineOmitsAnEmptyTitle(t *testing.T) {
	if got := views.ResultHeadline(4, ""); got != "You’re starting at Unit 4" || strings.Contains(got, ":") {
		t.Fatalf("empty title headline %q", got)
	}
	if got := views.UnitLabel(4, ""); got != "Unit 4" {
		t.Fatalf("empty unit label %q", got)
	}
	if got := views.ResultHeadline(4, "At home"); got != "You’re starting at Unit 4: At home" {
		t.Fatalf("titled headline %q", got)
	}
	res := placement.RunResult{
		ReadingReady: true,
		StartUnit:    4,
		Listening:    placement.SkillResult{Tested: true, Level: 2, Estimate: 10},
		Reading:      placement.SkillResult{Tested: true, Level: 1, Estimate: 4},
	}
	out := resultView(res)
	if out.KnewNothing || out.LettersFirst || out.Headline != "You’re starting at Unit 4" || out.Sub != views.ResultLede {
		t.Fatalf("result before a unit lookup: %+v", out)
	}
}
