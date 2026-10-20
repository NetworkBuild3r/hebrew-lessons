package placement

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// The result, the review of misses, the typing part, and the re-check.

func TestResultShapeAndMissesReview(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 20)
	ctx := context.Background()
	user := newLearner(t, store, "result")
	d, _ := e.loadDeck()
	runID, res, _ := drive(t, e, user, script{letters: 6, pattern: cycle(), typing: "skip"})

	if res.RunID != runID || res.UserID != user || res.Kind != "initial" || !res.StartedAt.Equal(when) || !res.FinishedAt.Equal(when) {
		t.Fatalf("header %+v", res)
	}
	// Level and estimate follow the bands that were asked.
	for _, s := range []SkillResult{res.Listening, res.Reading} {
		if !s.Tested {
			t.Fatalf("%s was not tested", s.Skill)
		}
		var tallies [BandCount]Tally
		for _, b := range s.Bands {
			tallies[b.Band-1] = Tally{Asked: b.Asked, Right: b.Right, Wrong: b.Wrong, DontKnow: b.DontKnow}
			if b.Asked != b.Right+b.Wrong+b.DontKnow || b.Share != Share(tallies[b.Band-1]) {
				t.Fatalf("band %+v", b)
			}
		}
		if s.Level != LevelOf(tallies) || s.Estimate != int(EstimateWords(tallies)+0.5) {
			t.Fatalf("%s: level %d estimate %d from %+v", s.Skill, s.Level, s.Estimate, s.Bands)
		}
	}

	// A miss for each wrong answer and each "I don't know", with what the review screen shows.
	var want []Miss
	for _, it := range itemsOf(t, e, runID) {
		if (it.Part != string(PartListening) && it.Part != string(PartReading)) || it.Outcome == OutcomeRight {
			continue
		}
		w := d.byID[it.ItemID]
		clip, _, _ := store.AudioIDByText(ctx, audio.NormalizeText(w.Text))
		m := Miss{LexemeID: it.LexemeID, Skill: it.Skill, RightGloss: w.Gloss, Pointed: w.Text, AudioID: clip}
		if it.Outcome == OutcomeWrong {
			m.Picked = it.Picked
			if m.Picked == "" || m.Picked == w.Gloss {
				t.Fatalf("item %d: picked %q for a wrong answer", it.Seq, m.Picked)
			}
		}
		want = append(want, m)
	}
	if len(want) == 0 || !reflect.DeepEqual(res.Misses, want) {
		t.Fatalf("misses\n got: %+v\nwant: %+v", res.Misses, want)
	}
	for _, m := range res.Misses {
		if m.AudioID == "" || m.Pointed == "" || m.RightGloss == "" || m.LexemeID == 0 {
			t.Fatalf("miss is missing a field: %+v", m)
		}
	}

	// The start unit is what the store reports for the learner.
	place, ok, err := store.LatestPlacement(ctx, user)
	if err != nil || !ok || res.StartUnit != place.SuggestedStartPosition {
		t.Fatalf("start unit %d, store says %d %v %v", res.StartUnit, place.SuggestedStartPosition, ok, err)
	}

	// Finishing again, or asking for the result, gives the same result and writes nothing more.
	exposures := countRows(t, store, `SELECT COUNT(*) FROM exposures`)
	again, err := e.Finish(ctx, user, runID)
	if err != nil {
		t.Fatal(err)
	}
	read, err := e.Result(ctx, user, runID)
	if err != nil {
		t.Fatal(err)
	}
	next, err := e.Next(ctx, user, runID)
	if err != nil || !next.Done || next.Result == nil {
		t.Fatalf("Next on a finished run: %+v %v", next, err)
	}
	for name, got := range map[string]RunResult{"Finish": again, "Result": read, "Next": *next.Result} {
		a, _ := json.Marshal(got)
		b, _ := json.Marshal(res)
		if string(a) != string(b) {
			t.Fatalf("%s returned another result:\n%s\n%s", name, a, b)
		}
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM exposures`); n != exposures {
		t.Fatalf("exposures went from %d to %d on a second Finish", exposures, n)
	}
	if latest, ok, err := e.Latest(ctx, user); err != nil || !ok || latest.RunID != runID {
		t.Fatalf("Latest = %+v %v %v", latest, ok, err)
	}
}

// Finish and Result are refused before the run is over.
func TestFinishBeforeThePartsAreOverIsRefused(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 21)
	ctx := context.Background()
	user := newLearner(t, store, "early")
	runID, err := e.Start(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Next(ctx, user, runID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Finish(ctx, user, runID); !errors.Is(err, ErrNotReady) {
		t.Fatalf("Finish at the first item: %v, want ErrNotReady", err)
	}
	if _, err := e.Result(ctx, user, runID); !errors.Is(err, ErrNotReady) {
		t.Fatalf("Result of an open run: %v, want ErrNotReady", err)
	}
	if _, err := e.StartTyping(ctx, user, runID); !errors.Is(err, ErrNotReady) {
		t.Fatalf("StartTyping at the first item: %v, want ErrNotReady", err)
	}
	if run, _ := store.GetPlacementRun(ctx, runID); !run.FinishedAt.IsZero() {
		t.Fatal("the run was finished")
	}
}

// After a v3 run, the store reads it the way it read a v2 run: finished, with a level and bands. The word
// to deck-item link that gives a word its band is there too.
func TestResultIsReadableThroughTheStoreReads(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 22)
	ctx := context.Background()
	user := newLearner(t, store, "compat")
	runID, res, _ := drive(t, e, user, script{letters: 6, pattern: knows(3), typing: "skip"})

	place, ok, err := store.LatestPlacement(ctx, user)
	if err != nil || !ok || !place.Finished {
		t.Fatalf("LatestPlacement = %+v %v %v", place, ok, err)
	}
	if place.Level != res.Reading.Level-1 {
		t.Fatalf("stored level %d, want the reading level %d less one", place.Level, res.Reading.Level)
	}
	if place.EstimatedKnownWords != res.Reading.Estimate {
		t.Fatalf("estimated known words %d, want %d", place.EstimatedKnownWords, res.Reading.Estimate)
	}
	if len(place.BandScores) != 4 {
		t.Fatalf("band scores %+v, want the four bands that were asked", place.BandScores)
	}
	for i, b := range place.BandScores {
		if b.Band != i || b.Rated == 0 {
			t.Fatalf("band score %d: %+v (a deck band, 0 to 4)", i, b)
		}
	}
	// Bands 1 to 3 pooled over both skills: 12 right of 12.
	if b := place.BandScores[0]; b.Rated != 2*FirstSize || b.Good != 2*FirstSize || b.GoodPlusRate != 1 {
		t.Fatalf("band 0: %+v", b)
	}
	if bands, ok, err := store.PlacementBands(ctx, user); err != nil || !ok || len(bands) != 4 {
		t.Fatalf("PlacementBands = %v %v %v", bands, ok, err)
	}

	pairs, err := store.PlacementItemLexemes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]int64{}
	for _, p := range pairs {
		have[p.ItemID] = p.LexemeID
	}
	words := 0
	for _, it := range itemsOf(t, e, runID) {
		if it.Part == string(PartLetters) {
			if _, ok := have[it.ItemID]; ok {
				t.Fatalf("letter %s is linked to a lexeme", it.ItemID)
			}
			continue
		}
		words++
		if have[it.ItemID] != it.LexemeID {
			t.Fatalf("item %s: link to lexeme %d, want %d", it.ItemID, have[it.ItemID], it.LexemeID)
		}
		if _, band, ok := Lookup(it.ItemID); !ok || band+1 != it.Band {
			t.Fatalf("item %s: Lookup band %d, stored band %d", it.ItemID, band, it.Band)
		}
	}
	if words == 0 {
		t.Fatal("no words")
	}
}

// Part 3 asks only words that were right, grades the typing with fsrs.GradeTyped, and counts only an exact
// answer as right. Pointing is ignored, as the grader ignores it.
func TestTypingIsGradedByGradeTyped(t *testing.T) {
	for _, tc := range []struct {
		typing    string
		wantRight bool
		wantCard  bool
	}{
		{"right", true, true},
		{"offbyone", false, false}, // one letter off is Hard to the grader, which is not right
		{"wrong", false, false},
		{"idk", false, false},
	} {
		t.Run(tc.typing, func(t *testing.T) {
			e, store := openEngine(t, fixedClock{when}, 23)
			ctx := context.Background()
			user := newLearner(t, store, "typing")
			d, _ := e.loadDeck()
			runID, res, _ := drive(t, e, user, script{letters: 6, pattern: knows(1), typing: tc.typing})
			items := itemsOf(t, e, runID)
			rightBefore := map[string]bool{}
			typed := 0
			for _, it := range items {
				switch it.Part {
				case string(PartListening), string(PartReading):
					if it.Outcome == OutcomeRight {
						rightBefore[it.ItemID] = true
					}
				case string(PartTyping):
					typed++
					w := d.byID[it.ItemID]
					if !rightBefore[it.ItemID] {
						t.Fatalf("typing asked %s, which was not right before", it.ItemID)
					}
					if (it.Outcome == OutcomeRight) != tc.wantRight {
						t.Fatalf("%s: outcome %s", tc.typing, it.Outcome)
					}
					if tc.typing != "idk" && it.Picked == "" {
						t.Fatalf("typed text was not stored")
					}
					// The recall card is seeded Good by a right typed answer, and by nothing else.
					id, hasCard, err := store.CardID(ctx, user, it.LexemeID, fsrs.SkillRecall)
					if err != nil {
						t.Fatal(err)
					}
					if hasCard != tc.wantCard {
						t.Fatalf("%s: recall card exists = %v", tc.typing, hasCard)
					}
					if hasCard {
						if logs, _ := store.ReviewLogs(ctx, id); len(logs) != 1 || logs[0].Rating != int(fsrs.Good) {
							t.Fatalf("recall card logs %+v", logs)
						}
					}
					attempts, _ := store.ListAttempts(ctx, user, it.LexemeID)
					found := false
					for _, a := range attempts {
						if a.Skill == fsrs.SkillRecall {
							found = true
							if a.Activity != "type_it" || a.Source != "placement" || a.Prompt != w.Gloss || a.Correct != tc.wantRight {
								t.Fatalf("typing attempt %+v", a)
							}
						}
					}
					if !found {
						t.Fatalf("no recall attempt for %s", it.ItemID)
					}
				}
			}
			if typed == 0 || typed > TypingMax {
				t.Fatalf("%d typing items, want 1 to %d", typed, TypingMax)
			}
			if !res.Typing.Taken || res.Typing.Asked != typed || (res.Typing.Right == typed) != tc.wantRight {
				t.Fatalf("typing result %+v for %d items", res.Typing, typed)
			}
			missed := 0
			for _, m := range res.Misses {
				if m.Skill == fsrs.SkillRecall {
					missed++
				}
			}
			if wantMissed := map[bool]int{true: 0, false: typed}[tc.wantRight]; missed != wantMissed {
				t.Fatalf("%d typing misses, want %d", missed, wantMissed)
			}
		})
	}
}

// Typing is optional: skipping it leaves no typing items and no recall cards, and Finish ends the run.
func TestTypingCanBeSkipped(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 24)
	user := newLearner(t, store, "skip")
	runID, res, steps := drive(t, e, user, script{letters: 6, pattern: knows(2), typing: "skip"})
	if res.Typing.Taken || res.Typing.Asked != 0 || countPart(itemsOf(t, e, runID), PartTyping) != 0 {
		t.Fatalf("typing was taken: %+v", res.Typing)
	}
	offered := false
	for _, st := range steps {
		offered = offered || st.TypingOffer
	}
	if !offered {
		t.Fatal("typing was never offered")
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND skill = 'recall'`, user); n != 0 {
		t.Fatalf("%d recall cards", n)
	}
}

