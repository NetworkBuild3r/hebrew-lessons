// Package dbtest inserts fixture rows for tests.
//
// The helpers used to live on db.Store. They are not part of the server.
// Tests are the only importers, and TestNotLinkedIntoHebrewBinary checks
// that `go list -deps ./cmd/hebrew` does not include this package.
package dbtest

import (
	"context"
	"fmt"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// InsertLexeme inserts a lexeme with text_nfc and returns the id.
func InsertLexeme(ctx context.Context, s *db.Store, textNFC string) (int64, error) {
	res, err := s.ExecContext(ctx, `INSERT INTO lexemes (text_nfc) VALUES (?)`, textNFC)
	if err != nil {
		return 0, fmt.Errorf("dbtest: insert lexeme: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("dbtest: insert lexeme: %w", err)
	}
	return id, nil
}

// InsertUnit inserts a draft or published unit and returns the id.
func InsertUnit(ctx context.Context, s *db.Store, title, status string) (int64, error) {
	res, err := s.ExecContext(ctx, `
		INSERT INTO units (position, title, situation, concept, status, rev, doc)
		VALUES (1, ?, 'street', 'greetings', ?, 1, '{"title":"t"}')`,
		title, status,
	)
	if err != nil {
		return 0, fmt.Errorf("dbtest: insert unit: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("dbtest: insert unit: %w", err)
	}
	return id, nil
}

// InsertLessonItem inserts one lesson_items row.
func InsertLessonItem(ctx context.Context, s *db.Store, unitID int64, lessonIx int, lexemeID int64, role, skill string) error {
	if _, err := s.ExecContext(ctx, `
		INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill)
		VALUES (?, ?, ?, ?, ?)`,
		unitID, lessonIx, lexemeID, role, skill,
	); err != nil {
		return fmt.Errorf("dbtest: insert lesson item: %w", err)
	}
	return nil
}

// InsertLessonRun inserts one lesson_runs row.
// A nil or zero finished time is stored as NULL, so the run is still in progress.
func InsertLessonRun(ctx context.Context, s *db.Store, userID, unitID int64, lessonIx int, started time.Time, finished *time.Time) (int64, error) {
	var done any
	if finished != nil && !finished.IsZero() {
		done = db.FormatTime(*finished)
	}
	res, err := s.ExecContext(ctx, `
		INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?)`,
		userID, unitID, lessonIx, db.FormatTime(started), done,
	)
	if err != nil {
		return 0, fmt.Errorf("dbtest: insert lesson run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("dbtest: insert lesson run: %w", err)
	}
	return id, nil
}
