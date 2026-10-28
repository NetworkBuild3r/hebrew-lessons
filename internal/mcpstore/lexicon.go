package mcpstore

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

// SearchLexemes matches pointed text, unpointed text, and the English gloss.
// A hit is an exact or prefix match on the pointed or unpointed form, or a
// gloss that equals the query or one semicolon-separated part of it.
// Suggestions are the nearest unpointed spellings, and only when hits is empty.
// limit caps each list. Gender, plural, and translit come from the lexicon
// columns added in migration 002. irregular_plural is on units.LexemeFacts
// because mcp.Lexeme has no field for it. Forms are attached for verbs,
// nouns, and adjectives (INIT-023/SPEC-006).
func (s *Store) SearchLexemes(ctx context.Context, query string, limit int) ([]mcp.Lexeme, []mcp.Lexeme, error) {
	conn, err := s.conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	pointed := normalize(query)
	bare := stripNiqqud(pointed)
	glossQ := strings.ToLower(pointed)
	// Clip texts are joined once. audio_assets has no index on text_nfc,
	// voice, or model; adding one needs a migration.
	rows, err := conn.QueryContext(ctx, `
		SELECT l.id, l.text_nfc, l.bare, l.gloss_en, l.pos, l.gender, l.plural, l.translit, l.root, l.binyan,
		       CASE WHEN a.text_nfc IS NOT NULL THEN 1 ELSE 0 END
		FROM lexemes AS l
		LEFT JOIN (
			SELECT DISTINCT text_nfc FROM audio_assets
		) AS a ON a.text_nfc = l.text_nfc`)
	if err != nil {
		return nil, nil, fmt.Errorf("mcpstore: search: %w", err)
	}
	defer rows.Close()

	var hits, suggestions []rankedLexeme
	best := nearMissDistance + 1
	for rows.Next() {
		lx, id, storedBare, err := scanLexeme(rows)
		if err != nil {
			return nil, nil, err
		}
		storedPointed := normalize(lx.TextNFC)
		if storedBare == "" {
			storedBare = stripNiqqud(storedPointed)
		} else {
			storedBare = normalize(storedBare)
		}
		row := rankedLexeme{lx: lx, id: id, pos: lx.POS}
		if lexemeHit(storedPointed, storedBare, lx.Gloss, pointed, bare, glossQ) {
			hits = append(hits, row)
			continue
		}
		if bare == "" || storedBare == "" {
			continue
		}
		dist := levenshtein(bare, storedBare)
		if dist == 0 || dist > nearMissDistance {
			continue
		}
		if dist < best {
			best = dist
			suggestions = suggestions[:0]
		}
		if dist == best {
			suggestions = append(suggestions, row)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("mcpstore: search: %w", err)
	}
	slices.SortFunc(hits, func(a, b rankedLexeme) int { return cmp.Compare(a.lx.TextNFC, b.lx.TextNFC) })
	slices.SortFunc(suggestions, func(a, b rankedLexeme) int { return cmp.Compare(a.lx.TextNFC, b.lx.TextNFC) })
	if len(hits) > 0 {
		suggestions = nil
	}
	outHits, err := s.attachForms(ctx, clipRanked(hits, limit))
	if err != nil {
		return nil, nil, err
	}
	outSug, err := s.attachForms(ctx, clipRanked(suggestions, limit))
	if err != nil {
		return nil, nil, err
	}
	return outHits, outSug, nil
}

// attachForms fills forms for verbs, nouns, and adjectives. INIT-023/SPEC-006
func (s *Store) attachForms(ctx context.Context, rows []rankedLexeme) ([]mcp.Lexeme, error) {
	out := make([]mcp.Lexeme, 0, len(rows))
	for _, row := range rows {
		lx := row.lx
		switch row.pos {
		case "verb":
			forms, err := s.db.VerbFormsFor(ctx, row.id)
			if err != nil {
				return nil, err
			}
			lx.Forms = verbWordForms(forms)
		case "noun", "adjective", "n", "adj":
			forms, err := s.db.NounFormsFor(ctx, row.id)
			if err != nil {
				return nil, err
			}
			lx.Forms = nounWordForms(forms)
		}
		out = append(out, lx)
	}
	if out == nil {
		out = []mcp.Lexeme{}
	}
	return out, nil
}

type rankedLexeme struct {
	lx  mcp.Lexeme
	id  int64
	pos string
}

func clipRanked(in []rankedLexeme, n int) []rankedLexeme {
	if len(in) == 0 {
		return nil
	}
	if n > len(in) {
		n = len(in)
	}
	return append([]rankedLexeme(nil), in[:n]...)
}

func verbWordForms(rows []db.VerbForm) []mcp.WordForm {
	if len(rows) == 0 {
		return nil
	}
	out := make([]mcp.WordForm, len(rows))
	for i, row := range rows {
		out[i] = mcp.WordForm{
			Tense: row.Tense, Person: row.Person, Number: row.Number, Gender: row.Gender,
			He: row.FormHe, Translit: row.Translit, HasAudio: row.AudioID != "",
		}
	}
	return out
}

func nounWordForms(rows []db.NounForm) []mcp.WordForm {
	if len(rows) == 0 {
		return nil
	}
	out := make([]mcp.WordForm, len(rows))
	for i, row := range rows {
		out[i] = mcp.WordForm{
			Kind: row.Kind, Number: row.Number, Person: row.Person, Gender: row.Gender,
			OwnerNumber: row.OwnerNumber, He: row.FormHe, Translit: row.Translit, HasAudio: row.AudioID != "",
		}
	}
	return out
}

func lexemeHit(pointed, bare, gloss, qPointed, qBare, qGloss string) bool {
	if qPointed != "" && (pointed == qPointed || strings.HasPrefix(pointed, qPointed)) {
		return true
	}
	if qBare != "" && (bare == qBare || strings.HasPrefix(bare, qBare)) {
		return true
	}
	return glossEquals(gloss, qGloss)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLexeme(rows rowScanner) (mcp.Lexeme, int64, string, error) {
	var lx mcp.Lexeme
	var id int64
	var gender, plural, translit sql.NullString
	var audio int
	if err := rows.Scan(
		&id, &lx.TextNFC, &lx.Bare, &lx.Gloss, &lx.POS, &gender, &plural,
		&translit, &lx.Root, &lx.Binyan, &audio,
	); err != nil {
		return mcp.Lexeme{}, 0, "", fmt.Errorf("mcpstore: search: %w", err)
	}
	if gender.Valid {
		lx.Gender = gender.String
	}
	if plural.Valid {
		lx.Plural = plural.String
	}
	if translit.Valid {
		lx.Translit = translit.String
	}
	lx.AudioPresent = audio != 0
	return lx, id, lx.Bare, nil
}
