package placement

import (
	"context"
	"fmt"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// TestEngineFinishStoresStopReason locks the value Engine.finish writes.
// A finished run never stores an empty stop reason. band_stop is an adaptive
// stop and wins over the typing cap. cap_cards is TypingMax answered typing
// items with a right word still untyped. exhausted is every other finish,
// including a cleared deck, a skipped or early typing part, and a cap that
// was not reached. Migration 003 stores NULL only when a run finished before
// a stop; this engine path does not.
func TestEngineFinishStoresStopReason(t *testing.T) {
	cases := []struct {
		name string
		want string
		run  func(t *testing.T, e *Engine, store *db.Store, user int64) int64
	}{
		{name: "exhausted", want: stopExhausted, run: driveScript(script{letters: LettersAsked, pattern: always('R'), typing: "skip"})},
		{name: "band_stop", want: stopBand, run: driveScript(script{letters: LettersAsked, pattern: always('W'), typing: "skip"})},
		{name: "cap_cards", want: stopCards, run: driveScript(script{letters: LettersAsked, pattern: always('R'), typing: "right"})},
		{name: "recheck listening stops", want: stopBand, run: recheckStop(SkillListen)},
		{name: "recheck reading stops", want: stopBand, run: recheckStop(SkillRead)},
		{name: "recheck stops above band 1", want: stopBand, run: recheckAboveBand1()},
		{name: "listening only stops", want: stopBand, run: driveScript(script{letters: 0, pattern: always('W'), typing: "skip"})},
		{name: "reading only stops", want: stopBand, run: driveScript(script{letters: LettersAsked, pattern: readWrong, typing: "skip"})},
		{name: "band stop wins over the cap", want: stopBand, run: driveScript(script{letters: LettersAsked, pattern: readWrong, typing: "right"})},
		{name: "typing max with every right word typed", want: stopExhausted, run: typedBoundary(TypingMax, TypingMax, false)},
		{name: "one under the typing cap", want: stopExhausted, run: typedBoundary(TypingMax-1, TypingMax+1, false)},
		{name: "open typing item is not counted", want: stopExhausted, run: typedBoundary(TypingMax-1, TypingMax+1, true)},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, store := openEngine(t, fixedClock{when}, uint64(40+i))
			user := newLearner(t, store, "stop-"+fmt.Sprint(i))
			runID := tc.run(t, e, store, user)
			run, err := store.GetPlacementRun(context.Background(), runID)
			if err != nil {
				t.Fatal(err)
			}
			if run.FinishedAt.IsZero() {
				t.Fatal("the run is not finished")
			}
			if run.StopReason != tc.want {
				t.Fatalf("stop reason %q, want %q", run.StopReason, tc.want)
			}
		})
	}
}

// readWrong clears listening and stops reading, so the stop comes from reading alone.
func readWrong(skill string, _, _ int) byte {
	if skill == SkillRead {
		return 'W'
	}
	return 'R'
}

func driveScript(sc script) func(*testing.T, *Engine, *db.Store, int64) int64 {
	return func(t *testing.T, e *Engine, _ *db.Store, user int64) int64 {
		t.Helper()
		runID, _, _ := drive(t, e, user, sc)
		return runID
	}
}

// recheckAboveBand1 places the learner in band 1, then a reading recheck of
// the next band answers wrong. The stop has to use that band: clamping every
// recheck to band 1 would ignore the answers.
func recheckAboveBand1() func(*testing.T, *Engine, *db.Store, int64) int64 {
	return func(t *testing.T, e *Engine, _ *db.Store, user int64) int64 {
		t.Helper()
		ctx := context.Background()
		drive(t, e, user, script{letters: LettersAsked, pattern: knows(1), typing: "skip"})
		runID, err := e.NewRecheck(ctx, user, SkillRead)
		if err != nil {
			t.Fatal(err)
		}
		answerRun(t, e, user, runID, script{pattern: always('W')})
		items := itemsOf(t, e, runID)
		above := false
		for _, it := range items {
			if it.Part == string(PartReading) && it.Band > 1 && it.Outcome != "" {
				above = true
			}
		}
		if !above {
			t.Fatal("the recheck never answered above band 1")
		}
		return runID
	}
}

