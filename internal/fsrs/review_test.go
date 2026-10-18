package fsrs

import (
	"context"
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
		Dir:      DirHeEn,
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
	if log.Rating != int(Good) || !log.Peeked {
		t.Fatalf("log rating/peeked = %d/%v", log.Rating, log.Peeked)
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
}

func TestSuspendExcludesFromQueue(t *testing.T) {
	rev, store, clk := openReview(t)
	ctx := context.Background()
	userID := mustUser(t, store, "learner@example.com")
	dueID := mustCard(t, store, db.Card{
		UserID:   userID,
		LexemeID: mustLexeme(t, store, "שלום"),
		Dir:      DirHeEn,
		Due:      clk.Now().Add(-time.Hour),
	})
	futureID := mustCard(t, store, db.Card{
		UserID:   userID,
		LexemeID: mustLexeme(t, store, "להתראות"),
		Dir:      DirHeEn,
		Due:      clk.Now().Add(time.Hour),
	})

	if err := rev.Suspend(ctx, dueID, true); err != nil {
		t.Fatal(err)
	}
	sess, err := rev.Queue(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.Items) != 0 {
		t.Fatalf("suspended card was queued: %+v", sess.Items)
	}
	if err := rev.Suspend(ctx, dueID, false); err != nil {
		t.Fatal(err)
	}
	sess, err = rev.Queue(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.Items) != 1 || sess.Items[0].CardID != dueID {
		t.Fatalf("queue = %+v, want card %d", sess.Items, dueID)
	}
	for _, item := range sess.Items {
		if item.CardID == futureID {
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
		{name: "20.9 days", card: Card{Dir: DirHeEn, Stability: 20.9, Ratings: good}, want: false},
		{name: "21 days", card: Card{Dir: DirHeEn, Stability: 21, Ratings: good}, want: true},
		{name: "again in the last 3", card: Card{Dir: DirHeEn, Stability: 40, Ratings: []Rating{Good, Again, Good, Easy}}, want: false},
		{name: "again older than the last 3", card: Card{Dir: DirHeEn, Stability: 21, Ratings: []Rating{Again, Good, Hard, Easy}}, want: true},
		{name: "en_he", card: Card{Dir: DirEnHe, Stability: 80, Ratings: good}, want: false},
		{name: "no reviews yet", card: Card{Dir: DirHeEn, Stability: 21}, want: true},
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
		Dir:      DirHeEn,
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
