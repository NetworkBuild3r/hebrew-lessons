package mcpstore

import (
	"context"
	"errors"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

// LearnerCards maps db.CoachCards onto the card page get_context and
// get_learner_words return. Suspended cards are included.
func (s *Store) LearnerCards(ctx context.Context, q mcp.CardQuery) (mcp.CardPage, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.CardPage{}, err
	}
	page, err := s.db.CoachCards(ctx, db.CoachQuery{
		UserID:    q.UserID,
		Cursor:    q.Cursor,
		Limit:     q.Limit,
		State:     q.State,
		DueBefore: q.DueBefore,
	})
	if errors.Is(err, db.ErrBadCoachCursor) {
		return mcp.CardPage{}, mcp.ErrBadCardCursor
	}
	if err != nil {
		return mcp.CardPage{}, err
	}
	out := mcp.CardPage{Cards: make([]mcp.LearnerCard, 0, len(page.Cards)), NextCursor: page.NextCursor}
	for _, row := range page.Cards {
		out.Cards = append(out.Cards, coachCard(row))
	}
	return out, nil
}

// RecentAccuracy maps db.CoachAccuracy. The window ends at the database clock.
// Good and Easy are ratings 3 and 4. The last-50 ratings are any age.
// Lesson scores are finished lesson_runs in the same 7 days.
func (s *Store) RecentAccuracy(ctx context.Context, userID int64) (mcp.RecentAccuracy, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.RecentAccuracy{}, err
	}
	row, err := s.db.CoachAccuracy(
		ctx,
		userID,
		s.now(),
		time.Duration(mcp.AccuracyWindowDays)*24*time.Hour,
		mcp.AccuracyReviewCap,
	)
	if err != nil {
		return mcp.RecentAccuracy{}, err
	}
	return mcp.RecentAccuracy{
		WindowDays:      mcp.AccuracyWindowDays,
		Reviews7d:       coachCount(row.Reviews7d),
		LastReviews:     coachCount(row.LastReviews),
		LastReviewLimit: mcp.AccuracyReviewCap,
		Lessons7d:       coachLessons(row.Lessons7d),
	}, nil
}

func coachCard(row db.CoachCard) mcp.LearnerCard {
	card := mcp.LearnerCard{
		Lexeme:     row.Lexeme,
		POS:        row.POS,
		Gloss:      row.Gloss,
		Skill:      row.Skill,
		State:      row.State,
		Stability:  row.Stability,
		Difficulty: row.Difficulty,
		Due:        row.Due,
		Lapses:     row.Lapses,
		Reps:       row.Reps,
	}
	if row.HasTranslit {
		text := row.Translit
		card.Translit = &text
	}
	if row.HasRating {
		rating := row.LastRating
		card.LastRating = &rating
	}
	if row.LastText != "" {
		at := row.LastReview
		card.LastReview = &at
	}
	return card
}

func coachCount(row db.CoachCount) mcp.AccuracyCount {
	out := mcp.AccuracyCount{Count: row.Count, GoodOrEasy: row.GoodOrEasy}
	if row.Count > 0 {
		rate := float64(row.GoodOrEasy) / float64(row.Count)
		out.Rate = &rate
	}
	return out
}

func coachLessons(row db.CoachLessons) mcp.LessonAccuracy {
	out := mcp.LessonAccuracy{Finished: row.Finished, Scored: row.Scored}
	if row.Scored > 0 {
		mean := row.ScoreSum / float64(row.Scored)
		out.MeanScore = &mean
	}
	return out
}
