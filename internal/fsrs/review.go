package fsrs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"

	gofsrs "github.com/open-spaced-repetition/go-fsrs/v3"
)

// Review schedules cards.
// The study queue is package session. The clock is the process clock from configuration.
type Review struct {
	store *db.Store
	clock clock.Clock
	sched *gofsrs.FSRS
}

// New schedules with go-fsrs v3.3.1 (FSRS-5) and clk.
// A nil clk uses the clock the store was opened with.
// Fuzz is off, so the same card and time always produce the same interval.
func New(store *db.Store, clk clock.Clock) *Review {
	if clk == nil && store != nil {
		clk = store.Clock()
	}
	param := gofsrs.DefaultParam()
	param.EnableFuzz = false
	return &Review{
		store: store,
		clock: clk,
		sched: gofsrs.NewFSRS(param),
	}
}

func (r *Review) ready() error {
	if r == nil || r.store == nil {
		return errors.New("fsrs: review is not configured")
	}
	if r.clock == nil {
		return errors.New("fsrs: clock is not configured")
	}
	return nil
}

// ScheduleNew plans the first review of a new card. It does not write.
// The result is what Grade stores after inserting a new he_en card and
// reviewing it at now. rating is 1 Again, 2 Hard, 3 Good, or 4 Easy.
func (r *Review) ScheduleNew(now time.Time, rating Rating) (db.Card, db.ReviewLog, error) {
	if r == nil || r.sched == nil {
		return db.Card{}, db.ReviewLog{}, errors.New("fsrs: review is not configured")
	}
	if rating < Again || rating > Easy {
		return db.Card{}, db.ReviewLog{}, fmt.Errorf("fsrs: rating %d is outside 1..4", rating)
	}
	info := r.sched.Next(gofsrs.NewCard(), now, gofsrs.Rating(rating))
	state, err := stateName(info.Card.State)
	if err != nil {
		return db.Card{}, db.ReviewLog{}, err
	}
	card := db.Card{
		State:      state,
		Due:        info.Card.Due,
		Stability:  info.Card.Stability,
		Difficulty: info.Card.Difficulty,
		Reps:       int(info.Card.Reps),
		Lapses:     int(info.Card.Lapses),
		LastReview: info.Card.LastReview,
	}
	log := db.ReviewLog{
		Rating:        int(rating),
		ReviewedAt:    now,
		ElapsedDays:   int(info.ReviewLog.ElapsedDays),
		ScheduledDays: int(info.Card.ScheduledDays),
	}
	return card, log, nil
}

// Grade applies one rating to a card and appends a review_log row.
// rating is 1 Again, 2 Hard, 3 Good, or 4 Easy.
// peeked is true when the learner revealed the answer first.
// The review time is the configured clock, not the wall clock.
// It stores no chosen option. GradeWithChoice does, when the learner picked one.
func (r *Review) Grade(ctx context.Context, cardID int64, rating Rating, peeked bool) error {
	return r.GradeWithChoice(ctx, cardID, rating, peeked, "")
}

// GradeWithChoice is Grade with the wrong option the learner picked.
// chosen is pointed NFC. An empty chosen is stored as NULL.
func (r *Review) GradeWithChoice(ctx context.Context, cardID int64, rating Rating, peeked bool, chosen string) error {
	if err := r.ready(); err != nil {
		return err
	}
	if rating < Again || rating > Easy {
		return fmt.Errorf("fsrs: rating %d is outside 1..4", rating)
	}
	row, err := r.store.GetCard(ctx, cardID)
	if err != nil {
		return err
	}
	prev, err := schedulerCard(row)
	if err != nil {
		return err
	}
	now := r.clock.Now()
	info := r.sched.Next(prev, now, gofsrs.Rating(rating))
	state, err := stateName(info.Card.State)
	if err != nil {
		return err
	}
	next := row
	next.State = state
	next.Due = info.Card.Due
	next.Stability = info.Card.Stability
	next.Difficulty = info.Card.Difficulty
	next.Reps = int(info.Card.Reps)
	next.Lapses = int(info.Card.Lapses)
	next.LastReview = info.Card.LastReview
	// scheduled_days is the interval this rating scheduled (the next card).
	// go-fsrs ReviewLog.ScheduledDays is the previous interval, which is not
	// what the column records. elapsed_days is the gap FSRS just measured.
	log := db.ReviewLog{
		CardID:        cardID,
		Rating:        int(rating),
		ReviewedAt:    now,
		ElapsedDays:   int(info.ReviewLog.ElapsedDays),
		ScheduledDays: int(info.Card.ScheduledDays),
		Peeked:        peeked,
		Chosen:        chosen,
	}
	return r.store.ApplyReview(ctx, next, log)
}

