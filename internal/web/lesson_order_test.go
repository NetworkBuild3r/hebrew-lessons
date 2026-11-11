package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// INIT-029/SPEC-003: order serve and grade.

const (
	orderHello   = "שָׁלוֹם"
	orderMorning = "בֹּקֶר"
	orderThanks  = "תּוֹדָה"
)

func orderLine() string {
	return orderHello + " " + orderMorning + " " + orderThanks
}

func orderAcceptLine() string {
	return orderThanks + " " + orderHello + " " + orderMorning
}

type orderFixture struct {
	store  *db.Store
	h      http.Handler
	cookie *http.Cookie
	path   string
	unitID int64
	userID int64
	dbPath string
	lesson db.CourseLesson
	clk    time.Time
}

func setupOrderLesson(t *testing.T, accept []string) orderFixture {
	t.Helper()
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "hebrew.sqlite")
	store, err := db.Open(ctx, dbPath, fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	sp := func(s string) *string { return &s }
	if _, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: orderHello, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: orderMorning, Gloss: "morning", Translit: sp("boker"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: orderThanks, Gloss: "thanks", Translit: sp("toda"), Gender: sp("f"), POS: sp("noun")},
	}); err != nil {
		t.Fatal(err)
	}
	line := orderLine()
	clipID := strings.Repeat("cd", 32)
	if _, err := store.InsertAudio(ctx, db.Asset{
		ID: clipID, TextNFC: units.Normalize(line), Voice: "voice", Model: "eleven_v3", Bytes: 4,
	}); err != nil {
		t.Fatal(err)
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Order", Situation: "Build a greeting.", Concept: "Word order.", Position: 1,
		NewWords: []string{orderHello, orderMorning, orderThanks},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{{
			Type: units.ActivityOrder, Text: line, Translation: "hello morning thanks", Accept: accept,
		}}}},
	}, "shared")
	publish(t, store, unitID)
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: fixedClock(now), Store: store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	lesson, err := store.CourseLesson(ctx, user.ID, unitID, 0)
	if err != nil {
		t.Fatal(err)
	}
	return orderFixture{
		store:  store,
		h:      NewOptions(nil, svc, Options{Store: store}),
		cookie: mintSession(t, store, user.ID, "brian-csrf", now.Add(time.Hour)),
		path:   "/lesson/" + itoa(unitID) + "/0",
		unitID: unitID,
		userID: user.ID,
		dbPath: dbPath,
		lesson: lesson,
		clk:    now,
	}
}

