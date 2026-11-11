package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/lessons"
)

// ClassSheet is one stored sheet and the lines the lesson builder reads.
type ClassSheet struct {
	ID      int64
	Ref     string
	ClassOn string
	Title   string
	Status  string
	Lines   []lessons.Line
}

// SaveClassSheet stores one import for one learner.
// The same file reference with the same lines changes nothing.
// A different body on that reference replaces the lines and the check.
func (s *Store) SaveClassSheet(ctx context.Context, learnerID int64, in lessons.Import) (id int64, unchanged bool, err error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, false, err
	}
	if s.clock == nil {
		return 0, false, fmt.Errorf("db: clock is not configured")
	}
	if learnerID <= 0 || in.Ref == "" {
		return 0, false, fmt.Errorf("db: sheet needs a learner and a file reference")
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()

	status := "failed"
	if in.Passed {
		status = "passed"
	}
	now := FormatTime(s.clock.Now())
	var existing int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM class_sheets WHERE learner_id = ? AND sheet_ref = ?`, learnerID, in.Ref).Scan(&existing)
	if err != nil && err != sql.ErrNoRows {
		return 0, false, fmt.Errorf("db: read sheet: %w", err)
	}
	if err == nil {
		same, err := sameLines(ctx, tx, existing, in.Lines)
		if err != nil {
			return 0, false, err
		}
		if same {
			return existing, true, nil
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM class_lines WHERE sheet_id = ?`, existing); err != nil {
			return 0, false, fmt.Errorf("db: replace sheet lines: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE class_sheets SET class_on = ?, title = ?, status = ?, check_note = ?, checked_at = ? WHERE id = ?`,
			in.ClassOn, in.Title, status, in.Note, now, existing); err != nil {
			return 0, false, fmt.Errorf("db: update sheet: %w", err)
		}
		id = existing
	} else {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO class_sheets (learner_id, sheet_ref, class_on, title, status, check_note, created_at, checked_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			learnerID, in.Ref, in.ClassOn, in.Title, status, in.Note, now, now)
		if err != nil {
			return 0, false, fmt.Errorf("db: insert sheet: %w", err)
		}
		id, err = res.LastInsertId()
		if err != nil {
			return 0, false, err
		}
	}
	for _, line := range in.Lines {
		if err := insertClassLine(ctx, tx, id, line); err != nil {
			return 0, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	return id, false, nil
}

func sameLines(ctx context.Context, tx *sql.Tx, sheetID int64, want []lessons.Line) (bool, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT position, tutor_text, learner_text, gloss_en, state FROM class_lines
		WHERE sheet_id = ? ORDER BY position`, sheetID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	var got []lessons.Line
	for rows.Next() {
		var pos int
		var tutor, learner, gloss, state sql.NullString
		if err := rows.Scan(&pos, &tutor, &learner, &gloss, &state); err != nil {
			return false, err
		}
		got = append(got, lessons.Line{
			Position: pos, Tutor: tutor.String, Learner: learner.String, Gloss: gloss.String, Held: state.String == "held",
		})
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if len(got) != len(want) {
		return false, nil
	}
	for i := range want {
		if got[i].Tutor != want[i].Tutor || got[i].Learner != want[i].Learner || got[i].Gloss != want[i].Gloss || got[i].Held != want[i].Held {
			return false, nil
		}
	}
	return true, nil
}

func insertClassLine(ctx context.Context, tx *sql.Tx, sheetID int64, line lessons.Line) error {
	state := "ok"
	reason := ""
	if line.Held {
		state = "held"
		reason = line.HeldReason
		if reason == "" {
			reason = "held"
		}
	}
	var tutor, learner, gloss any
	if line.Tutor != "" {
		tutor = line.Tutor
	}
	if line.Learner != "" {
		learner = line.Learner
	}
	if line.Gloss != "" {
		gloss = line.Gloss
	}
	// A held line with no speaker still needs exactly one text column.
	if tutor == nil && learner == nil {
		learner = " "
		if line.HeldReason != "" {
			learner = trimLine(line.HeldReason)
		}
	}
	points := 0
	if line.HasPoints {
		points = 1
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO class_lines (sheet_id, position, tutor_text, learner_text, gloss_en, has_points, state, held_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sheetID, line.Position, tutor, learner, gloss, points, state, reason)
	if err != nil {
		return fmt.Errorf("db: insert class line: %w", err)
	}
	return nil
}

func trimLine(s string) string {
	if len(s) > 2000 {
		return s[:2000]
	}
	if s == "" {
		return " "
	}
	return s
}

// ClassSheets returns one learner's sheets, newest class first.
func (s *Store) ClassSheets(ctx context.Context, learnerID int64) ([]ClassSheet, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, sheet_ref, class_on, title, status FROM class_sheets
		WHERE learner_id = ? ORDER BY class_on DESC, id DESC`, learnerID)
	if err != nil {
		return nil, fmt.Errorf("db: list sheets: %w", err)
	}
	defer rows.Close()
	var out []ClassSheet
	for rows.Next() {
		var sh ClassSheet
		if err := rows.Scan(&sh.ID, &sh.Ref, &sh.ClassOn, &sh.Title, &sh.Status); err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		lines, err := s.classLines(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Lines = lines
	}
	return out, nil
}

func (s *Store) classLines(ctx context.Context, sheetID int64) ([]lessons.Line, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT position, tutor_text, learner_text, gloss_en, has_points, state, held_reason
		FROM class_lines WHERE sheet_id = ? ORDER BY position`, sheetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []lessons.Line
	for rows.Next() {
		var line lessons.Line
		var tutor, learner, gloss sql.NullString
		var points int
		var state, reason string
		if err := rows.Scan(&line.Position, &tutor, &learner, &gloss, &points, &state, &reason); err != nil {
			return nil, err
		}
		line.Tutor, line.Learner, line.Gloss = tutor.String, learner.String, gloss.String
		line.HasPoints = points == 1
		line.Held = state == "held"
		line.HeldReason = reason
		out = append(out, line)
	}
	return out, rows.Err()
}

// FlagItem takes one word out of lessons and review. It does not change the answer.
func (s *Store) FlagItem(ctx context.Context, learnerID, lexemeID, attemptID int64, reason, batch string) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if s.clock == nil {
		return fmt.Errorf("db: clock is not configured")
	}
	switch reason {
	case "also_right", "wrong_question", "bad_item":
	default:
		return fmt.Errorf("db: flag reason %q", reason)
	}
	var attempt any
	if attemptID > 0 {
		attempt = attemptID
	}
	var batchVal any
	if batch != "" {
		batchVal = batch
	}
	_, err = conn.ExecContext(ctx, `
		INSERT INTO item_flags (learner_id, lexeme_id, attempt_id, void_batch_id, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		learnerID, lexemeID, attempt, batchVal, reason, FormatTime(s.clock.Now()))
	if err != nil {
		return fmt.Errorf("db: flag item: %w", err)
	}
	return nil
}

// UndoFlag marks the learner's open flags in one batch as undone.
func (s *Store) UndoFlag(ctx context.Context, learnerID int64, batch string) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if s.clock == nil {
		return fmt.Errorf("db: clock is not configured")
	}
	_, err = conn.ExecContext(ctx, `
		UPDATE item_flags SET undone_at = ? WHERE learner_id = ? AND void_batch_id = ? AND undone_at IS NULL`,
		FormatTime(s.clock.Now()), learnerID, batch)
	if err != nil {
		return fmt.Errorf("db: undo flag: %w", err)
	}
	return nil
}