// Suspend sets whether the card is kept out of the due and study lists.
// The card and its review log stay. Passing false queues it again once it is due.
func (r *Review) Suspend(ctx context.Context, cardID int64, suspended bool) error {
	if err := r.ready(); err != nil {
		return err
	}
	return r.store.SetCardSuspended(ctx, cardID, suspended)
}

// RecordMiss appends one confusion_misses row at the configured clock.
// kind is letter_pair or agreement. key is the pair, for example ב/כ.
// lexemeID is the word the miss was about. 0 stores NULL.
func (r *Review) RecordMiss(ctx context.Context, learnerID int64, kind, key string, lexemeID int64) error {
	if err := r.ready(); err != nil {
		return err
	}
	switch kind {
	case MissLetterPair, MissAgreement:
	default:
		return fmt.Errorf("fsrs: confusion kind %q is not letter_pair or agreement", kind)
	}
	if key == "" {
		return errors.New("fsrs: confusion key is empty")
	}
	return r.store.InsertConfusionMiss(ctx, learnerID, kind, key, lexemeID, r.clock.Now())
}

// Card loads one card and its review ratings, oldest first.
func (r *Review) Card(ctx context.Context, cardID int64) (Card, error) {
	if err := r.ready(); err != nil {
		return Card{}, err
	}
	row, err := r.store.GetCard(ctx, cardID)
	if err != nil {
		return Card{}, err
	}
	logs, err := r.store.ReviewLogs(ctx, cardID)
	if err != nil {
		return Card{}, err
	}
	card := cardFromRow(row)
	card.Ratings = make([]Rating, len(logs))
	for i, log := range logs {
		card.Ratings[i] = Rating(log.Rating)
	}
	return card, nil
}

func cardFromRow(row db.Card) Card {
	return Card{
		ID:         row.ID,
		UserID:     row.UserID,
		LexemeID:   row.LexemeID,
		Dir:        row.Dir,
		State:      row.State,
		Due:        row.Due,
		Stability:  row.Stability,
		Difficulty: row.Difficulty,
		Reps:       row.Reps,
		Lapses:     row.Lapses,
		LastReview: row.LastReview,
		Suspended:  row.Suspended,
	}
}

func schedulerCard(row db.Card) (gofsrs.Card, error) {
	state, err := schedulerState(row.State)
	if err != nil {
		return gofsrs.Card{}, err
	}
	var reps uint64
	if row.Reps > 0 {
		reps = uint64(row.Reps)
	}
	var lapses uint64
	if row.Lapses > 0 {
		lapses = uint64(row.Lapses)
	}
	return gofsrs.Card{
		Due:        row.Due,
		Stability:  row.Stability,
		Difficulty: row.Difficulty,
		Reps:       reps,
		Lapses:     lapses,
		State:      state,
		LastReview: row.LastReview,
	}, nil
}

func schedulerState(state string) (gofsrs.State, error) {
	switch state {
	case StateNew:
		return gofsrs.New, nil
	case StateLearning:
		return gofsrs.Learning, nil
	case StateReview:
		return gofsrs.Review, nil
	case StateRelearning:
		return gofsrs.Relearning, nil
	default:
		return 0, fmt.Errorf("fsrs: card state %q is unknown", state)
	}
}

func stateName(state gofsrs.State) (string, error) {
	switch state {
	case gofsrs.New:
		return StateNew, nil
	case gofsrs.Learning:
		return StateLearning, nil
	case gofsrs.Review:
		return StateReview, nil
	case gofsrs.Relearning:
		return StateRelearning, nil
	default:
		return "", fmt.Errorf("fsrs: scheduler state %d is unknown", state)
	}
}