// A learner who got nothing right is offered typing, and taking it ends the run at once.
func TestTypingWithNothingToAskFinishes(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 25)
	ctx := context.Background()
	user := newLearner(t, store, "none")
	d, _ := e.loadDeck()
	runID, err := e.Start(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	state := newScriptState(script{letters: 6, pattern: always('I')})
	step, _ := e.Next(ctx, user, runID)
	for step.Item != nil {
		step, err = e.Answer(ctx, user, runID, step.Item.ID, state.answer(d, openRow(t, e, runID, step.Item.ID)))
		if err != nil {
			t.Fatal(err)
		}
	}
	if !step.TypingOffer {
		t.Fatalf("step %+v, want the typing offer", step)
	}
	done, err := e.StartTyping(ctx, user, runID)
	if err != nil || !done.Done || done.Result == nil || done.Result.Typing.Taken {
		t.Fatalf("StartTyping with nothing to ask: %+v %v", done, err)
	}
}

// The offer can be taken after a lost reply: Next repeats it, and StartTyping may be called twice.
func TestTypingOfferIsStable(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 26)
	ctx := context.Background()
	user := newLearner(t, store, "offer")
	d, _ := e.loadDeck()
	runID, _ := e.Start(ctx, user)
	state := newScriptState(script{letters: 6, pattern: always('R')})
	step, _ := e.Next(ctx, user, runID)
	var err error
	for step.Item != nil {
		step, err = e.Answer(ctx, user, runID, step.Item.ID, state.answer(d, openRow(t, e, runID, step.Item.ID)))
		if err != nil {
			t.Fatal(err)
		}
	}
	again, err := e.Next(ctx, user, runID)
	if err != nil || !again.TypingOffer {
		t.Fatalf("Next at the offer: %+v %v", again, err)
	}
	first, err := e.StartTyping(ctx, user, runID)
	if err != nil || first.Item == nil || first.Item.Part != PartTyping || !first.Item.NewPart {
		t.Fatalf("StartTyping: %+v %v", first, err)
	}
	second, err := e.StartTyping(ctx, user, runID)
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if err != nil || string(a) != string(b) {
		t.Fatalf("StartTyping twice: %s then %s (%v)", a, b, err)
	}
	// Finishing in the middle of typing is allowed and keeps what was typed.
	res, err := e.Finish(ctx, user, runID)
	if err != nil || res.Typing.Asked != 0 {
		t.Fatalf("Finish during typing: %+v %v", res, err)
	}
}

