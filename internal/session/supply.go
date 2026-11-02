package session

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
)

// seedNewFromDeck introduces placement-list words when the learner is at or
// above TargetSuccess and fewer due new cards wait than the accuracy tier
// allows. Called only from a full build (only == nil), before ListStudyCards.
// INIT-027/SPEC-005; ADR-0023 D5.
func (s *Service) seedNewFromDeck(ctx context.Context, userID int64, accuracy float64, now time.Time, gate activity.Gate) error {
	if accuracy < TargetSuccess {
		return nil
	}
	waiting, err := s.store.CountDueNewCards(ctx, userID, now)
	if err != nil {
		return err
	}
	cap := newAllowance(accuracy, MaxNewPerSession)
	if waiting >= cap {
		return nil
	}
	need := cap - waiting
	ids, err := s.pickSeedLexemes(ctx, userID, need)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	skills := func(hasClip bool) []string {
		out := make([]string, 0, 3)
		for _, sk := range []string{fsrs.SkillRead, fsrs.SkillRecall} {
			if gate.Open(sk) {
				out = append(out, sk)
			}
		}
		if hasClip && gate.Open(fsrs.SkillListen) {
			out = append(out, fsrs.SkillListen)
		}
		return out
	}
	_, err = s.record.Introduce(ctx, userID, ids, skills)
	return err
}

// pickSeedLexemes walks the placement deck in band order, then file order,
// and returns up to need lexeme ids the learner has no card for.
func (s *Service) pickSeedLexemes(ctx context.Context, userID int64, need int) ([]int64, error) {
	if need <= 0 {
		return nil, nil
	}
	rows, err := placement.Deck()
	if err != nil {
		return nil, err
	}
	// Band first, file order within a band (Deck is file order).
	order := append([]placement.DeckRow(nil), rows...)
	slices.SortStableFunc(order, func(a, b placement.DeckRow) int {
		return a.Band - b.Band
	})

	pairs, err := s.store.PlacementItemLexemes(ctx)
	if err != nil {
		return nil, err
	}
	itemLexeme := map[string]int64{}
	for _, p := range pairs {
		if p.ItemID == "" || p.LexemeID <= 0 {
			continue
		}
		if _, ok := itemLexeme[p.ItemID]; !ok {
			itemLexeme[p.ItemID] = p.LexemeID
		}
	}

	haveCard, err := s.store.LearnerCardLexemeIDs(ctx, userID)
	if err != nil {
		return nil, err
	}

	picked := make([]int64, 0, need)
	seenLex := map[int64]struct{}{}
	for _, row := range order {
		if len(picked) >= need {
			break
		}
		if !row.Verified || strings.TrimSpace(row.GlossEN) == "" {
			continue
		}
		lexemeID, ok := itemLexeme[row.ID]
		if !ok {
			continue
		}
		if _, dup := seenLex[lexemeID]; dup {
			continue
		}
		if _, has := haveCard[lexemeID]; has {
			continue
		}
		face, found, err := s.store.LexemeByID(ctx, lexemeID)
		if err != nil {
			return nil, err
		}
		if !found || strings.TrimSpace(face.Gloss) == "" {
			continue
		}
		seenLex[lexemeID] = struct{}{}
		picked = append(picked, lexemeID)
	}
	if len(picked) > need {
		return nil, fmt.Errorf("session: seeded %d words, need at most %d", len(picked), need)
	}
	return picked, nil
}
