package evidence

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// laterDayLookback is how far back LaterDayResults considers practice.
// INIT-029/SPEC-007; ADR-0025 D8.
const laterDayLookback = 60 * 24 * time.Hour

// laterDayEnough is the minimum later-day answers before Enough is true.
// INIT-029/SPEC-007; ADR-0025 D8.
const laterDayEnough = 20

// PracticeResult is later-day review outcomes for one practice activity kind.
// Kind is the activity string already stored on attempts (for example cloze,
// order, read_pick). Enough is false when Total is below 20; the count is
// still returned. INIT-029/SPEC-007; ADR-0025 D8.
type PracticeResult struct {
	Kind   string
	Right  int
	Total  int
	Enough bool
}

// attemptSlice is one attempts row used by LaterDayResults.
type attemptSlice struct {
	ID       int64
	LexemeID int64
	Activity string
	Source   string
	Correct  bool
	At       time.Time
	Day      string // civil study day in the learner's zone
}

// LaterDayResults reports, for one learner, how often words practiced each
// activity way were answered right on their first checked review on a later
// study day in zone. Practice and review answers come from attempts; Kind is
// the stored activity string. A word practiced several ways on one day counts
// once for each way. Same-day review does not count. Lookback is 60 days from
// now. Below 20 answers Enough is false. INIT-029/SPEC-007; ADR-0025 D8.
func LaterDayResults(ctx context.Context, store *db.Store, learnerID int64, now time.Time, zone studyday.Zone) ([]PracticeResult, error) {
	if store == nil {
		return nil, errors.New("evidence: store is not configured")
	}
	if learnerID <= 0 {
		return nil, errors.New("evidence: later-day results need a learner")
	}
	if zone.Loc == nil {
		return nil, errors.New("evidence: later-day results need a study zone")
	}

	since := now.Add(-laterDayLookback)
	rows, err := loadAttemptsForLaterDay(ctx, store, learnerID, since, now)
	if err != nil {
		return nil, err
	}

	attempts := make([]attemptSlice, 0, len(rows))
	for _, r := range rows {
		attempts = append(attempts, attemptSlice{
			ID:       r.ID,
			LexemeID: r.LexemeID,
			Activity: r.Activity,
			Source:   r.Source,
			Correct:  r.Correct,
			At:       r.At,
			Day:      studyday.Today(r.At, zone),
		})
	}

	// Practice events: one per (activity, word, study day) in the lookback.
	type practiceKey struct {
		kind   string
		lexeme int64
		day    string
	}
	practiced := make(map[practiceKey]struct{})
	for _, a := range attempts {
		if a.Activity == "" {
			continue
		}
		practiced[practiceKey{kind: a.Activity, lexeme: a.LexemeID, day: a.Day}] = struct{}{}
	}

	// Checked review answers oldest first per word. Flashcard is self-graded,
	// not a checked review (ADR-0024).
	reviewsByLexeme := make(map[int64][]attemptSlice)
	for _, a := range attempts {
		if a.Source != SourceReview || a.Activity == ActivityFlashcard {
			continue
		}
		reviewsByLexeme[a.LexemeID] = append(reviewsByLexeme[a.LexemeID], a)
	}

	type tally struct{ right, total int }
	byKind := make(map[string]*tally)
	for key := range practiced {
		rev, ok := firstLaterDayReview(reviewsByLexeme[key.lexeme], key.day)
		if !ok {
			continue
		}
		t := byKind[key.kind]
		if t == nil {
			t = &tally{}
			byKind[key.kind] = t
		}
		t.total++
		if rev.Correct {
			t.right++
		}
	}

	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	out := make([]PracticeResult, 0, len(kinds))
	for _, k := range kinds {
		t := byKind[k]
		out = append(out, PracticeResult{
			Kind:   k,
			Right:  t.right,
			Total:  t.total,
			Enough: t.total >= laterDayEnough,
		})
	}
	return out, nil
}

// firstLaterDayReview is the earliest checked review whose study day is after
// practiceDay. INIT-029/SPEC-007.
func firstLaterDayReview(reviews []attemptSlice, practiceDay string) (attemptSlice, bool) {
	for _, r := range reviews {
		if r.Day > practiceDay {
			return r, true
		}
	}
	return attemptSlice{}, false
}

type laterDayRow struct {
	ID       int64
	LexemeID int64
	Activity string
	Source   string
	Correct  bool
	At       time.Time
}

// loadAttemptsForLaterDay reads this learner's attempts from since through now.
// uses attempts_user_created (user_id, created_at). INIT-029/SPEC-007.
func loadAttemptsForLaterDay(ctx context.Context, store *db.Store, learnerID int64, since, now time.Time) ([]laterDayRow, error) {
	conn, err := store.DB()
	if err != nil {
		return nil, err
	}
	// Pad the text bounds one day either side: older rows may lack fixed-width
	// fractional seconds, so lexical compare can disagree with time order
	// (same pattern as db.AttemptTimes).
	rows, err := conn.QueryContext(ctx, `
		SELECT id, lexeme_id, activity, source, correct, created_at
		FROM attempts
		WHERE user_id = ? AND created_at >= ? AND created_at <= ?
		ORDER BY created_at ASC, id ASC`,
		learnerID,
		db.FormatTime(since.Add(-24*time.Hour)),
		db.FormatTime(now.Add(24*time.Hour)),
	)
	if err != nil {
		return nil, fmt.Errorf("evidence: later-day attempts: %w", err)
	}
	defer rows.Close()

	var out []laterDayRow
	for rows.Next() {
		var (
			r       laterDayRow
			correct int
			at      string
		)
		if err := rows.Scan(&r.ID, &r.LexemeID, &r.Activity, &r.Source, &correct, &at); err != nil {
			return nil, fmt.Errorf("evidence: later-day attempts: %w", err)
		}
		r.Correct = correct == 1
		r.At, err = db.ParseTime(at)
		if err != nil {
			return nil, fmt.Errorf("evidence: later-day attempts: %w", err)
		}
		if r.At.Before(since) || r.At.After(now) {
			continue
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("evidence: later-day attempts: %w", err)
	}
	return out, nil
}
