package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/richtext"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

const verseNoteMaxBytes = 8000

// Verse layers. gloss is a short note; the word-by-word glosses live in
// verse_word_glosses. The other three are the reader sections.
const (
	LayerGloss  = "gloss"
	LayerRoots  = "roots"
	LayerManner = "manner"
	LayerBridge = "bridge"
)

// MissingVerseError means the book, chapter, and verse are not in tanakh_verses.
// The write is refused.
type MissingVerseError struct {
	Book    string
	Chapter int
	Verse   int
}

func (e MissingVerseError) Error() string {
	return fmt.Sprintf("db: %s %d:%d is not in the Tanakh", e.Book, e.Chapter, e.Verse)
}

// VerseNoteInput is one layer. Status is ignored: a write is always a draft.
type VerseNoteInput struct {
	Layer     string
	Body      string
	DraftedBy int64
}

// VerseNote is one stored layer.
type VerseNote struct {
	Book      string
	Chapter   int
	Verse     int
	Layer     string
	Body      string
	Status    string
	DraftedBy int64
	CheckedBy int64
	UpdatedAt string
}

// VerseGlossInput is one word. Status is ignored: a write is always a draft.
type VerseGlossInput struct {
	Position int
	WordHe   string
	GlossEn  string
	Root     string
	LexemeID int64
}

// VerseGloss is one stored word gloss.
type VerseGloss struct {
	Book     string
	Chapter  int
	Verse    int
	Position int
	WordHe   string
	GlossEn  string
	Root     string
	LexemeID int64
	Status   string
}

// TanakhVerse is the pointed text and the English of one verse.
type TanakhVerse struct {
	Book    string
	Chapter int
	Verse   int
	He      string
	En      string
}

// VerseRef is a chapter and verse in one book.
type VerseRef struct {
	Chapter int
	Verse   int
}

