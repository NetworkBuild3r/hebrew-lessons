package fsrs

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"

	gofsrs "github.com/open-spaced-repetition/go-fsrs/v3"
)

func TestModulePinned(t *testing.T) {
	if ModuleVersion != "v3.3.1" {
		t.Fatalf("ModuleVersion = %s, want v3.3.1", ModuleVersion)
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var body []byte
	for {
		candidate := filepath.Join(dir, "go.mod")
		body, err = os.ReadFile(candidate)
		if err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
	if !strings.Contains(string(body), "github.com/open-spaced-repetition/go-fsrs/v3 v3.3.1") {
		t.Fatalf("go.mod does not pin go-fsrs v3.3.1:\n%s", body)
	}
}

func TestGradeUpdatesCardAndLog(t *testing.T) {
	rev, store, clk := openReview(t)
	ctx := context.Background()
	userID := mustUser(t, store, "learner@example.com")
	lexemeID := mustLexeme(t, store, "שָׁלוֹם")
	cardID := mustCard(t, store, db.Card{
		UserID:   userID,
		LexemeID: lexemeID,
		Skill:    SkillRead,
		State:    StateNew,
		Due:      clk.Now(),
	})

	if err := rev.Grade(ctx, cardID, Good, true); err != nil {
		t.Fatal(err)
	}
	got, err := rev.Card(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	param := gofsrs.DefaultParam()
	param.EnableFuzz = false
	want := gofsrs.NewFSRS(param).Next(gofsrs.NewCard(), clk.Now(), gofsrs.Good)
	if got.State != StateLearning {
		t.Fatalf("state = %s, want learning", got.State)
	}
	if got.Stability != want.Card.Stability || got.Difficulty != want.Card.Difficulty {
		t.Fatalf("stability/difficulty = %v/%v, want %v/%v", got.Stability, got.Difficulty, want.Card.Stability, want.Card.Difficulty)
	}
	if got.Reps != int(want.Card.Reps) || got.Lapses != int(want.Card.Lapses) {
		t.Fatalf("reps/lapses = %d/%d, want %d/%d", got.Reps, got.Lapses, want.Card.Reps, want.Card.Lapses)
	}
	if !got.Due.Equal(want.Card.Due) {
		t.Fatalf("due = %s, want %s", got.Due, want.Card.Due)
	}
	if !got.LastReview.Equal(clk.Now()) {
		t.Fatalf("last review = %s, want the fake clock %s", got.LastReview, clk.Now())
	}
	if len(got.Ratings) != 1 || got.Ratings[0] != Good {
		t.Fatalf("ratings = %v", got.Ratings)
	}

	logs, err := store.ReviewLogs(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("log rows = %d, want 1", len(logs))
	}
	log := logs[0]
	if log.Rating != int(Good) || !log.Peeked || log.Chosen != "" {
		t.Fatalf("log rating/peeked/chosen = %d/%v/%q", log.Rating, log.Peeked, log.Chosen)
	}
	if !log.ReviewedAt.Equal(clk.Now()) {
		t.Fatalf("reviewed_at = %s, want %s", log.ReviewedAt, clk.Now())
	}
	if log.ElapsedDays != int(want.ReviewLog.ElapsedDays) || log.ScheduledDays != int(want.Card.ScheduledDays) {
		t.Fatalf("elapsed/scheduled = %d/%d, want %d/%d", log.ElapsedDays, log.ScheduledDays, want.ReviewLog.ElapsedDays, want.Card.ScheduledDays)
	}

	if err := rev.Grade(ctx, cardID, 5, false); err == nil {
		t.Fatal("rating 5 should fail")
	}
	logs, err = store.ReviewLogs(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("bad rating wrote a log row, rows = %d", len(logs))
	}
	if err := rev.Grade(ctx, 99999, Good, false); err == nil {
		t.Fatal("missing card should fail")
	}

	if err := rev.GradeWithChoice(ctx, cardID, Again, false, "שָׁלוֹם"); err != nil {
		t.Fatal(err)
	}
	logs, err = store.ReviewLogs(ctx, cardID)
	if err != nil || len(logs) != 2 || logs[1].Chosen != "שָׁלוֹם" || logs[1].Rating != int(Again) {
		t.Fatalf("choice log = %+v %v", logs, err)
	}
	if err := rev.RecordMiss(ctx, userID, MissLetterPair, "ב/כ", lexemeID); err != nil {
		t.Fatal(err)
	}
	stats, err := store.StruggleStats(ctx, userID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Confusions) != 1 || stats.Confusions[0].A != "ב" || stats.Confusions[0].B != "כ" || stats.Confusions[0].Count != 1 {
		t.Fatalf("confusions = %+v", stats.Confusions)
	}
	if len(stats.Lexemes) != 1 || len(stats.Lexemes[0].ConfusionKeys) != 1 || stats.Lexemes[0].ConfusionKeys[0] != "ב/כ" {
		t.Fatalf("lexeme = %+v", stats.Lexemes)
	}
}

func TestSuspendExcludesFromQueue(t *testing.T) {
	rev, store, clk := openReview(t)
	ctx := context.Background()
	userID := mustUser(t, store, "learner@example.com")
	dueID := mustCard(t, store, db.Card{
		UserID:   userID,
		LexemeID: mustLexeme(t, store, "שלום"),
		Skill:    SkillRead,
		Due:      clk.Now().Add(-time.Hour),
	})
	futureID := mustCard(t, store, db.Card{
		UserID:   userID,
		LexemeID: mustLexeme(t, store, "להתראות"),
		Skill:    SkillRead,
		Due:      clk.Now().Add(time.Hour),
	})

	if err := rev.Suspend(ctx, dueID, true); err != nil {
		t.Fatal(err)
	}
	due, err := store.ListDueCards(ctx, userID, clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Fatalf("suspended card was due: %+v", due)
	}
	if err := rev.Suspend(ctx, dueID, false); err != nil {
		t.Fatal(err)
	}
	due, err = store.ListDueCards(ctx, userID, clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].ID != dueID {
		t.Fatalf("due = %+v, want card %d", due, dueID)
	}
	for _, card := range due {
		if card.ID == futureID {
			t.Fatal("a card due in the future was queued")
		}
	}
	if err := rev.Suspend(ctx, 99999, true); err == nil {
		t.Fatal("missing card should fail")
	}
}

func TestShowUnpointed(t *testing.T) {
	good := []Rating{Good, Good, Good}
	cases := []struct {
		name string
		card Card
		want bool
	}{
		{name: "20.9 days", card: Card{Skill: SkillRead, Stability: 20.9, Ratings: good}, want: false},
		{name: "21 days", card: Card{Skill: SkillRead, Stability: 21, Ratings: good}, want: true},
		{name: "again in the last 3", card: Card{Skill: SkillRead, Stability: 40, Ratings: []Rating{Good, Again, Good, Easy}}, want: false},
		{name: "again older than the last 3", card: Card{Skill: SkillRead, Stability: 21, Ratings: []Rating{Again, Good, Hard, Easy}}, want: true},
		{name: "recall", card: Card{Skill: SkillRecall, Stability: 80, Ratings: good}, want: false},
		{name: "no reviews yet", card: Card{Skill: SkillRead, Stability: 21}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShowUnpointed(tc.card); got != tc.want {
				t.Fatalf("ShowUnpointed = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShowUnpointedUsesReviewLog(t *testing.T) {
	rev, store, clk := openReview(t)
	ctx := context.Background()
	cardID := mustCard(t, store, db.Card{
		UserID:   mustUser(t, store, "learner@example.com"),
		LexemeID: mustLexeme(t, store, "שלום"),
		Skill:    SkillRead,
		Due:      clk.Now(),
	})
	for _, rating := range []Rating{Good, Good, Again} {
		if err := rev.Grade(ctx, cardID, rating, false); err != nil {
			t.Fatal(err)
		}
	}
	card, err := rev.Card(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	if len(card.Ratings) != 3 || card.Ratings[0] != Good || card.Ratings[1] != Good || card.Ratings[2] != Again {
		t.Fatalf("ratings = %v, want Good Good Again", card.Ratings)
	}
	card.Stability = 21
	if ShowUnpointed(card) {
		t.Fatal("an Again in the last 3 should keep the card pointed")
	}
	card.Stability = 20.9
	card.Ratings = []Rating{Good, Good, Good}
	if ShowUnpointed(card) {
		t.Fatal("stability 20.9 should keep the card pointed")
	}
	card.Stability = 21
	if !ShowUnpointed(card) {
		t.Fatal("stability 21 with no Again should be unpointed")
	}
}

// TestQueueSchedulerRemoved keeps the study queue in internal/session.
// Letter and pattern drills are card kinds there. This package schedules.
func TestQueueSchedulerRemoved(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "queue.go" || name == "queue_test.go" {
			t.Fatalf("%s is still a second scheduler", name)
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "func (r *Review) Queue") {
			t.Fatalf("%s still builds a queue", name)
		}
	}
}

// TestLearnAheadElapsedUsesTheAnswerTime grades a learning card 15 minutes
// before its due time. Last review was 23h50m earlier, so the real elapsed
// day count is 0. The nominal due is 24h5m after the last review, which
// would record 1. The log stores the answer time.
func TestLearnAheadElapsedUsesTheAnswerTime(t *testing.T) {
	last := time.Date(2026, 10, 18, 22, 0, 0, 0, time.UTC)
	due := time.Date(2026, 10, 19, 22, 5, 0, 0, time.UTC)
	answer := time.Date(2026, 10, 19, 21, 50, 0, 0, time.UTC)
	nominal := int(math.Floor(due.Sub(last).Hours() / 24))
	real := int(math.Floor(answer.Sub(last).Hours() / 24))
	if nominal != 1 || real != 0 {
		t.Fatalf("fixture nominal elapsed %d, real elapsed %d", nominal, real)
	}
	if answer.After(due) || due.Sub(answer) > 20*time.Minute {
		t.Fatal("the answer is not inside a 20 minute learn-ahead of the due time")
	}

	rev, store, clk := openReview(t)
	clk.t = answer
	ctx := context.Background()
	userID := mustUser(t, store, "learner@example.com")
	lexemeID := mustLexeme(t, store, "שָׁלוֹם")
	cardID := mustCard(t, store, db.Card{
		UserID:     userID,
		LexemeID:   lexemeID,
		Skill:      SkillRead,
		State:      StateLearning,
		Due:        due,
		Stability:  1,
		Difficulty: 5,
		Reps:       1,
		LastReview: last,
	})
	if err := rev.Grade(ctx, cardID, Again, false); err != nil {
		t.Fatal(err)
	}
	logs, err := store.ReviewLogs(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("log rows = %d, want 1", len(logs))
	}
	if logs[0].ElapsedDays != real {
		t.Fatalf("elapsed_days = %d, want %d from the answer time", logs[0].ElapsedDays, real)
	}
	if !logs[0].ReviewedAt.Equal(answer) {
		t.Fatalf("reviewed_at = %s, want %s", logs[0].ReviewedAt, answer)
	}
}
