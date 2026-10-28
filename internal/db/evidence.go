package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Attempt is one row of attempts: one checked answer by one learner on one word.
// UnitID 0 is stored as NULL. CreatedAt is the time the answer was checked.
type Attempt struct {
	ID        int64
	UserID    int64
	LexemeID  int64
	Skill     string
	Activity  string
	Source    string
	UnitID    int64
	Prompt    string
	Response  string
	Correct   bool
	DontKnow  bool
	HintUsed  bool
	LatencyMS int64
	CreatedAt time.Time
}

// Confusion is one confusion_misses row written with an attempt.
// Kind is letter_pair, agreement, or other. Key is the pair, for example ב/כ.
type Confusion struct {
	Kind string
	Key  string
}

// CardStep decides, inside the attempt transaction, how the learner's card for
// the attempt's lexeme and skill changes. prev is the stored card, or nil when
// the learner has none. Return apply false to leave the cards alone.
// When apply is true, next is the card after the review and log is its review_log row.
// The card id and the log's card id are set by WriteAttempt.
//
// A CardStep runs while the transaction holds the only connection. It must not
// call the Store.
type CardStep func(prev *Card) (next Card, log ReviewLog, apply bool, err error)

// AttemptWrite is everything one checked answer writes.
// Step, Confusion and Extra are optional. Extra holds further confusion rows
// for the same attempt, for example the letter pairs found in a typed answer.
type AttemptWrite struct {
	Attempt   Attempt
	Step      CardStep
	Confusion *Confusion
	Extra     []Confusion
}

// AttemptResult says what WriteAttempt stored.
// CardID is 0 when no card was written. Scheduled is true when a card was
// inserted or updated and a review_log row was appended.
type AttemptResult struct {
	AttemptID int64
	CardID    int64
	Scheduled bool
}

