package web

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestDialogueSideHidesTheLearnersLine(t *testing.T) {
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
		{TextNFC: dlgHello, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: dlgMorning, Gloss: "morning", Translit: sp("boker"), Gender: sp("m"), POS: sp("noun")},
	})
	if err != nil {
		t.Fatal(err)
	}
	var morningID int64
	for _, row := range saved {
		if row.TextNFC == dlgMorning {
			morningID = row.ID
		}
	}
	for _, c := range []struct{ id, text, voice string }{
		{strings.Repeat("a1", 32), dlgHello, dlgFemale},
		{strings.Repeat("b2", 32), dlgMorning, dlgMale},
	} {
		if _, err := store.InsertAudio(ctx, db.Asset{ID: c.id, TextNFC: c.text, Voice: c.voice, Model: "eleven_v3", Bytes: 4}); err != nil {
			t.Fatal(err)
		}
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Greeting", Situation: "Two people meet.", Concept: "A greeting.", Position: 1,
		NewWords: []string{dlgHello, dlgMorning},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{{
			Type: units.ActivityDialogue,
			Play: units.SpeakerB,
			Lines: []units.DialogueLine{
				{Speaker: units.SpeakerA, Text: dlgHello},
				{Speaker: units.SpeakerB, Text: dlgMorning, PromptEN: "good morning"},
			},
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
	h := NewOptions(nil, svc, Options{Store: store, Voices: audio.Voices{Female: dlgFemale, Male: dlgMale}})
	cookie := mintSession(t, store, user.ID, "brian-csrf", now.Add(time.Hour))
	path := "/lesson/" + itoa(unitID) + "/0"

	first := activityHTML(t, getAuthed(t, h, cookie, path))
	if !strings.Contains(first, dlgHello) || !strings.Contains(first, `value="next"`) {
		t.Fatalf("the other speaker's line is not shown:\n%s", first)
	}
	runID := runValue(t, first)
	ask := activityHTML(t, postLesson(t, h, cookie, path, runID, "next", true, "brian-csrf").Body.String())
	if containsHebrew(ask) || strings.Contains(ask, "<audio ") {
		t.Fatalf("the learner's line shows Hebrew or audio before they speak:\n%s", ask)
	}
	if !strings.Contains(ask, "good morning") || !strings.Contains(ask, `value="said"`) {
		t.Fatalf("the cue is missing:\n%s", ask)
	}
	back := activityHTML(t, getAuthed(t, h, cookie, path))
	if containsHebrew(back) || !strings.Contains(back, "good morning") {
		t.Fatalf("leaving did not resume on the learner's line:\n%s", back)
	}
	revealed := activityHTML(t, postLesson(t, h, cookie, path, runID, "said", true, "brian-csrf").Body.String())
	if !strings.Contains(revealed, dlgMorning) || !strings.Contains(revealed, "<audio ") || !strings.Contains(revealed, `value="got"`) {
		t.Fatalf("reveal is missing the line:\n%s", revealed)
	}
	done := postLesson(t, h, cookie, path, runID, "got", true, "brian-csrf")
	if done.Code != 200 || !strings.Contains(done.Body.String(), "Lesson complete") {
		t.Fatalf("the dialogue did not finish:\n%s", done.Body.String())
	}
	rows, err := store.ListAttempts(ctx, user.ID, morningID)
	if err != nil || len(rows) != 1 || rows[0].Activity != evidence.ActivitySay || rows[0].Source != evidence.SourceSelf || rows[0].Prompt != "good morning" {
		t.Fatalf("attempts = %+v, %v", rows, err)
	}
}
