package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Void reasons (ADR-0033 D1). INIT-038/SPEC-002.
const (
	VoidReasonUnit1234   = "unit_12_34"
	VoidReasonNoCueBlank = "no_cue_blank"
	VoidReasonAlsoRight  = "also_right"
	VoidReasonReport     = "report"
)

// EvidenceOpts chooses whether a reader counts voided answers.
// The zero value skips a void whose undone_at is null. IncludeVoided counts
// those rows, for an audit or the one-time comparison. A void whose undone_at
// is set always counts. INIT-038/SPEC-002.
type EvidenceOpts struct {
	IncludeVoided bool
}

// AnswerVoid is one answer_voids row. ReviewLogID and ConfusionID are 0 when
// the void does not name that row. UndoneAt is zero while the void is in force.
type AnswerVoid struct {
	ID          int64
	AttemptID   int64
	ReviewLogID int64
	ConfusionID int64
	LearnerID   int64
	Reason      string
	Actor       string
	BatchID     string
	CreatedAt   time.Time
	UndoneAt    time.Time
}

// AnswerVoidWrite is one row of a batch. ReviewLogID and ConfusionID are 0
// when the answer did not write that row. A reader skips a review_log row or
// a confusion row only when the void names it.
type AnswerVoidWrite struct {
	AttemptID   int64
	ReviewLogID int64
	ConfusionID int64
	LearnerID   int64
	Reason      string
	Actor       string
}

// skipVoid is an AND NOT EXISTS clause. column is attempt_id, review_log_id,
// or confusion_id. rowID is a fixed SQL expression for the evidence row, such
// as "attempts.id" or "rl.id". Neither value is learner input.
// includeVoided returns an empty string so the query counts the row.
func skipVoid(includeVoided bool, column, rowID string) string {
	if includeVoided {
		return ""
	}
	switch column {
	case "attempt_id", "review_log_id", "confusion_id":
	default:
		panic("db: skipVoid column " + column)
	}
	return ` AND NOT EXISTS (
		SELECT 1 FROM answer_voids AS av
		WHERE av.` + column + ` = ` + rowID + ` AND av.undone_at IS NULL)`
}

// InsertAnswerVoids writes one answer_voids row per item, in one transaction,
// under batchID. It does not edit attempts, review_log, or confusion_misses.
// The attempt must belong to the named learner. A review_log id, when set,
// must be that learner's review of the same word and skill. A confusion id,
// when set, must be that learner's miss, and the same word when it names one.
func (s *Store) InsertAnswerVoids(ctx context.Context, batchID string, at time.Time, rows []AnswerVoidWrite) ([]AnswerVoid, error) {
	if err := validVoidBatch(batchID, at, rows); err != nil {
		return nil, err
	}
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("db: begin voids: %w", err)
	}
	defer tx.Rollback()

	created := FormatTime(at)
	out := make([]AnswerVoid, 0, len(rows))
	for _, row := range rows {
		lexeme, skill, err := voidAttemptOwned(ctx, tx, row)
		if err != nil {
			return nil, err
		}
		if err := voidReviewOwned(ctx, tx, row, lexeme, skill); err != nil {
			return nil, err
		}
		if err := voidConfusionOwned(ctx, tx, row, lexeme); err != nil {
			return nil, err
		}
		var review, confusion any
		if row.ReviewLogID != 0 {
			review = row.ReviewLogID
		}
		if row.ConfusionID != 0 {
			confusion = row.ConfusionID
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO answer_voids (
				attempt_id, review_log_id, confusion_id, learner_id, reason, actor, batch_id, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			row.AttemptID, review, confusion, row.LearnerID, row.Reason, row.Actor, batchID, created,
		)
		if err != nil {
			return nil, fmt.Errorf("db: insert void: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("db: insert void: %w", err)
		}
		out = append(out, AnswerVoid{
			ID: id, AttemptID: row.AttemptID, ReviewLogID: row.ReviewLogID, ConfusionID: row.ConfusionID,
			LearnerID: row.LearnerID, Reason: row.Reason, Actor: row.Actor, BatchID: batchID, CreatedAt: at.UTC(),
		})
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("db: commit voids: %w", err)
	}
	return out, nil
}

// UndoAnswerVoidBatch sets undone_at on every active row in the batch.
// The answer counts again. Rows already undone keep their first undone_at.
// An unknown batch is an error. Attempts, review_log, and confusion_misses
// are left as they are.
func (s *Store) UndoAnswerVoidBatch(ctx context.Context, batchID string, at time.Time) error {
	if strings.TrimSpace(batchID) == "" || len(batchID) > 200 {
		return errors.New("db: undo void batch needs a batch id")
	}
	if at.IsZero() {
		return errors.New("db: undo void batch needs a time")
	}
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin undo voids: %w", err)
	}
	defer tx.Rollback()
	var n, active int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN undone_at IS NULL THEN 1 ELSE 0 END), 0)
		FROM answer_voids WHERE batch_id = ?`, batchID).Scan(&n, &active); err != nil {
		return fmt.Errorf("db: undo void batch: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("db: void batch %s: not found", batchID)
	}
	if active > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE answer_voids SET undone_at = ? WHERE batch_id = ? AND undone_at IS NULL`,
			FormatTime(at), batchID); err != nil {
			return fmt.Errorf("db: undo void batch: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit undo voids: %w", err)
	}
	return nil
}

