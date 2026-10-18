package path

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/learner/config"
)

// Service reads published plans, lesson progress, and review rows.
// It never writes. Construct it with a read-only connection.
type Service struct {
	db  *sql.DB
	cfg config.Config
	now func() time.Time
	loc *time.Location
}

// New returns a service that uses time.Now and the process local zone.
// cfg supplies the ADR-0017 thresholds. A zero Config does not apply the
// defaults; call config.Load.
func New(db *sql.DB, cfg config.Config) *Service {
	return &Service{db: db, cfg: cfg, now: time.Now}
}

// Config returns the thresholds this service was built with.
func (s *Service) Config() config.Config {
	if s == nil {
		return config.Config{}
	}
	return s.cfg
}

// WithClock returns a copy that uses now for DueCount and Path.
// The database and config are shared. now nil means time.Now.
func (s *Service) WithClock(now func() time.Time) *Service {
	cp := *s
	cp.now = now
	return &cp
}

// WithLocation returns a copy that buckets due dates in loc.
// nil means the process local zone, matching Node's Date local fields.
func (s *Service) WithLocation(loc *time.Location) *Service {
	cp := *s
	cp.loc = loc
	return &cp
}

// Path returns the course line for userID.
//
// planID empty selects the assigned plan: the readable published personal
// plan whose assignee_id equals the user's username, lowest plans.id first.
// A shared plan is not a fallback. An explicit planID is that row, when
// the snapshot is published and readable. Personal plans are readable by
// the assignee. A shared plan is readable only when every lesson is
// visibility "everyone", or "selected" with a lesson_grants row for this
// user, matching server/api/plan-routes.js mayReadPublished. One hidden
// lesson makes the shared plan unreadable.
//
// The read uses the published snapshot, not the working copy.
func (s *Service) Path(ctx context.Context, userID int64, planID string) (Path, error) {
	if err := ctxErr(ctx); err != nil {
		return Path{}, err
	}
	if userID <= 0 {
		return Path{}, ErrBadUser
	}
	if s == nil || s.db == nil {
		return Path{}, errors.New("learner: nil database")
	}
	username, err := s.username(ctx, userID)
	if err != nil {
		return Path{}, err
	}
	grants, err := s.grants(ctx, userID)
	if err != nil {
		return Path{}, err
	}
	doc, err := s.loadPlan(ctx, username, planID, grants)
	if err != nil {
		return Path{}, err
	}
	finished, err := s.finishedSlugs(ctx, userID)
	if err != nil {
		return Path{}, err
	}
	due, err := s.countDue(ctx, userID, s.instant())
	if err != nil {
		return Path{}, err
	}
	return assemble(doc, finished, due, s.cfg), nil
}

func (s *Service) loadPlan(ctx context.Context, username, planID string, grants map[string]struct{}) (planDoc, error) {
	if planID != "" {
		row, err := s.planByID(ctx, planID)
		if err != nil {
			return planDoc{}, err
		}
		doc, ok, err := acceptPlan(row, username, grants)
		if err != nil {
			return planDoc{}, err
		}
		if !ok {
			return planDoc{}, ErrNotFound
		}
		return doc, nil
	}
	rows, err := s.assignedPlans(ctx, username)
	if err != nil {
		return planDoc{}, err
	}
	for _, row := range rows {
		doc, ok, err := acceptPlan(row, username, grants)
		if err != nil {
			return planDoc{}, err
		}
		if ok {
			return doc, nil
		}
	}
	return planDoc{}, ErrNoPlan
}

type planRow struct {
	id       string
	status   string
	audience string
	assignee sql.NullString
	snapshot sql.NullString
}

func (s *Service) planByID(ctx context.Context, planID string) (planRow, error) {
	var row planRow
	err := s.db.QueryRowContext(ctx,
		`SELECT id, status, audience, assignee_id, snapshot FROM plans WHERE id = ?`,
		planID,
	).Scan(&row.id, &row.status, &row.audience, &row.assignee, &row.snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return planRow{}, ErrNotFound
	}
	if err != nil {
		return planRow{}, fmt.Errorf("learner: plan %s: %w", planID, err)
	}
	return row, nil
}

func (s *Service) assignedPlans(ctx context.Context, username string) ([]planRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, status, audience, assignee_id, snapshot
		 FROM plans
		 WHERE status = 'published' AND audience = 'personal' AND assignee_id = ?
		 ORDER BY id ASC`,
		username,
	)
	if err != nil {
		return nil, fmt.Errorf("learner: assigned plans: %w", err)
	}
	defer rows.Close()
	var out []planRow
	for rows.Next() {
		var row planRow
		if err := rows.Scan(&row.id, &row.status, &row.audience, &row.assignee, &row.snapshot); err != nil {
			return nil, fmt.Errorf("learner: assigned plans: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("learner: assigned plans: %w", err)
	}
	return out, nil
}

func acceptPlan(row planRow, username string, grants map[string]struct{}) (planDoc, bool, error) {
	if row.status != "published" || !row.snapshot.Valid || row.snapshot.String == "" {
		return planDoc{}, false, nil
	}
	doc, err := parseSnapshot(row.id, row.snapshot.String)
	if err != nil {
		return planDoc{}, false, err
	}
	if doc.Status != "published" || doc.Audience != row.audience {
		return planDoc{}, false, nil
	}
	switch row.audience {
	case "personal":
		assignee := ""
		if row.assignee.Valid {
			assignee = row.assignee.String
		}
		if assignee == "" || assignee != username || doc.Assignee != username {
			return planDoc{}, false, nil
		}
	case "shared":
		if row.assignee.Valid && row.assignee.String != "" {
			return planDoc{}, false, nil
		}
		if doc.Assignee != "" || !sharedVisible(doc, grants) {
			return planDoc{}, false, nil
		}
	default:
		return planDoc{}, false, nil
	}
	doc.ID = row.id
	return doc, true, nil
}

func sharedVisible(doc planDoc, grants map[string]struct{}) bool {
	n := 0
	for _, unit := range doc.Units {
		for _, lesson := range unit.Lessons {
			n++
			switch lesson.Visibility {
			case "everyone":
			case "selected":
				if _, ok := grants[lesson.ID]; !ok {
					return false
				}
			default:
				return false
			}
		}
	}
	return n > 0
}

func (s *Service) username(ctx context.Context, userID int64) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT username FROM users WHERE id = ?`, userID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("learner: user: %w", err)
	}
	return name, nil
}

func (s *Service) grants(ctx context.Context, userID int64) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT slug FROM lesson_grants WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("learner: grants: %w", err)
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, fmt.Errorf("learner: grants: %w", err)
		}
		out[slug] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("learner: grants: %w", err)
	}
	return out, nil
}

func (s *Service) finishedSlugs(ctx context.Context, userID int64) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT slug, state_json FROM lesson_progress WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("learner: lesson progress: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var slug, state string
		if err := rows.Scan(&slug, &state); err != nil {
			return nil, fmt.Errorf("learner: lesson progress: %w", err)
		}
		if progressFinished(state) {
			out[slug] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("learner: lesson progress: %w", err)
	}
	return out, nil
}

func progressFinished(state string) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(state), &raw); err != nil {
		return false
	}
	return positiveNumber(raw["completedRounds"])
}

func ctxErr(ctx context.Context) error {
	if ctx == nil {
		return errors.New("learner: nil context")
	}
	return ctx.Err()
}
