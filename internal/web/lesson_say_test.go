package web

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestSayStepHidesHebrewUntilTheLearnerSpeaks(t *testing.T) {
	const word = "\u05e9\u05b8\u05dc\u05d5\u05b9\u05dd"
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	ctx := context.Background()
	store, err := db.Open(ctx, t.TempDir()+"/hebrew.sqlite", clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	sp := func(s string) *string { return &s }
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: word, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
	})
	if err != nil {
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
	for _, want := range []string{"good morning", "Say it in Hebrew", `value="said"`, "I said it", `class="say-cue"`} {
		if !strings.Contains(ask, want) {
			t.Errorf("ask missing %s", want)
		}
	}

	runID := runValue(t, ask)
	revealed := activityHTML(t, postLesson(t, h, cookie, path, runID, "said", true, "brian-csrf").Body.String())
	if !strings.Contains(revealed, word) || !strings.Contains(revealed, "<audio ") || !strings.Contains(revealed, "data-autoplay") {
		t.Fatalf("reveal is missing the model or its clip:\n%s", revealed)
	}
	if !strings.Contains(revealed, `value="got"`) || !strings.Contains(revealed, `value="notyet"`) || !strings.Contains(revealed, "Got it") || !strings.Contains(revealed, "Not yet") {
		t.Fatalf("reveal is missing the self-check:\n%s", revealed)
	}
	if strings.Contains(revealed, `value="said"`) {
		t.Fatal("reveal still offers I said it")
	}

	done := postLesson(t, h, cookie, path, runID, "got", true, "brian-csrf")
	if done.Code != 200 {
		t.Fatalf("self-check = %d\n%s", done.Code, done.Body.String())
	}
	rows, err := store.ListAttempts(ctx, user.ID, saved[0].ID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("attempts = %+v, %v", rows, err)
	}
	row := rows[0]
	if row.Activity != evidence.ActivitySay || row.Source != evidence.SourceSelf || row.Skill != fsrs.SkillRecall || !row.Correct || row.Response != "got it" || row.Prompt != "good morning" {
		t.Fatalf("attempt = %+v", row)
	}
	again := postLesson(t, h, cookie, path, runID, "notyet", true, "brian-csrf")
	if again.Code != 200 {
		t.Fatalf("repeat = %d", again.Code)
	}
	rows, err = store.ListAttempts(ctx, user.ID, saved[0].ID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("a second post wrote another attempt: %+v, %v", rows, err)
	}
}
