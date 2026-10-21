package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// This file is the read side of get_progress and the coach note table (INIT-015/SPEC-021).
// Every query here is keyed on user_id. There is no query that takes a learner from
// anything but its caller, and none that reads across learners.

// MaxCoachNotesPerDay is the most notes one learner can get in the 24 hours before a new
// note. A coach note is read into the next lesson's prompt, so the table is not a log.
const MaxCoachNotesPerDay = 100

// ErrTooManyCoachNotes means the learner already has MaxCoachNotesPerDay notes from the
// last 24 hours.
var ErrTooManyCoachNotes = errors.New("db: too many coach notes today")

// CoachNote is one row of coach_notes.
type CoachNote struct {
	ID        int64
	UserID    int64
	Author    string
	Body      string
	CreatedAt time.Time
}

// UserExists reports whether a user has the id.
func (s *Store) UserExists(ctx context.Context, id int64) (bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return false, err
	}
	if id <= 0 {
		return false, nil
	}
	var one int
	err = conn.QueryRowContext(ctx, `SELECT 1 FROM users WHERE id = ?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("db: user exists: %w", err)
	}
	return true, nil
}

// AddCoachNote stores a note for the learner and returns its id. The caller has checked
// the length and the text. It refuses the note when the learner already has
// MaxCoachNotesPerDay notes from the 24 hours before at, and writes nothing.
func (s *Store) AddCoachNote(ctx context.Context, userID int64, author, body string, at time.Time) (int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("db: begin coach note: %w", err)
	}
	defer tx.Rollback()
	var recent int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM coach_notes WHERE user_id = ? AND created_at > ?`,
		userID, FormatTime(at.Add(-24*time.Hour))).Scan(&recent); err != nil {
		return 0, fmt.Errorf("db: count coach notes: %w", err)
	}
	if recent >= MaxCoachNotesPerDay {
		return 0, ErrTooManyCoachNotes
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO coach_notes (user_id, author, body, created_at) VALUES (?, ?, ?, ?)`,
		userID, author, body, FormatTime(at))
	if err != nil {
		return 0, fmt.Errorf("db: insert coach note: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("db: coach note id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("db: commit coach note: %w", err)
	}
	return id, nil
}

// RecentCoachNotes returns the learner's newest notes, newest first, at most limit.
func (s *Store) RecentCoachNotes(ctx context.Context, userID int64, limit int) ([]CoachNote, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		return []CoachNote{}, nil
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, user_id, author, body, created_at FROM coach_notes
		WHERE user_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("db: coach notes: %w", err)
	}
	defer rows.Close()
	out := []CoachNote{}
	for rows.Next() {
		var n CoachNote
		var at string
		if err := rows.Scan(&n.ID, &n.UserID, &n.Author, &n.Body, &at); err != nil {
			return nil, fmt.Errorf("db: coach notes: %w", err)
		}
		if n.CreatedAt, err = ParseTime(at); err != nil {
			return nil, fmt.Errorf("db: coach notes: %w", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: coach notes: %w", err)
	}
	return out, nil
}

// SnapshotRow is one row of daily_snapshots, with its day.
type SnapshotRow struct {
	Day       string
	Skill     string
	State     string
	Count     int
	EstKnown  float64
	Retention *float64
}

// SnapshotRows returns the learner's snapshot rows for the days from through to, both
// YYYY-MM-DD and inclusive, by day, skill, and state.
func (s *Store) SnapshotRows(ctx context.Context, userID int64, from, to string) ([]SnapshotRow, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT day, skill, state, count, est_known, retention FROM daily_snapshots
		WHERE user_id = ? AND day >= ? AND day <= ?
		ORDER BY day, skill, state`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("db: snapshot rows: %w", err)
	}
	defer rows.Close()
	out := []SnapshotRow{}
	for rows.Next() {
		var r SnapshotRow
		var ret sql.NullFloat64
		if err := rows.Scan(&r.Day, &r.Skill, &r.State, &r.Count, &r.EstKnown, &ret); err != nil {
			return nil, fmt.Errorf("db: snapshot rows: %w", err)
		}
		if ret.Valid {
			v := ret.Float64
			r.Retention = &v
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: snapshot rows: %w", err)
	}
	return out, nil
}

// RegressionRow is a regression event with its word's text. Text is empty and LexemeID is 0
// for a band_slip.
type RegressionRow struct {
	Day       string
	Kind      string
	LexemeID  int64
	Text      string
	Skill     string
	FromState string
	ToState   string
	Cause     string
}

