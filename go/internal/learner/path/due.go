package path

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// msPerDay is the civil-day length used by server/learner/model.js
// localDayNumber (thresholds.MS_PER_DAY).
const msPerDay int64 = 24 * 60 * 60 * 1000

// DueCount returns the number of due review rows for userID.
//
// A row in review_items is due when all of these hold:
//
//   - direction is he_en or en_he
//   - suspended is not 1
//   - due parses as an instant
//   - the local calendar day of due is on or before the local calendar day
//     of now (due_in_days <= 0)
//
// localDayNumber matches server/learner/model.js: the UTC day number of the
// local year, month, and day. now is the service clock. The zone is the
// process local zone unless WithLocation set one. Two due directions are
// two rows. Another user's rows are not counted. The 30-due and 10-new
// caps in shared/player/review.js are not applied. The daily-mix library
// and seen-lesson filters are not applied. A card suspended on one
// direction does not hide the other direction.
//
// ADR-0017 §3 says this is "the number of due review rows the review queue
// already counts." HebrewReview.isDue and server/learner/cards.js isDue
// compare instants (due <= now) and do not use the local day. A row due
// later today is due here and is not due in that instant test. The local
// day is what the task requires, and it is the bucketing model.js already
// uses for due_in_days. The golden file records both numbers.
//
// userID must be positive. An unknown user has no rows and returns 0.
// The query is read-only.
func (s *Service) DueCount(ctx context.Context, userID int64) (int, error) {
	if err := ctxErr(ctx); err != nil {
		return 0, err
	}
	if userID <= 0 {
		return 0, ErrBadUser
	}
	if s == nil || s.db == nil {
		return 0, errors.New("learner: nil database")
	}
	return s.countDue(ctx, userID, s.instant())
}

func (s *Service) countDue(ctx context.Context, userID int64, now time.Time) (int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT direction, due, suspended FROM review_items WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		return 0, fmt.Errorf("learner: due count: %w", err)
	}
	defer rows.Close()

	nowDay := localDayNumber(now.UnixMilli(), s.zone())
	n := 0
	for rows.Next() {
		var direction, due string
		var suspended int
		if err := rows.Scan(&direction, &due, &suspended); err != nil {
			return 0, fmt.Errorf("learner: due count: %w", err)
		}
		if suspended == 1 {
			continue
		}
		if direction != "he_en" && direction != "en_he" {
			continue
		}
		dueMs, ok := parseInstant(due)
		if !ok {
			continue
		}
		if localDayNumber(dueMs, s.zone())-nowDay <= 0 {
			n++
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("learner: due count: %w", err)
	}
	return n, nil
}

func (s *Service) instant() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *Service) zone() *time.Location {
	if s != nil && s.loc != nil {
		return s.loc
	}
	return time.Local
}

// localDayNumber is server/learner/model.js localDayNumber.
// Date.UTC(year, month, day) / MS_PER_DAY, rounded. Midnight UTC divides
// evenly by msPerDay, so the rounding step does not move the result.
func localDayNumber(ms int64, loc *time.Location) int64 {
	if loc == nil {
		loc = time.Local
	}
	y, m, d := time.UnixMilli(ms).In(loc).Date()
	utc := time.Date(y, m, d, 0, 0, 0, 0, time.UTC).UnixMilli()
	return utc / msPerDay
}

func parseInstant(raw string) (int64, bool) {
	if raw == "" {
		return 0, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed.UnixMilli(), true
		}
	}
	return 0, false
}
