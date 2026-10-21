package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
)

// TestNothingRatedFinishIsNotAServerError is the handler mapping for a v2
// finish that has no ratings. The service returns ErrNothingRated and writes
// nothing; the placement finish route answers 409, not 500.
func TestNothingRatedFinishIsNotAServerError(t *testing.T) {
	rec := httptest.NewRecorder()
	failPlacement(rec, placement.ErrNothingRated)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}

	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	learner := insertPlacedUser(t, store, "empty@example.com", "sub-empty", "csrf-empty", now)
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         fixedClock(now),
		Store:         store,
		AllowedEmails: []string{"empty@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewWithPlacement(nil, svc, finishErrDeck{err: placement.ErrNothingRated}, store)
	posted := learner.post(t, h, "/_hx/placement/finish", url.Values{"run_id": {"1"}, "_csrf": {learner.csrf}}, learner.csrf, true)
	if posted.Code != http.StatusConflict {
		t.Fatalf("POST finish = %d, want 409", posted.Code)
	}
}

// finishErrDeck is a placement deck whose Finish returns one error.
type finishErrDeck struct{ err error }

func (finishErrDeck) Start(context.Context, int64) (int64, error) { return 0, nil }
func (finishErrDeck) Next(context.Context, int64, int64) (placement.Step, error) {
	return placement.Step{}, nil
}
func (finishErrDeck) Answer(context.Context, int64, int64, string, placement.AnswerIn) (placement.Step, error) {
	return placement.Step{}, nil
}
func (finishErrDeck) StartTyping(context.Context, int64, int64) (placement.Step, error) {
	return placement.Step{}, nil
}
func (finishErrDeck) Replace(context.Context, int64, int64, string) (placement.Step, error) {
	return placement.Step{}, nil
}
func (d finishErrDeck) Finish(context.Context, int64, int64) (placement.RunResult, error) {
	return placement.RunResult{}, d.err
}
func (finishErrDeck) Result(context.Context, int64, int64) (placement.RunResult, error) {
	return placement.RunResult{}, nil
}
func (finishErrDeck) Latest(context.Context, int64) (placement.RunResult, bool, error) {
	return placement.RunResult{}, false, nil
}
