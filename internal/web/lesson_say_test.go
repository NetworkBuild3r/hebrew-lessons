package web

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestSayStepHidesHebrewUntilTheLearnerSpeaks(t *testing.T) {
	const word = "\u05e9\u05b8\u05dc\u05d5\u05b9\u05dd"
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "hebrew.sqlite")
	store, err := db.Open(ctx, dbPath, clk)
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
		{TextNFC: word, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
	}); err != nil {
		t.Fatal(err)
	}
	clipID := strings.Repeat("ab", 32)
	if _, err := store.InsertAudio(ctx, db.Asset{
		ID: clipID, TextNFC: units.Normalize(word), Voice: "voice", Model: "eleven_v3", Bytes: 4,
	}); err != nil {
		t.Fatal(err)
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Greeting", Situation: "A hello.", Concept: "Say hello.", Position: 1,
		NewWords: []string{word},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{{
			Type: units.ActivitySay, Text: word, PromptEN: "good morning",
		}}}},
	}, "shared")
	publish(t, store, unitID)
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: clk, Store: store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	cookie := mintSession(t, store, user.ID, "brian-csrf", now.Add(time.Hour))
	path := "/lesson/" + itoa(unitID) + "/0"

	ask := activityHTML(t, getAuthed(t, h, cookie, path))
	if containsHebrew(ask) || strings.Contains(ask, "<audio ") {
		t.Fatalf("ask screen shows Hebrew or audio:\n%s", ask)
	}
	for _, want := range []string{"good morning", "Say it out loud, then hear it.", `value="said"`, "I said it", `class="say-cue"`} {
		if !strings.Contains(ask, want) {
			t.Errorf("ask missing %s", want)
		}
	}

	runID := runValue(t, ask)
	revealed := activityHTML(t, postLesson(t, h, cookie, path, runID, "said", true, "brian-csrf").Body.String())
	if !strings.Contains(revealed, word) || !strings.Contains(revealed, "<audio ") || !strings.Contains(revealed, "data-autoplay") {
		t.Fatalf("reveal is missing the model or its clip:\n%s", revealed)
	}
	if !strings.Contains(revealed, `value="continue"`) || (!strings.Contains(revealed, "Finish") && !strings.Contains(revealed, "Continue")) {
		t.Fatalf("reveal is missing Continue:\n%s", revealed)
	}
	if !strings.Contains(revealed, `data-pace-audio="line-clip"`) || !strings.Contains(revealed, "Slower") {
		t.Fatalf("reveal is missing PaceControls:\n%s", revealed)
	}
	for _, banned := range []string{`value="got"`, `value="notyet"`, "Got it", "Not yet", "How did that go?"} {
		if strings.Contains(revealed, banned) {
			t.Fatalf("reveal still has self-grade %q:\n%s", banned, revealed)
		}
	}
	if strings.Contains(revealed, `value="said"`) {
		t.Fatal("reveal still offers I said it")
	}
	for _, bad := range []string{"lf-result--right", "lf-result--wrong", "act-grade"} {
		if strings.Contains(revealed, bad) {
			t.Fatalf("reveal shows grade styling %q:\n%s", bad, revealed)
		}
	}

	attemptsBefore := scalar(t, dbPath, `SELECT COUNT(*) FROM attempts`)
	reviewsBefore := scalar(t, dbPath, `SELECT COUNT(*) FROM review_log`)
	correctBefore := scalar(t, dbPath, `SELECT correct_n FROM lesson_runs WHERE id = `+runID)

	stuck := postLesson(t, h, cookie, path, runID, "got", true, "brian-csrf")
	if stuck.Code != 200 {
		t.Fatalf("got post = %d", stuck.Code)
	}
	stuckHTML := activityHTML(t, stuck.Body.String())
	if !strings.Contains(stuckHTML, `value="continue"`) || strings.Contains(stuckHTML, "Lesson complete") {
		t.Fatalf("got must not advance the say reveal:\n%s", stuckHTML)
	}
	if n := scalar(t, dbPath, `SELECT correct_n FROM lesson_runs WHERE id = `+runID); n != correctBefore {
		t.Fatalf("got changed correct_n: before=%d after=%d", correctBefore, n)
	}

	done := postLesson(t, h, cookie, path, runID, "continue", true, "brian-csrf")
	if done.Code != 200 {
		t.Fatalf("continue = %d\n%s", done.Code, done.Body.String())
	}
	if !strings.Contains(done.Body.String(), "Lesson complete") {
		t.Fatalf("say step did not advance:\n%s", done.Body.String())
	}
	// Lesson-end may still seed new-word cards; the say continue itself must
	// write no attempt and no review_log (INIT-036/SPEC-004).
	if n := scalar(t, dbPath, `SELECT COUNT(*) FROM attempts`); n != attemptsBefore {
		t.Fatalf("attempts changed: before=%d after=%d", attemptsBefore, n)
	}
	if n := scalar(t, dbPath, `SELECT COUNT(*) FROM attempts WHERE activity = 'say'`); n != 0 {
		t.Fatalf("say attempts = %d", n)
	}
	if n := scalar(t, dbPath, `SELECT COUNT(*) FROM review_log`); n != reviewsBefore {
		t.Fatalf("review_log changed: before=%d after=%d", reviewsBefore, n)
	}
	if n := scalar(t, dbPath, `SELECT correct_n FROM lesson_runs WHERE id = `+runID); n != correctBefore {
		t.Fatalf("continue changed correct_n: before=%d after=%d", correctBefore, n)
	}
}

