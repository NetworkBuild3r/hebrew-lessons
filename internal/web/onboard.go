package web

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// PlacementRun is one finished placement, newest first, as Backend Dev's
// PlacementHistory will return it (migration 014, branch
// cursor/v3-onboarding-placement-history).
//
// It is not db.PlacementRun. That type is already the placement_runs table
// row (id, user, started_at, level, result, …). These two cannot share a name
// in package db without a rename on their side.
//
// StartUnitTitle is empty when no published in-scope unit has that position.
// The page then renders "Unit {n}" with no ": {name}".
type PlacementRun struct {
	ID             int64
	FinishedAt     time.Time
	StartUnit      int
	StartUnitTitle string
	WordsKnown     int
}

// Onboarding is the db API this feature calls. *db.Store does not implement
// it yet: PlacementHistory there returns []db.PlacementHistoryRow, and
// User has no OnboardedAt. Tests pass a *memOnboard. Production uses
// storeOnboard, which only adapts the history query that already exists.
//
// TODO(backend): when migration 014 lands, replace storeOnboard with the
// store. FinishPlacement sets onboarded_at in that same transaction; this
// package does not call SetOnboarded from the placement finish handler.
type Onboarding interface {
	// SetOnboarded records "Start from the basics". It is idempotent and
	// keeps the first value.
	SetOnboarded(ctx context.Context, userID int64, at time.Time) error
	// OnboardedAt is User.OnboardedAt. Nil means the column is null.
	OnboardedAt(ctx context.Context, userID int64) (*time.Time, error)
	// PlacementHistory is finished runs only, newest first. limit <= 0
	// returns every finished run. The limit is applied by the store.
	PlacementHistory(ctx context.Context, userID int64, limit int) ([]PlacementRun, error)
	// PlacementHistoryCount is the number of finished runs, for "Show all ({n})".
	PlacementHistoryCount(ctx context.Context, userID int64) (int, error)
}

// lessonAttempts is not in Backend Dev's signatures. The welcome gate needs
// it (OnboardedAt is null AND no finished run AND no lesson attempts). The
// in-memory fake records it. storeOnboard does not, so the gate stays off
// until a store method exists. See the report.
type lessonAttempts interface {
	HasLessonAttempt(ctx context.Context, userID int64) (bool, error)
}

// errOnboardUnwired is returned when a call needs a column migration 014
// has not added yet. History reads do not use it: they adapt the existing query.
var errOnboardUnwired = errors.New("onboarding: User.OnboardedAt is not on the store until migration 014")

// storeOnboard adapts the placement history *db.Store already serves.
// SetOnboarded and OnboardedAt are not wired. The welcome gate checks for
// HasLessonAttempt, which this type does not have, and stays off.
type storeOnboard struct {
	store *db.Store
}

func (a storeOnboard) SetOnboarded(context.Context, int64, time.Time) error {
	// The basics handlers call this. Until migration 014 there is nowhere
	// to write. Returning an error would 500 the form; the flag cannot be
	// stored, so the call is a no-op and the handler still redirects home.
	slog.Warn("onboarding: SetOnboarded dropped; waiting for migration 014")
	return nil
}

func (a storeOnboard) OnboardedAt(context.Context, int64) (*time.Time, error) {
	return nil, errOnboardUnwired
}

