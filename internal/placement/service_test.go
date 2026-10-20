package placement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// Tests of the v3 engine on a real store: what a run writes, what the client is told, and who may answer.

func newLearner(t *testing.T, store *db.Store, name string) int64 {
	t.Helper()
	return insertLearner(t, store, name+"@example.com", "sub-"+name)
}

// cycle answers right, wrong, and "I don't know" in turn, so a run has all three.
func cycle() func(string, int, int) byte {
	n := 0
	return func(string, int, int) byte {
		n++
		return "RWI"[n%3]
	}
}

// ac-3: after a run, cards exist only for right answers, each rated Good, and every wrong or "I don't know"
// item has an exposure and no card.
func TestSeedOnlyRightAnswersBecomeGoodCards(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 1)
	ctx := context.Background()
	user := newLearner(t, store, "seed")
	runID, res, _ := drive(t, e, user, script{letters: 6, pattern: cycle(), typing: "skip"})

	var right, wrong, idk, cards int
	for _, it := range itemsOf(t, e, runID) {
		if it.Part != string(PartListening) && it.Part != string(PartReading) {
			continue
		}
		if it.LexemeID == 0 {
			t.Fatalf("item %d has no lexeme", it.Seq)
		}
		_, hasCard, err := store.CardID(ctx, user, it.LexemeID, it.Skill)
		if err != nil {
			t.Fatal(err)
		}
		exposures := countRows(t, store, `SELECT COUNT(*) FROM exposures WHERE user_id = ? AND lexeme_id = ? AND context = 'placement_review'`, user, it.LexemeID)
		attempts, err := store.ListAttempts(ctx, user, it.LexemeID)
		if err != nil {
			t.Fatal(err)
		}
		if len(attempts) != 1 {
			t.Fatalf("item %d: %d attempts, want 1", it.Seq, len(attempts))
		}
		a := attempts[0]
		if a.Source != "placement" || a.Skill != it.Skill {
			t.Fatalf("item %d: attempt source %q skill %q", it.Seq, a.Source, a.Skill)
		}
		switch it.Outcome {
		case OutcomeRight:
			right++
			if !hasCard {
				t.Fatalf("item %d: a right answer has no card", it.Seq)
			}
			id, _, _ := store.CardID(ctx, user, it.LexemeID, it.Skill)
			logs, err := store.ReviewLogs(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if len(logs) != 1 || logs[0].Rating != int(fsrs.Good) {
				t.Fatalf("item %d: review logs %+v, want one rated Good", it.Seq, logs)
			}
			if exposures != 0 || !a.Correct || a.DontKnow {
				t.Fatalf("item %d: right answer has %d exposures, attempt %+v", it.Seq, exposures, a)
			}
			cards++
		case OutcomeWrong, OutcomeDontKnow:
			if it.Outcome == OutcomeWrong {
				wrong++
			} else {
				idk++
			}
			if hasCard {
				t.Fatalf("item %d: a %s answer has a card", it.Seq, it.Outcome)
			}
			if exposures != 1 {
				t.Fatalf("item %d: a %s answer has %d exposures, want 1", it.Seq, it.Outcome, exposures)
			}
			if a.Correct || a.DontKnow != (it.Outcome == OutcomeDontKnow) {
				t.Fatalf("item %d: attempt %+v for a %s answer", it.Seq, a, it.Outcome)
			}
		default:
			t.Fatalf("item %d has outcome %q", it.Seq, it.Outcome)
		}
	}
	if right == 0 || wrong == 0 || idk == 0 {
		t.Fatalf("the script gave %d right, %d wrong, %d idk: the test needs all three", right, wrong, idk)
	}
	if n, err := store.CountCards(ctx, user); err != nil || n != cards {
		t.Fatalf("cards = %d (%v), want %d, one for each right answer and none else", n, err, cards)
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM review_log`); n != cards {
		t.Fatalf("review_log rows = %d, want %d", n, cards)
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM exposures WHERE user_id = ?`, user); n != wrong+idk {
		t.Fatalf("exposures = %d, want %d", n, wrong+idk)
	}
	if len(res.Misses) != wrong+idk {
		t.Fatalf("misses = %d, want %d", len(res.Misses), wrong+idk)
	}
	// Part 0 writes no attempt: a letter is not a word.
	if n := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, user); n != right+wrong+idk {
		t.Fatalf("attempts = %d, want %d", n, right+wrong+idk)
	}
}

