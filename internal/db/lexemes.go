package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// LexemeResult is one upsert_lexemes row.
// Status is lexicon.StatusCreated, StatusUpdated, or StatusUnchanged.
// TextNFC is the normalized key, not the raw argument.
type LexemeResult struct {
	TextNFC string
	ID      int64
	Status  string
}

// UpsertLexemes inserts or updates a batch in one transaction.
// The unique key is lexemes.text_nfc. An identical row is left in place
// and reported unchanged. Any invalid entry returns *lexicon.Invalid
// and writes nothing. After the rows are applied, lesson_items are
// rebuilt for every unit whose document references a batch entry by
// pointed text or by its bare form. Those unit ids are returned sorted.
// Rebuilding does not add a unit revision.
func (s *Store) UpsertLexemes(ctx context.Context, entries []lexicon.Input) ([]LexemeResult, []int64, error) {
	rows, bad := lexicon.Prepare(entries)
	if bad != nil {
		return nil, nil, bad
	}
	conn, err := s.connOrErr()
	if err != nil {
		return nil, nil, err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("db: begin lexemes: %w", err)
	}
	defer tx.Rollback()

	out := make([]LexemeResult, 0, len(rows))
	keys := make(map[string]bool, len(rows)*2)
	for _, row := range rows {
		saved, err := upsertLexeme(ctx, tx, row)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, saved)
		keys[row.TextNFC] = true
		if row.Bare != "" {
			keys[row.Bare] = true
		}
	}
	rebuilt, err := rebuildReferencedUnits(ctx, tx, keys)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("db: commit lexemes: %w", err)
	}
	return out, rebuilt, nil
}

type storedLexeme struct {
	id       int64
	bare     string
	gloss    string
	pos      string
	gender   string
	plural   string
	translit string
	verified bool
}

func upsertLexeme(ctx context.Context, tx *sql.Tx, row lexicon.Row) (LexemeResult, error) {
	cur, ok, err := readLexeme(ctx, tx, row.TextNFC)
	if err != nil {
		return LexemeResult{}, err
	}
	if !ok {
		id, err := insertLexemeRow(ctx, tx, row)
		if err != nil {
			return LexemeResult{}, err
		}
		return LexemeResult{TextNFC: row.TextNFC, ID: id, Status: lexicon.StatusCreated}, nil
	}
	if lexemeUnchanged(cur, row) {
		return LexemeResult{TextNFC: row.TextNFC, ID: cur.id, Status: lexicon.StatusUnchanged}, nil
	}
	if err := updateLexeme(ctx, tx, cur, row); err != nil {
		return LexemeResult{}, err
	}
	return LexemeResult{TextNFC: row.TextNFC, ID: cur.id, Status: lexicon.StatusUpdated}, nil
}

func readLexeme(ctx context.Context, tx *sql.Tx, text string) (storedLexeme, bool, error) {
	var row storedLexeme
	var gender, plural, translit sql.NullString
	var verified int
	err := tx.QueryRowContext(ctx, `
		SELECT id, bare, gloss_en, pos, gender, plural, translit, verified
		FROM lexemes WHERE text_nfc = ?`, text,
	).Scan(&row.id, &row.bare, &row.gloss, &row.pos, &gender, &plural, &translit, &verified)
	if errors.Is(err, sql.ErrNoRows) {
		return storedLexeme{}, false, nil
	}
	if err != nil {
		return storedLexeme{}, false, fmt.Errorf("db: read lexeme: %w", err)
	}
	row.gender = gender.String
	row.plural = plural.String
	row.translit = translit.String
	row.verified = verified != 0
	return row, true, nil
}

func lexemeUnchanged(cur storedLexeme, row lexicon.Row) bool {
	return cur == mergedLexeme(cur, row)
}

