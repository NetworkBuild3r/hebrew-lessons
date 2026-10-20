package session

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// TestRecordWritesAnAttempt is the review-path check for INIT-015/SPEC-005:
// a self-graded card writes one attempts row with the card's skill, activity
// flashcard and source review, and the review_log row beside it.
func TestRecordWritesAnAttempt(t *testing.T) {
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
			sess := mustBuild(t, svc, userID)
			card, ok := sess.Next()
			if !ok || card.CardID != cardID {
				t.Fatalf("served %+v, %v", card, ok)
			}
			revealAndRecord(t, sess, ctx, cardID, tc.rating)

			attempts, err := store.ListAttempts(ctx, userID, card.LexemeID)
			if err != nil || len(attempts) != 1 {
				t.Fatalf("attempts = %+v, %v", attempts, err)
			}
			got := attempts[0]
			if got.Skill != tc.skill || got.Activity != "flashcard" || got.Source != "review" ||
				got.Correct != tc.correct || got.DontKnow || got.UnitID != 0 || !got.CreatedAt.Equal(clk.Now()) {
				t.Fatalf("attempt = %+v", got)
			}
			logs := mustReviewLogs(t, store, cardID)
			if len(logs) != 2 || logs[1].Rating != int(tc.rating) || !logs[1].Peeked {
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
	lex := mustLexeme(t, store, "גלוי")
	for i := 0; i < MinMissesForDrill; i++ {
		if err := fsrs.New(store, clk).RecordMiss(ctx, userID, fsrs.MissLetterPair, "ב/כ", lex); err != nil {
			t.Fatal(err)
		}
	}
	sess := mustBuild(t, svc, userID)
	drill, ok := sess.Next()
	if !ok || drill.CardID >= 0 {
		t.Fatalf("first visit = %+v, %v; want a drill step", drill, ok)
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
