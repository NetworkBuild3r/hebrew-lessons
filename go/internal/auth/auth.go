// Package auth resolves an existing session. It does not sign anyone in.
//
// Login, logout, Google, and scrypt are out of scope. Resolve only reads
// sessions and users. It does not update last_seen_at (Node's
// server/auth/sessions.js resolve does; this port stays read-only).
package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// User is the signed-in account. It has no password hash.
// The shape matches server/accounts.js publicUser, including role.
type User struct {
	ID                 int64   `json:"id"`
	Username           string  `json:"username"`
	DisplayName        *string `json:"display_name"`
	Role               string  `json:"role"`
	Status             string  `json:"status"`
	AuthProvider       string  `json:"auth_provider"`
	GoogleEmail        *string `json:"google_email"`
	MustChangePassword bool    `json:"must_change_password"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

type contextKey int

const (
	userKey contextKey = iota
	tokenHashKey
)

// HashToken is the SHA-256 hex of the raw session token, UTF-8.
// sessions.token_hash stores this value (server/auth/sessions.js hashToken).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewContext stores user and the session token hash for later middleware.
// A nil user leaves ctx unchanged.
func NewContext(ctx context.Context, user *User, tokenHash string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if user == nil {
		return ctx
	}
	ctx = context.WithValue(ctx, userKey, user)
	ctx = context.WithValue(ctx, tokenHashKey, tokenHash)
	return ctx
}

// UserFromContext returns the user Middleware or NewContext stored, or nil.
func UserFromContext(ctx context.Context) *User {
	if ctx == nil {
		return nil
	}
	user, _ := ctx.Value(userKey).(*User)
	return user
}

// TokenHashFromContext returns the SHA-256 hex of the raw session token
// that resolved the current user, or "" when no session is stored.
func TokenHashFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	hash, _ := ctx.Value(tokenHashKey).(string)
	return hash
}

// Resolve looks up token and returns the user, or (nil, nil) when the token
// is missing, unknown, expired, or the account is not active.
//
// Expiry matches server/auth/sessions.js: Date.parse(expires_at) must be
// strictly greater than now, compared in Unix milliseconds. now is the
// caller's clock. The query is a SELECT. last_seen_at is not written.
func Resolve(ctx context.Context, conn *sql.DB, token string, now time.Time) (*User, error) {
	if token == "" {
		return nil, nil
	}
	if conn == nil {
		return nil, errors.New("auth: nil database")
	}
	if ctx == nil {
		return nil, errors.New("auth: nil context")
	}

	const query = `
		SELECT u.id, u.username, u.display_name, u.role, u.status,
		       u.auth_provider, u.google_email, u.must_change_password,
		       u.created_at, u.updated_at, s.expires_at
		FROM sessions AS s
		JOIN users AS u ON u.id = s.user_id
		WHERE s.token_hash = ?`

	var user User
	var displayName, googleEmail sql.NullString
	var mustChange int
	var expiresAt string
	err := conn.QueryRowContext(ctx, query, HashToken(token)).Scan(
		&user.ID,
		&user.Username,
		&displayName,
		&user.Role,
		&user.Status,
		&user.AuthProvider,
		&googleEmail,
		&mustChange,
		&user.CreatedAt,
		&user.UpdatedAt,
		&expiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("auth: resolve session: %w", err)
	}
	if !expiresAfter(expiresAt, now) {
		return nil, nil
	}
	if user.Status != "active" {
		return nil, nil
	}
	user.DisplayName = stringPtr(displayName, false)
	user.GoogleEmail = stringPtr(googleEmail, true)
	user.MustChangePassword = mustChange != 0
	return &user, nil
}

// expiresAfter reports whether expiresAt is strictly after now, in Unix
// milliseconds, matching Date.parse(expires_at) > now.getTime().
// An unparseable timestamp is expired (Date.parse returns NaN, and the
// Node comparison fails closed).
func expiresAfter(expiresAt string, now time.Time) bool {
	exp, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return false
	}
	return exp.UnixMilli() > now.UnixMilli()
}

// stringPtr copies a nullable column. emptyAsNil is for google_email, where
// Node's `value || null` turns "" into null. display_name keeps "".
func stringPtr(value sql.NullString, emptyAsNil bool) *string {
	if !value.Valid {
		return nil
	}
	if emptyAsNil && value.String == "" {
		return nil
	}
	s := value.String
	return &s
}
