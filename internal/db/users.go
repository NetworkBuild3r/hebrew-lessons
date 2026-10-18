package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// User is a row in users.
type User struct {
	ID        int64
	Email     string
	GoogleSub string
	Role      string
	CreatedAt time.Time
}

// CreateUser inserts one user. role must be learner or admin.
// email is stored lowercase. created_at comes from the store clock.
func (s *Store) CreateUser(ctx context.Context, email, googleSub, role string) (User, error) {
	if err := s.ready(); err != nil {
		return User{}, err
	}
	email, googleSub, role, err := normalizeUser(email, googleSub, role)
	if err != nil {
		return User{}, err
	}
	created := formatTime(s.clock.Now())
	res, err := s.exec(ctx,
		`INSERT INTO users (email, google_sub, role, created_at) VALUES (?, ?, ?, ?)`,
		email, googleSub, role, created,
	)
	if err != nil {
		return User{}, fmt.Errorf("db: insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("db: insert user id: %w", err)
	}
	createdAt, err := parseStoredTime(created)
	if err != nil {
		return User{}, err
	}
	return User{ID: id, Email: email, GoogleSub: googleSub, Role: role, CreatedAt: createdAt}, nil
}

// UpsertGoogleUser inserts a user or updates the row found by google_sub,
// falling back to email. email is stored lowercase. created_at comes from
// the store clock. The call is one transaction.
func (s *Store) UpsertGoogleUser(ctx context.Context, email, googleSub, role string) (User, error) {
	if err := s.ready(); err != nil {
		return User{}, err
	}
	email, googleSub, role, err := normalizeUser(email, googleSub, role)
	if err != nil {
		return User{}, err
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("db: begin user upsert: %w", err)
	}
	defer tx.Rollback()

	u, err := findUserWhere(ctx, tx, "google_sub = ?", googleSub)
	if errors.Is(err, ErrNotFound) {
		u, err = findUserWhere(ctx, tx, "email = ?", email)
	}
	if errors.Is(err, ErrNotFound) {
		created := formatTime(s.clock.Now())
		res, err := tx.ExecContext(ctx,
			`INSERT INTO users (email, google_sub, role, created_at) VALUES (?, ?, ?, ?)`,
			email, googleSub, role, created,
		)
		if err != nil {
			return User{}, fmt.Errorf("db: insert user: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return User{}, fmt.Errorf("db: insert user id: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return User{}, fmt.Errorf("db: commit user: %w", err)
		}
		createdAt, err := parseStoredTime(created)
		if err != nil {
			return User{}, err
		}
		return User{ID: id, Email: email, GoogleSub: googleSub, Role: role, CreatedAt: createdAt}, nil
	}
	if err != nil {
		return User{}, err
	}

	newEmail := email
	other, err := findUserWhere(ctx, tx, "email = ?", email)
	if err == nil && other.ID != u.ID {
		// The Google account's current email belongs to someone else.
		// Keep the row we already matched.
		newEmail = u.Email
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return User{}, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET email = ?, google_sub = ?, role = ? WHERE id = ?`,
		newEmail, googleSub, role, u.ID,
	); err != nil {
		return User{}, fmt.Errorf("db: update user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("db: commit user: %w", err)
	}
	u.Email = newEmail
	u.GoogleSub = googleSub
	u.Role = role
	return u, nil
}

// UserByEmail returns the user with that email. The match is lowercase.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	if err := s.ready(); err != nil {
		return User{}, err
	}
	u, err := findUserWhere(ctx, s.conn, "email = ?", strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return User{}, err
	}
	return u, nil
}

type rowQuery interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func findUserWhere(ctx context.Context, q rowQuery, clause, arg string) (User, error) {
	row := q.QueryRowContext(ctx,
		`SELECT id, email, google_sub, role, created_at FROM users WHERE `+clause, arg)
	return scanUser(row)
}

func scanUser(row *sql.Row) (User, error) {
	var u User
	var created string
	err := row.Scan(&u.ID, &u.Email, &u.GoogleSub, &u.Role, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("db: scan user: %w", err)
	}
	u.CreatedAt, err = parseStoredTime(created)
	if err != nil {
		return User{}, fmt.Errorf("db: user created_at: %w", err)
	}
	return u, nil
}

func normalizeUser(email, googleSub, role string) (string, string, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	googleSub = strings.TrimSpace(googleSub)
	role = strings.TrimSpace(role)
	if email == "" || googleSub == "" {
		return "", "", "", errors.New("db: email and google_sub are required")
	}
	if role != "learner" && role != "admin" {
		return "", "", "", fmt.Errorf("db: role %q must be learner or admin", role)
	}
	return email, googleSub, role, nil
}

func (s *Store) ready() error {
	if s == nil || s.conn == nil {
		return errors.New("db: store is closed")
	}
	return nil
}

func formatTime(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format(time.RFC3339)
}

func parseStoredTime(v string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339Nano, v)
}
