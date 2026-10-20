package placement

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// Helpers for the v3 engine tests. Nothing here calls a TTS provider: audio is a row in audio_assets,
// or, in the simulations, a lookup that always finds a clip.

// openEngine builds an engine on a fresh temporary database with a clip for every word and letter of the
// real deck. The seed source is fixed, so the runs it deals are reproducible.
func openEngine(t *testing.T, clk clock.Clock, srcSeed uint64) (*Engine, *db.Store) {
	t.Helper()
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedAudio(t, store, true)
	return NewEngine(store, nil, clk, WithSource(rand.NewPCG(srcSeed, 7))), store
}

// seedAudio inserts a clip row for every word of the real deck, and for every letter when letters is set.
func seedAudio(t *testing.T, store *db.Store, letters bool) {
	t.Helper()
	items, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, it := range items {
		texts = append(texts, it.TextNFC)
	}
	if letters {
		ls, err := Letters()
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range ls {
			texts = append(texts, l.SyllableText)
		}
	}
	for i, text := range texts {
		if _, err := store.ExecContext(context.Background(), `
			INSERT INTO audio_assets (id, text_nfc, voice, model, bytes, created_at)
			VALUES (?, ?, 'voice', 'model', 1, '2026-01-01T00:00:00.000000000Z')`,
			fmt.Sprintf("%064x", i+1), audio.NormalizeText(text)); err != nil {
			t.Fatal(err)
		}
	}
}

