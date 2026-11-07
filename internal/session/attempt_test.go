package session

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// TestSelfGradeWritesAnAttempt is the review-path check for INIT-015/SPEC-005:
// SelfGrade writes one attempts row with activity flashcard and source review.
// ADR-0028 D6 left unaskable cards out of the mix, so the flashcard row is
// exercised through the recorder directly.
func TestSelfGradeWritesAnAttempt(t *testing.T) {
	for _, tc := range []struct {
		skill   string
		rating  fsrs.Rating
		correct bool
	}{
		{fsrs.SkillRead, fsrs.Good, true},
		{fsrs.SkillRecall, fsrs.Again, false},
		{fsrs.SkillRead, fsrs.Easy, true},
	} {
		t.Run(tc.skill+"/"+string(rune('0'+tc.rating)), func(t *testing.T) {
			svc, store, clk := openSvc(t, 1)
			ctx := context.Background()
			userID := mustUser(t, store)
			cardID := addCard(t, store, userID, "גלוי", tc.skill, clk.Now().Add(-time.Hour), 0, int(fsrs.Good), clk.Now().Add(-48*time.Hour), false)
			if err := svc.record.SelfGrade(ctx, cardID, tc.rating); err != nil {
				t.Fatal(err)
			}
			lexeme := lexemeOf(t, store, cardID)
			attempts, err := store.ListAttempts(ctx, userID, lexeme)
			if err != nil || len(attempts) != 1 {
				t.Fatalf("attempts = %+v, %v", attempts, err)
			}
			got := attempts[0]
			if got.Skill != tc.skill || got.Activity != "flashcard" || got.Source != "review" ||
				got.Correct != tc.correct || got.DontKnow || got.UnitID != 0 || !got.CreatedAt.Equal(clk.Now()) {
				t.Fatalf("attempt = %+v", got)
			}
			logs := mustReviewLogs(t, store, cardID)
			if len(logs) != 2 || logs[1].Rating != int(tc.rating) {
				t.Fatalf("review_log = %+v, want the seed row and one new row", logs)
			}
		})
	}
}

// A drill step has no card, so it writes no attempt.
func TestDrillStepWritesNoAttempt(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	fillPool(t, store, 10)
	lex := mustLexeme(t, store, "גלוי")
	for i := 0; i < MinMissesForDrill; i++ {
		if err := fsrs.New(store, clk).RecordMiss(ctx, userID, fsrs.MissLetterPair, "ב/כ", lex); err != nil {
			t.Fatal(err)
		}
	}
	sess := mustBuild(t, svc, userID)
	drill, ok := sess.Next()
	// A sitting with only drills (no askable study card) still serves the drill.
	if !ok || drill.CardID >= 0 {
		// Prefer a drill; if a study card is first, advance to the drill.
		for ok && drill.CardID >= 0 {
			drill, ok = sess.Next()
		}
	}
	if !ok || drill.CardID >= 0 {
		t.Fatalf("visit = %+v, %v; want a drill step", drill, ok)
	}
	revealAndRecord(t, sess, ctx, drill.CardID, fsrs.Good)
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM attempts`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("attempts = %d, %v", n, err)
	}
}
