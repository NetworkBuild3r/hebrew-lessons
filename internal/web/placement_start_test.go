package web

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// A finished v2 run with nothing rated (the production shape: cardsRated 0, level -1)
// does not block a v3 start. POST /_hx/placement/start deals the first letter when
// every letter clip is stored.
func TestPlacementStartAfterFinishedV2(t *testing.T) {
	h, store, learner, _ := newPlacementRouter(t)
	ctx := context.Background()
	old := seedFinishedV2(t, store, learner.id)
	before, err := store.GetPlacementRun(ctx, old)
	if err != nil {
		t.Fatal(err)
	}

	rec := learner.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {learner.csrf}}, learner.csrf, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("start = %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if screenOf(body) != views.PlaceLetters || phaseOf(body) != views.PlaceAsk {
		t.Fatalf("screen %q phase %q, want the first letter", screenOf(body), phaseOf(body))
	}
	runID, err := strconv.ParseInt(hidden(t, body, "run_id"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if runID == old {
		t.Fatal("start resumed the finished v2 run")
	}
	if placementRunCount(t, store, learner.id) != 2 {
		t.Fatalf("runs = %d, want the finished v2 row and the new v3 run", placementRunCount(t, store, learner.id))
	}
	after, err := store.GetPlacementRun(ctx, old)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("finished v2 changed\nbefore %+v\nafter  %+v", before, after)
	}
	meta, ok, err := store.GetPlacementV3Run(ctx, runID)
	if err != nil || !ok || meta.Kind != db.RunInitial || meta.Phase != db.PhaseParts {
		t.Fatalf("v3 meta %+v ok %v err %v", meta, ok, err)
	}
	open, err := store.GetPlacementRun(ctx, runID)
	if err != nil || !open.FinishedAt.IsZero() {
		t.Fatalf("v3 run %+v err %v", open, err)
	}
	items, err := store.ListPlacementItems(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Seq != 1 || items[0].Part != string(placement.PartLetters) || items[0].Outcome != "" {
		t.Fatalf("first item %+v", items)
	}
}

// One missing letter clip is the 503 from POST /_hx/placement/start. Start writes
// no run. The log names the letter id and not the clip text.
func TestPlacementStartMissingLetterClip(t *testing.T) {
	h, store, learner, _ := newPlacementRouter(t)
	ctx := context.Background()
	old := seedFinishedV2(t, store, learner.id)
	before, err := store.GetPlacementRun(ctx, old)
	if err != nil {
		t.Fatal(err)
	}
	letter := dropLetterClip(t, store, "letter-da")

	var logs bytes.Buffer
	restore := swapLog(t, &logs)
	defer restore()
	rec := learner.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {learner.csrf}}, learner.csrf, true)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("start = %d %s, want 503", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != http.StatusText(http.StatusServiceUnavailable)+"\n" {
		t.Fatalf("body %q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), letter.ID) || strings.Contains(rec.Body.String(), letter.SyllableText) {
		t.Fatalf("response names the letter or the clip: %q", rec.Body.String())
	}
	if placementRunCount(t, store, learner.id) != 1 {
		t.Fatalf("runs = %d, want only the finished v2 row", placementRunCount(t, store, learner.id))
	}
	if n := v3RunCount(t, store); n != 0 {
		t.Fatalf("%d v3 runs were written", n)
	}
	after, err := store.GetPlacementRun(ctx, old)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("finished v2 changed\nbefore %+v\nafter  %+v", before, after)
	}
	got := logs.String()
	if strings.Count(got, `msg="placement start"`) != 1 {
		t.Fatalf("log lines = %q", got)
	}
	if !strings.Contains(got, "letter "+letter.ID) {
		t.Fatalf("log %q does not name %s", got, letter.ID)
	}
	if strings.Contains(got, letter.SyllableText) {
		t.Fatalf("log names the clip text: %q", got)
	}
}

