package mcpstore

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

// SearchLexemes matches pointed text, unpointed text, and the English gloss.
// A hit is an exact or prefix match on the pointed or unpointed form, or a
// gloss that equals the query or one semicolon-separated part of it.
// Suggestions are the nearest unpointed spellings, and only when hits is empty.
// limit caps each list. Gender, plural, and translit come from the lexicon
// columns added in migration 002. irregular_plural is on units.LexemeFacts
// because mcp.Lexeme has no field for it.
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
		SELECT l.text_nfc, l.bare, l.gloss_en, l.pos, l.gender, l.plural, l.translit, l.root, l.binyan,
		       CASE WHEN a.text_nfc IS NOT NULL THEN 1 ELSE 0 END
		FROM lexemes AS l
		LEFT JOIN (
			SELECT DISTINCT text_nfc FROM audio_assets
		) AS a ON a.text_nfc = l.text_nfc`)
	if err != nil {
		return nil, nil, fmt.Errorf("mcpstore: search: %w", err)
	}
	defer rows.Close()

	var hits, suggestions []mcp.Lexeme
	best := nearMissDistance + 1
	for rows.Next() {
		lx, storedBare, err := scanLexeme(rows)
		if err != nil {
			return nil, nil, err
		}
		storedPointed := normalize(lx.TextNFC)
		if storedBare == "" {
			storedBare = stripNiqqud(storedPointed)
		} else {
			storedBare = normalize(storedBare)
		}
		if lexemeHit(storedPointed, storedBare, lx.Gloss, pointed, bare, glossQ) {
			hits = append(hits, lx)
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
			suggestions = append(suggestions, lx)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("mcpstore: search: %w", err)
	}
	slices.SortFunc(hits, func(a, b mcp.Lexeme) int { return cmp.Compare(a.TextNFC, b.TextNFC) })
	slices.SortFunc(suggestions, func(a, b mcp.Lexeme) int { return cmp.Compare(a.TextNFC, b.TextNFC) })
	if len(hits) > 0 {
		suggestions = nil
	}
	return clipLexemes(hits, limit), clipLexemes(suggestions, limit), nil
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

func scanLexeme(rows rowScanner) (mcp.Lexeme, string, error) {
	var lx mcp.Lexeme
	var gender, plural, translit sql.NullString
	var audio int
	if err := rows.Scan(
		&lx.TextNFC, &lx.Bare, &lx.Gloss, &lx.POS, &gender, &plural,
		&translit, &lx.Root, &lx.Binyan, &audio,
	); err != nil {
		return mcp.Lexeme{}, "", fmt.Errorf("mcpstore: search: %w", err)
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
	return lx, lx.Bare, nil
}

func clipLexemes(in []mcp.Lexeme, n int) []mcp.Lexeme {
	if len(in) == 0 {
		return []mcp.Lexeme{}
	}
	if n > len(in) {
		n = len(in)
	}
	return append([]mcp.Lexeme(nil), in[:n]...)
}