// WriteAttempt appends the attempt row and, when w asks for them, the card
// update, the review_log row, and the confusion_misses row, in one transaction.
// If any step fails nothing is stored.
// The card is read inside the transaction, so two answers in a row for the
// same word cannot both start from the same stored state.
func (s *Store) WriteAttempt(ctx context.Context, w AttemptWrite) (AttemptResult, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return AttemptResult{}, err
	}
	a := w.Attempt
	if a.UserID == 0 || a.LexemeID == 0 {
		return AttemptResult{}, errors.New("db: attempt needs a learner and a lexeme")
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return AttemptResult{}, fmt.Errorf("db: begin attempt: %w", err)
	}
	defer tx.Rollback()

	var unit any
	if a.UnitID != 0 {
		unit = a.UnitID
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO attempts (
			user_id, lexeme_id, skill, activity, source, unit_id, prompt, response,
			correct, dont_know, hint_used, latency_ms, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.UserID, a.LexemeID, a.Skill, a.Activity, a.Source, unit, a.Prompt, a.Response,
		evidenceBool(a.Correct), evidenceBool(a.DontKnow), evidenceBool(a.HintUsed), a.LatencyMS, FormatTime(a.CreatedAt),
	)
	if err != nil {
		return AttemptResult{}, fmt.Errorf("db: insert attempt: %w", err)
	}
	out := AttemptResult{}
	out.AttemptID, err = res.LastInsertId()
	if err != nil {
		return AttemptResult{}, fmt.Errorf("db: insert attempt: %w", err)
	}

	if w.Step != nil {
		prev, err := cardTx(ctx, tx, a.UserID, a.LexemeID, a.Skill)
		if err != nil {
			return AttemptResult{}, err
		}
		next, log, apply, err := w.Step(prev)
		if err != nil {
			return AttemptResult{}, err
		}
		if apply {
			next.UserID, next.LexemeID, next.Skill = a.UserID, a.LexemeID, a.Skill
			if prev == nil {
				next.ID, err = insertCardTx(ctx, tx, next)
			} else {
				next.ID = prev.ID
				err = updateCardTx(ctx, tx, next)
			}
			if err != nil {
				return AttemptResult{}, err
			}
			log.CardID = next.ID
			if err := insertReviewTx(ctx, tx, log); err != nil {
				return AttemptResult{}, err
			}
			out.CardID = next.ID
			out.Scheduled = true
		}
	}

	confusions := w.Extra
	if w.Confusion != nil {
		confusions = append([]Confusion{*w.Confusion}, w.Extra...)
	}
	for _, c := range confusions {
		var lexeme any = a.LexemeID
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at)
			VALUES (?, ?, ?, ?, ?)`,
			a.UserID, c.Kind, c.Key, lexeme, FormatTime(a.CreatedAt),
		); err != nil {
			return AttemptResult{}, fmt.Errorf("db: insert confusion: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return AttemptResult{}, fmt.Errorf("db: commit attempt: %w", err)
	}
	return out, nil
}

// CorrectLatencies returns the latency in milliseconds of the learner's most
// recent correct attempts for one activity, newest first, at most limit.
// An attempt with a latency of 0 was not timed and is left out.
func (s *Store) CorrectLatencies(ctx context.Context, userID int64, activity string, limit int) ([]int64, error) {
	return s.correctLatencies(ctx, userID, activity, 0, time.Time{}, limit)
}

// CorrectLatenciesBefore is CorrectLatencies limited to attempts strictly
// before beforeID at beforeAt. INIT-023/SPEC-003 uses it when replaying a
// stored attempt so the median matches what Attempt saw before that row.
func (s *Store) CorrectLatenciesBefore(ctx context.Context, userID int64, activity string, beforeID int64, beforeAt time.Time, limit int) ([]int64, error) {
	return s.correctLatencies(ctx, userID, activity, beforeID, beforeAt, limit)
}

func (s *Store) correctLatencies(ctx context.Context, userID int64, activity string, beforeID int64, beforeAt time.Time, limit int) ([]int64, error) {
	if limit <= 0 {
		return nil, nil
	}
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	if beforeID > 0 {
		at := FormatTime(beforeAt)
		rows, err = conn.QueryContext(ctx, `
			SELECT latency_ms
			FROM attempts
			WHERE user_id = ? AND activity = ? AND correct = 1 AND latency_ms > 0
			  AND (created_at < ? OR (created_at = ? AND id < ?))
			ORDER BY created_at DESC, id DESC
			LIMIT ?`, userID, activity, at, at, beforeID, limit)
	} else {
		rows, err = conn.QueryContext(ctx, `
			SELECT latency_ms
			FROM attempts
			WHERE user_id = ? AND activity = ? AND correct = 1 AND latency_ms > 0
			ORDER BY created_at DESC, id DESC
			LIMIT ?`, userID, activity, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("db: correct latencies: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var ms int64
		if err := rows.Scan(&ms); err != nil {
			return nil, fmt.Errorf("db: correct latencies: %w", err)
		}
		out = append(out, ms)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: correct latencies: %w", err)
	}
	return out, nil
}

// LexemeSkill is one learner and one word, used when scanning evidence by skill.
type LexemeSkill struct {
	UserID   int64
	LexemeID int64
}

// ListenReplayTargets returns every learner and word that has listen attempts
// and no listen card. INIT-023/SPEC-003 replays those answers into cards once.
func (s *Store) ListenReplayTargets(ctx context.Context) ([]LexemeSkill, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT DISTINCT a.user_id, a.lexeme_id
		FROM attempts AS a
		WHERE a.skill = 'listen'
		  AND NOT EXISTS (
		    SELECT 1 FROM cards AS c
		    WHERE c.user_id = a.user_id AND c.lexeme_id = a.lexeme_id AND c.skill = 'listen'
		  )
		ORDER BY a.user_id, a.lexeme_id`)
	if err != nil {
		return nil, fmt.Errorf("db: listen replay targets: %w", err)
	}
	defer rows.Close()
	var out []LexemeSkill
	for rows.Next() {
		var t LexemeSkill
		if err := rows.Scan(&t.UserID, &t.LexemeID); err != nil {
			return nil, fmt.Errorf("db: listen replay targets: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: listen replay targets: %w", err)
	}
	return out, nil
}

// WriteReplayedCard inserts a card and its review_log rows in one transaction.
// INIT-023/SPEC-003: it writes nothing when a card for that user, lexeme, and
// skill already exists, so a second run leaves an existing listen card alone.
func (s *Store) WriteReplayedCard(ctx context.Context, card Card, logs []ReviewLog) (bool, error) {
	if card.UserID == 0 || card.LexemeID == 0 || card.Skill == "" {
		return false, errors.New("db: replayed card needs a learner, lexeme, and skill")
	}
	if len(logs) == 0 {
		return false, nil
	}
	conn, err := s.connOrErr()
	if err != nil {
		return false, err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("db: begin replayed card: %w", err)
	}
	defer tx.Rollback()

	prev, err := cardTx(ctx, tx, card.UserID, card.LexemeID, card.Skill)
	if err != nil {
		return false, err
	}
	if prev != nil {
		return false, nil
	}
	id, err := insertCardTx(ctx, tx, card)
	if err != nil {
		return false, err
	}
	for _, log := range logs {
		log.CardID = id
		if err := insertReviewTx(ctx, tx, log); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("db: commit replayed card: %w", err)
	}
	return true, nil
}

// ListAttempts returns the learner's attempts for one lexeme, oldest first.
func (s *Store) ListAttempts(ctx context.Context, userID, lexemeID int64) ([]Attempt, error) {
	return s.listAttempts(ctx, userID, lexemeID, "")
}

// ListAttemptsBySkill returns the learner's attempts for one lexeme and skill, oldest first.
// INIT-023/SPEC-003 reads listen attempts this way when replaying a word into a card.
func (s *Store) ListAttemptsBySkill(ctx context.Context, userID, lexemeID int64, skill string) ([]Attempt, error) {
	if skill == "" {
		return nil, errors.New("db: list attempts by skill needs a skill")
	}
	return s.listAttempts(ctx, userID, lexemeID, skill)
}

