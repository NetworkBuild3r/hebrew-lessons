package placement

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// The activity names a placement attempt stores. They are the names internal/activity uses for the
// same questions, which that package owns and this one cannot import.
const (
	activityListenPick = "listen_pick"
	activityReadPick   = "read_pick"
	activityTypeIt     = "type_it"
)

// Engine is the v3 placement test. It holds no run state: every call reads the run's items from the
// store, so any process can serve the next request, and a restart loses nothing.
//
// Every method takes the signed-in learner's id, from the session and never from the request, and
// treats a run that is not that learner's as not found.
type Engine struct {
	store *db.Store
	rec   *evidence.Recorder
	clock clock.Clock

	srcMu sync.Mutex
	src   rand.Source

	// fixture and letters replace the embedded deck in tests.
	fixture []fixtureItem
	letters []Letter
	// audioLookup replaces the audio_assets read in tests that run without a store.
	audioLookup func(ctx context.Context, text string) (string, bool, error)

	deckOnce sync.Once
	deck     *deckV3
	deckErr  error
}

// EngineOption changes how NewEngine builds an Engine.
type EngineOption func(*Engine)

// WithSource sets where new runs draw their seed. A run's seed fixes its whole deal, so a source with
// a known state gives a reproducible run. The default is a PCG seeded from the clock.
func WithSource(src rand.Source) EngineOption {
	return func(e *Engine) {
		if src != nil {
			e.src = src
		}
	}
}

