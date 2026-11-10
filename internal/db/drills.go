package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// DrillAnswer is one graded drill pick for a confusion pair.
type DrillAnswer struct {
	ID        int64
	LearnerID int64
	Kind      string
	Key       string
	Correct   bool
	LatencyMS int64
	At        time.Time
}

// InsertDrillAnswer appends one drill answer for the learner.
func (s *Store) InsertDrillAnswer(ctx context.Context, a DrillAnswer) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	correct := 0
	if a.Correct {
		correct = 1
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO drill_answers (learner_id, kind, key, correct, latency_ms, answered_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		a.LearnerID, a.Kind, a.Key, correct, a.LatencyMS, FormatTime(a.At),
	); err != nil {
		return fmt.Errorf("db: insert drill answer: %w", err)
	}
	return nil
}

// DrillAnswersSince returns answers for one learner, kind, and key at or after
// since, oldest first.
func (s *Store) DrillAnswersSince(ctx context.Context, learnerID int64, kind, key string, since time.Time) ([]DrillAnswer, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, learner_id, kind, key, correct, latency_ms, answered_at
		FROM drill_answers
		WHERE learner_id = ? AND kind = ? AND key = ? AND answered_at >= ?
		ORDER BY answered_at ASC, id ASC`,
		learnerID, kind, key, FormatTime(since),
	)
	if err != nil {
		return nil, fmt.Errorf("db: drill answers: %w", err)
	}
	defer rows.Close()
	var out []DrillAnswer
	for rows.Next() {
		var a DrillAnswer
		var correct int
		var at string
		if err := rows.Scan(&a.ID, &a.LearnerID, &a.Kind, &a.Key, &correct, &a.LatencyMS, &at); err != nil {
			return nil, fmt.Errorf("db: drill answers: %w", err)
		}
		a.Correct = correct != 0
		a.At, err = ParseTime(at)
		if err != nil {
			return nil, fmt.Errorf("db: drill answers: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: drill answers: %w", err)
	}
	return out, nil
}

// CorrectDrillLatencies returns the latencies of the newest timed right
// answers (latency_ms > 0) for the learner, newest first, at most limit.
func (s *Store) CorrectDrillLatencies(ctx context.Context, learnerID int64, limit int) ([]int64, error) {
	if limit <= 0 {
		return nil, nil
	}
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT latency_ms
		FROM drill_answers
		WHERE learner_id = ? AND correct = 1 AND latency_ms > 0
		ORDER BY answered_at DESC, id DESC
		LIMIT ?`, learnerID, limit)
	if err != nil {
		return nil, fmt.Errorf("db: drill latencies: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var ms int64
		if err := rows.Scan(&ms); err != nil {
			return nil, fmt.Errorf("db: drill latencies: %w", err)
		}
		out = append(out, ms)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: drill latencies: %w", err)
	}
	return out, nil
}

// InsertConfusionClear records that the learner cleared one confusion pair.
func (s *Store) InsertConfusionClear(ctx context.Context, learnerID int64, kind, key string, at time.Time) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO confusion_clears (learner_id, kind, key, cleared_at)
		VALUES (?, ?, ?, ?)`,
		learnerID, kind, key, FormatTime(at),
	); err != nil {
		return fmt.Errorf("db: insert confusion clear: %w", err)
	}
	return nil
}

// LatestConfusionClear returns the newest clear for the pair, if any.
func (s *Store) LatestConfusionClear(ctx context.Context, learnerID int64, kind, key string) (time.Time, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return time.Time{}, false, err
	}
	var raw string
	err = conn.QueryRowContext(ctx, `
		SELECT cleared_at
		FROM confusion_clears
		WHERE learner_id = ? AND kind = ? AND key = ?
		ORDER BY cleared_at DESC, id DESC
		LIMIT 1`, learnerID, kind, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("db: latest confusion clear: %w", err)
	}
	at, err := ParseTime(raw)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("db: latest confusion clear: %w", err)
	}
	return at, true, nil
}

// LastConfusionMiss returns the newest miss for the pair, if any.
// An active void leaves that confusion out. INIT-038/SPEC-002.
func (s *Store) LastConfusionMiss(ctx context.Context, learnerID int64, kind, key string) (time.Time, bool, error) {
	return s.lastConfusionMiss(ctx, learnerID, kind, key, EvidenceOpts{})
}

// LastConfusionMissOpts is LastConfusionMiss with EvidenceOpts.
func (s *Store) LastConfusionMissOpts(ctx context.Context, learnerID int64, kind, key string, opts EvidenceOpts) (time.Time, bool, error) {
	return s.lastConfusionMiss(ctx, learnerID, kind, key, opts)
}

func (s *Store) lastConfusionMiss(ctx context.Context, learnerID int64, kind, key string, opts EvidenceOpts) (time.Time, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return time.Time{}, false, err
	}
	var raw string
	err = conn.QueryRowContext(ctx, `
		SELECT at
		FROM confusion_misses
		WHERE learner_id = ? AND kind = ? AND key = ?`+skipVoid(opts.IncludeVoided, "confusion_id", "confusion_misses.id")+`
		ORDER BY at DESC, id DESC
		LIMIT 1`, learnerID, kind, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("db: last confusion miss: %w", err)
	}
	at, err := ParseTime(raw)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("db: last confusion miss: %w", err)
	}
	return at, true, nil
}
