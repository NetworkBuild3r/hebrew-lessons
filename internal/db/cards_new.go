package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// NewCardWordResult is one lexeme's outcome from CreateNewCards.
type NewCardWordResult struct {
	LexemeID int64
	Created  []string
}

// NewCardsResult is the outcome of CreateNewCards for every requested word.
type NewCardsResult struct {
	Words []NewCardWordResult
	Due   time.Time
}

// insertNewCard inserts one new-state card with INSERT OR IGNORE.
// It returns whether this call created the row. Shared by RecordTaught and
// CreateNewCards (INIT-027/SPEC-005). createdAtText is FormatTime of the clock
// moment this insert represents.
func insertNewCard(ctx context.Context, tx *sql.Tx, learnerID, lexemeID int64, skill, dueText, createdAtText string) (bool, error) {
	res, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO cards (
			user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, created_at
		) VALUES (?, ?, ?, 'new', ?, 0, 0, 0, 0, ?)`,
		learnerID, lexemeID, skill, dueText, createdAtText,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// CreateNewCards inserts new-state cards for the named lexemes and skills.
// skills(hasClip) lists the skills to try for that lexeme. INSERT OR IGNORE
// leaves an existing card unchanged. It writes no taught_words, attempts,
// exposures, or review_log. INIT-027/SPEC-005.
func (s *Store) CreateNewCards(ctx context.Context, learnerID int64, lexemeIDs []int64, due time.Time, skills func(hasClip bool) []string) (NewCardsResult, error) {
	out := NewCardsResult{Due: due, Words: make([]NewCardWordResult, 0, len(lexemeIDs))}
	conn, err := s.connOrErr()
	if err != nil {
		return out, err
	}
	if learnerID <= 0 {
		return out, errors.New("db: no learner")
	}
	if skills == nil {
		return out, errors.New("db: no skills")
	}
	if len(lexemeIDs) == 0 {
		return out, nil
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("db: begin new cards: %w", err)
	}
	defer tx.Rollback()

	dueText := FormatTime(due)
	for _, lexemeID := range lexemeIDs {
		hasClip, err := lexemeHasStoredClip(ctx, tx, lexemeID)
		if err != nil {
			return out, err
		}
		word := NewCardWordResult{LexemeID: lexemeID, Created: []string{}}
		for _, skill := range skills(hasClip) {
			if skill == "" {
				continue
			}
			inserted, err := insertNewCard(ctx, tx, learnerID, lexemeID, skill, dueText, dueText)
			if err != nil {
				return out, fmt.Errorf("db: new card: %w", err)
			}
			if inserted {
				word.Created = append(word.Created, skill)
			}
		}
		out.Words = append(out.Words, word)
	}
	if err := tx.Commit(); err != nil {
		return out, fmt.Errorf("db: commit new cards: %w", err)
	}
	return out, nil
}

// CountDueNewCards counts the learner's unsuspended study cards that have no
// review yet and are due at or before now — the same cards classify treats as
// kindNew. INIT-027/SPEC-005.
func (s *Store) CountDueNewCards(ctx context.Context, userID int64, now time.Time) (int, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	var n int
	err = conn.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM cards AS c
		WHERE c.user_id = ?
		  AND c.suspended = 0
		  AND c.skill IN ('read', 'recall', 'listen', 'spell')
		  AND c.due <= ?
		  AND NOT EXISTS (
		    SELECT 1 FROM review_log AS rl
		    WHERE rl.card_id = c.id AND rl.reviewed_at <= ?
		  )
`+introNotFinished+`
`, userID, FormatTime(now), FormatTime(now), LearnerScope(userID)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("db: count due new: %w", err)
	}
	return n, nil
}

// LearnerCardLexemeIDs returns every lexeme id that already has a card for the
// learner, any skill. INIT-027/SPEC-005.
func (s *Store) LearnerCardLexemeIDs(ctx context.Context, userID int64) (map[int64]struct{}, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT DISTINCT lexeme_id FROM cards WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("db: learner card lexemes: %w", err)
	}
	defer rows.Close()
	out := map[int64]struct{}{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("db: learner card lexemes: %w", err)
		}
		out[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: learner card lexemes: %w", err)
	}
	return out, nil
}
