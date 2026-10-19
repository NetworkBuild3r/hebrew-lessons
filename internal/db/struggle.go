package db

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"
)

// struggleWindow is how far back LexemeStruggles counts Again ratings.
const struggleWindow = 7 * 24 * time.Hour

// LexemeStruggle is one reviewed lexeme, for the learner's weak-word list.
// Reviews is the number of review_log rows. Lapses is the sum of cards.lapses
// for this learner and lexeme. AgainRate is Again ratings divided by Reviews.
// Misses7d is Again ratings in the last 7 days on the store clock.
type LexemeStruggle struct {
	LexemeID     int64
	TextNFC      string
	Gloss        string
	Reviews      int
	Lapses       int
	AgainRate    float64
	Misses7d     int
	LastReviewAt time.Time
}

// LexemeStruggles returns reviewed lexemes for the learner.
// The order is Misses7d, then Lapses, then AgainRate, then TextNFC.
// topN ≤ 0 returns every reviewed lexeme.
func (s *Store) LexemeStruggles(ctx context.Context, userID int64, topN int) ([]LexemeStruggle, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	if s.clock == nil {
		return nil, fmt.Errorf("db: clock is not configured")
	}
	since := FormatTime(s.clock.Now().Add(-struggleWindow))
	rows, err := conn.QueryContext(ctx, `
		SELECT
			l.id,
			l.text_nfc,
			l.gloss_en,
			COUNT(rl.id),
			(
				SELECT COALESCE(SUM(c2.lapses), 0)
				FROM cards AS c2
				WHERE c2.user_id = c.user_id AND c2.lexeme_id = c.lexeme_id
			),
			COALESCE(SUM(CASE WHEN rl.rating = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN rl.rating = 1 AND rl.reviewed_at >= ? THEN 1 ELSE 0 END), 0),
			MAX(rl.reviewed_at)
		FROM cards AS c
		JOIN lexemes AS l ON l.id = c.lexeme_id
		JOIN review_log AS rl ON rl.card_id = c.id
		WHERE c.user_id = ?
		GROUP BY l.id, l.text_nfc, l.gloss_en`, since, userID)
	if err != nil {
		return nil, fmt.Errorf("db: lexeme struggles: %w", err)
	}
	defer rows.Close()
	var out []LexemeStruggle
	for rows.Next() {
		var row LexemeStruggle
		var last string
		var agains int
		if err := rows.Scan(
			&row.LexemeID, &row.TextNFC, &row.Gloss, &row.Reviews, &row.Lapses,
			&agains, &row.Misses7d, &last,
		); err != nil {
			return nil, fmt.Errorf("db: lexeme struggles: %w", err)
		}
		if row.Reviews > 0 {
			row.AgainRate = float64(agains) / float64(row.Reviews)
		}
		row.LastReviewAt, err = ParseTime(last)
		if err != nil {
			return nil, fmt.Errorf("db: lexeme struggles: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: lexeme struggles: %w", err)
	}
	slices.SortFunc(out, func(a, b LexemeStruggle) int {
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
	if topN > 0 && len(out) > topN {
		out = out[:topN]
	}
	return out, nil
}
