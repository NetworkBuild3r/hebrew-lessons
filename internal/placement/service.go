package placement

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

const bands = 5 // bands 0 through 4

// Service runs the yes/no placement test.
// The clock stamps started_at, finished_at, and the shuffle seed.
type Service struct {
	store *db.Store
	clock clock.Clock
}

// New uses store and clk. A nil clock uses the clock the store was opened with.
func New(store *db.Store, clk clock.Clock) *Service {
	if clk == nil && store != nil {
		clk = store.Clock()
	}
	return &Service{store: store, clock: clk}
}

func (s *Service) ready() error {
	if s == nil || s.store == nil {
		return errors.New("placement: service is not configured")
	}
	if s.clock == nil {
		return errors.New("placement: clock is not configured")
	}
	return nil
}

// Start opens a run and returns the words the learner sees, in deal order.
func (s *Service) Start(ctx context.Context, userID int64) (int64, []Item, error) {
	if err := s.ready(); err != nil {
		return 0, nil, err
	}
	cat, err := catalog()
	if err != nil {
		return 0, nil, err
	}
	if len(cat) == 0 {
		return 0, nil, errors.New("placement: fixture is empty")
	}
	now := s.clock.Now()
	seed := now.UnixNano()
	if seed == 0 {
		seed = 1
	}
	runID, err := s.store.InsertPlacementRun(ctx, userID, now, seed)
	if err != nil {
		return 0, nil, err
	}
	dealt := deal(seed, cat)
	items := make([]Item, len(dealt))
	for i, it := range dealt {
		items[i] = it.public()
	}
	return runID, items, nil
}

// Answer records whether the learner said they know itemID.
// A second answer for the same item replaces the first.
func (s *Service) Answer(ctx context.Context, runID int64, itemID string, yes bool) error {
	if err := s.ready(); err != nil {
		return err
	}
	if itemID == "" {
		return errors.New("placement: item id is empty")
	}
	if _, err := s.loadOpen(ctx, runID); err != nil {
		return err
	}
	cat, err := catalog()
	if err != nil {
		return err
	}
	if !knownItem(cat, itemID) {
		return fmt.Errorf("placement: item %s is not in the fixture", itemID)
	}
	return s.store.UpsertPlacementAnswer(ctx, runID, itemID, yes)
}

// Finish scores the run and, when the run is reliable, seeds known lexemes.
func (s *Service) Finish(ctx context.Context, runID int64) (Result, error) {
	if err := s.ready(); err != nil {
		return Result{}, err
	}
	run, err := s.loadOpen(ctx, runID)
	if err != nil {
		return Result{}, err
	}
	cat, err := catalog()
	if err != nil {
		return Result{}, err
	}
	rows, err := s.store.ListPlacementAnswers(ctx, runID)
	if err != nil {
		return Result{}, err
	}
	said := map[string]bool{}
	for _, row := range rows {
		said[row.ItemID] = row.SaidYes
	}
	if len(said) != len(cat) {
		return Result{}, errors.New("placement: not every item has an answer")
	}
	answers := make([]Answer, 0, len(cat))
	items := make([]ScoreItem, len(cat))
	for i, it := range cat {
		yes, ok := said[it.ID]
		if !ok {
			return Result{}, errors.New("placement: not every item has an answer")
		}
		answers = append(answers, Answer{ItemID: it.ID, Yes: yes})
		items[i] = ScoreItem{ID: it.ID, Band: it.Band, Pseudo: it.Pseudo}
	}

	result := Score(answers, items)
	seeds := seedsFor(cat, said, result)
	now := s.clock.Now()
	result.RunID = runID
	result.UserID = run.UserID
	result.StartedAt = run.StartedAt
	result.FinishedAt = now

	body, err := json.Marshal(storedResult{Bands: result.Bands, EstimatedKnownWords: result.EstimatedKnownWords})
	if err != nil {
		return Result{}, err
	}
	dbSeeds := make([]db.PlacementSeed, len(seeds))
	for i, seed := range seeds {
		dbSeeds[i] = db.PlacementSeed{TextNFC: seed.text, GlossEN: seed.gloss, POS: seed.pos}
	}
	if err := s.store.FinishPlacement(ctx, runID, now, result.FalseAlarmRate, result.Reliable, result.Level, string(body), dbSeeds); err != nil {
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

// Latest returns the learner's most recently finished run.
// It returns (nil, nil) when the learner has not finished one.
func (s *Service) Latest(ctx context.Context, userID int64) (*Result, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	run, err := s.store.LatestFinishedPlacement(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result, err := resultFromRun(run)
	if err != nil {
		return nil, err
	}
	place, ok, err := s.store.LatestPlacement(ctx, userID)
	if err != nil {
		return nil, err
	}
	if ok {
		result.SuggestedStartPosition = place.SuggestedStartPosition
	}
	return result, nil
}

// KnownLexemeIDs is the union documented on the package for get_context.
func (s *Service) KnownLexemeIDs(ctx context.Context, userID int64) ([]int64, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.store.KnownLexemeIDs(ctx, userID)
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

func knownItem(cat []fixtureItem, id string) bool {
	for _, it := range cat {
		if it.ID == id {
			return true
		}
	}
	return false
}

type wordSeed struct {
	text, gloss, pos string
}

// seedsFor lists the yes answers on real words in passed bands.
// An unreliable run seeds nothing. Pseudo-words are never seeded.
func seedsFor(cat []fixtureItem, said map[string]bool, result Result) []wordSeed {
	if !result.Reliable {
		return nil
	}
	var seeds []wordSeed
	for _, it := range cat {
		if it.Pseudo || it.Band == nil || *it.Band < 0 || *it.Band >= len(result.Bands) {
			continue
		}
		if !result.Bands[*it.Band].Passed || !said[it.ID] {
			continue
		}
		seeds = append(seeds, wordSeed{text: it.TextNFC, gloss: it.GlossEN, pos: it.POS})
	}
	return seeds
}

// correct is (h − f) / (1 − f). A band passes at or above passAt.
// blocked is true when f is 1, which passes nothing.
func correct(h, f float64, blocked bool) (float64, bool) {
	if blocked || f >= 1 {
		return 0, false
	}
	c := (h - f) / (1 - f)
	return c, c >= passAt
}

func resultFromRun(run db.PlacementRun) (*Result, error) {
	var stored storedResult
	if err := json.Unmarshal([]byte(run.Result), &stored); err != nil {
		return nil, fmt.Errorf("placement: run %d result: %w", run.ID, err)
	}
	if !run.Level.Valid {
		return nil, fmt.Errorf("placement: run %d has no level", run.ID)
	}
	rate := 0.0
	if run.FalseAlarm.Valid {
		rate = run.FalseAlarm.Float64
	}
	return &Result{
		RunID:               run.ID,
		UserID:              run.UserID,
		StartedAt:           run.StartedAt,
		FinishedAt:          run.FinishedAt,
		FalseAlarmRate:      rate,
		Reliable:            run.Reliable,
		Level:               int(run.Level.Int64),
		Bands:               stored.Bands,
		EstimatedKnownWords: stored.EstimatedKnownWords,
	}, nil
}
