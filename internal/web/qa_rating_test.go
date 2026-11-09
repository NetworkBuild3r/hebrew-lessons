package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// TestQAReviewGradeRejectsInvalidRating posts to the review submit route,
// POST /_hx/review/grade. GET /review only renders the queue. The session,
// CSRF token, and Origin are valid. Each invalid rating is sent after a
// successful reveal, so the 400 is the rating check in grade, not
// session.ErrNotRevealed. Those two rejections both write
// http.StatusText(400), so the body cannot tell them apart.
func TestQAReviewGradeRejectsInvalidRating(t *testing.T) {
	const origin = "https://learn.example"
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	reviewed := now.Add(-48 * time.Hour)
	lex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	cardID := addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 4, int(fsrs.Good), reviewed)
	before := mustGetCard(t, store, cardID)
	logsBefore := len(mustLogs(t, store, cardID))
	if logsBefore != 1 {
		t.Fatalf("seed logs = %d, want 1", logsBefore)
	}
	getLearner(t, h, learner, "/review")

	// A valid rating with no reveal is the reveal-check rejection. It writes
	// nothing, and the serve stays current. The rating check uses the same
	// status text, so the body match is recorded and the revealed posts below
	// are what reach the 1–4 check.
	revealReject := postGrade(t, h, learner, cardID, "3", origin)
	if revealReject.Code != http.StatusBadRequest {
		t.Fatalf("unrevealed valid rating = %d, want the reveal check 400\n%s", revealReject.Code, revealReject.Body.String())
	}
	if n := len(mustLogs(t, store, cardID)); n != logsBefore {
		t.Fatalf("reveal check wrote a log, rows = %d", n)
	}

	for _, rating := range []string{"0", "5", "abc", ""} {
		t.Run(ratingLabel(rating), func(t *testing.T) {
			res := gradeRevealed(t, h, learner, cardID, rating, origin)
			assertRatingCheck(t, res, revealReject.Body.String())
			if n := len(mustLogs(t, store, cardID)); n != logsBefore {
				t.Fatalf("review_log rows = %d, want %d", n, logsBefore)
			}
			after := mustGetCard(t, store, cardID)
			if !sameReviewCard(before, after) {
				t.Fatalf("card changed from %+v to %+v", before, after)
			}
		})
	}

	t.Run("valid", func(t *testing.T) {
		res := gradeRevealed(t, h, learner, cardID, "3", origin)
		if res.Code != http.StatusOK {
			t.Fatalf("valid rating = %d\n%s", res.Code, res.Body.String())
		}
		if n := len(mustLogs(t, store, cardID)); n != logsBefore+1 {
			t.Fatalf("review_log rows = %d, want %d", n, logsBefore+1)
		}
		after := mustGetCard(t, store, cardID)
		if sameReviewCard(before, after) {
			t.Fatalf("valid rating left the card unchanged: %+v", after)
		}
	})
}

func ratingLabel(rating string) string {
	if rating == "" {
		return "empty"
	}
	return rating
}

// assertRatingCheck requires the 400 from grade's rating parse and 1–4 range.
// gradeRevealed has already required a 200 from reveal, so this is not
// ErrNotRevealed. When the two bodies differ, the response must not be the
// reveal-check rejection.
func assertRatingCheck(t *testing.T, res *httptest.ResponseRecorder, revealBody string) {
	t.Helper()
	if res.Code != http.StatusBadRequest {
		if res.Code >= 200 && res.Code < 300 {
			t.Fatalf("BUG: accepts invalid rating: owner Backend Dev")
		}
		t.Fatalf("status = %d, want 400 from the rating check\n%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	want := http.StatusText(http.StatusBadRequest) + "\n"
	if body != revealBody && strings.Contains(body, "not revealed") {
		t.Fatalf("400 is the reveal check (%q), not the rating check (%q)", body, revealBody)
	}
	if body != want {
		t.Fatalf("body = %q, want the rating check %q (reveal check is %q)", body, want, revealBody)
	}
}

func mustGetCard(t *testing.T, store *db.Store, id int64) db.Card {
	t.Helper()
	card, err := store.GetCard(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return card
}

func sameReviewCard(a, b db.Card) bool {
	return a.ID == b.ID && a.State == b.State && a.Due.Equal(b.Due) &&
		a.Stability == b.Stability && a.Difficulty == b.Difficulty &&
		a.Reps == b.Reps && a.Lapses == b.Lapses && a.LastReview.Equal(b.LastReview) &&
		a.Suspended == b.Suspended
}
