package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// AttemptTallyRow is the checked-answer count for one skill and lexeme in a window.
// Answers are attempts with source in review, lesson, or practice. Placement is
// excluded. A miss is an answer that was not correct (dont_know counts as a miss).
type AttemptTallyRow struct {
	Skill    string
	LexemeID int64
	Answers  int
	Misses   int
	LastMiss time.Time // Zero when Misses is 0.
}

// AttemptTally returns per-(skill, lexeme) answer and miss counts for one learner
// between since and asOf inclusive. Rows with zero answers are omitted.
func (s *Store) AttemptTally(ctx context.Context, userID int64, since, asOf time.Time) ([]AttemptTallyRow, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, fmt.Errorf("db: attempt tally needs a learner")
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT skill, lexeme_id,
			COUNT(*),
			COALESCE(SUM(CASE WHEN correct = 0 OR dont_know = 1 THEN 1 ELSE 0 END), 0),
			MAX(CASE WHEN correct = 0 OR dont_know = 1 THEN created_at END)
		FROM attempts
		WHERE user_id = ?
		  AND created_at >= ?
		  AND created_at <= ?
		  AND source IN ('review', 'lesson', 'practice')
		GROUP BY skill, lexeme_id
		ORDER BY skill, lexeme_id`,
		userID, FormatTime(since), FormatTime(asOf),
	)
	if err != nil {
		return nil, fmt.Errorf("db: attempt tally: %w", err)
	}
	defer rows.Close()
	var out []AttemptTallyRow
	for rows.Next() {
		var row AttemptTallyRow
		var last sql.NullString
		if err := rows.Scan(&row.Skill, &row.LexemeID, &row.Answers, &row.Misses, &last); err != nil {
			return nil, fmt.Errorf("db: attempt tally: %w", err)
		}
		if last.Valid && last.String != "" {
			at, err := ParseTime(last.String)
			if err != nil {
				return nil, fmt.Errorf("db: attempt tally: %w", err)
			}
			row.LastMiss = at
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: attempt tally: %w", err)
	}
	return out, nil
}

// CountLexemesWithAudio returns how many lexemes have a clip by the one
// lookup (INIT-038/SPEC-003). Listening askability reads this.
func (s *Store) CountLexemesWithAudio(ctx context.Context) (int, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	var n int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM lexemes AS l
		WHERE `+LexemeHasClipSQL("l")).Scan(&n); err != nil {
		return 0, fmt.Errorf("db: count lexemes with audio: %w", err)
	}
	return n, nil
}
