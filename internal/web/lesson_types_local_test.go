package web

import (
	"context"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// TestLocalLessonTypes opens one lesson step for every activity the player
// has, which is what the five design types are built from.
func TestLocalLessonTypes(t *testing.T) {
	const (
		peace   = "שָׁלוֹם"
		morning = "בֹּקֶר"
		water   = "מַיִם"
	)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShellAt(t, fixedClock(now), "https://learn.example", nil, nil)
	ctx := context.Background()
	sp := func(s string) *string { return &s }
	if _, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: peace, Gloss: "peace, hello", Translit: sp("shalom"), POS: sp("noun"), Gender: sp("m")},
		{TextNFC: morning, Gloss: "morning", Translit: sp("boker"), POS: sp("noun"), Gender: sp("m")},
		{TextNFC: water, Gloss: "water", Translit: sp("mayim"), POS: sp("noun"), Gender: sp("m")},
	}); err != nil {
		t.Fatal(err)
	}
	learner := addLearner(t, store, "learner1@example.com", "sub-local", now)
	peaceID, _, _, _, ok, err := store.LexemeByText(ctx, peace)
	if err != nil || !ok {
		t.Fatal(err, ok)
	}
	line := peace + " " + morning + " " + water
	steps := []struct {
		name   string
		design string
		act    units.Activity
		want   string
		answer string
	}{
		{"story", "word · read, and the easy lesson", units.Activity{Type: units.ActivityStory, Text: line}, "Read the story", "continue"},
		{"recall", "word · check", units.Activity{Type: units.ActivityRecall, Text: peace}, "What does this Hebrew mean?", fmt.Sprint(peaceID)},
		{"listen_pick", "listening", units.Activity{Type: units.ActivityListenPick, Text: peace}, "Listen, then pick the meaning.", "continue"},
		{"produce", "word · write, typed", units.Activity{Type: units.ActivityProduce, Text: peace}, "data-hk-input", peace},
		{"cloze", "word · use, shown not scored", units.Activity{Type: units.ActivityCloze, Text: line, Target: peace, Translation: "peace in the morning"}, peace, "continue"},
		{"order", "word · use, shown not scored", units.Activity{Type: units.ActivityOrder, Text: line, Translation: "peace morning water"}, "Put the words in order.", "continue"},
		{"say", "speaking, not graded", units.Activity{Type: units.ActivitySay, Text: peace, PromptEN: "peace"}, "Say it out loud", "continue"},
		{"shadow", "listen and repeat", units.Activity{Type: units.ActivityShadow, Text: line}, "Listen, then say the line.", "continue"},
		{"pattern", "conjugation", units.Activity{Type: units.ActivityPattern, Text: line}, "Read the example.", "continue"},
		{"dialogue", "not one of the five types", units.Activity{Type: units.ActivityDialogue, Lines: []units.DialogueLine{
			{Speaker: "a", Text: peace},
			{Speaker: "b", Text: morning},
		}}, "Listen to the conversation", "continue"},
	}
	acts := make([]units.Activity, len(steps))
	for i, step := range steps {
		acts[i] = step.act
	}
	id := putLesson(t, store, nil, units.Unit{
		Title: "Every step", Situation: "A local check.", Concept: "One of each", Position: 0,
		NewWords: []string{peace, morning, water},
		Lessons:  []units.Lesson{{Kind: units.KindLesson, Activities: acts}},
	}, "shared")
	publish(t, store, id)
	path := fmt.Sprintf("/lesson/%d/0", id)
	body := getAuthed(t, h, learner.cookie, path)
	runID := runValue(t, body)
	for _, step := range steps {
		shown := strings.Contains(body, step.want)
		if shown {
			t.Logf("ok  %-12s  %s", step.name, step.design)
		} else if step.name == "story" || step.name == "recall" || step.name == "produce" {
			t.Errorf("%s (%s) did not show %q", step.name, step.design, step.want)
		} else {
			t.Logf("gap %-12s  %s  (screen was step %s)", step.name, step.design, dataStep(body))
		}
		before := dataStep(body)
		posted := postLesson(t, h, learner.cookie, path, runID, step.answer, true, learner.csrf)
		if posted.Code != 200 {
			t.Fatalf("%s advance = %d", step.name, posted.Code)
		}
		body = posted.Body.String()
		if dataStep(body) == before {
			posted = postLesson(t, h, learner.cookie, path, runID, fmt.Sprint(peaceID), true, learner.csrf)
			if posted.Code == 200 {
				body = posted.Body.String()
			}
		}
		if dataStep(body) == before {
			posted = postLesson(t, h, learner.cookie, path, runID, "continue", true, learner.csrf)
			if posted.Code == 200 {
				body = posted.Body.String()
			}
		}
	}
	if !strings.Contains(body, "Lesson complete") && !strings.Contains(body, "complete") {
		t.Logf("last screen did not say complete:\n%s", clip(body, 400))
	}
}

