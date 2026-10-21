package web

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
)

// TestReviewSittingSeed draws a seed when a sitting starts, keeps it across
// GET and POST in that sitting, and draws a different one for the next sitting.
// The queue built from the stored seed is the queue the handler serves.
func TestReviewSittingSeed(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	store, err := db.Open(context.Background(), t.TempDir()+"/hebrew.sqlite", clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	s := newServer(store)
	closed := func(context.Context, int64) (activity.Gate, error) {
		return activity.Gate{LettersPassed: false}, nil
	}
	s.study = session.New(store, clk, 1, session.WithGate(closed))
	h := reviewHandler(t, s, clk)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	reviewed := now.Add(-48 * time.Hour)
	due := now.Add(-time.Hour)
	for i := 0; i < 8; i++ {
		lex := addLexeme(t, store, "seed-"+string(rune('a'+i)), "g", "t", "m", "")
		addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, due, 1, int(fsrs.Good), reviewed)
	}

	first := getLearner(t, h, learner, "/review")
	cardID := attr(t, first, "data-card")
	shown := postReveal(t, h, learner, mustInt(t, cardID), "https://learn.example")
	if shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d", shown.Code)
	}
	again := getLearner(t, h, learner, "/review")
	if attr(t, again, "data-card") != cardID {
		t.Fatalf("POST then GET changed card %s to %s", cardID, attr(t, again, "data-card"))
	}

	s.mu.Lock()
	q := onlyQueue(t, s)
	seed := q.seed
	ptr := q
	s.mu.Unlock()
	if got := firstCardID(t, store, clk, seed, closed, learner.user.ID); got != cardID {
		t.Fatalf("served %s, seed %d builds %s", cardID, seed, got)
	}

	secondGet := getLearner(t, h, learner, "/review")
	if attr(t, secondGet, "data-card") != cardID {
		t.Fatal("a second GET rebuilt the sitting")
	}
	s.mu.Lock()
	if onlyQueue(t, s) != ptr || ptr.seed != seed {
		s.mu.Unlock()
		t.Fatal("the sitting seed changed across requests")
	}
	ptr.done = true
	s.mu.Unlock()

	next := getLearner(t, h, learner, "/review")
	s.mu.Lock()
	q2 := onlyQueue(t, s)
	s.mu.Unlock()
	if q2 == ptr || q2.seed == seed {
		t.Fatalf("new sitting reused seed %d", seed)
	}
	if got := firstCardID(t, store, clk, q2.seed, closed, learner.user.ID); got != attr(t, next, "data-card") {
		t.Fatalf("second sitting served %s, seed %d builds %s", attr(t, next, "data-card"), q2.seed, got)
	}
}

// TestInjectedReviewSeedIsStable is the test path: an injected service seed
// is the sitting seed, including after the queue is rebuilt.
func TestInjectedReviewSeedIsStable(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openAt(t, fixedClock(now))
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	reviewed := now.Add(-48 * time.Hour)
	due := now.Add(-time.Hour)
	for i := 0; i < 8; i++ {
		lex := addLexeme(t, store, "fix-"+string(rune('a'+i)), "g", "t", "m", "")
		addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, due, 1, int(fsrs.Good), reviewed)
	}
	closed := func(context.Context, int64) (activity.Gate, error) {
		return activity.Gate{LettersPassed: false}, nil
	}
	want := cardIDs(t, store, fixedClock(now), 1, closed, learner.user.ID)
	got := walkReview(t, h, learner)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("queue = %v, want the injected seed's order %v", got, want)
	}
}

func reviewHandler(t *testing.T, s *server, clk fixedClock) http.Handler {
	t.Helper()
	authSvc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		CookieSecure:  false,
		Clock:         clk,
		Store:         s.store,
		AllowedEmails: []string{"a@example.com", "b@example.com", "en@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	mux := s.routes(nil, CSRFFunc(auth.CSRFToken), true, false)
	authSvc.Register(mux)
	return securityHeaders(authSvc.Middleware(mux))
}

func onlyQueue(t *testing.T, s *server) *studyQueue {
	t.Helper()
	if len(s.queues) != 1 {
		t.Fatalf("queues = %d, want 1", len(s.queues))
	}
	for _, q := range s.queues {
		return q
	}
	return nil
}

func firstCardID(t *testing.T, store *db.Store, clk fixedClock, seed int64, gate func(context.Context, int64) (activity.Gate, error), userID int64) string {
	t.Helper()
	ids := cardIDs(t, store, clk, seed, gate, userID)
	if len(ids) == 0 {
		t.Fatal("seed built an empty queue")
	}
	return ids[0]
}

func cardIDs(t *testing.T, store *db.Store, clk fixedClock, seed int64, gate func(context.Context, int64) (activity.Gate, error), userID int64) []string {
	t.Helper()
	sess, err := session.New(store, clk, seed, session.WithGate(gate)).BuildSession(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for {
		card, ok := sess.Next()
		if !ok {
			return ids
		}
		ids = append(ids, itoa(card.CardID))
		if len(ids) > session.SessionCap+session.MaxNewPerSession {
			t.Fatal("queue did not end")
		}
	}
}

func walkReview(t *testing.T, h http.Handler, learner learnerFix) []string {
	t.Helper()
	body := getLearner(t, h, learner, "/review")
	var ids []string
	for len(ids) <= session.SessionCap {
		if strings.Contains(body, `data-done="true"`) {
			return ids
		}
		id := mustInt(t, attr(t, body, "data-card"))
		ids = append(ids, itoa(id))
		rec := gradeRevealed(t, h, learner, id, "3", "https://learn.example")
		if rec.Code != http.StatusOK {
			t.Fatalf("grade = %d\n%s", rec.Code, rec.Body.String())
		}
		body = rec.Body.String()
	}
	t.Fatal("review did not end")
	return nil
}
