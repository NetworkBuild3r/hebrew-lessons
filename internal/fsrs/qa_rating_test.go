package fsrs

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// TestQAGradeRejectsOutOfRange checks Review.Grade, the grade entry point.
// Rating is an int, so a non-integer cannot be passed to it.
func TestQAGradeRejectsOutOfRange(t *testing.T) {
	for _, rating := range []Rating{0, 5, -1} {
		t.Run(ratingName(rating), func(t *testing.T) {
			rev, store, clk := openReview(t)
			ctx := context.Background()
			cardID := mustCard(t, store, db.Card{
				UserID:     mustUser(t, store, "learner@example.com"),
				LexemeID:   mustLexeme(t, store, "שָׁלוֹם"),
				Dir:        DirHeEn,
				State:      StateReview,
				Due:        clk.Now(),
				Stability:  4,
				Difficulty: 5,
				Reps:       2,
				Lapses:     0,
				LastReview: clk.Now().Add(-48 * time.Hour),
			})
			before, err := store.GetCard(ctx, cardID)
			if err != nil {
				t.Fatal(err)
			}
			logsBefore, err := store.ReviewLogs(ctx, cardID)
			if err != nil {
				t.Fatal(err)
			}
			if len(logsBefore) != 0 {
				t.Fatalf("seed logs = %d, want 0", len(logsBefore))
			}
			err = rev.Grade(ctx, cardID, rating, false)
			if err == nil {
				t.Fatalf("BUG: accepts invalid rating: owner Backend Dev")
			}
			// The review_log CHECK (rating BETWEEN 1 AND 4) also rejects a
			// write. Require the Go range error so that constraint is not
			// what makes this test pass.
			want := fmt.Sprintf("fsrs: rating %d is outside 1..4", rating)
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("Grade(%d) = %v, want the 1..4 rating check", rating, err)
			}
			after, err := store.GetCard(ctx, cardID)
			if err != nil {
				t.Fatal(err)
			}
			if !sameCardSchedule(before, after) {
				t.Fatalf("card changed from %+v to %+v", before, after)
			}
			logs, err := store.ReviewLogs(ctx, cardID)
			if err != nil {
				t.Fatal(err)
			}
			if len(logs) != 0 {
				t.Fatalf("review_log rows = %d, want 0", len(logs))
			}
		})
	}
}

func ratingName(rating Rating) string {
	switch rating {
	case 0:
		return "0"
	case 5:
		return "5"
	case -1:
		return "-1"
	default:
		return "other"
	}
}

func sameCardSchedule(a, b db.Card) bool {
	return a.ID == b.ID && a.State == b.State && a.Due.Equal(b.Due) &&
		a.Stability == b.Stability && a.Difficulty == b.Difficulty &&
		a.Reps == b.Reps && a.Lapses == b.Lapses && a.LastReview.Equal(b.LastReview) &&
		a.Suspended == b.Suspended
}
