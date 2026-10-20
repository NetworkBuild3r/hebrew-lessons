package placement

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Engine.Replace (INIT-015/SPEC-012): a listening clip the client cannot play is swapped for another
// word of the band. The item keeps its id and place, the old word is remembered and never dealt again,
// and nothing is stored as an answer.

// toListening answers the letters right and returns the run, the learner, and the first listening item.
func toListening(t *testing.T, e *Engine, name string) (int64, int64, Step) {
	t.Helper()
	ctx := context.Background()
	d, err := e.loadDeck()
	if err != nil {
		t.Fatal(err)
	}
	user := newLearner(t, e.store, name)
	runID, err := e.Start(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	step, err := e.Next(ctx, user, runID)
	if err != nil {
		t.Fatal(err)
	}
	state := newScriptState(script{letters: 6, pattern: always('R')})
	for step.Item != nil && step.Item.Part == PartLetters {
		step, err = e.Answer(ctx, user, runID, step.Item.ID, state.answer(d, openRow(t, e, runID, step.Item.ID)))
		if err != nil {
			t.Fatal(err)
		}
	}
	if step.Item == nil || step.Item.Part != PartListening {
		t.Fatalf("expected a listening item, got %+v", step)
	}
	return runID, user, step
}

func TestReplaceSwapsTheWordInPlace(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 41)
	ctx := context.Background()
	runID, user, step := toListening(t, e, "replace")
	first := *step.Item
	old := openRow(t, e, runID, first.ID)
	attempts := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, user)
	items := len(itemsOf(t, e, runID))

	got, err := e.Replace(ctx, user, runID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Item == nil || got.Item.ID != first.ID || got.Item.Part != PartListening || got.Item.Seq != first.Seq {
		t.Fatalf("the replacement is not the same item: %+v", got.Item)
	}
	if got.Item.AudioURL == "" || got.Item.AudioURL == first.AudioURL {
		t.Errorf("clip %q was %q", got.Item.AudioURL, first.AudioURL)
	}
	if got.Item.Prompt != "" || len(got.Item.Options) != OptionCount {
		t.Errorf("the replacement is not a listening item with no Hebrew: %+v", got.Item)
	}
	now := openRow(t, e, runID, first.ID)
	if now.ItemID == old.ItemID || now.Band != old.Band || now.Outcome != "" || !now.CorrectIdx.Valid {
		t.Errorf("the row was not swapped within its band: %+v then %+v", old, now)
	}
	if !strings.Contains(now.Options, `"skipped":["`+old.ItemID+`"]`) {
		t.Errorf("the old word is not remembered: %s", now.Options)
	}
	if n := len(itemsOf(t, e, runID)); n != items {
		t.Errorf("%d items, was %d", n, items)
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, user); n != attempts {
		t.Errorf("a replaced clip wrote %d attempts", n-attempts)
	}

	// Replace again: both earlier words are remembered, and the third word is a new one.
	again, err := e.Replace(ctx, user, runID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	third := openRow(t, e, runID, again.Item.ID)
	if third.ItemID == old.ItemID || third.ItemID == now.ItemID {
		t.Errorf("the third word %s is one already swapped out", third.ItemID)
	}
	if !strings.Contains(third.Options, old.ItemID) || !strings.Contains(third.Options, now.ItemID) {
		t.Errorf("not every swapped word is remembered: %s", third.Options)
	}

	// The rest of the run never deals a swapped-out word.
	d, _ := e.loadDeck()
	state := newScriptState(script{letters: 6, pattern: always('R'), typing: "skip"})
	step = again
	for i := 0; i < 400 && !step.Done; i++ {
		switch {
		case step.TypingOffer:
			if _, err := e.Finish(ctx, user, runID); err != nil {
				t.Fatal(err)
			}
			step.Done = true
		case step.Item != nil:
			if step, err = e.Answer(ctx, user, runID, step.Item.ID, state.answer(d, openRow(t, e, runID, step.Item.ID))); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("empty step %+v", step)
		}
	}
	for _, it := range itemsOf(t, e, runID) {
		if it.Part != string(PartTyping) && (it.ItemID == old.ItemID || it.ItemID == now.ItemID) {
			t.Errorf("item %d deals swapped-out word %s", it.Seq, it.ItemID)
		}
	}
}

func TestReplaceRefusals(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 42)
	ctx := context.Background()
	runID, user, step := toListening(t, e, "refuse")
	other := newLearner(t, store, "other")

	if _, err := e.Replace(ctx, other, runID, step.Item.ID); !errors.Is(err, ErrNotYourRun) {
		t.Errorf("another learner: %v", err)
	}
	if _, err := e.Replace(ctx, user, runID, "p99"); !errors.Is(err, ErrWrongItem) {
		t.Errorf("a made-up item: %v", err)
	}
	// An answered item is not the open one.
	d, _ := e.loadDeck()
	answered := step.Item.ID
	if _, err := e.Answer(ctx, user, runID, answered, newScriptState(script{letters: 6, pattern: always('R')}).answer(d, openRow(t, e, runID, answered))); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Replace(ctx, user, runID, answered); !errors.Is(err, ErrWrongItem) {
		t.Errorf("an answered item: %v", err)
	}

	// A letters item has no spare clip.
	letters := newLearner(t, store, "letters")
	id, err := e.Start(ctx, letters)
	if err != nil {
		t.Fatal(err)
	}
	st, err := e.Next(ctx, letters, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Replace(ctx, letters, id, st.Item.ID); !errors.Is(err, ErrNotReady) {
		t.Errorf("a letters item: %v", err)
	}
}

func TestReplaceWithNoClipLeftKeepsTheItem(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 43)
	ctx := context.Background()
	runID, user, step := toListening(t, e, "noclip")
	before := openRow(t, e, runID, step.Item.ID)
	// Every clip but the one on screen is gone.
	if _, err := store.ExecContext(ctx, `DELETE FROM audio_assets WHERE id != ?`, strings.TrimSuffix(strings.TrimPrefix(step.Item.AudioURL, "/audio/"), ".mp3")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Replace(ctx, user, runID, step.Item.ID); !errors.Is(err, ErrNoAudio) {
		t.Fatalf("Replace = %v, want ErrNoAudio", err)
	}
	after := openRow(t, e, runID, step.Item.ID)
	if after.ItemID != before.ItemID || after.Options != before.Options || after.Outcome != "" {
		t.Errorf("a refused replacement changed the item: %+v then %+v", before, after)
	}
}

// The screen hides its button after MaxReplaces, but a client can post replace as often as it
// likes, so the engine holds the cap: a learner cannot skip words until a familiar one comes up.
func TestReplaceIsCappedPerItem(t *testing.T) {
	e, _ := openEngine(t, fixedClock{when}, 44)
	ctx := context.Background()
	runID, user, step := toListening(t, e, "cap")
	id := step.Item.ID
	for i := 0; i < MaxReplaces; i++ {
		if _, err := e.Replace(ctx, user, runID, id); err != nil {
			t.Fatalf("replace %d: %v", i+1, err)
		}
	}
	before := openRow(t, e, runID, id)
	for i := 0; i < 5; i++ {
		if _, err := e.Replace(ctx, user, runID, id); !errors.Is(err, ErrNotReady) {
			t.Fatalf("replace past the cap = %v, want ErrNotReady", err)
		}
	}
	after := openRow(t, e, runID, id)
	if after.ItemID != before.ItemID || after.Options != before.Options {
		t.Errorf("a refused replacement changed the item: %+v then %+v", before, after)
	}
}