func (f orderFixture) openAsk(t *testing.T) (db.PlayerRun, views.Activity) {
	t.Helper()
	body := getAuthed(t, f.h, f.cookie, f.path)
	runID, err := strconv.ParseInt(runValue(t, body), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	run, err := f.store.PlayerRunFor(context.Background(), f.userID, runID)
	if err != nil {
		t.Fatal(err)
	}
	p := player{store: f.store}
	screen, err := p.screenFor(context.Background(), views.NewPage("en", "brian-csrf", "lesson"), f.lesson, run, lessonQuery{})
	if err != nil {
		t.Fatal(err)
	}
	return run, screen.Activity
}

func postOrder(t *testing.T, f orderFixture, runID string, positions []int) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("run", runID)
	form.Set("answer", "check")
	for _, p := range positions {
		form.Add("order", strconv.Itoa(p))
	}
	form.Set("_csrf", "brian-csrf")
	req := httptest.NewRequest(http.MethodPost, f.path, strings.NewReader(form.Encode()))
	req.Host = "learn.example"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.Header.Set("Origin", "https://learn.example")
	req.Header.Set("X-CSRF-Token", "brian-csrf")
	req.AddCookie(f.cookie)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func positionsForTokens(served, want []string) []int {
	used := make([]bool, len(served))
	out := make([]int, len(want))
	for i, tok := range want {
		for j, s := range served {
			if used[j] || s != tok {
				continue
			}
			out[i] = j
			used[j] = true
			break
		}
	}
	return out
}

func wrongPermutation(served []string) []int {
	// Rotate the served indices by one so the built line is not the answer
	// when the served bank itself is already not the answer.
	n := len(served)
	out := make([]int, n)
	for i := range out {
		out[i] = (i + 1) % n
	}
	act := units.Activity{Type: units.ActivityOrder, Text: orderLine(), Accept: []string{orderAcceptLine()}}
	if _, correct, ok := orderGrade(served, out, act); ok && !correct {
		return out
	}
	// Fall back: reverse indices.
	for i := range out {
		out[i] = n - 1 - i
	}
	return out
}

// TestOrderAsksToBuildTheStoredSentence: the ask is the shuffled words and
// the English cue. The sentence clip is not on the ask. Opening writes nothing.
func TestOrderAsksToBuildTheStoredSentence(t *testing.T) {
	f := setupOrderLesson(t, []string{orderAcceptLine()})
	run, ask := f.openAsk(t)
	if !ask.Asking || len(ask.OrderChips) != 3 {
		t.Fatalf("order ask: chips %v asking %v", ask.OrderChips, ask.Asking)
	}
	if orderMatchesAnswer(ask.OrderChips, f.lesson.Activities[0]) {
		t.Fatalf("the bank was already an answer: %v", ask.OrderChips)
	}
	if ask.Cue != "hello morning thanks" {
		t.Fatalf("cue = %q", ask.Cue)
	}
	if ask.Audio != "" {
		t.Fatal("the sentence clip was on the ask")
	}
	p := player{store: f.store}
	target, err := p.targetOf(context.Background(), f.lesson, f.lesson.Activities[0])
	if err != nil {
		t.Fatal(err)
	}
	n, err := p.askChoiceCount(context.Background(), f.userID, f.lesson, run, f.lesson.Activities[0], target)
	if err != nil || n != 3 {
		t.Fatalf("askChoiceCount = %d, %v", n, err)
	}
	if scalar(t, f.dbPath, `SELECT COUNT(*) FROM attempts`) != 0 {
		t.Fatal("opening an order step wrote an attempt")
	}
}

// TestOrderCheckGradesTheStoredSentence: the stored line and an accept line
// are right. Another order is wrong. Either way one attempt is written, and
// no card is scheduled from this step.
func TestOrderCheckGradesTheStoredSentence(t *testing.T) {
	f := setupOrderLesson(t, []string{orderAcceptLine()})
	run, ask := f.openAsk(t)
	right := positionsForTokens(ask.OrderChips, units.Tokenize(orderLine()))
	rec := postOrder(t, f, strconv.FormatInt(run.ID, 10), right)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "feedback--right") {
		t.Fatalf("stored line = %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "/audio/") {
		t.Fatal("the reveal did not play the sentence clip")
	}
	if n := scalar(t, f.dbPath, `SELECT COUNT(*) FROM attempts`); n != 1 {
		t.Fatalf("attempts = %d", n)
	}
	if n := scalar(t, f.dbPath, `SELECT COUNT(*) FROM cards`); n != 0 {
		t.Fatalf("order scheduled a card: %d", n)
	}

	f2 := setupOrderLesson(t, []string{orderAcceptLine()})
	run2, ask2 := f2.openAsk(t)
	also := positionsForTokens(ask2.OrderChips, units.Tokenize(orderAcceptLine()))
	rec = postOrder(t, f2, strconv.FormatInt(run2.ID, 10), also)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "feedback--right") {
		t.Fatalf("accept line = %d\n%s", rec.Code, rec.Body.String())
	}

	f3 := setupOrderLesson(t, []string{orderAcceptLine()})
	run3, ask3 := f3.openAsk(t)
	rec = postOrder(t, f3, strconv.FormatInt(run3.ID, 10), wrongPermutation(ask3.OrderChips))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Not quite") {
		t.Fatalf("wrong order = %d\n%s", rec.Code, rec.Body.String())
	}
}

// TestOrderContinueBeforeCheckStays: Continue on the ask does not skip the
// build. After Check, Continue leaves the step.
func TestOrderContinueBeforeCheckStays(t *testing.T) {
	f := setupOrderLesson(t, nil)
	run, ask := f.openAsk(t)
	rec := postLesson(t, f.h, f.cookie, f.path, strconv.FormatInt(run.ID, 10), "continue", true, "brian-csrf")
	if rec.Code != http.StatusOK {
		t.Fatalf("continue = %d\n%s", rec.Code, rec.Body.String())
	}
	if n := scalar(t, f.dbPath, `SELECT COUNT(*) FROM attempts`); n != 0 {
		t.Fatalf("continue wrote %d attempts", n)
	}
	after, err := f.store.PlayerRunFor(context.Background(), f.userID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Phase != "ask" || after.ActivityIx != run.ActivityIx {
		t.Fatal("continue left the build")
	}
	right := positionsForTokens(ask.OrderChips, units.Tokenize(orderLine()))
	rec = postOrder(t, f, strconv.FormatInt(run.ID, 10), right)
	if rec.Code != http.StatusOK {
		t.Fatalf("check = %d", rec.Code)
	}
	rec = postLesson(t, f.h, f.cookie, f.path, strconv.FormatInt(run.ID, 10), "continue", true, "brian-csrf")
	if rec.Code != http.StatusOK {
		t.Fatalf("continue after check = %d", rec.Code)
	}
	done, err := f.store.PlayerRunFor(context.Background(), f.userID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !done.Finished {
		t.Fatal("continue after the build did not finish the lesson")
	}
}

func scalarString(t *testing.T, path, query string, args ...any) string {
	t.Helper()
	var s string
	if err := sqlScalar(t, path, query, args, &s); err != nil {
		t.Fatal(err)
	}
	return s
}
