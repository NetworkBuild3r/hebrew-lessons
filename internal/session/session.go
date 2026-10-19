package session

import (
	"context"
	"errors"
	"fmt"
	"slices"

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
// Next and Record update the cursor. Copying the value copies the cursor,
// so the caller keeps one Session and passes a pointer to helpers.
// The handler that built it has already checked ownership and CSRF.
type Session struct {
	cards    []CardRef
	i        int
	last     CardRef
	hasLast  bool
	requeued map[int64]struct{}
	grade    gradeFunc
}

// BuildSession queues at most SessionCap cards for userID.
// The caller has already checked that userID is the signed-in learner.
func (s *Service) BuildSession(ctx context.Context, userID int64) (Session, error) {
	if err := s.ready(); err != nil {
		return Session{}, err
	}
	now := s.clock.Now()
	cards, err := s.store.ListStudyCards(ctx, userID, now.Add(-ConfusionWindow))
	if err != nil {
		return Session{}, err
	}
	ratings, err := s.store.RecentReviewRatings(ctx, userID, now, RecentReviews)
	if err != nil {
		return Session{}, err
	}
	stats, err := s.store.StruggleStats(ctx, userID, 0)
	if err != nil {
		return Session{}, err
	}
	knownIDs, err := s.store.KnownLexemeIDs(ctx, userID)
	if err != nil {
		return Session{}, err
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
	return Session{
		cards: mix(cands, accuracyOf(ratings), now, s.seed),
		grade: func(ctx context.Context, cardID int64, rating fsrs.Rating) error {
			return review.GradeWithChoice(ctx, cardID, rating, false, "")
		},
	}, nil
}

// Next returns the next card. The bool is false when the queue is finished.
func (s *Session) Next() (CardRef, bool) {
	if s == nil || s.i >= len(s.cards) {
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
func (s *Session) Record(ctx context.Context, cardID int64, rating fsrs.Rating) error {
	if s == nil {
		return errors.New("session: session is not configured")
	}
	if !s.hasLast || cardID != s.last.CardID {
		return fmt.Errorf("session: card %d is not the card just served", cardID)
	}
	if s.grade == nil {
		return errors.New("session: review is not configured")
	}
	if err := s.grade(ctx, cardID, rating); err != nil {
		return err
	}
	s.hasLast = false
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
