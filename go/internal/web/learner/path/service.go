package path

import (
	"context"
	"database/sql"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/learner/config"
)

// DevUserID is the anonymous reader until sessions exist.
const DevUserID int64 = 1

// DemoPlanID is the fixture plan New returns until the learner service
// is imported. It is not part of the PR #37 API.
const DemoPlanID = "path-demo"

// Service is the stand-in for learner/path.Service.
// New(db, cfg) matches that constructor. db is stored and not read:
// this package has no database queries. Path and DueCount return the
// fixture course. After PR #37 merges, call
// learner/path.New(db, cfg) instead of this New.
type Service struct {
	db     *sql.DB
	cfg    config.Config
	now    func() time.Time
	loc    *time.Location
	course Path
	due    int
}

// New returns a service. A nil db is valid here; the learner constructor
// requires the read-only handle and will use it after the import swap.
func New(db *sql.DB, cfg config.Config) *Service {
	return &Service{
		db:     db,
		cfg:    cfg,
		now:    time.Now,
		course: DemoCourse(cfg, 9),
		due:    9,
	}
}

// Config returns the thresholds this service was built with.
func (s *Service) Config() config.Config {
	if s == nil {
		return config.Config{}
	}
	return s.cfg
}

// WithClock returns a copy that uses now. The fixture does not read it.
// The learner service uses it for DueCount and Path.
func (s *Service) WithClock(now func() time.Time) *Service {
	cp := *s
	cp.now = now
	return &cp
}

// WithLocation returns a copy that buckets due dates in loc.
// nil means the process local zone. The fixture does not read it.
func (s *Service) WithLocation(loc *time.Location) *Service {
	cp := *s
	cp.loc = loc
	return &cp
}

// UseFixture replaces the course and the due count. Tests use it.
// The learner service has no fixture hook; delete the call at the swap.
func (s *Service) UseFixture(course Path, due int) {
	s.course = course
	s.due = due
}

// DueCount is the review queue's due-row count.
func (s *Service) DueCount(ctx context.Context, userID int64) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if userID <= 0 {
		return 0, ErrBadUser
	}
	return s.due, nil
}

// Path returns the course line for userID.
// planID empty selects the assigned plan on the learner service.
// This stand-in returns the fixture course for every id.
func (s *Service) Path(ctx context.Context, userID int64, planID string) (Path, error) {
	if err := ctx.Err(); err != nil {
		return Path{}, err
	}
	if userID <= 0 {
		return Path{}, ErrBadUser
	}
	// db, now, and loc are the learner service's inputs. This stand-in
	// stores them and does not query.
	_ = s.db
	_ = s.loc
	if s.now != nil {
		_ = s.now()
	}
	course := s.course
	if planID != "" {
		course.PlanID = planID
	}
	return course, nil
}