// A word the learner already has a card for is not scheduled again by a right placement answer, and a miss is
// not given a card. This is the recorder's rule for placement, seen through the engine.
func TestSeedLeavesAnExistingCardAlone(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 2)
	ctx := context.Background()
	user := newLearner(t, store, "existing")
	d, _ := e.loadDeck()
	runID, err := e.Start(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	// Answer Part 0, then give the first listening word a card with five reps before it is asked.
	state := newScriptState(script{letters: 6, pattern: always('R')})
	var step Step
	for {
		step, err = e.Next(ctx, user, runID)
		if err != nil {
			t.Fatal(err)
		}
		if step.Item.Part != PartLetters {
			break
		}
		if _, err = e.Answer(ctx, user, runID, step.Item.ID, state.answer(d, openRow(t, e, runID, step.Item.ID))); err != nil {
			t.Fatal(err)
		}
	}
	open := openRow(t, e, runID, step.Item.ID)
	word := d.byID[open.ItemID]
	lex, err := e.lexeme(ctx, word)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended)
		VALUES (?, ?, 'listen', 'review', '2026-12-01T00:00:00.000000000Z', 30, 5, 5, 0, '2026-10-01T00:00:00.000000000Z', 0)`, user, lex); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Answer(ctx, user, runID, step.Item.ID, state.answer(d, open)); err != nil {
		t.Fatal(err)
	}
	id, _, _ := store.CardID(ctx, user, lex, "listen")
	card, err := store.GetCard(ctx, id)
	if err != nil || card.Reps != 5 || card.Stability != 30 {
		t.Fatalf("existing card was rescheduled: %+v %v", card, err)
	}
	if logs, _ := store.ReviewLogs(ctx, id); len(logs) != 0 {
		t.Fatalf("existing card got %d review logs", len(logs))
	}
}

// ac-4: the reply to an answer holds no field that says whether the answer was right.
func TestNoFeedbackStepsCarryNoCorrectnessField(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 3)
	user := newLearner(t, store, "nofeedback")
	_, _, steps := drive(t, e, user, script{letters: 6, pattern: knows(3), typing: "right"})
	allowed := map[string]bool{
		"item": true, "typing_offer": true, "done": true, "result": true,
		"id": true, "part": true, "kind": true, "seq": true, "new_part": true, "audio_url": true,
		"prompt": true, "options": true, "text": true,
	}
	if len(steps) < 60 {
		t.Fatalf("only %d steps", len(steps))
	}
	for i, st := range steps {
		if st.Done {
			continue
		}
		body, err := json.Marshal(st)
		if err != nil {
			t.Fatal(err)
		}
		var generic any
		if err := json.Unmarshal(body, &generic); err != nil {
			t.Fatal(err)
		}
		walkKeys(generic, func(path, key string) {
			if !allowed[key] {
				t.Errorf("step %d: field %q at %s is not in the list of fields a reply may carry: %s", i, key, path, body)
			}
		})
	}
	last := steps[len(steps)-1]
	if !last.Done || last.Result == nil {
		t.Fatal("the last step is not the result")
	}
}

func walkKeys(v any, visit func(path, key string)) {
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				visit(path, k)
				walk(path+"."+k, child)
			}
		case []any:
			for _, child := range x {
				walk(path+"[]", child)
			}
		}
	}
	walk("$", v)
}

// The reply to a right answer, to a wrong answer, and to "I don't know" is the same bytes. If a reply differed
// by how the last answer went, a client could read the difference.
func TestNoFeedbackReplyIsTheSameWhateverTheAnswer(t *testing.T) {
	replyFor := func(kind byte) (string, string) {
		e, store := openEngine(t, fixedClock{when}, 4) // the same seed source, so the same deal
		user := newLearner(t, store, "same")
		ctx := context.Background()
		d, _ := e.loadDeck()
		runID, err := e.Start(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		state := newScriptState(script{letters: 6, pattern: always(kind)})
		step, err := e.Next(ctx, user, runID)
		if err != nil {
			t.Fatal(err)
		}
		first, _ := json.Marshal(step)
		in := state.answer(d, openRow(t, e, runID, step.Item.ID))
		if kind == 'W' {
			state.letters = 99 // a wrong letter
			in = state.answer(d, openRow(t, e, runID, step.Item.ID))
		}
		if kind == 'I' {
			in = AnswerIn{DontKnow: true}
		}
		next, err := e.Answer(ctx, user, runID, step.Item.ID, in)
		if err != nil {
			t.Fatal(err)
		}
		reply, _ := json.Marshal(next)
		return string(first), string(reply)
	}
	firstR, right := replyFor('R')
	firstW, wrong := replyFor('W')
	firstI, idk := replyFor('I')
	if firstR != firstW || firstR != firstI {
		t.Fatalf("the same seed dealt different first items:\n%s\n%s\n%s", firstR, firstW, firstI)
	}
	if right != wrong || right != idk {
		t.Fatalf("replies differ by answer:\n right: %s\n wrong: %s\n idk:   %s", right, wrong, idk)
	}

	// The same holds after the last item of the parts: the typing offer is the same for a learner who knew everything
	// and for one who knew nothing but passed the letters.
	offer := func(pattern func(string, int, int) byte) string {
		e, store := openEngine(t, fixedClock{when}, 4)
		user := newLearner(t, store, "offer")
		_, _, steps := drive(t, e, user, script{letters: 6, pattern: pattern, typing: "skip"})
		for _, st := range steps {
			if st.TypingOffer {
				body, _ := json.Marshal(st)
				return string(body)
			}
		}
		t.Fatal("no typing offer")
		return ""
	}
	if a, b := offer(always('R')), offer(always('I')); a != b || a != `{"typing_offer":true}` {
		t.Fatalf("typing offers differ: %s and %s", a, b)
	}
}

// Listening has audio and no Hebrew. Reading has pointed Hebrew, no audio, and no transliteration. Letters
// have audio. Typing shows the English meaning.
func TestNoFeedbackItemsShowOnlyWhatTheirPartAllows(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 5)
	user := newLearner(t, store, "parts")
	d, _ := e.loadDeck()
	runID, _, steps := drive(t, e, user, script{letters: 6, pattern: knows(2), typing: "right"})
	seen := map[Part]int{}
	for _, st := range steps {
		it := st.Item
		if it == nil {
			continue
		}
		seen[it.Part]++
		row := openRow(t, e, runID, it.ID)
		word := d.byID[row.ItemID]
		hasLatin := func(s string) bool {
			for _, r := range s {
				if r < 0x80 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
					return true
				}
			}
			return false
		}
		switch it.Part {
		case PartListening:
			if it.AudioURL == "" || !strings.HasPrefix(it.AudioURL, "/audio/") || it.Prompt != "" {
				t.Errorf("listening item %d: audio %q prompt %q", it.Seq, it.AudioURL, it.Prompt)
			}
			if len(it.Options) != OptionCount {
				t.Errorf("listening item %d has %d options", it.Seq, len(it.Options))
			}
		case PartReading:
			if it.AudioURL != "" || it.Prompt != word.Text || hasLatin(it.Prompt) {
				t.Errorf("reading item %d: audio %q prompt %q, want only %q", it.Seq, it.AudioURL, it.Prompt, word.Text)
			}
			if body, _ := json.Marshal(it); strings.Contains(string(body), "translit") || (len(word.Translit) > 3 && strings.Contains(string(body), word.Translit)) {
				t.Errorf("reading item %d carries the transliteration: %s", it.Seq, body)
			}
		case PartLetters:
			if it.AudioURL == "" || it.Prompt != "" || len(it.Options) != OptionCount {
				t.Errorf("letters item %d: %+v", it.Seq, it)
			}
		case PartTyping:
			if it.Kind != KindType || it.Prompt != word.Gloss || len(it.Options) != 0 || it.AudioURL != "" {
				t.Errorf("typing item %d: %+v", it.Seq, it)
			}
		}
		// Option ids are positions. An id that said which option is right would be feedback.
		for i, o := range it.Options {
			if o.ID != fmt.Sprintf("o%d", i) {
				t.Errorf("item %d: option %d has id %q", it.Seq, i, o.ID)
			}
		}
		if (it.Kind == KindPick) != (it.Part != PartTyping) {
			t.Errorf("item %d of part %s has kind %s", it.Seq, it.Part, it.Kind)
		}
	}
	for _, p := range []Part{PartLetters, PartListening, PartReading, PartTyping} {
		if seen[p] == 0 {
			t.Errorf("no %s item was dealt", p)
		}
	}
	// The first item of each part says so, and no other does.
	first := map[Part]bool{}
	for _, st := range steps {
		if st.Item == nil {
			continue
		}
		if st.Item.NewPart == first[st.Item.Part] {
			t.Errorf("item %d of part %s: new_part = %v, but the part was seen before = %v", st.Item.Seq, st.Item.Part, st.Item.NewPart, first[st.Item.Part])
		}
		first[st.Item.Part] = true
	}
}

// Words with no clip are never dealt in listening. They may still be asked in reading.
func TestAdaptiveListeningNeverDealsAWordWithoutAudio(t *testing.T) {
	e := simEngine()
	items, _ := catalog()
	silent := map[string]bool{}
	for i, it := range items {
		if i%3 == 0 {
			silent[audio.NormalizeText(it.TextNFC)] = true
		}
	}
	e.audioLookup = func(_ context.Context, text string) (string, bool, error) {
		return "clip", !silent[text], nil
	}
	d, _ := e.loadDeck()
	reading := 0
	for seed := int64(1); seed <= 100; seed++ {
		_, got := simulate(t, e, seed, script{letters: 6, pattern: always('R'), typing: "skip"})
		for _, it := range got {
			w := d.byID[it.ItemID]
			if it.Part == string(PartListening) && silent[audio.NormalizeText(w.Text)] {
				t.Fatalf("seed %d: %s has no clip and was dealt in listening", seed, w.ID)
			}
			if it.Part == string(PartReading) && silent[audio.NormalizeText(w.Text)] {
				reading++
			}
		}
	}
	if reading == 0 {
		t.Error("silent words never appear in reading: the filter is too wide")
	}
}

// ac-5: fewer than five letters right skips the reading part and records reading_ready = false.
func TestAdaptiveLettersGateSkipsReading(t *testing.T) {
	for _, tc := range []struct {
		right     int
		wantReady bool
	}{{0, false}, {4, false}, {5, true}, {6, true}} {
		t.Run(fmt.Sprintf("%d letters right", tc.right), func(t *testing.T) {
			e, store := openEngine(t, fixedClock{when}, 6)
			user := newLearner(t, store, "letters")
			runID, res, steps := drive(t, e, user, script{letters: tc.right, pattern: knows(2), typing: "right"})
			reading := countPart(itemsOf(t, e, runID), PartReading)
			typing := countPart(itemsOf(t, e, runID), PartTyping)
			if res.ReadingReady != tc.wantReady || res.Reading.Tested != tc.wantReady {
				t.Fatalf("reading_ready %v, tested %v, want %v", res.ReadingReady, res.Reading.Tested, tc.wantReady)
			}
			if res.LettersAsked != LettersAsked || res.LettersRight != tc.right {
				t.Fatalf("letters %d of %d", res.LettersRight, res.LettersAsked)
			}
			offered := false
			for _, st := range steps {
				offered = offered || st.TypingOffer
			}
			stored, err := store.GetPlacementRun(context.Background(), runID)
			if err != nil {
				t.Fatal(err)
			}
			var body struct {
				ReadingReady bool `json:"reading_ready"`
			}
			if err := json.Unmarshal([]byte(stored.Result), &body); err != nil {
				t.Fatal(err)
			}
			if body.ReadingReady != tc.wantReady {
				t.Fatalf("stored reading_ready = %v, want %v", body.ReadingReady, tc.wantReady)
			}
			if tc.wantReady {
				if reading == 0 || !offered || typing == 0 {
					t.Fatalf("reading %d, typing offered %v, typing items %d: want all of them", reading, offered, typing)
				}
				return
			}
			if reading != 0 || len(res.Reading.Bands) != 0 || res.Reading.Level != 0 || res.Reading.Estimate != 0 {
				t.Fatalf("reading was tested: %d items, %+v", reading, res.Reading)
			}
			if offered || typing != 0 {
				t.Fatalf("typing was offered (%v) or asked (%d) without the letters", offered, typing)
			}
			if !res.Listening.Tested || res.Listening.Level == 0 {
				t.Fatalf("listening was not tested: %+v", res.Listening)
			}
		})
	}
}

// ac-7: a check-in is due 28 days after the last placement or re-check, and not before.
func TestRecheckDueIsFalseAtDay27AndTrueAtDay28(t *testing.T) {
	now := when
	e, store := openEngine(t, movingClock{&now}, 7)
	ctx := context.Background()
	user := newLearner(t, store, "due")
	if st, err := e.RecheckDue(ctx, user); err != nil || st.Due || st.HasPlacement {
		t.Fatalf("before any placement: %+v %v", st, err)
	}
	_, res, _ := drive(t, e, user, script{letters: 6, pattern: knows(3), typing: "skip"})
	finished := res.FinishedAt
	if !finished.Equal(when) {
		t.Fatalf("finished at %s, want %s", finished, when)
	}
	day := 24 * time.Hour
	for _, c := range []struct {
		at        time.Duration
		due, late bool
	}{
		{0, false, false},
		{27 * day, false, false},
		{28*day - time.Nanosecond, false, false},
		{28 * day, true, false},
		{35 * day, true, false},
		{42*day - time.Nanosecond, true, false},
		{42 * day, true, true},
		{90 * day, true, true},
	} {
		now = finished.Add(c.at)
		st, err := e.RecheckDue(ctx, user)
		if err != nil || st.Due != c.due || st.Late != c.late || !st.HasPlacement {
			t.Errorf("at +%v: %+v %v, want due %v late %v", c.at, st, err, c.due, c.late)
		}
		if !st.LastAt.Equal(finished) || !st.DueAt.Equal(finished.Add(28*day)) {
			t.Errorf("at +%v: last %s due at %s", c.at, st.LastAt, st.DueAt)
		}
	}

	// Another learner's run does not make this learner due, and an unfinished run is not a placement.
	other := newLearner(t, store, "other")
	if st, err := e.RecheckDue(ctx, other); err != nil || st.Due || st.HasPlacement {
		t.Fatalf("another learner: %+v %v", st, err)
	}
	if _, err := e.Start(ctx, other); err != nil {
		t.Fatal(err)
	}
	if st, err := e.RecheckDue(ctx, other); err != nil || st.Due || st.HasPlacement {
		t.Fatalf("an unfinished run: %+v %v", st, err)
	}
}

// Only the item the server dealt to this run, and left open, can be answered.
func TestAnswerAcceptedOnlyForTheDealtItem(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 8)
	ctx := context.Background()
	user := newLearner(t, store, "owner")
	thief := newLearner(t, store, "thief")
	d, _ := e.loadDeck()
	runID, err := e.Start(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	otherRun, err := e.Start(ctx, thief)
	if err != nil {
		t.Fatal(err)
	}
	step, err := e.Next(ctx, user, runID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Next(ctx, thief, otherRun); err != nil {
		t.Fatal(err)
	}
	open := openRow(t, e, runID, step.Item.ID)
	good := newScriptState(script{letters: 6, pattern: always('R')}).answer(d, open)
	attempts := func() int { return countRows(t, store, `SELECT COUNT(*) FROM attempts`) }
	answered := func() int {
		return countRows(t, store, `SELECT COUNT(*) FROM placement_items WHERE outcome IS NOT NULL`)
	}

	refused := func(name string, want error, call func() error) {
		t.Helper()
		if err := call(); !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", name, err, want)
		}
		if attempts() != 0 || answered() != 0 {
			t.Errorf("%s: something was written (attempts %d, answered %d)", name, attempts(), answered())
		}
	}
	refused("an item never dealt", ErrWrongItem, func() error { _, err := e.Answer(ctx, user, runID, "p99", good); return err })
	refused("an empty id", ErrWrongItem, func() error { _, err := e.Answer(ctx, user, runID, "", good); return err })
	refused("a fixture id", ErrWrongItem, func() error { _, err := e.Answer(ctx, user, runID, open.ItemID, good); return err })
	refused("the next item before this one is answered", ErrWrongItem, func() error { _, err := e.Answer(ctx, user, runID, "p2", good); return err })
	refused("another learner's run", ErrNotYourRun, func() error { _, err := e.Answer(ctx, thief, runID, step.Item.ID, good); return err })
	refused("my token on the other learner's run", ErrNotYourRun, func() error { _, err := e.Answer(ctx, user, otherRun, "p1", good); return err })
	refused("a run that does not exist", ErrNotYourRun, func() error { _, err := e.Answer(ctx, user, 9999, "p1", good); return err })
	refused("Next on another learner's run", ErrNotYourRun, func() error { _, err := e.Next(ctx, thief, runID); return err })
	refused("Finish on another learner's run", ErrNotYourRun, func() error { _, err := e.Finish(ctx, thief, runID); return err })
	refused("StartTyping on another learner's run", ErrNotYourRun, func() error { _, err := e.StartTyping(ctx, thief, runID); return err })
	refused("Result on another learner's run", ErrNotYourRun, func() error { _, err := e.Result(ctx, thief, runID); return err })
	// An answer that does not fit the item.
	refused("two choices", ErrBadAnswer, func() error {
		_, err := e.Answer(ctx, user, runID, step.Item.ID, AnswerIn{Option: good.Option, DontKnow: true})
		return err
	})
	refused("no choice", ErrBadAnswer, func() error { _, err := e.Answer(ctx, user, runID, step.Item.ID, AnswerIn{}); return err })
	refused("an option not offered", ErrBadAnswer, func() error {
		_, err := e.Answer(ctx, user, runID, step.Item.ID, AnswerIn{Option: "o7"})
		return err
	})
	refused("typed text on a pick item", ErrBadAnswer, func() error {
		_, err := e.Answer(ctx, user, runID, step.Item.ID, AnswerIn{Typed: "x"})
		return err
	})
	refused("blank typed text", ErrBadAnswer, func() error {
		_, err := e.Answer(ctx, user, runID, step.Item.ID, AnswerIn{Typed: "   "})
		return err
	})

	// The right answer is accepted once, and the same item cannot be answered again.
	if _, err := e.Answer(ctx, user, runID, step.Item.ID, good); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Answer(ctx, user, runID, step.Item.ID, good); !errors.Is(err, ErrWrongItem) {
		t.Fatalf("answering the same item twice: %v, want ErrWrongItem", err)
	}
	if answered() != 1 {
		t.Fatalf("%d items answered, want 1", answered())
	}
}

// A v2 run is not a v3 run: the engine will not answer it, and a finished run takes no more answers.
func TestAnswerRefusesAV2RunAndAFinishedRun(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 9)
	ctx := context.Background()
	user := newLearner(t, store, "v2")
	v2 := New(store, nil, fixedClock{when})
	v2Run, err := v2.Start(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Answer(ctx, user, v2Run, "p1", AnswerIn{DontKnow: true}); !errors.Is(err, ErrNotYourRun) {
		t.Fatalf("a v2 run: %v, want ErrNotYourRun", err)
	}
	if _, err := e.Next(ctx, user, v2Run); !errors.Is(err, ErrNotYourRun) {
		t.Fatalf("Next on a v2 run: %v", err)
	}

	user2 := newLearner(t, store, "finished")
	runID, _, _ := drive(t, e, user2, script{letters: 6, pattern: always('R'), typing: "skip"})
	if _, err := e.Answer(ctx, user2, runID, "p1", AnswerIn{DontKnow: true}); !errors.Is(err, ErrFinished) {
		t.Fatalf("a finished run: %v, want ErrFinished", err)
	}
	if _, err := e.StartTyping(ctx, user2, runID); !errors.Is(err, ErrFinished) {
		t.Fatalf("StartTyping on a finished run: %v", err)
	}
}

// Two requests that answer the same item at once count once. Run with -race.
func TestAnswerTwiceAtOnceCountsOnce(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 10)
	ctx := context.Background()
	user := newLearner(t, store, "race")
	d, _ := e.loadDeck()
	runID, err := e.Start(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	state := newScriptState(script{letters: 6, pattern: always('R')})
	for { // skip Part 0 so the attempt path is the one raced
		step, err := e.Next(ctx, user, runID)
		if err != nil {
			t.Fatal(err)
		}
		if step.Item.Part != PartLetters {
			break
		}
		if _, err := e.Answer(ctx, user, runID, step.Item.ID, state.answer(d, openRow(t, e, runID, step.Item.ID))); err != nil {
			t.Fatal(err)
		}
	}
	step, _ := e.Next(ctx, user, runID)
	in := state.answer(d, openRow(t, e, runID, step.Item.ID))
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.Answer(ctx, user, runID, step.Item.ID, in)
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, ErrWrongItem):
			t.Errorf("unexpected error %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d of 8 concurrent answers were accepted, want exactly 1", ok)
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, user); n != 1 {
		t.Fatalf("%d attempts, want 1", n)
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, user); n != 1 {
		t.Fatalf("%d cards, want 1", n)
	}
}

// Asking for the next item is safe to repeat and to do at once: the learner is dealt one item, not several.
func TestNextIsStableAndDealsOnce(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 11)
	ctx := context.Background()
	user := newLearner(t, store, "next")
	runID, err := e.Start(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	steps := make([]Step, 6)
	errs := make([]error, 6)
	for i := range steps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			steps[i], errs[i] = e.Next(ctx, user, runID)
		}()
	}
	wg.Wait()
	want, _ := json.Marshal(steps[0])
	for i := range steps {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if got, _ := json.Marshal(steps[i]); string(got) != string(want) {
			t.Fatalf("call %d returned %s, want %s", i, got, want)
		}
	}
	if n := len(itemsOf(t, e, runID)); n != 1 {
		t.Fatalf("%d items dealt by 6 calls, want 1", n)
	}
}

// Start resumes the learner's open run, shares it between callers, closes a v2 run left open, and keeps
// finished v2 runs as they were.
func TestStartResumesAndRetiresTheV2Run(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 12)
	ctx := context.Background()
	user := newLearner(t, store, "start")

	// A finished v2 run with an answer, then an open v2 run.
	oldRun, err := store.InsertPlacementRun(ctx, user, when.Add(-48*time.Hour), 5)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ShowPlacementCard(ctx, oldRun, "boker", 1, when.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	lex, _ := store.UpsertPlacementLexeme(ctx, "boker", "morning", "noun", "boker", "m")
	if err := store.RatePlacementAnswer(ctx, oldRun, "boker", 3, lex, when.Add(-47*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishPlacement(ctx, oldRun, when.Add(-47*time.Hour), 0, "", `{"bands":[],"estimated_known_words":3}`); err != nil {
		t.Fatal(err)
	}
	beforeRun, _ := store.GetPlacementRun(ctx, oldRun)
	beforeAnswers, _ := store.ListPlacementAnswers(ctx, oldRun)
	openV2, err := store.InsertPlacementRun(ctx, user, when.Add(-time.Hour), 6)
	if err != nil {
		t.Fatal(err)
	}

	runID, err := e.Start(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if runID == openV2 || runID == oldRun {
		t.Fatalf("Start returned the v2 run %d", runID)
	}
	if run, _ := store.GetPlacementRun(ctx, openV2); run.FinishedAt.IsZero() {
		t.Fatal("the open v2 run was left open and would block the new one")
	}
	afterRun, _ := store.GetPlacementRun(ctx, oldRun)
	afterAnswers, _ := store.ListPlacementAnswers(ctx, oldRun)
	if afterRun != beforeRun || len(afterAnswers) != 1 || afterAnswers[0] != beforeAnswers[0] {
		t.Fatalf("the finished v2 run changed:\n%+v\n%+v", beforeRun, afterRun)
	}
	if meta, ok, _ := store.GetPlacementV3Run(ctx, runID); !ok || meta.Kind != db.RunInitial || meta.Phase != db.PhaseParts {
		t.Fatalf("v3 row %+v %v", meta, ok)
	}

	again, err := e.Start(ctx, user)
	if err != nil || again != runID {
		t.Fatalf("Start again = %d %v, want %d", again, err, runID)
	}

	// Callers that start at once share one run.
	racer := newLearner(t, store, "racer")
	ids := make([]int64, 6)
	errs := make([]error, 6)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids[i], errs[i] = e.Start(ctx, racer)
		}()
	}
	wg.Wait()
	for i := range ids {
		if errs[i] != nil || ids[i] != ids[0] {
			t.Fatalf("concurrent Start %d = %d %v, want %d", i, ids[i], errs[i], ids[0])
		}
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM placement_runs WHERE user_id = ? AND finished_at IS NULL`, racer); n != 1 {
		t.Fatalf("%d open runs", n)
	}
}