// An open v2 run would block the unique open-run index. Start closes it with
// the unscored finish (level -1, no ratings) and opens a v3 run.
func TestPlacementStartClosesOpenV2(t *testing.T) {
	h, store, learner, _ := newPlacementRouter(t)
	ctx := context.Background()
	when := store.Clock().Now()
	old, err := store.InsertPlacementRun(ctx, learner.id, when.Add(-time.Hour), 6)
	if err != nil {
		t.Fatal(err)
	}

	rec := learner.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {learner.csrf}}, learner.csrf, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("start = %d %s", rec.Code, rec.Body.String())
	}
	runID, err := strconv.ParseInt(hidden(t, rec.Body.String(), "run_id"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if runID == old {
		t.Fatal("start kept the v2 run open")
	}
	closed, err := store.GetPlacementRun(ctx, old)
	if err != nil {
		t.Fatal(err)
	}
	if closed.FinishedAt.IsZero() || !closed.Level.Valid || closed.Level.Int64 != -1 || closed.StopReason != "" {
		t.Fatalf("closed v2 = %+v", closed)
	}
	if closed.Result != `{"bands":[],"estimated_known_words":0}` {
		t.Fatalf("closed v2 result %s", closed.Result)
	}
	answers, err := store.ListPlacementAnswers(ctx, old)
	if err != nil {
		t.Fatal(err)
	}
	if len(answers) != 0 {
		t.Fatalf("closed v2 has ratings: %+v", answers)
	}
	if _, ok, err := store.GetPlacementV3Run(ctx, old); err != nil || ok {
		t.Fatalf("closed v2 gained a v3 row ok=%v err=%v", ok, err)
	}
	meta, ok, err := store.GetPlacementV3Run(ctx, runID)
	if err != nil || !ok || meta.Kind != db.RunInitial {
		t.Fatalf("v3 meta %+v ok %v err %v", meta, ok, err)
	}
	if screenOf(rec.Body.String()) != views.PlaceLetters {
		t.Fatalf("screen %q", screenOf(rec.Body.String()))
	}
}

// ErrNoAudio and ErrDeckShort from Start, and ErrDeckShort from the Next that
// Start deals, each log one warning. A conflict does not.
func TestPlacementStartLogsDeckErrors(t *testing.T) {
	letters, err := placement.Letters()
	if err != nil {
		t.Fatal(err)
	}
	letter := letters[0]
	noAudio := fmt.Errorf("%w: letter %s", placement.ErrNoAudio, letter.ID)
	cases := []struct {
		name     string
		startErr error
		nextErr  error
		status   int
		want     string
	}{
		{name: "no audio", startErr: noAudio, status: http.StatusServiceUnavailable, want: "letter " + letter.ID},
		{name: "deck short on start", startErr: placement.ErrDeckShort, status: http.StatusServiceUnavailable, want: placement.ErrDeckShort.Error()},
		{name: "deck short on next", nextErr: placement.ErrDeckShort, status: http.StatusServiceUnavailable, want: placement.ErrDeckShort.Error()},
		{name: "not your run", startErr: placement.ErrNotYourRun, status: http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, learner := newStubPlacementRouter(t, startFailDeck{startErr: tc.startErr, nextErr: tc.nextErr, runID: 7})
			var logs bytes.Buffer
			restore := swapLog(t, &logs)
			defer restore()
			rec := learner.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {learner.csrf}}, learner.csrf, true)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d, body %s", rec.Code, tc.status, rec.Body.String())
			}
			got := logs.String()
			if tc.want == "" {
				if strings.Contains(got, "placement start") {
					t.Fatalf("logged %q", got)
				}
				return
			}
			if strings.Count(got, `msg="placement start"`) != 1 || !strings.Contains(got, tc.want) {
				t.Fatalf("log %q", got)
			}
			if strings.Contains(got, letter.SyllableText) {
				t.Fatalf("log names the clip text: %q", got)
			}
		})
	}
}

