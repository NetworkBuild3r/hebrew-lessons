package mcp

import (
	"errors"
	"fmt"
	"time"
)

const (
	// contextCardCap is how many cards get_context returns. The rest
	// are paged with get_learner_words.
	contextCardCap = 200
	// learnerWordsDefaultLimit is the get_learner_words page size when
	// limit is omitted or not positive.
	learnerWordsDefaultLimit = 50
	// learnerWordsMaxLimit is the largest page get_learner_words returns.
	learnerWordsMaxLimit = 200
	// AccuracyWindowDays is the recent-accuracy window. It matches the
	// 7-day span the session mix uses for confusion and lapses.
	AccuracyWindowDays = 7
	// AccuracyReviewCap is how many of the newest ratings recentAccuracy
	// reads. It matches the 50 ratings the session mix reads.
	AccuracyReviewCap = 50

	// ratingGood and ratingEasy are the FSRS ratings counted as success.
	// 1 is Again and 2 is Hard. The session mix uses the same pair.
	ratingGood = 3
	ratingEasy = 4
)

// ErrBadCardCursor means a page cursor could not be read.
var ErrBadCardCursor = errors.New("The page cursor is not valid.")

const cardsTruncatedHint = "The card list is capped, due soonest first. Call get_learner_words to page the rest."

// LearnerCard is one FSRS card joined to its lexeme.
// Translit, LastRating, and LastReview are null when the store has no value.
// Times are UTC.
type LearnerCard struct {
	Lexeme     string     `json:"lexeme"`
	Translit   *string    `json:"translit"`
	POS        string     `json:"pos"`
	Gloss      string     `json:"gloss"`
	Skill      string     `json:"skill"`
	State      string     `json:"state"`
	Stability  float64    `json:"stability"`
	Difficulty float64    `json:"difficulty"`
	Due        time.Time  `json:"due"`
	Lapses     int        `json:"lapses"`
	Reps       int        `json:"reps"`
	LastRating *int       `json:"lastRating"`
	LastReview *time.Time `json:"lastReview"`
}

// CardQuery selects one page of a learner's cards.
// Cursor is the opaque nextCursor from the previous page. Empty is the first page.
// State empty is every state. DueBefore, when set, keeps cards due strictly before it.
type CardQuery struct {
	UserID    int64
	Cursor    string
	Limit     int
	State     string
	DueBefore *time.Time
}

// CardPage is one page of cards.
// NextCursor is empty when this page is the last.
type CardPage struct {
	Cards      []LearnerCard
	NextCursor string
}

// MixTarget is the unit new-word budget plus the session mix.
// It is course config, not a per-learner ratio.
type MixTarget struct {
	MaxNewWordsPerUnit   int     `json:"maxNewWordsPerUnit"`
	MinNewWordsPerLesson int     `json:"minNewWordsPerLesson"`
	MaxNewWordsPerLesson int     `json:"maxNewWordsPerLesson"`
	TargetSuccess        float64 `json:"targetSuccess"`
	MaxNewPerSession     int     `json:"maxNewPerSession"`
	MinNewPerSession     int     `json:"minNewPerSession" jsonschema:"Fewest new cards a session introduces when a review is due and that many unreviewed cards are waiting."`
	MaxNewShare          float64 `json:"maxNewShare"`
}

// CardsDueNow counts the learner's unsuspended cards due at or before now.
// Total is the size of that set. New, Learning, Review, and Relearning are
// its FSRS states. A state outside those four still counts in Total.
// It is null on get_context when learnerId is omitted.
type CardsDueNow struct {
	New        int `json:"new"`
	Learning   int `json:"learning"`
	Review     int `json:"review"`
	Relearning int `json:"relearning"`
	Total      int `json:"total"`
}

// CardsDueNowFromStates counts states returned by the due-card query.
// Total is len(states).
func CardsDueNowFromStates(states []string) CardsDueNow {
	var out CardsDueNow
	out.Total = len(states)
	for _, state := range states {
		switch state {
		case "new":
			out.New++
		case "learning":
			out.Learning++
		case "review":
			out.Review++
		case "relearning":
			out.Relearning++
		}
	}
	return out
}

