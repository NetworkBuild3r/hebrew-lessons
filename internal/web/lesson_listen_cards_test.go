package web

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

type listenCardFix struct {
	store  *db.Store
	dbPath string
	h      http.Handler
	cookie *http.Cookie
	userID int64
	ids    map[string]int64
	now    time.Time
	ctx    context.Context
}

func openListenCardFix(t *testing.T) listenCardFix {
	t.Helper()
	const (
		hello   = "שָׁלוֹם"
		morning = "בֹּקֶר"
		evening = "עֶרֶב"
		night   = "לַיְלָה"
	)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "hebrew.sqlite")
	store, err := db.Open(ctx, dbPath, fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	brian, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	sp := func(s string) *string { return &s }
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: hello, Gloss: "hello", Translit: sp("shalom"), POS: sp("noun")},
		{TextNFC: morning, Gloss: "morning", Translit: sp("boker"), POS: sp("noun")},
		{TextNFC: evening, Gloss: "evening", Translit: sp("erev"), POS: sp("noun")},
		{TextNFC: night, Gloss: "night", Translit: sp("layla"), POS: sp("noun")},
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	for _, row := range saved {
		ids[row.TextNFC] = row.ID
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: fixedClock(now), Store: store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return listenCardFix{
		store: store, dbPath: dbPath, h: NewOptions(nil, svc, Options{Store: store}),
		cookie: mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour)),
		userID: brian.ID, ids: ids, now: now, ctx: ctx,
	}
}

func (fx listenCardFix) publishListenLesson(t *testing.T, title, target string, fillers ...string) (unitID int64, path string) {
	t.Helper()
	acts := []units.Activity{{Type: units.ActivityListenPick, Text: target}}
	story := target
	for _, w := range fillers {
		story += " " + w
	}
	acts = append(acts, units.Activity{Type: units.ActivityStory, Text: story})
	unitID = putLesson(t, fx.store, nil, units.Unit{
		Title: title, Position: 1, NewWords: []string{target},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: acts}},
	}, "shared")
	publish(t, fx.store, unitID)
	return unitID, "/lesson/" + itoa(unitID) + "/0"
}

// INIT-023/SPEC-002 ac-1: a right lesson listen_pick on a word with no listen
// card creates one with one review_log row.
func TestListenPickRightCreatesListenCard(t *testing.T) {
	fx := openListenCardFix(t)
	const hello = "שָׁלוֹם"
	_, path := fx.publishListenLesson(t, "Right listen", hello, "בֹּקֶר", "עֶרֶב", "לַיְלָה")
	runID := runValue(t, getAuthed(t, fx.h, fx.cookie, path))
	if rec := postLesson(t, fx.h, fx.cookie, path, runID, itoa(fx.ids[hello]), true, "brian-csrf"); rec.Code != http.StatusOK {
		t.Fatalf("right answer = %d\n%s", rec.Code, rec.Body.String())
	}
	if scalar(t, fx.dbPath, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = 'listen'`, fx.userID, fx.ids[hello]) != 1 {
		t.Fatal("a right listen_pick did not create a listen card")
	}
	if scalar(t, fx.dbPath, `SELECT COUNT(*) FROM review_log`) != 1 {
		t.Fatal("a right listen_pick did not write one review_log row")
	}
	if scalar(t, fx.dbPath, `SELECT rating FROM review_log`) != 3 {
		t.Fatal("a right first listen_pick should rate Good")
	}
}

// INIT-023/SPEC-002 ac-2: a wrong first listen_pick creates no card.
func TestListenPickWrongFirstCreatesNoCard(t *testing.T) {
	fx := openListenCardFix(t)
	const morning = "בֹּקֶר"
	_, path := fx.publishListenLesson(t, "Wrong first listen", morning, "שָׁלוֹם", "עֶרֶב", "לַיְלָה")
	runID := runValue(t, getAuthed(t, fx.h, fx.cookie, path))
	if rec := postLesson(t, fx.h, fx.cookie, path, runID, itoa(fx.ids["שָׁלוֹם"]), true, "brian-csrf"); rec.Code != http.StatusOK {
		t.Fatalf("wrong answer = %d\n%s", rec.Code, rec.Body.String())
	}
	if scalar(t, fx.dbPath, `SELECT COUNT(*) FROM attempts WHERE lexeme_id = ? AND skill = 'listen' AND correct = 0`, fx.ids[morning]) != 1 {
		t.Fatal("a wrong first listen_pick did not store the attempt")
	}
	if scalar(t, fx.dbPath, `SELECT COUNT(*) FROM cards WHERE skill = 'listen'`) != 0 {
		t.Fatal("a wrong first listen_pick created a listen card")
	}
	if scalar(t, fx.dbPath, `SELECT COUNT(*) FROM review_log`) != 0 {
		t.Fatal("a wrong first listen_pick wrote a review_log row")
	}
}

// INIT-023/SPEC-002 ac-2: a wrong listen_pick on a word that already has a
// listen card rates Again on that card.
func TestListenPickWrongRatesExistingCardAgain(t *testing.T) {
	fx := openListenCardFix(t)
	const evening = "עֶרֶב"
	_, path := fx.publishListenLesson(t, "Wrong existing listen", evening, "שָׁלוֹם", "בֹּקֶר", "לַיְלָה")
	if _, err := fx.store.InsertCard(fx.ctx, db.Card{
		UserID: fx.userID, LexemeID: fx.ids[evening], Skill: fsrs.SkillListen,
		State: "review", Due: fx.now.Add(-time.Hour), Stability: 1, Difficulty: 5, Reps: 1,
		LastReview: fx.now.Add(-24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	runID := runValue(t, getAuthed(t, fx.h, fx.cookie, path))
	if rec := postLesson(t, fx.h, fx.cookie, path, runID, itoa(fx.ids["שָׁלוֹם"]), true, "brian-csrf"); rec.Code != http.StatusOK {
		t.Fatalf("wrong answer = %d\n%s", rec.Code, rec.Body.String())
	}
	if scalar(t, fx.dbPath, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = 'listen'`, fx.userID, fx.ids[evening]) != 1 {
		t.Fatal("a wrong listen_pick duplicated or dropped the listen card")
	}
	if scalar(t, fx.dbPath, `SELECT COUNT(*) FROM review_log`) != 1 {
		t.Fatal("a wrong listen_pick on an existing card did not append a review_log row")
	}
	if scalar(t, fx.dbPath, `SELECT rating FROM review_log`) != 1 {
		t.Fatal("a wrong listen_pick on an existing card did not rate Again")
	}
}
