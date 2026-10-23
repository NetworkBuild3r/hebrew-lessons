package brief

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// idBytes is the size of a brief id: 128 bits.
const idBytes = 16

// idLen is the length of an id as base64url text without padding.
const idLen = 22

var idEncoding = base64.RawURLEncoding.Strict()

// newID returns a fresh random id. It reads only the random source and fails if the source
// does. It never falls back to anything weaker.
func (b *Builder) newID() (string, error) {
	var raw [idBytes]byte
	r := b.rand
	if r == nil {
		r = rand.Reader
	}
	if _, err := io.ReadFull(r, raw[:]); err != nil {
		return "", fmt.Errorf("brief: random id: %w", err)
	}
	return idEncoding.EncodeToString(raw[:]), nil
}

// validID reports whether id has the shape newID makes. An id of any other shape cannot be
// in the table, so lookups of it skip the query.
func validID(id string) bool {
	if len(id) != idLen {
		return false
	}
	raw, err := idEncoding.DecodeString(id)
	return err == nil && len(raw) == idBytes
}

func (b *Builder) insert(ctx context.Context, id string, userID int64, theme string, payload []byte, now time.Time) error {
	conn, err := b.store.DB()
	if err != nil {
		return fmt.Errorf("brief: %w", err)
	}
	_, err = conn.ExecContext(ctx,
		`INSERT INTO lesson_briefs (id, user_id, theme, payload_json, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, userID, theme, string(payload), db.FormatTime(now))
	if err != nil {
		return fmt.Errorf("brief: store: %w", err)
	}
	return nil
}

// Get returns one of the learner's briefs. An id that is malformed, unknown, or another
// learner's is ErrNotFound, and there is no way to tell the cases apart. A brief past its
// TTL is ErrExpired. A used brief is returned with UsedByUnitID set.
func (b *Builder) Get(ctx context.Context, userID int64, id string) (Brief, error) {
	if err := b.ready(); err != nil {
		return Brief{}, err
	}
	if userID <= 0 || !validID(id) {
		return Brief{}, ErrNotFound
	}
	conn, err := b.store.DB()
	if err != nil {
		return Brief{}, fmt.Errorf("brief: %w", err)
	}
	var payload, created string
	var used sql.NullInt64
	err = conn.QueryRowContext(ctx,
		`SELECT payload_json, created_at, used_by_unit_id FROM lesson_briefs WHERE id = ? AND user_id = ?`,
		id, userID).Scan(&payload, &created, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return Brief{}, ErrNotFound
	}
	if err != nil {
		return Brief{}, fmt.Errorf("brief: read: %w", err)
	}
	at, err := db.ParseTime(created)
	if err != nil {
		return Brief{}, fmt.Errorf("brief: read: %w", err)
	}
	out := Brief{ID: id, UserID: userID, CreatedAt: at, ExpiresAt: at.Add(TTL)}
	if used.Valid {
		unit := used.Int64
		out.UsedByUnitID = &unit
	}
	if err := json.Unmarshal([]byte(payload), &out.Plan); err != nil {
		return Brief{}, fmt.Errorf("brief: read payload: %w", err)
	}
	if out.V < 1 || out.V > Version {
		return Brief{}, fmt.Errorf("brief: payload version %d, want 1 to %d", out.V, Version)
	}
	if !b.clock.Now().Before(out.ExpiresAt) {
		return Brief{}, ErrExpired
	}
	return out, nil
}

// MarkUsed records that a unit was built from the brief. It works once: a second call
// returns ErrUsed and leaves the first unit in place. It is scoped to the learner like Get,
// and an expired brief cannot be used.
func (b *Builder) MarkUsed(ctx context.Context, userID int64, id string, unitID int64) error {
	if err := b.ready(); err != nil {
		return err
	}
	if userID <= 0 || !validID(id) {
		return ErrNotFound
	}
	if unitID <= 0 {
		return errors.New("brief: no unit")
	}
	conn, err := b.store.DB()
	if err != nil {
		return fmt.Errorf("brief: %w", err)
	}
	cutoff := db.FormatTime(b.clock.Now().Add(-TTL))
	res, err := conn.ExecContext(ctx, `
		UPDATE lesson_briefs SET used_by_unit_id = ?
		WHERE id = ? AND user_id = ? AND used_by_unit_id IS NULL AND created_at > ?`,
		unitID, id, userID, cutoff)
	if err != nil {
		return fmt.Errorf("brief: mark used: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("brief: mark used: %w", err)
	}
	if n == 1 {
		return nil
	}
	// Nothing changed: the brief is missing, expired, or already used.
	if _, err := b.Get(ctx, userID, id); err != nil {
		return err
	}
	return ErrUsed
}

// coachNotes reads the newest notes left for the learner in the last CoachNoteDays.
func (b *Builder) coachNotes(ctx context.Context, userID int64, now time.Time) ([]CoachNote, error) {
	conn, err := b.store.DB()
	if err != nil {
		return nil, fmt.Errorf("brief: %w", err)
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT author, body, created_at FROM coach_notes
		WHERE user_id = ? AND created_at >= ?
		ORDER BY created_at DESC, id DESC LIMIT ?`,
		userID, db.FormatTime(now.AddDate(0, 0, -CoachNoteDays)), MaxCoachNotes)
	if err != nil {
		return nil, fmt.Errorf("brief: coach notes: %w", err)
	}
	defer rows.Close()
	out := []CoachNote{}
	for rows.Next() {
		var n CoachNote
		var at string
		if err := rows.Scan(&n.Author, &n.Body, &at); err != nil {
			return nil, fmt.Errorf("brief: coach notes: %w", err)
		}
		if n.At, err = db.ParseTime(at); err != nil {
			return nil, fmt.Errorf("brief: coach notes: %w", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("brief: coach notes: %w", err)
	}
	return out, nil
}
