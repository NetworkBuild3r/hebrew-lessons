package pages

import (
	"context"
	"database/sql"
	"errors"
)

// Queryer is the read-only database handle from internal/db.
// *sql.DB satisfies it. Pages never write.
type Queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// PlanSnapshot is one published plan document, still the stored JSON text.
type PlanSnapshot struct {
	ID       string
	Snapshot string
}

// Source reads guide catalogs. Implementations must not normalize the JSON.
type Source interface {
	Snapshots(ctx context.Context) ([]PlanSnapshot, error)
	HasGrant(ctx context.Context, lessonID string, userID int64) (bool, error)
}

// SQLSource reads plans and lesson_grants through a read-only handle.
type SQLSource struct {
	DB Queryer
}

// NewSQLSource wraps the handle opened by internal/db.
func NewSQLSource(db Queryer) SQLSource {
	return SQLSource{DB: db}
}

const snapshotQuery = `
SELECT id, snapshot FROM plans
WHERE snapshot IS NOT NULL AND snapshot != ''`

const grantQuery = `
SELECT 1 FROM lesson_grants WHERE slug = ? AND user_id = ? LIMIT 1`

// Snapshots returns stored plan documents in table order. The JSON is not rewritten.
func (s SQLSource) Snapshots(ctx context.Context) ([]PlanSnapshot, error) {
	if s.DB == nil {
		return nil, errors.New("guides: no database")
	}
	rows, err := s.DB.QueryContext(ctx, snapshotQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PlanSnapshot
	for rows.Next() {
		var row PlanSnapshot
		if err := rows.Scan(&row.ID, &row.Snapshot); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// HasGrant reports whether userID may read a selected lesson.
func (s SQLSource) HasGrant(ctx context.Context, lessonID string, userID int64) (bool, error) {
	if s.DB == nil {
		return false, errors.New("guides: no database")
	}
	rows, err := s.DB.QueryContext(ctx, grantQuery, lessonID, userID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	ok := rows.Next()
	if err := rows.Err(); err != nil {
		return false, err
	}
	return ok, nil
}
