package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Unit progress states for UnitProgress. INIT-030/SPEC-002.
const (
	UnitUntouched = "untouched"
	UnitStarted   = "started"
	UnitFinished  = "finished"
)

// UnitProgress is one learner's progress through one unit in their scope.
// State is untouched, started, or finished. LastFinishedAt is zero when none.
// MeanScore is the mean of finished lesson scores, or zero when none.
// INIT-030/SPEC-002.
type UnitProgress struct {
	UnitID          int64
	State           string
	LessonsFinished int
	LessonsTotal    int
	LastFinishedAt  time.Time
	MeanScore       float64
}

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

// CompletedSharedUnits returns published shared units this learner has finished.
// A draft contributes nothing. A unit is finished when it has at least one
// lesson and every lesson in its doc has a lesson_runs row for this user
// with finished_at set. CompletedAt is the latest of those finishes.
// Learner-scoped units are skipped.
func (s *Store) CompletedSharedUnits(ctx context.Context, userID int64) ([]UnitCompletion, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, position, doc
		FROM units
		WHERE scope = 'shared' AND status = 'published'
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

// FinishedLessons maps each unit to the lesson indexes this learner has finished.
func (s *Store) FinishedLessons(ctx context.Context, userID int64) (map[int64]map[int]time.Time, error) {
	return s.finishedLessons(ctx, userID)
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

// UnitProgress lists every unit in the learner's scope with untouched, started,
// or finished state, lesson counts, last finish time, and mean finished score.
// Scope matches UnitsInScope: shared units plus learner:<id>. Another learner's
// runs do not change state. A unit with zero lessons is never finished.
// INIT-030/SPEC-002.
func (s *Store) UnitProgress(ctx context.Context, userID int64) ([]UnitProgress, error) {
	units, err := s.UnitsInScope(ctx, userID)
	if err != nil {
		return nil, err
	}
	finishes, err := s.finishedLessons(ctx, userID)
	if err != nil {
		return nil, err
	}
	started, err := s.unitsWithRuns(ctx, userID)
	if err != nil {
		return nil, err
	}
	scores, err := s.finishedLessonScores(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]UnitProgress, 0, len(units))
	for _, unit := range units {
		total := lessonCount(unit.Doc)
		done := finishes[unit.ID]
		finishedN := 0
		var latest time.Time
		for ix := 0; ix < total; ix++ {
			at, ok := done[ix]
			if !ok {
				continue
			}
			finishedN++
			if at.After(latest) {
				latest = at
			}
		}
		p := UnitProgress{
			UnitID:          unit.ID,
			LessonsFinished: finishedN,
			LessonsTotal:    total,
			LastFinishedAt:  latest,
			MeanScore:       meanFinishedScores(scores[unit.ID], total),
		}
		switch {
		case !started[unit.ID]:
			p.State = UnitUntouched
		case total > 0 && finishedN == total:
			p.State = UnitFinished
		default:
			p.State = UnitStarted
		}
		out = append(out, p)
	}
	return out, nil
}

// IsUntouched is true when this learner has no lesson_runs row for the unit.
// INIT-030/SPEC-002.
func (s *Store) IsUntouched(ctx context.Context, userID, unitID int64) (bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return false, err
	}
	return isUntouched(ctx, conn, userID, unitID)
}

// IsUntouchedTx is IsUntouched on a caller-owned *sql.Tx. There is no store-wide
// WithTx helper; the revise/publish path begins its own transaction and calls
// this so a concurrent StartPlayerRun cannot slip between the check and the
// write. INIT-030/SPEC-002.
func IsUntouchedTx(ctx context.Context, tx *sql.Tx, userID, unitID int64) (bool, error) {
	return isUntouched(ctx, tx, userID, unitID)
}

func isUntouched(ctx context.Context, q unitQueryer, userID, unitID int64) (bool, error) {
	var n int
	if err := q.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM lesson_runs
		WHERE user_id = ? AND unit_id = ?`, userID, unitID).Scan(&n); err != nil {
		return false, fmt.Errorf("db: is untouched: %w", err)
	}
	return n == 0, nil
}

// unitsWithRuns is the set of unit ids that have any lesson_runs row for the learner.
func (s *Store) unitsWithRuns(ctx context.Context, userID int64) (map[int64]bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT DISTINCT unit_id FROM lesson_runs WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("db: units with runs: %w", err)
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var unitID int64
		if err := rows.Scan(&unitID); err != nil {
			return nil, fmt.Errorf("db: units with runs: %w", err)
		}
		out[unitID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: units with runs: %w", err)
	}
	return out, nil
}

// finishedLessonScores maps unit -> lesson_ix -> score of the latest finished run.
// A finished run with a null score is omitted from the map.
func (s *Store) finishedLessonScores(ctx context.Context, userID int64) (map[int64]map[int]float64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT unit_id, lesson_ix, score
		FROM lesson_runs
		WHERE user_id = ? AND finished_at IS NOT NULL AND score IS NOT NULL
		  AND finished_at = (
			SELECT MAX(finished_at) FROM lesson_runs AS lr
			WHERE lr.user_id = lesson_runs.user_id
			  AND lr.unit_id = lesson_runs.unit_id
			  AND lr.lesson_ix = lesson_runs.lesson_ix
			  AND lr.finished_at IS NOT NULL
		  )`, userID)
	if err != nil {
		return nil, fmt.Errorf("db: finished lesson scores: %w", err)
	}
	defer rows.Close()
	out := map[int64]map[int]float64{}
	for rows.Next() {
		var unitID int64
		var ix int
		var score float64
		if err := rows.Scan(&unitID, &ix, &score); err != nil {
			return nil, fmt.Errorf("db: finished lesson scores: %w", err)
		}
		if out[unitID] == nil {
			out[unitID] = map[int]float64{}
		}
		out[unitID][ix] = score
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: finished lesson scores: %w", err)
	}
	return out, nil
}

func meanFinishedScores(byIx map[int]float64, total int) float64 {
	if total <= 0 || len(byIx) == 0 {
		return 0
	}
	sum := 0.0
	n := 0
	for ix, score := range byIx {
		if ix < 0 || ix >= total {
			continue
		}
		sum += score
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// LessonCount is the number of lessons in a unit document.
// A document that is not an object, or that has no lessons array, has none.
func LessonCount(doc string) int {
	return lessonCount(doc)
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