// NewEngine builds the engine. A nil clk uses the clock the store was opened with. A nil rec is built
// on that clock.
func NewEngine(store *db.Store, rec *evidence.Recorder, clk clock.Clock, opts ...EngineOption) *Engine {
	if clk == nil && store != nil {
		clk = store.Clock()
	}
	if rec == nil {
		rec = evidence.New(store, nil, clk)
	}
	e := &Engine{store: store, rec: rec, clock: clk}
	if clk != nil {
		e.src = rand.NewPCG(uint64(clk.Now().UnixNano()), 0x9e3779b97f4a7c15)
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Engine) ready() error {
	if e == nil || e.store == nil || e.rec == nil {
		return errors.New("placement: engine is not configured")
	}
	if e.clock == nil || e.src == nil {
		return errors.New("placement: engine has no clock")
	}
	return nil
}

func (e *Engine) loadDeck() (*deckV3, error) {
	e.deckOnce.Do(func() {
		items, letters := e.fixture, e.letters
		if items == nil {
			var err error
			if items, err = catalog(); err != nil {
				e.deckErr = err
				return
			}
		}
		if letters == nil {
			var err error
			if letters, err = Letters(); err != nil {
				e.deckErr = err
				return
			}
		}
		e.deck, e.deckErr = newDeckV3(items, letters)
	})
	return e.deck, e.deckErr
}

func (e *Engine) newSeed() int64 {
	e.srcMu.Lock()
	defer e.srcMu.Unlock()
	seed := int64(e.src.Uint64() >> 1)
	if seed == 0 {
		seed = 1
	}
	return seed
}

// audioID is the clip for a pointed text, if the store has one.
func (e *Engine) audioID(ctx context.Context, text string) (string, bool, error) {
	if e.audioLookup != nil {
		return e.audioLookup(ctx, text)
	}
	return e.store.AudioIDByText(ctx, audio.NormalizeText(text))
}

// Start opens a v3 placement for the learner, or returns the one they left unfinished.
//
// A v2 run that was left open is closed unscored first: the v2 deck is replaced, and its open run
// would otherwise block the new one. Finished v2 runs are kept and stay readable. Every letters clip must
// exist, or Start returns ErrNoAudio. The listening part drops, word by word, any word with no clip.
func (e *Engine) Start(ctx context.Context, userID int64) (int64, error) {
	if err := e.ready(); err != nil {
		return 0, err
	}
	d, err := e.loadDeck()
	if err != nil {
		return 0, err
	}
	if id, ok, err := e.resumeOpen(ctx, userID); err != nil || ok {
		return id, err
	}
	for _, l := range d.letters {
		if _, ok, err := e.audioID(ctx, l.SyllableText); err != nil {
			return 0, err
		} else if !ok {
			return 0, fmt.Errorf("%w: letter %s", ErrNoAudio, l.ID)
		}
	}
	return e.open(ctx, userID, db.RunInitial, "", 0)
}

// open starts a run, sharing the winner's run when two callers start at once.
func (e *Engine) open(ctx context.Context, userID int64, kind, skill string, band int) (int64, error) {
	id, err := e.store.StartPlacementV3(ctx, userID, e.clock.Now(), e.newSeed(), kind, skill, band)
	if errors.Is(err, db.ErrOpenPlacementRun) {
		if id, ok, err := e.resumeOpen(ctx, userID); err != nil || ok {
			return id, err
		}
		return 0, errors.New("placement: open run was not stored")
	}
	return id, err
}

// resumeOpen returns the learner's open v3 run. An open v2 run is closed unscored and ok is false.
func (e *Engine) resumeOpen(ctx context.Context, userID int64) (int64, bool, error) {
	open, ok, err := e.store.OpenPlacementRun(ctx, userID)
	if err != nil || !ok {
		return 0, false, err
	}
	_, isV3, err := e.store.GetPlacementV3Run(ctx, open.ID)
	if err != nil {
		return 0, false, err
	}
	if isV3 {
		return open.ID, true, nil
	}
	if err := e.store.FinishPlacement(ctx, open.ID, e.clock.Now(), -1, "", `{"bands":[],"estimated_known_words":0}`); err != nil {
		return 0, false, err
	}
	return 0, false, nil
}

// runCtx is a run as loaded for one call.
type runCtx struct {
	run   db.PlacementRun
	meta  db.PlacementV3Run
	items []db.PlacementItem
}

// load reads a run and checks that it is the learner's and a v3 run. A finished run is returned only
// when allowFinished is set, and is ErrFinished otherwise.
func (e *Engine) load(ctx context.Context, userID, runID int64, allowFinished bool) (runCtx, error) {
	run, err := e.store.GetPlacementRun(ctx, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return runCtx{}, ErrNotYourRun
	}
	if err != nil {
		return runCtx{}, err
	}
	if run.UserID != userID {
		return runCtx{}, ErrNotYourRun
	}
	meta, ok, err := e.store.GetPlacementV3Run(ctx, runID)
	if err != nil {
		return runCtx{}, err
	}
	if !ok {
		return runCtx{}, ErrNotYourRun
	}
	if !run.FinishedAt.IsZero() && !allowFinished {
		return runCtx{}, ErrFinished
	}
	items, err := e.store.ListPlacementItems(ctx, runID)
	if err != nil {
		return runCtx{}, err
	}
	return runCtx{run: run, meta: meta, items: items}, nil
}

// Next returns the item the learner should answer now: the one already dealt and unanswered, or a new one.
// It is safe to call again after a lost reply. On a finished run it returns the result.
func (e *Engine) Next(ctx context.Context, userID, runID int64) (Step, error) {
	if err := e.ready(); err != nil {
		return Step{}, err
	}
	d, err := e.loadDeck()
	if err != nil {
		return Step{}, err
	}
	rc, err := e.load(ctx, userID, runID, true)
	if err != nil {
		return Step{}, err
	}
	if !rc.run.FinishedAt.IsZero() {
		res, err := e.resultFor(ctx, d, rc)
		if err != nil {
			return Step{}, err
		}
		return Step{Done: true, Result: &res}, nil
	}
	return e.advance(ctx, d, rc)
}

// Answer checks the answer against the item the server dealt, stores it, and returns the next step.
//
// itemID must be the id of the run's open item, or Answer returns ErrWrongItem and writes nothing: an
// id from another run, an item already answered, and one the server never dealt are all refused.
// The reply never says whether the answer was right.
//
// A word answer goes through evidence.Recorder with source placement, so a right one seeds that skill's
// card as Good and a wrong one or "I don't know" seeds nothing. The wrong word's exposure is written when
// the run finishes, because that is when the learner sees it. A letters item names no word and writes no
// attempt.
func (e *Engine) Answer(ctx context.Context, userID, runID int64, itemID string, in AnswerIn) (Step, error) {
	if err := e.ready(); err != nil {
		return Step{}, err
	}
	d, err := e.loadDeck()
	if err != nil {
		return Step{}, err
	}
	rc, err := e.load(ctx, userID, runID, false)
	if err != nil {
		return Step{}, err
	}
	open := openItem(rc.items)
	if open == nil || itemToken(*open) != itemID {
		return Step{}, ErrWrongItem
	}
	stored, err := parseStored(open.Options)
	if err != nil {
		return Step{}, err
	}
	outcome, picked, chosen, err := judge(d, *open, stored, in)
	if err != nil {
		return Step{}, err
	}

	var lexemeID, chosenID int64
	var word Word
	if open.Part != string(PartLetters) {
		word = d.byID[open.ItemID]
		if lexemeID, err = e.lexeme(ctx, word); err != nil {
			return Step{}, err
		}
		if chosen != "" {
			if chosenID, err = e.lexeme(ctx, d.byID[chosen]); err != nil {
				return Step{}, err
			}
		}
	}
	now := e.clock.Now()
	latency := now.Sub(open.ShownAt)
	if latency < 0 {
		latency = 0
	}
	claimed, err := e.store.ClaimPlacementItem(ctx, runID, open.Seq, outcome, picked, lexemeID, now, latency)
	if err != nil {
		return Step{}, err
	}
	if !claimed {
		return Step{}, ErrWrongItem
	}
	if open.Part != string(PartLetters) {
		a := evidence.Attempt{
			UserID:         userID,
			LexemeID:       lexemeID,
			Skill:          open.Skill,
			Activity:       activityFor(open.Part),
			Source:         evidence.SourcePlacement,
			Prompt:         promptFor(open.Part, word),
			Response:       picked,
			Correct:        outcome == OutcomeRight,
			DontKnow:       outcome == OutcomeDontKnow,
			Latency:        latency,
			ChosenLexemeID: chosenID,
		}
		if _, err := e.rec.Attempt(ctx, a); err != nil {
			// The attempt is the record of the answer. Without it the answer is not counted, so the item is reopened.
			if rerr := e.store.ReleasePlacementItem(ctx, runID, open.Seq); rerr != nil {
				err = errors.Join(err, rerr)
			}
			return Step{}, err
		}
	}
	rc.items, err = e.store.ListPlacementItems(ctx, runID)
	if err != nil {
		return Step{}, err
	}
	return e.advance(ctx, d, rc)
}

// judge decides the outcome of an answer from the item the server stored, and returns what was picked
// or typed and the fixture id of the word a wrong pick names. It refuses an answer that does not fit.
func judge(d *deckV3, it db.PlacementItem, stored storedItem, in AnswerIn) (outcome, picked, chosen string, err error) {
	typed := strings.TrimSpace(in.Typed)
	set := 0
	for _, on := range []bool{in.Option != "", typed != "", in.DontKnow} {
		if on {
			set++
		}
	}
	if set != 1 {
		return "", "", "", ErrBadAnswer
	}
	if in.DontKnow {
		return OutcomeDontKnow, "", "", nil
	}
	if it.Part == string(PartTyping) {
		if typed == "" {
			return "", "", "", ErrBadAnswer
		}
		w := d.byID[it.ItemID]
		if fsrs.GradeTyped(w.Text, typed) == fsrs.Good {
			return OutcomeRight, typed, "", nil
		}
		return OutcomeWrong, typed, "", nil
	}
	if in.Option == "" {
		return "", "", "", ErrBadAnswer
	}
	for i, o := range stored.Options {
		if o.ID != in.Option {
			continue
		}
		if it.CorrectIdx.Valid && int64(i) == it.CorrectIdx.Int64 {
			return OutcomeRight, o.Text, "", nil
		}
		return OutcomeWrong, o.Text, o.Item, nil
	}
	return "", "", "", ErrBadAnswer
}

func activityFor(part string) string {
	switch part {
	case string(PartListening):
		return activityListenPick
	case string(PartReading):
		return activityReadPick
	default:
		return activityTypeIt
	}
}

// promptFor is the question as text, for attempts.prompt: the Hebrew of a listening or reading item,
// and the English meaning of a typing item.
func promptFor(part string, w Word) string {
	if part == string(PartTyping) {
		return w.Gloss
	}
	return w.Text
}

// lexeme makes sure the word has a lexicon row and returns its id. A row that exists keeps its
// verified flag. This is the same upsert the v2 deck made when a word was rated.
func (e *Engine) lexeme(ctx context.Context, w Word) (int64, error) {
	if w.ID == "" {
		return 0, errors.New("placement: item names a word that is not in the deck")
	}
	return e.store.UpsertPlacementLexeme(ctx, w.Text, w.Gloss, w.POS, w.Translit, w.Gender)
}

// StartTyping opens the optional typing part. It is valid once the reading part is over, when Step
// has TypingOffer set. With no word to ask, the run finishes and the step is Done.
func (e *Engine) StartTyping(ctx context.Context, userID, runID int64) (Step, error) {
	if err := e.ready(); err != nil {
		return Step{}, err
	}
	d, err := e.loadDeck()
	if err != nil {
		return Step{}, err
	}
	rc, err := e.load(ctx, userID, runID, false)
	if err != nil {
		return Step{}, err
	}
	if rc.meta.Kind != db.RunInitial {
		return Step{}, ErrNotReady
	}
	act, err := e.plan(ctx, d, &rc)
	if err != nil {
		return Step{}, err
	}
	switch {
	case rc.meta.Phase == db.PhaseTyping:
	case act.kind == actTypingOffer || rc.meta.Phase == db.PhaseTypingOffer:
		if _, err := e.store.SetPlacementV3Phase(ctx, runID, db.PhaseTyping); err != nil {
			return Step{}, err
		}
		rc.meta.Phase = db.PhaseTyping
	default:
		return Step{}, ErrNotReady
	}
	return e.advance(ctx, d, rc)
}

// Finish ends the run and returns its result. It is the way to skip the typing part, and it stops the
// typing part early. Before the parts are over it returns ErrNotReady. On a finished run it returns the
// same result again and writes nothing.
func (e *Engine) Finish(ctx context.Context, userID, runID int64) (RunResult, error) {
	if err := e.ready(); err != nil {
		return RunResult{}, err
	}
	d, err := e.loadDeck()
	if err != nil {
		return RunResult{}, err
	}
	rc, err := e.load(ctx, userID, runID, true)
	if err != nil {
		return RunResult{}, err
	}
	if !rc.run.FinishedAt.IsZero() {
		return e.resultFor(ctx, d, rc)
	}
	act, err := e.plan(ctx, d, &rc)
	if err != nil {
		return RunResult{}, err
	}
	if act.kind != actFinish && act.kind != actTypingOffer && rc.meta.Phase == db.PhaseParts {
		return RunResult{}, ErrNotReady
	}
	return e.finish(ctx, d, rc)
}

// Result returns the result of a finished run. An unfinished run is ErrNotReady.
func (e *Engine) Result(ctx context.Context, userID, runID int64) (RunResult, error) {
	if err := e.ready(); err != nil {
		return RunResult{}, err
	}
	d, err := e.loadDeck()
	if err != nil {
		return RunResult{}, err
	}
	rc, err := e.load(ctx, userID, runID, true)
	if err != nil {
		return RunResult{}, err
	}
	if rc.run.FinishedAt.IsZero() {
		return RunResult{}, ErrNotReady
	}
	return e.resultFor(ctx, d, rc)
}

// Latest returns the learner's newest finished v3 result, an initial run or a re-check. A re-check's
// result is the whole current picture, so this is where a screen reads the learner's levels.
// ok is false when the learner has no finished v3 run.
func (e *Engine) Latest(ctx context.Context, userID int64) (RunResult, bool, error) {
	if err := e.ready(); err != nil {
		return RunResult{}, false, err
	}
	d, err := e.loadDeck()
	if err != nil {
		return RunResult{}, false, err
	}
	run, meta, ok, err := e.store.LatestFinishedPlacementV3(ctx, userID, 0)
	if err != nil || !ok {
		return RunResult{}, false, err
	}
	items, err := e.store.ListPlacementItems(ctx, run.ID)
	if err != nil {
		return RunResult{}, false, err
	}
	res, err := e.resultFor(ctx, d, runCtx{run: run, meta: meta, items: items})
	return res, err == nil, err
}

// ---- planning ----

type actKind int

const (
	actOpen actKind = iota
	actDeal
	actTypingOffer
	actFinish
)

// action is what the run needs next.
type action struct {
	kind actKind
	item db.PlacementItem // the open item (actOpen) or the item to insert (actDeal)
}

// advance carries the run to its next step and returns it.
func (e *Engine) advance(ctx context.Context, d *deckV3, rc runCtx) (Step, error) {
	var err error
	for try := 0; try < 3; try++ {
		var act action
		if act, err = e.plan(ctx, d, &rc); err != nil {
			return Step{}, err
		}
		switch act.kind {
		case actOpen:
			it, err := render(d, rc.items, act.item)
			return Step{Item: &it}, err
		case actDeal:
			err := e.store.InsertPlacementItem(ctx, act.item)
			if errors.Is(err, db.ErrPlacementItemExists) {
				// Another request dealt first. Read its item instead of dealing a second one.
				if rc.items, err = e.store.ListPlacementItems(ctx, rc.run.ID); err != nil {
					return Step{}, err
				}
				continue
			}
			if err != nil {
				return Step{}, err
			}
			rc.items = append(rc.items, act.item)
			it, err := render(d, rc.items, act.item)
			return Step{Item: &it}, err
		case actTypingOffer:
			if rc.meta.Phase == db.PhaseParts {
				if _, err := e.store.SetPlacementV3Phase(ctx, rc.run.ID, db.PhaseTypingOffer); err != nil {
					return Step{}, err
				}
			}
			return Step{TypingOffer: true}, nil
		default:
			res, err := e.finish(ctx, d, rc)
			if err != nil {
				return Step{}, err
			}
			return Step{Done: true, Result: &res}, nil
		}
	}
	return Step{}, errors.New("placement: could not deal an item")
}

// plan decides what the run needs next from its stored items alone.
func (e *Engine) plan(ctx context.Context, d *deckV3, rc *runCtx) (action, error) {
	if open := openItem(rc.items); open != nil {
		return action{kind: actOpen, item: *open}, nil
	}
	seed := rc.run.Seed

	if rc.meta.Kind == db.RunRecheck {
		part, skill := PartListening, SkillListen
		if rc.meta.RecheckSkill == SkillRead {
			part, skill = PartReading, SkillRead
		}
		st := replay(answersOf(rc.items, part), rc.meta.RecheckBand, true)
		if !st.Done {
			if it, ok, err := e.dealWord(ctx, d, rc, part, skill, st.Band); err != nil || ok {
				return action{kind: actDeal, item: it}, err
			}
		}
		return action{kind: actFinish}, nil
	}

	// Part 0.
	letters := d.letterOrder(seed)
	if n := countPart(rc.items, PartLetters); n < min(LettersAsked, len(letters)) {
		it, err := e.dealLetter(ctx, rc, letters[n], n+1)
		return action{kind: actDeal, item: it}, err
	}
	ready := readingReady(rc.items, len(letters))

	// Parts 1 and 2.
	parts := []struct {
		part  Part
		skill string
	}{{PartListening, SkillListen}, {PartReading, SkillRead}}
	for _, p := range parts {
		if p.part == PartReading && !ready {
			break
		}
		st := replay(answersOf(rc.items, p.part), 1, false)
		if st.Done {
			continue
		}
		it, ok, err := e.dealWord(ctx, d, rc, p.part, p.skill, st.Band)
		if err != nil {
			return action{}, err
		}
		if ok {
			return action{kind: actDeal, item: it}, nil
		}
		// No word is left to deal in the band: the skill ends here.
	}

	// Part 3.
	if !ready {
		return action{kind: actFinish}, nil
	}
	switch rc.meta.Phase {
	case db.PhaseParts, db.PhaseTypingOffer:
		return action{kind: actTypingOffer}, nil
	}
	typed := map[string]bool{}
	for _, it := range rc.items {
		if it.Part == string(PartTyping) {
			typed[it.ItemID] = true
		}
	}
	if len(typed) >= TypingMax {
		return action{kind: actFinish}, nil
	}
	for _, id := range rightWords(rc.items) {
		if typed[id] {
			continue
		}
		w := d.byID[id]
		return action{kind: actDeal, item: db.PlacementItem{
			RunID: rc.run.ID, Seq: len(rc.items) + 1, Part: string(PartTyping), Skill: SkillRecall,
			Band: w.Band, ItemID: w.ID, Options: `{"options":[]}`, ShownAt: e.clock.Now(),
		}}, nil
	}
	return action{kind: actFinish}, nil
}

// dealLetter builds the nth Part 0 item.
func (e *Engine) dealLetter(ctx context.Context, rc *runCtx, l Letter, n int) (db.PlacementItem, error) {
	clip, ok, err := e.audioID(ctx, l.SyllableText)
	if err != nil {
		return db.PlacementItem{}, err
	}
	if !ok {
		return db.PlacementItem{}, fmt.Errorf("%w: letter %s", ErrNoAudio, l.ID)
	}
	seq := len(rc.items) + 1
	opts, correct := letterOptions(l, itemRNG(rc.run.Seed, seq))
	body, err := json.Marshal(storedItem{Audio: clip, Options: toStored(opts, nil)})
	if err != nil {
		return db.PlacementItem{}, err
	}
	return db.PlacementItem{
		RunID: rc.run.ID, Seq: seq, Part: string(PartLetters), ItemID: l.ID, Options: string(body),
		CorrectIdx: sql.NullInt64{Int64: int64(correct), Valid: true}, ShownAt: e.clock.Now(),
	}, nil
}

// dealWord builds the next pick item of a skill in a band. It returns false when the band has no
// word left that this run has not asked, and that has a clip when the part is listening.
func (e *Engine) dealWord(ctx context.Context, d *deckV3, rc *runCtx, part Part, skill string, band int) (db.PlacementItem, bool, error) {
	asked := map[string]bool{}
	for _, it := range rc.items {
		if it.Part == string(PartListening) || it.Part == string(PartReading) {
			asked[it.ItemID] = true
		}
	}
	var clip string
	var ferr error
	var fits func(Word) bool
	if part == PartListening {
		fits = func(w Word) bool {
			id, ok, err := e.audioID(ctx, w.Text)
			if err != nil {
				ferr = err
				return false
			}
			if ok {
				clip = id
			}
			return ok
		}
	}
	w, ok := d.next(rc.run.Seed, band, asked, fits)
	if ferr != nil {
		return db.PlacementItem{}, false, ferr
	}
	if !ok {
		return db.PlacementItem{}, false, nil
	}
	seq := len(rc.items) + 1
	opts, words, correct, err := d.pickOptions(w, itemRNG(rc.run.Seed, seq))
	if err != nil {
		if errors.Is(err, errNoDistractors) {
			err = fmt.Errorf("%w: %v", ErrDeckShort, err)
		}
		return db.PlacementItem{}, false, err
	}
	si := storedItem{Options: toStored(opts, words)}
	if part == PartListening {
		si.Audio = clip
	}
	body, err := json.Marshal(si)
	if err != nil {
		return db.PlacementItem{}, false, err
	}
	return db.PlacementItem{
		RunID: rc.run.ID, Seq: seq, Part: string(part), Skill: skill, Band: w.Band, ItemID: w.ID,
		Options: string(body), CorrectIdx: sql.NullInt64{Int64: int64(correct), Valid: true}, ShownAt: e.clock.Now(),
	}, true, nil
}

// storedItem is placement_items.options. Item on an option is the fixture id of the word behind it.
type storedItem struct {
	Audio   string         `json:"audio,omitempty"`
	Options []storedOption `json:"options"`
}

type storedOption struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Item string `json:"item,omitempty"`
}

