package evidence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"

	"golang.org/x/text/unicode/norm"
)

// Sources an attempt can come from. They match the attempts.source check.
const (
	SourcePlacement = "placement"
	SourceReview    = "review"
	SourceLesson    = "lesson"
	SourcePractice  = "practice"
)

// ActivityFlashcard is the activity SelfGrade writes.
const ActivityFlashcard = "flashcard"

// Contexts an exposure can come from. They match the exposures.context check.
const (
	ContextStory           = "story"
	ContextDialogue        = "dialogue"
	ContextGlossTap        = "gloss_tap"
	ContextPlacementReview = "placement_review"
	ContextLessonIntro     = "lesson_intro"
	ContextLessonListen    = "lesson_listen"
)

const (
	// MaxResponseRunes is the most characters of a response that are stored.
	MaxResponseRunes = 200

	// maxPromptRunes bounds the prompt, which the server writes.
	maxPromptRunes = 500
)

// Attempt is one checked answer, as a screen reports it.
//
// UserID is the signed-in learner. Handlers take it from the auth session and
// never from form input. Latency is the time from showing the question to the
// answer. Zero means the screen did not time it, and the attempt is never Easy.
// ChosenLexemeID is the word the learner picked when a wrong pick names another
// word, or 0. SkipSchedule stores the attempt and leaves every card alone.
//
// NearMiss marks a right answer that was one letter off, which the typed grader
// rates Hard (north-star section 6). Confusions are further confusion rows to write
// with the attempt, for example the letter pairs found by aligning a typed answer.
type Attempt struct {
	UserID         int64
	LexemeID       int64
	Skill          string
	Activity       string
	Source         string
	UnitID         int64
	Prompt         string
	Response       string
	Correct        bool
	DontKnow       bool
	HintUsed       bool
	Latency        time.Duration
	ChosenLexemeID int64
	SkipSchedule   bool
	NearMiss       bool
	Confusions     []Confusion
}

// Confusion is one confusion_misses row to write with an attempt.
// Kind is letter_pair, agreement, or other. Key is the pair.
type Confusion = db.Confusion

const (
	// MaxConfusions is the most extra confusion rows one attempt may carry.
	MaxConfusions = 8

	// maxConfusionKeyRunes bounds a confusion key.
	maxConfusionKeyRunes = 200
)

// Recorder writes checked answers. It is safe for use by several goroutines:
// the store has one connection and each write is one transaction.
type Recorder struct {
	store  *db.Store
	review *fsrs.Review
	clock  clock.Clock
	// exposureSink, when set, receives Expose's write instead of the store.
	// Production leaves it nil. A test sets it for a context the exposures
	// CHECK does not allow yet.
	exposureSink interface {
		InsertExposures(ctx context.Context, userID int64, lexemeIDs []int64, where string, unitID int64, at time.Time) error
	}
}

// New builds a recorder. A nil clk uses the clock the store was opened with.
// A nil review is built on that same clock.
func New(store *db.Store, review *fsrs.Review, clk clock.Clock) *Recorder {
	if clk == nil && store != nil {
		clk = store.Clock()
	}
	if review == nil {
		review = fsrs.New(store, clk)
	}
	return &Recorder{store: store, review: review, clock: clk}
}

func (r *Recorder) ready() error {
	if r == nil || r.store == nil {
		return errors.New("evidence: recorder is not configured")
	}
	if r.clock == nil {
		return errors.New("evidence: clock is not configured")
	}
	if r.review == nil {
		return errors.New("evidence: review is not configured")
	}
	return nil
}

// Attempt stores one checked answer and returns its rating.
//
// In one transaction it writes the attempts row, applies the rating to the
// learner's card for the word and skill (creating the card if there is none),
// appends the review_log row, and writes a confusion_misses row when a wrong
// pick names another word (a letter_pair when it is one letter off, otherwise
// kind other). If any write fails,
// nothing is stored.
//
// A placement attempt follows ADR-0018 D2: only a right answer seeds a card,
// as Good, and only when the word has no card yet. A wrong answer or "I don't
// know" is stored with no card. The returned rating is Good for a right
// placement answer and Again otherwise.
//
// SkipSchedule stores the attempt and leaves every card alone. The lesson
// player uses it: its new words get their cards when the lesson ends, as in v2.
//
// An unknown skill or source, an empty activity, a missing learner or word, a
// negative latency, or an answer that is both right and "I don't know" is
// refused and nothing is written.
func (r *Recorder) Attempt(ctx context.Context, a Attempt) (fsrs.Rating, error) {
	if err := r.ready(); err != nil {
		return 0, err
	}
	if err := a.validate(); err != nil {
		return 0, err
	}
	now := r.clock.Now()

	latencies, err := r.store.CorrectLatencies(ctx, a.UserID, a.Activity, MedianWindow)
	if err != nil {
		return 0, err
	}
	rating := Grade(a, Median(latencies))

	seedOnly := a.Source == SourcePlacement
	if seedOnly {
		rating = fsrs.Again
		if a.Correct && !a.DontKnow {
			rating = fsrs.Good
		}
	}

	chosenText, confusion, err := r.confusionFor(ctx, a)
	if err != nil {
		return 0, err
	}

	var step db.CardStep
	switch {
	case a.SkipSchedule:
	case seedOnly && rating != fsrs.Good:
	case seedOnly:
		// D2: seed a card for a right answer, and never re-review an existing one.
		step = func(prev *db.Card) (db.Card, db.ReviewLog, bool, error) {
			if prev != nil {
				return db.Card{}, db.ReviewLog{}, false, nil
			}
			card, log, err := r.review.ScheduleNext(nil, now, rating)
			return card, log, err == nil, err
		}
	default:
		step = func(prev *db.Card) (db.Card, db.ReviewLog, bool, error) {
			card, log, err := r.review.ScheduleNext(prev, now, rating)
			log.Chosen = chosenText
			return card, log, err == nil, err
		}
	}

	_, err = r.store.WriteAttempt(ctx, db.AttemptWrite{
		Attempt:   a.row(now),
		Step:      step,
		Confusion: confusion,
		Extra:     a.Confusions,
	})
	if err != nil {
		return 0, err
	}
	return rating, nil
}