// seedFinishedV2 is the production row: one finished v2 run, no ratings, level -1.
func seedFinishedV2(t *testing.T, store *db.Store, userID int64) int64 {
	t.Helper()
	ctx := context.Background()
	when := store.Clock().Now()
	id, err := store.InsertPlacementRun(ctx, userID, when.Add(-2*time.Hour), 4)
	if err != nil {
		t.Fatal(err)
	}
	const result = `{"bands":[],"estimated_known_words":0}`
	if err := store.FinishPlacement(ctx, id, when.Add(-time.Hour), -1, "", result); err != nil {
		t.Fatal(err)
	}
	place, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok || !place.Finished || place.CardsRated != 0 || place.Level != -1 || place.EstimatedKnownWords != 0 {
		t.Fatalf("seed %+v ok %v err %v", place, ok, err)
	}
	if _, isV3, err := store.GetPlacementV3Run(ctx, id); err != nil || isV3 {
		t.Fatalf("seed is v3 ok %v err %v", isV3, err)
	}
	return id
}

func dropLetterClip(t *testing.T, store *db.Store, id string) placement.Letter {
	t.Helper()
	letters, err := placement.Letters()
	if err != nil {
		t.Fatal(err)
	}
	var letter placement.Letter
	for _, l := range letters {
		if l.ID == id {
			letter = l
			break
		}
	}
	if letter.ID == "" {
		t.Fatalf("no letter %s", id)
	}
	res, err := store.ExecContext(context.Background(), `DELETE FROM audio_assets WHERE text_nfc = ?`, audio.NormalizeText(letter.SyllableText))
	if err != nil {
		t.Fatal(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("letter clip was not seeded")
	}
	return letter
}

func placementRunCount(t *testing.T, store *db.Store, userID int64) int {
	t.Helper()
	return queryCount(t, store, `SELECT COUNT(*) FROM placement_runs WHERE user_id = ?`, userID)
}

func v3RunCount(t *testing.T, store *db.Store) int {
	t.Helper()
	return queryCount(t, store, `SELECT COUNT(*) FROM placement_v3_runs`)
}

func queryCount(t *testing.T, store *db.Store, query string, args ...any) int {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func swapLog(t *testing.T, buf *bytes.Buffer) func() {
	t.Helper()
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))
	return func() { slog.SetDefault(prev) }
}

type startFailDeck struct {
	startErr error
	nextErr  error
	runID    int64
}

func (d startFailDeck) Start(context.Context, int64) (int64, error) {
	if d.startErr != nil {
		return 0, d.startErr
	}
	return d.runID, nil
}

func (d startFailDeck) Next(context.Context, int64, int64) (placement.Step, error) {
	return placement.Step{}, d.nextErr
}

func (d startFailDeck) Answer(context.Context, int64, int64, string, placement.AnswerIn) (placement.Step, error) {
	return placement.Step{}, nil
}

func (d startFailDeck) StartTyping(context.Context, int64, int64) (placement.Step, error) {
	return placement.Step{}, nil
}

func (d startFailDeck) Replace(context.Context, int64, int64, string) (placement.Step, error) {
	return placement.Step{}, nil
}

func (d startFailDeck) Finish(context.Context, int64, int64) (placement.RunResult, error) {
	return placement.RunResult{}, nil
}

func (d startFailDeck) Result(context.Context, int64, int64) (placement.RunResult, error) {
	return placement.RunResult{}, nil
}

func (d startFailDeck) Latest(context.Context, int64) (placement.RunResult, bool, error) {
	return placement.RunResult{}, false, nil
}

func newStubPlacementRouter(t *testing.T, deck placementDeck) (http.Handler, placedUser) {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	learner := insertPlacedUser(t, store, "a@example.com", "sub-a", "csrf-a", now)
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		CookieSecure:  false,
		Clock:         fixedClock(now),
		Store:         store,
		AllowedEmails: []string{"a@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewWithPlacement(nil, svc, deck, store), learner
}
