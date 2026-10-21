package mcpstore

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

// UnitsInScope maps db.UnitsInScope onto the outline get_context returns.
// A nil learnerID is shared units only. db.UnitsInScope takes an int64 and
// always includes that learner's private units, so the nil case drops them.
func (s *Store) UnitsInScope(ctx context.Context, learnerID *int64) ([]mcp.UnitOutline, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	var id int64
	if learnerID != nil {
		id = *learnerID
	}
	rows, err := s.db.UnitsInScope(ctx, id)
	if err != nil {
		return nil, err
	}
	out := []mcp.UnitOutline{}
	for _, row := range rows {
		if learnerID == nil && row.Scope != mcp.ScopeShared {
			continue
		}
		out = append(out, mcp.UnitOutline{
			ID:       row.ID,
			Position: row.Position,
			Title:    row.Title,
			Status:   row.Status,
			Scope:    row.Scope,
		})
	}
	return out, nil
}

// LatestPlacement copies db.LatestPlacement. Finished, Sittings, CardsRated,
// StopReason, and each band tally are the stored values.
func (s *Store) LatestPlacement(ctx context.Context, userID int64) (mcp.Placement, bool, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.Placement{}, false, err
	}
	row, ok, err := s.db.LatestPlacement(ctx, userID)
	if err != nil || !ok {
		return mcp.Placement{}, ok, err
	}
	var bands []mcp.BandScore
	if row.BandScores != nil {
		bands = make([]mcp.BandScore, len(row.BandScores))
		for i, band := range row.BandScores {
			bands[i] = mcp.BandScore{
				Band:         band.Band,
				Rated:        band.Rated,
				Again:        band.Again,
				Hard:         band.Hard,
				Good:         band.Good,
				Easy:         band.Easy,
				DontKnow:     band.DontKnow,
				GoodPlusRate: band.GoodPlusRate,
			}
		}
	}
	return mcp.Placement{
		TakenAt:                row.TakenAt,
		Finished:               row.Finished,
		Incomplete:             row.Incomplete,
		Sittings:               row.Sittings,
		Level:                  row.Level,
		CardsRated:             row.CardsRated,
		StopReason:             row.StopReason,
		BandScores:             bands,
		EstimatedKnownWords:    row.EstimatedKnownWords,
		SuggestedStartPosition: row.SuggestedStartPosition,
	}, true, nil
}

// PlacementKnownLexemes is db.PlacementKnownLexemes: Good and Easy answers
// on the newest placement run.
func (s *Store) PlacementKnownLexemes(ctx context.Context, userID int64) ([]string, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return s.db.PlacementKnownLexemes(ctx, userID)
}

// PlacementLearningLexemes is db.PlacementLearningLexemes: don't-know, Again,
// and Hard answers on the newest placement run.
func (s *Store) PlacementLearningLexemes(ctx context.Context, userID int64) ([]string, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return s.db.PlacementLearningLexemes(ctx, userID)
}

// struggleWindow is the span of the confusion list and of Misses7d.
const struggleWindow = learner.MissWindowDays * 24 * time.Hour

// StruggleStats is the learner's struggle picture for get_context. Lexemes are the
// words the learner model calls struggling (learner.Model.List with that state),
// sorted by pointed text. The shape is the one db.StruggleStats gave: Reviews, Lapses,
// and AgainRate come from the review_log, Misses7d is Again ratings in the last 7 days,
// and ConfusionKeys are the keys confused in that window. Confusions are the last 7
// days of confusion_misses. Top is the first topN texts ordered by Misses7d, then
// Lapses, then AgainRate, ties by text. topN of 0 or less leaves Top empty.
func (s *Store) StruggleStats(ctx context.Context, userID int64, topN int) (mcp.Struggle, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.Struggle{}, err
	}
	model := learner.New(s.db, nil)
	var words []learner.WordState
	var after learner.Cursor
	for {
		page, next, err := model.List(ctx, userID, learner.Filter{
			State: learner.Struggling,
			Limit: learner.MaxPageSize,
			After: after,
		})
		if err != nil {
			return mcp.Struggle{}, err
		}
		words = append(words, page...)
		if next == "" {
			break
		}
		after = next
	}
	pairs, err := model.ConfusionPairs(ctx, userID, struggleWindow)
	if err != nil {
		return mcp.Struggle{}, err
	}

	var lexemes []mcp.StruggleLexeme
	if len(words) > 0 {
		lexemes = make([]mcp.StruggleLexeme, len(words))
		for i, w := range words {
			last := w.Review.LastReview
			if last.IsZero() {
				last = w.LastSeen
			}
			lexemes[i] = mcp.StruggleLexeme{
				LexemeID:      w.LexemeID,
				TextNFC:       w.Text,
				Gloss:         w.Gloss,
				Reviews:       w.Review.Reviews,
				Lapses:        w.Review.Lapses,
				AgainRate:     w.Review.AgainRate,
				Misses7d:      w.Review.Misses7d,
				LastSeen:      last,
				ConfusionKeys: w.Review.ConfusionKeys,
			}
		}
		slices.SortFunc(lexemes, func(a, b mcp.StruggleLexeme) int {
			return cmp.Compare(a.TextNFC, b.TextNFC)
		})
	}
	var confusions []mcp.StruggleConfusion
	if len(pairs) > 0 {
		confusions = make([]mcp.StruggleConfusion, len(pairs))
		for i, pair := range pairs {
			confusions[i] = mcp.StruggleConfusion{
				Kind:   pair.Kind,
				A:      pair.A,
				B:      pair.B,
				Count:  pair.Count,
				LastAt: pair.LastAt,
			}
		}
	}
	var top []string
	if topN > 0 && len(lexemes) > 0 {
		ranked := slices.Clone(lexemes)
		slices.SortFunc(ranked, func(a, b mcp.StruggleLexeme) int {
			if n := cmp.Compare(b.Misses7d, a.Misses7d); n != 0 {
				return n
			}
			if n := cmp.Compare(b.Lapses, a.Lapses); n != 0 {
				return n
			}
			if n := cmp.Compare(b.AgainRate, a.AgainRate); n != 0 {
				return n
			}
			return cmp.Compare(a.TextNFC, b.TextNFC)
		})
		if len(ranked) > topN {
			ranked = ranked[:topN]
		}
		top = make([]string, len(ranked))
		for i, lx := range ranked {
			top[i] = lx.TextNFC
		}
	}
	return mcp.Struggle{Lexemes: lexemes, Confusions: confusions, Top: top}, nil
}

// CompletedSharedUnits maps db.CompletedSharedUnits.
func (s *Store) CompletedSharedUnits(ctx context.Context, userID int64) ([]mcp.UnitCompletion, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.CompletedSharedUnits(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]mcp.UnitCompletion, len(rows))
	for i, row := range rows {
		out[i] = mcp.UnitCompletion{
			UnitID:      row.UnitID,
			Position:    row.Position,
			CompletedAt: row.CompletedAt,
		}
	}
	return out, nil
}

// LearnerKnownLexemes is db.LearnerKnownLexemes.
func (s *Store) LearnerKnownLexemes(ctx context.Context, userID int64) ([]string, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return s.db.LearnerKnownLexemes(ctx, userID)
}
