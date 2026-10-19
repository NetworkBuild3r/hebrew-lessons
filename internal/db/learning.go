package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const cardColumns = `id, user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended`

// introNotFinished keeps a card out while its lexeme is still a new word
// of a lesson this learner has not finished. Recycled words, and words
// with no intro row, stay in the queue. ListDueCards and ListStudyCards
// share it.
const introNotFinished = `
		  AND NOT EXISTS (
		    SELECT 1
		    FROM lesson_items AS li
		    WHERE li.lexeme_id = c.lexeme_id
		      AND li.role = 'new'
		      AND NOT EXISTS (
		        SELECT 1
		        FROM lesson_runs AS lr
		        WHERE lr.user_id = c.user_id
		          AND lr.unit_id = li.unit_id
		          AND lr.lesson_ix = li.lesson_ix
		          AND lr.finished_at IS NOT NULL
		      )
		  )`

// Card is one row of cards.
type Card struct {
	ID         int64
	UserID     int64
	LexemeID   int64
	Skill      string
	State      string
	Due        time.Time
	Stability  float64
	Difficulty float64
	Reps       int
	Lapses     int
	LastReview time.Time // Zero when the card has never been reviewed.
	Suspended  bool
}

// ReviewLog is one row of review_log.
type ReviewLog struct {
	ID            int64
	CardID        int64
	Rating        int
	ReviewedAt    time.Time
	ElapsedDays   int
	ScheduledDays int
	Peeked        bool
	Chosen        string // Pointed NFC of the wrong option. Empty when the learner did not pick one.
}

// StudyCard is a learner's read or recall card plus the fields the
// post-placement mix reads. LatestRating is the newest review_log rating at
// or before the now passed to ListStudyCards. HasRating is false when the
// card has no such review. RecentLapse is true when an Again rating was
// stored at or after the lapseSince passed to ListStudyCards.
// HasIntro is true when a finished lesson introduces this lexeme as new.
// IntroPosition is that unit's position and IntroLesson is lesson_ix,
// from the earliest such lesson.
type StudyCard struct {
	Card
	LatestRating  int
	HasRating     bool
	RecentLapse   bool
	HasIntro      bool
	IntroPosition int
	IntroLesson   int
}

func (s *Store) connOrErr() (*sql.DB, error) {
	if s == nil || s.conn == nil {
		return nil, errors.New("db: store is closed")
	}
	return s.conn, nil
}

func scanCard(scan func(dest ...any) error) (Card, error) {
	var card Card
	var due, last sql.NullString
	var suspended int
	err := scan(
		&card.ID, &card.UserID, &card.LexemeID, &card.Skill, &card.State,
		&due, &card.Stability, &card.Difficulty, &card.Reps, &card.Lapses,
		&last, &suspended,
	)
	if err != nil {
		return Card{}, err
	}
	if due.Valid {
		card.Due, err = ParseTime(due.String)
		if err != nil {
			return Card{}, fmt.Errorf("db: card %d due: %w", card.ID, err)
		}
	}
	if last.Valid {
		card.LastReview, err = ParseTime(last.String)
		if err != nil {
			return Card{}, fmt.Errorf("db: card %d last_review: %w", card.ID, err)
		}
	}
	card.Suspended = suspended != 0
	return card, nil
}

// LexemeRef is the learner-facing face of one dictionary entry.
type LexemeRef struct {
	TextNFC  string
	Gloss    string
	Translit string
	Gender   string
	AudioID  string
}

// LexemeByID reads the text, gloss, transliteration, gender, and audio id.
// ok is false when the id is not in lexemes.
func (s *Store) LexemeByID(ctx context.Context, id int64) (LexemeRef, bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return LexemeRef{}, false, err
	}
	var face LexemeRef
	var translit, gender, audio sql.NullString
	err = conn.QueryRowContext(ctx, `
		SELECT text_nfc, gloss_en, translit, gender, audio_id
		FROM lexemes WHERE id = ?`, id).Scan(&face.TextNFC, &face.Gloss, &translit, &gender, &audio)
	if errors.Is(err, sql.ErrNoRows) {
		return LexemeRef{}, false, nil
	}
	if err != nil {
		return LexemeRef{}, false, fmt.Errorf("db: lexeme %d: %w", id, err)
	}
	face.Translit = translit.String
	face.Gender = gender.String
	face.AudioID = audio.String
	return face, true, nil
}

// GetCard reads one card by id.
func (s *Store) GetCard(ctx context.Context, id int64) (Card, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return Card{}, err
	}
	card, err := scanCard(conn.QueryRowContext(ctx, `SELECT `+cardColumns+` FROM cards WHERE id = ?`, id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return Card{}, fmt.Errorf("db: card %d: not found", id)
	}
	if err != nil {
		return Card{}, fmt.Errorf("db: get card: %w", err)
	}
	return card, nil
}