func dataStep(body string) string {
	const key = `data-step="`
	i := strings.Index(body, key)
	if i < 0 {
		return ""
	}
	rest := body[i+len(key):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// TestServeLocalLessons keeps a local player up when LOCAL_LESSONS=1.
func TestServeLocalLessons(t *testing.T) {
	if os.Getenv("LOCAL_LESSONS") != "1" {
		t.Skip("set LOCAL_LESSONS=1 to keep the local player running")
	}
	const (
		peace   = "שָׁלוֹם"
		morning = "בֹּקֶר"
		water   = "מַיִם"
	)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ln, err := net.Listen("tcp", "127.0.0.1:8787")
	if err != nil {
		t.Fatal(err)
	}
	origin := "http://127.0.0.1:8787"
	store, h := openShellAt(t, fixedClock(now), origin, nil, nil)
	ctx := context.Background()
	sp := func(s string) *string { return &s }
	if _, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: peace, Gloss: "peace, hello", Translit: sp("shalom"), POS: sp("noun"), Gender: sp("m")},
		{TextNFC: morning, Gloss: "morning", Translit: sp("boker"), POS: sp("noun"), Gender: sp("m")},
		{TextNFC: water, Gloss: "water", Translit: sp("mayim"), POS: sp("noun"), Gender: sp("m")},
	}); err != nil {
		t.Fatal(err)
	}
	learner := addLearner(t, store, "learner1@example.com", "sub-local", now)
	line := peace + " " + morning + " " + water
	var first int64
	for i, act := range []units.Activity{
		{Type: units.ActivityStory, Text: line},
		{Type: units.ActivityRecall, Text: peace},
		{Type: units.ActivityListenPick, Text: peace},
		{Type: units.ActivityProduce, Text: peace},
		{Type: units.ActivityCloze, Text: line, Target: peace, Translation: "peace in the morning"},
		{Type: units.ActivityOrder, Text: line, Translation: "peace morning water"},
		{Type: units.ActivitySay, Text: peace, PromptEN: "peace"},
		{Type: units.ActivityShadow, Text: line},
		{Type: units.ActivityPattern, Text: line},
		{Type: units.ActivityDialogue, Lines: []units.DialogueLine{{Speaker: "a", Text: peace}, {Speaker: "b", Text: morning}}},
	} {
		id := putLesson(t, store, nil, units.Unit{
			Title: act.Type, Situation: "A local check.", Concept: "One step", Position: i,
			NewWords: []string{peace},
			Lessons:  []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{act}}},
		}, "shared")
		publish(t, store, id)
		if first == 0 {
			first = id
		}
		fmt.Printf("LESSON %s %s/lesson/%d/0\n", act.Type, origin, id)
	}
	srv := httptest.NewUnstartedServer(h)
	srv.Listener = ln
	srv.Start()
	defer srv.Close()
	fmt.Printf("COOKIE hebrew_session=%s\n", learner.cookie.Value)
	fmt.Printf("OPEN %s/lesson/%d/0\n", origin, first)
	select {}
}
