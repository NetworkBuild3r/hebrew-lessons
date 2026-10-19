package mcpstore

import (
	"context"
	"fmt"
	"slices"
	"strings"

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

// StruggleStats maps db.StruggleStats. ConfusionKeys are the keys stored
// against confusion_misses.lexeme_id. Top is the ranked text.
func (s *Store) StruggleStats(ctx context.Context, userID int64, topN int) (mcp.Struggle, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.Struggle{}, err
	}
	row, err := s.db.StruggleStats(ctx, userID, topN)
	if err != nil {
		return mcp.Struggle{}, err
	}
	var lexemes []mcp.StruggleLexeme
	if row.Lexemes != nil {
		lexemes = make([]mcp.StruggleLexeme, len(row.Lexemes))
		for i, lx := range row.Lexemes {
			lexemes[i] = mcp.StruggleLexeme{
				LexemeID:      lx.LexemeID,
				TextNFC:       lx.TextNFC,
				Gloss:         lx.Gloss,
				Reviews:       lx.Reviews,
				Lapses:        lx.Lapses,
				AgainRate:     lx.AgainRate,
				Misses7d:      lx.Misses7d,
				LastSeen:      lx.LastSeen,
				ConfusionKeys: lx.ConfusionKeys,
			}
		}
	}
	var confusions []mcp.StruggleConfusion
	if row.Confusions != nil {
		confusions = make([]mcp.StruggleConfusion, len(row.Confusions))
		for i, pair := range row.Confusions {
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
	if row.Top != nil {
		top = make([]string, len(row.Top))
		for i, lx := range row.Top {
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

// LearnerKnownLexemes is KnownLexemeIDs mapped to pointed text and sorted.
func (s *Store) LearnerKnownLexemes(ctx context.Context, userID int64) ([]string, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	ids, err := s.db.KnownLexemeIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []string{}, nil
	}
	conn, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	marks := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		marks[i] = "?"
		args[i] = id
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT text_nfc FROM lexemes WHERE id IN (`+strings.Join(marks, ", ")+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("mcpstore: known lexemes: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, fmt.Errorf("mcpstore: known lexemes: %w", err)
		}
		out = append(out, text)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mcpstore: known lexemes: %w", err)
	}
	slices.Sort(out)
	return out, nil
}
