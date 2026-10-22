package web

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// PlacementRun is one finished placement, newest first.
//
// It is not db.PlacementRun. That type is the placement_runs table row
// (id, user, started_at, level, result, …). These two cannot share a name
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

// Onboarding is the db API this feature calls. The welcome gate calls
// IsOnboarded and nothing else. It does not OR onboarded_at, finished
// runs, and lesson attempts itself.
type Onboarding interface {
	// SetOnboarded records "Start from the basics". It is idempotent and
	// keeps the first value.
	SetOnboarded(ctx context.Context, userID int64, at time.Time) error
	// IsOnboarded is true when onboarded_at is set, or a placement run is
	// finished, or the learner has a lesson attempt. The store method
	// lazily backfills onboarded_at. The gate redirects only when this is false.
	IsOnboarded(ctx context.Context, userID int64) (bool, error)
	// PlacementHistory is finished runs only, newest first. limit <= 0
	// returns every finished run. The limit is applied by the store.
	PlacementHistory(ctx context.Context, userID int64, limit int) ([]PlacementRun, error)
	// PlacementHistoryCount is the number of finished runs, for "Show all ({n})".
	PlacementHistoryCount(ctx context.Context, userID int64) (int, error)
}

// onboardStore is the *db.Store surface this adapter calls. A missing
// method fails at compile time. The gate calls IsOnboarded and does not
// combine the columns itself.
type onboardStore interface {
	SetOnboarded(ctx context.Context, userID int64, at time.Time) error
	IsOnboarded(ctx context.Context, userID int64) (bool, error)
	PlacementHistory(ctx context.Context, userID int64, limit int) ([]db.PlacementHistoryRow, error)
	PlacementHistoryCount(ctx context.Context, userID int64) (int, error)
}

var _ onboardStore = (*db.Store)(nil)

// storeOnboard is *db.Store behind Onboarding.
type storeOnboard struct {
	store onboardStore
}

func (a storeOnboard) SetOnboarded(ctx context.Context, userID int64, at time.Time) error {
	if a.store == nil {
		return errors.New("onboarding: no store")
	}
	return a.store.SetOnboarded(ctx, userID, at)
}

func (a storeOnboard) IsOnboarded(ctx context.Context, userID int64) (bool, error) {
	if a.store == nil {
		return false, errors.New("onboarding: no store")
	}
	return a.store.IsOnboarded(ctx, userID)
}

func (a storeOnboard) PlacementHistory(ctx context.Context, userID int64, limit int) ([]PlacementRun, error) {
	if a.store == nil {
		return nil, nil
	}
	rows, err := a.store.PlacementHistory(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]PlacementRun, 0, len(rows))
	for _, r := range rows {
		out = append(out, PlacementRun{
			ID:             r.ID,
			FinishedAt:     r.FinishedAt,
			StartUnit:      r.StartUnit,
			StartUnitTitle: r.StartUnitTitle,
			WordsKnown:     r.WordsKnown,
		})
	}
	return out, nil
}

func (a storeOnboard) PlacementHistoryCount(ctx context.Context, userID int64) (int, error) {
	if a.store == nil {
		return 0, nil
	}
	return a.store.PlacementHistoryCount(ctx, userID)
}

// testOnboard, when set, replaces storeOnboard for handlers whose Options.Onboard
// is nil. Production leaves it nil, so the welcome gate uses the store.
// Package tests set it so a page test is not redirected before it can render.
// A test of the gate passes storeOnboard explicitly and does not use this.
var testOnboard func(*db.Store) Onboarding

var (
	_ Onboarding = storeOnboard{}
	_ Onboarding = (*memOnboard)(nil)
)

// memOnboard is the in-memory fake. IsOnboarded is the same rule Backend
// described: onboarded_at, or a finished run, or a lesson attempt, and a
// true result backfills onboarded_at. An abandoned run sets none of those.
// SetOnboarded keeps the first timestamp.
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

func (m *memOnboard) IsOnboarded(_ context.Context, userID int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.onboarded[userID]; ok {
		return true, nil
	}
	if len(m.runs[userID]) == 0 && !m.lessons[userID] {
		return false, nil
	}
	// Lazy backfill. The first timestamp stays, matching COALESCE.
	m.onboarded[userID] = time.Now().UTC()
	return true, nil
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
// do not. The gate treats this the way attempts.source <> 'placement' will.
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

// recordRun appends a finished run and does not set onboarded_at.
// IsOnboarded still counts the run and backfills the timestamp.
func (m *memOnboard) recordRun(userID int64, run PlacementRun) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs[userID] = append(m.runs[userID], run)
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
