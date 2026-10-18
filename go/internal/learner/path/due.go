package path

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
	"unicode/utf16"
)

// reviewDueCap and reviewNewCap are HebrewReview.DEFAULT_CAPS in
// shared/player/review.js. server/learner/cards.js selectDue applies them
// to the due deck. The Review page shows that deck's length
// (GET /api/me/cards?deck=due, shared/review-page.js setCount).
//
// These are not the ADR-0017 review_stop_due_count. That threshold decides
// whether a path node is inserted. This cap decides how many cards the
// Review page lists.
const (
	reviewDueCap = 30
	reviewNewCap = 10
)

// DueCount is the number of cards on the Review page due deck.
//
// A row counts when server/learner/cards.js isDue and HebrewReview.isDue
// would accept it: direction is he_en or en_he, suspended is not 1, due
// parses, and due <= now. Comparison is the instant, not the local
// calendar day. A row due later today does not count until that instant.
// A row due exactly at now does count. Two due directions are two cards.
// Suspension is per row: a suspended direction does not hide the other.
//
// The due deck then applies the same selection as cards.js selectDue:
// oldest due first, at most reviewDueCap cards, at most reviewNewCap new
// words, and the same lesson interleaving. A word is new when every
// non-suspended row of that word has null last_at and null last_result.
// The lesson is the first sources entry other than "self", "ai", and
// "lesson". The daily-mix library and seen-lesson filters in
// server/learner/mix.js reviewItems are not applied. The Review page does
// not use them.
//
// The service clock supplies now. WithLocation is not read: isDue does
// not bucket by local date. userID must be positive. An unknown user has
// no rows and returns 0. The query is read-only.
func (s *Service) DueCount(ctx context.Context, userID int64) (int, error) {
	items, err := s.dueQueue(ctx, userID, s.instant())
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

type dueItem struct {
	word      string
	direction string
	lesson    string
	dueMs     int64
	isNew     bool
}

func (s *Service) countDue(ctx context.Context, userID int64, now time.Time) (int, error) {
	items, err := s.dueQueue(ctx, userID, now)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

func (s *Service) dueQueue(ctx context.Context, userID int64, now time.Time) ([]dueItem, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, ErrBadUser
	}
	if s == nil || s.db == nil {
		return nil, errors.New("learner: nil database")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT word, direction, due, suspended, last_result, last_at, sources
		 FROM review_items WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("learner: due count: %w", err)
	}
	defer rows.Close()

	type stored struct {
		word, direction, due, sources string
		lastResult, lastAt            sql.NullString
	}
	groups := map[string][]stored{}
	for rows.Next() {
		var word, direction, due, sources string
		var suspended int
		var lastResult, lastAt sql.NullString
		if err := rows.Scan(&word, &direction, &due, &suspended, &lastResult, &lastAt, &sources); err != nil {
			return nil, fmt.Errorf("learner: due count: %w", err)
		}
		if suspended == 1 {
			continue
		}
		groups[word] = append(groups[word], stored{
			word: word, direction: direction, due: due, sources: sources,
			lastResult: lastResult, lastAt: lastAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("learner: due count: %w", err)
	}

	nowMs := now.UnixMilli()
	var items []dueItem
	for _, group := range groups {
		fresh := true
		for _, row := range group {
			if row.lastAt.Valid || row.lastResult.Valid {
				fresh = false
				break
			}
		}
		for _, row := range group {
			if row.direction != "he_en" && row.direction != "en_he" {
				continue
			}
			dueMs, ok := parseInstant(row.due)
			if !ok || dueMs > nowMs {
				continue
			}
			items = append(items, dueItem{
				word:      row.word,
				direction: row.direction,
				lesson:    lessonOf(row.sources),
				dueMs:     dueMs,
				isNew:     fresh,
			})
		}
	}
	return selectDue(items), nil
}

func selectDue(items []dueItem) []dueItem {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].dueMs != items[j].dueMs {
			return items[i].dueMs < items[j].dueMs
		}
		if items[i].word != items[j].word {
			return jsLess(items[i].word, items[j].word)
		}
		return jsLess(items[i].direction, items[j].direction)
	})
	pending := append([]dueItem(nil), items...)
	selected := make([]dueItem, 0, len(pending))
	newWords := map[string]struct{}{}
	for len(pending) > 0 && len(selected) < reviewDueCap {
		var lastLesson string
		sameLesson := false
		if len(selected) >= 2 {
			a := selected[len(selected)-2].lesson
			b := selected[len(selected)-1].lesson
			sameLesson = a != "" && a == b
			lastLesson = a
		}
		otherWaiting := false
		if sameLesson {
			for _, item := range pending {
				if item.lesson != "" && item.lesson != lastLesson {
					otherWaiting = true
					break
				}
			}
		}
		pick := -1
		for i, candidate := range pending {
			if candidate.isNew {
				if _, seen := newWords[candidate.word]; !seen && len(newWords) >= reviewNewCap {
					continue
				}
			}
			if otherWaiting && candidate.lesson == lastLesson {
				continue
			}
			pick = i
			break
		}
		if pick < 0 {
			break
		}
		chosen := pending[pick]
		pending = append(pending[:pick], pending[pick+1:]...)
		if chosen.isNew {
			newWords[chosen.word] = struct{}{}
		}
		selected = append(selected, chosen)
	}
	return selected
}

func lessonOf(raw string) string {
	if raw == "" {
		return ""
	}
	var sources []string
	if err := json.Unmarshal([]byte(raw), &sources); err != nil {
		return ""
	}
	for _, source := range sources {
		if source == "" || source == "self" || source == "ai" || source == "lesson" {
			continue
		}
		return source
	}
	return ""
}

// jsLess is JavaScript's UTF-16 code-unit order, which is the order
// cards.js uses when two due instants tie.
func jsLess(a, b string) bool {
	aa := utf16.Encode([]rune(a))
	bb := utf16.Encode([]rune(b))
	n := len(aa)
	if len(bb) < n {
		n = len(bb)
	}
	for i := 0; i < n; i++ {
		if aa[i] != bb[i] {
			return aa[i] < bb[i]
		}
	}
	return len(aa) < len(bb)
}

func (s *Service) instant() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}

func parseInstant(raw string) (int64, bool) {
	if raw == "" {
		return 0, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed.UnixMilli(), true
		}
	}
	return 0, false
}
