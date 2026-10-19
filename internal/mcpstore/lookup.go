package mcpstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// KnownLexemes is the shared-unit authoring view: pointed NFC text of
// newWords on shared units whose position is lower than beforePosition.
// units_scope_position covers the scope and position filter.
func (s *Store) KnownLexemes(ctx context.Context, beforePosition int) (map[string]bool, error) {
	conn, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT doc FROM units
		WHERE position < ? AND scope = 'shared'`, beforePosition)
	if err != nil {
		return nil, fmt.Errorf("mcpstore: known lexemes: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var doc string
		if err := rows.Scan(&doc); err != nil {
			return nil, fmt.Errorf("mcpstore: known lexemes: %w", err)
		}
		var body struct {
			NewWords []string `json:"newWords"`
		}
		if err := json.Unmarshal([]byte(doc), &body); err != nil {
			return nil, fmt.Errorf("mcpstore: known lexemes: %w", err)
		}
		for _, word := range body.NewWords {
			n := normalize(word)
			if n != "" {
				out[n] = true
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mcpstore: known lexemes: %w", err)
	}
	return out, nil
}

// HasAudio reports a clip for this exact text.
// db.Store.AudioIDByText filters audio_assets on text_nfc only.
// text_nfc, voice, and model have no index. Adding one needs a migration.
func (s *Store) HasAudio(ctx context.Context, textNFC string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if s == nil || s.db == nil {
		return false, errors.New("mcpstore: database is not open")
	}
	_, ok, err := s.db.AudioIDByText(ctx, textNFC)
	return ok, err
}

// LexemeByText returns the lexicon id for the exact pointed text.
func (s *Store) LexemeByText(ctx context.Context, textNFC string) (int64, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	if s == nil || s.db == nil {
		return 0, false, errors.New("mcpstore: database is not open")
	}
	id, _, _, _, ok, err := s.db.LexemeByText(ctx, textNFC)
	return id, ok, err
}

// Facts returns the lexicon fields for the exact pointed text.
// Gender, plural, and irregular_plural are the migration 002 columns.
func (s *Store) Facts(ctx context.Context, textNFC string) (units.LexemeFacts, bool, error) {
	conn, err := s.conn(ctx)
	if err != nil {
		return units.LexemeFacts{}, false, err
	}
	var facts units.LexemeFacts
	var gender, plural sql.NullString
	var irregular int
	err = conn.QueryRowContext(ctx, `
		SELECT pos, gender, plural, irregular_plural, bare
		FROM lexemes WHERE text_nfc = ?`, textNFC,
	).Scan(&facts.POS, &gender, &plural, &irregular, &facts.Bare)
	if errors.Is(err, sql.ErrNoRows) {
		return units.LexemeFacts{}, false, nil
	}
	if err != nil {
		return units.LexemeFacts{}, false, fmt.Errorf("mcpstore: facts: %w", err)
	}
	if gender.Valid {
		facts.Gender = gender.String
	}
	if plural.Valid {
		facts.Plural = plural.String
	}
	facts.IrregularPlural = irregular != 0
	return facts, true, nil
}