func toStored(opts []Option, words []Word) []storedOption {
	out := make([]storedOption, len(opts))
	for i, o := range opts {
		out[i] = storedOption{ID: o.ID, Text: o.Text}
		if i < len(words) {
			out[i].Item = words[i].ID
		}
	}
	return out
}

func parseStored(body string) (storedItem, error) {
	var si storedItem
	if err := json.Unmarshal([]byte(body), &si); err != nil {
		return storedItem{}, fmt.Errorf("placement: stored item: %w", err)
	}
	return si, nil
}

// itemToken is the id the client sees for a stored item. It carries the order the item was dealt in
// and nothing about the word or its options.
func itemToken(it db.PlacementItem) string { return fmt.Sprintf("p%d", it.Seq) }

// render is the client's view of a stored item. It leaves out the band, the right option, and the
// word behind each option.
func render(d *deckV3, all []db.PlacementItem, it db.PlacementItem) (Item, error) {
	si, err := parseStored(it.Options)
	if err != nil {
		return Item{}, err
	}
	out := Item{ID: itemToken(it), Part: Part(it.Part), Kind: KindPick, Seq: it.Seq}
	for _, o := range all {
		if o.Part == it.Part {
			out.NewPart = o.Seq == it.Seq
			break
		}
	}
	if si.Audio != "" {
		out.AudioURL = "/audio/" + si.Audio + ".mp3"
	}
	switch out.Part {
	case PartReading:
		out.Prompt = d.byID[it.ItemID].Text
	case PartTyping:
		out.Kind = KindType
		out.Prompt = d.byID[it.ItemID].Gloss
	}
	if out.Kind == KindPick {
		for _, o := range si.Options {
			out.Options = append(out.Options, Option{ID: o.ID, Text: o.Text})
		}
	}
	return out, nil
}

