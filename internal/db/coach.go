package db

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrBadCoachCursor means a page cursor was not written by CoachCards.
var ErrBadCoachCursor = errors.New("coach cursor is not valid")

// CoachQuery selects one page of a learner's cards.
// Limit must be from 1 to 200. Cursor empty is the first page.
// State empty is every state. DueBefore, when set, is exclusive.
type CoachQuery struct {
	UserID    int64
	Cursor    string
	Limit     int
	State     string
	DueBefore *time.Time
}

// CoachCard is one card joined to its lexeme.
// DueText and LastText are the stored timestamp strings. The page cursor
// uses those strings so a seek compares the same text the ORDER BY uses.
// HasTranslit is false when translit is null or empty.
// HasRating is false when the card has no review_log row.
// LastText is empty when last_review is null.
type CoachCard struct {
	ID          int64
	Lexeme      string
	Translit    string
	HasTranslit bool
	POS         string
	Gloss       string
	Dir         string
	State       string
	Stability   float64
	Difficulty  float64
	Due         time.Time
	DueText     string
	Lapses      int
	Reps        int
	LastRating  int
	HasRating   bool
	LastReview  time.Time
	LastText    string
}

// CoachPage is one page. NextCursor is empty on the last page.
type CoachPage struct {
	Cards      []CoachCard
	NextCursor string
}

// CoachAccuracy is the review and lesson counts behind recentAccuracy.
// Reviews7d is the window ending at now. LastReviews is the newest lastN
// ratings at or before now, of any age. Lessons7d counts finished runs
// in the same window. A null score is finished and not scored.
type CoachAccuracy struct {
	Reviews7d   CoachCount
	LastReviews CoachCount
	Lessons7d   CoachLessons
}

// CoachCount is a Good-or-Easy tally. Good is rating 3 and Easy is rating 4.
type CoachCount struct {
	Count      int
	GoodOrEasy int
}

// CoachLessons is finished lesson_runs. ScoreSum is the sum of non-null scores.
type CoachLessons struct {
	Finished int
	Scored   int
	ScoreSum float64
}

type coachCursor struct {
	Due  string `json:"d"`
	Last string `json:"r"`
	ID   int64  `json:"i"`
}

