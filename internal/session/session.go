package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

// Service loads a learner's cards and grades the answers.
// The clock and the seed are the only inputs that change the queue for a
// given database. New takes the clock the store was opened with.
type Service struct {
	store  *db.Store
	record *evidence.Recorder
	clock  clock.Clock
	seed   int64
	gate   func(ctx context.Context, userID int64) (activity.Gate, error)
	bands  learner.BandSource
}

// Option changes how New builds a Service.
type Option func(*Service)

// WithGate sets how the service learns which skills the learner can be asked through.
// The default opens every skill: no code decides the letters check yet (north-star
// section 4.1), so the default is what the mix did before the gate existed.
func WithGate(gate func(ctx context.Context, userID int64) (activity.Gate, error)) Option {
	return func(s *Service) { s.gate = gate }
}

// WithBands sets where the learner model gets placement bands. The default is the
// placement deck.
func WithBands(b learner.BandSource) Option {
	return func(s *Service) { s.bands = b }
}

// New builds a service. A nil clock uses the clock on store.
// seed breaks ties between cards that are due at the same time.
func New(store *db.Store, clk clock.Clock, seed int64, opts ...Option) *Service {
	if clk == nil && store != nil {
		clk = store.Clock()
	}
	s := &Service{
		store:  store,
		record: evidence.New(store, fsrs.New(store, clk), clk),
		clock:  clk,
		seed:   seed,
		gate: func(context.Context, int64) (activity.Gate, error) {
			return activity.OpenGate(), nil
		},
		bands: activity.PlacementBands(store),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Service) ready() error {
	if s == nil || s.store == nil {
		return errors.New("session: service is not configured")
	}
	if s.clock == nil {
		return errors.New("session: clock is not configured")
	}
	if s.record == nil {
		return errors.New("session: recorder is not configured")
	}
	return nil
}

// CardRef is one visit in the queue.
// Struggling is true when the card was selected as a struggling card.
// New is true when the card has no review yet. A new card is not a known
// card and not a struggling card. Kind is the card skill (read, recall, listen
// or spell) or letter or pattern. Key and Step are set on a drill. A drill's
// CardID is negative and is not an FSRS card.
//
// Activity is the activity the mix chose for the visit (activity.Kind) and AskSkill the
// skill it asks through. They are empty on a drill. CardID and Skill stay the card the
// mix selected, which is what a flashcard grades: AskSkill is the card's own skill only
// when the word's weakest skill is that one. A checked activity is built with
// activity.Engine.Build for LexemeID, answered with Check, and the visit is closed with
// Session.Answered. Flashcard is graded with Record, as before. Intro is true on a new
// card whose word has not been shown yet: the screen records Engine.Expose first.
type CardRef struct {
	CardID     int64
	LexemeID   int64
	Skill      string
	Struggling bool
	New        bool
	Kind       string
	Key        string
	Step       int
	Activity   activity.Kind
	AskSkill   string
	Intro      bool
}

type gradeFunc func(context.Context, int64, fsrs.Rating, bool) error

// ErrNotServed means Record or Reveal was given a card other than the one Next just served.
var ErrNotServed = errors.New("session: card is not the one just served")

// ErrNotRevealed means Record was asked to grade a card this serve has not revealed.
// The grade is not applied and no review_log row is written.
var ErrNotRevealed = errors.New("session: card was not revealed")

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
	peeked   bool
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
	knownIDs, err := s.store.KnownLexemeIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	gate, err := s.gate(ctx, userID)
	if err != nil {
		return nil, err
	}
	facts, err := s.factsByLexeme(ctx, userID)
	if err != nil {
		return nil, err
	}
	// The learner model reads at the same instant as everything above.
	model := learner.New(s.store, fixedClock{now}, learner.WithBands(&onceBands{src: s.bands}))
	pairs, err := model.ConfusionPairs(ctx, userID, ConfusionWindow)
	if err != nil {
		return nil, err
	}
	known := map[int64]struct{}{}
	for _, id := range knownIDs {
		known[id] = struct{}{}
	}
	words := map[int64]learner.WordState{}
	cands := make([]Candidate, 0, len(cards))
	for _, card := range cards {
		w, ok := words[card.LexemeID]
		if !ok {
			w, err = model.Word(ctx, userID, card.LexemeID)
			if err != nil {
				return nil, err
			}
			words[card.LexemeID] = w
		}
		_, isKnown := known[card.LexemeID]
		cands = append(cands, Candidate{
			ID:            card.ID,
			LexemeID:      card.LexemeID,
			Skill:         card.Skill,
			Due:           card.Due,
			Lapses:        card.Lapses,
			Latest:        card.LatestRating,
			HasRating:     card.HasRating,
			RecentLapse:   card.RecentLapse,
			Confused:      len(w.Review.ConfusionKeys) > 0,
			KnownLexeme:   isKnown,
			HasIntro:      card.HasIntro,
			IntroPosition: card.IntroPosition,
			IntroLesson:   card.IntroLesson,
			Skills:        activity.ViewsOf(w),
			Facts:         facts.of(card.LexemeID),
			Exposed:       w.Exposures > 0,
		})
	}
	cands = append(cands, drillCandidates(pairs)...)
	record := s.record
	return &Session{
		userID: userID,
		cards:  mixGate(cands, accuracyOf(ratings), now, s.seed, gate),
		grade: func(ctx context.Context, cardID int64, rating fsrs.Rating, _ bool) error {
			if rating < fsrs.Again || rating > fsrs.Easy {
				return fmt.Errorf("session: rating %d is outside 1..4", rating)
			}
			// A drill step has no FSRS card. The queue records the visit.
			if cardID <= 0 {
				return nil
			}
			return record.SelfGrade(ctx, cardID, rating)
		},
	}, nil
}

