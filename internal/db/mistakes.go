package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

const (
	maxMistakeRunes = 200
	maxNoteRunes    = 300
	maxMistakeBatch = 50
)

// MistakeInput is one personal mistake. A repeat of the same learner, wrong
// form, and right form increments times and last_at. On is the learner's civil
// day (YYYY-MM-DD). A new pair stores first_on = last_on = On. A repeat sets
// first_on to the earlier day and last_on to the later one. A row with null
// days takes On for both. An empty On leaves the day columns alone (legacy).
// It never writes the shared list. INIT-026/SPEC-005.
type MistakeInput struct {
	Wrong    string
	Right    string
	Note     string
	Pattern  string
	LexemeID int64
	Source   string
	On       string
}

// LearnerMistake is one stored personal mistake.
type LearnerMistake struct {
	ID       int64
	Wrong    string
	Right    string
	Note     string
	Pattern  string
	LexemeID int64
	Times    int
	FirstAt  string
	LastAt   string
	FirstOn  string
	LastOn   string
	Source   string
}

// CommonMistakeInput is one shared mistake. There is no learner on it.
type CommonMistakeInput struct {
	Pattern string
	Wrong   string
	Right   string
	Explain string
	Status  string
}

// CommonMistake is one shared mistake.
type CommonMistake struct {
	ID      int64
	Pattern string
	Wrong   string
	Right   string
	Explain string
	Status  string
}

// MissingLemmaError means a verb-form batch named a lemma that is not in the
// lexicon. The batch wrote nothing.
type MissingLemmaError struct{ ID int64 }

func (e MissingLemmaError) Error() string {
	return fmt.Sprintf("db: lemma %d is not in the lexicon", e.ID)
}

// RecordMistakes writes one learner's mistakes in one transaction. learnerID
// is the caller's. A pair that is already stored increments times and last_at.
// When On is set it must be YYYY-MM-DD; a new pair stores first_on = last_on =
// On, a repeat keeps the earlier first_on and later last_on, and a row with
// null days takes On for both. More than 50, a field that is too long, or an
// unknown pattern writes nothing. INIT-026/SPEC-005.
func (s *Store) RecordMistakes(ctx context.Context, learnerID int64, rows []MistakeInput) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if learnerID <= 0 {
		return errors.New("db: no learner")
	}
	if len(rows) == 0 {
		return errors.New("db: no mistakes")
	}
	if len(rows) > maxMistakeBatch {
		return fmt.Errorf("db: at most %d mistakes", maxMistakeBatch)
	}
	clean := make([]MistakeInput, len(rows))
	for i, row := range rows {
		row.Wrong = units.Normalize(row.Wrong)
		row.Right = units.Normalize(row.Right)
		if row.Wrong == "" || utf8.RuneCountInString(row.Wrong) > maxMistakeRunes {
			return fmt.Errorf("db: wrong form %d", i)
		}
		if row.Right == "" || utf8.RuneCountInString(row.Right) > maxMistakeRunes {
			return fmt.Errorf("db: right form %d", i)
		}
		if utf8.RuneCountInString(row.Note) > maxNoteRunes {
			return fmt.Errorf("db: note %d", i)
		}
		if row.Source != "tutor" && row.Source != "app" {
			return fmt.Errorf("db: source %d", i)
		}
		if row.On != "" {
			if _, err := time.Parse(time.DateOnly, row.On); err != nil {
				return fmt.Errorf("db: on %d is not YYYY-MM-DD", i)
			}
		}
		clean[i] = row
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin mistakes: %w", err)
	}
	defer tx.Rollback()
	now := FormatTime(s.clock.Now().UTC())
	for _, row := range clean {
		var pattern any
		if row.Pattern != "" {
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM grammar_patterns WHERE slug = ?`, row.Pattern).Scan(&n); err != nil {
				return fmt.Errorf("db: pattern: %w", err)
			}
			if n != 1 {
				return fmt.Errorf("db: unknown pattern %q", row.Pattern)
			}
			pattern = row.Pattern
		}
		var lexeme any
		if row.LexemeID > 0 {
			lexeme = row.LexemeID
		}
		var on any
		if row.On != "" {
			on = row.On
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO learner_mistakes (learner_id, wrong_he, right_he, note_en, pattern_slug, lexeme_id, times, first_at, last_at, first_on, last_on, source)
			VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?)
			ON CONFLICT (learner_id, wrong_he, right_he) DO UPDATE SET
				times = learner_mistakes.times + 1,
				last_at = excluded.last_at,
				note_en = CASE WHEN excluded.note_en <> '' THEN excluded.note_en ELSE learner_mistakes.note_en END,
				pattern_slug = COALESCE(excluded.pattern_slug, learner_mistakes.pattern_slug),
				lexeme_id = COALESCE(excluded.lexeme_id, learner_mistakes.lexeme_id),
				first_on = CASE
					WHEN excluded.first_on IS NULL THEN learner_mistakes.first_on
					WHEN learner_mistakes.first_on IS NULL THEN excluded.first_on
					WHEN excluded.first_on < learner_mistakes.first_on THEN excluded.first_on
					ELSE learner_mistakes.first_on
				END,
				last_on = CASE
					WHEN excluded.last_on IS NULL THEN learner_mistakes.last_on
					WHEN learner_mistakes.last_on IS NULL THEN excluded.last_on
					WHEN excluded.last_on > learner_mistakes.last_on THEN excluded.last_on
					ELSE learner_mistakes.last_on
				END`,
			learnerID, row.Wrong, row.Right, row.Note, pattern, lexeme, now, now, on, on, row.Source); err != nil {
			return fmt.Errorf("db: record mistake: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit mistakes: %w", err)
	}
	return nil
}