// RegressionRows returns the learner's events for the days from through to, newest first,
// at most limit.
func (s *Store) RegressionRows(ctx context.Context, userID int64, from, to string, limit int) ([]RegressionRow, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT e.day, e.kind, COALESCE(e.lexeme_id, 0), COALESCE(l.text_nfc, ''), COALESCE(e.skill, ''), COALESCE(e.from_state, ''),
		       COALESCE(e.to_state, ''), e.cause
		FROM regression_events AS e
		LEFT JOIN lexemes AS l ON l.id = e.lexeme_id
		WHERE e.user_id = ? AND e.day >= ? AND e.day <= ?
		ORDER BY e.day DESC, e.id DESC LIMIT ?`, userID, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("db: regression rows: %w", err)
	}
	defer rows.Close()
	out := []RegressionRow{}
	for rows.Next() {
		var r RegressionRow
		if err := rows.Scan(&r.Day, &r.Kind, &r.LexemeID, &r.Text, &r.Skill, &r.FromState, &r.ToState, &r.Cause); err != nil {
			return nil, fmt.Errorf("db: regression rows: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: regression rows: %w", err)
	}
	return out, nil
}

// WeeklyReportRow is one weekly report without its payload.
type WeeklyReportRow struct {
	WeekStart string
	Summary   string
	CreatedAt time.Time
}

// WeeklyReportRows returns the learner's reports whose week starts from through to,
// newest first, at most limit.
func (s *Store) WeeklyReportRows(ctx context.Context, userID int64, from, to string, limit int) ([]WeeklyReportRow, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT week_start, summary, created_at FROM weekly_reports
		WHERE user_id = ? AND week_start >= ? AND week_start <= ?
		ORDER BY week_start DESC LIMIT ?`, userID, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("db: weekly reports: %w", err)
	}
	defer rows.Close()
	out := []WeeklyReportRow{}
	for rows.Next() {
		var r WeeklyReportRow
		var at string
		if err := rows.Scan(&r.WeekStart, &r.Summary, &at); err != nil {
			return nil, fmt.Errorf("db: weekly reports: %w", err)
		}
		if r.CreatedAt, err = ParseTime(at); err != nil {
			return nil, fmt.Errorf("db: weekly reports: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: weekly reports: %w", err)
	}
	return out, nil
}

// PlacementHistoryRow is one finished placement run.
//
// ID, FinishedAt, StartUnit, StartUnitTitle, and WordsKnown are the onboarding
// history. PlacementRun is the placement_runs row the open-run read returns, so
// this value keeps its own name. Kind is initial or recheck for a v3 run and
// empty for a v2 run. Level and Result are what Progress and get_progress read.
// StartUnitTitle is the published unit at StartUnit in the learner's scope
// (their own unit, or else a shared one). It is empty when there is none,
// including when the only unit at that position is outside the learner's scope.
// A NULL start_unit or words_known, from a run finished before those columns,
// is read as 0 and an empty title.
type PlacementHistoryRow struct {
	ID             int64
	RunID          int64
	Kind           string
	FinishedAt     time.Time
	Level          int
	Result         string
	StartUnit      int
	StartUnitTitle string
	WordsKnown     int
}

// PlacementHistory returns the learner's finished runs, newest finish first.
// A run counts when finished_at is set, except the v2 run Engine.Start closes
// so a v3 run can start. An unfinished run is left out. limit <= 0 returns
// every such run. The LIMIT is in the SQL.
func (s *Store) PlacementHistory(ctx context.Context, userID int64, limit int) ([]PlacementHistoryRow, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	query := `
		SELECT r.id, COALESCE(v.kind, ''), r.finished_at, COALESCE(r.level, -1), COALESCE(r.result, ''),
		       COALESCE(r.start_unit, 0), COALESCE(r.words_known, 0),
		       CASE WHEN r.start_unit IS NULL THEN '' ELSE COALESCE((
		           SELECT u.title
		           FROM units AS u
		           WHERE u.status = 'published'
		             AND u.position = r.start_unit
		             AND (u.scope = 'shared' OR u.scope = 'learner:' || r.user_id)
		           ORDER BY CASE WHEN u.scope = 'shared' THEN 1 ELSE 0 END, u.id
		           LIMIT 1
		       ), '') END
		FROM placement_runs AS r
		LEFT JOIN placement_v3_runs AS v ON v.run_id = r.id
		WHERE r.user_id = ? AND r.finished_at IS NOT NULL
		  AND (` + placementNotAutoClosedSQL("r") + `)
		ORDER BY r.finished_at DESC, r.id DESC`
	args := []any{userID}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("db: placement history: %w", err)
	}
	defer rows.Close()
	out := []PlacementHistoryRow{}
	for rows.Next() {
		var r PlacementHistoryRow
		var at string
		if err := rows.Scan(&r.ID, &r.Kind, &at, &r.Level, &r.Result, &r.StartUnit, &r.WordsKnown, &r.StartUnitTitle); err != nil {
			return nil, fmt.Errorf("db: placement history: %w", err)
		}
		r.RunID = r.ID
		if r.FinishedAt, err = ParseTime(at); err != nil {
			return nil, fmt.Errorf("db: placement history: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: placement history: %w", err)
	}
	return out, nil
}

// PlacementHistoryCount is how many finished runs this learner has.
// The v2 run Engine.Start closes is not counted, and neither is an unfinished run.
func (s *Store) PlacementHistoryCount(ctx context.Context, userID int64) (int, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	var n int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM placement_runs AS r
		WHERE r.user_id = ? AND r.finished_at IS NOT NULL
		  AND (`+placementNotAutoClosedSQL("r")+`)`, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("db: placement history count: %w", err)
	}
	return n, nil
}
