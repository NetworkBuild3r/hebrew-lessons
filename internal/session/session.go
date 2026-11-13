package session

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"slices"
	"sync"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
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

// WithSeed overrides the seed passed to New. Tests inject it so a queue can
// be built again. A review sitting passes its own seed to BuildSessionWithSeed.
func WithSeed(seed int64) Option {
	return func(s *Service) { s.seed = seed }
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

// Seed is the mix seed New or WithSeed stored. BuildSession uses it.
// A review sitting can pass a different seed to BuildSessionWithSeed.
func (s *Service) Seed() int64 {
	if s == nil {
		return 0
	}
	return s.seed
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
// mix selected. OwnActivity is the activity through the card's own skill; MovedUp is
// true when AskSkill is the implying skill (INIT-027/SPEC-004). A checked activity is
// built with activity.Engine.Build for LexemeID, answered with Check, and the visit is
// closed with Session.Answered. Flashcard is graded with Record, as before. Intro is
// true on a new card whose word has not been shown yet: the screen records Engine.Expose first.
type CardRef struct {
	CardID      int64
	LexemeID    int64
	Skill       string
	Struggling  bool
	New         bool
	Kind        string
	Key         string
	Step        int
	Activity    activity.Kind
	AskSkill    string
	Intro       bool
	MovedUp     bool
	OwnActivity activity.Kind
	// Source says whether the word is on the common-word course, came from the
	// learner's tutor, or came from somewhere else.
	Source Source
}

// Source says where a study card's word came from.
type Source string

const (
	// SourceCourse is a word on the common-word course list.
	SourceCourse Source = "course"
	// SourceTutor is a word that came from the learner's tutor, and not the course.
	SourceTutor Source = "tutor"
	// SourceOther is anything else: an earlier lesson, or a placement seed.
	SourceOther Source = "other"
)

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
// AnswerOutcome reports implied credit and a slow right Hard for a checked visit.
// INIT-027/SPEC-004.
type AnswerOutcome struct {
	Implied bool
	Slow    bool
}

// Tally is the sitting's counters. Updated under the session mutex by Record,
// Answered, and DrillAnswered. NewMet counts visits with CardRef.New when Next
// serves them (INIT-027/SPEC-004).
type Tally struct {
	Answered     int
	Right        int
	MovedUp      int
	PairsCleared int
	Slow         int
	NewMet       int
}

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
	span     int
	store    *db.Store
	clock    clock.Clock
	record   *evidence.Recorder
	tally    Tally
	minutes  int
	// SkippedNoAudio is how many due listening cards this sitting left out
	// because the word has no clip (ADR-0028 D6). They stay due.
	SkippedNoAudio int
}

// Minutes is the session's planned length. SPEC-011 shows it on Today.
func (s *Session) Minutes() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.minutes
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
// now is read once and passed to every load, including the learner model.
// The caller checks that userID is the signed-in learner.
func (s *Service) BuildSession(ctx context.Context, userID int64) (*Session, error) {
	return s.build(ctx, userID, nil, s.seed)
}

// BuildSessionWithSeed is BuildSession with the sitting's mix seed.
// The same seed rebuilds the same queue. BuildSession uses the seed from New.
func (s *Service) BuildSessionWithSeed(ctx context.Context, userID int64, seed int64) (*Session, error) {
	return s.build(ctx, userID, nil, seed)
}

// BuildSessionFor is BuildSession for a review set: only the cards of the given words are
// queued, and no letter or pattern drill is woven in. The mix is the same, so the cap, the
// known and struggle shares and the activity choice still apply. A word with no card
// leaves nothing to queue, and an empty set builds an empty session. This is what the
// band-slip alert on Today links to (INIT-015/SPEC-017).
func (s *Service) BuildSessionFor(ctx context.Context, userID int64, lexemeIDs []int64) (*Session, error) {
	only := make(map[int64]struct{}, len(lexemeIDs))
	for _, id := range lexemeIDs {
		only[id] = struct{}{}
	}
	return s.build(ctx, userID, only, s.seed)
}

// sourceOf says where a word came from. The course wins over a tutor sheet,
// because the course is what ranks the word.
func sourceOf(lexemeID int64, courseRank map[int64]int, tutor map[int64]bool) Source {
	if courseRank[lexemeID] > 0 {
		return SourceCourse
	}
	if tutor[lexemeID] {
		return SourceTutor
	}
	return SourceOther
}

// tutorLexemeIDs is the stored lexemes whose words appear on one of this
// learner's class sheets.
func (s *Service) tutorLexemeIDs(ctx context.Context, userID int64) (map[int64]bool, error) {
	sheets, err := s.store.ClassSheets(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := map[int64]bool{}
	if len(sheets) == 0 {
		return out, nil
	}
	spellings, err := s.store.LexemeIDsBySpelling(ctx)
	if err != nil {
		return nil, err
	}
	for _, sheet := range sheets {
		for _, line := range sheet.Lines {
			if line.Held || line.Tutor == "" {
				continue
			}
			for _, tok := range units.Tokenize(line.Tutor) {
				for _, id := range spellings[units.StripNiqqud(units.Normalize(tok))] {
					out[id] = true
				}
			}
		}
	}
	return out, nil
}

// build is BuildSession. only, when not nil, keeps the cards of those words and no drill.
// seed is the sitting seed: it orders equal due times, places new cards, and
// chooses the miss gap.
func (s *Service) build(ctx context.Context, userID int64, only map[int64]struct{}, seed int64) (*Session, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	cardsN, minutes, err := s.sessionBudget(ctx, userID)
	if err != nil {
		return nil, err
	}
	zone, err := s.store.StudyZone(ctx, userID)
	if err != nil {
		return nil, err
	}
	ratings, err := s.store.RecentReviewRatings(ctx, userID, now, RecentReviews)
	if err != nil {
		return nil, err
	}
	accuracy := accuracyOf(ratings)
	gate, err := s.gate(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Full builds seed placement-list words before ListStudyCards so the
	// new cards join this sitting (INIT-027/SPEC-005). BuildSessionFor skips it.
	if only == nil {
		if err := s.seedNewFromDeck(ctx, userID, accuracy, now, gate); err != nil {
			return nil, err
		}
		// Introduce dues cards at the recorder clock's now; pin the build to
		// that same instant so the seeded cards classify as due new.
		now = s.clock.Now()
	}
	cards, err := s.store.ListStudyCards(ctx, userID, now, now.Add(-ConfusionWindow))
	if err != nil {
		return nil, err
	}
	if only != nil {
		cards = slices.DeleteFunc(cards, func(c db.StudyCard) bool {
			_, keep := only[c.LexemeID]
			return !keep
		})
	}
	knownIDs, err := s.store.KnownLexemeIDs(ctx, userID)
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
	courseRows, err := s.store.CourseLexemes(ctx)
	if err != nil {
		return nil, err
	}
	courseRank := map[int64]int{}
	for _, row := range courseRows {
		courseRank[row.LexemeID] = row.Rank
	}
	tutorIDs, err := s.tutorLexemeIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	byLexSkill := map[lexemeSkill]db.StudyCard{}
	for _, card := range cards {
		byLexSkill[lexemeSkill{card.LexemeID, card.Skill}] = card
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
		var implyID int64
		var implyBlocked bool
		if imply, ok := activity.Implies(card.Skill); ok {
			if other, exists := byLexSkill[lexemeSkill{card.LexemeID, imply}]; exists {
				implyID = other.ID
				implyBlocked = other.RecentAgainOrHard
			}
		}
		cands = append(cands, Candidate{
			ID:                        card.ID,
			LexemeID:                  card.LexemeID,
			Skill:                     card.Skill,
			Due:                       studyday.EffectiveDue(card.Due, card.LastReview, zone),
			Lapses:                    card.Lapses,
			Latest:                    card.LatestRating,
			HasRating:                 card.HasRating,
			RecentLapse:               card.RecentLapse,
			Confused:                  len(w.Review.ConfusionKeys) > 0,
			KnownLexeme:               isKnown,
			HasIntro:                  card.HasIntro,
			IntroPosition:             card.IntroPosition,
			IntroLesson:               card.IntroLesson,
			HasFreq:                   courseRank[card.LexemeID] > 0,
			FreqRank:                  courseRank[card.LexemeID],
			Source:                    sourceOf(card.LexemeID, courseRank, tutorIDs),
			Skills:                    activity.ViewsOf(w),
			Facts:                     facts.of(card.LexemeID),
			Exposed:                   w.Exposures > 0,
			ImplyingCardID:            implyID,
			ImplyingRecentAgainOrHard: implyBlocked,
		})
	}
	if only == nil {
		cands = append(cands, drillCandidates(pairs)...)
	}
	record := s.record
	queue, skippedNoAudio := mixGateCap(cands, accuracy, now, seed, gate, cardsN)
	sess := &Session{
		userID:         userID,
		cards:          queue,
		span:           requeueSpan(seed),
		store:          s.store,
		clock:          s.clock,
		record:         record,
		minutes:        minutes,
		SkippedNoAudio: skippedNoAudio,
	}
	sess.grade = func(ctx context.Context, cardID int64, rating fsrs.Rating, _ bool) error {
		if rating < fsrs.Again || rating > fsrs.Easy {
			return fmt.Errorf("session: rating %d is outside 1..4", rating)
		}
		// A drill step has no FSRS card. Grade it into drill_answers
		// (INIT-027/SPEC-003). Restoring `return nil` here fails the
		// pattern-Good test.
		if cardID <= 0 {
			_, err := sess.recordDrillAnswerLocked(ctx, rating >= fsrs.Good, 0)
			return err
		}
		return record.SelfGrade(ctx, cardID, rating)
	}
	return sess, nil
}

// drillCandidates is one drill per confusion key that reached the miss
// threshold. The steps are negative card ids, stable for a kind, key, and
// step, and unique inside this build, so Record can tell them apart and two
// pairs do not share the id the letter picker seeds from. They do not enter
// the known or struggle plan.
func drillCandidates(pairs []db.ConfusionPair) []Candidate {
	var out []Candidate
	used := map[int64]struct{}{}
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
			out = append(out, Candidate{
				ID:   drillCardID(kind, key, step, used),
				Kind: kind,
				Key:  key,
				Step: step,
			})
		}
	}
	return out
}

// drillCardID is a negative id that stays the same for this kind, key, and
// step across queue rebuilds. Sequential ids restarted at -1, so every pair's
// first step rendered data-card="-1" and the letter picker chose the same side.
func drillCardID(kind, key string, step int, used map[int64]struct{}) int64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s\x00%s\x00%d", kind, key, step)
	id := -int64(h.Sum64() & 0x7fffffff)
	if id == 0 {
		id = -1
	}
	for {
		if _, taken := used[id]; !taken {
			used[id] = struct{}{}
			return id
		}
		id--
		if id == 0 {
			id = -1
		}
	}
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
// NewMet on Tally counts a visit with CardRef.New when it is served here.
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
	if s.last.New {
		s.tally.NewMet++
	}
	return s.last, true
}

