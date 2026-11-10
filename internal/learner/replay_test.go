package learner

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// gradeCard inserts a new card and applies ratings through fsrs.Review.Grade,
// advancing the fake clock by step between grades. Returns the card id.
func (e *env) gradeCard(lexeme int64, skill string, step time.Duration, ratings ...fsrs.Rating) int64 {
	e.t.Helper()
	ctx := context.Background()
	id, err := e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lexeme, Skill: skill,
		State: fsrs.StateNew, Due: e.clk.Now(),
	})
	if err != nil {
		e.t.Fatal(err)
	}
	rev := fsrs.New(e.store, e.clk)
	for i, r := range ratings {
		if i > 0 && step > 0 {
			e.clk.t = e.clk.t.Add(step)
		}
		if err := rev.Grade(ctx, id, r, false); err != nil {
			e.t.Fatalf("grade %d: %v", r, err)
		}
	}
	return id
}

func (e *env) implyGood(cardID int64) {
	e.t.Helper()
	rec := evidence.New(e.store, fsrs.New(e.store, e.clk), e.clk)
	if err := rec.Implied(context.Background(), e.user, cardID, fsrs.Good); err != nil {
		e.t.Fatal(err)
	}
}

// Replay of a Grade path matches the stored card; implied Good does too.
func TestReplayHistoryMatchesStoredCard(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lex := e.lexeme()
	id := e.gradeCard(lex, fsrs.SkillRead, 24*time.Hour, fsrs.Good, fsrs.Good, fsrs.Again, fsrs.Good)
	e.clk.t = e.clk.t.Add(time.Hour)
	e.implyGood(id)

	histories, err := e.store.CardHistories(ctx, e.user, time.Time{})
	if err != nil || len(histories) != 1 {
		t.Fatalf("histories = %v, %v", histories, err)
	}
	zone, err := e.store.StudyZone(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := replayHistory(replayScheduler(), histories[0], zone)
	if err != nil || !ok {
		t.Fatal(err, ok)
	}
	stored, err := e.store.GetCard(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if cardReplayMismatch(stored, got) {
		t.Fatalf("mismatch:\nstored=%+v\nreplay=%+v", stored, got)
	}
	if stored.Lapses < 1 {
		t.Fatalf("lapses = %d, want at least 1 from Again", stored.Lapses)
	}
}

func TestReplayEmptyHistory(t *testing.T) {
	_, ok, err := replayHistory(replayScheduler(), db.CardHistory{CardID: 1}, studyday.Zone{})
	if err != nil || ok {
		t.Fatalf("empty: ok=%v err=%v", ok, err)
	}
}

func TestCardReplayMismatchTolerance(t *testing.T) {
	base := db.Card{
		State: fsrs.StateReview, Stability: 10, Difficulty: 5,
		Reps: 3, Lapses: 1, Due: testNow, LastReview: testNow.Add(-time.Hour),
	}
	same := base
	same.Stability += replayStabilityEps / 2
	same.Difficulty -= replayDifficultyEps / 2
	same.Due = testNow.Add(replayDueSlack / 2)
	same.LastReview = base.LastReview.Add(replayDueSlack / 2)
	if cardReplayMismatch(base, same) {
		t.Fatal("within epsilon should match")
	}
	diffStab := base
	diffStab.Stability += replayStabilityEps * 2
	if !cardReplayMismatch(base, diffStab) {
		t.Fatal("stability drift should mismatch")
	}
	diffDue := base
	diffDue.Due = testNow.Add(replayDueSlack + time.Millisecond)
	if !cardReplayMismatch(base, diffDue) {
		t.Fatal("due beyond 1s should mismatch")
	}
	diffLast := base
	diffLast.LastReview = base.LastReview.Add(replayDueSlack + time.Millisecond)
	if !cardReplayMismatch(base, diffLast) {
		t.Fatal("LastReview beyond 1s should mismatch")
	}
	if math.Abs(base.Stability-same.Stability) >= replayStabilityEps {
		t.Fatal("fixture drift too large")
	}
}
