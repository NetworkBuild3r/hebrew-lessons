package learner

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

// fakeBands is a BandSource with a fixed map.
type fakeBands map[int64]int

func (f fakeBands) LexemeBands(context.Context) (map[int64]int, error) { return f, nil }

var testNow = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

type env struct {
	t     *testing.T
	store *db.Store
	clk   *fakeClock
	model *Model
	user  int64
	words int
}

func newEnv(t *testing.T, opts ...Option) *env {
	t.Helper()
	clk := &fakeClock{t: testNow}
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.InsertUser(context.Background(), "a@example.com", "sub-a", "learner")
	if err != nil {
		t.Fatal(err)
	}
	return &env{t: t, store: store, clk: clk, model: New(store, clk, opts...), user: user}
}

func (e *env) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.store.ExecContext(context.Background(), query, args...); err != nil {
		e.t.Fatalf("exec %s: %v", query, err)
	}
}

// lexeme adds a word. The text is unique per call.
func (e *env) lexeme() int64 {
	e.t.Helper()
	e.words++
	res, err := e.store.ExecContext(context.Background(), `INSERT INTO lexemes (text_nfc, gloss_en) VALUES (?, ?)`,
		fmt.Sprintf("מילה%d", e.words), fmt.Sprintf("word %d", e.words))
	if err != nil {
		e.t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) otherUser() int64 {
	e.t.Helper()
	id, err := e.store.InsertUser(context.Background(), "b@example.com", "sub-b", "learner")
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

// attempts records answers, oldest first: R right, W wrong. They are one minute apart and end
// an hour before now.
func (e *env) attempts(user, lexeme int64, skill, h string) {
	e.t.Helper()
	for i := range len(h) {
		at := testNow.Add(-time.Hour).Add(-time.Duration(len(h)-1-i) * time.Minute)
		correct := 0
		if h[i] == 'R' {
			correct = 1
		}
		e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', ?, 0, 0, 0, ?)`, user, lexeme, skill, correct, db.FormatTime(at))
	}
}

// card adds a card reviewed daysAgo days before now, and one review_log row per rating,
// oldest first, each a day apart ending at the review time.
func (e *env) card(user, lexeme int64, skill string, stability float64, daysAgo int, ratings ...int) int64 {
	e.t.Helper()
	last := testNow.AddDate(0, 0, -daysAgo)
	id, err := e.store.InsertCard(context.Background(), db.Card{
		UserID: user, LexemeID: lexeme, Skill: skill, State: fsrs.StateReview,
		Due: testNow, Stability: stability, Difficulty: 5, Reps: len(ratings), LastReview: last,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	for i, r := range ratings {
		at := last.Add(-time.Duration(len(ratings)-1-i) * 24 * time.Hour)
		e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, ?, ?, 0, 1)`,
			id, r, db.FormatTime(at))
	}
	return id
}

func (e *env) expose(user, lexeme int64) {
	e.t.Helper()
	e.exec(`INSERT INTO exposures (user_id, lexeme_id, context, created_at) VALUES (?, ?, 'story', ?)`,
		user, lexeme, db.FormatTime(testNow.Add(-2*time.Hour)))
}

// known makes a word known in a skill: three right answers, a card of that stability reviewed
// yesterday, and three successful reviews.
func (e *env) known(user, lexeme int64, skill string, stability float64) {
	e.t.Helper()
	e.attempts(user, lexeme, skill, "RRR")
	e.card(user, lexeme, skill, stability, 1, 3, 3, 3)
}

func (e *env) word(lexeme int64) WordState {
	e.t.Helper()
	w, err := e.model.Word(context.Background(), e.user, lexeme)
	if err != nil {
		e.t.Fatal(err)
	}
	return w
}

func skillOf(w WordState, skill string) SkillState {
	for _, s := range w.Skills {
		if s.Skill == skill {
			return s
		}
	}
	return SkillState{}
}

// The rules driven from real rows: every one of the seven states, read through the database.
func TestWordStateFromStoredEvidence(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	unseen := e.lexeme()
	seenExposure := e.lexeme()
	e.expose(e.user, seenExposure)
	seenAnswered := e.lexeme()
	e.attempts(e.user, seenAnswered, fsrs.SkillRead, "R")
	learning := e.lexeme()
	e.attempts(e.user, learning, fsrs.SkillRead, "RR")
	e.card(e.user, learning, fsrs.SkillRead, 2, 1, 3, 3)
	struggling := e.lexeme()
	e.attempts(e.user, struggling, fsrs.SkillRead, "RWRWRWRR")
	e.card(e.user, struggling, fsrs.SkillRead, 4, 1, 3)
	known := e.lexeme()
	e.known(e.user, known, fsrs.SkillRead, 10)
	solid := e.lexeme()
	e.known(e.user, solid, fsrs.SkillRead, 30)
	regressed := e.lexeme()
	e.attempts(e.user, regressed, fsrs.SkillRead, "RRR")
	e.card(e.user, regressed, fsrs.SkillRead, 5, 60, 3, 3, 3)

	want := map[int64]State{
		unseen: Unseen, seenExposure: Seen, seenAnswered: Seen, learning: Learning,
		struggling: Struggling, known: Known, solid: Solid, regressed: Regressed,
	}
	for id, state := range want {
		w := e.word(id)
		if w.State != state {
			t.Errorf("word %d: state %s, want %s (read: %+v)", id, w.State, state, skillOf(w, fsrs.SkillRead))
		}
		if w.LexemeID != id || w.Text == "" || w.Gloss == "" || len(w.Skills) != 4 {
			t.Errorf("word %d: incomplete %+v", id, w)
		}
	}

	// The same states through List, filtered one at a time.
	for _, state := range States {
		page, next, err := e.model.List(ctx, e.user, Filter{State: state})
		if err != nil {
			t.Fatal(err)
		}
		var got []int64
		for _, w := range page {
			got = append(got, w.LexemeID)
		}
		var wantIDs []int64
		for id := int64(1); id <= int64(e.words); id++ {
			if want[id] == state {
				wantIDs = append(wantIDs, id)
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(wantIDs) || next != "" {
			t.Errorf("List(state=%s) = %v next %q, want %v", state, got, next, wantIDs)
		}
	}
}

func TestWordFieldsFromStoredEvidence(t *testing.T) {
	e := newEnv(t)
	id := e.lexeme()
	e.attempts(e.user, id, fsrs.SkillRead, "RWRRR")
	e.card(e.user, id, fsrs.SkillRead, 10, 1, 3, 3, 3)
	w := e.word(id)

	r := skillOf(w, fsrs.SkillRead)
	if r.Attempts != 5 || r.Window != 5 || math.Abs(r.Accuracy-0.8) > 1e-9 || r.AgainRecent != 1 {
		t.Errorf("read skill = %+v", r)
	}
	wantR := fsrs.Retrievability(10, 1)
	if !r.HasCard || r.Stability != 10 || math.Abs(r.Retrievability-wantR) > 1e-9 || r.Successes != 3 || !r.WasLearned {
		t.Errorf("read card fields = %+v, want retrievability %v", r, wantR)
	}
	if w.Stability != 10 || math.Abs(w.Retrievability-wantR) > 1e-9 || w.Attempts != 5 || math.Abs(w.Accuracy-0.8) > 1e-9 {
		t.Errorf("word fields = %+v", w)
	}
	if want := testNow.Add(-time.Hour); !w.LastSeen.Equal(want) {
		t.Errorf("LastSeen = %v, want %v", w.LastSeen, want)
	}
	if w.Review.Reviews != 3 || w.Review.Agains != 0 {
		t.Errorf("review stats = %+v", w.Review)
	}
	for _, other := range []string{fsrs.SkillListen, fsrs.SkillRecall, fsrs.SkillSpell} {
		if s := skillOf(w, other); s.State != Unseen || s.Attempts != 0 || s.HasCard {
			t.Errorf("%s = %+v, want untouched", other, s)
		}
	}
	// Retrievability falls as the clock moves on, and is not stored.
	e.clk.t = testNow.AddDate(0, 0, 30)
	later := e.word(id)
	if later.Retrievability >= w.Retrievability {
		t.Errorf("retrievability did not fall: %v then %v", w.Retrievability, later.Retrievability)
	}
}

// A word's overall state is its weakest skill among those with evidence.
func TestOverallIsWeakestSkillWithEvidence(t *testing.T) {
	e := newEnv(t)

	onlyRead := e.lexeme()
	e.known(e.user, onlyRead, fsrs.SkillRead, 10)
	if w := e.word(onlyRead); w.State != Known {
		t.Errorf("known in read, nothing else: %s (a skill with no evidence is left out)", w.State)
	}

	mixed := e.lexeme()
	e.known(e.user, mixed, fsrs.SkillRead, 30)
	e.attempts(e.user, mixed, fsrs.SkillListen, "WWW")
	w := e.word(mixed)
	if w.State != Struggling {
		t.Errorf("solid in read, struggling in listen: %s", w.State)
	}
	if skillOf(w, fsrs.SkillRead).State != Solid || skillOf(w, fsrs.SkillListen).State != Struggling {
		t.Errorf("skills = %+v", w.Skills)
	}
	// The cause reads the skills: it hears (listen is weak) but reads fine, which no rule names.
	if w.Cause != CauseNone {
		t.Errorf("cause = %q", w.Cause)
	}

	decoding := e.lexeme()
	e.known(e.user, decoding, fsrs.SkillListen, 10)
	e.attempts(e.user, decoding, fsrs.SkillRead, "WWW")
	if w := e.word(decoding); w.State != Struggling || w.Cause != CauseDecoding {
		t.Errorf("listen known, read struggling: state %s cause %q", w.State, w.Cause)
	}

	exposedAndRead := e.lexeme()
	e.expose(e.user, exposedAndRead)
	e.known(e.user, exposedAndRead, fsrs.SkillRead, 10)
	if w := e.word(exposedAndRead); w.State != Known {
		t.Errorf("exposed, and known in read: %s", w.State)
	}
}

func TestWordRefusesBadInput(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if _, err := e.model.Word(ctx, 0, 1); err == nil {
		t.Error("Word with no learner succeeded")
	}
	if _, err := e.model.Word(ctx, e.user, 0); !errors.Is(err, ErrUnknownWord) {
		t.Errorf("lexeme 0: %v", err)
	}
	if _, err := e.model.Word(ctx, e.user, 99999); !errors.Is(err, ErrUnknownWord) {
		t.Errorf("missing lexeme: %v", err)
	}
	if _, _, err := e.model.List(ctx, 0, Filter{}); err == nil {
		t.Error("List with no learner succeeded")
	}
	if _, err := e.model.Summary(ctx, 0); err == nil {
		t.Error("Summary with no learner succeeded")
	}
	var nilModel *Model
	if _, err := nilModel.Word(ctx, 1, 1); err == nil {
		t.Error("nil model succeeded")
	}
}

// One learner's evidence never reaches another's answers.
func TestLearnersAreIsolated(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	other := e.otherUser()
	id := e.lexeme()
	e.attempts(other, id, fsrs.SkillRead, "WWW")
	e.card(other, id, fsrs.SkillRead, 3, 1, 1, 1, 1)
	e.expose(other, id)
	e.exec(`INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at) VALUES (?, 'letter_pair', 'ב/כ', ?, ?)`,
		other, id, db.FormatTime(testNow.Add(-time.Hour)))

	if w := e.word(id); w.State != Unseen || w.Attempts != 0 || w.Review.Reviews != 0 || len(w.Confusions) != 0 {
		t.Errorf("learner A sees learner B: %+v", w)
	}
	page, _, err := e.model.List(ctx, e.user, Filter{State: Struggling})
	if err != nil || len(page) != 0 {
		t.Errorf("List for A = %v, %v", page, err)
	}
	theirs, err := New(e.store, e.clk).Word(ctx, other, id)
	if err != nil || theirs.State != Struggling {
		t.Errorf("learner B = %+v, %v", theirs, err)
	}
	sum, err := e.model.Summary(ctx, e.user)
	if err != nil || sum.Words[Struggling] != 0 || sum.Words[Unseen] != 1 {
		t.Errorf("Summary for A = %+v, %v", sum, err)
	}
	pairs, err := e.model.ConfusionPairs(ctx, e.user, 7*24*time.Hour)
	if err != nil || len(pairs) != 0 {
		t.Errorf("ConfusionPairs for A = %v, %v", pairs, err)
	}
}

// A card with no attempts still counts, through its review_log, where rating 1 is an Again.
// Once the skill has an attempt, the attempts are the outcomes.
func TestReviewLogStandsInForAttemptsOnOldCards(t *testing.T) {
	e := newEnv(t)
	old := e.lexeme()
	e.card(e.user, old, fsrs.SkillRead, 4, 1, 3, 1, 1, 1)
	w := e.word(old)
	if w.State != Struggling {
		t.Errorf("3 Again in the review_log of an old card: %s", w.State)
	}
	if r := skillOf(w, fsrs.SkillRead); r.Attempts != 4 || r.AgainRecent != 3 {
		t.Errorf("read = %+v", r)
	}

	fresh := e.lexeme()
	e.card(e.user, fresh, fsrs.SkillRead, 10, 1, 1, 1, 1, 3, 3, 3)
	e.attempts(e.user, fresh, fsrs.SkillRead, "RRR")
	w = e.word(fresh)
	if r := skillOf(w, fsrs.SkillRead); r.Attempts != 3 || r.AgainRecent != 0 {
		t.Errorf("with attempts present the review_log is not an outcome: %+v", r)
	}
	if w.State != Known {
		t.Errorf("state = %s", w.State)
	}
}

// What the query layer counts equals what OutcomeCounts says for the same history.
func TestStoredCountsMatchOutcomeCounts(t *testing.T) {
	e := newEnv(t)
	rng := rand.New(rand.NewSource(13))
	type want struct {
		id    int64
		skill string
		ev    Evidence
	}
	var wants []want
	for i := range 40 {
		id := e.lexeme()
		skill := Skills[i%len(Skills)]
		n := 1 + rng.Intn(30)
		h := make([]byte, n)
		newestFirst := make([]bool, n)
		for j := range h {
			if rng.Intn(3) == 0 {
				h[j] = 'W'
			} else {
				h[j] = 'R'
			}
			newestFirst[n-1-j] = h[j] == 'R'
		}
		e.attempts(e.user, id, skill, string(h))
		wants = append(wants, want{id, skill, OutcomeCounts(newestFirst)})
	}
	ev, err := e.store.LearnerEvidence(context.Background(), e.user, e.model.query(testNow, 0))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]db.LearnerSkill{}
	for _, s := range ev.Skills {
		got[fmt.Sprint(s.LexemeID, s.Skill)] = s
	}
	for _, w := range wants {
		s, ok := got[fmt.Sprint(w.id, w.skill)]
		if !ok {
			t.Fatalf("no row for %d %s", w.id, w.skill)
		}
		have := Evidence{
			Outcomes: s.Outcomes, Window: s.Window, WindowCorrect: s.WindowCorrect,
			AgainOf: s.AgainOf, Agains: s.Agains, Recent: s.Recent, RecentRight: s.RecentRight,
		}
		if have != w.ev {
			t.Errorf("word %d %s: stored counts %+v, OutcomeCounts %+v", w.id, w.skill, have, w.ev)
		}
	}
}

func TestListPagesByCursor(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	var ids []int64
	for range 12 {
		id := e.lexeme()
		ids = append(ids, id)
		e.attempts(e.user, id, fsrs.SkillRead, "WWW")
	}
	for range 3 {
		e.lexeme() // unseen
	}

	var got []int64
	var cursor Cursor
	pages := 0
	for {
		page, next, err := e.model.List(ctx, e.user, Filter{State: Struggling, Limit: 5, After: cursor})
		if err != nil {
			t.Fatal(err)
		}
		pages++
		if len(page) > 5 {
			t.Fatalf("page of %d, limit 5", len(page))
		}
		for _, w := range page {
			got = append(got, w.LexemeID)
		}
		if next == "" {
			break
		}
		if len(page) != 5 {
			t.Fatalf("a page of %d has a next cursor", len(page))
		}
		cursor = next
	}
	if pages != 3 || fmt.Sprint(got) != fmt.Sprint(ids) {
		t.Fatalf("pages %d, got %v, want %v", pages, got, ids)
	}

	// A list that fills the page exactly has no next cursor.
	page, next, err := e.model.List(ctx, e.user, Filter{State: Struggling, Limit: 12})
	if err != nil || len(page) != 12 || next != "" {
		t.Fatalf("exact page: %d words, next %q, %v", len(page), next, err)
	}
	// No filter lists unseen words too: 12 struggling and 3 unseen.
	page, next, err = e.model.List(ctx, e.user, Filter{Limit: 200})
	if err != nil || len(page) != 15 || next != "" {
		t.Fatalf("unfiltered: %d words, next %q, %v", len(page), next, err)
	}
	// The default page size.
	if page, _, err := e.model.List(ctx, e.user, Filter{}); err != nil || len(page) != 15 {
		t.Fatalf("default page: %d, %v", len(page), err)
	}
}

func TestListPageSizeIsCappedAt200(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.exec(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 250)
		INSERT INTO lexemes (text_nfc) SELECT 'w' || i FROM n`)
	page, next, err := e.model.List(ctx, e.user, Filter{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != MaxPageSize || next == "" {
		t.Fatalf("first page: %d words, next %q, want %d and a cursor", len(page), next, MaxPageSize)
	}
	page, next, err = e.model.List(ctx, e.user, Filter{Limit: 1000, After: next})
	if err != nil || len(page) != 50 || next != "" {
		t.Fatalf("second page: %d words, next %q, %v", len(page), next, err)
	}
	if page[0].LexemeID != 201 || page[0].Text != "w201" {
		t.Fatalf("second page starts at %d %q", page[0].LexemeID, page[0].Text)
	}
	if page, _, err := e.model.List(ctx, e.user, Filter{Limit: -5}); err != nil || len(page) != DefaultPageSize {
		t.Fatalf("negative limit: %d, %v", len(page), err)
	}
}

func TestListFiltersBySkillAndBand(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a, b, c := e.lexeme(), e.lexeme(), e.lexeme()
	e.attempts(e.user, a, fsrs.SkillListen, "WWW")
	e.attempts(e.user, b, fsrs.SkillRead, "WWW")
	e.attempts(e.user, c, fsrs.SkillRead, "WWW")
	e.model.bands = fakeBands{a: 1, b: 1, c: 2}

	ids := func(f Filter) string {
		page, _, err := e.model.List(ctx, e.user, f)
		if err != nil {
			t.Fatal(err)
		}
		var out []int64
		for _, w := range page {
			out = append(out, w.LexemeID)
		}
		return fmt.Sprint(out)
	}
	one, two := 1, 2
	if got := ids(Filter{State: Struggling}); got != fmt.Sprint([]int64{a, b, c}) {
		t.Errorf("overall struggling = %s", got)
	}
	if got := ids(Filter{State: Struggling, Skill: fsrs.SkillRead}); got != fmt.Sprint([]int64{b, c}) {
		t.Errorf("read struggling = %s", got)
	}
	if got := ids(Filter{State: Unseen, Skill: fsrs.SkillRead}); got != fmt.Sprint([]int64{a}) {
		t.Errorf("read unseen = %s", got)
	}
	if got := ids(Filter{State: Struggling, Band: &one}); got != fmt.Sprint([]int64{a, b}) {
		t.Errorf("band 1 = %s", got)
	}
	if got := ids(Filter{State: Struggling, Skill: fsrs.SkillRead, Band: &two}); got != fmt.Sprint([]int64{c}) {
		t.Errorf("band 2 read = %s", got)
	}
	if w := e.word(a); w.Band != 1 {
		t.Errorf("Word band = %d", w.Band)
	}
}

func TestListRefusesBadFilters(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	bad, high := -1, 5
	for name, f := range map[string]Filter{
		"state":       {State: "mastered"},
		"skill":       {Skill: "write"},
		"band low":    {Band: &bad},
		"band high":   {Band: &high},
		"not base64":  {After: "!!"},
		"wrong shape": {After: Cursor("YWJj")},
		"negative id": {After: encodeCursor(-1)},
	} {
		if _, _, err := e.model.List(ctx, e.user, f); err == nil {
			t.Errorf("%s: no error", name)
		} else if name == "not base64" || name == "wrong shape" || name == "negative id" {
			if !errors.Is(err, ErrBadCursor) {
				t.Errorf("%s: %v, want ErrBadCursor", name, err)
			}
		}
	}
}

func TestCursorRoundTrip(t *testing.T) {
	for _, id := range []int64{0, 1, 200, 1 << 40} {
		got, err := decodeCursor(encodeCursor(id))
		if err != nil || got != id {
			t.Errorf("cursor %d came back as %d, %v", id, got, err)
		}
	}
	if id, err := decodeCursor(""); err != nil || id != 0 {
		t.Errorf("empty cursor = %d, %v", id, err)
	}
}

// A band slip: enough struggling words of one band in the last week.
func TestBandSlipThroughTheModel(t *testing.T) {
	e := newEnv(t)
	bands := fakeBands{}
	var band1 []int64
	for range BandSlipWords {
		id := e.lexeme()
		bands[id] = 1
		band1 = append(band1, id)
		e.attempts(e.user, id, fsrs.SkillListen, "WWW")
	}
	e.model.bands = bands

	for _, id := range band1 {
		w := e.word(id)
		if w.BandStrugglers != BandSlipWords || w.Cause != CauseBandSlip {
			t.Errorf("word %d: band strugglers %d, cause %q", id, w.BandStrugglers, w.Cause)
		}
	}
	page, _, err := e.model.List(context.Background(), e.user, Filter{State: Struggling})
	if err != nil || len(page) != BandSlipWords {
		t.Fatal(page, err)
	}
	for _, w := range page {
		if w.Cause != CauseBandSlip {
			t.Errorf("List word %d: cause %q", w.LexemeID, w.Cause)
		}
	}

	// One fewer struggling word, and the band is not slipping.
	e.exec(`DELETE FROM attempts WHERE lexeme_id = ?`, band1[0])
	for _, id := range band1[1:] {
		if w := e.word(id); w.BandStrugglers != BandSlipWords-1 || w.Cause != CauseNone {
			t.Errorf("word %d: band strugglers %d, cause %q", id, w.BandStrugglers, w.Cause)
		}
	}

	// Words last seen more than a week ago do not count toward a slip.
	e.exec(`UPDATE attempts SET created_at = ?`, db.FormatTime(testNow.AddDate(0, 0, -BandSlipDays-1)))
	e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		SELECT user_id, ?, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at FROM attempts WHERE lexeme_id = ?`,
		band1[0], band1[1])
	if w := e.word(band1[1]); w.BandStrugglers != 0 {
		t.Errorf("stale words counted: %d", w.BandStrugglers)
	}
}

// Confusion rows reach the cause and the 7-day keys.
func TestConfusionPairThroughTheModel(t *testing.T) {
	e := newEnv(t)
	id := e.lexeme()
	e.attempts(e.user, id, fsrs.SkillListen, "WWW")
	add := func(kind, key string, ago time.Duration) {
		e.exec(`INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at) VALUES (?, ?, ?, ?, ?)`,
			e.user, kind, key, id, db.FormatTime(testNow.Add(-ago)))
	}
	add("letter_pair", "ב/כ", time.Hour)
	add("letter_pair", "ב/כ", 2*time.Hour)
	if w := e.word(id); w.Cause != CauseNone {
		t.Errorf("two confusions: cause %q", w.Cause)
	}
	add("letter_pair", "ב/כ", 10*24*time.Hour) // inside 30 days, outside 7
	add("other", "ט/ת", 45*24*time.Hour)       // outside 30 days
	w := e.word(id)
	if w.Cause != CauseConfusion {
		t.Errorf("three of one pair: cause %q", w.Cause)
	}
	if len(w.Confusions) != 3 {
		t.Errorf("confusions in the window = %d, want 3", len(w.Confusions))
	}
	if fmt.Sprint(w.Review.ConfusionKeys) != "[ב/כ]" {
		t.Errorf("7-day keys = %v", w.Review.ConfusionKeys)
	}
}

// ac-4: Summary's estimated words known, against a fixture worked by hand.
//
// Seven words, with the bands the placement deck would give them:
//
//	L1 band 0, known in read      L2 band 0, learning in read   L3 band 1, known in read
//	L4 band 1, struggling in read L5 band 2, solid in read      L6 no band, known in read
//	L7 band 1, no evidence
//
// The finished placement run says: band 0 rated Good 3 and Easy 1 (4 known words), band 1
// rate 0.5, band 2 rate 0.25, band 3 rate 0.1, band 4 rate 0. Band widths are 500, 500, 1000, 2000.
//
// read: evidenced words per band: band 0 has 2, band 1 has 2, band 2 has 1.
//
//	band 0   4 - 2                       =   2
//	band 1   0.5  * (500 - 2)            = 249
//	band 2   0.25 * (500 - 1)            = 124.75
//	band 3   0.1  * 1000                 = 100
//	band 4   0    * 2000                 =   0
//	known and solid words: L1, L3, L5, L6 =   4
//	estimate                              = 479.75
//
// listen has no evidence at all: 4 + 250 + 125 + 100 + 0 + 0 = 479.
func TestSummaryEstimatedKnownMatchesHandComputedFixture(t *testing.T) {
	e := newEnv(t)
	l1, l2, l3, l4, l5, l6, l7 := e.lexeme(), e.lexeme(), e.lexeme(), e.lexeme(), e.lexeme(), e.lexeme(), e.lexeme()
	e.model.bands = fakeBands{l1: 0, l2: 0, l3: 1, l4: 1, l5: 2, l7: 1}
	ctx := context.Background()

	e.known(e.user, l1, fsrs.SkillRead, 10)
	e.attempts(e.user, l2, fsrs.SkillRead, "RR")
	e.card(e.user, l2, fsrs.SkillRead, 2, 1, 3, 3)
	e.known(e.user, l3, fsrs.SkillRead, 10)
	e.attempts(e.user, l4, fsrs.SkillRead, "WWW")
	e.known(e.user, l5, fsrs.SkillRead, 40)
	e.known(e.user, l6, fsrs.SkillRead, 10)

	e.exec(`INSERT INTO placement_runs (user_id, started_at, finished_at, level, stop_reason, result, seed, sitting_no)
		VALUES (?, ?, ?, 2, NULL, ?, 1, 1)`, e.user, db.FormatTime(testNow.Add(-48*time.Hour)), db.FormatTime(testNow.Add(-47*time.Hour)),
		`{"estimated_known_words": 999, "bands": [
			{"band": 0, "rated": 10, "good": 3, "easy": 1, "good_plus_rate": 0.4},
			{"band": 1, "rated": 20, "good": 10, "good_plus_rate": 0.5},
			{"band": 2, "rated": 20, "good": 5, "good_plus_rate": 0.25},
			{"band": 3, "rated": 20, "good": 2, "good_plus_rate": 0.1},
			{"band": 4, "rated": 20, "good_plus_rate": 0}]}`)

	sum, err := e.model.Summary(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Skills) != 4 {
		t.Fatalf("skills = %d", len(sum.Skills))
	}
	byName := map[string]SkillSummary{}
	for i, s := range sum.Skills {
		if s.Skill != Skills[i] {
			t.Errorf("skill %d is %s, want %s", i, s.Skill, Skills[i])
		}
		byName[s.Skill] = s
	}
	if got, want := byName[fsrs.SkillRead].EstimatedKnown, 479.75; math.Abs(got-want) > 1e-9 {
		t.Errorf("read estimate = %v, want %v", got, want)
	}
	if got, want := byName[fsrs.SkillListen].EstimatedKnown, 479.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("listen estimate = %v, want %v", got, want)
	}

	// Counts per skill and state. L7 and no one else is unseen in read.
	wantRead := map[State]int{Unseen: 1, Learning: 1, Struggling: 1, Known: 3, Solid: 1}
	for _, s := range States {
		if got := byName[fsrs.SkillRead].Counts[s]; got != wantRead[s] {
			t.Errorf("read %s = %d, want %d", s, got, wantRead[s])
		}
	}
	if got := byName[fsrs.SkillListen].Counts[Unseen]; got != 7 {
		t.Errorf("listen unseen = %d, want 7", got)
	}
	// Overall: L1, L3, L6 known, L2 learning, L4 struggling, L5 solid, L7 unseen.
	wantWords := map[State]int{Unseen: 1, Learning: 1, Struggling: 1, Known: 3, Solid: 1}
	for _, s := range States {
		if sum.Words[s] != wantWords[s] {
			t.Errorf("words %s = %d, want %d", s, sum.Words[s], wantWords[s])
		}
	}

	// Without a finished placement the estimate is only the model's count.
	e.exec(`DELETE FROM placement_runs`)
	sum, err = e.model.Summary(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sum.Skills {
		want := map[string]float64{fsrs.SkillRead: 4}[s.Skill]
		if s.EstimatedKnown != want {
			t.Errorf("%s estimate without placement = %v, want %v", s.Skill, s.EstimatedKnown, want)
		}
	}
	// An unfinished run adds nothing either.
	e.exec(`INSERT INTO placement_runs (user_id, started_at, seed, sitting_no) VALUES (?, ?, 1, 1)`, e.user, db.FormatTime(testNow))
	sum, err = e.model.Summary(ctx, e.user)
	if err != nil || sum.Skills[1].EstimatedKnown != 4 {
		t.Errorf("unfinished run: %v, %v", sum.Skills[1].EstimatedKnown, err)
	}
}

// Retention is the share of due reviews in the last 30 days not rated Again.
func TestSummaryRetention(t *testing.T) {
	e := newEnv(t)
	id := e.lexeme()
	card := e.card(e.user, id, fsrs.SkillRead, 10, 1)
	other := e.card(e.user, id, fsrs.SkillListen, 10, 1)
	review := func(card int64, rating, elapsed int, ago time.Duration) {
		e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, ?, ?, ?, 1)`,
			card, rating, db.FormatTime(testNow.Add(-ago)), elapsed)
	}
	day := 24 * time.Hour
	review(card, 3, 5, 2*day)   // due, right
	review(card, 4, 9, 3*day)   // due, right
	review(card, 1, 7, 4*day)   // due, wrong
	review(card, 2, 1, 5*day)   // due (exactly 1 day), right
	review(card, 1, 0, day)     // same-sitting repeat: not due
	review(card, 1, 12, 40*day) // outside the 30 days
	review(other, 1, 3, day)    // another skill: due, wrong

	sum, err := e.model.Summary(context.Background(), e.user)
	if err != nil {
		t.Fatal(err)
	}
	read, listen, recall := sum.Skills[1], sum.Skills[0], sum.Skills[2]
	if read.DueReviews != 4 || read.Retention == nil || *read.Retention != 0.75 {
		t.Errorf("read retention = %v of %d", deref(read.Retention), read.DueReviews)
	}
	if listen.DueReviews != 1 || listen.Retention == nil || *listen.Retention != 0 {
		t.Errorf("listen retention = %v of %d", deref(listen.Retention), listen.DueReviews)
	}
	if recall.Retention != nil || recall.DueReviews != 0 {
		t.Errorf("recall retention = %v of %d, want none", deref(recall.Retention), recall.DueReviews)
	}
	if sum.DueReviews != 5 || sum.Retention == nil || math.Abs(*sum.Retention-0.6) > 1e-9 {
		t.Errorf("overall retention = %v of %d, want 0.6 of 5", deref(sum.Retention), sum.DueReviews)
	}
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// The estimate formula alone, with the widths placement uses.
func TestEstimateKnown(t *testing.T) {
	placed := []db.BandScore{
		{Band: 0, Good: 2, Easy: 2},
		{Band: 1, GoodPlusRate: 1},
		{Band: 2, GoodPlusRate: 0.5},
		{Band: 3, GoodPlusRate: 0.25},
		{Band: 4, GoodPlusRate: 0.1},
		{Band: 9, GoodPlusRate: 1}, // not a band
	}
	// 4 + 500 + 250 + 250 + 200, then 7 known words.
	if got := estimateKnown(placed, true, [5]int{}, 7); got != 1211 {
		t.Errorf("no evidence: %v, want 1211", got)
	}
	// Evidence cuts the placement part, never below zero.
	if got := estimateKnown(placed, true, [5]int{9, 600, 0, 3000, 0}, 0); got != 0+0+250+0+200 {
		t.Errorf("heavy evidence: %v, want 450", got)
	}
	if got := estimateKnown(placed, false, [5]int{1, 1, 1, 1, 1}, 5); got != 5 {
		t.Errorf("no placement: %v, want 5", got)
	}
}

// bandRankWidth must match the widths internal/placement scores with. A band whose 20 words
// are all rated Good has a rate of 1, so the run's estimate is that band's width.
func TestBandRankWidthsMatchPlacement(t *testing.T) {
	for band := 1; band <= 4; band++ {
		var tallies [5]placement.BandTally
		tallies[band] = placement.BandTally{Ratings: slices.Repeat([]int{placement.Good}, 20), Dealable: 20}
		if got := placement.Score(tallies).EstimatedKnownWords; float64(got) != bandRankWidth[band] {
			t.Errorf("band %d: placement width %d, learner width %v", band, got, bandRankWidth[band])
		}
	}
	if bandRankWidth[0] != 0 {
		t.Errorf("band 0 width = %v, want 0", bandRankWidth[0])
	}
}
