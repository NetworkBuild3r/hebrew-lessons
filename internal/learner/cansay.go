package learner

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// CanSayWeek is how far back a word still counts as newly sayable.
const CanSayWeek = 7 * 24 * time.Hour

// CanSayNext is the most words "almost there" names.
const CanSayNext = 5

// CanSayItem is one thing the learner can say, or is close to saying.
type CanSayItem struct {
	LexemeID int64
	Text     string
	Translit string
	Gloss    string
}

// CanSay is what the learner can produce from memory. Recall is the measured skill closest
// to saying a word, so the list is the recall state. It reads one learner only.
type CanSay struct {
	// Items are the words and phrases whose recall state is known or solid. Phrases come
	// first, then the most recently learned.
	Items []CanSayItem
	Total int
	// AddedThisWeek counts Items that moved into known or solid within the last 7 days and
	// are still there. A word that has since slipped is not in Items, so it is not counted.
	AddedThisWeek int
	// Next are up to 5 words in learning for recall, the most accurate first.
	Next []CanSayItem
}

// CanSay reports what the learner can say. The clock decides the week boundary.
func (m *Model) CanSay(ctx context.Context, userID int64) (CanSay, error) {
	var out CanSay
	if err := m.ready(userID); err != nil {
		return out, err
	}
	known, err := m.listState(ctx, userID, Filter{State: Known, Skill: fsrs.SkillRecall})
	if err != nil {
		return out, err
	}
	solid, err := m.listState(ctx, userID, Filter{State: Solid, Skill: fsrs.SkillRecall})
	if err != nil {
		return out, err
	}
	words := append(known, solid...)

	learned, err := m.store.RecallLearnedAt(ctx, userID)
	if err != nil {
		return out, err
	}
	ids := make([]int64, len(words))
	for i, w := range words {
		ids[i] = w.LexemeID
	}
	texts, err := m.store.LexemeTexts(ctx, ids)
	if err != nil {
		return out, err
	}

	since := m.clock.Now().Add(-CanSayWeek)
	slices.SortFunc(words, func(a, b WordState) int {
		ap, bp := strings.Contains(a.Text, " "), strings.Contains(b.Text, " ")
		if ap != bp {
			if ap {
				return -1
			}
			return 1
		}
		at, bt := learned[a.LexemeID], learned[b.LexemeID]
		if !at.Equal(bt) {
			if at.After(bt) {
				return -1
			}
			return 1
		}
		return cmpInt64(a.LexemeID, b.LexemeID)
	})
	out.Items = make([]CanSayItem, 0, len(words))
	for _, w := range words {
		out.Items = append(out.Items, sayItem(w, texts[w.LexemeID].Translit))
		if when, ok := learned[w.LexemeID]; ok && !when.Before(since) {
			out.AddedThisWeek++
		}
	}
	out.Total = len(out.Items)

	learning, err := m.listState(ctx, userID, Filter{State: Learning, Skill: fsrs.SkillRecall})
	if err != nil {
		return out, err
	}
	slices.SortFunc(learning, func(a, b WordState) int {
		if a.Accuracy != b.Accuracy {
			if a.Accuracy > b.Accuracy {
				return -1
			}
			return 1
		}
		return cmpInt64(a.LexemeID, b.LexemeID)
	})
	if len(learning) > CanSayNext {
		learning = learning[:CanSayNext]
	}
	nextIDs := make([]int64, len(learning))
	for i, w := range learning {
		nextIDs[i] = w.LexemeID
	}
	nextTexts, err := m.store.LexemeTexts(ctx, nextIDs)
	if err != nil {
		return out, err
	}
	out.Next = make([]CanSayItem, 0, len(learning))
	for _, w := range learning {
		out.Next = append(out.Next, sayItem(w, nextTexts[w.LexemeID].Translit))
	}
	return out, nil
}

// listState reads every page of one filter.
func (m *Model) listState(ctx context.Context, userID int64, f Filter) ([]WordState, error) {
	f.Limit = MaxPageSize
	var out []WordState
	for {
		page, next, err := m.List(ctx, userID, f)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if next == "" {
			return out, nil
		}
		f.After = next
	}
}

func sayItem(w WordState, translit string) CanSayItem {
	return CanSayItem{LexemeID: w.LexemeID, Text: w.Text, Translit: translit, Gloss: w.Gloss}
}

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