// ---- reading the stored items ----

func openItem(items []db.PlacementItem) *db.PlacementItem {
	for i := range items {
		if items[i].Outcome == "" {
			return &items[i]
		}
	}
	return nil
}

func countPart(items []db.PlacementItem, part Part) int {
	n := 0
	for _, it := range items {
		if it.Part == string(part) {
			n++
		}
	}
	return n
}

// answersOf is the answered items of a part, in order, as the adaptive rule reads them.
func answersOf(items []db.PlacementItem, part Part) []bandOutcome {
	var out []bandOutcome
	for _, it := range items {
		if it.Part == string(part) && it.Outcome != "" {
			out = append(out, bandOutcome{Band: it.Band, Outcome: it.Outcome})
		}
	}
	return out
}

// readingReady reports that enough letters were right for reading to be tested. With no letters in
// the deck there is no Part 0 and reading is ready.
func readingReady(items []db.PlacementItem, deckLetters int) bool {
	right := 0
	for _, it := range items {
		if it.Part == string(PartLetters) && it.Outcome == OutcomeRight {
			right++
		}
	}
	return right >= min(LettersPass, deckLetters, LettersAsked)
}

// rightWords is the fixture ids of words answered right in listening or reading, in the order asked.
func rightWords(items []db.PlacementItem) []string {
	var out []string
	seen := map[string]bool{}
	for _, it := range items {
		if (it.Part == string(PartListening) || it.Part == string(PartReading)) && it.Outcome == OutcomeRight && !seen[it.ItemID] {
			seen[it.ItemID] = true
			out = append(out, it.ItemID)
		}
	}
	return out
}

