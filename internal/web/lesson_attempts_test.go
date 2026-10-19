package web

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// TestLessonChecksWriteAttempts is the lesson-player path check for
// INIT-015/SPEC-005. Every checked step writes one attempts row with the
// step's skill, activity, source lesson, and the unit. A repeated post of the
// same answer writes nothing. The learner id is the signed-in learner. The
// lesson still gives its new words cards only when it ends, as in v2.
func TestLessonChecksWriteAttempts(t *testing.T) {
	const (
		hello   = "שָׁלוֹם"
		morning = "בֹּקֶר"
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
	other, err := store.CreateUser(ctx, "other@example.com", "sub-other", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	sp := func(s string) *string { return &s }
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: hello, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: morning, Gloss: "morning", Translit: sp("boker"), Gender: sp("m"), POS: sp("noun")},
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	for _, row := range saved {
		ids[row.TextNFC] = row.ID
	}
	line := hello + " " + morning
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Greetings", Situation: "Saying hello.", Position: 1,
		NewWords: []string{hello, morning},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: hello},
			{Type: units.ActivityListenPick, Text: line},
			{Type: units.ActivityProduce, Text: hello},
			{Type: units.ActivityCloze, Text: line},
			{Type: units.ActivityStory, Text: line},
		}}},
	}, "shared")
	publish(t, store, unitID)

	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         fixedClock(now),
		Store:         store,
		AllowedEmails: []string{"brian@example.com", "other@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	cookie := mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour))
	otherCookie := mintSession(t, store, other.ID, "other-csrf", now.Add(time.Hour))
	path := "/lesson/" + itoa(unitID) + "/0"
	runID := runValue(t, getAuthed(t, h, cookie, path))
	post := func(answer string) {
		t.Helper()
		if rec := postLesson(t, h, cookie, path, runID, answer, true, "brian-csrf"); rec.Code != http.StatusOK {
			t.Fatalf("POST %q = %d\n%s", answer, rec.Code, rec.Body.String())
		}
	}
	attempts := func() int {
		return scalar(t, dbPath, `SELECT COUNT(*) FROM attempts`)
	}

	post(itoa(ids[morning])) // recall: wrong pick
	if attempts() != 1 {
		t.Fatalf("attempts after one answer = %d, want 1", attempts())
	}
	post(itoa(ids[morning])) // the same step again: a repeat, not a second answer
	if attempts() != 1 {
		t.Fatalf("a repeated post wrote an attempt: %d", attempts())
	}
	post("continue")
	post(itoa(ids[hello])) // listen_pick: right
	post("continue")
	post(itoa(ids[morning])) // produce: wrong pick
	post("continue")
	post(itoa(ids[hello])) // cloze: right
	post("continue")
	// The story step is not checked.
	if attempts() != 4 {
		t.Fatalf("attempts after four checked steps = %d, want 4", attempts())
	}

	// Lesson checks schedule nothing. The cards come when the lesson ends, as in v2.
	if scalar(t, dbPath, `SELECT COUNT(*) FROM review_log`) != 0 || scalar(t, dbPath, `SELECT COUNT(*) FROM cards`) != 0 {
		t.Fatal("a lesson check wrote a card or a review_log row")
	}
	post("continue") // the story step ends the lesson
	if scalar(t, dbPath, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, brian.ID) != 4 {
		t.Fatal("finishing the lesson did not enqueue a read and a recall card for each new word")
	}
	if attempts() != 4 {
		t.Fatalf("finishing wrote an attempt: %d", attempts())
	}

	rows, err := store.ListAttempts(ctx, brian.ID, ids[hello])
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		skill, activity, response string
		correct                   bool
	}
	wants := []want{
		{"read", "recall", morning, false},
		{"listen", "listen_pick", hello, true},
		{"recall", "produce", morning, false},
		{"read", "cloze", hello, true},
	}
	if len(rows) != len(wants) {
		t.Fatalf("attempts for the target word = %d, want %d", len(rows), len(wants))
	}
	for i, w := range wants {
		got := rows[i]
		if got.Skill != w.skill || got.Activity != w.activity || got.Source != "lesson" || got.UnitID != unitID ||
			got.Correct != w.correct || got.DontKnow || got.HintUsed || got.UserID != brian.ID ||
			got.Response != w.response || got.Prompt == "" || !got.CreatedAt.Equal(now) {
			t.Fatalf("attempt %d = %+v, want %+v", i, got, w)
		}
	}
	if n := scalar(t, dbPath, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, other.ID); n != 0 {
		t.Fatalf("the other learner has %d attempts", n)
	}

	// Another learner cannot answer on this run, and nothing is written.
	before := attempts()
	if rec := postLesson(t, h, otherCookie, path, runID, itoa(ids[hello]), true, "other-csrf"); rec.Code != http.StatusNotFound {
		t.Fatalf("other learner post = %d", rec.Code)
	}
	if attempts() != before {
		t.Fatal("a refused post wrote an attempt")
	}
}

// A forged answer is wrong, is stored cut to 200 characters, and cannot break the write.
func TestLessonForgedAnswerIsStoredBounded(t *testing.T) {
	const hello = "שָׁלוֹם"
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
		{TextNFC: hello, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
	})
	if err != nil {
		t.Fatal(err)
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Greetings", Position: 1, NewWords: []string{hello},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: hello},
		}}},
	}, "shared")
	publish(t, store, unitID)
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: fixedClock(now), Store: store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	cookie := mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour))
	path := "/lesson/" + itoa(unitID) + "/0"
	runID := runValue(t, getAuthed(t, h, cookie, path))

	forged := "<b>" + strings.Repeat("x", 1000)
	if rec := postLesson(t, h, cookie, path, runID, forged, true, "brian-csrf"); rec.Code != http.StatusOK {
		t.Fatalf("forged answer = %d", rec.Code)
	}
	rows, err := store.ListAttempts(ctx, brian.ID, saved[0].ID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("attempts = %+v, %v", rows, err)
	}
	if rows[0].Correct || len([]rune(rows[0].Response)) != 200 || !strings.HasPrefix(rows[0].Response, "<b>xxx") {
		t.Fatalf("forged attempt = correct %v, %d characters", rows[0].Correct, len([]rune(rows[0].Response)))
	}
}
