package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
// created_at. created_at is formatTime, so the text order matches time
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
		a.ID, a.TextNFC, a.Voice, a.Model, a.Bytes, formatTime(at),
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
	a.CreatedAt, err = parseTime(created)
	if err != nil {
		return Asset{}, false, fmt.Errorf("db: audio by text: %w", err)
	}
	return a, true, nil
}