// UpsertVerseNotes writes layers for one verse. Each row is stored as draft.
// A checked row whose body is written again goes back to draft, and checked_by
// is cleared. A verse that is not in tanakh_verses writes nothing.
func (s *Store) UpsertVerseNotes(ctx context.Context, book string, chapter, verse int, notes []VerseNoteInput) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if err := validateNotes(notes); err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin verse notes: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := requireVerse(ctx, tx, book, chapter, verse); err != nil {
		return err
	}
	stamp := FormatTime(s.clock.Now())
	for _, note := range notes {
		var drafted any
		if note.DraftedBy > 0 {
			drafted = note.DraftedBy
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO verse_notes (book, chapter, verse, layer, body, status, drafted_by, checked_by, updated_at)
			VALUES (?, ?, ?, ?, ?, 'draft', ?, NULL, ?)
			ON CONFLICT (book, chapter, verse, layer) DO UPDATE SET
				body = excluded.body,
				status = 'draft',
				drafted_by = excluded.drafted_by,
				checked_by = NULL,
				updated_at = excluded.updated_at`,
			book, chapter, verse, note.Layer, note.Body, drafted, stamp); err != nil {
			return fmt.Errorf("db: verse notes: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit verse notes: %w", err)
	}
	return nil
}

// CheckVerseNote marks one layer checked and records the person who checked it.
func (s *Store) CheckVerseNote(ctx context.Context, book string, chapter, verse int, layer string, checkerID int64) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if checkerID <= 0 {
		return errors.New("db: a verse note needs the person who checked it")
	}
	if !validLayer(layer) {
		return errors.New("db: a verse note layer is gloss, roots, manner, or bridge")
	}
	res, err := conn.ExecContext(ctx, `
		UPDATE verse_notes
		SET status = 'checked', checked_by = ?, updated_at = ?
		WHERE book = ? AND chapter = ? AND verse = ? AND layer = ?`,
		checkerID, FormatTime(s.clock.Now()), book, chapter, verse, layer)
	if err != nil {
		return fmt.Errorf("db: check verse note: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: check verse note: %w", err)
	}
	if n != 1 {
		return errors.New("db: that verse note is not there")
	}
	return nil
}

// VerseNotes reads one verse. checkedOnly leaves drafts out.
func (s *Store) VerseNotes(ctx context.Context, book string, chapter, verse int, checkedOnly bool) ([]VerseNote, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	q := `SELECT book, chapter, verse, layer, body, status, drafted_by, checked_by, updated_at
		FROM verse_notes WHERE book = ? AND chapter = ? AND verse = ?`
	if checkedOnly {
		q += ` AND status = 'checked'`
	}
	q += ` ORDER BY CASE layer WHEN 'gloss' THEN 0 WHEN 'roots' THEN 1 WHEN 'manner' THEN 2 ELSE 3 END`
	rows, err := conn.QueryContext(ctx, q, book, chapter, verse)
	if err != nil {
		return nil, fmt.Errorf("db: verse notes: %w", err)
	}
	defer rows.Close()
	var out []VerseNote
	for rows.Next() {
		var row VerseNote
		var drafted, checked sql.NullInt64
		if err := rows.Scan(&row.Book, &row.Chapter, &row.Verse, &row.Layer, &row.Body, &row.Status, &drafted, &checked, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("db: verse notes: %w", err)
		}
		row.DraftedBy = drafted.Int64
		row.CheckedBy = checked.Int64
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: verse notes: %w", err)
	}
	if out == nil {
		out = []VerseNote{}
	}
	return out, nil
}

// UpsertVerseGlosses writes word glosses for one verse. Each row is a draft.
// A verse that is not in tanakh_verses writes nothing.
func (s *Store) UpsertVerseGlosses(ctx context.Context, book string, chapter, verse int, glosses []VerseGlossInput) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	cleaned, err := validateGlosses(glosses)
	if err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin verse glosses: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := requireVerse(ctx, tx, book, chapter, verse); err != nil {
		return err
	}
	for _, g := range cleaned {
		if g.LexemeID > 0 {
			ok, err := lexemeExists(ctx, tx, g.LexemeID)
			if err != nil {
				return fmt.Errorf("db: verse glosses: %w", err)
			}
			if !ok {
				return fmt.Errorf("db: lexeme %d is not in the lexicon", g.LexemeID)
			}
		}
		var root, lex any
		if g.Root != "" {
			root = g.Root
		}
		if g.LexemeID > 0 {
			lex = g.LexemeID
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO verse_word_glosses (book, chapter, verse, position, word_he, gloss_en, root, lexeme_id, status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'draft')
			ON CONFLICT (book, chapter, verse, position) DO UPDATE SET
				word_he = excluded.word_he,
				gloss_en = excluded.gloss_en,
				root = excluded.root,
				lexeme_id = excluded.lexeme_id,
				status = 'draft'`,
			book, chapter, verse, g.Position, g.WordHe, g.GlossEn, root, lex); err != nil {
			return fmt.Errorf("db: verse glosses: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit verse glosses: %w", err)
	}
	return nil
}

// CheckVerseGloss marks one word gloss checked. The person is recorded on the
// verse note, which is the row that has checked_by.
func (s *Store) CheckVerseGloss(ctx context.Context, book string, chapter, verse, position int) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	res, err := conn.ExecContext(ctx, `
		UPDATE verse_word_glosses SET status = 'checked'
		WHERE book = ? AND chapter = ? AND verse = ? AND position = ?`,
		book, chapter, verse, position)
	if err != nil {
		return fmt.Errorf("db: check verse gloss: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: check verse gloss: %w", err)
	}
	if n != 1 {
		return errors.New("db: that word gloss is not there")
	}
	return nil
}

