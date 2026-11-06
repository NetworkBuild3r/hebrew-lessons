package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// This file is the query layer of internal/jobs (INIT-015/SPEC-014). It reads and writes
// daily_snapshots, regression_events, and job_state. The rules that decide what to write
// are in internal/jobs. Nothing here decides a state or a regression.

// UserIDs returns every user id, ascending.
func (s *Store) UserIDs(ctx context.Context) ([]int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `SELECT id FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("db: user ids: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("db: user ids: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: user ids: %w", err)
	}
	return out, nil
}

// LastSnapshotDays returns the newest snapshot day of every user that has one. A user with
// no snapshot is not in the map. One query serves all users.
func (s *Store) LastSnapshotDays(ctx context.Context) (map[int64]string, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `SELECT user_id, MAX(day) FROM daily_snapshots GROUP BY user_id`)
	if err != nil {
		return nil, fmt.Errorf("db: last snapshot days: %w", err)
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var day string
		if err := rows.Scan(&id, &day); err != nil {
			return nil, fmt.Errorf("db: last snapshot days: %w", err)
		}
		out[id] = day
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: last snapshot days: %w", err)
	}
	return out, nil
}

// HasSnapshot reports whether the learner has any snapshot row for the day.
func (s *Store) HasSnapshot(ctx context.Context, userID int64, day string) (bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return false, err
	}
	return hasSnapshot(ctx, conn, userID, day)
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func hasSnapshot(ctx context.Context, q queryer, userID int64, day string) (bool, error) {
	var one int
	err := q.QueryRowContext(ctx, `SELECT 1 FROM daily_snapshots WHERE user_id = ? AND day = ? LIMIT 1`, userID, day).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("db: has snapshot: %w", err)
	}
	return true, nil
}

// DailySnapshotRow is one row of daily_snapshots, without its user and day.
type DailySnapshotRow struct {
	Skill    string
	State    string
	Count    int
	EstKnown float64
	// Retention is null when there was not enough data.
	Retention *float64
	// RuleVersion is which learner-model rules produced the row. 0 means 1.
	RuleVersion int
}

// DailyTotal is one row of daily_totals.
type DailyTotal struct {
	Day           string
	KnownAnySkill int
	RuleVersion   int
}

// RegressionEvent is one row of regression_events. A word row has a LexemeID, a Skill, and
// both states. A band_slip row has none of them.
type RegressionEvent struct {
	UserID    int64
	Kind      string // "word" or "band_slip".
	LexemeID  int64
	Skill     string
	FromState string
	ToState   string
	Day       string
	Cause     string
}

// Kinds of regression event.
const (
	EventWord     = "word"
	EventBandSlip = "band_slip"
)

// DailyTx is one transaction of the daily job: the writes for one learner and one day.
type DailyTx struct{ tx *sql.Tx }

// InDailyTx runs fn in one transaction and commits when fn returns nil. Any error rolls
// every write of fn back. The connection is the process's only connection, so fn must not
// call other Store methods: they would wait for the transaction.
func (s *Store) InDailyTx(ctx context.Context, fn func(*DailyTx) error) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin daily: %w", err)
	}
	defer tx.Rollback()
	if err := fn(&DailyTx{tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit daily: %w", err)
	}
	return nil
}

// HasSnapshot reports whether the learner has any snapshot row for the day.
func (t *DailyTx) HasSnapshot(ctx context.Context, userID int64, day string) (bool, error) {
	return hasSnapshot(ctx, t.tx, userID, day)
}

// InsertSnapshot writes the rows of one learner and one day. A row that exists is an error,
// so a second write of the same day fails and the caller's transaction rolls back.
// RuleVersion 0 is stored as 1.
func (t *DailyTx) InsertSnapshot(ctx context.Context, userID int64, day string, rows []DailySnapshotRow) error {
	stmt, err := t.tx.PrepareContext(ctx, `
		INSERT INTO daily_snapshots (user_id, day, skill, state, count, est_known, retention, rule_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("db: insert snapshot: %w", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		var retention any
		if r.Retention != nil {
			retention = *r.Retention
		}
		rv := r.RuleVersion
		if rv == 0 {
			rv = 1
		}
		if _, err := stmt.ExecContext(ctx, userID, day, r.Skill, r.State, r.Count, r.EstKnown, retention, rv); err != nil {
			return fmt.Errorf("db: insert snapshot %s %s: %w", r.Skill, r.State, err)
		}
	}
	return nil
}

// InsertTotals writes one daily_totals row. ruleVersion 0 is stored as 1.
func (t *DailyTx) InsertTotals(ctx context.Context, userID int64, day string, knownAny, ruleVersion int) error {
	if ruleVersion == 0 {
		ruleVersion = 1
	}
	if _, err := t.tx.ExecContext(ctx, `
		INSERT INTO daily_totals (user_id, day, known_any_skill, rule_version)
		VALUES (?, ?, ?, ?)`, userID, day, knownAny, ruleVersion); err != nil {
		return fmt.Errorf("db: insert totals: %w", err)
	}
	return nil
}

// ReplaceDay deletes the learner's snapshot and totals rows for the day and inserts the
// new ones inside the caller's transaction. ruleVersion stamps every inserted row; 0 means 1.
func (t *DailyTx) ReplaceDay(ctx context.Context, userID int64, day string, rows []DailySnapshotRow, knownAny, ruleVersion int) error {
	if ruleVersion == 0 {
		ruleVersion = 1
	}
	if _, err := t.tx.ExecContext(ctx, `DELETE FROM daily_snapshots WHERE user_id = ? AND day = ?`, userID, day); err != nil {
		return fmt.Errorf("db: replace day snapshots: %w", err)
	}
	if _, err := t.tx.ExecContext(ctx, `DELETE FROM daily_totals WHERE user_id = ? AND day = ?`, userID, day); err != nil {
		return fmt.Errorf("db: replace day totals: %w", err)
	}
	stamped := make([]DailySnapshotRow, len(rows))
	copy(stamped, rows)
	for i := range stamped {
		stamped[i].RuleVersion = ruleVersion
	}
	if err := t.InsertSnapshot(ctx, userID, day, stamped); err != nil {
		return err
	}
	return t.InsertTotals(ctx, userID, day, knownAny, ruleVersion)
}

// ListTotals returns the learner's daily_totals rows for from through to, both inclusive, by day.
func (s *Store) ListTotals(ctx context.Context, userID int64, from, to string) ([]DailyTotal, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT day, known_any_skill, rule_version FROM daily_totals
		WHERE user_id = ? AND day >= ? AND day <= ?
		ORDER BY day`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("db: list totals: %w", err)
	}
	defer rows.Close()
	out := []DailyTotal{}
	for rows.Next() {
		var r DailyTotal
		if err := rows.Scan(&r.Day, &r.KnownAnySkill, &r.RuleVersion); err != nil {
			return nil, fmt.Errorf("db: list totals: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list totals: %w", err)
	}
	return out, nil
}

// InsertRegression writes one regression event.
func (t *DailyTx) InsertRegression(ctx context.Context, e RegressionEvent) error {
	kind := e.Kind
	if kind == "" {
		kind = EventWord
	}
	var lexeme, skill, from, to any
	if kind == EventWord {
		lexeme, skill, from, to = e.LexemeID, e.Skill, e.FromState, e.ToState
	}
	if _, err := t.tx.ExecContext(ctx, `
		INSERT INTO regression_events (user_id, kind, lexeme_id, skill, from_state, to_state, day, cause)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.UserID, kind, lexeme, skill, from, to, e.Day, e.Cause); err != nil {
		return fmt.Errorf("db: insert regression: %w", err)
	}
	return nil
}

