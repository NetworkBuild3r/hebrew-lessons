package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// Service loads a learner's cards and grades the answers.
// The clock and the seed are the only inputs that change the queue for a
// given database. New takes the clock the store was opened with.
type Service struct {
	store  *db.Store
	review *fsrs.Review
	clock  clock.Clock
	seed   int64
}

// New builds a service. A nil clock uses the clock on store.
// seed breaks ties between cards that are due at the same time.
func New(store *db.Store, clk clock.Clock, seed int64) *Service {
	if clk == nil && store != nil {
		clk = store.Clock()
	}
	return &Service{
		store:  store,
		review: fsrs.New(store, clk),
		clock:  clk,
		seed:   seed,
	}
}

func (s *Service) ready() error {
	if s == nil || s.store == nil {
		return errors.New("session: service is not configured")
	}
	if s.clock == nil {
		return errors.New("session: clock is not configured")
	}
	if s.review == nil {
		return errors.New("session: review is not configured")
	}
	return nil
}

// CardRef is one visit in the queue.
// Struggling is true when the card was selected as a struggling card.
type CardRef struct {
	CardID     int64
	LexemeID   int64
	Dir        string
	Struggling bool
}

type gradeFunc func(context.Context, int64, fsrs.Rating) error

// Session is one learner's queue, held in memory.
// Next and Record share mu. BuildSession returns a pointer because a mutex
// must not be copied. The handler that built it checks UserID against the
// signed-in learner, and it checks CSRF on every mutating request.
type Session struct {
	mu       sync.Mutex
	userID   int64
	cards    []CardRef
	i        int
	last     CardRef
	hasLast  bool
	requeued map[int64]struct{}
	grade    gradeFunc
}

// UserID is the learner this queue was built for.
func (s *Session) UserID() int64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.userID
}

// BuildSession queues at most SessionCap cards for userID.
// now is read once and passed to every load, including StruggleStats.
// The caller checks that userID is the signed-in learner.
func (s *Service) BuildSession(ctx context.Context, userID int64) (*Session, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	cards, err := s.store.ListStudyCards(ctx, userID, now, now.Add(-ConfusionWindow))
	if err != nil {
		return nil, err
	}
	ratings, err := s.store.RecentReviewRatings(ctx, userID, now, RecentReviews)
	if err != nil {
		return nil, err
	}
	stats, err := s.store.StruggleStatsAt(ctx, userID, 0, now)
	if err != nil {
		return nil, err
	}
	knownIDs, err := s.store.KnownLexemeIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	confused := map[int64]struct{}{}
	for _, lex := range stats.Lexemes {
		if len(lex.ConfusionKeys) > 0 {
			confused[lex.LexemeID] = struct{}{}
		}
	}
	known := map[int64]struct{}{}
	for _, id := range knownIDs {
		known[id] = struct{}{}
	}
	cands := make([]Candidate, 0, len(cards))
	for _, card := range cards {
		_, isKnown := known[card.LexemeID]
		_, isConfused := confused[card.LexemeID]
		cands = append(cands, Candidate{
			ID:          card.ID,
			LexemeID:    card.LexemeID,
			Dir:         card.Dir,
			Due:         card.Due,
			Lapses:      card.Lapses,
			Latest:      card.LatestRating,
			HasRating:   card.HasRating,
			RecentLapse: card.RecentLapse,
			Confused:    isConfused,
			KnownLexeme: isKnown,
		})
	}
	review := s.review
	return &Session{
		userID: userID,
		cards:  mix(cands, accuracyOf(ratings), now, s.seed),
		grade: func(ctx context.Context, cardID int64, rating fsrs.Rating) error {
			return review.GradeWithChoice(ctx, cardID, rating, false, "")
		},
	}, nil
}

// Next returns the next card. The bool is false when the queue is finished.
func (s *Session) Next() (CardRef, bool) {
	if s == nil {
		return CardRef{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.i >= len(s.cards) {
		return CardRef{}, false
	}
	s.last = s.cards[s.i]
	s.hasLast = true
	s.i++
	return s.last, true
}

// Record grades cardID through GradeWithChoice and, on Again, re-queues it
// once. cardID is the card Next just returned. The handler has already
// checked ownership and CSRF. A second Again on the same card is graded
// and is not inserted again.
//
// The served card is marked graded before the write. The mark is put back
// only when the write fails, so two Records of the same serve cannot both
// append a review_log row. mu is held across the write: the store is one
// SQLite connection, and the re-queue edits the same slice Next reads.
func (s *Session) Record(ctx context.Context, cardID int64, rating fsrs.Rating) error {
	if s == nil {
		return errors.New("session: session is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasLast || cardID != s.last.CardID {
		return fmt.Errorf("session: card %d is not the card just served", cardID)
	}
	if s.grade == nil {
		return errors.New("session: review is not configured")
	}
	s.hasLast = false
	if err := s.grade(ctx, cardID, rating); err != nil {
		s.hasLast = true
		return err
	}
	if rating != fsrs.Again {
		return nil
	}
	if s.requeued == nil {
		s.requeued = map[int64]struct{}{}
	}
	if _, done := s.requeued[cardID]; done {
		return nil
	}
	s.requeued[cardID] = struct{}{}
	at := s.i + RequeueGap
	if at > len(s.cards) {
		at = len(s.cards)
	}
	s.cards = slices.Insert(s.cards, at, s.last)
	return nil
}
