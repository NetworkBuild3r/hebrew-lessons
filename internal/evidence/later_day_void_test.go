package evidence

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// A voided later-day review drops out of the share, and counts again when the
// void is undone or the include-voided read is used. INIT-038/SPEC-002.
func TestLaterDayResultsSkipsVoidedAttempt(t *testing.T) {
	e := newLaterDayEnv(t, "UTC")
	ctx := context.Background()
	lex := e.lex("מָחָר")
	practice := e.now.Add(-72 * time.Hour)
	review := e.now.Add(-24 * time.Hour)
	e.insert(e.user, lex, "cloze", "lesson", true, practice)

	wrote, err := e.store.WriteAttempt(ctx, db.AttemptWrite{
		Attempt: db.Attempt{
			UserID: e.user, LexemeID: lex, Skill: "read", Activity: "read_pick",
			Source: SourceReview, Prompt: "p", Response: "r", Correct: true,
			LatencyMS: 1000, CreatedAt: review,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	before, err := LaterDayResults(ctx, e.store, e.user, e.now, e.zone)
	if err != nil {
		t.Fatal(err)
	}
	got := resultByKind(before)
	if got["cloze"].Total != 1 || got["cloze"].Right != 1 {
		t.Fatalf("before void = %+v", got["cloze"])
	}

	if _, err := e.store.InsertAnswerVoids(ctx, "batch-later", e.now, []db.AnswerVoidWrite{{
		AttemptID: wrote.AttemptID, LearnerID: e.user,
		Reason: db.VoidReasonAlsoRight, Actor: "learner",
	}}); err != nil {
		t.Fatal(err)
	}
	after, err := LaterDayResults(ctx, e.store, e.user, e.now, e.zone)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Fatalf("voided review still counts: %+v", after)
	}
	included, err := LaterDayResultsIncludeVoided(ctx, e.store, e.user, e.now, e.zone)
	if err != nil {
		t.Fatal(err)
	}
	if resultByKind(included)["cloze"].Total != 1 {
		t.Fatalf("include-voided = %+v", included)
	}
	if err := e.store.UndoAnswerVoidBatch(ctx, "batch-later", e.now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	restored, err := LaterDayResults(ctx, e.store, e.user, e.now, e.zone)
	if err != nil {
		t.Fatal(err)
	}
	if resultByKind(restored)["cloze"].Total != 1 || resultByKind(restored)["cloze"].Right != 1 {
		t.Fatalf("after undo = %+v", restored)
	}
}
