package activity

import (
	"context"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
)

// Expose records that words were shown to the learner with no question asked, as a
// lesson_intro exposure. New words open with it before their first check: the session mix
// sets CardRef.Intro on a new card whose word has none, and the screen that shows the word
// calls Expose. unitID 0 means no unit. A word listed twice is recorded once.
func (e *Engine) Expose(ctx context.Context, userID int64, lexemeIDs []int64, unitID int64) error {
	if err := e.ready(); err != nil {
		return err
	}
	return e.rec.Expose(ctx, userID, lexemeIDs, evidence.ContextLessonIntro, unitID)
}
