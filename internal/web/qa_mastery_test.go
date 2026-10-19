package web

import (
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Finishing a lesson writes lesson_runs and enqueues new cards at stability 0.
// It does not make those cards mature, and it does not mark them recalled.
func TestQALessonFinishIsNotMastery(t *testing.T) {
	const word = "צָב"
	store, h, now := openApp(t)
	ctx := t.Context()
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	addLexeme(t, store, word, "zephyrmail", "xqstav", "m", "")
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Probe", Situation: "A probe.", Position: 0,
		NewWords: []string{word},
		Lessons: []units.Lesson{{
			Kind:       units.KindLesson,
			Activities: []units.Activity{{Type: units.ActivityStory, Text: word}},
		}},
	}, "shared")
	publish(t, store, unitID)

	body := getAuthed(t, h, learner.cookie, "/lesson")
	runID := runValue(t, body)
	done := postLesson(t, h, learner.cookie, "/lesson/"+itoa(unitID)+"/0", runID, "continue", true, learner.csrf)
	if done.Code != 200 || !strings.Contains(done.Body.String(), "Lesson complete.") {
		t.Fatalf("finish = %d %s", done.Code, done.Body.String())
	}

	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var finished int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM lesson_runs
		WHERE user_id = ? AND unit_id = ? AND finished_at IS NOT NULL`,
		learner.user.ID, unitID).Scan(&finished); err != nil {
		t.Fatal(err)
	}
	if finished != 1 {
		t.Fatalf("finished lesson_runs = %d, want 1", finished)
	}

	due, err := store.ListDueCards(ctx, learner.user.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 2 {
		t.Fatalf("due cards = %d, want the two new directions", len(due))
	}
	var cards int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, learner.user.ID).Scan(&cards); err != nil {
		t.Fatal(err)
	}
	if cards != len(due) {
		t.Fatalf("cards = %d, due = %d", cards, len(due))
	}
	for _, card := range due {
		if card.Stability >= 21 || card.State != "new" || card.Lapses != 0 || card.Reps != 0 {
			t.Fatalf("introduced card is mature or already reviewed: %+v", card)
		}
		logs, err := store.ReviewLogs(ctx, card.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(logs) != 0 {
			t.Fatalf("card %d has reviews %+v", card.ID, logs)
		}
	}

	knownIDs, err := store.KnownLexemeIDs(ctx, learner.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	knownText, err := store.LearnerKnownLexemes(ctx, learner.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(knownIDs) != 0 || len(knownText) != 0 {
		t.Fatalf("known ids %v texts %v", knownIDs, knownText)
	}
	place, taken, err := store.LatestPlacement(ctx, learner.user.ID)
	if err != nil || taken || place.EstimatedKnownWords != 0 {
		t.Fatalf("placement taken=%v estimate=%d err=%v", taken, place.EstimatedKnownWords, err)
	}

	home := getAuthed(t, h, learner.cookie, "/")
	if !strings.Contains(home, `data-known="0"`) {
		t.Fatalf("Today counts the finished lesson as known:\n%s", home)
	}
	progress := getAuthed(t, h, learner.cookie, "/progress")
	if !strings.Contains(progress, `>0</bdi></span> <span lang="en">Words</span>`) {
		t.Fatalf("progress counts the finished lesson as words you can remember:\n%s", progress)
	}
	if strings.Contains(progress, "mastered") || strings.Contains(home, "mastered") {
		t.Fatal("a finished lesson is labeled mastered")
	}
}