// MistakesFor returns one learner's mistakes, most repeated first and the
// latest of those first. limit caps the page. There is no method that lists
// every learner's rows.
func (s *Store) MistakesFor(ctx context.Context, learnerID int64, limit int) ([]LearnerMistake, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	if learnerID <= 0 {
		return nil, errors.New("db: no learner")
	}
	if limit <= 0 {
		limit = 10
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, wrong_he, right_he, note_en, COALESCE(pattern_slug, ''), COALESCE(lexeme_id, 0), times, first_at, last_at,
			COALESCE(first_on, ''), COALESCE(last_on, ''), source
		FROM learner_mistakes
		WHERE learner_id = ?
		ORDER BY times DESC, last_at DESC, id
		LIMIT ?`, learnerID, limit)
	if err != nil {
		return nil, fmt.Errorf("db: mistakes: %w", err)
	}
	defer rows.Close()
	var out []LearnerMistake
	for rows.Next() {
		var row LearnerMistake
		if err := rows.Scan(&row.ID, &row.Wrong, &row.Right, &row.Note, &row.Pattern, &row.LexemeID, &row.Times, &row.FirstAt, &row.LastAt, &row.FirstOn, &row.LastOn, &row.Source); err != nil {
			return nil, fmt.Errorf("db: mistakes: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: mistakes: %w", err)
	}
	if out == nil {
		out = []LearnerMistake{}
	}
	return out, nil
}

// UpsertCommonMistakes writes the shared list. A repeat of the same pattern
// and pair replaces the explanation and the status. There is no learner column.
func (s *Store) UpsertCommonMistakes(ctx context.Context, rows []CommonMistakeInput) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return errors.New("db: no common mistakes")
	}
	if len(rows) > maxMistakeBatch {
		return fmt.Errorf("db: at most %d common mistakes", maxMistakeBatch)
	}
	clean := make([]CommonMistakeInput, len(rows))
	for i, row := range rows {
		row.Wrong = units.Normalize(row.Wrong)
		row.Right = units.Normalize(row.Right)
		if row.Wrong == "" || utf8.RuneCountInString(row.Wrong) > maxMistakeRunes || row.Right == "" || utf8.RuneCountInString(row.Right) > maxMistakeRunes {
			return fmt.Errorf("db: common mistake %d", i)
		}
		if utf8.RuneCountInString(row.Explain) > maxNoteRunes {
			return fmt.Errorf("db: explanation %d", i)
		}
		if row.Status != "draft" && row.Status != "published" {
			return fmt.Errorf("db: status %d", i)
		}
		clean[i] = row
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin common mistakes: %w", err)
	}
	defer tx.Rollback()
	for _, row := range clean {
		var pattern any
		if row.Pattern != "" {
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM grammar_patterns WHERE slug = ?`, row.Pattern).Scan(&n); err != nil {
				return err
			}
			if n != 1 {
				return fmt.Errorf("db: unknown pattern %q", row.Pattern)
			}
			pattern = row.Pattern
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO common_mistakes (pattern_slug, wrong_he, right_he, explain_en, status)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (ifnull(pattern_slug, ''), wrong_he, right_he) DO UPDATE SET
				explain_en = excluded.explain_en,
				status = excluded.status`,
			pattern, row.Wrong, row.Right, row.Explain, row.Status); err != nil {
			return fmt.Errorf("db: common mistake: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit common mistakes: %w", err)
	}
	return nil
}

// CommonMistakesFor returns the shared mistakes for one pattern. publishedOnly
// leaves drafts out. A personal mistake is never in this result.
func (s *Store) CommonMistakesFor(ctx context.Context, patternSlug string, publishedOnly bool) ([]CommonMistake, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	q := `
		SELECT id, COALESCE(pattern_slug, ''), wrong_he, right_he, explain_en, status
		FROM common_mistakes
		WHERE pattern_slug = ?`
	if publishedOnly {
		q += ` AND status = 'published'`
	}
	q += ` ORDER BY id`
	rows, err := conn.QueryContext(ctx, q, patternSlug)
	if err != nil {
		return nil, fmt.Errorf("db: common mistakes: %w", err)
	}
	defer rows.Close()
	var out []CommonMistake
	for rows.Next() {
		var row CommonMistake
		if err := rows.Scan(&row.ID, &row.Pattern, &row.Wrong, &row.Right, &row.Explain, &row.Status); err != nil {
			return nil, fmt.Errorf("db: common mistakes: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: common mistakes: %w", err)
	}
	if out == nil {
		out = []CommonMistake{}
	}
	return out, nil
}

// CheckedVerseCount is how many verse notes are checked. Isaiah appears in
// navigation when this is at least one.
func (s *Store) CheckedVerseCount(ctx context.Context) (int, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	var n int
	if err := conn.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM verse_notes WHERE status = 'checked')
			+ (SELECT COUNT(*) FROM verse_word_glosses WHERE status = 'checked')`).Scan(&n); err != nil {
		return 0, fmt.Errorf("db: verse notes: %w", err)
	}
	return n, nil
}

// lexemeExists reports whether id is a lexicon row. Used inside a transaction.
func lexemeExists(ctx context.Context, tx *sql.Tx, id int64) (bool, error) {
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM lexemes WHERE id = ?`, id).Scan(&n); err != nil {
		return false, err
	}
	return n == 1, nil
}
