package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Session is a row in sessions. IDHash is the digest of the cookie value.
// The raw id is not stored.
type Session struct {
	IDHash    string
	UserID    int64
	CSRF      string
	ExpiresAt time.Time
}

// InsertSession writes one session. expiresAt is stored as UTC RFC3339.
func (s *Store) InsertSession(ctx context.Context, idHash string, userID int64, csrf string, expiresAt time.Time) error {
	if err := s.ready(); err != nil {
		return err
	}
	_, err := s.exec(ctx,
		`INSERT INTO sessions (id_hash, user_id, csrf, expires_at) VALUES (?, ?, ?, ?)`,
		idHash, userID, csrf, formatTime(expiresAt),
	)
	if err != nil {
		return fmt.Errorf("db: insert session: %w", err)
	}
	return nil
}

// SessionUser loads the session and its user. ErrNotFound means no row.
func (s *Store) SessionUser(ctx context.Context, idHash string) (Session, User, error) {
	if err := s.ready(); err != nil {
		return Session{}, User{}, err
	}
	row := s.queryRow(ctx, `
		SELECT s.id_hash, s.user_id, s.csrf, s.expires_at,
		       u.id, u.email, u.google_sub, u.role, u.created_at
		FROM sessions AS s
		JOIN users AS u ON u.id = s.user_id
		WHERE s.id_hash = ?`, idHash)
	var sess Session
	var exp string
	var u User
	var created string
	err := row.Scan(&sess.IDHash, &sess.UserID, &sess.CSRF, &exp,
		&u.ID, &u.Email, &u.GoogleSub, &u.Role, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, User{}, ErrNotFound
	}
	if err != nil {
		return Session{}, User{}, fmt.Errorf("db: scan session: %w", err)
	}
	sess.ExpiresAt, err = parseStoredTime(exp)
	if err != nil {
		return Session{}, User{}, fmt.Errorf("db: session expires_at: %w", err)
	}
	u.CreatedAt, err = parseStoredTime(created)
	if err != nil {
		return Session{}, User{}, fmt.Errorf("db: user created_at: %w", err)
	}
	return sess, u, nil
}

// UpdateSessionExpiry moves expires_at. Used when a live session slides.
func (s *Store) UpdateSessionExpiry(ctx context.Context, idHash string, expiresAt time.Time) error {
	if err := s.ready(); err != nil {
		return err
	}
	res, err := s.exec(ctx, `UPDATE sessions SET expires_at = ? WHERE id_hash = ?`, formatTime(expiresAt), idHash)
	if err != nil {
		return fmt.Errorf("db: update session: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: update session: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSession removes one session. A missing row is not an error.
func (s *Store) DeleteSession(ctx context.Context, idHash string) error {
	if err := s.ready(); err != nil {
		return err
	}
	_, err := s.exec(ctx, `DELETE FROM sessions WHERE id_hash = ?`, idHash)
	if err != nil {
		return fmt.Errorf("db: delete session: %w", err)
	}
	return nil
}

// DeleteUserSessions removes every session for the user. Used when the
// address is no longer on the allowlist.
func (s *Store) DeleteUserSessions(ctx context.Context, userID int64) error {
	if err := s.ready(); err != nil {
		return err
	}
	_, err := s.exec(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("db: delete user sessions: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes rows whose expires_at is at or before now.
// Callers run this when they already touch the sessions table.
func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	if err := s.ready(); err != nil {
		return err
	}
	_, err := s.exec(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, formatTime(now))
	if err != nil {
		return fmt.Errorf("db: delete expired sessions: %w", err)
	}
	return nil
}