// SelfGrade records a flashcard the learner graded themselves.
// rating is 1 Again, 2 Hard, 3 Good, or 4 Easy, and is applied as given. The
// attempt has activity flashcard, source review, the card's skill, and
// correct = rating > Again. The review_log row is marked peeked, because a
// flashcard is graded after the answer is shown.
// The caller has checked that the learner owns cardID.
func (r *Recorder) SelfGrade(ctx context.Context, cardID int64, rating fsrs.Rating) error {
	if err := r.ready(); err != nil {
		return err
	}
	if rating < fsrs.Again || rating > fsrs.Easy {
		return fmt.Errorf("evidence: rating %d is outside 1..4", rating)
	}
	card, err := r.store.GetCard(ctx, cardID)
	if err != nil {
		return err
	}
	now := r.clock.Now()
	prompt := ""
	face, ok, err := r.store.LexemeByID(ctx, card.LexemeID)
	if err != nil {
		return err
	}
	if ok {
		prompt = face.TextNFC
		if card.Skill == fsrs.SkillRecall {
			prompt = face.Gloss
		}
	}
	a := Attempt{
		UserID:   card.UserID,
		LexemeID: card.LexemeID,
		Skill:    card.Skill,
		Activity: ActivityFlashcard,
		Source:   SourceReview,
		Prompt:   prompt,
		Response: selfGradeLabel(rating),
		Correct:  rating > fsrs.Again,
	}
	_, err = r.store.WriteAttempt(ctx, db.AttemptWrite{
		Attempt: a.row(now),
		Step: func(prev *db.Card) (db.Card, db.ReviewLog, bool, error) {
			next, log, err := r.review.ScheduleNext(prev, now, rating)
			log.Peeked = true
			return next, log, err == nil, err
		},
	})
	return err
}

