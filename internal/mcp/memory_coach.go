package mcp

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"time"
)

type storedCard struct {
	id   int64
	user int64
	card LearnerCard
}

type storedReview struct {
	id     int64
	user   int64
	rating int
	at     time.Time
}

type storedLesson struct {
	user     int64
	finished *time.Time
	score    *float64
}

type memCursor struct {
	Due     int64 `json:"d"`
	Last    int64 `json:"r"`
	HasLast bool  `json:"h"`
	ID      int64 `json:"i"`
}

// AddCard stores one card for userID and returns its id.
// Due and LastReview are stored in UTC. A nil translit, last rating,
// or last review stays null.
func (m *Memory) AddCard(userID int64, card LearnerCard) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextCard++
	card.Due = card.Due.UTC()
	if card.LastReview != nil {
		at := card.LastReview.UTC()
		card.LastReview = &at
	}
	m.cards = append(m.cards, storedCard{id: m.nextCard, user: userID, card: card})
	return m.nextCard
}

// AddReview records one rating for recentAccuracy. It does not change a card.
func (m *Memory) AddReview(userID int64, rating int, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextCard++
	m.reviews = append(m.reviews, storedReview{id: m.nextCard, user: userID, rating: rating, at: at.UTC()})
}

// AddLessonRun records one lesson run. A nil finished time is still in progress
// and is not counted. A nil score counts as finished once finished is set.
func (m *Memory) AddLessonRun(userID int64, finished *time.Time, score *float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var at *time.Time
	if finished != nil {
		u := finished.UTC()
		at = &u
	}
	m.lessons = append(m.lessons, storedLesson{user: userID, finished: at, score: score})
}

// LearnerCards implements CourseReader.
func (m *Memory) LearnerCards(ctx context.Context, q CardQuery) (CardPage, error) {
	if err := ctx.Err(); err != nil {
		return CardPage{}, err
	}
	if q.Limit <= 0 || q.Limit > learnerWordsMaxLimit {
		return CardPage{}, errBadCardLimit
	}
	if !validCardState(q.State) {
		return CardPage{}, errBadCardState
	}
	var cur memCursor
	var haveCursor bool
	if q.Cursor != "" {
		decoded, err := decodeMemCursor(q.Cursor)
		if err != nil {
			return CardPage{}, ErrBadCardCursor
		}
		cur = decoded
		haveCursor = true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rows := make([]storedCard, 0, len(m.cards))
	for _, row := range m.cards {
		if row.user != q.UserID {
			continue
		}
		if q.State != "" && row.card.State != q.State {
			continue
		}
		if q.DueBefore != nil && !row.card.Due.Before(q.DueBefore.UTC()) {
			continue
		}
		if haveCursor && !memAfter(row, cur) {
			continue
		}
		rows = append(rows, row)
	}
	slices.SortFunc(rows, func(a, b storedCard) int {
		if c := a.card.Due.Compare(b.card.Due); c != 0 {
			return c
		}
		if c := compareLastDesc(a.card.LastReview, b.card.LastReview); c != 0 {
			return c
		}
		return cmp.Compare(a.id, b.id)
	})
	page := CardPage{Cards: []LearnerCard{}}
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		cursor, err := encodeMemCursor(rows[len(rows)-1])
		if err != nil {
			return CardPage{}, err
		}
		page.NextCursor = cursor
	}
	for _, row := range rows {
		page.Cards = append(page.Cards, row.card)
	}
	return page, nil
}

// CardsDueNow implements CourseReader.
// Memory stores no suspended flag and no intro-lesson rows, so every
// stored card whose due is at or before the wall clock is counted.
// The database reader calls ListDueCards instead.
func (m *Memory) CardsDueNow(ctx context.Context, userID int64) (CardsDueNow, error) {
	if err := ctx.Err(); err != nil {
		return CardsDueNow{}, err
	}
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()
	states := make([]string, 0)
	for _, row := range m.cards {
		if row.user != userID || row.card.Due.After(now) {
			continue
		}
		states = append(states, row.card.State)
	}
	return CardsDueNowFromStates(states), nil
}

// RecentAccuracy implements CourseReader.
// The window ends at the wall clock. Reviews and finished lessons after
// that instant are ignored.
func (m *Memory) RecentAccuracy(ctx context.Context, userID int64) (RecentAccuracy, error) {
	if err := ctx.Err(); err != nil {
		return RecentAccuracy{}, err
	}
	now := time.Now().UTC()
	start := now.Add(-time.Duration(AccuracyWindowDays) * 24 * time.Hour)
	m.mu.Lock()
	defer m.mu.Unlock()

	var inWindow, windowGood int
	var latest []storedReview
	for _, row := range m.reviews {
		if row.user != userID || row.at.After(now) {
			continue
		}
		latest = append(latest, row)
		if !row.at.Before(start) {
			inWindow++
			if goodOrEasy(row.rating) {
				windowGood++
			}
		}
	}
	slices.SortFunc(latest, func(a, b storedReview) int {
		if c := b.at.Compare(a.at); c != 0 {
			return c
		}
		return cmp.Compare(b.id, a.id)
	})
	if len(latest) > AccuracyReviewCap {
		latest = latest[:AccuracyReviewCap]
	}
	lastGood := 0
	for _, row := range latest {
		if goodOrEasy(row.rating) {
			lastGood++
		}
	}

	var finished, scored int
	var sum float64
	for _, row := range m.lessons {
		if row.user != userID || row.finished == nil || row.finished.After(now) || row.finished.Before(start) {
			continue
		}
		finished++
		if row.score != nil {
			scored++
			sum += *row.score
		}
	}
	return recentAccuracy(
		countAccuracy(inWindow, windowGood),
		countAccuracy(len(latest), lastGood),
		lessonAccuracy(finished, scored, sum),
	), nil
}

func compareLastDesc(a, b *time.Time) int {
	aMiss := a == nil || a.IsZero()
	bMiss := b == nil || b.IsZero()
	if aMiss && bMiss {
		return 0
	}
	if aMiss {
		return 1
	}
	if bMiss {
		return -1
	}
	return b.Compare(*a)
}

func memAfter(row storedCard, cur memCursor) bool {
	curDue := time.Unix(0, cur.Due).UTC()
	if row.card.Due.After(curDue) {
		return true
	}
	if !row.card.Due.Equal(curDue) {
		return false
	}
	var curLast *time.Time
	if cur.HasLast {
		at := time.Unix(0, cur.Last).UTC()
		curLast = &at
	}
	if c := compareLastDesc(row.card.LastReview, curLast); c != 0 {
		return c > 0
	}
	return row.id > cur.ID
}

func encodeMemCursor(row storedCard) (string, error) {
	cur := memCursor{Due: row.card.Due.UnixNano(), ID: row.id}
	if row.card.LastReview != nil && !row.card.LastReview.IsZero() {
		cur.HasLast = true
		cur.Last = row.card.LastReview.UnixNano()
	}
	raw, err := json.Marshal(cur)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeMemCursor(token string) (memCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return memCursor{}, err
	}
	var cur memCursor
	if err := json.Unmarshal(raw, &cur); err != nil {
		return memCursor{}, err
	}
	if cur.ID <= 0 {
		return memCursor{}, ErrBadCardCursor
	}
	return cur, nil
}

var (
	errBadCardLimit = errors.New("card page limit is out of range")
	errBadCardState = errors.New("card state is not valid")
)