// ListAnswerVoidsByLearner returns that learner's voids, oldest first,
// including ones already undone.
func (s *Store) ListAnswerVoidsByLearner(ctx context.Context, learnerID int64) ([]AnswerVoid, error) {
	if learnerID <= 0 {
		return nil, errors.New("db: list voids needs a learner")
	}
	return s.listAnswerVoids(ctx, `WHERE learner_id = ?`, learnerID)
}

// ListAnswerVoidsByBatch returns one batch, oldest first, including rows
// already undone. An unknown batch is an empty list.
func (s *Store) ListAnswerVoidsByBatch(ctx context.Context, batchID string) ([]AnswerVoid, error) {
	if strings.TrimSpace(batchID) == "" {
		return nil, errors.New("db: list voids needs a batch")
	}
	return s.listAnswerVoids(ctx, `WHERE batch_id = ?`, batchID)
}

func (s *Store) listAnswerVoids(ctx context.Context, where string, arg any) ([]AnswerVoid, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, attempt_id, review_log_id, confusion_id, learner_id, reason, actor, batch_id, created_at, undone_at
		FROM answer_voids `+where+` ORDER BY id`, arg)
	if err != nil {
		return nil, fmt.Errorf("db: list voids: %w", err)
	}
	defer rows.Close()
	var out []AnswerVoid
	for rows.Next() {
		v, err := scanAnswerVoid(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list voids: %w", err)
	}
	return out, nil
}

func scanAnswerVoid(scan func(dest ...any) error) (AnswerVoid, error) {
	var v AnswerVoid
	var review, confusion sql.NullInt64
	var created string
	var undone sql.NullString
	if err := scan(&v.ID, &v.AttemptID, &review, &confusion, &v.LearnerID, &v.Reason, &v.Actor, &v.BatchID, &created, &undone); err != nil {
		return AnswerVoid{}, fmt.Errorf("db: list voids: %w", err)
	}
	if review.Valid {
		v.ReviewLogID = review.Int64
	}
	if confusion.Valid {
		v.ConfusionID = confusion.Int64
	}
	var err error
	if v.CreatedAt, err = ParseTime(created); err != nil {
		return AnswerVoid{}, fmt.Errorf("db: list voids: %w", err)
	}
	if undone.Valid {
		if v.UndoneAt, err = ParseTime(undone.String); err != nil {
			return AnswerVoid{}, fmt.Errorf("db: list voids: %w", err)
		}
	}
	return v, nil
}

func validVoidBatch(batchID string, at time.Time, rows []AnswerVoidWrite) error {
	if strings.TrimSpace(batchID) == "" || len(batchID) > 200 {
		return errors.New("db: void batch needs a batch id")
	}
	if at.IsZero() {
		return errors.New("db: void batch needs a time")
	}
	if len(rows) == 0 {
		return errors.New("db: void batch needs a row")
	}
	for i, row := range rows {
		if row.AttemptID <= 0 || row.LearnerID <= 0 {
			return fmt.Errorf("db: void row %d needs an attempt and a learner", i)
		}
		if row.ReviewLogID < 0 || row.ConfusionID < 0 {
			return fmt.Errorf("db: void row %d has a negative id", i)
		}
		if !validVoidReason(row.Reason) {
			return fmt.Errorf("db: void row %d reason %q", i, row.Reason)
		}
		if strings.TrimSpace(row.Actor) == "" || len(row.Actor) > 200 {
			return fmt.Errorf("db: void row %d needs an actor", i)
		}
	}
	return nil
}

func validVoidReason(reason string) bool {
	switch reason {
	case VoidReasonUnit1234, VoidReasonNoCueBlank, VoidReasonAlsoRight, VoidReasonReport:
		return true
	default:
		return false
	}
}

func voidAttemptOwned(ctx context.Context, tx *sql.Tx, row AnswerVoidWrite) (lexeme int64, skill string, err error) {
	var user int64
	err = tx.QueryRowContext(ctx, `SELECT user_id, lexeme_id, skill FROM attempts WHERE id = ?`, row.AttemptID).Scan(&user, &lexeme, &skill)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", fmt.Errorf("db: void attempt %d: not found", row.AttemptID)
	}
	if err != nil {
		return 0, "", fmt.Errorf("db: void attempt: %w", err)
	}
	if user != row.LearnerID {
		return 0, "", fmt.Errorf("db: void attempt %d belongs to learner %d", row.AttemptID, user)
	}
	return lexeme, skill, nil
}

func voidReviewOwned(ctx context.Context, tx *sql.Tx, row AnswerVoidWrite, lexeme int64, skill string) error {
	if row.ReviewLogID == 0 {
		return nil
	}
	var user, cardLex int64
	var cardSkill string
	err := tx.QueryRowContext(ctx, `
		SELECT c.user_id, c.lexeme_id, c.skill
		FROM review_log AS rl
		JOIN cards AS c ON c.id = rl.card_id
		WHERE rl.id = ?`, row.ReviewLogID).Scan(&user, &cardLex, &cardSkill)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("db: void review %d: not found", row.ReviewLogID)
	}
	if err != nil {
		return fmt.Errorf("db: void review: %w", err)
	}
	if user != row.LearnerID || cardLex != lexeme || cardSkill != skill {
		return fmt.Errorf("db: void review %d is not this learner's review of the attempt", row.ReviewLogID)
	}
	return nil
}

func voidConfusionOwned(ctx context.Context, tx *sql.Tx, row AnswerVoidWrite, lexeme int64) error {
	if row.ConfusionID == 0 {
		return nil
	}
	var learner int64
	var stored sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT learner_id, lexeme_id FROM confusion_misses WHERE id = ?`, row.ConfusionID).Scan(&learner, &stored)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("db: void confusion %d: not found", row.ConfusionID)
	}
	if err != nil {
		return fmt.Errorf("db: void confusion: %w", err)
	}
	if learner != row.LearnerID {
		return fmt.Errorf("db: void confusion %d belongs to learner %d", row.ConfusionID, learner)
	}
	if stored.Valid && stored.Int64 != lexeme {
		return fmt.Errorf("db: void confusion %d is a different word", row.ConfusionID)
	}
	return nil
}