// InsertCard inserts a card and returns its id.
// An empty State is stored as new. A zero LastReview is stored as NULL.
func (s *Store) InsertCard(ctx context.Context, card Card) (int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	if card.State == "" {
		card.State = "new"
	}
	var last any
	if !card.LastReview.IsZero() {
		last = FormatTime(card.LastReview)
	}
	suspended := 0
	if card.Suspended {
		suspended = 1
	}
	res, err := conn.ExecContext(ctx, `
		INSERT INTO cards (
			user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		card.UserID, card.LexemeID, card.Skill, card.State, FormatTime(card.Due),
		card.Stability, card.Difficulty, card.Reps, card.Lapses, last, suspended,
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

// ApplyReview updates the card's schedule and appends one review_log row.
// Both writes share a transaction. scheduled_days is the interval this
// rating scheduled, which is what the column records.
func (s *Store) ApplyReview(ctx context.Context, card Card, log ReviewLog) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if card.ID == 0 {
		return errors.New("db: card id is empty")
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin review: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		UPDATE cards
		SET state = ?, due = ?, stability = ?, difficulty = ?, reps = ?, lapses = ?, last_review = ?
		WHERE id = ?`,
		card.State, FormatTime(card.Due), card.Stability, card.Difficulty,
		card.Reps, card.Lapses, FormatTime(card.LastReview), card.ID,
	)
	if err != nil {
		return fmt.Errorf("db: update card: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: update card: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("db: card %d: not found", card.ID)
	}
	peeked := 0
	if log.Peeked {
		peeked = 1
	}
	var chosen any
	if log.Chosen != "" {
		chosen = log.Chosen
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, chosen)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		card.ID, log.Rating, FormatTime(log.ReviewedAt), log.ElapsedDays, log.ScheduledDays, peeked, chosen,
	); err != nil {
		return fmt.Errorf("db: insert review: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit review: %w", err)
	}
	return nil
}

// SetCardSuspended sets cards.suspended. A suspended card stays in the table
// and stays out of the due query.
func (s *Store) SetCardSuspended(ctx context.Context, id int64, suspended bool) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	flag := 0
	if suspended {
		flag = 1
	}
	res, err := conn.ExecContext(ctx, `UPDATE cards SET suspended = ? WHERE id = ?`, flag, id)
	if err != nil {
		return fmt.Errorf("db: suspend card: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db: suspend card: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("db: card %d: not found", id)
	}
	return nil
}

// ListDueCards returns cards that are due, not suspended, and not still
// waiting on an intro lesson.
//
// A lexeme listed as lesson_items.role = 'new' is excluded until this learner
// has a lesson_runs row for that unit and lesson with finished_at set.
// Recycled words, and words with no intro row, are not excluded.
func (s *Store) ListDueCards(ctx context.Context, userID int64, now time.Time) ([]Card, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT `+cardColumns+`
		FROM cards AS c
		WHERE c.user_id = ?
		  AND c.suspended = 0
		  AND c.due <= ?
`+introNotFinished+`
		ORDER BY c.due, c.id`, userID, FormatTime(now))
	if err != nil {
		return nil, fmt.Errorf("db: list due cards: %w", err)
	}
	defer rows.Close()
	var out []Card
	for rows.Next() {
		card, err := scanCard(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("db: list due cards: %w", err)
		}
		out = append(out, card)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: list due cards: %w", err)
	}
	return out, nil
}

// ReviewLogs returns the card's log, oldest first.
func (s *Store) ReviewLogs(ctx context.Context, cardID int64) ([]ReviewLog, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, chosen
		FROM review_log
		WHERE card_id = ?
		ORDER BY id`, cardID)
	if err != nil {
		return nil, fmt.Errorf("db: review log: %w", err)
	}
	defer rows.Close()
	var out []ReviewLog
	for rows.Next() {
		var log ReviewLog
		var reviewed string
		var peeked int
		var chosen sql.NullString
		if err := rows.Scan(&log.ID, &log.CardID, &log.Rating, &reviewed, &log.ElapsedDays, &log.ScheduledDays, &peeked, &chosen); err != nil {
			return nil, fmt.Errorf("db: review log: %w", err)
		}
		log.ReviewedAt, err = ParseTime(reviewed)
		if err != nil {
			return nil, fmt.Errorf("db: review log: %w", err)
		}
		log.Peeked = peeked != 0
		if chosen.Valid {
			log.Chosen = chosen.String
		}
		out = append(out, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: review log: %w", err)
	}
	return out, nil
}

