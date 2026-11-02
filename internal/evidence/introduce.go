package evidence

import (
	"context"
	"errors"
	"fmt"
)

// IntroduceWordResult is one lexeme's outcome from Introduce.
type IntroduceWordResult struct {
	LexemeID int64
	Created  []string
}

// IntroduceResult is the outcome of Introduce for every requested word.
type IntroduceResult struct {
	Words []IntroduceWordResult
}

// Introduce creates new-state cards due at the recorder clock's now for the
// named lexemes. skills(hasClip) chooses the skills per lexeme. It never
// writes taught_words, attempts, exposures, or review_log, and never changes
// an existing card. INIT-027/SPEC-005; ADR-0023 D5.
func (r *Recorder) Introduce(ctx context.Context, learnerID int64, lexemeIDs []int64, skills func(hasClip bool) []string) (IntroduceResult, error) {
	out := IntroduceResult{}
	if err := r.ready(); err != nil {
		return out, err
	}
	if learnerID <= 0 {
		return out, errors.New("evidence: introduce has no learner")
	}
	if skills == nil {
		return out, errors.New("evidence: introduce has no skills")
	}
	ids := make([]int64, 0, len(lexemeIDs))
	seen := map[int64]struct{}{}
	for _, id := range lexemeIDs {
		if id <= 0 {
			return out, fmt.Errorf("evidence: introduce lexeme id %d is not valid", id)
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return out, nil
	}
	due := r.clock.Now()
	stored, err := r.store.CreateNewCards(ctx, learnerID, ids, due, skills)
	if err != nil {
		return out, err
	}
	out.Words = make([]IntroduceWordResult, len(stored.Words))
	for i, w := range stored.Words {
		created := w.Created
		if created == nil {
			created = []string{}
		}
		out.Words[i] = IntroduceWordResult{LexemeID: w.LexemeID, Created: created}
	}
	return out, nil
}
