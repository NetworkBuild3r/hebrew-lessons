package placement

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// Service deals the placement deck and, on a reliable finish, seeds each
// rated real word as its first FSRS review.
// The clock stamps started_at, shown_at, answered_at, finished_at, and the
// shuffle seed. The review service grades the seeded cards on that same clock.
type Service struct {
	store   *db.Store
	review  *fsrs.Review
	clock   clock.Clock
	fixture []fixtureItem
	cards   int
	limit   time.Duration
}

// New uses store, review, and clk.
// A nil clock uses the clock the store was opened with.
// A nil review uses that same clock.
func New(store *db.Store, review *fsrs.Review, clk clock.Clock) *Service {
	if clk == nil && store != nil {
		clk = store.Clock()
	}
	if review == nil {
		review = fsrs.New(store, clk)
	}
	return &Service{store: store, review: review, clock: clk}
}

func (s *Service) ready() error {
	if s == nil || s.store == nil {
		return errors.New("placement: service is not configured")
	}
	if s.clock == nil {
		return errors.New("placement: clock is not configured")
	}
	if s.review == nil {
		return errors.New("placement: review is not configured")
	}
	return nil
}

func (s *Service) cardLimit() int {
	if s.cards > 0 {
		return s.cards
	}
	return cardCap
}

func (s *Service) timeLimit() time.Duration {
	if s.limit > 0 {
		return s.limit
	}
	return timeCap
}

func (s *Service) loadFixture() ([]fixtureItem, error) {
	if s.fixture != nil {
		return s.fixture, nil
	}
	return catalog()
}

// Start opens a run for the learner. The deck is dealt one card at a time
// from the seed stored on the run.
func (s *Service) Start(ctx context.Context, userID int64) (int64, error) {
	if err := s.ready(); err != nil {
		return 0, err
	}
	cat, err := s.loadFixture()
	if err != nil {
		return 0, err
	}
	if len(cat) == 0 {
		return 0, errors.New("placement: fixture is empty")
	}
	now := s.clock.Now()
	seed := now.UnixNano()
	if seed == 0 {
		seed = 1
	}
	return s.store.InsertPlacementRun(ctx, userID, now, seed)
}

// Next returns the card the learner should rate.
// done is true when the deck has stopped: the card cap, the time cap, a band
// under 50%, or the fixture is exhausted. An already-dealt unanswered card is
// returned even after a cap, so it can still be rated. The call after that
// returns done. A finished run returns an error.
func (s *Service) Next(ctx context.Context, runID int64) (*Card, bool, error) {
	if err := s.ready(); err != nil {
		return nil, false, err
	}
	run, err := s.loadOpen(ctx, runID)
	if err != nil {
		return nil, false, err
	}
	rows, err := s.store.ListPlacementAnswers(ctx, runID)
	if err != nil {
		return nil, false, err
	}
	if open, ok := unanswered(rows); ok {
		it, err := s.find(open.ItemID)
		if err != nil {
			return nil, false, err
		}
		card, err := s.present(ctx, it)
		if err != nil {
			return nil, false, err
		}
		return &card, false, nil
	}
	if len(rows) >= s.cardLimit() || !s.clock.Now().Before(run.StartedAt.Add(s.timeLimit())) {
		return nil, true, nil
	}
	cat, err := s.loadFixture()
	if err != nil {
		return nil, false, err
	}
	shown := map[string]struct{}{}
	for _, row := range rows {
		shown[row.ItemID] = struct{}{}
	}
	rated := ratingsOf(rows)
	for _, block := range blocks(run.Seed, cat) {
		if len(block) == 0 {
			continue
		}
		var next *fixtureItem
		for i := range block {
			if _, ok := shown[block[i].ID]; ok {
				continue
			}
			it := block[i]
			next = &it
			break
		}
		if next != nil {
			if err := s.store.ShowPlacementCard(ctx, runID, next.ID, s.clock.Now()); err != nil {
				return nil, false, err
			}
			card, err := s.present(ctx, *next)
			if err != nil {
				return nil, false, err
			}
			return &card, false, nil
		}
		complete, realShown, goodPlus := blockProgress(block, rated)
		if !complete {
			return nil, false, errors.New("placement: deck has a gap")
		}
		if realShown > 0 && goodPlus*2 < realShown {
			return nil, true, nil
		}
	}
	return nil, true, nil
}