// Reveal marks the served card as peeked. Record then stores that flag.
// A grade without Reveal is rejected with ErrNotRevealed.
// The mark does not depend on the card type. Every review mode uses it.
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
// Again, re-queues it once, unless the queue is a single lexeme. cardID is the card Next just returned. The handler has already
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
	// The grade guard is this peeked flag. It does not look at the card kind.
	// A new review mode does not add a guard.
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
	s.tally.Answered++
	if rating > fsrs.Again {
		s.tally.Right++
	}
	if rating == fsrs.Hard {
		s.tally.Slow++
	}
	// A miss is scheduled and the sitting moves on. It is not asked again
	// in this sitting (INIT-028/SPEC-002, ADR-0024 D3).
	return nil
}

// requeueSalt mixes the sitting seed into the miss gap. It is not a card id.
const requeueSalt int64 = 0x524551

// requeueSpan is how many other visits sit between a miss and its one
// requeue: RequeueGap, one more, or two more. The same seed always returns
// the same span.
func requeueSpan(seed int64) int {
	return RequeueGap + int(tieBreak(seed, requeueSalt)%3)
}

// requeueLocked inserts the served card span visits ahead, once per card.
// span is 3, 4, or 5. The insert is clamped to the end of the queue.
// A queue of one study lexeme is not requeued: the same word would be the
// next Continue. The caller holds mu.
func (s *Session) requeueLocked(cardID int64) {
	s.requeueRefLocked(cardID, s.last)
}