// recheckStop finishes a placement that tested the skill, then a one-band
// recheck that answers wrong until the adaptive rule stops.
func recheckStop(skill string) func(*testing.T, *Engine, *db.Store, int64) int64 {
	return func(t *testing.T, e *Engine, _ *db.Store, user int64) int64 {
		t.Helper()
		ctx := context.Background()
		initial := script{letters: LettersAsked, pattern: always('W'), typing: "skip"}
		if skill == SkillListen {
			// Letters failed, so the first run never deals reading.
			initial.letters = 0
		}
		drive(t, e, user, initial)
		runID, err := e.NewRecheck(ctx, user, skill)
		if err != nil {
			t.Fatal(err)
		}
		answerRun(t, e, user, runID, script{pattern: always('W')})
		return runID
	}
}

// typedBoundary finishes from the typing phase. right words were right in
// listening. answered of them are typed, and one more typing item is left
// open when open is set. The phase is typing, so Finish is allowed before
// the parts would otherwise be over.
func typedBoundary(answered, right int, open bool) func(*testing.T, *Engine, *db.Store, int64) int64 {
	return func(t *testing.T, e *Engine, store *db.Store, user int64) int64 {
		t.Helper()
		ctx := context.Background()
		runID, err := e.Start(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		seq := 1
		for i := 0; i < right; i++ {
			storeOutcome(t, store, runID, seq, string(PartListening), SkillListen, 1, fmt.Sprintf("w%d", i), OutcomeRight)
			seq++
		}
		for i := 0; i < answered; i++ {
			storeOutcome(t, store, runID, seq, string(PartTyping), SkillRecall, 1, fmt.Sprintf("w%d", i), OutcomeRight)
			seq++
		}
		if open {
			storeOutcome(t, store, runID, seq, string(PartTyping), SkillRecall, 1, fmt.Sprintf("w%d", answered), "")
		}
		ok, err := store.SetPlacementV3Phase(ctx, runID, db.PhaseTyping)
		if err != nil || !ok {
			t.Fatalf("phase: ok=%v err=%v", ok, err)
		}
		if _, err := e.Finish(ctx, user, runID); err != nil {
			t.Fatal(err)
		}
		return runID
	}
}

func storeOutcome(t *testing.T, store *db.Store, runID int64, seq int, part, skill string, band int, itemID, outcome string) {
	t.Helper()
	ctx := context.Background()
	it := db.PlacementItem{
		RunID: runID, Seq: seq, Part: part, Skill: skill, Band: band, ItemID: itemID,
		Options: `{"options":[]}`, ShownAt: when,
	}
	if err := store.InsertPlacementItem(ctx, it); err != nil {
		t.Fatal(err)
	}
	if outcome == "" {
		return
	}
	ok, err := store.ClaimPlacementItem(ctx, runID, seq, outcome, "", 0, when, 0)
	if err != nil || !ok {
		t.Fatalf("claim %s: ok=%v err=%v", itemID, ok, err)
	}
}

// answerRun answers a run that is already open until the engine finishes it.
func answerRun(t *testing.T, e *Engine, user, runID int64, sc script) {
	t.Helper()
	ctx := context.Background()
	d, err := e.loadDeck()
	if err != nil {
		t.Fatal(err)
	}
	state := newScriptState(sc)
	step, err := e.Next(ctx, user, runID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 80 && !step.Done; i++ {
		if step.Item == nil {
			t.Fatalf("step %+v", step)
		}
		step, err = e.Answer(ctx, user, runID, step.Item.ID, state.answer(d, openRow(t, e, runID, step.Item.ID)))
		if err != nil {
			t.Fatal(err)
		}
	}
	if !step.Done {
		t.Fatal("the run did not finish")
	}
}