// Rate stores the learner's grade for the open card.
// rating is 1 Again, 2 Hard, 3 Good, 4 Easy, or 0 for "I don't know".
// The returned flag is true when the word was invented.
func (s *Service) Rate(ctx context.Context, runID int64, itemID string, rating int) (Rated, error) {
	if err := s.ready(); err != nil {
		return Rated{}, err
	}
	if rating < DontKnow || rating > Easy {
		return Rated{}, fmt.Errorf("placement: rating %d is outside 0..4", rating)
	}
	if itemID == "" {
		return Rated{}, errors.New("placement: item id is empty")
	}
	if _, err := s.loadOpen(ctx, runID); err != nil {
		return Rated{}, err
	}
	rows, err := s.store.ListPlacementAnswers(ctx, runID)
	if err != nil {
		return Rated{}, err
	}
	open, ok := unanswered(rows)
	if !ok || open.ItemID != itemID {
		return Rated{}, fmt.Errorf("placement: card %s is not the open card", itemID)
	}
	it, err := s.find(itemID)
	if err != nil {
		return Rated{}, err
	}
	if err := s.store.RatePlacementAnswer(ctx, runID, itemID, rating, s.clock.Now()); err != nil {
		return Rated{}, err
	}
	return Rated{IsPseudo: it.Pseudo}, nil
}

// Finish scores the run. A reliable run seeds every rated real word: the
// lexeme, a learner row, and a he_en card graded with this rating. "I don't
// know" is graded Again. A word that already has a he_en card is left as it
// is. An unreliable run stores the score and seeds nothing.
func (s *Service) Finish(ctx context.Context, runID int64) (Result, error) {
	if err := s.ready(); err != nil {
		return Result{}, err
	}
	run, err := s.loadOpen(ctx, runID)
	if err != nil {
		return Result{}, err
	}
	cat, err := s.loadFixture()
	if err != nil {
		return Result{}, err
	}
	rows, err := s.store.ListPlacementAnswers(ctx, runID)
	if err != nil {
		return Result{}, err
	}
	answers := make([]Answer, 0, len(rows))
	for _, row := range rows {
		if !row.Rating.Valid {
			continue
		}
		answers = append(answers, Answer{ItemID: row.ItemID, Rating: int(row.Rating.Int64)})
	}
	items := make([]ScoreItem, len(cat))
	for i, it := range cat {
		items[i] = ScoreItem{ID: it.ID, Band: it.Band, Pseudo: it.Pseudo}
	}
	result := Score(answers, items)
	now := s.clock.Now()
	result.Cap = s.capReason(run, rows, blocks(run.Seed, cat), now)
	result.RunID = runID
	result.UserID = run.UserID
	result.StartedAt = run.StartedAt
	result.FinishedAt = now

	if result.Reliable {
		if err := s.seed(ctx, run.UserID, cat, rows, now); err != nil {
			return Result{}, err
		}
	}
	body, err := json.Marshal(storedResult{Bands: result.Bands, EstimatedKnownWords: result.EstimatedKnownWords})
	if err != nil {
		return Result{}, err
	}
	if err := s.store.FinishPlacement(ctx, runID, now, result.FalseAlarmRate, result.Reliable, result.Level, result.Cap, string(body)); err != nil {
		return Result{}, err
	}
	place, ok, err := s.store.LatestPlacement(ctx, run.UserID)
	if err != nil {
		return Result{}, err
	}
	if ok {
		result.SuggestedStartPosition = place.SuggestedStartPosition
	}
	return result, nil
}

