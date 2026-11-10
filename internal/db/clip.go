package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
)

// errListenNeedsClip is returned when a path tries to insert a listen card
// for a word that has no clip. INIT-038/SPEC-003.
var errListenNeedsClip = fmt.Errorf("db: listen card needs a clip")

// sqlIdent is a plain SQL identifier. Clip SQL interpolates only aliases
// that match it; values stay bound parameters.
var sqlIdent = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// LexemeHasClipSQL is the one clip lookup (INIT-038/SPEC-003).
// A word has a clip when lexAlias.audio_id is set or an audio_assets row
// exists for its exact pointed text. lexAlias is the lexemes table alias
// in the caller's query.
func LexemeHasClipSQL(lexAlias string) string {
	if !sqlIdent.MatchString(lexAlias) {
		panic("db: lexeme alias")
	}
	return `((` + lexAlias + `.audio_id IS NOT NULL AND ` + lexAlias + `.audio_id != '')
		OR EXISTS (
			SELECT 1 FROM audio_assets AS clip_row
			WHERE clip_row.text_nfc = ` + lexAlias + `.text_nfc
		))`
}

// lexemeClipIDSQL is the clip id the one lookup resolves (INIT-038/SPEC-003).
// It is lexemes.audio_id when that is set, otherwise the audio_assets id
// AudioByText would play for the exact pointed text, otherwise empty.
func lexemeClipIDSQL(lexAlias string) string {
	if !sqlIdent.MatchString(lexAlias) {
		panic("db: lexeme alias")
	}
	return `COALESCE(NULLIF(` + lexAlias + `.audio_id, ''), (
		SELECT clip_row.id FROM audio_assets AS clip_row
		WHERE clip_row.text_nfc = ` + lexAlias + `.text_nfc
		ORDER BY CASE clip_row.model
			WHEN 'eleven_v4' THEN 0
			WHEN 'eleven_v3' THEN 1
			WHEN 'legacy' THEN 2
			ELSE 3
		END, clip_row.created_at DESC, clip_row.id ASC
		LIMIT 1
	), '')`
}

// listenCardHasClipSQL keeps a listen card only when its word has a clip
// (INIT-038/SPEC-003). Other skills are unchanged. cardAlias is the cards
// table alias.
func listenCardHasClipSQL(cardAlias string) string {
	if !sqlIdent.MatchString(cardAlias) {
		panic("db: card alias")
	}
	return `(` + cardAlias + `.skill != 'listen'
		OR EXISTS (
			SELECT 1 FROM lexemes AS l
			WHERE l.id = ` + cardAlias + `.lexeme_id
			  AND ` + LexemeHasClipSQL("l") + `
		))`
}

// queryLexemeClip reports whether the lexeme row exists and whether the one
// lookup says it has a clip.
func queryLexemeClip(ctx context.Context, q rowQuery, lexemeID int64) (exists bool, has bool, err error) {
	var found int
	var clips sql.NullInt64
	err = q.QueryRowContext(ctx, `
		SELECT COUNT(*),
			SUM(CASE WHEN `+LexemeHasClipSQL("l")+` THEN 1 ELSE 0 END)
		FROM lexemes AS l
		WHERE l.id = ?`, lexemeID).Scan(&found, &clips)
	if err != nil {
		return false, false, fmt.Errorf("db: lexeme audio: %w", err)
	}
	if found == 0 {
		return false, false, nil
	}
	return true, clips.Valid && clips.Int64 > 0, nil
}

// lexemeHasStoredClip reports the one lookup for a lexeme inside a
// transaction. A missing lexeme is an error: card creation must not guess.
func lexemeHasStoredClip(ctx context.Context, tx *sql.Tx, lexemeID int64) (bool, error) {
	exists, has, err := queryLexemeClip(ctx, tx, lexemeID)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, fmt.Errorf("db: lexeme %d not found", lexemeID)
	}
	return has, nil
}

// LexemeHasClip reports the one lookup for a lexeme. A missing lexeme is
// false, the same as a word with no clip. INIT-038/SPEC-003.
func (s *Store) LexemeHasClip(ctx context.Context, lexemeID int64) (bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return false, err
	}
	_, has, err := queryLexemeClip(ctx, conn, lexemeID)
	return has, err
}
