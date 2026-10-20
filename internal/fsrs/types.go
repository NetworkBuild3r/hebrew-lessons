package fsrs

import (
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
	// SkillListen is Hebrew heard, meaning picked.
	SkillListen = "listen"
	// SkillRead is Hebrew shown, English recalled.
	SkillRead = "read"
	// SkillRecall is English shown, Hebrew produced.
	SkillRecall = "recall"
	// SkillSpell is a word spelled from memory.
	SkillSpell = "spell"

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
	// MissOther is a wrong pick of another word that is neither a one-letter
	// pair nor an agreement miss. The key is the two words, in pointed NFC.
	MissOther = "other"
)

const (
	// unpointedStabilityDays is the read stability at which the spelling
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
	Skill      string
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

// ShowUnpointed reports whether a read card may be shown without niqqud.
// That is true only when stability is at least 21 days and none of the last
// three ratings were Again. A recall card is always pointed on the way in,
// so this is false for that direction.
func ShowUnpointed(card Card) bool {
	if card.Skill != SkillRead {
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