// drillCandidates is one drill per confusion key that reached the miss
// threshold. The steps are negative card ids, unique inside this build,
// so Record can tell them apart. They do not enter the known or struggle plan.
func drillCandidates(pairs []db.ConfusionPair) []Candidate {
	var out []Candidate
	n := 0
	for _, pair := range pairs {
		if pair.Count < MinMissesForDrill {
			continue
		}
		kind, ok := drillKind(pair.Kind)
		if !ok {
			continue
		}
		key := pair.A
		if pair.B != "" {
			key = pair.A + "/" + pair.B
		}
		for step := 0; step < DrillSteps; step++ {
			if len(out) >= DrillCap {
				return out
			}
			n++
			out = append(out, Candidate{
				ID:   -int64(n),
				Kind: kind,
				Key:  key,
				Step: step,
			})
		}
	}
	return out
}

func drillKind(confusionKind string) (string, bool) {
	switch confusionKind {
	case fsrs.MissLetterPair:
		return KindLetter, true
	case fsrs.MissAgreement:
		return KindPattern, true
	default:
		return "", false
	}
}

// Place reports the visit Next just returned.
// index is 1-based. total counts study cards and drill steps, including a
// re-queued visit. retry is true when this visit was inserted after Again.
func (s *Session) Place() (index, total int, retry bool) {
	if s == nil {
		return 0, 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	total = len(s.cards)
	if !s.hasLast {
		return 0, total, false
	}
	if s.requeued != nil {
		_, retry = s.requeued[s.last.CardID]
	}
	return s.i, total, retry
}

// Next returns the next card. The bool is false when the queue is finished.
// The new serve starts unrevealed. Peeked is set only by Reveal.
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
	s.peeked = false
	s.i++
	return s.last, true
}

// Reveal marks the served card as peeked. Record then stores that flag.
// A grade without Reveal is rejected with ErrNotRevealed.
func (s *Session) Reveal(cardID int64) error {
	if s == nil {
		return errors.New("session: session is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasLast || cardID != s.last.CardID {
		return fmt.Errorf("session: card %d: %w", cardID, ErrNotServed)
	}
	s.peeked = true
	return nil
}

// Record grades cardID through evidence.Recorder.SelfGrade, which writes the
// attempts row, the card update, and the review_log row together, and, on
// Again, re-queues it once. cardID is the card Next just returned. The handler has already
// checked ownership and CSRF. A second Again on the same card is graded
// and is not inserted again.
//
// peeked on the review_log row is true only when Reveal was called for this
// serve. A grade of an unrevealed card returns ErrNotRevealed. ApplyReview
// is not called and no review_log row is written. The serve stays current,
// so a later Reveal and Record can still grade it.
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
		return fmt.Errorf("session: card %d: %w", cardID, ErrNotServed)
	}
	if !s.peeked {
		return fmt.Errorf("session: card %d: %w", cardID, ErrNotRevealed)
	}
	if s.grade == nil {
		return errors.New("session: review is not configured")
	}
	peeked := s.peeked
	s.hasLast = false
	if err := s.grade(ctx, cardID, rating, peeked); err != nil {
		s.hasLast = true
		return err
	}
	s.peeked = false
	if rating != fsrs.Again {
		return nil
	}
	s.requeueLocked(cardID)
	return nil
}

// requeueLocked inserts the served card RequeueGap visits ahead, once per card.
// The caller holds mu.
func (s *Session) requeueLocked(cardID int64) {
	if s.requeued == nil {
		s.requeued = map[int64]struct{}{}
	}
	if _, done := s.requeued[cardID]; done {
		return
	}
	s.requeued[cardID] = struct{}{}
	at := s.i + RequeueGap
	if at > len(s.cards) {
		at = len(s.cards)
	}
	s.cards = slices.Insert(s.cards, at, s.last)
}

// ErrNotChecked means Answered was given a visit that has no checked activity: a flashcard
// or a drill step. Those are graded with Record.
var ErrNotChecked = errors.New("session: visit has no checked activity")

// Answered closes the visit of the card Next just served after the learner answered its
// checked activity. activity.Engine.Check has already written the evidence, so nothing is
// graded here. A wrong answer puts the visit back once, RequeueGap visits ahead, as an
// Again does for a flashcard. cardID is the card Next returned, and a second call for the
// same serve is refused with ErrNotServed.
func (s *Session) Answered(cardID int64, correct bool) error {
	if s == nil {
		return errors.New("session: session is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasLast || cardID != s.last.CardID {
		return fmt.Errorf("session: card %d: %w", cardID, ErrNotServed)
	}
	if s.last.Activity == "" || s.last.Activity == activity.Flashcard {
		return fmt.Errorf("session: card %d: %w", cardID, ErrNotChecked)
	}
	s.hasLast = false
	s.peeked = false
	if !correct {
		s.requeueLocked(cardID)
	}
	return nil
}