// InsertConfusionMiss appends one miss. The 7-day total is not stored.
// lexemeID 0 is stored as NULL: the miss is not tied to a word.
func (s *Store) InsertConfusionMiss(ctx context.Context, learnerID int64, kind, key string, lexemeID int64, at time.Time) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	var lexeme any
	if lexemeID != 0 {
		lexeme = lexemeID
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at)
		VALUES (?, ?, ?, ?, ?)`,
		learnerID, kind, key, lexeme, FormatTime(at),
	); err != nil {
		return fmt.Errorf("db: insert confusion: %w", err)
	}
	return nil
}

// InsertUser inserts a user and returns the id.
// created_at comes from the store clock.
func (s *Store) InsertUser(ctx context.Context, email, sub, role string) (int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	if s.clock == nil {
		return 0, errors.New("db: clock is not configured")
	}
	res, err := conn.ExecContext(ctx, `
		INSERT INTO users (email, google_sub, role, created_at)
		VALUES (?, ?, ?, ?)`,
		email, sub, role, FormatTime(s.clock.Now()),
	)
	if err != nil {
		return 0, fmt.Errorf("db: insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("db: insert user: %w", err)
	}
	return id, nil
}

// ListStudyCards returns the learner's unsuspended read and recall cards,
// including cards that are not yet due. Words still waiting on their intro
// lesson are left out, the same way ListDueCards leaves them out.
// now caps the latest-rating lookup: a review_log row after now is ignored.
// lapseSince is the start of the recent-lapse window. An Again at that
// time or later sets RecentLapse. The clock that produced now and lapseSince
// is the caller's; this query does not read the wall clock.
func (s *Store) ListStudyCards(ctx context.Context, userID int64, now, lapseSince time.Time) ([]StudyCard, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT `+cardColumns+`,
			(
				SELECT rl.rating
				FROM review_log AS rl
				WHERE rl.card_id = c.id
				  AND rl.reviewed_at <= ?
				ORDER BY rl.reviewed_at DESC, rl.id DESC
				LIMIT 1
			),
			CASE WHEN EXISTS (
				SELECT 1
				FROM review_log AS rl
				WHERE rl.card_id = c.id
				  AND rl.rating = 1
				  AND rl.reviewed_at >= ?
			) THEN 1 ELSE 0 END,
			(
				SELECT u.position
				FROM lesson_items AS li
				JOIN units AS u ON u.id = li.unit_id
				WHERE li.lexeme_id = c.lexeme_id
				  AND li.role = 'new'
				  AND EXISTS (
				    SELECT 1
				    FROM lesson_runs AS lr
				    WHERE lr.user_id = c.user_id
				      AND lr.unit_id = li.unit_id
				      AND lr.lesson_ix = li.lesson_ix
				      AND lr.finished_at IS NOT NULL
				  )
				ORDER BY u.position, li.lesson_ix, u.id
				LIMIT 1
			),
			(
				SELECT li.lesson_ix
				FROM lesson_items AS li
				JOIN units AS u ON u.id = li.unit_id
				WHERE li.lexeme_id = c.lexeme_id
				  AND li.role = 'new'
				  AND EXISTS (
				    SELECT 1
				    FROM lesson_runs AS lr
				    WHERE lr.user_id = c.user_id
				      AND lr.unit_id = li.unit_id
				      AND lr.lesson_ix = li.lesson_ix
				      AND lr.finished_at IS NOT NULL
				  )
				ORDER BY u.position, li.lesson_ix, u.id
				LIMIT 1
			)
		FROM cards AS c
		WHERE c.user_id = ?
		  AND c.suspended = 0
		  AND c.skill IN ('read', 'recall')
`+introNotFinished+`
		ORDER BY c.due, c.id`, FormatTime(now), FormatTime(lapseSince), userID)
	if err != nil {
		return nil, fmt.Errorf("db: study cards: %w", err)
	}
	defer rows.Close()
	var out []StudyCard
	for rows.Next() {
		card, err := scanStudyCard(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("db: study cards: %w", err)
		}
		out = append(out, card)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: study cards: %w", err)
	}
	return out, nil
}

func scanStudyCard(scan func(dest ...any) error) (StudyCard, error) {
	var latest sql.NullInt64
	var recent int
	var position, lesson sql.NullInt64
	card, err := scanCard(func(dest ...any) error {
		args := make([]any, 0, len(dest)+4)
		args = append(args, dest...)
		args = append(args, &latest, &recent, &position, &lesson)
		return scan(args...)
	})
	if err != nil {
		return StudyCard{}, err
	}
	out := StudyCard{Card: card, RecentLapse: recent != 0}
	if latest.Valid {
		out.HasRating = true
		out.LatestRating = int(latest.Int64)
	}
	if position.Valid && lesson.Valid {
		out.HasIntro = true
		out.IntroPosition = int(position.Int64)
		out.IntroLesson = int(lesson.Int64)
	}
	return out, nil
}

// RecentReviewRatings returns up to limit ratings for the learner, newest
// first. reviewed_at is the clock time stored with the review. Rows later
// than now are left out, and ties break on id descending, so the last N
// reviews are fixed once the clock and the log are fixed.
func (s *Store) RecentReviewRatings(ctx context.Context, userID int64, now time.Time, limit int) ([]int, error) {
	if limit <= 0 {
		return nil, nil
	}
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT rl.rating
		FROM review_log AS rl
		JOIN cards AS c ON c.id = rl.card_id
		WHERE c.user_id = ? AND rl.reviewed_at <= ?
		ORDER BY rl.reviewed_at DESC, rl.id DESC
		LIMIT ?`, userID, FormatTime(now), limit)
	if err != nil {
		return nil, fmt.Errorf("db: recent ratings: %w", err)
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var rating int
		if err := rows.Scan(&rating); err != nil {
			return nil, fmt.Errorf("db: recent ratings: %w", err)
		}
		out = append(out, rating)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: recent ratings: %w", err)
	}
	return out, nil
}