// requeueRefLocked is requeueLocked with an explicit visit copy (INIT-027/SPEC-004
// uses it so a wrong moved-up answer comes back on the own skill).
func (s *Session) requeueRefLocked(cardID int64, ref CardRef) {
	if s.requeued == nil {
		s.requeued = map[int64]struct{}{}
	}
	if _, done := s.requeued[cardID]; done {
		return
	}
	if s.oneLexemeLocked() {
		return
	}
	s.requeued[cardID] = struct{}{}
	at := s.i + s.span
	if at > len(s.cards) {
		at = len(s.cards)
	}
	s.cards = slices.Insert(s.cards, at, ref)
}

// oneLexemeLocked reports whether every study visit in the queue is the same
// lexeme. Drills are not study visits. The caller holds mu.
func (s *Session) oneLexemeLocked() bool {
	// A drill step is not the word looping. It is requeued as before.
	if s.last.LexemeID == 0 || s.last.Kind == KindLetter || s.last.Kind == KindPattern {
		return false
	}
	var id int64
	seen := false
	for _, c := range s.cards {
		if c.LexemeID == 0 || c.Kind == KindLetter || c.Kind == KindPattern {
			continue
		}
		if !seen {
			id = c.LexemeID
			seen = true
			continue
		}
		if c.LexemeID != id {
			return false
		}
	}
	return seen
}

