package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TaughtWordResult is one word's outcome from RecordTaught.
type TaughtWordResult struct {
	LexemeID int64
	Already  bool
	Created  []string
	HadCard  []string
}

// TaughtResult is the outcome of RecordTaught for every requested word.
type TaughtResult struct {
	Words []TaughtWordResult
	Due   time.Time
}

// RemoveTaughtWordResult is one word's outcome from RemoveTaughtRows.
type RemoveTaughtWordResult struct {
	LexemeID int64
	Removed  []string
	Kept     []string
}

// RemoveTaughtResult is the outcome of RemoveTaughtRows.
type RemoveTaughtResult struct {
	Words []RemoveTaughtWordResult
}

// RecordTaught inserts taught_words rows and new schedule cards in one
// transaction. due is the study-day cutoff already computed by the caller.
// Skills attempted are read, recall, and listen when the lexeme has a stored
// clip (non-empty lexemes.audio_id, or an audio_assets row for its text — the
// same fact lookup reports as hasAudio / audioPresent). INSERT OR IGNORE
// leaves existing cards unchanged. cards_created lists only skills this call
// inserted. A duplicate (learner, lexeme, day) reports Already and writes
// nothing for that word. INIT-026/SPEC-005.
func (s *Store) RecordTaught(ctx context.Context, learnerID int64, lexemeIDs []int64, on string, due time.Time) (TaughtResult, error) {
	out := TaughtResult{Due: due, Words: make([]TaughtWordResult, 0, len(lexemeIDs))}
	conn, err := s.connOrErr()
	if err != nil {
		return out, err
	}
	if learnerID <= 0 {
		return out, errors.New("db: no learner")
	}
	if len(lexemeIDs) == 0 {
		return out, errors.New("db: no lexemes")
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("db: begin taught: %w", err)
	}
	defer tx.Rollback()

	dueText := FormatTime(due)
	createdAt := FormatTime(s.clock.Now().UTC())

	for _, lexemeID := range lexemeIDs {
		word := TaughtWordResult{LexemeID: lexemeID}
		var exists int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM taught_words
			WHERE learner_id = ? AND lexeme_id = ? AND taught_on = ?`,
			learnerID, lexemeID, on,
		).Scan(&exists); err != nil {
			return out, fmt.Errorf("db: taught exists: %w", err)
		}
		if exists > 0 {
			word.Already = true
			out.Words = append(out.Words, word)
			continue
		}

		hasAudio, err := lexemeHasStoredClip(ctx, tx, lexemeID)
		if err != nil {
			return out, err
		}
		skills := []string{"read", "recall"}
		if hasAudio {
			skills = append(skills, "listen")
		}

		var created, hadCard []string
		for _, skill := range skills {
			res, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO cards (
					user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses
				) VALUES (?, ?, ?, 'new', ?, 0, 0, 0, 0)`,
				learnerID, lexemeID, skill, dueText,
			)
			if err != nil {
				return out, fmt.Errorf("db: taught card: %w", err)
			}
			n, err := res.RowsAffected()
			if err != nil {
				return out, fmt.Errorf("db: taught card: %w", err)
			}
			if n > 0 {
				created = append(created, skill)
			} else {
				hadCard = append(hadCard, skill)
			}
		}
		if created == nil {
			created = []string{}
		}
		if hadCard == nil {
			hadCard = []string{}
		}
		word.Created = created
		word.HadCard = hadCard

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO taught_words (learner_id, lexeme_id, taught_on, cards_created, created_at)
			VALUES (?, ?, ?, ?, ?)`,
			learnerID, lexemeID, on, strings.Join(created, ","), createdAt,
		); err != nil {
			return out, fmt.Errorf("db: insert taught: %w", err)
		}
		out.Words = append(out.Words, word)
	}

	if err := tx.Commit(); err != nil {
		return out, fmt.Errorf("db: commit taught: %w", err)
	}
	return out, nil
}

// RemoveTaughtRows deletes taught_words for the learner, day, and lexemes, and
// deletes only cards listed in each row's cards_created that still have
// reps = 0 and no review_log row. Reviewed cards are kept and reported.
// INIT-026/SPEC-005.
func (s *Store) RemoveTaughtRows(ctx context.Context, learnerID int64, on string, lexemeIDs []int64) (RemoveTaughtResult, error) {
	out := RemoveTaughtResult{Words: make([]RemoveTaughtWordResult, 0, len(lexemeIDs))}
	conn, err := s.connOrErr()
	if err != nil {
		return out, err
	}
	if learnerID <= 0 {
		return out, errors.New("db: no learner")
	}
	if len(lexemeIDs) == 0 {
		return out, errors.New("db: no lexemes")
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("db: begin remove taught: %w", err)
	}
	defer tx.Rollback()

	for _, lexemeID := range lexemeIDs {
		word := RemoveTaughtWordResult{LexemeID: lexemeID, Removed: []string{}, Kept: []string{}}
		var id int64
		var cardsCreated string
		err := tx.QueryRowContext(ctx, `
			SELECT id, cards_created FROM taught_words
			WHERE learner_id = ? AND lexeme_id = ? AND taught_on = ?`,
			learnerID, lexemeID, on,
		).Scan(&id, &cardsCreated)
		if errors.Is(err, sql.ErrNoRows) {
			out.Words = append(out.Words, word)
			continue
		}
		if err != nil {
			return out, fmt.Errorf("db: taught row: %w", err)
		}

		for _, skill := range splitSkills(cardsCreated) {
			var cardID int64
			var reps int
			err := tx.QueryRowContext(ctx, `
				SELECT id, reps FROM cards
				WHERE user_id = ? AND lexeme_id = ? AND skill = ?`,
				learnerID, lexemeID, skill,
			).Scan(&cardID, &reps)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return out, fmt.Errorf("db: taught card lookup: %w", err)
			}
			var reviews int
			if err := tx.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM review_log WHERE card_id = ?`, cardID,
			).Scan(&reviews); err != nil {
				return out, fmt.Errorf("db: taught review_log: %w", err)
			}
			if reps == 0 && reviews == 0 {
				if _, err := tx.ExecContext(ctx, `DELETE FROM cards WHERE id = ? AND user_id = ?`, cardID, learnerID); err != nil {
					return out, fmt.Errorf("db: delete taught card: %w", err)
				}
				word.Removed = append(word.Removed, skill)
			} else {
				word.Kept = append(word.Kept, skill)
			}
		}

		if _, err := tx.ExecContext(ctx, `
			DELETE FROM taught_words WHERE id = ? AND learner_id = ?`, id, learnerID); err != nil {
			return out, fmt.Errorf("db: delete taught: %w", err)
		}
		out.Words = append(out.Words, word)
	}

	if err := tx.Commit(); err != nil {
		return out, fmt.Errorf("db: commit remove taught: %w", err)
	}
	return out, nil
}

// lexemeHasStoredClip reports whether lookup would treat the lexeme as having
// audio: a non-empty lexemes.audio_id, or a clip in audio_assets for its text.
func lexemeHasStoredClip(ctx context.Context, tx *sql.Tx, lexemeID int64) (bool, error) {
	var audio sql.NullString
	var text string
	err := tx.QueryRowContext(ctx, `
		SELECT text_nfc, audio_id FROM lexemes WHERE id = ?`, lexemeID,
	).Scan(&text, &audio)
	if errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("db: lexeme %d not found", lexemeID)
	}
	if err != nil {
		return false, fmt.Errorf("db: lexeme audio: %w", err)
	}
	if audio.Valid && audio.String != "" {
		return true, nil
	}
	var n int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audio_assets WHERE text_nfc = ?`, text,
	).Scan(&n); err != nil {
		return false, fmt.Errorf("db: audio_assets: %w", err)
	}
	return n > 0, nil
}

func splitSkills(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