func (s *Service) capReason(run db.PlacementRun, rows []db.PlacementAnswer, deck [][]fixtureItem, now time.Time) string {
	if len(rows) >= s.cardLimit() {
		return capCards
	}
	if !now.Before(run.StartedAt.Add(s.timeLimit())) {
		return capTime
	}
	rated := ratingsOf(rows)
	for _, block := range deck {
		if len(block) == 0 {
			continue
		}
		complete, realShown, goodPlus := blockProgress(block, rated)
		if !complete {
			return ""
		}
		if realShown > 0 && goodPlus*2 < realShown {
			return capBand
		}
	}
	return ""
}

func (s *Service) seed(ctx context.Context, userID int64, cat []fixtureItem, rows []db.PlacementAnswer, at time.Time) error {
	byID := make(map[string]fixtureItem, len(cat))
	for _, it := range cat {
		byID[it.ID] = it
	}
	for _, row := range rows {
		if !row.Rating.Valid {
			continue
		}
		it, ok := byID[row.ItemID]
		if !ok || it.Pseudo || it.Band == nil {
			continue
		}
		rating := int(row.Rating.Int64)
		if rating < DontKnow || rating > Easy {
			return fmt.Errorf("placement: rating %d is outside 0..4", rating)
		}
		existing, _, _, _, found, err := s.store.LexemeByText(ctx, it.TextNFC)
		if err != nil {
			return err
		}
		if found {
			if _, has, err := s.store.CardID(ctx, userID, existing, fsrs.DirHeEn); err != nil {
				return err
			} else if has {
				continue
			}
		}
		lexemeID, err := s.store.UpsertPlacementLexeme(ctx, it.TextNFC, it.GlossEN, it.POS)
		if err != nil {
			return err
		}
		if _, has, err := s.store.CardID(ctx, userID, lexemeID, fsrs.DirHeEn); err != nil {
			return err
		} else if has {
			continue
		}
		status := "learning"
		if rating == Good || rating == Easy {
			status = "known"
		}
		if err := s.store.SetPlacementLearner(ctx, userID, lexemeID, status, at); err != nil {
			return err
		}
		cardID, err := s.store.InsertCard(ctx, db.Card{
			UserID:   userID,
			LexemeID: lexemeID,
			Dir:      fsrs.DirHeEn,
			State:    fsrs.StateNew,
			Due:      at,
		})
		if err != nil {
			return err
		}
		grade := fsrs.Rating(rating)
		if rating == DontKnow {
			grade = fsrs.Again
		}
		if err := s.review.Grade(ctx, cardID, grade, false); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) present(ctx context.Context, it fixtureItem) (Card, error) {
	card := it.card()
	id, ok, err := s.store.AudioIDByText(ctx, it.TextNFC)
	if err != nil {
		return Card{}, err
	}
	if ok {
		card.AudioURL = "/audio/" + id + ".mp3"
	}
	return card, nil
}

func (s *Service) find(id string) (fixtureItem, error) {
	cat, err := s.loadFixture()
	if err != nil {
		return fixtureItem{}, err
	}
	for _, it := range cat {
		if it.ID == id {
			return it, nil
		}
	}
	return fixtureItem{}, fmt.Errorf("placement: item %s is not in the fixture", id)
}

func (s *Service) loadOpen(ctx context.Context, runID int64) (db.PlacementRun, error) {
	run, err := s.store.GetPlacementRun(ctx, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.PlacementRun{}, fmt.Errorf("placement: run %d not found", runID)
	}
	if err != nil {
		return db.PlacementRun{}, err
	}
	if !run.FinishedAt.IsZero() {
		return db.PlacementRun{}, fmt.Errorf("placement: run %d is already finished", runID)
	}
	return run, nil
}

func unanswered(rows []db.PlacementAnswer) (db.PlacementAnswer, bool) {
	for _, row := range rows {
		if !row.Rating.Valid {
			return row, true
		}
	}
	return db.PlacementAnswer{}, false
}

func ratingsOf(rows []db.PlacementAnswer) func(string) (int, bool) {
	rated := make(map[string]int, len(rows))
	for _, row := range rows {
		if row.Rating.Valid {
			rated[row.ItemID] = int(row.Rating.Int64)
		}
	}
	return func(id string) (int, bool) {
		rating, ok := rated[id]
		return rating, ok
	}
}