// ErrNotChecked means Answered was given a visit that has no checked activity: a flashcard
// or a drill step. Those are graded with Record.
var ErrNotChecked = errors.New("session: visit has no checked activity")

// Answered closes the visit of the card Next just served after the learner answered its
// checked activity. activity.Engine.Check has already written the evidence for the asked
// skill. When the visit was MovedUp and the answer is Correct with Good or Easy, Implied
// schedules the own card as Good. A wrong answer is not asked again in this sitting.
// INIT-027/SPEC-004, INIT-028/SPEC-002.
func (s *Session) Answered(ctx context.Context, cardID int64, res activity.Result) (AnswerOutcome, error) {
	if s == nil {
		return AnswerOutcome{}, errors.New("session: session is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasLast || cardID != s.last.CardID {
		return AnswerOutcome{}, fmt.Errorf("session: card %d: %w", cardID, ErrNotServed)
	}
	if s.last.Activity == "" || s.last.Activity == activity.Flashcard {
		return AnswerOutcome{}, fmt.Errorf("session: card %d: %w", cardID, ErrNotChecked)
	}
	out := AnswerOutcome{
		Slow: res.Correct && res.Rating == fsrs.Hard && !res.NearMiss,
	}
	s.hasLast = false
	s.peeked = false
	s.tally.Answered++
	if res.Correct {
		s.tally.Right++
	}
	if out.Slow {
		s.tally.Slow++
	}
	if s.last.MovedUp && res.Correct && (res.Rating == fsrs.Good || res.Rating == fsrs.Easy) {
		if s.record == nil {
			s.hasLast = true
			return AnswerOutcome{}, errors.New("session: recorder is not configured")
		}
		if err := s.record.Implied(ctx, s.userID, cardID, fsrs.Good); err != nil {
			s.hasLast = true
			s.tally.Answered--
			if res.Correct {
				s.tally.Right--
			}
			if out.Slow {
				s.tally.Slow--
			}
			return AnswerOutcome{}, err
		}
		out.Implied = true
		s.tally.MovedUp++
	}
	// A wrong answer, including a moved-up one, is not put back into this
	// sitting (INIT-028/SPEC-002, ADR-0024 D3).
	return out, nil
}

// UseChecked marks the served visit as a checked activity when the screen
// could build one and the mix had left a flashcard. INIT-028/SPEC-002.
func (s *Session) UseChecked(cardID int64, kind activity.Kind, skill string) error {
	if s == nil {
		return errors.New("session: session is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasLast || cardID != s.last.CardID {
		return fmt.Errorf("session: card %d: %w", cardID, ErrNotServed)
	}
	if kind == "" || kind == activity.Flashcard {
		return fmt.Errorf("session: card %d: %w", cardID, ErrNotChecked)
	}
	s.last.Activity = kind
	if skill != "" {
		s.last.AskSkill = skill
	}
	return nil
}

// Tally returns a copy of the sitting counters. INIT-027/SPEC-004.
func (s *Session) Tally() Tally {
	if s == nil {
		return Tally{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tally
}