// The test refuses to start without the letters clips: Part 0 plays a sound.
func TestStartNeedsTheLettersAudio(t *testing.T) {
	clk := fixedClock{when}
	store, err := db.Open(context.Background(), t.TempDir()+"/hebrew.sqlite", clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedAudio(t, store, false)
	e := NewEngine(store, nil, clk)
	user := newLearner(t, store, "noaudio")
	if _, err := e.Start(context.Background(), user); !errors.Is(err, ErrNoAudio) {
		t.Fatalf("Start without letters audio: %v, want ErrNoAudio", err)
	}
	if n := countRows(t, store, `SELECT COUNT(*) FROM placement_runs`); n != 0 {
		t.Fatalf("%d runs were written", n)
	}
}

// New runs draw their seed from the injected source, and a run's deal follows its seed.
func TestSeedComesFromTheInjectedSource(t *testing.T) {
	first := func(srcSeed uint64) (int64, string) {
		e, store := openEngine(t, fixedClock{when}, srcSeed)
		user := newLearner(t, store, "src")
		runID, err := e.Start(context.Background(), user)
		if err != nil {
			t.Fatal(err)
		}
		run, _ := store.GetPlacementRun(context.Background(), runID)
		step, err := e.Next(context.Background(), user, runID)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(step)
		return run.Seed, string(body)
	}
	seedA, itemA := first(100)
	seedB, itemB := first(100)
	seedC, itemC := first(101)
	if seedA != seedB || itemA != itemB {
		t.Fatalf("the same source gave different runs: %d %d", seedA, seedB)
	}
	if seedA == seedC || itemA == itemC {
		t.Fatalf("different sources gave the same run: %d %d", seedA, seedC)
	}
	if seedA <= 0 {
		t.Fatalf("seed %d is not positive", seedA)
	}
}

// The database refuses a word asked twice, as a backstop under the planner.
func TestStoreRefusesAWordAskedTwice(t *testing.T) {
	e, store := openEngine(t, fixedClock{when}, 13)
	ctx := context.Background()
	user := newLearner(t, store, "dup")
	runID, err := e.Start(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	row := func(seq int, part string) db.PlacementItem {
		return db.PlacementItem{RunID: runID, Seq: seq, Part: part, Skill: "listen", Band: 1, ItemID: "boker", Options: `{"options":[]}`, ShownAt: when}
	}
	if err := store.InsertPlacementItem(ctx, row(1, "listening")); err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"listening", "reading"} {
		if err := store.InsertPlacementItem(ctx, row(2, part)); !errors.Is(err, db.ErrPlacementItemExists) {
			t.Fatalf("%s repeat: %v, want ErrPlacementItemExists", part, err)
		}
	}
	if err := store.InsertPlacementItem(ctx, row(1, "typing")); !errors.Is(err, db.ErrPlacementItemExists) {
		t.Fatalf("the same sequence twice: %v", err)
	}
	if err := store.InsertPlacementItem(ctx, row(2, "typing")); err != nil {
		t.Fatalf("typing asks a word again on purpose: %v", err)
	}
}