// mergedLexeme applies an update. Omitted pos, gender, plural, translit,
// and verified keep the stored column. A set pointer replaces it.
func mergedLexeme(cur storedLexeme, row lexicon.Row) storedLexeme {
	out := cur
	out.bare = row.Bare
	out.gloss = row.Gloss
	if row.POS != nil {
		out.pos = *row.POS
	}
	if row.Gender != nil {
		out.gender = *row.Gender
	}
	if row.Plural != nil {
		out.plural = *row.Plural
	}
	if row.Translit != nil {
		out.translit = *row.Translit
	}
	if row.Verified != nil {
		out.verified = *row.Verified
	}
	return out
}

func insertLexemeRow(ctx context.Context, tx *sql.Tx, row lexicon.Row) (int64, error) {
	res, err := tx.ExecContext(ctx, `
		INSERT INTO lexemes (text_nfc, bare, gloss_en, pos, gender, plural, translit, verified)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		row.TextNFC, row.Bare, row.Gloss, strOrEmpty(row.POS),
		nullIfEmpty(strOrEmpty(row.Gender)), nullIfEmpty(strOrEmpty(row.Plural)), nullIfEmpty(strOrEmpty(row.Translit)), verifiedBit(row.Verified),
	)
	if err != nil {
		return 0, fmt.Errorf("db: insert lexeme: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("db: insert lexeme: %w", err)
	}
	return id, nil
}

func updateLexeme(ctx context.Context, tx *sql.Tx, cur storedLexeme, row lexicon.Row) error {
	next := mergedLexeme(cur, row)
	_, err := tx.ExecContext(ctx, `
		UPDATE lexemes
		SET bare = ?, gloss_en = ?, pos = ?, gender = ?, plural = ?, translit = ?, verified = ?
		WHERE id = ?`,
		next.bare, next.gloss, next.pos,
		nullIfEmpty(next.gender), nullIfEmpty(next.plural), nullIfEmpty(next.translit), verifiedBitBool(next.verified), next.id,
	)
	if err != nil {
		return fmt.Errorf("db: update lexeme: %w", err)
	}
	return nil
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func verifiedBitBool(v bool) int {
	if v {
		return 1
	}
	return 0
}

func verifiedBit(v *bool) int {
	if v != nil && *v {
		return 1
	}
	return 0
}

// rebuildReferencedUnits replaces lesson_items for every unit whose
// new words or activity tokens match a batch key. Keys are the normalized
// text_nfc and the bare form. Tokenizing is lessonTokens, the same split
// replaceLessonItems uses to resolve a word. The lookup inside that
// rebuild is still the exact text_nfc.
func rebuildReferencedUnits(ctx context.Context, tx *sql.Tx, keys map[string]bool) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, doc FROM units ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("db: list units: %w", err)
	}
	defer rows.Close()
	var ids []int64
	var docs []units.Unit
	for rows.Next() {
		var id int64
		var raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, fmt.Errorf("db: list units: %w", err)
		}
		var doc units.Unit
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			return nil, fmt.Errorf("db: unit %d doc: %w", id, err)
		}
		if !docReferences(doc, keys) {
			continue
		}
		ids = append(ids, id)
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list units: %w", err)
	}
	for i, id := range ids {
		if err := replaceLessonItems(ctx, tx, id, docs[i]); err != nil {
			return nil, err
		}
	}
	if ids == nil {
		ids = []int64{}
	}
	slices.Sort(ids)
	return ids, nil
}

// docReferences reports whether doc mentions any key. Activity text is
// split with lessonTokens. newWords is normalized the same way. A match
// on the pointed text or on its consonant skeleton counts.
func docReferences(doc units.Unit, keys map[string]bool) bool {
	for _, word := range doc.NewWords {
		n := units.Normalize(word)
		if n == "" {
			continue
		}
		if keys[n] || keys[units.StripNiqqud(n)] {
			return true
		}
	}
	for _, lesson := range doc.Lessons {
		for _, act := range lesson.Activities {
			for _, tok := range lessonTokens(act.Text) {
				if keys[tok] || keys[units.StripNiqqud(tok)] {
					return true
				}
			}
		}
	}
	return false
}
