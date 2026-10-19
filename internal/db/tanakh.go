package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Verse is one tanakh_verses row.
// He is pointed NFC. HePlain is units.Plain of that Hebrew.
// License is Public Domain for every row the importer writes.
type Verse struct {
	Book          string
	Chapter       int
	Verse         int
	He            string
	HePlain       string
	En            string
	SourceVersion string
	License       string
}

const verseSelect = `book, chapter, verse, he, he_plain, en, source_version, license`

// VersesByRef returns the inclusive verse range of one chapter.
// book may be the canonical slug, the English title, or a common alias.
// An unknown book, a chapter below 1, or a reversed range is an error.
// A range with no stored verses returns an empty slice.
func (s *Store) VersesByRef(ctx context.Context, book string, chapter, fromVerse, toVerse int) ([]Verse, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	slug, ok := CanonicalBook(book)
	if !ok {
		return nil, fmt.Errorf("db: unknown tanakh book %q", book)
	}
	if chapter < 1 || fromVerse < 1 || toVerse < fromVerse {
		return nil, fmt.Errorf("db: tanakh range %s %d:%d-%d is invalid", slug, chapter, fromVerse, toVerse)
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT `+verseSelect+`
		FROM tanakh_verses
		WHERE book = ? AND chapter = ? AND verse >= ? AND verse <= ?
		ORDER BY verse`, slug, chapter, fromVerse, toVerse)
	if err != nil {
		return nil, fmt.Errorf("db: tanakh verses: %w", err)
	}
	defer rows.Close()
	return scanVerses(rows)
}

// VersesContaining returns verses whose he_plain contains lexemePlain as a
// whole token. Tokens split on whitespace, maqaf (U+05BE), and punctuation,
// using units.Tokenize and units.Plain. A lexeme that sits inside a longer
// word does not match. limit must be positive. Results are in canonical
// book order, then chapter, then verse, and are capped at limit.
// A blank lexeme, or one that is not a single token, returns no rows.
func (s *Store) VersesContaining(ctx context.Context, lexemePlain string, limit int) ([]Verse, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, errors.New("db: tanakh limit must be positive")
	}
	needle, ok := singleToken(lexemePlain)
	if !ok {
		return []Verse{}, nil
	}
	rows, err := conn.QueryContext(ctx, `SELECT `+verseSelect+` FROM tanakh_verses`)
	if err != nil {
		return nil, fmt.Errorf("db: tanakh search: %w", err)
	}
	defer rows.Close()
	all, err := scanVerses(rows)
	if err != nil {
		return nil, err
	}
	matched := make([]Verse, 0)
	for _, v := range all {
		if verseHasToken(v.HePlain, needle) {
			matched = append(matched, v)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		a, b := matched[i], matched[j]
		if ra, rb := tanakhRank(a.Book), tanakhRank(b.Book); ra != rb {
			return ra < rb
		}
		if a.Chapter != b.Chapter {
			return a.Chapter < b.Chapter
		}
		return a.Verse < b.Verse
	})
	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

func singleToken(lexeme string) (string, bool) {
	toks := plainTokens(lexeme)
	if len(toks) != 1 {
		return "", false
	}
	return toks[0], true
}

func verseHasToken(hePlain, needle string) bool {
	for _, tok := range plainTokens(hePlain) {
		if tok == needle {
			return true
		}
	}
	return false
}

// plainTokens is the consonant tokens of s. units.Tokenize splits on
// whitespace, maqaf, and punctuation; units.Plain strips niqqud and
// cantillation so a pointed query hits the stored skeleton.
func plainTokens(s string) []string {
	raw := units.Tokenize(s)
	out := make([]string, 0, len(raw))
	for _, tok := range raw {
		p := units.Plain(tok)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func scanVerses(rows *sql.Rows) ([]Verse, error) {
	var out []Verse
	for rows.Next() {
		var v Verse
		if err := rows.Scan(&v.Book, &v.Chapter, &v.Verse, &v.He, &v.HePlain, &v.En, &v.SourceVersion, &v.License); err != nil {
			return nil, fmt.Errorf("db: tanakh scan: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: tanakh scan: %w", err)
	}
	if out == nil {
		out = []Verse{}
	}
	return out, nil
}

// TanakhMeta returns one tanakh_meta value.
// ok is false when the key is not stored.
func (s *Store) TanakhMeta(ctx context.Context, key string) (value string, ok bool, err error) {
	conn, err := s.connOrErr()
	if err != nil {
		return "", false, err
	}
	err = conn.QueryRowContext(ctx, `SELECT value FROM tanakh_meta WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("db: tanakh meta: %w", err)
	}
	return value, true, nil
}