// RecentAccuracy is the learner's recent review and lesson record.
// Rate and MeanScore are null when the corresponding count is zero.
type RecentAccuracy struct {
	WindowDays      int            `json:"windowDays"`
	Reviews7d       AccuracyCount  `json:"reviews7d"`
	LastReviews     AccuracyCount  `json:"lastReviews"`
	LastReviewLimit int            `json:"lastReviewLimit"`
	Lessons7d       LessonAccuracy `json:"lessons7d"`
}

// AccuracyCount is a Good-or-Easy share.
type AccuracyCount struct {
	Count      int      `json:"count"`
	GoodOrEasy int      `json:"goodOrEasy"`
	Rate       *float64 `json:"rate"`
}

// LessonAccuracy is finished lesson_runs in the window.
// Scored omits runs whose score is null. MeanScore is the mean of the rest.
// Finished, Scored, and MeanScore stay zero or null until the lesson player
// writes lesson_runs. Coach from the review half until then.
type LessonAccuracy struct {
	Finished  int      `json:"finished"`
	Scored    int      `json:"scored"`
	MeanScore *float64 `json:"meanScore" jsonschema:"Mean of non-null lesson_runs scores. Null when none are scored. Stays null until the lesson player writes lesson_runs."`
}

func countAccuracy(count, good int) AccuracyCount {
	out := AccuracyCount{Count: count, GoodOrEasy: good}
	if count > 0 {
		rate := float64(good) / float64(count)
		out.Rate = &rate
	}
	return out
}

func lessonAccuracy(finished, scored int, sum float64) LessonAccuracy {
	out := LessonAccuracy{Finished: finished, Scored: scored}
	if scored > 0 {
		mean := sum / float64(scored)
		out.MeanScore = &mean
	}
	return out
}

func recentAccuracy(reviews7, last AccuracyCount, lessons LessonAccuracy) RecentAccuracy {
	return RecentAccuracy{
		WindowDays:      AccuracyWindowDays,
		Reviews7d:       reviews7,
		LastReviews:     last,
		LastReviewLimit: AccuracyReviewCap,
		Lessons7d:       lessons,
	}
}

func goodOrEasy(rating int) bool {
	return rating == ratingGood || rating == ratingEasy
}

func validCardState(state string) bool {
	switch state {
	case "", "new", "learning", "review", "relearning":
		return true
	default:
		return false
	}
}

func coachContextDescription() string {
	mix := currentMixTarget()
	return fmt.Sprintf("When learnerId is set, cards lists that learner's FSRS cards, one per card, including suspended cards. Order is due soonest first, then most recently reviewed, then card id. The list is capped at %d. cardsTruncated is true when cards were left out; call get_learner_words to page the rest. Each card has lexeme (pointed NFC), translit, pos, gloss, skill, state, stability, difficulty, due, lapses, reps, lastRating, and lastReview. Times are UTC RFC3339. translit, lastRating, and lastReview are null when unknown. Recycle cards that are due. Reinforce words with high lapses. recentAccuracy is null when learnerId is omitted. Otherwise the review half counts Good and Easy ratings over the last %d days and over the last %d reviews. The lesson half, lessons7d counts and meanScore, stays zero until the lesson player writes lesson_runs. Coach from the review half until then. rate and meanScore are null when the count is 0. mixTarget keeps the unit budgets, at most %d new words in a unit and %d to %d per lesson, and the session mix: targetSuccess %g, maxNewPerSession %d, minNewPerSession %d, and maxNewShare %g. With reviews due, new cards are min(maxNewPerSession, max(minNewPerSession, floor(maxNewShare * sessionSize))), where sessionSize is the reviewed cards plus the new cards. No reviews still allows the full maxNewPerSession.",
		contextCardCap, AccuracyWindowDays, AccuracyReviewCap,
		mix.MaxNewWordsPerUnit, mix.MinNewWordsPerLesson, mix.MaxNewWordsPerLesson,
		mix.TargetSuccess, mix.MaxNewPerSession, mix.MinNewPerSession, mix.MaxNewShare)
}
