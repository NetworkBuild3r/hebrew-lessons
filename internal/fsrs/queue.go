package fsrs

import "context"

// Queue builds the learner's session.
//
// The session holds at most MaxQueue items. Due cards, letter drills, and
// pattern drills are interleaved, and two items of the same lexeme are not
// placed next to each other. A new word stays out until the lesson that
// introduces it has a finished lesson_runs row; that lesson practises the
// word as a block, which is not part of this queue.
//
// A confusion key with MinMissesForDrill or more misses inside
// ConfusionWindow adds one drill of DrillItems steps, at most once.
func (r *Review) Queue(ctx context.Context, userID int64) (*Session, error) {
	if err := r.ready(); err != nil {
		return nil, err
	}
	now := r.clock.Now()
	due, err := r.store.ListDueCards(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	counts, err := r.store.ConfusionCounts(ctx, userID, now.Add(-ConfusionWindow))
	if err != nil {
		return nil, err
	}

	cards := make([]Item, 0, len(due))
	for _, card := range due {
		cards = append(cards, Item{
			Kind:     ItemCard,
			CardID:   card.ID,
			LexemeID: card.LexemeID,
			Dir:      card.Dir,
		})
	}
	var letters, patterns []Item
	for _, count := range counts {
		if count.Count < MinMissesForDrill {
			continue
		}
		kind, ok := drillKind(count.Kind)
		if !ok {
			continue
		}
		for i := 0; i < DrillItems; i++ {
			item := Item{Kind: kind, Key: count.Key, Index: i}
			switch kind {
			case ItemLetter:
				letters = append(letters, item)
			case ItemPattern:
				patterns = append(patterns, item)
			}
		}
	}
	items := assembleQueue(cards, letters, patterns)
	if items == nil {
		items = []Item{}
	}
	return &Session{Items: items}, nil
}

func drillKind(confusionKind string) (Kind, bool) {
	switch confusionKind {
	case MissLetterPair:
		return ItemLetter, true
	case MissAgreement:
		return ItemPattern, true
	default:
		return "", false
	}
}

// assembleQueue interleaves the three sources and stops at MaxQueue.
// The rotation is card, letter, pattern. A source that is empty is skipped.
// An item whose lexeme matches the previous item is held back until something
// else can sit between them. If nothing can, that item is left for a later session.
func assembleQueue(cards, letters, patterns []Item) []Item {
	buckets := [3][]Item{
		append([]Item(nil), cards...),
		append([]Item(nil), letters...),
		append([]Item(nil), patterns...),
	}
	kinds := [3]Kind{ItemCard, ItemLetter, ItemPattern}
	var out []Item
	var lastKind Kind
	var lastLexeme int64
	started := false
	cursor := 0
	for len(out) < MaxQueue {
		placed := false
		for step := 0; step < len(buckets); step++ {
			b := (cursor + step) % len(buckets)
			if len(buckets[b]) == 0 {
				continue
			}
			if started && kinds[b] == lastKind && otherPlaceable(buckets, b, lastLexeme) {
				continue
			}
			idx := placeableIndex(buckets[b], lastLexeme, started)
			if idx < 0 {
				continue
			}
			item := buckets[b][idx]
			buckets[b] = append(buckets[b][:idx], buckets[b][idx+1:]...)
			out = append(out, item)
			lastKind = kinds[b]
			lastLexeme = item.LexemeID
			started = true
			cursor = (b + 1) % len(buckets)
			placed = true
			break
		}
		if !placed {
			break
		}
	}
	return out
}

func otherPlaceable(buckets [3][]Item, self int, lastLexeme int64) bool {
	for b := range buckets {
		if b == self {
			continue
		}
		if placeableIndex(buckets[b], lastLexeme, true) >= 0 {
			return true
		}
	}
	return false
}

func placeableIndex(items []Item, lastLexeme int64, started bool) int {
	for i, item := range items {
		if started && item.LexemeID != 0 && item.LexemeID == lastLexeme {
			continue
		}
		return i
	}
	return -1
}
