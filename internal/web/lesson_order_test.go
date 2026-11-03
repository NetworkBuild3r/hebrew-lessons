package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
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

// TestOrderServedStableAndNotAnswer: same step, same run, same served order,
// and that order is not the answer. INIT-029/SPEC-003 AC1.
func TestOrderServedStableAndNotAnswer(t *testing.T) {
	f := setupOrderLesson(t, []string{orderAcceptLine()})
	run, ask := f.openAsk(t)
	if len(ask.OrderChips) != 3 {
		t.Fatalf("chips = %v", ask.OrderChips)
	}
	if ask.Prompt != "Put the words in order." {
		t.Fatalf("prompt = %q", ask.Prompt)
	}
	if ask.Cue != "hello morning thanks" {
		t.Fatalf("cue = %q", ask.Cue)
	}
	for _, chip := range ask.OrderChips {
		if chip != orderHello && chip != orderMorning && chip != orderThanks {
			t.Fatalf("unexpected chip %q in %v", chip, ask.OrderChips)
		}
	}
	act := f.lesson.Activities[0]
	if orderMatchesAnswer(ask.OrderChips, act) {
		t.Fatalf("served order is the answer: %v", ask.OrderChips)
	}
	again := orderServed(run, act)
	if !slices.Equal(again, ask.OrderChips) {
		t.Fatalf("reload order %v != first %v", again, ask.OrderChips)
	}
	p := player{store: f.store}
	screen, err := p.screenFor(context.Background(), views.NewPage("en", "brian-csrf", "lesson"), f.lesson, run, lessonQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(screen.Activity.OrderChips, ask.OrderChips) {
		t.Fatalf("screen chips %v != %v", screen.Activity.OrderChips, ask.OrderChips)
	}
	if screen.Activity.Words != nil {
		t.Fatalf("ask leaked answer-order words: %+v", screen.Activity.Words)
	}
	if screen.Activity.Audio != "" {
		t.Fatalf("ask leaked the clip: %q", screen.Activity.Audio)
	}
}

// TestOrderRightAndAcceptRecordRight: a post of the right order, and of an
// accept order, records a right attempt. INIT-029/SPEC-003 AC2.
func TestOrderRightAndAcceptRecordRight(t *testing.T) {
	for _, name := range []string{"text", "accept"} {
		t.Run(name, func(t *testing.T) {
			f := setupOrderLesson(t, []string{orderAcceptLine()})
			run, ask := f.openAsk(t)
			want := units.Tokenize(orderLine())
			if name == "accept" {
				want = units.Tokenize(orderAcceptLine())
			}
			pos := positionsForTokens(ask.OrderChips, want)
			rec := postOrder(t, f, strconv.FormatInt(run.ID, 10), pos)
			if rec.Code != http.StatusOK {
				t.Fatalf("POST = %d\n%s", rec.Code, rec.Body.String())
			}
			if n := scalar(t, f.dbPath, `SELECT COUNT(*) FROM attempts WHERE activity = 'order' AND correct = 1`); n != 1 {
				t.Fatalf("right attempts = %d", n)
			}
			if n := scalar(t, f.dbPath, `SELECT COUNT(*) FROM cards`); n != 0 {
				t.Fatalf("order scheduled a card: %d", n)
			}
			skill := scalarString(t, f.dbPath, `SELECT skill FROM attempts WHERE activity = 'order'`)
			if skill != "recall" {
				t.Fatalf("skill = %q", skill)
			}
		})
	}
}

// TestOrderWrongPermutationRecordsWrong: any other permutation records a
// wrong attempt. INIT-029/SPEC-003 AC3.
func TestOrderWrongPermutationRecordsWrong(t *testing.T) {
	f := setupOrderLesson(t, []string{orderAcceptLine()})
	run, ask := f.openAsk(t)
	pos := wrongPermutation(ask.OrderChips)
	act := f.lesson.Activities[0]
	if _, correct, ok := orderGrade(ask.OrderChips, pos, act); !ok || correct {
		t.Fatalf("fixture positions are not a wrong permutation: %v → %v", ask.OrderChips, pos)
	}
	rec := postOrder(t, f, strconv.FormatInt(run.ID, 10), pos)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST = %d\n%s", rec.Code, rec.Body.String())
	}
	if n := scalar(t, f.dbPath, `SELECT COUNT(*) FROM attempts WHERE activity = 'order' AND correct = 0`); n != 1 {
		t.Fatalf("wrong attempts = %d", n)
	}
	if n := scalar(t, f.dbPath, `SELECT COUNT(*) FROM cards`); n != 0 {
		t.Fatalf("wrong order scheduled a card: %d", n)
	}
}