// CountTanakhVerses returns the number of stored verses.
func (s *Store) CountTanakhVerses(ctx context.Context) (int, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM tanakh_verses`).Scan(&n); err != nil {
		return 0, fmt.Errorf("db: count tanakh: %w", err)
	}
	return n, nil
}

// ReplaceTanakh upserts verses and replaces tanakh_meta in one transaction.
// A stored verse whose (book, chapter, verse) is not in verses is deleted.
// license must be Public Domain. The caller computes he and he_plain.
func (s *Store) ReplaceTanakh(ctx context.Context, verses []Verse, meta map[string]string) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	for _, v := range verses {
		if err := validateVerse(v); err != nil {
			return err
		}
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: tanakh: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS tanakh_import_keys`); err != nil {
		return fmt.Errorf("db: tanakh: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		CREATE TEMP TABLE tanakh_import_keys (
			book TEXT NOT NULL,
			chapter INTEGER NOT NULL,
			verse INTEGER NOT NULL,
			PRIMARY KEY (book, chapter, verse)
		)`); err != nil {
		return fmt.Errorf("db: tanakh: %w", err)
	}
	ins, err := tx.PrepareContext(ctx, `
		INSERT INTO tanakh_verses (
			book, chapter, verse, he, he_plain, en, source_version, license
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (book, chapter, verse) DO UPDATE SET
			he = excluded.he,
			he_plain = excluded.he_plain,
			en = excluded.en,
			source_version = excluded.source_version,
			license = excluded.license`)
	if err != nil {
		return fmt.Errorf("db: tanakh: %w", err)
	}
	defer ins.Close()
	keyIns, err := tx.PrepareContext(ctx, `
		INSERT INTO tanakh_import_keys (book, chapter, verse) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("db: tanakh: %w", err)
	}
	defer keyIns.Close()
	for _, v := range verses {
		if _, err := ins.ExecContext(ctx, v.Book, v.Chapter, v.Verse, v.He, v.HePlain, v.En, v.SourceVersion, v.License); err != nil {
			return fmt.Errorf("db: tanakh upsert %s %d:%d: %w", v.Book, v.Chapter, v.Verse, err)
		}
		if _, err := keyIns.ExecContext(ctx, v.Book, v.Chapter, v.Verse); err != nil {
			return fmt.Errorf("db: tanakh key %s %d:%d: %w", v.Book, v.Chapter, v.Verse, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM tanakh_verses
		WHERE NOT EXISTS (
			SELECT 1 FROM tanakh_import_keys k
			WHERE k.book = tanakh_verses.book
				AND k.chapter = tanakh_verses.chapter
				AND k.verse = tanakh_verses.verse
		)`); err != nil {
		return fmt.Errorf("db: tanakh delete: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM tanakh_meta`); err != nil {
		return fmt.Errorf("db: tanakh meta: %w", err)
	}
	metaIns, err := tx.PrepareContext(ctx, `INSERT INTO tanakh_meta (key, value) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("db: tanakh meta: %w", err)
	}
	defer metaIns.Close()
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, err := metaIns.ExecContext(ctx, k, meta[k]); err != nil {
			return fmt.Errorf("db: tanakh meta %s: %w", k, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE tanakh_import_keys`); err != nil {
		return fmt.Errorf("db: tanakh: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: tanakh: %w", err)
	}
	return nil
}

func validateVerse(v Verse) error {
	if _, ok := bookOrder[v.Book]; !ok {
		return fmt.Errorf("db: tanakh book %q is not a canonical slug", v.Book)
	}
	if v.Chapter < 1 || v.Verse < 1 {
		return fmt.Errorf("db: tanakh %s %d:%d is invalid", v.Book, v.Chapter, v.Verse)
	}
	if v.He == "" || v.HePlain == "" || v.En == "" || v.SourceVersion == "" {
		return fmt.Errorf("db: tanakh %s %d:%d is missing text", v.Book, v.Chapter, v.Verse)
	}
	if v.License != "Public Domain" {
		return fmt.Errorf("db: tanakh %s %d:%d license %q is not Public Domain", v.Book, v.Chapter, v.Verse, v.License)
	}
	return nil
}
