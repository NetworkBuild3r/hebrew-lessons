package fsrs

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"

	gofsrs "github.com/open-spaced-repetition/go-fsrs/v3"
)

// fsrsRef is one grade in a DB-backed sequence, and the card go-fsrs v3.3.1
// produces for it with fuzz off. The floats are the library's values.
// Comparisons allow 1e-6. late is how long after this step's due the next
// grade waits. Zero means the next grade happens when the card is due.
type fsrsRef struct {
	rating     Rating
	state      string
	stability  float64
	difficulty float64
	due        time.Time
	reps       int
	lapses     int
	late       time.Duration
}

func TestDBBackedFSRSReference(t *testing.T) {
	start := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	t.Run("good good again good", func(t *testing.T) {
		// Reviewed when due. The third grade is Again on a review card, so
		// the card lapses into relearning. The fourth grade is Good.
		runFSRSSteps(t, start, []fsrsRef{
			{
				rating: Good, state: StateLearning,
				stability: 3.173, difficulty: 5.282434422319005,
				due:  time.Date(2026, 3, 1, 9, 10, 0, 0, time.UTC),
				reps: 1, lapses: 0,
			},
			{
				rating: Good, state: StateReview,
				stability: 4.466858064362218, difficulty: 5.272967931287446,
				due:  time.Date(2026, 3, 5, 9, 10, 0, 0, time.UTC),
				reps: 2, lapses: 0,
			},
			{
				rating: Again, state: StateRelearning,
				stability: 1.2976302777410647, difficulty: 6.790567694566929,
				due:  time.Date(2026, 3, 5, 9, 15, 0, 0, time.UTC),
				reps: 3, lapses: 1,
			},
			{
				rating: Good, state: StateReview,
				stability: 1.8267665523757515, difficulty: 6.774163790483029,
				due:  time.Date(2026, 3, 7, 9, 15, 0, 0, time.UTC),
				reps: 4, lapses: 1,
			},
		})
	})
	t.Run("400 days late", func(t *testing.T) {
		// Two on-time Goods put the card in review. The third Good is 400
		// days after that due time.
		runFSRSSteps(t, start, []fsrsRef{
			{
				rating: Good, state: StateLearning,
				stability: 3.173, difficulty: 5.282434422319005,
				due:  time.Date(2026, 3, 1, 9, 10, 0, 0, time.UTC),
				reps: 1, lapses: 0,
			},
			{
				rating: Good, state: StateReview,
				stability: 4.466858064362218, difficulty: 5.272967931287446,
				due:  time.Date(2026, 3, 5, 9, 10, 0, 0, time.UTC),
				reps: 2, lapses: 0,
				late: 400 * 24 * time.Hour,
			},
			{
				rating: Good, state: StateReview,
				stability: 128.1907161692527, difficulty: 5.263544986114632,
				due:  time.Date(2027, 8, 15, 9, 10, 0, 0, time.UTC),
				reps: 3, lapses: 0,
			},
		})
	})
}

func runFSRSSteps(t *testing.T, start time.Time, steps []fsrsRef) {
	t.Helper()
	rev, store, clk := openReview(t)
	if !clk.Now().Equal(start) {
		t.Fatalf("clock = %s, want %s", clk.Now(), start)
	}
	ctx := context.Background()
	cardID := mustCard(t, store, db.Card{
		UserID:   mustUser(t, store, "learner@example.com"),
		LexemeID: mustLexeme(t, store, "שָׁלוֹם"),
		Skill:    SkillRead,
		State:    StateNew,
		Due:      start,
	})

	param := gofsrs.DefaultParam()
	param.EnableFuzz = false
	sched := gofsrs.NewFSRS(param)
	mem := gofsrs.NewCard()

	for i, step := range steps {
		now := clk.Now()
		info := sched.Next(mem, now, gofsrs.Rating(step.rating))
		mem = info.Card
		if err := rev.Grade(ctx, cardID, step.rating, false); err != nil {
			t.Fatalf("step %d: %v", i+1, err)
		}
		got, err := rev.Card(ctx, cardID)
		if err != nil {
			t.Fatalf("step %d: %v", i+1, err)
		}
		assertFSRSStep(t, i+1, got, mem, step)
		if i+1 < len(steps) {
			clk.t = mem.Due.Add(step.late)
		}
	}
}

func assertFSRSStep(t *testing.T, n int, got Card, live gofsrs.Card, want fsrsRef) {
	t.Helper()
	liveState, err := stateName(live.State)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != want.state || liveState != want.state {
		t.Fatalf("step %d state: db=%s go-fsrs=%s want=%s", n, got.State, liveState, want.state)
	}
	if math.Abs(got.Stability-want.stability) > 1e-6 || math.Abs(live.Stability-want.stability) > 1e-6 {
		t.Fatalf("step %d stability: db=%v go-fsrs=%v want=%v", n, got.Stability, live.Stability, want.stability)
	}
	if math.Abs(got.Difficulty-want.difficulty) > 1e-6 || math.Abs(live.Difficulty-want.difficulty) > 1e-6 {
		t.Fatalf("step %d difficulty: db=%v go-fsrs=%v want=%v", n, got.Difficulty, live.Difficulty, want.difficulty)
	}
	if !got.Due.Equal(want.due) || !live.Due.Equal(want.due) {
		t.Fatalf("step %d due: db=%s go-fsrs=%s want=%s", n, got.Due.UTC(), live.Due.UTC(), want.due.UTC())
	}
	if got.Reps != want.reps || int(live.Reps) != want.reps || got.Lapses != want.lapses || int(live.Lapses) != want.lapses {
		t.Fatalf("step %d reps/lapses: db=%d/%d go-fsrs=%d/%d want=%d/%d",
			n, got.Reps, got.Lapses, live.Reps, live.Lapses, want.reps, want.lapses)
	}
	if got.Stability != live.Stability || got.Difficulty != live.Difficulty || !got.Due.Equal(live.Due) || !got.LastReview.Equal(live.LastReview) {
		t.Fatalf("step %d db drifted from go-fsrs: db=%+v fsrs due=%s stab=%v diff=%v last=%s",
			n, got, live.Due, live.Stability, live.Difficulty, live.LastReview)
	}
}
