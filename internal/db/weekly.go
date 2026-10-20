package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// This file is the query layer of the weekly report job (INIT-015/SPEC-023). It reads what the
// daily job wrote and writes weekly_reports. The rules that decide what a week holds are in
// internal/jobs. Nothing here decides a gain, a loss, or a sentence.

// maxWeekEvents bounds the events read for one learner and week. A week of real use writes
// a few dozen. The cap only stops a runaway table from filling memory.
const maxWeekEvents = 5000

// WeeklyReport is one row of weekly_reports. The payload is the full report as JSON and the
// summary is three plain sentences. Neither holds anything beyond the learner's own words
// and counts: no email, no typed answers.
type WeeklyReport struct {
	UserID      int64
	WeekStart   string
	PayloadJSON string
	Summary     string
	CreatedAt   time.Time
}

// HasWeeklyReport reports whether the learner has a report for the week.
func (s *Store) HasWeeklyReport(ctx context.Context, userID int64, weekStart string) (bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return false, err
	}
	return hasWeeklyReport(ctx, conn, userID, weekStart)
}

func hasWeeklyReport(ctx context.Context, q queryer, userID int64, weekStart string) (bool, error) {
	var one int
	err := q.QueryRowContext(ctx, `SELECT 1 FROM weekly_reports WHERE user_id = ? AND week_start = ?`, userID, weekStart).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("db: has weekly report: %w", err)
	}
	return true, nil
}

// HasWeeklyReport reports whether the learner has a report for the week, inside the transaction.
func (t *DailyTx) HasWeeklyReport(ctx context.Context, userID int64, weekStart string) (bool, error) {
	return hasWeeklyReport(ctx, t.tx, userID, weekStart)
}

// InsertWeeklyReport writes one report. A report that exists is an error (the primary key is
// the learner and the week), so a second write of the same week fails and the caller's
// transaction rolls back. The row is never updated in place.
func (t *DailyTx) InsertWeeklyReport(ctx context.Context, r WeeklyReport) error {
	if _, err := t.tx.ExecContext(ctx, `
		INSERT INTO weekly_reports (user_id, week_start, payload_json, summary, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		r.UserID, r.WeekStart, r.PayloadJSON, r.Summary, FormatTime(r.CreatedAt)); err != nil {
		return fmt.Errorf("db: insert weekly report: %w", err)
	}
	return nil
}

// WeekEvent is one regression event with its word's text. Text is empty for a band_slip.
type WeekEvent struct {
	Day       string
	Kind      string
	LexemeID  int64
	Text      string
	Skill     string
	FromState string
	ToState   string
	Cause     string
}

// WeekEvents returns the learner's regression events for the days from through to, both
// YYYY-MM-DD and inclusive, oldest first.
func (s *Store) WeekEvents(ctx context.Context, userID int64, from, to string) ([]WeekEvent, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT e.day, e.kind, COALESCE(e.lexeme_id, 0), COALESCE(l.text_nfc, ''), COALESCE(e.skill, ''),
		       COALESCE(e.from_state, ''), COALESCE(e.to_state, ''), e.cause
		FROM regression_events AS e
		LEFT JOIN lexemes AS l ON l.id = e.lexeme_id
		WHERE e.user_id = ? AND e.day >= ? AND e.day <= ?
		ORDER BY e.day, e.id LIMIT ?`, userID, from, to, maxWeekEvents)
	if err != nil {
		return nil, fmt.Errorf("db: week events: %w", err)
	}
	defer rows.Close()
	var out []WeekEvent
	for rows.Next() {
		var e WeekEvent
		if err := rows.Scan(&e.Day, &e.Kind, &e.LexemeID, &e.Text, &e.Skill, &e.FromState, &e.ToState, &e.Cause); err != nil {
			return nil, fmt.Errorf("db: week events: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: week events: %w", err)
	}
	return out, nil
}

// AttemptTime is when one answer was checked and how long the learner took.
type AttemptTime struct {
	At        time.Time
	LatencyMS int
}

// AttemptTimes returns the learner's answers checked at or after from and before to, oldest
// first. The bounds are instants. The query reads a day either side and keeps the rows whose
// parsed time is inside, because rows written before the fixed-width format sort differently
// as text.
func (s *Store) AttemptTimes(ctx context.Context, userID int64, from, to time.Time) ([]AttemptTime, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT created_at, latency_ms FROM attempts
		WHERE user_id = ? AND created_at >= ? AND created_at < ?
		ORDER BY created_at, id`,
		userID, FormatTime(from.Add(-24*time.Hour)), FormatTime(to.Add(24*time.Hour)))
	if err != nil {
		return nil, fmt.Errorf("db: attempt times: %w", err)
	}
	defer rows.Close()
	var out []AttemptTime
	for rows.Next() {
		var at string
		var ms int
		if err := rows.Scan(&at, &ms); err != nil {
			return nil, fmt.Errorf("db: attempt times: %w", err)
		}
		t, err := ParseTime(at)
		if err != nil {
			return nil, fmt.Errorf("db: attempt times: %w", err)
		}
		if t.Before(from) || !t.Before(to) {
			continue
		}
		out = append(out, AttemptTime{At: t, LatencyMS: ms})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: attempt times: %w", err)
	}
	return out, nil
}