func rawDB(t *testing.T, store *db.Store) *sql.DB {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func countRows(t *testing.T, store *db.Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := rawDB(t, store).QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// script says how a scripted learner answers.
//
// letters is how many of the six letters are answered right. pattern gives the answer to the nth
// item (from 0) that a skill is asked in a band: R right, W wrong, I "I don't know". typing is what
// the learner does at the offer: skip, right, wrong, offbyone, idk.
type script struct {
	letters int
	pattern func(skill string, band, nth int) byte
	typing  string
}

// knows is a learner who answers every word of the first k bands right, and the rest at chance: one
// right in four, on a fixed beat so the test does not depend on luck.
func knows(k int) func(string, int, int) byte {
	return func(_ string, band, nth int) byte {
		if band <= k || nth%4 == 0 {
			return 'R'
		}
		return 'W'
	}
}

// knowsOnly is a learner who answers the first k bands right and says "I don't know" to the rest.
func knowsOnly(k int) func(string, int, int) byte {
	return func(_ string, band, _ int) byte {
		if band <= k {
			return 'R'
		}
		return 'I'
	}
}

func always(b byte) func(string, int, int) byte {
	return func(string, int, int) byte { return b }
}

// scriptState counts what the scripted learner has been asked.
type scriptState struct {
	sc      script
	letters int
	counts  map[string]int
}

func newScriptState(sc script) *scriptState {
	return &scriptState{sc: sc, counts: map[string]int{}}
}

// answer is the scripted answer to an open item. d and the stored options are the test's view of the
// truth: the learner "knows" a word by the band the deck gives it.
func (s *scriptState) answer(d *deckV3, it db.PlacementItem) AnswerIn {
	stored, err := parseStored(it.Options)
	if err != nil {
		panic(err)
	}
	right := AnswerIn{}
	if it.CorrectIdx.Valid {
		right.Option = stored.Options[it.CorrectIdx.Int64].ID
	}
	wrong := func() AnswerIn {
		for i, o := range stored.Options {
			if !it.CorrectIdx.Valid || int64(i) != it.CorrectIdx.Int64 {
				return AnswerIn{Option: o.ID}
			}
		}
		panic("no wrong option")
	}
	switch it.Part {
	case string(PartLetters):
		if s.letters < s.sc.letters {
			s.letters++
			return right
		}
		s.letters++
		return wrong()
	case string(PartTyping):
		w := d.byID[it.ItemID]
		switch s.sc.typing {
		case "right":
			return AnswerIn{Typed: unpointed(w.Text)}
		case "offbyone":
			r := []rune(unpointed(w.Text))
			return AnswerIn{Typed: string(r[:len(r)-1])}
		case "idk":
			return AnswerIn{DontKnow: true}
		default:
			return AnswerIn{Typed: "xyz"}
		}
	}
	key := fmt.Sprintf("%s/%d", it.Skill, it.Band)
	nth := s.counts[key]
	s.counts[key]++
	switch s.sc.pattern(it.Skill, it.Band, nth) {
	case 'R':
		return right
	case 'I':
		return AnswerIn{DontKnow: true}
	default:
		return wrong()
	}
}

// drive runs a learner through a whole run on the real store and engine, and returns the run id and the
// result. It records every Step the engine returned, in order.
func drive(t *testing.T, e *Engine, userID int64, sc script) (int64, RunResult, []Step) {
	t.Helper()
	ctx := context.Background()
	d, err := e.loadDeck()
	if err != nil {
		t.Fatal(err)
	}
	runID, err := e.Start(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	state := newScriptState(sc)
	step, err := e.Next(ctx, userID, runID)
	if err != nil {
		t.Fatal(err)
	}
	steps := []Step{step}
	for i := 0; i < 400; i++ {
		switch {
		case step.Done:
			return runID, *step.Result, steps
		case step.TypingOffer:
			if sc.typing == "" || sc.typing == "skip" {
				res, err := e.Finish(ctx, userID, runID)
				if err != nil {
					t.Fatal(err)
				}
				return runID, res, steps
			}
			step, err = e.StartTyping(ctx, userID, runID)
		case step.Item != nil:
			open := openRow(t, e, runID, step.Item.ID)
			step, err = e.Answer(ctx, userID, runID, step.Item.ID, state.answer(d, open))
		default:
			t.Fatalf("empty step %+v", step)
		}
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, step)
	}
	t.Fatal("run did not finish in 400 steps")
	return 0, RunResult{}, nil
}

// openRow is the stored row of the open item, found by the id the client was given.
func openRow(t *testing.T, e *Engine, runID int64, token string) db.PlacementItem {
	t.Helper()
	items, err := e.store.ListPlacementItems(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if itemToken(it) == token {
			return it
		}
	}
	t.Fatalf("no item %s in run %d", token, runID)
	return db.PlacementItem{}
}

func itemsOf(t *testing.T, e *Engine, runID int64) []db.PlacementItem {
	t.Helper()
	items, err := e.store.ListPlacementItems(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	return items
}

// simulate runs a scripted learner through the engine's planner with no store: the items live in
// memory. It exercises the real dealing, options, and scoring code, and is fast enough for
// thousands of runs. Typing is taken or skipped as the script says.
func simulate(t testing.TB, e *Engine, seed int64, sc script) (RunResult, []db.PlacementItem) {
	t.Helper()
	d, err := e.loadDeck()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rc := runCtx{
		run:  db.PlacementRun{ID: 1, UserID: 1, Seed: seed, StartedAt: when},
		meta: db.PlacementV3Run{RunID: 1, Kind: db.RunInitial, Phase: db.PhaseParts},
	}
	state := newScriptState(sc)
	for i := 0; i < 500; i++ {
		act, err := e.plan(ctx, d, &rc)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		switch act.kind {
		case actDeal:
			rc.items = append(rc.items, act.item)
		case actOpen:
			for j := range rc.items {
				if rc.items[j].Seq != act.item.Seq {
					continue
				}
				stored, _ := parseStored(rc.items[j].Options)
				outcome, picked, _, err := judge(d, rc.items[j], stored, state.answer(d, rc.items[j]))
				if err != nil {
					t.Fatalf("seed %d: %v", seed, err)
				}
				rc.items[j].Outcome, rc.items[j].Picked, rc.items[j].AnsweredAt = outcome, picked, when
			}
		case actTypingOffer:
			if sc.typing == "" || sc.typing == "skip" {
				return finishSim(t, e, d, rc)
			}
			rc.meta.Phase = db.PhaseTyping
		default:
			return finishSim(t, e, d, rc)
		}
	}
	t.Fatalf("seed %d: simulation did not finish", seed)
	return RunResult{}, nil
}

func finishSim(t testing.TB, e *Engine, d *deckV3, rc runCtx) (RunResult, []db.PlacementItem) {
	t.Helper()
	res, err := e.computeResult(context.Background(), d, rc)
	if err != nil {
		t.Fatal(err)
	}
	return res, rc.items
}

// simEngine is an engine with no store and a clip for every text.
func simEngine() *Engine {
	return &Engine{
		clock: fixedClock{when},
		audioLookup: func(_ context.Context, text string) (string, bool, error) {
			return "clip-" + text, true, nil
		},
	}
}

// answeredIn counts the answered items of a skill in a band.
func answeredIn(items []db.PlacementItem, skill string, band int) int {
	n := 0
	for _, it := range items {
		if it.Skill == skill && it.Band == band && it.Outcome != "" {
			n++
		}
	}
	return n
}
