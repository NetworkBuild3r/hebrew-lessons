package mcp

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"
)

// Shared pieces of the learner tools (INIT-015/SPEC-021): who the tool is for, and how
// often a caller may ask.

const (
	// learnerToolsPerMinute is the calls a caller may make to the learner tools in one
	// minute. The server had no per-token limiter, so SPEC-021 aud-1 adds this one. It
	// counts get_learner_model, get_progress, get_lesson_brief, add_coach_note,
	// unpublish_unit, and validate_unit with a briefId. Each of the first four reads the
	// whole learner model or writes a row.
	learnerToolsPerMinute = 60
	rateWindow            = time.Minute
	// maxRateKeys bounds the limiter's memory. A window that has passed is dropped first.
	maxRateKeys = 4096

	// maxCoachNotesPerDay is db.MaxCoachNotesPerDay. A test in mcpstore keeps them equal.
	maxCoachNotesPerDay = 100
	// maxCoachNoteRunes is the longest note body, as coach_notes.body allows.
	maxCoachNoteRunes = 500
	// maxAuthorRunes is the longest note author, as coach_notes.author allows.
	maxAuthorRunes = 100
	// maxThemeRunes is the longest brief theme, as lesson_briefs.theme allows.
	maxThemeRunes = 200
	// maxCursorLen bounds a page cursor before it reaches the store.
	maxCursorLen = 256
	// maxProgressDays is the longest range get_progress reads.
	maxProgressDays = 366
	// defaultProgressDays is the range when from is left out.
	defaultProgressDays = 30
	// contextNotes is how many coach notes get_context returns.
	contextNotes = 5
)

const (
	msgRateLimited     = "Too many requests. Try again in a minute."
	msgNoLearnerStore  = "The learner data is not connected."
	msgLearnerRequired = "learnerId is required for an admin or the static token."
	msgLearnerBad      = "learnerId must be a positive learner id."
	msgNoSuchLearner   = "No learner has that id."
	// msgNoUnitInScope is the one answer for a unit that is missing, shared, or another
	// learner's, so the reply does not say which.
	msgNoUnitInScope = "No learner-scoped unit with that id was found for this learner."
)

// limiter is a fixed-window counter per caller.
type limiter struct {
	mu      sync.Mutex
	now     func() time.Time
	max     int
	window  time.Duration
	buckets map[string]rateBucket
}

type rateBucket struct {
	start time.Time
	n     int
}

func newLimiter(now func() time.Time, max int, window time.Duration) *limiter {
	return &limiter{now: now, max: max, window: window, buckets: map[string]rateBucket{}}
}

// allow counts one call for key and reports whether it is within the limit.
func (l *limiter) allow(key string) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[key]
	if !ok || now.Sub(b.start) >= l.window {
		if !ok && len(l.buckets) >= maxRateKeys {
			for k, old := range l.buckets {
				if now.Sub(old.start) >= l.window {
					delete(l.buckets, k)
				}
			}
		}
		b = rateBucket{start: now}
	}
	if b.n >= l.max {
		l.buckets[key] = b
		return false
	}
	b.n++
	l.buckets[key] = b
	return true
}

// rateKey names a caller. The bearer token is not on the principal, so the caller is the
// actor, id, and role the validator gave it. The static token is one caller.
func rateKey(p Principal) string {
	return p.Role + "|" + p.Source + "|" + p.Actor + "|" + strconv.FormatInt(p.UserID, 10)
}

// limit counts a call for the caller and returns the fixed rate-limit error when over.
func (s *service) limit(p Principal) error {
	if !s.limiter.allow(rateKey(p)) {
		return errors.New(msgRateLimited)
	}
	return nil
}

// learnerFor is the learner a tool acts for. It is the only place a tool gets a learner id.
//
//   - A learner principal gets its own id. A requested id that differs is refused with the
//     same text get_context uses. The role middleware has already refused it, and this is
//     the second check for a tool that is called some other way.
//   - An admin, including the static token, must name a learner, and the learner must exist.
//   - Any other role has no learner.
func (s *service) learnerFor(ctx context.Context, p Principal, requested *int64) (int64, error) {
	switch p.Role {
	case RoleLearner:
		if p.UserID <= 0 {
			return 0, errors.New(denialMessage())
		}
		if requested != nil && *requested != p.UserID {
			return 0, errors.New(msgOtherLearner)
		}
		return p.UserID, nil
	case RoleAdmin:
		if requested == nil {
			return 0, errors.New(msgLearnerRequired)
		}
		if *requested <= 0 {
			return 0, errors.New(msgLearnerBad)
		}
		// A store with no learner data (validate_unit only reads a brief) cannot look the
		// id up. A brief of an id that does not exist is not found either way.
		if s.learner != nil {
			ok, err := s.learner.LearnerExists(ctx, *requested)
			if err != nil {
				return 0, storeErr("The learner could not be looked up.", err)
			}
			if !ok {
				return 0, errors.New(msgNoSuchLearner)
			}
		}
		return *requested, nil
	default:
		return 0, errors.New(denialMessage())
	}
}

// learnerTool is the start of every learner tool: the role gate, the rate limit, the store,
// and the learner. Nothing past it runs for a caller that fails any of them.
func (s *service) learnerTool(ctx context.Context, tool string, requested *int64) (Principal, int64, LearnerData, error) {
	p, err := s.gate(ctx, tool)
	if err != nil {
		return p, 0, nil, err
	}
	if err := s.limit(p); err != nil {
		return p, 0, nil, err
	}
	if s.learner == nil {
		return p, 0, nil, errors.New(msgNoLearnerStore)
	}
	id, err := s.learnerFor(ctx, p, requested)
	if err != nil {
		return p, 0, nil, err
	}
	return p, id, s.learner, nil
}