// A re-check asks the band at the learner's boundary only, in one skill, in one short sitting, and says what moved.
func TestRecheckAsksTheBoundaryBandOnly(t *testing.T) {
	for _, tc := range []struct {
		name      string
		pattern   func(string, int, int) byte
		wantMoved string
		wantLevel int
	}{
		{"knows the next band", always('R'), "up", 4},
		{"still at chance", knows(3), "same", 3},
		{"says idk", knowsOnly(3), "same", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := when
			e, store := openEngine(t, movingClock{&now}, 30)
			ctx := context.Background()
			user := newLearner(t, store, "recheck")
			_, first, _ := drive(t, e, user, script{letters: 6, pattern: knows(3), typing: "skip"})
			if first.Listening.Level != 3 || first.Reading.Level != 3 {
				t.Fatalf("initial levels %d %d", first.Listening.Level, first.Reading.Level)
			}
			now = now.Add(30 * 24 * time.Hour)
			if st, _ := e.RecheckDue(ctx, user); !st.Due {
				t.Fatalf("not due at 30 days: %+v", st)
			}

			runID, err := e.NewRecheck(ctx, user, SkillRead)
			if err != nil {
				t.Fatal(err)
			}
			if again, err := e.NewRecheck(ctx, user, SkillRead); err != nil || again != runID {
				t.Fatalf("a second NewRecheck = %d %v, want the open run %d", again, err, runID)
			}
			state := newScriptState(script{pattern: tc.pattern})
			d, _ := e.loadDeck()
			step, err := e.Next(ctx, user, runID)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for step.Item != nil {
				count++
				if step.Item.Part != PartReading || step.Item.Kind != KindPick {
					t.Fatalf("re-check item %+v, want reading picks only", step.Item)
				}
				if row := openRow(t, e, runID, step.Item.ID); row.Band != 4 {
					t.Fatalf("re-check asked band %d, want the boundary band 4", row.Band)
				}
				step, err = e.Answer(ctx, user, runID, step.Item.ID, state.answer(d, openRow(t, e, runID, step.Item.ID)))
				if err != nil {
					t.Fatal(err)
				}
			}
			if count < FirstSize || count > FirstSize+ExtraSize {
				t.Fatalf("re-check asked %d items, want %d to %d", count, FirstSize, FirstSize+ExtraSize)
			}
			if !step.Done || step.Result == nil {
				t.Fatalf("final step %+v", step)
			}
			res := *step.Result
			if res.Kind != "recheck" || res.Recheck == nil || res.Recheck.Skill != SkillRead || res.Recheck.Band != 4 ||
				res.Recheck.PreviousLevel != 3 || res.Recheck.Level != tc.wantLevel || res.Recheck.Moved != tc.wantMoved {
				t.Fatalf("recheck result %+v", res.Recheck)
			}
			if res.Reading.Level != tc.wantLevel || res.Listening.Level != 3 {
				t.Fatalf("levels after: listening %d, reading %d, want 3 and %d", res.Listening.Level, res.Reading.Level, tc.wantLevel)
			}
			// The listening result and the letters are carried over from the placement.
			if !reflect.DeepEqual(res.Listening, first.Listening) || res.LettersRight != first.LettersRight || !res.ReadingReady {
				t.Fatalf("the re-check lost the rest of the picture:\n%+v\n%+v", res.Listening, first.Listening)
			}
			// Every attempt of the re-check is a placement attempt, and nothing else was scheduled.
			if n := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ? AND source = 'placement'`, user); n < 2*(3*FirstSize)+count {
				t.Fatalf("%d placement attempts", n)
			}
			// The next due date is measured from the re-check.
			st, _ := e.RecheckDue(ctx, user)
			if !st.LastAt.Equal(now) || st.Due {
				t.Fatalf("after the re-check: %+v", st)
			}
			if latest, ok, _ := e.Latest(ctx, user); !ok || latest.RunID != runID || latest.Reading.Level != tc.wantLevel {
				t.Fatalf("Latest = %+v", latest)
			}
			// The re-check's own result reads back the same way later.
			read, err := e.Result(ctx, user, runID)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(read)
			b, _ := json.Marshal(res)
			if string(a) != string(b) {
				t.Fatalf("result read back differs:\n%s\n%s", a, b)
			}
		})
	}
}

func TestRecheckRefusals(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 31)
	ctx := context.Background()
	user := newLearner(t, store, "refuse")
	if _, err := e.NewRecheck(ctx, user, SkillListen); !errors.Is(err, ErrNoPlacement) {
		t.Fatalf("no placement: %v, want ErrNoPlacement", err)
	}
	if _, err := e.NewRecheck(ctx, user, "recall"); err == nil || errors.Is(err, ErrNoPlacement) {
		t.Fatalf("recall: %v", err)
	}
	drive(t, e, user, script{letters: 2, pattern: knows(2), typing: "skip"})
	if _, err := e.NewRecheck(ctx, user, SkillRead); !errors.Is(err, ErrNotReady) {
		t.Fatalf("reading was not tested: %v, want ErrNotReady", err)
	}
	id, err := e.NewRecheck(ctx, user, SkillListen)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.NewRecheck(ctx, user, SkillRead); !errors.Is(err, ErrNotReady) {
		// reading is refused for the first reason, which is checked before the open run
		t.Fatalf("got %v", err)
	}
	if _, err := e.Start(ctx, user); err != nil {
		t.Fatal(err)
	} else if meta, _, _ := store.GetPlacementV3Run(ctx, id); meta.Kind != "recheck" {
		t.Fatalf("meta %+v", meta)
	}
	// A learner with a v3 placement but another learner's re-check id cannot drive it.
	if _, err := e.Next(ctx, newLearner(t, store, "intruder"), id); !errors.Is(err, ErrNotYourRun) {
		t.Fatalf("another learner's re-check: %v", err)
	}
}

func TestBoundaryBand(t *testing.T) {
	for level, want := range map[int]int{-1: 1, 0: 1, 1: 2, 2: 3, 3: 4, 4: 5, 5: 5, 6: 5} {
		if got := boundaryBand(level); got != want {
			t.Errorf("boundaryBand(%d) = %d, want %d", level, got, want)
		}
	}
}

// The reading part of a run for a learner who dropped out in the middle is still readable as nothing: an open
// run has no result, and the learner's latest result is the last finished one.
func TestLatestIgnoresAnOpenRun(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 32)
	ctx := context.Background()
	user := newLearner(t, store, "latest")
	if _, ok, err := e.Latest(ctx, user); err != nil || ok {
		t.Fatalf("Latest with no run: %v %v", ok, err)
	}
	if _, err := e.Start(ctx, user); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := e.Latest(ctx, user); err != nil || ok {
		t.Fatalf("Latest with an open run: %v %v", ok, err)
	}
}