// ---- results ----

// compat is the shape db.LatestPlacement reads from placement_runs.result. It carries the v2 keys
// (bands, estimated_known_words) so that the learner model and the MCP read still work, and adds v3
// keys. Each band is the deck's band number, 0 to 4, pooled over the skills that were tested: rated is
// asked, good is right, again is wrong, and good_plus_rate is the known share, corrected for guessing.
type compat struct {
	V                   int            `json:"v"`
	Kind                string         `json:"kind"`
	Bands               []db.BandScore `json:"bands"`
	EstimatedKnownWords int            `json:"estimated_known_words"`
	ReadingReady        bool           `json:"reading_ready"`
	Listening           compatSkill    `json:"listening"`
	Reading             compatSkill    `json:"reading"`
	Typing              TypingResult   `json:"typing"`
	Recheck             *RecheckMoved  `json:"recheck,omitempty"`
	LettersRight        int            `json:"letters_right"`
}

type compatSkill struct {
	Tested   bool `json:"tested"`
	Level    int  `json:"level"`
	Estimate int  `json:"estimate"`
}

func toCompat(r RunResult) compat {
	pooled := [BandCount]Tally{}
	for _, s := range []SkillResult{r.Listening, r.Reading} {
		if !s.Tested {
			continue
		}
		for _, b := range s.Bands {
			t := &pooled[b.Band-1]
			t.Asked += b.Asked
			t.Right += b.Right
			t.Wrong += b.Wrong
			t.DontKnow += b.DontKnow
		}
	}
	c := compat{
		V: 3, Kind: r.Kind, ReadingReady: r.ReadingReady, Typing: r.Typing, Recheck: r.Recheck,
		LettersRight: r.LettersRight, Bands: []db.BandScore{},
		Listening: compatSkill{r.Listening.Tested, r.Listening.Level, r.Listening.Estimate},
		Reading:   compatSkill{r.Reading.Tested, r.Reading.Level, r.Reading.Estimate},
	}
	for i, t := range pooled {
		if t.Asked == 0 {
			continue
		}
		c.Bands = append(c.Bands, db.BandScore{
			Band: i, Rated: t.Asked, Good: t.Right, Again: t.Wrong, DontKnow: t.DontKnow, GoodPlusRate: Share(t),
		})
	}
	c.EstimatedKnownWords = r.Listening.Estimate
	if r.Reading.Tested {
		c.EstimatedKnownWords = r.Reading.Estimate
	}
	return c
}

