package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// CourseRow is one ready course word to add to the shared lexicon.
type CourseRow struct {
	Order    int
	Rank     int
	Lemma    string
	Pointed  string
	Translit string
	Gloss    string
	POS      string
	Gender   string
	Root     string
	Binyan   string
}

// CourseSync counts what one sync did. A stored lexeme is never changed.
type CourseSync struct {
	Added   int
	Linked  int
	Skipped int
	Moved   int
}

// CourseLexeme is one course word already in the lexicon, in course order.
type CourseLexeme struct {
	LexemeID int64
	Order    int
	Rank     int
	Lemma    string
}

// SyncCourse inserts course words that are not already in the lexicon and
// records the ones that are. It never updates a lexeme row and never deletes.
func (s *Store) SyncCourse(ctx context.Context, rows []CourseRow) (CourseSync, error) {
	var out CourseSync
	if len(rows) == 0 {
		return out, nil
	}
	conn, err := s.connOrErr()
	if err != nil {
		return out, err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("db: begin course: %w", err)
	}
	defer tx.Rollback()

	spellings, err := lexemeSpellings(ctx, tx)
	if err != nil {
		return out, err
	}
	have, err := courseLinks(ctx, tx)
	if err != nil {
		return out, err
	}
	now := FormatTime(s.clock.Now().UTC())
	for _, row := range rows {
		spelling := units.StripNiqqud(units.Normalize(row.Pointed))
		ids := spellings[spelling]
		switch len(ids) {
		case 0:
			id, err := insertCourseLexeme(ctx, tx, row)
			if err != nil {
				return out, err
			}
			if err := insertCourseWord(ctx, tx, id, row, 0, now); err != nil {
				return out, err
			}
			spellings[spelling] = []int64{id}
			have[id] = courseLink{Order: row.Order, Rank: row.Rank, Lemma: row.Lemma}
			out.Added++
		case 1:
			id := ids[0]
			cur, ok := have[id]
			if !ok {
				if err := insertCourseWord(ctx, tx, id, row, 1, now); err != nil {
					return out, err
				}
				have[id] = courseLink{Order: row.Order, Rank: row.Rank, Lemma: row.Lemma}
				out.Linked++
				continue
			}
			if cur.Order == row.Order && cur.Rank == row.Rank && cur.Lemma == row.Lemma {
				continue
			}
			if err := moveCourseWord(ctx, tx, id, row); err != nil {
				return out, err
			}
			have[id] = courseLink{Order: row.Order, Rank: row.Rank, Lemma: row.Lemma}
			out.Moved++
		default:
			out.Skipped++
			slog.Debug("course skip", "pointed", row.Pointed, "spellings", len(ids))
		}
	}
	if err := tx.Commit(); err != nil {
		return out, fmt.Errorf("db: commit course: %w", err)
	}
	return out, nil
}

type courseLink struct {
	Order int
	Rank  int
	Lemma string
}

func lexemeSpellings(ctx context.Context, tx *sql.Tx) (map[string][]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, text_nfc, bare FROM lexemes`)
	if err != nil {
		return nil, fmt.Errorf("db: course spellings: %w", err)
	}
	defer rows.Close()
	out := map[string][]int64{}
	for rows.Next() {
		var id int64
		var text, bare string
		if err := rows.Scan(&id, &text, &bare); err != nil {
			return nil, fmt.Errorf("db: course spellings: %w", err)
		}
		spelling := bare
		if spelling == "" {
			spelling = units.StripNiqqud(text)
		}
		out[spelling] = append(out[spelling], id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: course spellings: %w", err)
	}
	return out, nil
}

func courseLinks(ctx context.Context, tx *sql.Tx) (map[int64]courseLink, error) {
	rows, err := tx.QueryContext(ctx, `SELECT lexeme_id, course_order, list_rank, lemma FROM course_words`)
	if err != nil {
		return nil, fmt.Errorf("db: course links: %w", err)
	}
	defer rows.Close()
	out := map[int64]courseLink{}
	for rows.Next() {
		var id int64
		var link courseLink
		if err := rows.Scan(&id, &link.Order, &link.Rank, &link.Lemma); err != nil {
			return nil, fmt.Errorf("db: course links: %w", err)
		}
		out[id] = link
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: course links: %w", err)
	}
	return out, nil
}

func insertCourseLexeme(ctx context.Context, tx *sql.Tx, row CourseRow) (int64, error) {
	prepared, bad := lexicon.Prepare([]lexicon.Input{courseInput(row)})
	if bad != nil {
		return 0, bad
	}
	return insertLexemeRow(ctx, tx, prepared[0])
}

func courseInput(row CourseRow) lexicon.Input {
	pos := row.POS
	translit := row.Translit
	in := lexicon.Input{TextNFC: row.Pointed, Gloss: row.Gloss, Translit: &translit, POS: &pos}
	if row.Gender != "" {
		gender := row.Gender
		in.Gender = &gender
	}
	if row.Root != "" {
		root := row.Root
		in.Root = &root
	}
	if row.Binyan != "" {
		binyan := row.Binyan
		in.Binyan = &binyan
	}
	return in
}

func insertCourseWord(ctx context.Context, tx *sql.Tx, id int64, row CourseRow, linked int, now string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO course_words (lexeme_id, course_order, list_rank, lemma, linked, added_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, row.Order, row.Rank, row.Lemma, linked, now)
	if err != nil {
		return fmt.Errorf("db: insert course word: %w", err)
	}
	return nil
}

func moveCourseWord(ctx context.Context, tx *sql.Tx, id int64, row CourseRow) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE course_words SET course_order = ?, list_rank = ?, lemma = ? WHERE lexeme_id = ?`,
		row.Order, row.Rank, row.Lemma, id)
	if err != nil {
		return fmt.Errorf("db: move course word: %w", err)
	}
	return nil
}