func (s *Store) listAttempts(ctx context.Context, userID, lexemeID int64, skill string) ([]Attempt, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	if skill == "" {
		rows, err = conn.QueryContext(ctx, `
			SELECT id, user_id, lexeme_id, skill, activity, source, unit_id, prompt, response,
				correct, dont_know, hint_used, latency_ms, created_at
			FROM attempts
			WHERE user_id = ? AND lexeme_id = ?
			ORDER BY id`, userID, lexemeID)
	} else {
		rows, err = conn.QueryContext(ctx, `
			SELECT id, user_id, lexeme_id, skill, activity, source, unit_id, prompt, response,
				correct, dont_know, hint_used, latency_ms, created_at
			FROM attempts
			WHERE user_id = ? AND lexeme_id = ? AND skill = ?
			ORDER BY id`, userID, lexemeID, skill)
	}
	if err != nil {
		return nil, fmt.Errorf("db: list attempts: %w", err)
	}
	defer rows.Close()
	var out []Attempt
	for rows.Next() {
		var a Attempt
		var unit sql.NullInt64
		var correct, dontKnow, hint int
		var created string
		if err := rows.Scan(&a.ID, &a.UserID, &a.LexemeID, &a.Skill, &a.Activity, &a.Source, &unit,
			&a.Prompt, &a.Response, &correct, &dontKnow, &hint, &a.LatencyMS, &created); err != nil {
			return nil, fmt.Errorf("db: list attempts: %w", err)
		}
		a.UnitID = unit.Int64
		a.Correct, a.DontKnow, a.HintUsed = correct != 0, dontKnow != 0, hint != 0
		a.CreatedAt, err = ParseTime(created)
		if err != nil {
			return nil, fmt.Errorf("db: list attempts: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list attempts: %w", err)
	}
	return out, nil
}

// InsertExposures appends one exposures row per lexeme id, in one transaction.
// unitID 0 is stored as NULL. An empty list writes nothing.
func (s *Store) InsertExposures(ctx context.Context, userID int64, lexemeIDs []int64, context string, unitID int64, at time.Time) error {
	if len(lexemeIDs) == 0 {
		return nil
	}
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin exposures: %w", err)
	}
	defer tx.Rollback()
	var unit any
	if unitID != 0 {
		unit = unitID
	}
	for _, id := range lexemeIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO exposures (user_id, lexeme_id, context, unit_id, created_at)
			VALUES (?, ?, ?, ?, ?)`,
			userID, id, context, unit, FormatTime(at),
		); err != nil {
			return fmt.Errorf("db: insert exposure: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit exposures: %w", err)
	}
	return nil
}

func evidenceBool(b bool) int {
	if b {
		return 1
	}
	return 0
}

// cardTx reads the learner's card for a lexeme and skill. A missing card is nil.
func cardTx(ctx context.Context, tx *sql.Tx, userID, lexemeID int64, skill string) (*Card, error) {
	card, err := scanCard(tx.QueryRowContext(ctx, `
		SELECT `+cardColumns+` FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = ?`,
		userID, lexemeID, skill).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: read card: %w", err)
	}
	return &card, nil
}

func insertCardTx(ctx context.Context, tx *sql.Tx, card Card) (int64, error) {
	if card.State == "" {
		card.State = "new"
	}
	var last any
	if !card.LastReview.IsZero() {
		last = FormatTime(card.LastReview)
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO cards (
			user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		card.UserID, card.LexemeID, card.Skill, card.State, FormatTime(card.Due),
		card.Stability, card.Difficulty, card.Reps, card.Lapses, last, evidenceBool(card.Suspended),
	)
	if err != nil {
		return 0, fmt.Errorf("db: insert card: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("db: insert card: %w", err)
	}
	return id, nil
}

func updateCardTx(ctx context.Context, tx *sql.Tx, card Card) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE cards
		SET state = ?, due = ?, stability = ?, difficulty = ?, reps = ?, lapses = ?, last_review = ?
		WHERE id = ?`,
		card.State, FormatTime(card.Due), card.Stability, card.Difficulty,
		card.Reps, card.Lapses, FormatTime(card.LastReview), card.ID,
	); err != nil {
		return fmt.Errorf("db: update card: %w", err)
	}
	return nil
}

func insertReviewTx(ctx context.Context, tx *sql.Tx, log ReviewLog) error {
	var chosen any
	if log.Chosen != "" {
		chosen = log.Chosen
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, chosen)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		log.CardID, log.Rating, FormatTime(log.ReviewedAt), log.ElapsedDays, log.ScheduledDays, evidenceBool(log.Peeked), chosen,
	); err != nil {
		return fmt.Errorf("db: insert review: %w", err)
	}
	return nil
}
