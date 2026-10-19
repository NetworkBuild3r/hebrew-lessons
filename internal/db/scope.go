package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// UnitRow is one units row, including its scope.
// Doc is the unit JSON. This is not units.Unit.
type UnitRow struct {
	ID        int64
	Position  int
	Title     string
	Situation string
	Concept   string
	Status    string
	Rev       int
	Doc       string
	Scope     string
}

// UnitCompletion is a shared unit whose every lesson this learner has finished.
// CompletedAt is the latest of those finishes.
type UnitCompletion struct {
	UnitID      int64
	Position    int
	CompletedAt time.Time
}

// CompletedSharedUnits returns shared units this learner has finished.
// A unit is finished when it has at least one lesson and every lesson in
// its doc has a lesson_runs row for this user with finished_at set.
// CompletedAt is the latest of those finishes. Learner-scoped units are skipped.
func (s *Store) CompletedSharedUnits(ctx context.Context, userID int64) ([]UnitCompletion, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, position, doc
		FROM units
		WHERE scope = 'shared'
		ORDER BY position, id`)
	if err != nil {
		return nil, fmt.Errorf("db: completed units: %w", err)
	}
	type sharedUnit struct {
		id       int64
		position int
		doc      string
	}
	var units []sharedUnit
	for rows.Next() {
		var row sharedUnit
		if err := rows.Scan(&row.id, &row.position, &row.doc); err != nil {
			rows.Close()
			return nil, fmt.Errorf("db: completed units: %w", err)
		}
		units = append(units, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("db: completed units: %w", err)
	}
	rows.Close()

	finishes, err := s.finishedLessons(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []UnitCompletion
	for _, unit := range units {
		n := lessonCount(unit.doc)
		if n == 0 {
			continue
		}
		done := finishes[unit.id]
		if len(done) < n {
			continue
		}
		var latest time.Time
		complete := true
		for ix := 0; ix < n; ix++ {
			at, ok := done[ix]
			if !ok {
				complete = false
				break
			}
			if at.After(latest) {
				latest = at
			}
		}
		if !complete {
			continue
		}
		out = append(out, UnitCompletion{
			UnitID:      unit.id,
			Position:    unit.position,
			CompletedAt: latest,
		})
	}
	return out, nil
}

// UnitsInScope returns shared units plus this learner's private units, by position.
// A private unit's scope is learner:<id>. Another learner's units are left out.
func (s *Store) UnitsInScope(ctx context.Context, learnerID int64) ([]UnitRow, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, position, title, situation, concept, status, rev, doc, scope
		FROM units
		WHERE scope = 'shared' OR scope = ?
		ORDER BY position, id`, fmt.Sprintf("learner:%d", learnerID))
	if err != nil {
		return nil, fmt.Errorf("db: units in scope: %w", err)
	}
	defer rows.Close()
	var out []UnitRow
	for rows.Next() {
		var row UnitRow
		if err := rows.Scan(
			&row.ID, &row.Position, &row.Title, &row.Situation, &row.Concept,
			&row.Status, &row.Rev, &row.Doc, &row.Scope,
		); err != nil {
			return nil, fmt.Errorf("db: units in scope: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: units in scope: %w", err)
	}
	return out, nil
}

func (s *Store) finishedLessons(ctx context.Context, userID int64) (map[int64]map[int]time.Time, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT unit_id, lesson_ix, MAX(finished_at)
		FROM lesson_runs
		WHERE user_id = ? AND finished_at IS NOT NULL
		GROUP BY unit_id, lesson_ix`, userID)
	if err != nil {
		return nil, fmt.Errorf("db: finished lessons: %w", err)
	}
	defer rows.Close()
	out := map[int64]map[int]time.Time{}
	for rows.Next() {
		var unitID int64
		var ix int
		var at string
		if err := rows.Scan(&unitID, &ix, &at); err != nil {
			return nil, fmt.Errorf("db: finished lessons: %w", err)
		}
		when, err := ParseTime(at)
		if err != nil {
			return nil, fmt.Errorf("db: finished lessons: %w", err)
		}
		if out[unitID] == nil {
			out[unitID] = map[int]time.Time{}
		}
		out[unitID][ix] = when
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: finished lessons: %w", err)
	}
	return out, nil
}

// lessonCount is the number of lessons in a unit document.
// A document that is not an object, or that has no lessons array, has none.
func lessonCount(doc string) int {
	var body struct {
		Lessons []json.RawMessage `json:"lessons"`
	}
	if err := json.Unmarshal([]byte(doc), &body); err != nil {
		return 0
	}
	return len(body.Lessons)
}