// legacyLevel is the number stored in placement_runs.level, which means the highest deck band
// completed, 0 to 4, or −1 for none. It is the reading level when reading was tested, and the listening
// level otherwise, less one because this package numbers bands from 1.
func legacyLevel(r RunResult) int {
	level := r.Listening.Level
	if r.Reading.Tested {
		level = r.Reading.Level
	}
	return level - 1
}

// finish scores the run, writes the review exposures, and marks the run finished. Two callers that
// finish together get the same stored result: the one that loses reads what the winner stored.
func (e *Engine) finish(ctx context.Context, d *deckV3, rc runCtx) (RunResult, error) {
	res, err := e.computeResult(ctx, d, rc)
	if err != nil {
		return RunResult{}, err
	}
	var ids []int64
	for _, m := range res.Misses {
		ids = append(ids, m.LexemeID)
	}
	if len(ids) > 0 {
		if err := e.rec.Expose(ctx, rc.run.UserID, ids, evidence.ContextPlacementReview, 0); err != nil {
			return RunResult{}, err
		}
	}
	now := e.clock.Now()
	body, err := json.Marshal(toCompat(res))
	if err != nil {
		return RunResult{}, err
	}
	err = e.store.FinishPlacement(ctx, rc.run.ID, now, legacyLevel(res), "", string(body))
	if err != nil {
		run, gerr := e.store.GetPlacementRun(ctx, rc.run.ID)
		if gerr == nil && !run.FinishedAt.IsZero() {
			rc.run = run
			return e.resultFor(ctx, d, rc)
		}
		return RunResult{}, err
	}
	rc.run.FinishedAt = now
	return e.resultFor(ctx, d, rc)
}

