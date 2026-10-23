package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Asset is one audio_assets row.
// ID is the lowercase hex sha256 of the mp3 bytes. TextNFC is the exact
// pointed text the clip speaks, already normalized by the writer.
type Asset struct {
	ID        string
	TextNFC   string
	Voice     string
	Model     string
	Bytes     int64
	CreatedAt time.Time
}

// audioByTextQuery prefers the newest known model, then the newest
// created_at. created_at is FormatTime, so the text order matches time
// order. id is the last key so two rows with the same model and timestamp
// still resolve the same way every call.
//
// Unlisted models sort after legacy. eleven_v4 beats eleven_v3, and both
// beat legacy, even when the older model was stored later.
const audioByTextQuery = `
SELECT id, text_nfc, voice, model, bytes, created_at
FROM audio_assets
WHERE text_nfc = ?
ORDER BY CASE model
    WHEN 'eleven_v4' THEN 0
    WHEN 'eleven_v3' THEN 1
    WHEN 'legacy' THEN 2
    ELSE 3
END, created_at DESC, id ASC
LIMIT 1`

// audioByTextVoiceQuery is audioByTextQuery limited to one voice id. A dialogue
// line is played in its speaker's voice, so the same text can have one clip per
// voice, and each lookup has to name the voice it wants.
const audioByTextVoiceQuery = `
SELECT id, text_nfc, voice, model, bytes, created_at
FROM audio_assets
WHERE text_nfc = ? AND voice = ?
ORDER BY CASE model
    WHEN 'eleven_v4' THEN 0
    WHEN 'eleven_v3' THEN 1
    WHEN 'legacy' THEN 2
    ELSE 3
END, created_at DESC, id ASC
LIMIT 1`

// AudioByTextVoice is AudioByText for one voice: the clip of textNFC whose voice
// column is exactly voice. A clip in any other voice is not a match, and an empty
// voice matches nothing. The model and time order are the same as AudioByText.
func (s *Store) AudioByTextVoice(ctx context.Context, textNFC, voice string) (Asset, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return Asset{}, false, err
	}
	if voice == "" {
		return Asset{}, false, nil
	}
	var a Asset
	var created string
	err = conn.QueryRowContext(ctx, audioByTextVoiceQuery, textNFC, voice).Scan(
		&a.ID, &a.TextNFC, &a.Voice, &a.Model, &a.Bytes, &created,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, false, nil
	}
	if err != nil {
		return Asset{}, false, fmt.Errorf("db: audio by text and voice: %w", err)
	}
	a.CreatedAt, err = ParseTime(created)
	if err != nil {
		return Asset{}, false, fmt.Errorf("db: audio by text and voice: %w", err)
	}
	return a, true, nil
}

// InsertAudio inserts one clip. A row with the same id is left unchanged
// and inserted is false. created_at uses the store clock when a.CreatedAt
// is zero.
func (s *Store) InsertAudio(ctx context.Context, a Asset) (inserted bool, err error) {
	conn, err := s.connOrErr()
	if err != nil {
		return false, err
	}
	if a.ID == "" || a.TextNFC == "" {
		return false, errors.New("db: audio id and text are required")
	}
	if a.Bytes < 0 {
		return false, errors.New("db: audio size is negative")
	}
	at := a.CreatedAt
	if at.IsZero() {
		if s.clock == nil {
			return false, errors.New("db: clock is not configured")
		}
		at = s.clock.Now()
	}
	res, err := conn.ExecContext(ctx, `
		INSERT OR IGNORE INTO audio_assets (id, text_nfc, voice, model, bytes, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		a.ID, a.TextNFC, a.Voice, a.Model, a.Bytes, FormatTime(at),
	)
	if err != nil {
		return false, fmt.Errorf("db: insert audio: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("db: insert audio: %w", err)
	}
	return n > 0, nil
}

// HasAudio reports whether a clip with this sha256 id is already stored.
func (s *Store) HasAudio(ctx context.Context, id string) (bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return false, err
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM audio_assets WHERE id = ?`, id).Scan(&n); err != nil {
		return false, fmt.Errorf("db: has audio: %w", err)
	}
	return n > 0, nil
}

// AudioByText returns the clip the app should play for textNFC.
// textNFC must already be the normalized form stored in the table.
// The preference is eleven_v4, then eleven_v3, then legacy, then any
// other model, and within a model the newest created_at. ok is false
// when no row matches.
func (s *Store) AudioByText(ctx context.Context, textNFC string) (Asset, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return Asset{}, false, err
	}
	var a Asset
	var created string
	err = conn.QueryRowContext(ctx, audioByTextQuery, textNFC).Scan(
		&a.ID, &a.TextNFC, &a.Voice, &a.Model, &a.Bytes, &created,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, false, nil
	}
	if err != nil {
		return Asset{}, false, fmt.Errorf("db: audio by text: %w", err)
	}
	a.CreatedAt, err = ParseTime(created)
	if err != nil {
		return Asset{}, false, fmt.Errorf("db: audio by text: %w", err)
	}
	return a, true, nil
}

// AudioIDs returns the clip id for each text that has one. A text with several clips keeps
// the greatest id, which is stable. A text with none is left out.
func (s *Store) AudioIDs(ctx context.Context, texts []string) (map[string]string, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	const chunk = 500
	for start := 0; start < len(texts); start += chunk {
		end := min(start+chunk, len(texts))
		part := texts[start:end]
		marks := strings.TrimSuffix(strings.Repeat("?,", len(part)), ",")
		args := make([]any, len(part))
		for i, text := range part {
			args[i] = text
		}
		rows, err := conn.QueryContext(ctx,
			`SELECT text_nfc, MAX(id) FROM audio_assets WHERE text_nfc IN (`+marks+`) GROUP BY text_nfc`, args...)
		if err != nil {
			return nil, fmt.Errorf("db: audio ids: %w", err)
		}
		for rows.Next() {
			var text, id string
			if err := rows.Scan(&text, &id); err != nil {
				rows.Close()
				return nil, fmt.Errorf("db: audio ids: %w", err)
			}
			out[text] = id
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("db: audio ids: %w", err)
		}
	}
	return out, nil
}
