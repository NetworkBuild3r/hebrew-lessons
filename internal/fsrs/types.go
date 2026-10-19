package fsrs

import (
	"fmt"
	"time"
)

// ModuleVersion is the go-fsrs release this package is written against.
const ModuleVersion = "v3.3.1"

// Rating is an FSRS grade. The numbers match go-fsrs and review_log.rating.
type Rating int

const (
	Again Rating = 1
	Hard  Rating = 2
	Good  Rating = 3
	Easy  Rating = 4
)

const (
	// DirHeEn is Hebrew shown, English recalled.
	DirHeEn = "he_en"
	// DirEnHe is English shown, Hebrew produced.
	DirEnHe = "en_he"

	// StateNew is a card that has not been reviewed.
	StateNew = "new"
	// StateLearning is a card still in short-term steps.
	StateLearning = "learning"
	// StateReview is a card on a multi-day interval.
	StateReview = "review"
	// StateRelearning is a review card that was forgotten.
	StateRelearning = "relearning"

	// MissLetterPair is a confused letter pair, for example ב/כ.
	MissLetterPair = "letter_pair"
	// MissAgreement is a confused agreement, practised as a pattern drill.
	MissAgreement = "agreement"
)

// Kind is one slot in a session queue.
type Kind string

const (
	// ItemCard is a due FSRS card.
	ItemCard Kind = "card"
	// ItemLetter is one step of a letter-pair drill.
	ItemLetter Kind = "letter"
	// ItemPattern is one step of an agreement drill.
	ItemPattern Kind = "pattern"
)

const (
	// MaxQueue is how many items Queue puts in a new session.
	// Session.RequeueMiss may grow the session past this.
	MaxQueue = 20

	// DrillItems is the length of one confusion drill.
	// A key produces one drill per session, not one drill per miss.
	DrillItems = 4

	// MinMissesForDrill is the 7-day count that adds a drill.
	// The count is rows in confusion_misses, not a stored counter.
	MinMissesForDrill = 2

	// ConfusionWindow is the rolling span Queue counts misses over.
	ConfusionWindow = 7 * 24 * time.Hour

	// unpointedStabilityDays is the he_en stability at which the spelling
	// may be shown without niqqud.
	unpointedStabilityDays = 21

	// unpointedRecent is how many of the latest ratings ShowUnpointed reads.
	unpointedRecent = 3
)

// Card is one learner's card for one lexeme in one direction.
// Ratings are the review_log grades, oldest first. ShowUnpointed reads at
// most the last three.
type Card struct {
	ID         int64
	UserID     int64
	LexemeID   int64
	Dir        string
	State      string
	Due        time.Time
	Stability  float64
	Difficulty float64
	Reps       int
	Lapses     int
	LastReview time.Time
	Suspended  bool
	Ratings    []Rating
}

// ShowUnpointed reports whether a he_en card may be shown without niqqud.
// That is true only when stability is at least 21 days and none of the last
// three ratings were Again. An en_he card is always pointed on the way in,
// so this is false for that direction.
func ShowUnpointed(card Card) bool {
	if card.Dir != DirHeEn {
		return false
	}
	if card.Stability < unpointedStabilityDays {
		return false
	}
	ratings := card.Ratings
	if len(ratings) > unpointedRecent {
		ratings = ratings[len(ratings)-unpointedRecent:]
	}
	for _, rating := range ratings {
		if rating == Again {
			return false
		}
	}
	return true
}

// Item is one step in a session.
// Card items set CardID, LexemeID, and Dir. Drill items set Key and Index.
// Index is 0..DrillItems-1 and distinguishes the steps of one drill.
type Item struct {
	Kind     Kind
	CardID   int64
	LexemeID int64
	Dir      string
	Key      string
	Index    int
}

func (it Item) identity() string {
	if it.Kind == ItemCard {
		return fmt.Sprintf("card:%d", it.CardID)
	}
	return fmt.Sprintf("%s:%d:%s", it.Kind, it.Index, it.Key)
}

// Session is one learner's queue.
// Items is what they see, front first. RequeueMiss appends to the end.
type Session struct {
	Items []Item
	once  map[string]struct{}
}

// RequeueMiss appends item to the end of the session.
// Call it when the learner grades the item Again. The session may grow past
// MaxQueue. The same item is appended at most once; a second miss returns false.
func (s *Session) RequeueMiss(item Item) bool {
	if s == nil {
		return false
	}
	id := item.identity()
	if s.once == nil {
		s.once = map[string]struct{}{}
	}
	if _, ok := s.once[id]; ok {
		return false
	}
	s.once[id] = struct{}{}
	s.Items = append(s.Items, item)
	return true
}