// BandSlipSince reports whether the learner has a band_slip event for the band on a day
// after the given one. A band_slip cause starts with "band_slip band=<n> ".
func (t *DailyTx) BandSlipSince(ctx context.Context, userID int64, band int, after string) (bool, error) {
	var one int
	err := t.tx.QueryRowContext(ctx, `
		SELECT 1 FROM regression_events
		WHERE user_id = ? AND kind = 'band_slip' AND day > ? AND cause LIKE ? LIMIT 1`,
		userID, after, fmt.Sprintf("band_slip band=%d %%", band)).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("db: band slip since: %w", err)
	}
	return true, nil
}

// JobState reads one job_state value. ok is false when the key is not there.
func (t *DailyTx) JobState(ctx context.Context, key string) (value string, ok bool, err error) {
	err = t.tx.QueryRowContext(ctx, `SELECT value FROM job_state WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("db: job state %s: %w", key, err)
	}
	return value, true, nil
}

// SetJobState writes one job_state value, replacing any older one.
func (t *DailyTx) SetJobState(ctx context.Context, key, value string, at time.Time) error {
	if _, err := t.tx.ExecContext(ctx, `
		INSERT INTO job_state (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, FormatTime(at)); err != nil {
		return fmt.Errorf("db: set job state %s: %w", key, err)
	}
	return nil
}

// JobState reads one job_state value outside a transaction. ok is false when the key is not there.
func (s *Store) JobState(ctx context.Context, key string) (value string, ok bool, err error) {
	conn, err := s.connOrErr()
	if err != nil {
		return "", false, err
	}
	err = conn.QueryRowContext(ctx, `SELECT value FROM job_state WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("db: job state %s: %w", key, err)
	}
	return value, true, nil
}

// TryLockJob takes the named lock for the owner. It succeeds when nobody holds the lock,
// when the owner already holds it (which refreshes it), or when the holder's last write is
// older than ttl, because a holder that died leaves its row behind. It reports false when
// a live holder has it. The take is one transaction, so two processes cannot both win.
func (s *Store) TryLockJob(ctx context.Context, key, owner string, now time.Time, ttl time.Duration) (bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return false, err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("db: lock %s: %w", key, err)
	}
	defer tx.Rollback()
	var holder, updated string
	err = tx.QueryRowContext(ctx, `SELECT value, updated_at FROM job_state WHERE key = ?`, key).Scan(&holder, &updated)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return false, fmt.Errorf("db: lock %s: %w", key, err)
	case holder != owner:
		at, perr := ParseTime(updated)
		if perr == nil && now.Sub(at) < ttl {
			return false, nil
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO job_state (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, owner, FormatTime(now)); err != nil {
		return false, fmt.Errorf("db: lock %s: %w", key, err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("db: lock %s: %w", key, err)
	}
	return true, nil
}

// UnlockJob releases the named lock when the owner holds it. A lock held by someone else,
// or no lock, is left alone.
func (s *Store) UnlockJob(ctx context.Context, key, owner string) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `DELETE FROM job_state WHERE key = ? AND value = ?`, key, owner); err != nil {
		return fmt.Errorf("db: unlock %s: %w", key, err)
	}
	return nil
}