// FlaggedLexemes are the words this learner has taken out, while the flag is open.
func (s *Store) FlaggedLexemes(ctx context.Context, learnerID int64) (map[int64]bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT lexeme_id FROM item_flags WHERE learner_id = ? AND undone_at IS NULL`, learnerID)
	if err != nil {
		return nil, fmt.Errorf("db: flagged words: %w", err)
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// RequirePassedSheet returns the id of a passed sheet with this file reference.
func (s *Store) RequirePassedSheet(ctx context.Context, learnerID int64, ref string) (int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	var id int64
	err = conn.QueryRowContext(ctx, `
		SELECT id FROM class_sheets WHERE learner_id = ? AND sheet_ref = ? AND status = 'passed'`,
		learnerID, ref).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("db: no passed class sheet %q", ref)
	}
	if err != nil {
		return 0, fmt.Errorf("db: class sheet: %w", err)
	}
	return id, nil
}

// AttachTaughtSheet sets the sheet on taught rows for these words on this day.
func (s *Store) AttachTaughtSheet(ctx context.Context, learnerID int64, lexemeIDs []int64, on string, sheetID int64) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	for _, lex := range lexemeIDs {
		if _, err := conn.ExecContext(ctx, `
			UPDATE taught_words SET sheet_id = ? WHERE learner_id = ? AND lexeme_id = ? AND taught_on = ?`,
			sheetID, learnerID, lex, on); err != nil {
			return fmt.Errorf("db: attach sheet: %w", err)
		}
	}
	return nil
}

// Story is the stored story for one lesson type.
type Story struct {
	Body     string
	Status   string
	Failures string
}

// SaveStory replaces the one story for this learner and lesson type.
func (s *Store) SaveStory(ctx context.Context, learnerID int64, lessonType, body, status, failures string) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if s.clock == nil {
		return fmt.Errorf("db: clock is not configured")
	}
	if lessonType != "word" && lessonType != "easy" {
		return fmt.Errorf("db: lesson type %q", lessonType)
	}
	_, err = conn.ExecContext(ctx, `
		INSERT INTO story_slots (learner_id, lesson_type, body, status, failures, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (learner_id, lesson_type) DO UPDATE SET
			body = excluded.body, status = excluded.status, failures = excluded.failures, created_at = excluded.created_at`,
		learnerID, lessonType, body, status, failures, FormatTime(s.clock.Now()))
	if err != nil {
		return fmt.Errorf("db: save story: %w", err)
	}
	return nil
}

// LoadStory reads the stored story. ok is false when there is none.
func (s *Store) LoadStory(ctx context.Context, learnerID int64, lessonType string) (Story, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return Story{}, false, err
	}
	var out Story
	err = conn.QueryRowContext(ctx, `
		SELECT body, status, failures FROM story_slots WHERE learner_id = ? AND lesson_type = ?`,
		learnerID, lessonType).Scan(&out.Body, &out.Status, &out.Failures)
	if err == sql.ErrNoRows {
		return Story{}, false, nil
	}
	if err != nil {
		return Story{}, false, fmt.Errorf("db: load story: %w", err)
	}
	return out, true, nil
}

// SetLessonType records whether a saved unit is a word lesson or an easy lesson.
func (s *Store) SetLessonType(ctx context.Context, unitID int64, kind string) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if kind != "word" && kind != "easy" {
		return fmt.Errorf("db: lesson type %q", kind)
	}
	_, err = conn.ExecContext(ctx, `UPDATE units SET lesson_type = ? WHERE id = ?`, kind, unitID)
	if err != nil {
		return fmt.Errorf("db: lesson type: %w", err)
	}
	return nil
}