// resultFor is the result of a finished run, worked out again from its items and the units stored now.
func (e *Engine) resultFor(ctx context.Context, d *deckV3, rc runCtx) (RunResult, error) {
	res, err := e.computeResult(ctx, d, rc)
	if err != nil {
		return RunResult{}, err
	}
	res.FinishedAt = rc.run.FinishedAt
	if place, ok, err := e.store.LatestPlacement(ctx, rc.run.UserID); err != nil {
		return RunResult{}, err
	} else if ok {
		res.StartUnit = place.SuggestedStartPosition
	}
	return res, nil
}

// computeResult scores a run from its items. FinishedAt and StartUnit are left for the caller.
func (e *Engine) computeResult(ctx context.Context, d *deckV3, rc runCtx) (RunResult, error) {
	res := RunResult{
		RunID: rc.run.ID, UserID: rc.run.UserID, Kind: rc.meta.Kind, StartedAt: rc.run.StartedAt, Misses: []Miss{},
	}
	var lt, rt [BandCount]Tally
	for _, it := range rc.items {
		if it.Outcome == "" {
			continue
		}
		switch it.Part {
		case string(PartLetters):
			res.LettersAsked++
			if it.Outcome == OutcomeRight {
				res.LettersRight++
			}
			continue
		case string(PartListening):
			lt[it.Band-1].Add(it.Outcome)
		case string(PartReading):
			rt[it.Band-1].Add(it.Outcome)
		case string(PartTyping):
			res.Typing.Taken = true
			res.Typing.Asked++
			if it.Outcome == OutcomeRight {
				res.Typing.Right++
			}
		}
		if it.Outcome != OutcomeRight {
			m, err := e.miss(ctx, d, it)
			if err != nil {
				return RunResult{}, err
			}
			res.Misses = append(res.Misses, m)
		}
	}
	if countPart(rc.items, PartTyping) > 0 {
		res.Typing.Taken = true
	}

	if rc.meta.Kind == db.RunRecheck {
		return e.recheckResult(ctx, d, rc, res, lt, rt)
	}
	res.ReadingReady = readingReady(rc.items, len(d.letters))
	res.Listening = skillResult(SkillListen, true, lt)
	res.Reading = skillResult(SkillRead, res.ReadingReady, rt)
	return res, nil
}

// miss is the review entry for one missed item.
func (e *Engine) miss(ctx context.Context, d *deckV3, it db.PlacementItem) (Miss, error) {
	w := d.byID[it.ItemID]
	m := Miss{LexemeID: it.LexemeID, Skill: it.Skill, Picked: it.Picked, RightGloss: w.Gloss, Pointed: w.Text}
	id, ok, err := e.audioID(ctx, w.Text)
	if err != nil {
		return Miss{}, err
	}
	if ok {
		m.AudioID = id
	}
	return m, nil
}
