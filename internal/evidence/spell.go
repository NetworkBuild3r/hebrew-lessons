package evidence

import (
	"context"
	"errors"
	"fmt"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// activityTypeIt is the typed recall activity. Spec.Typed is true for it; the
// only other typed activity is dictation, which already reviews spell cards.
const activityTypeIt = "type_it"

// creditTypedSpelling applies ADR-0027 D3 after a type_it answer on a recall
// card: a spell attempt, then an implied spelling review when a card exists or
// the letters gate is open. Wrong and don't-know answers are a no-op.
// Provenance: INIT-032/SPEC-004.
func (r *Recorder) creditTypedSpelling(ctx context.Context, a Attempt) error {
	if a.Activity != activityTypeIt || a.Skill != fsrs.SkillRecall {
		return nil
	}
	if a.DontKnow || !a.Correct {
		return nil
	}
	rating := fsrs.Good
	if a.NearMiss {
		rating = fsrs.Hard
	}

	spell := a
	spell.Skill = fsrs.SkillSpell
	spell.SkipSchedule = true
	now := r.clock.Now()
	if _, err := r.store.WriteAttempt(ctx, db.AttemptWrite{Attempt: spell.row(now)}); err != nil {
		return err
	}

	id, ok, err := r.store.CardID(ctx, a.UserID, a.LexemeID, fsrs.SkillSpell)
	if err != nil {
		return err
	}
	if !ok {
		if !a.LettersPassed {
			return nil
		}
		_, err := r.store.CreateNewCards(ctx, a.UserID, []int64{a.LexemeID}, now, func(bool) []string {
			return []string{fsrs.SkillSpell}
		})
		if err != nil {
			return err
		}
		id, ok, err = r.store.CardID(ctx, a.UserID, a.LexemeID, fsrs.SkillSpell)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("evidence: spelling card was not created")
		}
	}
	return r.spellImplied(ctx, a.UserID, id, rating)
}

// spellImplied schedules an implied spelling review on cardID. rating is Good
// or Hard (ADR-0027 D3). The card must belong to learnerID. Recorder.Implied
// stays Good-only; this is the spelling path.
func (r *Recorder) spellImplied(ctx context.Context, learnerID, cardID int64, rating fsrs.Rating) error {
	if err := r.ready(); err != nil {
		return err
	}
	if rating != fsrs.Good && rating != fsrs.Hard {
		return fmt.Errorf("evidence: spelling implied rating must be Good or Hard, got %d", rating)
	}
	if learnerID <= 0 {
		return errors.New("evidence: spelling implied has no learner")
	}
	card, err := r.store.GetCard(ctx, cardID)
	if err != nil {
		return err
	}
	if card.UserID != learnerID {
		return fmt.Errorf("evidence: card %d belongs to another learner", cardID)
	}
	now := r.clock.Now()
	next, log, err := r.review.ScheduleNext(&card, now, rating)
	if err != nil {
		return err
	}
	log.Implied = true
	return r.store.ApplyReview(ctx, next, log)
}
