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
	haveCard, err := s.store.LearnerCardLexemeIDs(ctx, userID)
	if err != nil {
		return err
	}
	behind, err := s.store.LexemesBehindUnfinishedIntro(ctx, userID)
	if err != nil {
		return err
	}
	ids, err := s.pickCourseLexemes(ctx, need, haveCard, behind)
	if err != nil {
		return err
	}
	if len(ids) < need {
		exclude := map[int64]struct{}{}
		for _, id := range ids {
			exclude[id] = struct{}{}
		}
		more, err := s.pickSeedLexemes(ctx, userID, need-len(ids), haveCard, exclude)
		if err != nil {
			return err
		}
		ids = append(ids, more...)
	}
	if len(ids) == 0 {
		return nil
	}
	skills := lessonSkills(gate)
	_, err = s.record.Introduce(ctx, userID, ids, skills)
	return err
}

// lessonSkills is the skills a new word gets from the seeder: read and recall
// when the gate is open, plus listen when the word has a clip.
func lessonSkills(gate activity.Gate) func(bool) []string {
	return func(hasClip bool) []string {
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
}

// taughtSkills is what one lesson's word gets: read and recall, plus listen when
// it has a clip. It is the same set RecordTaught writes, because opening a lesson
// on a word is the same act as recording it as taught. A closed letters gate does
// not empty this set: review serves a read card as a two-choice pick.
func taughtSkills(hasClip bool) []string {
	if hasClip {
		return []string{fsrs.SkillRead, fsrs.SkillRecall, fsrs.SkillListen}
	}
	return []string{fsrs.SkillRead, fsrs.SkillRecall}
}

// BuildLesson introduces the named words, then builds a sitting of them.
// It is what a lesson page uses to study its own words. An existing card is left
// as it is, and a word outside the list is not touched.
func (s *Service) BuildLesson(ctx context.Context, userID int64, lexemeIDs []int64) (*Session, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if len(lexemeIDs) > 0 {
		if _, err := s.record.Introduce(ctx, userID, lexemeIDs, taughtSkills); err != nil {
			return nil, err
		}
	}
	return s.BuildSessionFor(ctx, userID, lexemeIDs)
}

// pickCourseLexemes walks the course list and returns up to need lexeme ids
// the learner has no card for. A word behind an unfinished intro lesson is
// skipped, because review would hide its card.
func (s *Service) pickCourseLexemes(ctx context.Context, need int, haveCard map[int64]struct{}, behind map[int64]bool) ([]int64, error) {
	if need <= 0 {
		return nil, nil
	}
	rows, err := s.store.CourseLexemes(ctx)
	if err != nil {
		return nil, err
	}
	picked := make([]int64, 0, need)
	for _, row := range rows {
		if len(picked) >= need {
			break
		}
		if _, has := haveCard[row.LexemeID]; has || behind[row.LexemeID] {
			continue
		}
		face, found, err := s.store.LexemeByID(ctx, row.LexemeID)
		if err != nil {
			return nil, err
		}
		if !found || strings.TrimSpace(face.Gloss) == "" {
			continue
		}
		picked = append(picked, row.LexemeID)
	}
	return picked, nil
}

// CourseWaiting is how many course words the next full build would add.
// It is zero when recent accuracy is under TargetSuccess, or the new-card
// tier is already full. It does not write.
func (s *Service) CourseWaiting(ctx context.Context, userID int64) (int, string, error) {
	if err := s.ready(); err != nil {
		return 0, "", err
	}
	now := s.clock.Now()
	ratings, err := s.store.RecentReviewRatings(ctx, userID, now, RecentReviews)
	if err != nil {
		return 0, "", err
	}
	accuracy := accuracyOf(ratings)
	if accuracy < TargetSuccess {
		return 0, "Recent answers are under 85 out of 100 right, so today is review only.", nil
	}
	waiting, err := s.store.CountDueNewCards(ctx, userID, now)
	if err != nil {
		return 0, "", err
	}
	cap := newAllowance(accuracy, MaxNewPerSession)
	if waiting >= cap {
		return 0, "", nil
	}
	haveCard, err := s.store.LearnerCardLexemeIDs(ctx, userID)
	if err != nil {
		return 0, "", err
	}
	behind, err := s.store.LexemesBehindUnfinishedIntro(ctx, userID)
	if err != nil {
		return 0, "", err
	}
	ids, err := s.pickCourseLexemes(ctx, cap-waiting, haveCard, behind)
	if err != nil {
		return 0, "", err
	}
	return len(ids), "", nil
}

// pickSeedLexemes walks the placement deck in band order, then file order,
// and returns up to need lexeme ids the learner has no card for.
func (s *Service) pickSeedLexemes(ctx context.Context, userID int64, need int, haveCard map[int64]struct{}, exclude map[int64]struct{}) ([]int64, error) {
	if need <= 0 {
		return nil, nil
	}
	if haveCard == nil {
		var err error
		haveCard, err = s.store.LearnerCardLexemeIDs(ctx, userID)
		if err != nil {
			return nil, err
		}
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
		if _, skip := exclude[lexemeID]; skip {
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