// VerseGlosses reads word glosses for one verse. checkedOnly leaves drafts out.
func (s *Store) VerseGlosses(ctx context.Context, book string, chapter, verse int, checkedOnly bool) ([]VerseGloss, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	q := `SELECT book, chapter, verse, position, word_he, gloss_en, root, lexeme_id, status
		FROM verse_word_glosses WHERE book = ? AND chapter = ? AND verse = ?`
	if checkedOnly {
		q += ` AND status = 'checked'`
	}
	q += ` ORDER BY position`
	rows, err := conn.QueryContext(ctx, q, book, chapter, verse)
	if err != nil {
		return nil, fmt.Errorf("db: verse glosses: %w", err)
	}
	defer rows.Close()
	var out []VerseGloss
	for rows.Next() {
		var row VerseGloss
		var root sql.NullString
		var lex sql.NullInt64
		if err := rows.Scan(&row.Book, &row.Chapter, &row.Verse, &row.Position, &row.WordHe, &row.GlossEn, &root, &lex, &row.Status); err != nil {
			return nil, fmt.Errorf("db: verse glosses: %w", err)
		}
		row.Root = root.String
		row.LexemeID = lex.Int64
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: verse glosses: %w", err)
	}
	if out == nil {
		out = []VerseGloss{}
	}
	return out, nil
}

// TanakhVerse reads one verse. ok is false when it is not stored.
func (s *Store) TanakhVerse(ctx context.Context, book string, chapter, verse int) (TanakhVerse, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return TanakhVerse{}, false, err
	}
	var row TanakhVerse
	err = conn.QueryRowContext(ctx, `
		SELECT book, chapter, verse, he, en FROM tanakh_verses
		WHERE book = ? AND chapter = ? AND verse = ?`, book, chapter, verse).Scan(
		&row.Book, &row.Chapter, &row.Verse, &row.He, &row.En)
	if errors.Is(err, sql.ErrNoRows) {
		return TanakhVerse{}, false, nil
	}
	if err != nil {
		return TanakhVerse{}, false, fmt.Errorf("db: tanakh verse: %w", err)
	}
	return row, true, nil
}

// IsaiahChapters lists chapters of Isaiah that have a checked note or gloss.
func (s *Store) IsaiahChapters(ctx context.Context) ([]int, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT chapter FROM (
			SELECT chapter FROM verse_notes WHERE book = 'isaiah' AND status = 'checked'
			UNION
			SELECT chapter FROM verse_word_glosses WHERE book = 'isaiah' AND status = 'checked'
		) ORDER BY chapter`)
	if err != nil {
		return nil, fmt.Errorf("db: isaiah chapters: %w", err)
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("db: isaiah chapters: %w", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: isaiah chapters: %w", err)
	}
	if out == nil {
		out = []int{}
	}
	return out, nil
}

// IsaiahCheckedVerse is the first checked verse in a chapter, for the chapter list.
func (s *Store) IsaiahCheckedVerse(ctx context.Context, chapter int) (VerseRef, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return VerseRef{}, false, err
	}
	var ref VerseRef
	err = conn.QueryRowContext(ctx, `
		SELECT chapter, verse FROM (
			SELECT chapter, verse FROM verse_notes WHERE book = 'isaiah' AND chapter = ? AND status = 'checked'
			UNION
			SELECT chapter, verse FROM verse_word_glosses WHERE book = 'isaiah' AND chapter = ? AND status = 'checked'
		) ORDER BY verse LIMIT 1`, chapter, chapter).Scan(&ref.Chapter, &ref.Verse)
	if errors.Is(err, sql.ErrNoRows) {
		return VerseRef{}, false, nil
	}
	if err != nil {
		return VerseRef{}, false, fmt.Errorf("db: isaiah chapter: %w", err)
	}
	return ref, true, nil
}

// IsaiahNeighbors is the previous and next verse in the book, when each exists.
func (s *Store) IsaiahNeighbors(ctx context.Context, chapter, verse int) (prev, next VerseRef, hasPrev, hasNext bool, err error) {
	conn, err := s.connOrErr()
	if err != nil {
		return VerseRef{}, VerseRef{}, false, false, err
	}
	err = conn.QueryRowContext(ctx, `
		SELECT chapter, verse FROM tanakh_verses
		WHERE book = 'isaiah' AND (chapter < ? OR (chapter = ? AND verse < ?))
		ORDER BY chapter DESC, verse DESC LIMIT 1`, chapter, chapter, verse).Scan(&prev.Chapter, &prev.Verse)
	if err == nil {
		hasPrev = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		return VerseRef{}, VerseRef{}, false, false, fmt.Errorf("db: isaiah neighbors: %w", err)
	}
	err = conn.QueryRowContext(ctx, `
		SELECT chapter, verse FROM tanakh_verses
		WHERE book = 'isaiah' AND (chapter > ? OR (chapter = ? AND verse > ?))
		ORDER BY chapter, verse LIMIT 1`, chapter, chapter, verse).Scan(&next.Chapter, &next.Verse)
	if err == nil {
		hasNext = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		return VerseRef{}, VerseRef{}, false, false, fmt.Errorf("db: isaiah neighbors: %w", err)
	}
	return prev, next, hasPrev, hasNext, nil
}

func requireVerse(ctx context.Context, tx *sql.Tx, book string, chapter, verse int) error {
	var n int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tanakh_verses WHERE book = ? AND chapter = ? AND verse = ?`,
		book, chapter, verse).Scan(&n); err != nil {
		return fmt.Errorf("db: tanakh verse: %w", err)
	}
	if n != 1 {
		return MissingVerseError{Book: book, Chapter: chapter, Verse: verse}
	}
	return nil
}

