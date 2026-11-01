package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// UserTimeZone returns the learner's stored zone name.
// ok is false when the column is null or empty. IANA validation is not done here.
func (s *Store) UserTimeZone(ctx context.Context, id int64) (zone string, ok bool, err error) {
	conn, err := s.connOrErr()
	if err != nil {
		return "", false, err
	}
	var nz sql.NullString
	err = conn.QueryRowContext(ctx, `SELECT time_zone FROM users WHERE id = ?`, id).Scan(&nz)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, ErrNotFound
	}
	if err != nil {
		return "", false, fmt.Errorf("db: user time zone: %w", err)
	}
	if !nz.Valid || nz.String == "" {
		return "", false, nil
	}
	return nz.String, true, nil
}

// SetUserTimeZone stores the zone name on the user. It refuses an empty value and
// more than 64 characters. It does not validate IANA (SPEC-003).
func (s *Store) SetUserTimeZone(ctx context.Context, id int64, zone string) error {
	if err := s.ready(); err != nil {
		return err
	}
	if zone == "" {
		return errors.New("db: time zone is required")
	}
	if len(zone) > 64 {
		return fmt.Errorf("db: time zone length %d exceeds 64", len(zone))
	}
	res, err := s.exec(ctx, `UPDATE users SET time_zone = ? WHERE id = ?`, zone, id)
	if err != nil {
		return fmt.Errorf("db: set user time zone: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: set user time zone: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UserTimeZones returns every user id whose time_zone is non-null and non-empty.
// Used by scheduled jobs that need a civil day per learner.
func (s *Store) UserTimeZones(ctx context.Context) (map[int64]string, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx,
		`SELECT id, time_zone FROM users WHERE time_zone IS NOT NULL AND time_zone <> ''`)
	if err != nil {
		return nil, fmt.Errorf("db: user time zones: %w", err)
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var zone string
		if err := rows.Scan(&id, &zone); err != nil {
			return nil, fmt.Errorf("db: user time zones: %w", err)
		}
		out[id] = zone
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: user time zones: %w", err)
	}
	return out, nil
}
