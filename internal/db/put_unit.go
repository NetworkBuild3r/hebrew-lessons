package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// PutUnit inserts a unit at doc.Position, or updates the one already there,
// and writes a unit_revisions row in the same transaction.
// A change that only sets scope still writes a revision.
// scope is "shared" or "learner:" followed by one or more digits.
// The revision actor is put_unit. A new unit is a draft; an update keeps
// the status it already has. The returned id is the units row.
func (s *Store) PutUnit(ctx context.Context, doc units.Unit, scope string) (int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	if s.clock == nil {
		return 0, fmt.Errorf("db: clock is not configured")
	}
	if !validUnitScope(scope) {
		return 0, fmt.Errorf("db: unit scope %q", scope)
	}
	if doc.Position < 0 {
		return 0, fmt.Errorf("db: unit position %d", doc.Position)
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return 0, fmt.Errorf("db: unit doc: %w", err)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("db: begin unit: %w", err)
	}
	defer tx.Rollback()

	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM units WHERE position = ?`, doc.Position).Scan(&n); err != nil {
		return 0, fmt.Errorf("db: put unit: %w", err)
	}
	var id int64
	var rev int
	switch n {
	case 0:
		res, err := tx.ExecContext(ctx, `
			INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
			VALUES (?, ?, ?, ?, 'draft', 1, ?, ?)`,
			doc.Position, doc.Title, doc.Situation, doc.Concept, string(body), scope,
		)
		if err != nil {
			return 0, fmt.Errorf("db: put unit: %w", err)
		}
		id, err = res.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("db: put unit: %w", err)
		}
		rev = 1
	case 1:
		if err := tx.QueryRowContext(ctx, `SELECT id, rev FROM units WHERE position = ?`, doc.Position).Scan(&id, &rev); err != nil {
			return 0, fmt.Errorf("db: put unit: %w", err)
		}
		rev++
		if _, err := tx.ExecContext(ctx, `
			UPDATE units
			SET title = ?, situation = ?, concept = ?, doc = ?, scope = ?, rev = ?
			WHERE id = ?`,
			doc.Title, doc.Situation, doc.Concept, string(body), scope, rev, id,
		); err != nil {
			return 0, fmt.Errorf("db: put unit: %w", err)
		}
	default:
		return 0, fmt.Errorf("db: %d units at position %d", n, doc.Position)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO unit_revisions (unit_id, rev, doc, actor, created_at)
		VALUES (?, ?, ?, 'put_unit', ?)`,
		id, rev, string(body), FormatTime(s.clock.Now()),
	); err != nil {
		return 0, fmt.Errorf("db: put unit revision: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("db: commit unit: %w", err)
	}
	return id, nil
}

// validUnitScope accepts "shared" and "learner:" plus one or more ASCII digits.
// The SQL check is wider: it allows learner:1abc. This function does not.
func validUnitScope(scope string) bool {
	if scope == "shared" {
		return true
	}
	rest, ok := strings.CutPrefix(scope, "learner:")
	if !ok || rest == "" {
		return false
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