func validateNotes(notes []VerseNoteInput) error {
	if len(notes) == 0 || len(notes) > 4 {
		return errors.New("db: send 1 to 4 verse notes")
	}
	seen := map[string]bool{}
	for _, note := range notes {
		if !validLayer(note.Layer) {
			return errors.New("db: a verse note layer is gloss, roots, manner, or bridge")
		}
		if seen[note.Layer] {
			return errors.New("db: a verse has one note per layer")
		}
		seen[note.Layer] = true
		if !utf8.ValidString(note.Body) || len(note.Body) > verseNoteMaxBytes {
			return errors.New("db: a verse note is at most 8000 bytes")
		}
		if _, err := richtext.Parse(note.Body); err != nil {
			return fmt.Errorf("db: verse note: %w", err)
		}
	}
	return nil
}

func validateGlosses(glosses []VerseGlossInput) ([]VerseGlossInput, error) {
	if len(glosses) == 0 || len(glosses) > 40 {
		return nil, errors.New("db: send 1 to 40 word glosses")
	}
	out := make([]VerseGlossInput, len(glosses))
	seen := map[int]bool{}
	for i, g := range glosses {
		word := units.Normalize(g.WordHe)
		root := ""
		if strings.TrimSpace(g.Root) != "" {
			root = units.Normalize(g.Root)
		}
		gloss := strings.TrimSpace(g.GlossEn)
		if word == "" || utf8.RuneCountInString(word) > 80 {
			return nil, errors.New("db: a word gloss needs the Hebrew word")
		}
		if gloss == "" || utf8.RuneCountInString(gloss) > 200 {
			return nil, errors.New("db: a word gloss needs an English gloss of at most 200 characters")
		}
		if root != "" && utf8.RuneCountInString(root) > 40 {
			return nil, errors.New("db: a root is at most 40 characters")
		}
		if g.Position < 0 || g.Position > 80 {
			return nil, errors.New("db: a word position is from 0 to 80")
		}
		if seen[g.Position] {
			return nil, errors.New("db: a verse has one gloss per position")
		}
		seen[g.Position] = true
		out[i] = VerseGlossInput{Position: g.Position, WordHe: word, GlossEn: gloss, Root: root, LexemeID: g.LexemeID}
	}
	return out, nil
}

func validLayer(layer string) bool {
	switch layer {
	case LayerGloss, LayerRoots, LayerManner, LayerBridge:
		return true
	default:
		return false
	}
}