func (a storeOnboard) PlacementHistory(ctx context.Context, userID int64, limit int) ([]PlacementRun, error) {
	if a.store == nil {
		return nil, nil
	}
	// The existing query uses LIMIT ?. SQLite LIMIT 0 returns no rows.
	// Backend's limit <= 0 means all rows. Ask for a high cap until then.
	q := limit
	if q <= 0 {
		q = 10000
	}
	rows, err := a.store.PlacementHistory(ctx, userID, q)
	if err != nil {
		return nil, err
	}
	out := make([]PlacementRun, 0, len(rows))
	for _, r := range rows {
		run := PlacementRun{ID: r.RunID, FinishedAt: r.FinishedAt, StartUnit: r.Level}
		var body struct {
			EstimatedKnownWords int `json:"estimated_known_words"`
		}
		if r.Result != "" {
			_ = json.Unmarshal([]byte(r.Result), &body)
		}
		run.WordsKnown = body.EstimatedKnownWords
		// StartUnitTitle stays empty: the existing row has no unit title.
		// Level is the placement_runs.level column, which Backend will expose
		// as StartUnit. A null level comes back as -1.
		out = append(out, run)
	}
	if limit <= 0 || len(out) <= limit {
		return out, nil
	}
	return out[:limit], nil
}

func (a storeOnboard) PlacementHistoryCount(ctx context.Context, userID int64) (int, error) {
	rows, err := a.PlacementHistory(ctx, userID, 0)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rows {
		if !r.FinishedAt.IsZero() {
			n++
		}
	}
	return n, nil
}

// memOnboard is the in-memory fake. Finishing a run sets onboarded_at and
// appends a history row, the way Backend's finish transaction will. An
// abandoned run is recorded and counted by neither. SetOnboarded keeps the
// first timestamp.
type memOnboard struct {
	mu        sync.Mutex
	onboarded map[int64]time.Time
	runs      map[int64][]PlacementRun
	abandoned map[int64]int
	lessons   map[int64]bool
	setCalls  map[int64]int
}

func newMemOnboard() *memOnboard {
	return &memOnboard{
		onboarded: map[int64]time.Time{},
		runs:      map[int64][]PlacementRun{},
		abandoned: map[int64]int{},
		lessons:   map[int64]bool{},
		setCalls:  map[int64]int{},
	}
}

func (m *memOnboard) SetOnboarded(_ context.Context, userID int64, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCalls[userID]++
	if _, ok := m.onboarded[userID]; ok {
		return nil
	}
	m.onboarded[userID] = at
	return nil
}

func (m *memOnboard) OnboardedAt(_ context.Context, userID int64) (*time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	at, ok := m.onboarded[userID]
	if !ok {
		return nil, nil
	}
	return &at, nil
}

func (m *memOnboard) PlacementHistory(_ context.Context, userID int64, limit int) ([]PlacementRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rows := append([]PlacementRun(nil), m.runs[userID]...)
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].FinishedAt.Equal(rows[j].FinishedAt) {
			return rows[i].FinishedAt.After(rows[j].FinishedAt)
		}
		return rows[i].ID > rows[j].ID
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (m *memOnboard) PlacementHistoryCount(_ context.Context, userID int64) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.runs[userID]), nil
}

func (m *memOnboard) HasLessonAttempt(_ context.Context, userID int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lessons[userID], nil
}

// SetCalls is how many times SetOnboarded was called, including no-op repeats.
func (m *memOnboard) SetCalls(userID int64) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.setCalls[userID]
}

// MarkLesson records a lesson-source attempt. Abandoned and finished runs
// do not. The gate treats this the way attempts.source = 'lesson' will.
func (m *memOnboard) MarkLesson(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lessons[userID] = true
}

// Abandon records an unfinished run. It does not set onboarded_at and it
// does not appear in history, so the learner still sees /welcome.
func (m *memOnboard) Abandon(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.abandoned[userID]++
}

// Abandoned is how many open runs were recorded. History does not include them.
func (m *memOnboard) Abandoned(userID int64) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.abandoned[userID]
}

// RecordFinish is what Backend's finish transaction does: onboarded_at is
// set if it was null, and the run is appended to history. Calling it twice
// keeps the first onboarded_at and stores both runs.
func (m *memOnboard) RecordFinish(userID int64, run PlacementRun) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.onboarded[userID]; !ok {
		m.onboarded[userID] = run.FinishedAt
	}
	m.runs[userID] = append(m.runs[userID], run)
}
