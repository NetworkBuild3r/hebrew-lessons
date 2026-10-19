package placement

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// Service deals the placement deck. Each rating is that word's first he_en
// review: the card is created and graded through fsrs.Review.Grade, so the
// stability is the one FSRS-5 assigns a new card.
// The clock stamps the run, the answers, and the review.
type Service struct {
	store   *db.Store
	review  *fsrs.Review
	clock   clock.Clock
	fixture []fixtureItem
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

func (s *Service) loadFixture() ([]fixtureItem, error) {
	if s.fixture != nil {
		return s.fixture, nil
	}
	return catalog()
}

// Start returns the learner's unfinished run, or opens one.
// The deck order comes from the seed stored on that run.
// A run left unfinished is resumed on a later day. When its current sitting
// already has SittingSize answers, Start opens the next sitting.
func (s *Service) Start(ctx context.Context, userID int64) (int64, error) {
	if err := s.ready(); err != nil {
		return 0, err
	}
	open, ok, err := s.store.OpenPlacementRun(ctx, userID)
	if err != nil {
		return 0, err
	}
	if ok {
		if err := s.resumeSitting(ctx, open); err != nil {
			return 0, err
		}
		return open.ID, nil
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

// Next returns the next card after the last answered one.
// Done is set when a band's Again rate has reached StopAgainRate, the run
// has rated CardCap, or nothing dealable remains. That also scores the
// run. Break is set when the current sitting has SittingSize answers and
// the run is still open. A word that already has a he_en card is skipped.
// A card already dealt is returned so the learner can rate it. A finished
// run returns an error.
func (s *Service) Next(ctx context.Context, runID int64) (NextResult, error) {
	if err := s.ready(); err != nil {
		return NextResult{}, err
	}
	run, err := s.loadOpen(ctx, runID)
	if err != nil {
		return NextResult{}, err
	}
	rows, err := s.store.ListPlacementAnswers(ctx, runID)
	if err != nil {
		return NextResult{}, err
	}
	if open, ok := unanswered(rows); ok {
		it, err := s.find(open.ItemID)
		if err != nil {
			return NextResult{}, err
		}
		has, err := s.hasCard(ctx, run.UserID, it.TextNFC)
		if err != nil {
			return NextResult{}, err
		}
		if !has {
			card, err := s.present(ctx, it)
			if err != nil {
				return NextResult{}, err
			}
			return NextResult{Card: &card}, nil
		}
		if err := s.store.DeleteOpenPlacementAnswer(ctx, runID, open.ItemID); err != nil {
			return NextResult{}, err
		}
		rows, err = s.store.ListPlacementAnswers(ctx, runID)
		if err != nil {
			return NextResult{}, err
		}
	}
	cat, err := s.loadFixture()
	if err != nil {
		return NextResult{}, err
	}
	order := deck(run.Seed, cat)
	answered := map[string]int{}
	for _, row := range rows {
		if row.Rating.Valid {
			answered[row.ItemID] = int(row.Rating.Int64)
		}
	}
	counts := bandCounts(answered, indexItems(cat))
	if ratedCount(rows) >= CardCap || bandTripped(counts) {
		return s.finishNow(ctx, runID)
	}
	next, ok, err := s.pick(ctx, run, order, answered, counts)
	if err != nil {
		return NextResult{}, err
	}
	if !ok {
		return s.finishNow(ctx, runID)
	}
	if sittingRated(rows, run.SittingNo) >= SittingSize {
		return NextResult{Break: true}, nil
	}
	if err := s.store.ShowPlacementCard(ctx, runID, next.ID, run.SittingNo, s.clock.Now()); err != nil {
		return NextResult{}, err
	}
	card, err := s.present(ctx, next)
	if err != nil {
		return NextResult{}, err
	}
	return NextResult{Card: &card}, nil
}

// ContinueSitting opens the next sitting on the learner's unfinished run
// after the current one is full. "Keep going" calls it with the user id.
// "Done for today" does not, and does not finish the run.
func (s *Service) ContinueSitting(ctx context.Context, userID int64) error {
	if err := s.ready(); err != nil {
		return err
	}
	run, ok, err := s.store.OpenPlacementRun(ctx, userID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("placement: user %d has no open run", userID)
	}
	rows, err := s.store.ListPlacementAnswers(ctx, run.ID)
	if err != nil {
		return err
	}
	if sittingRated(rows, run.SittingNo) < SittingSize {
		return fmt.Errorf("placement: sitting %d is not full", run.SittingNo)
	}
	return s.store.AdvancePlacementSitting(ctx, run.ID)
}

func (s *Service) resumeSitting(ctx context.Context, run db.PlacementRun) error {
	rows, err := s.store.ListPlacementAnswers(ctx, run.ID)
	if err != nil {
		return err
	}
	if sittingRated(rows, run.SittingNo) < SittingSize {
		return nil
	}
	return s.store.AdvancePlacementSitting(ctx, run.ID)
}

func (s *Service) finishNow(ctx context.Context, runID int64) (NextResult, error) {
	if _, err := s.Finish(ctx, runID); err != nil {
		return NextResult{}, err
	}
	return NextResult{Done: true}, nil
}

func (s *Service) pick(ctx context.Context, run db.PlacementRun, order []fixtureItem, answered map[string]int, counts [bands]bandCount) (fixtureItem, bool, error) {
	for _, it := range order {
		if _, ok := answered[it.ID]; ok {
			continue
		}
		has, err := s.hasCard(ctx, run.UserID, it.TextNFC)
		if err != nil {
			return fixtureItem{}, false, err
		}
		if has {
			continue
		}
		if it.Band < 0 || it.Band >= bands {
			return fixtureItem{}, false, nil
		}
		if tripped(counts[it.Band]) {
			return fixtureItem{}, false, nil
		}
		if counts[it.Band].rated >= BandMax {
			continue
		}
		for band := 0; band < it.Band; band++ {
			if tripped(counts[band]) {
				return fixtureItem{}, false, nil
			}
		}
		return it, true, nil
	}
	return fixtureItem{}, false, nil
}

// Rate stores the grade and makes it the word's first he_en review.
// rating is 0 "I don't know this", 1 Again, 2 Hard, 3 Good, or 4 Easy.
// 0 is stored as DontKnow and graded as Again.
// A word that already has a card is not graded again.
func (s *Service) Rate(ctx context.Context, runID int64, itemID string, rating int) error {
	if err := s.ready(); err != nil {
		return err
	}
	if rating < DontKnow || rating > Easy {
		return fmt.Errorf("placement: rating %d is outside 0..4", rating)
	}
	if itemID == "" {
		return errors.New("placement: item id is empty")
	}
	run, err := s.loadOpen(ctx, runID)
	if err != nil {
		return err
	}
	rows, err := s.store.ListPlacementAnswers(ctx, runID)
	if err != nil {
		return err
	}
	open, ok := unanswered(rows)
	if !ok || open.ItemID != itemID {
		return fmt.Errorf("placement: card %s is not the open card", itemID)
	}
	it, err := s.find(itemID)
	if err != nil {
		return err
	}
	has, err := s.hasCard(ctx, run.UserID, it.TextNFC)
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("placement: %s already has a card", it.TextNFC)
	}
	lexemeID, err := s.store.UpsertPlacementLexeme(ctx, it.TextNFC, it.GlossEN, it.POS, it.Translit, it.Gender)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	cardID, err := s.store.InsertCard(ctx, db.Card{
		UserID:   run.UserID,
		LexemeID: lexemeID,
		Dir:      fsrs.DirHeEn,
		State:    fsrs.StateNew,
		Due:      now,
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
	return s.store.RatePlacementAnswer(ctx, runID, itemID, rating, lexemeID, now)
}

// Finish scores the run and marks it finished.
// A second call returns the stored result and does not grade anything again.
// SuggestedStartPosition is computed from the units and known words now.
func (s *Service) Finish(ctx context.Context, runID int64) (Result, error) {
	if err := s.ready(); err != nil {
		return Result{}, err
	}
	run, err := s.store.GetPlacementRun(ctx, runID)
	if err != nil {
		return Result{}, err
	}
	if !run.FinishedAt.IsZero() {
		return s.resultFrom(ctx, run)
	}
	cat, err := s.loadFixture()
	if err != nil {
		return Result{}, err
	}
	rows, err := s.store.ListPlacementAnswers(ctx, runID)
	if err != nil {
		return Result{}, err
	}
	answered := map[string]int{}
	for _, row := range rows {
		if row.Rating.Valid {
			answered[row.ItemID] = int(row.Rating.Int64)
		}
	}
	var tallies [bands]BandTally
	for _, it := range cat {
		if it.Band < 0 || it.Band >= bands {
			continue
		}
		if rating, ok := answered[it.ID]; ok {
			tallies[it.Band].Ratings = append(tallies[it.Band].Ratings, rating)
			tallies[it.Band].Dealable++
			continue
		}
		has, err := s.hasCard(ctx, run.UserID, it.TextNFC)
		if err != nil {
			return Result{}, err
		}
		if !has {
			tallies[it.Band].Dealable++
		}
	}
	for band := range tallies {
		if tallies[band].Dealable > BandMax {
			tallies[band].Dealable = BandMax
		}
	}
	result := Score(tallies)
	now := s.clock.Now()
	result.RunID = run.ID
	result.UserID = run.UserID
	result.StartedAt = run.StartedAt
	result.FinishedAt = now
	result.Sittings = run.SittingNo
	body, err := json.Marshal(storedResult{Bands: result.Bands, EstimatedKnownWords: result.EstimatedKnownWords})
	if err != nil {
		return Result{}, err
	}
	if err := s.store.FinishPlacement(ctx, runID, now, result.Level, result.StopReason, string(body)); err != nil {
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

func (s *Service) resultFrom(ctx context.Context, run db.PlacementRun) (Result, error) {
	var stored storedResult
	if run.Result != "" {
		if err := json.Unmarshal([]byte(run.Result), &stored); err != nil {
			return Result{}, fmt.Errorf("placement: run %d result: %w", run.ID, err)
		}
	}
	level := -1
	if run.Level.Valid {
		level = int(run.Level.Int64)
	}
	rated, err := s.store.CountPlacementRatings(ctx, run.ID)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		RunID:               run.ID,
		UserID:              run.UserID,
		StartedAt:           run.StartedAt,
		FinishedAt:          run.FinishedAt,
		Level:               level,
		CardsRated:          rated,
		StopReason:          run.StopReason,
		Sittings:            run.SittingNo,
		Bands:               stored.Bands,
		EstimatedKnownWords: stored.EstimatedKnownWords,
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
	it, ok := indexItems(cat)[id]
	if !ok {
		return fixtureItem{}, fmt.Errorf("placement: item %s is not in the fixture", id)
	}
	return it, nil
}

func (s *Service) hasCard(ctx context.Context, userID int64, text string) (bool, error) {
	lexemeID, _, _, _, ok, err := s.store.LexemeByText(ctx, text)
	if err != nil || !ok {
		return false, err
	}
	_, has, err := s.store.CardID(ctx, userID, lexemeID, fsrs.DirHeEn)
	return has, err
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

func indexItems(items []fixtureItem) map[string]fixtureItem {
	out := make(map[string]fixtureItem, len(items))
	for _, it := range items {
		out[it.ID] = it
	}
	return out
}

func unanswered(rows []db.PlacementAnswer) (db.PlacementAnswer, bool) {
	for _, row := range rows {
		if !row.Rating.Valid {
			return row, true
		}
	}
	return db.PlacementAnswer{}, false
}

func ratedCount(rows []db.PlacementAnswer) int {
	n := 0
	for _, row := range rows {
		if row.Rating.Valid {
			n++
		}
	}
	return n
}

func sittingRated(rows []db.PlacementAnswer, sitting int) int {
	n := 0
	for _, row := range rows {
		if row.SittingNo == sitting && row.Rating.Valid {
			n++
		}
	}
	return n
}

func bandCounts(answered map[string]int, byID map[string]fixtureItem) [bands]bandCount {
	var counts [bands]bandCount
	for id, rating := range answered {
		it, ok := byID[id]
		if !ok || it.Band < 0 || it.Band >= bands {
			continue
		}
		counts[it.Band].add(rating)
	}
	return counts
}

func bandTripped(counts [bands]bandCount) bool {
	for band := range counts {
		if tripped(counts[band]) {
			return true
		}
	}
	return false
}
