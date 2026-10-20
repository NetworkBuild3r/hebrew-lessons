package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Reads for the app shell (INIT-015/SPEC-017): the band-slip alert on Today, the lesson
// in progress, and what the Lessons list shows. They add no table and no column.

// BandSlip is one band_slip row of regression_events. Band is the placement band the job
// wrote, 0 to 4. Day is the calendar day the job gave it, YYYY-MM-DD.
type BandSlip struct {
	Band int
	Day  string
}

// RecentBandSlips lists the learner's band_slip events with a day on or after sinceDay
// (YYYY-MM-DD), newest day first and then by band. A row whose cause does not name a band
// is skipped, because it cannot be linked to a review set.
func (s *Store) RecentBandSlips(ctx context.Context, userID int64, sinceDay string) ([]BandSlip, error) {
	if userID <= 0 {
		return nil, ErrNoLearner
	}
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT day, cause FROM regression_events
		WHERE user_id = ? AND kind = 'band_slip' AND day >= ?
		ORDER BY day DESC, id DESC`, userID, sinceDay)
	if err != nil {
		return nil, fmt.Errorf("db: band slips: %w", err)
	}
	defer rows.Close()
	var out []BandSlip
	for rows.Next() {
		var day, cause string
		if err := rows.Scan(&day, &cause); err != nil {
			return nil, fmt.Errorf("db: band slips: %w", err)
		}
		band, ok := slipBand(cause)
		if !ok {
			continue
		}
		out = append(out, BandSlip{Band: band, Day: day})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: band slips: %w", err)
	}
	return out, nil
}

// slipBand reads the band out of "band_slip band=<n> ...", the cause the daily job writes.
func slipBand(cause string) (int, bool) {
	rest, ok := strings.CutPrefix(cause, "band_slip band=")
	if !ok {
		return 0, false
	}
	digits, _, _ := strings.Cut(rest, " ")
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// InProgressLessonRun is the learner's open run of one lesson when they have already
// answered something in it: past its first activity, or looking at an answer. A run that
// was only opened is not in progress, because there is nothing to continue.
func (s *Store) InProgressLessonRun(ctx context.Context, userID, unitID int64, lessonIx int) (PlayerRun, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return PlayerRun{}, false, err
	}
	run, err := scanPlayerRun(conn.QueryRowContext(ctx, `
		SELECT `+playerRunColumns+` FROM lesson_runs
		WHERE user_id = ? AND unit_id = ? AND lesson_ix = ? AND finished_at IS NULL
		  AND (activity_ix > 0 OR phase = 'reveal')
		ORDER BY id DESC LIMIT 1`, userID, unitID, lessonIx).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return PlayerRun{}, false, nil
	}
	if err != nil {
		return PlayerRun{}, false, fmt.Errorf("db: in-progress lesson: %w", err)
	}
	return run, true, nil
}

// UnitProgress is how far a learner is through one published unit in their scope.
type UnitProgress struct {
	UnitID   int64
	Position int
	Title    string
	Scope    string
	Lessons  int
	Finished int
}

// LearnerUnitProgress lists the published units in the learner's scope, by position, with
// the lessons finished in each. A unit the learner reported is left out: it is hidden
// for them (unit_reports). A unit with no lessons is skipped, as the player skips it.
func (s *Store) LearnerUnitProgress(ctx context.Context, userID int64) ([]UnitProgress, error) {
	if userID <= 0 {
		return nil, ErrNoLearner
	}
	rows, err := s.UnitsInScope(ctx, userID)
	if err != nil {
		return nil, err
	}
	hidden, err := s.HiddenUnitIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	gone := map[int64]bool{}
	for _, id := range hidden {
		gone[id] = true
	}
	finished, err := s.finishedLessons(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []UnitProgress
	for _, row := range rows {
		if row.Status != "published" || gone[row.ID] {
			continue
		}
		var doc struct {
			Lessons []json.RawMessage `json:"lessons"`
		}
		if err := json.Unmarshal([]byte(row.Doc), &doc); err != nil || len(doc.Lessons) == 0 {
			continue
		}
		done := 0
		for ix := range doc.Lessons {
			if _, ok := finished[row.ID][ix]; ok {
				done++
			}
		}
		out = append(out, UnitProgress{
			UnitID: row.ID, Position: row.Position, Title: row.Title, Scope: row.Scope,
			Lessons: len(doc.Lessons), Finished: done,
		})
	}
	return out, nil
}