// CourseLexemes returns every course word in course order.
func (s *Store) CourseLexemes(ctx context.Context) ([]CourseLexeme, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT lexeme_id, course_order, list_rank, lemma FROM course_words
		ORDER BY course_order, lexeme_id`)
	if err != nil {
		return nil, fmt.Errorf("db: course lexemes: %w", err)
	}
	defer rows.Close()
	var out []CourseLexeme
	for rows.Next() {
		var row CourseLexeme
		if err := rows.Scan(&row.LexemeID, &row.Order, &row.Rank, &row.Lemma); err != nil {
			return nil, fmt.Errorf("db: course lexemes: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: course lexemes: %w", err)
	}
	return out, nil
}

// CourseCardLexemes reports which course words this learner has any card for.
func (s *Store) CourseCardLexemes(ctx context.Context, userID int64) (map[int64]bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT w.lexeme_id,
		       EXISTS (SELECT 1 FROM cards AS c WHERE c.user_id = ? AND c.lexeme_id = w.lexeme_id)
		FROM course_words AS w
		ORDER BY w.course_order`, userID)
	if err != nil {
		return nil, fmt.Errorf("db: course cards: %w", err)
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		var has bool
		if err := rows.Scan(&id, &has); err != nil {
			return nil, fmt.Errorf("db: course cards: %w", err)
		}
		out[id] = has
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: course cards: %w", err)
	}
	return out, nil
}

// LexemeIDsBySpelling maps every stored lexeme's consonant spelling to its id.
// More than one id under a spelling means the spelling is ambiguous.
func (s *Store) LexemeIDsBySpelling(ctx context.Context) (map[string][]int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `SELECT id, text_nfc, bare FROM lexemes`)
	if err != nil {
		return nil, fmt.Errorf("db: lexeme spellings: %w", err)
	}
	defer rows.Close()
	out := map[string][]int64{}
	for rows.Next() {
		var id int64
		var text, bare string
		if err := rows.Scan(&id, &text, &bare); err != nil {
			return nil, fmt.Errorf("db: lexeme spellings: %w", err)
		}
		spelling := bare
		if spelling == "" {
			spelling = units.StripNiqqud(text)
		}
		out[spelling] = append(out[spelling], id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: lexeme spellings: %w", err)
	}
	return out, nil
}

// LexemesBehindUnfinishedIntro returns lexemes that are still a new word of a
// published lesson this learner has not finished. Review hides those cards,
// so the course must not seed them.
func (s *Store) LexemesBehindUnfinishedIntro(ctx context.Context, userID int64) (map[int64]bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT DISTINCT li.lexeme_id
		FROM lesson_items AS li
		JOIN units AS u ON u.id = li.unit_id
		WHERE li.role = 'new'
		  AND u.status = 'published'
		  AND (u.scope = 'shared' OR u.scope = ?)
		  AND NOT EXISTS (
		    SELECT 1 FROM lesson_runs AS lr
		    WHERE lr.user_id = ?
		      AND lr.unit_id = li.unit_id
		      AND lr.lesson_ix = li.lesson_ix
		      AND lr.finished_at IS NOT NULL
		  )`, LearnerScope(userID), userID)
	if err != nil {
		return nil, fmt.Errorf("db: unfinished intro: %w", err)
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("db: unfinished intro: %w", err)
		}
		out[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: unfinished intro: %w", err)
	}
	return out, nil
}

// CourseCardFirstCreated is the earliest card created_at for each course word
// this learner has.
func (s *Store) CourseCardFirstCreated(ctx context.Context, userID int64) (map[int64]time.Time, error) {
	return s.courseCardCreated(ctx, userID)
}

// CourseCardsCreated counts course words whose first card was created in the window.
func (s *Store) CourseCardsCreated(ctx context.Context, userID int64, from, to time.Time) (int, error) {
	all, err := s.courseCardCreated(ctx, userID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, at := range all {
		if !at.Before(from) && at.Before(to) {
			n++
		}
	}
	return n, nil
}

func (s *Store) courseCardCreated(ctx context.Context, userID int64) (map[int64]time.Time, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT c.lexeme_id, MIN(c.created_at)
		FROM cards AS c
		JOIN course_words AS w ON w.lexeme_id = c.lexeme_id
		WHERE c.user_id = ? AND c.created_at IS NOT NULL
		GROUP BY c.lexeme_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("db: course card created: %w", err)
	}
	defer rows.Close()
	out := map[int64]time.Time{}
	for rows.Next() {
		var id int64
		var at string
		if err := rows.Scan(&id, &at); err != nil {
			return nil, fmt.Errorf("db: course card created: %w", err)
		}
		parsed, err := ParseTime(at)
		if err != nil {
			return nil, err
		}
		out[id] = parsed
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: course card created: %w", err)
	}
	return out, nil
}