// TestOrderBadPositionsAre400: missing, repeated, or unknown position is 400
// and writes nothing. INIT-029/SPEC-003 AC4.
func TestOrderBadPositionsAre400(t *testing.T) {
	cases := []struct {
		name string
		pos  []int
	}{
		{"missing", []int{0, 1}},
		{"repeated", []int{0, 0, 1}},
		{"unknown", []int{0, 1, 9}},
		{"empty", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupOrderLesson(t, nil)
			run, _ := f.openAsk(t)
			before := scalar(t, f.dbPath, `SELECT COUNT(*) FROM attempts`)
			var rec *httptest.ResponseRecorder
			if c.pos == nil {
				form := url.Values{"run": {strconv.FormatInt(run.ID, 10)}, "_csrf": {"brian-csrf"}}
				req := httptest.NewRequest(http.MethodPost, f.path, strings.NewReader(form.Encode()))
				req.Host = "learn.example"
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Set("HX-Request", "true")
				req.Header.Set("Origin", "https://learn.example")
				req.Header.Set("X-CSRF-Token", "brian-csrf")
				req.AddCookie(f.cookie)
				rec = httptest.NewRecorder()
				f.h.ServeHTTP(rec, req)
			} else {
				rec = postOrder(t, f, strconv.FormatInt(run.ID, 10), c.pos)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("POST = %d, want 400\n%s", rec.Code, rec.Body.String())
			}
			if got := scalar(t, f.dbPath, `SELECT COUNT(*) FROM attempts`); got != before {
				t.Fatalf("attempts wrote %d → %d", before, got)
			}
		})
	}
}

// TestOrderRevealHasReadingEnglishAndClip: reveal carries reading, English,
// and clip. INIT-029/SPEC-003 AC5.
func TestOrderRevealHasReadingEnglishAndClip(t *testing.T) {
	f := setupOrderLesson(t, nil)
	run, ask := f.openAsk(t)
	pos := positionsForTokens(ask.OrderChips, units.Tokenize(orderLine()))
	rec := postOrder(t, f, strconv.FormatInt(run.ID, 10), pos)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST = %d\n%s", rec.Code, rec.Body.String())
	}
	run, err := f.store.PlayerRunFor(context.Background(), f.userID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Phase != "reveal" {
		t.Fatalf("phase = %q", run.Phase)
	}
	p := player{store: f.store}
	screen, err := p.screenFor(context.Background(), views.NewPage("en", "brian-csrf", "lesson"), f.lesson, run, lessonQuery{})
	if err != nil {
		t.Fatal(err)
	}
	a := screen.Activity
	if a.Cue != "hello morning thanks" {
		t.Fatalf("reveal English = %q", a.Cue)
	}
	if a.Audio == "" || !strings.Contains(a.Audio, "/audio/") {
		t.Fatalf("reveal clip = %q", a.Audio)
	}
	if len(a.OrderChips) != 0 {
		t.Fatalf("reveal still has chips: %v", a.OrderChips)
	}
	gotReading := map[string]string{}
	for _, w := range a.Words {
		if w.Plain || w.Gap {
			continue
		}
		if !w.ShowTranslit || w.Translit == "" {
			t.Fatalf("word %q missing reading: %+v", w.Text, w)
		}
		gotReading[w.Text] = w.Translit
	}
	for _, tok := range units.Tokenize(orderLine()) {
		if gotReading[tok] == "" {
			t.Fatalf("no reading for %q in %+v", tok, a.Words)
		}
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