// TestSayStepWritesNoLearnerEvidence is INIT-036/SPEC-004: finishing a say
// step leaves attempts, review_log, and cards unchanged while advancing.
func TestSayStepWritesNoLearnerEvidence(t *testing.T) {
	const word = "\u05e9\u05b8\u05dc\u05d5\u05b9\u05dd"
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "hebrew.sqlite")
	store, err := db.Open(ctx, dbPath, clk)
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
		{TextNFC: word, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
	}); err != nil {
		t.Fatal(err)
	}
	clipID := strings.Repeat("cd", 32)
	if _, err := store.InsertAudio(ctx, db.Asset{
		ID: clipID, TextNFC: units.Normalize(word), Voice: "voice", Model: "eleven_v3", Bytes: 4,
	}); err != nil {
		t.Fatal(err)
	}
	nextWord := "\u05d1\u05b9\u05bc\u05e7\u05b6\u05e8"
	if _, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: nextWord, Gloss: "morning", Translit: sp("boker"), Gender: sp("m"), POS: sp("noun")},
	}); err != nil {
		t.Fatal(err)
	}
	nextClip := strings.Repeat("ef", 32)
	if _, err := store.InsertAudio(ctx, db.Asset{
		ID: nextClip, TextNFC: units.Normalize(nextWord), Voice: "voice", Model: "eleven_v3", Bytes: 4,
	}); err != nil {
		t.Fatal(err)
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Greeting", Situation: "A hello.", Concept: "Say hello.", Position: 1,
		NewWords: []string{word, nextWord},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivitySay, Text: word, PromptEN: "good morning"},
			{Type: units.ActivityShadow, Text: nextWord},
		}}},
	}, "shared")
	publish(t, store, unitID)
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: clk, Store: store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	cookie := mintSession(t, store, user.ID, "brian-csrf", now.Add(time.Hour))
	path := "/lesson/" + itoa(unitID) + "/0"

	ask := activityHTML(t, getAuthed(t, h, cookie, path))
	runID := runValue(t, ask)
	revealed := activityHTML(t, postLesson(t, h, cookie, path, runID, "said", true, "brian-csrf").Body.String())
	if !strings.Contains(revealed, "<audio ") || !strings.Contains(revealed, "data-autoplay") {
		t.Fatalf("model did not play on reveal:\n%s", revealed)
	}

	attemptsBefore := scalar(t, dbPath, `SELECT COUNT(*) FROM attempts`)
	reviewsBefore := scalar(t, dbPath, `SELECT COUNT(*) FROM review_log`)
	cardsBefore := scalar(t, dbPath, `SELECT COUNT(*) FROM cards`)

	next := postLesson(t, h, cookie, path, runID, "continue", true, "brian-csrf")
	if next.Code != 200 {
		t.Fatalf("continue = %d\n%s", next.Code, next.Body.String())
	}
	html := activityHTML(t, next.Body.String())
	if !strings.Contains(html, nextWord) {
		t.Fatalf("did not advance to the next step:\n%s", html)
	}
	if n := scalar(t, dbPath, `SELECT COUNT(*) FROM attempts`); n != attemptsBefore {
		t.Fatalf("attempts changed: before=%d after=%d", attemptsBefore, n)
	}
	if n := scalar(t, dbPath, `SELECT COUNT(*) FROM review_log`); n != reviewsBefore {
		t.Fatalf("review_log changed: before=%d after=%d", reviewsBefore, n)
	}
	if n := scalar(t, dbPath, `SELECT COUNT(*) FROM cards`); n != cardsBefore {
		t.Fatalf("cards changed: before=%d after=%d", cardsBefore, n)
	}
}