// Expose records that words were shown to the learner with no question asked.
// where is one of the Context constants. unitID 0 means no unit. A word listed
// twice is recorded once. An empty list writes nothing.
func (r *Recorder) Expose(ctx context.Context, learnerID int64, lexemeIDs []int64, where string, unitID int64) error {
	if err := r.ready(); err != nil {
		return err
	}
	if learnerID <= 0 {
		return errors.New("evidence: exposure has no learner")
	}
	switch where {
	case ContextStory, ContextDialogue, ContextGlossTap, ContextPlacementReview, ContextLessonIntro, ContextLessonListen:
	default:
		return fmt.Errorf("evidence: exposure context %q is unknown", where)
	}
	seen := make(map[int64]struct{}, len(lexemeIDs))
	ids := make([]int64, 0, len(lexemeIDs))
	for _, id := range lexemeIDs {
		if id <= 0 {
			return fmt.Errorf("evidence: exposure lexeme id %d is not valid", id)
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if r.exposureSink != nil {
		return r.exposureSink.InsertExposures(ctx, learnerID, ids, where, unitID, r.clock.Now())
	}
	return r.store.InsertExposures(ctx, learnerID, ids, where, unitID, r.clock.Now())
}

func (a Attempt) validate() error {
	if a.UserID <= 0 {
		return errors.New("evidence: attempt has no learner")
	}
	if a.LexemeID <= 0 {
		return errors.New("evidence: attempt has no lexeme")
	}
	switch a.Skill {
	case fsrs.SkillListen, fsrs.SkillRead, fsrs.SkillRecall, fsrs.SkillSpell:
	default:
		return fmt.Errorf("evidence: skill %q is not listen, read, recall, or spell", a.Skill)
	}
	switch a.Source {
	case SourcePlacement, SourceReview, SourceLesson, SourcePractice:
	default:
		return fmt.Errorf("evidence: source %q is not placement, review, lesson, or practice", a.Source)
	}
	if strings.TrimSpace(a.Activity) == "" {
		return errors.New("evidence: attempt has no activity")
	}
	if a.Latency < 0 {
		return errors.New("evidence: latency is negative")
	}
	if a.DontKnow && a.Correct {
		return errors.New("evidence: an answer cannot be both right and \"I don't know\"")
	}
	if a.NearMiss && !a.Correct {
		return errors.New("evidence: a near miss is a right answer")
	}
	if len(a.Confusions) > MaxConfusions {
		return fmt.Errorf("evidence: %d confusions, at most %d", len(a.Confusions), MaxConfusions)
	}
	for _, c := range a.Confusions {
		switch c.Kind {
		case fsrs.MissLetterPair, fsrs.MissAgreement, fsrs.MissOther:
		default:
			return fmt.Errorf("evidence: confusion kind %q is not letter_pair, agreement, or other", c.Kind)
		}
		if strings.TrimSpace(c.Key) == "" || len([]rune(c.Key)) > maxConfusionKeyRunes {
			return errors.New("evidence: confusion key is empty or too long")
		}
	}
	return nil
}

// row is the stored form. The response is cut to MaxResponseRunes.
func (a Attempt) row(now time.Time) db.Attempt {
	return db.Attempt{
		UserID:    a.UserID,
		LexemeID:  a.LexemeID,
		Skill:     a.Skill,
		Activity:  a.Activity,
		Source:    a.Source,
		UnitID:    a.UnitID,
		Prompt:    truncateRunes(a.Prompt, maxPromptRunes),
		Response:  truncateRunes(a.Response, MaxResponseRunes),
		Correct:   a.Correct && !a.DontKnow,
		DontKnow:  a.DontKnow,
		HintUsed:  a.HintUsed,
		LatencyMS: a.Latency.Milliseconds(),
		CreatedAt: now,
	}
}

func truncateRunes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func selfGradeLabel(rating fsrs.Rating) string {
	switch rating {
	case fsrs.Again:
		return "forgot"
	case fsrs.Hard:
		return "hard"
	case fsrs.Good:
		return "got_it"
	default:
		return "easy"
	}
}

// confusionFor looks up the word the learner picked in place of the asked word.
// It returns the picked word's pointed text, for review_log.chosen, and the
// confusion row to write. Any wrong pick that names another word is a confusion.
// The row is a letter_pair when the two words differ by exactly one letter, and
// otherwise it is kind other, keyed by the two words in code point order, so the
// pair in either order is one key. Nothing here knows an agreement miss, so that
// kind is not written from this path.
func (r *Recorder) confusionFor(ctx context.Context, a Attempt) (string, *db.Confusion, error) {
	if a.Correct || a.ChosenLexemeID <= 0 || a.ChosenLexemeID == a.LexemeID {
		return "", nil, nil
	}
	picked, ok, err := r.store.LexemeByID(ctx, a.ChosenLexemeID)
	if err != nil || !ok {
		return "", nil, err
	}
	asked, ok, err := r.store.LexemeByID(ctx, a.LexemeID)
	if err != nil {
		return "", nil, err
	}
	if !ok {
		return picked.TextNFC, nil, nil
	}
	if key, ok := letterPair(asked.TextNFC, picked.TextNFC); ok {
		return picked.TextNFC, &db.Confusion{Kind: fsrs.MissLetterPair, Key: key}, nil
	}
	return picked.TextNFC, &db.Confusion{Kind: fsrs.MissOther, Key: wordPair(asked.TextNFC, picked.TextNFC)}, nil
}

// wordPair is the key of two words that are not one letter apart: both in pointed
// NFC, joined by a slash, the smaller by code point first.
func wordPair(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "/" + b
}

// letterPair is the confused pair of two spellings that differ by one letter.
// Niqqud and cantillation are ignored, and a final letter counts as its
// ordinary form. The pair is in code point order, so the pair in either order is one key.
func letterPair(a, b string) (string, bool) {
	x, y := baseLetters(a), baseLetters(b)
	for len(x) > 0 && len(y) > 0 && x[0] == y[0] {
		x, y = x[1:], y[1:]
	}
	for len(x) > 0 && len(y) > 0 && x[len(x)-1] == y[len(y)-1] {
		x, y = x[:len(x)-1], y[:len(y)-1]
	}
	if len(x) != 1 || len(y) != 1 || !unicode.IsLetter(x[0]) || !unicode.IsLetter(y[0]) {
		return "", false
	}
	if x[0] > y[0] {
		x, y = y, x
	}
	return string(x) + "/" + string(y), true
}

// ordinaryLetter maps a final letter (kaf, mem, nun, pe, tsadi) to its ordinary form.
func ordinaryLetter(r rune) rune {
	switch r {
	case '\u05DA':
		return '\u05DB'
	case '\u05DD':
		return '\u05DE'
	case '\u05DF':
		return '\u05E0'
	case '\u05E3':
		return '\u05E4'
	case '\u05E5':
		return '\u05E6'
	default:
		return r
	}
}

func baseLetters(s string) []rune {
	var out []rune
	for _, r := range norm.NFC.String(s) {
		if r >= 0x0591 && r <= 0x05C7 {
			continue
		}
		out = append(out, ordinaryLetter(r))
	}
	return out
}