// CoachCards lists every card for one learner, including suspended cards
// and cards whose intro lesson is unfinished. The study queue filters
// those out. This list does not, because the author needs every card.
// Order is due ascending, then last_review descending with nulls last,
// then id ascending.
func (s *Store) CoachCards(ctx context.Context, q CoachQuery) (CoachPage, error) {
	if q.Limit <= 0 || q.Limit > 200 {
		return CoachPage{}, fmt.Errorf("db: coach cards: limit %d", q.Limit)
	}
	switch q.State {
	case "", "new", "learning", "review", "relearning":
	default:
		return CoachPage{}, fmt.Errorf("db: coach cards: state %q", q.State)
	}
	var dueText, lastText string
	var cursorID int64
	if q.Cursor != "" {
		cur, err := decodeCoachCursor(q.Cursor)
		if err != nil {
			return CoachPage{}, ErrBadCoachCursor
		}
		dueText, lastText, cursorID = cur.Due, cur.Last, cur.ID
	}
	var before string
	if q.DueBefore != nil {
		before = FormatTime(*q.DueBefore)
	}
	conn, err := s.connOrErr()
	if err != nil {
		return CoachPage{}, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT l.text_nfc, l.translit, l.pos, l.gloss_en,
			c.id, c.dir, c.state, c.stability, c.difficulty, c.due, c.lapses, c.reps, c.last_review,
			(
				SELECT rl.rating
				FROM review_log AS rl
				WHERE rl.card_id = c.id
				ORDER BY rl.reviewed_at DESC, rl.id DESC
				LIMIT 1
			)
		FROM cards AS c
		JOIN lexemes AS l ON l.id = c.lexeme_id
		WHERE c.user_id = ?
		  AND (? = '' OR c.state = ?)
		  AND (? = '' OR c.due < ?)
		  AND (
		        ? = 0
		        OR c.due > ?
		        OR (c.due = ? AND COALESCE(c.last_review, '') < ?)
		        OR (c.due = ? AND COALESCE(c.last_review, '') = ? AND c.id > ?)
		      )
		ORDER BY c.due ASC, COALESCE(c.last_review, '') DESC, c.id ASC
		LIMIT ?`,
		q.UserID,
		q.State, q.State,
		before, before,
		cursorID,
		dueText,
		dueText, lastText,
		dueText, lastText, cursorID,
		q.Limit+1,
	)
	if err != nil {
		return CoachPage{}, fmt.Errorf("db: coach cards: %w", err)
	}
	defer rows.Close()
	var cards []CoachCard
	for rows.Next() {
		card, err := scanCoachCard(rows.Scan)
		if err != nil {
			return CoachPage{}, fmt.Errorf("db: coach cards: %w", err)
		}
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return CoachPage{}, fmt.Errorf("db: coach cards: %w", err)
	}
	page := CoachPage{}
	if len(cards) > q.Limit {
		last := cards[q.Limit-1]
		cursor, err := encodeCoachCursor(last.DueText, last.LastText, last.ID)
		if err != nil {
			return CoachPage{}, err
		}
		page.NextCursor = cursor
		cards = cards[:q.Limit]
	}
	if cards == nil {
		cards = []CoachCard{}
	}
	page.Cards = cards
	return page, nil
}

// CoachAccuracy counts reviews and finished lessons for one learner.
// now is the end of the window, inclusive. window is how far back the
// 7-day counts reach. lastN is the newest ratings at or before now.
func (s *Store) CoachAccuracy(ctx context.Context, userID int64, now time.Time, window time.Duration, lastN int) (CoachAccuracy, error) {
	if window < 0 || lastN < 0 {
		return CoachAccuracy{}, fmt.Errorf("db: coach accuracy: window %s last %d", window, lastN)
	}
	conn, err := s.connOrErr()
	if err != nil {
		return CoachAccuracy{}, err
	}
	start := FormatTime(now.Add(-window))
	end := FormatTime(now)
	var out CoachAccuracy
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(SUM(CASE WHEN rl.rating IN (3, 4) THEN 1 ELSE 0 END), 0)
		FROM review_log AS rl
		JOIN cards AS c ON c.id = rl.card_id
		WHERE c.user_id = ?
		  AND rl.reviewed_at >= ?
		  AND rl.reviewed_at <= ?`, userID, start, end).Scan(&out.Reviews7d.Count, &out.Reviews7d.GoodOrEasy); err != nil {
		return CoachAccuracy{}, fmt.Errorf("db: coach accuracy: %w", err)
	}
	ratings, err := s.RecentReviewRatings(ctx, userID, now, lastN)
	if err != nil {
		return CoachAccuracy{}, err
	}
	out.LastReviews.Count = len(ratings)
	for _, rating := range ratings {
		if rating == 3 || rating == 4 {
			out.LastReviews.GoodOrEasy++
		}
	}
	var sum sql.NullFloat64
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(SUM(CASE WHEN score IS NOT NULL THEN 1 ELSE 0 END), 0),
			SUM(CASE WHEN score IS NOT NULL THEN score ELSE 0 END)
		FROM lesson_runs
		WHERE user_id = ?
		  AND finished_at IS NOT NULL
		  AND finished_at >= ?
		  AND finished_at <= ?`, userID, start, end).Scan(&out.Lessons7d.Finished, &out.Lessons7d.Scored, &sum); err != nil {
		return CoachAccuracy{}, fmt.Errorf("db: coach accuracy: %w", err)
	}
	if sum.Valid {
		out.Lessons7d.ScoreSum = sum.Float64
	}
	return out, nil
}

func scanCoachCard(scan func(dest ...any) error) (CoachCard, error) {
	var card CoachCard
	var translit, due, last sql.NullString
	var rating sql.NullInt64
	err := scan(
		&card.Lexeme, &translit, &card.POS, &card.Gloss,
		&card.ID, &card.Dir, &card.State, &card.Stability, &card.Difficulty,
		&due, &card.Lapses, &card.Reps, &last, &rating,
	)
	if err != nil {
		return CoachCard{}, err
	}
	if translit.Valid && translit.String != "" {
		card.HasTranslit = true
		card.Translit = translit.String
	}
	if !due.Valid || due.String == "" {
		return CoachCard{}, fmt.Errorf("card %d due is null", card.ID)
	}
	card.DueText = due.String
	card.Due, err = ParseTime(due.String)
	if err != nil {
		return CoachCard{}, fmt.Errorf("card %d due: %w", card.ID, err)
	}
	if last.Valid && last.String != "" {
		card.LastText = last.String
		card.LastReview, err = ParseTime(last.String)
		if err != nil {
			return CoachCard{}, fmt.Errorf("card %d last_review: %w", card.ID, err)
		}
	}
	if rating.Valid {
		card.HasRating = true
		card.LastRating = int(rating.Int64)
	}
	return card, nil
}

func encodeCoachCursor(due, last string, id int64) (string, error) {
	raw, err := json.Marshal(coachCursor{Due: due, Last: last, ID: id})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeCoachCursor(token string) (coachCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return coachCursor{}, err
	}
	var cur coachCursor
	if err := json.Unmarshal(raw, &cur); err != nil {
		return coachCursor{}, err
	}
	if cur.Due == "" || cur.ID <= 0 {
		return coachCursor{}, ErrBadCoachCursor
	}
	return cur, nil
}
