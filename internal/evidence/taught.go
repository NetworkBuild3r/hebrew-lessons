package evidence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

const (
	maxTaughtBatch = 50
	maxTaughtAge   = 30 // days before today
)

// TaughtWordResult is one word's outcome from Taught.
type TaughtWordResult struct {
	LexemeID int64
	Already  bool
	Created  []string
	HadCard  []string
	Due      time.Time
}

// TaughtResult is the outcome of Taught for every requested word.
type TaughtResult struct {
	Words []TaughtWordResult
}

// RemoveTaughtWordResult is one word's outcome from RemoveTaught.
type RemoveTaughtWordResult struct {
	LexemeID int64
	Removed  []string
	Kept     []string
}

// RemoveTaughtResult is the outcome of RemoveTaught.
type RemoveTaughtResult struct {
	Words []RemoveTaughtWordResult
}

// Taught records words taught outside the app on the learner's civil day on,
// and schedules new cards due at the next study-day cutoff. It never grades,
// never writes attempts or review_log, and never changes an existing card.
// INIT-026/SPEC-005; ADR-0022 D2.
func (r *Recorder) Taught(ctx context.Context, learnerID int64, lexemeIDs []int64, on string, z studyday.Zone) (TaughtResult, error) {
	out := TaughtResult{}
	if err := r.ready(); err != nil {
		return out, err
	}
	if learnerID <= 0 {
		return out, errors.New("evidence: taught has no learner")
	}
	if z.Loc == nil {
		return out, errors.New("evidence: taught has no zone")
	}
	now := r.clock.Now()
	if err := validateOn(on, now, z); err != nil {
		return out, err
	}
	ids, err := normalizeLexemeIDs(lexemeIDs)
	if err != nil {
		return out, err
	}
	due := studyday.DueCutoff(now, z)
	stored, err := r.store.RecordTaught(ctx, learnerID, ids, on, due)
	if err != nil {
		return out, err
	}
	out.Words = make([]TaughtWordResult, len(stored.Words))
	for i, w := range stored.Words {
		out.Words[i] = TaughtWordResult{
			LexemeID: w.LexemeID,
			Already:  w.Already,
			Created:  w.Created,
			HadCard:  w.HadCard,
			Due:      due,
		}
	}
	return out, nil
}

// RemoveTaught undoes Taught for the named words on day on. It deletes the
// taught_words rows and only the cards that row created that still have
// reps = 0 and no review_log. INIT-026/SPEC-005; ADR-0022 D2.
func (r *Recorder) RemoveTaught(ctx context.Context, learnerID int64, on string, lexemeIDs []int64) (RemoveTaughtResult, error) {
	out := RemoveTaughtResult{}
	if err := r.ready(); err != nil {
		return out, err
	}
	if learnerID <= 0 {
		return out, errors.New("evidence: remove taught has no learner")
	}
	if _, err := time.Parse(time.DateOnly, on); err != nil {
		return out, errors.New("evidence: on must be YYYY-MM-DD")
	}
	ids, err := normalizeLexemeIDs(lexemeIDs)
	if err != nil {
		return out, err
	}
	stored, err := r.store.RemoveTaughtRows(ctx, learnerID, on, ids)
	if err != nil {
		return out, err
	}
	out.Words = make([]RemoveTaughtWordResult, len(stored.Words))
	for i, w := range stored.Words {
		out.Words[i] = RemoveTaughtWordResult{
			LexemeID: w.LexemeID,
			Removed:  w.Removed,
			Kept:     w.Kept,
		}
	}
	return out, nil
}

// validateOn checks on is YYYY-MM-DD, not after Today(now, z), and not more
// than 30 days before it.
func validateOn(on string, now time.Time, z studyday.Zone) error {
	if _, err := time.Parse(time.DateOnly, on); err != nil {
		return errors.New("evidence: on must be YYYY-MM-DD")
	}
	today := studyday.Today(now, z)
	if on > today {
		return fmt.Errorf("evidence: on %s is after today %s", on, today)
	}
	todayT, err := time.ParseInLocation(time.DateOnly, today, z.Loc)
	if err != nil {
		return fmt.Errorf("evidence: today: %w", err)
	}
	onT, err := time.ParseInLocation(time.DateOnly, on, z.Loc)
	if err != nil {
		return errors.New("evidence: on must be YYYY-MM-DD")
	}
	earliest := todayT.AddDate(0, 0, -maxTaughtAge)
	if onT.Before(earliest) {
		return fmt.Errorf("evidence: on %s is more than %d days before today", on, maxTaughtAge)
	}
	return nil
}

func normalizeLexemeIDs(lexemeIDs []int64) ([]int64, error) {
	if len(lexemeIDs) == 0 {
		return nil, errors.New("evidence: taught needs 1 to 50 lexeme ids")
	}
	seen := make(map[int64]struct{}, len(lexemeIDs))
	ids := make([]int64, 0, len(lexemeIDs))
	for _, id := range lexemeIDs {
		if id <= 0 {
			return nil, fmt.Errorf("evidence: taught lexeme id %d is not valid", id)
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 || len(ids) > maxTaughtBatch {
		return nil, fmt.Errorf("evidence: taught needs 1 to %d lexeme ids", maxTaughtBatch)
	}
	// Reject when the raw list was over the cap even if duplicates shrunk it.
	if len(lexemeIDs) > maxTaughtBatch {
		return nil, fmt.Errorf("evidence: taught needs 1 to %d lexeme ids", maxTaughtBatch)
	}
	return ids, nil
}
