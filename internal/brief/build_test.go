package brief

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

// fakeBands is a learner.BandSource with a fixed map.
type fakeBands map[int64]int

func (f fakeBands) LexemeBands(context.Context) (map[int64]int, error) { return f, nil }

// fakeTags is a TagSource with a fixed map.
type fakeTags map[int64][]string

func (f fakeTags) Tags(_ context.Context, ids []int64) (map[int64][]string, error) {
	out := map[int64][]string{}
	for _, id := range ids {
		if t, ok := f[id]; ok {
			out[id] = t
		}
	}
	return out, nil
}

var testNow = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type env struct {
	t     *testing.T
	store *db.Store
	clk   *fakeClock
	bands fakeBands
	b     *Builder
	user  int64
	words int
	x     execer
}

// newEnv opens a database with one learner. The learner model reads the env's bands map, which
// the test fills as it adds words. Options go to the Builder.
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
	e := &env{t: t, store: store, clk: clk, bands: fakeBands{}, user: user, x: store}
	model := learner.New(store, clk, learner.WithBands(e.bands))
	e.b = New(store, model, clk, opts...)
	return e
}

func (e *env) exec(query string, args ...any) sql.Result {
	e.t.Helper()
	res, err := e.x.ExecContext(context.Background(), query, args...)
	if err != nil {
		e.t.Fatalf("exec %s: %v", query, err)
	}
	return res
}

// batch runs fixture inserts in one transaction, which is much faster than one commit each.
func (e *env) batch(fn func()) {
	e.t.Helper()
	conn, err := e.store.DB()
	if err != nil {
		e.t.Fatal(err)
	}
	tx, err := conn.BeginTx(context.Background(), nil)
	if err != nil {
		e.t.Fatal(err)
	}
	e.x = tx
	defer func() { e.x = e.store }()
	fn()
	if err := tx.Commit(); err != nil {
		e.t.Fatal(err)
	}
}

// word adds a lexeme in a band (-1 for none) and returns its id.
func (e *env) word(gloss string, band int) int64 {
	e.t.Helper()
	e.words++
	res := e.exec(`INSERT INTO lexemes (text_nfc, gloss_en) VALUES (?, ?)`, fmt.Sprintf("מילה%d", e.words), gloss)
	id, err := res.LastInsertId()
	if err != nil {
		e.t.Fatal(err)
	}
	if band >= 0 {
		e.bands[id] = band
	}
	return id
}

func (e *env) other() int64 {
	e.t.Helper()
	id, err := e.store.InsertUser(context.Background(), "b@example.com", "sub-b", "learner")
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

// attempts adds answers, oldest first, R right and W wrong, one minute apart, ending ago before now.
func (e *env) attempts(user, lexeme int64, skill, h string, ago time.Duration) {
	e.t.Helper()
	for i := range len(h) {
		at := testNow.Add(-ago).Add(-time.Duration(len(h)-1-i) * time.Minute)
		correct := 0
		if h[i] == 'R' {
			correct = 1
		}
		e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', ?, 0, 0, 0, ?)`, user, lexeme, skill, correct, db.FormatTime(at))
	}
}

// card adds a card reviewed daysAgo days ago with one successful review_log row per rating.
func (e *env) card(user, lexeme int64, skill string, stability float64, daysAgo int, ratings ...int) {
	e.t.Helper()
	last := testNow.AddDate(0, 0, -daysAgo)
	res := e.exec(`INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended)
		VALUES (?, ?, ?, ?, ?, ?, 5, ?, 0, ?, 0)`, user, lexeme, skill, fsrs.StateReview, db.FormatTime(testNow), stability, len(ratings), db.FormatTime(last))
	id, err := res.LastInsertId()
	if err != nil {
		e.t.Fatal(err)
	}
	for i, r := range ratings {
		at := last.Add(-time.Duration(len(ratings)-1-i) * 24 * time.Hour)
		e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, ?, ?, 0, 1)`,
			id, r, db.FormatTime(at))
	}
}

// known makes a word known in a skill, last answered ago before now.
func (e *env) known(user, lexeme int64, skill string, ago time.Duration) {
	e.t.Helper()
	e.attempts(user, lexeme, skill, "RRR", ago)
	e.card(user, lexeme, skill, 10, 1, 3, 3, 3)
}

// struggling makes a word struggling in a skill with the given number of right answers of 8.
func (e *env) struggling(user, lexeme int64, skill string, rights int) {
	e.t.Helper()
	e.attempts(user, lexeme, skill, strings.Repeat("R", rights)+strings.Repeat("W", 8-rights), time.Hour)
}

// regressed makes a word that was learned and has faded in a skill.
func (e *env) regressed(user, lexeme int64, skill string) {
	e.t.Helper()
	e.attempts(user, lexeme, skill, "RRR", time.Hour)
	e.card(user, lexeme, skill, 5, 60, 3, 3, 3)
}

func (e *env) build(theme string) Brief {
	e.t.Helper()
	br, err := e.b.Build(context.Background(), e.user, theme)
	if err != nil {
		e.t.Fatalf("Build(%q): %v", theme, err)
	}
	return br
}

func ids[W interface{ id() int64 }](ws []W) []int64 {
	out := make([]int64, len(ws))
	for i, w := range ws {
		out[i] = w.id()
	}
	return out
}

func (w Word) id() int64 { return w.LexemeID }

// fixture is the learner of the acceptance tests: 160 words known in listen, all of band 0;
// 12 unseen words in band 1, three of them about food; 10 unseen in band 2; six struggling
// words in read with 0 to 5 right answers of 8, and one regressed word.
type fixture struct {
	*env
	knownIDs      []int64 // Known words 150 to 159 are the most recently seen, then 0 to 149 by age.
	band1         []int64
	band1Food     []int64
	band2         []int64
	strugglingIDs []int64 // Ascending right answers, so descending score.
	regressedID   int64
}

func newFixture(t *testing.T, opts ...Option) *fixture {
	t.Helper()
	f := &fixture{env: newEnv(t, opts...)}
	f.batch(func() {
		for i := range 160 {
			id := f.word(fmt.Sprintf("known %d", i), 0)
			f.knownIDs = append(f.knownIDs, id)
			ago := time.Duration(i+1) * time.Hour
			if i >= 150 {
				ago = time.Duration(i-149) * time.Minute
			}
			f.known(f.user, id, fsrs.SkillListen, ago)
		}
		for i := range 12 {
			gloss := fmt.Sprintf("band one %d", i)
			if i < 3 {
				gloss = fmt.Sprintf("bread (food) %d", i)
			}
			id := f.word(gloss, 1)
			f.band1 = append(f.band1, id)
			if i < 3 {
				f.band1Food = append(f.band1Food, id)
			}
		}
		for i := range 10 {
			f.band2 = append(f.band2, f.word(fmt.Sprintf("band two %d", i), 2))
		}
		for rights := range 6 {
			id := f.word(fmt.Sprintf("struggle %d", rights), 1)
			f.strugglingIDs = append(f.strugglingIDs, id)
			f.struggling(f.user, id, fsrs.SkillRead, rights)
		}
		f.regressedID = f.word("regressed", 1)
		f.regressed(f.user, f.regressedID, fsrs.SkillRead)
	})
	return f
}

func in(set []int64, id int64) bool { return slices.Contains(set, id) }

// ac-1: 5 to 8 new words from the lowest band with unseen words, none of them struggling.
func TestAC1NewWordsFromLowestBandNeverStruggling(t *testing.T) {
	f := newFixture(t)
	br := f.build("")

	if got := len(br.NewWords); got < MinNewWords || got > MaxNewWords {
		t.Fatalf("new words = %d, want %d to %d", got, MinNewWords, MaxNewWords)
	}
	for _, w := range br.NewWords {
		if w.Band != 1 || !in(f.band1, w.LexemeID) {
			t.Errorf("new word %d is band %d, want a band 1 word", w.LexemeID, w.Band)
		}
		if w.MinUses != NewWordMinUses {
			t.Errorf("new word %d min uses %d", w.LexemeID, w.MinUses)
		}
		st, err := f.b.model.Word(context.Background(), f.user, w.LexemeID)
		if err != nil {
			t.Fatal(err)
		}
		if st.State != learner.Unseen {
			t.Errorf("new word %d is %s, want unseen", w.LexemeID, st.State)
		}
		if in(f.strugglingIDs, w.LexemeID) || w.LexemeID == f.regressedID {
			t.Errorf("new word %d is a struggle word", w.LexemeID)
		}
	}
	// Band 1 holds 12 unseen words and also the six struggling and one regressed word, which
	// are not unseen, so no struggling word can have been offered.
	seen := map[int64]bool{}
	for _, w := range br.NewWords {
		if seen[w.LexemeID] {
			t.Errorf("new word %d repeated", w.LexemeID)
		}
		seen[w.LexemeID] = true
	}
}

// ac-2: each struggle word carries its failing skill and a count of 3 or more.
func TestAC2StruggleWordsCarrySkillAndCount(t *testing.T) {
	f := newFixture(t)
	br := f.build("")

	if len(br.StruggleWords) != MaxStruggleWords {
		t.Fatalf("struggle words = %d, want %d", len(br.StruggleWords), MaxStruggleWords)
	}
	for i, w := range br.StruggleWords {
		if w.Skill != fsrs.SkillRead {
			t.Errorf("word %d skill = %q, want read", w.LexemeID, w.Skill)
		}
		if w.RequiredCount < MinStruggleUses {
			t.Errorf("word %d required count %d, want at least %d", w.LexemeID, w.RequiredCount, MinStruggleUses)
		}
		if w.State != string(learner.Struggling) {
			t.Errorf("word %d state %q", w.LexemeID, w.State)
		}
		if !slices.Equal(w.Activities, []string{ActivityReadPick, ActivityWordMatch}) {
			t.Errorf("word %d activities %v", w.LexemeID, w.Activities)
		}
		// Worst first: 0 right answers, then 1, 2, 3, 4.
		if w.LexemeID != f.strugglingIDs[i] {
			t.Errorf("struggle word %d is %d, want %d (score %.1f)", i, w.LexemeID, f.strugglingIDs[i], w.Score)
		}
		if i > 0 && w.Score > br.StruggleWords[i-1].Score {
			t.Errorf("scores not descending at %d", i)
		}
	}
	if slices.Contains(ids(br.StruggleWords), f.strugglingIDs[5]) {
		t.Error("the sixth-worst word is in the brief")
	}
}

// The selection rules, one row per rule, in the order the report lists them.
func TestSelectionRules(t *testing.T) {
	f := newFixture(t)
	br := f.build("")

	t.Run("comfortable words are known or solid, at most 150", func(t *testing.T) {
		if len(br.ComfortableWords) != MaxComfortableWords {
			t.Fatalf("comfortable = %d, want %d (160 known)", len(br.ComfortableWords), MaxComfortableWords)
		}
		for _, w := range br.ComfortableWords {
			if !in(f.knownIDs, w.LexemeID) {
				t.Errorf("comfortable word %d is not a known word", w.LexemeID)
			}
		}
	})
	t.Run("comfortable words prefer the most recently seen", func(t *testing.T) {
		// Words 150 to 159 were answered minutes ago, so they come first. The rest follow by
		// recency, then id.
		want := append(slices.Clone(f.knownIDs[150:]), f.knownIDs[:140]...)
		if got := ids(br.ComfortableWords); !slices.Equal(got, want) {
			t.Errorf("comfortable order = %v..., want the most recently seen first", got[:3])
		}
	})
	t.Run("skill focus is the weakest skill", func(t *testing.T) {
		// Every evidenced read word is struggling or regressed, and no listen word is.
		if br.SkillFocus != fsrs.SkillRead {
			t.Errorf("focus = %q, want read", br.SkillFocus)
		}
	})
	t.Run("activities train the focus and each failing skill, without repeats", func(t *testing.T) {
		want := []string{ActivityReadPick, ActivityWordMatch}
		if !slices.Equal(br.Activities, want) {
			t.Errorf("activities = %v, want %v", br.Activities, want)
		}
	})
	t.Run("new word count falls as the review load grows", func(t *testing.T) {
		// 5 struggle words leave room for 8 - (5 - 3) = 6.
		if len(br.NewWords) != 6 {
			t.Errorf("new words = %d, want 6", len(br.NewWords))
		}
	})
	t.Run("targets and tier", func(t *testing.T) {
		if br.Targets.Coverage != 0.95 || br.Targets.NewWordMinUses != 3 {
			t.Errorf("targets = %+v", br.Targets)
		}
		if br.Tier.Name != TierDeveloping || br.Tier.KnownWords != 160 || br.Tier.MaxTextTokens != 60 {
			t.Errorf("tier = %+v", br.Tier)
		}
	})
	t.Run("theme default", func(t *testing.T) {
		if br.Theme != DefaultTheme || len(br.Notes) != 0 {
			t.Errorf("theme %q notes %v", br.Theme, br.Notes)
		}
	})
}

func TestRegressedWordsFillTheStruggleList(t *testing.T) {
	e := newEnv(t)
	e.batch(func() {
		for i := range 4 {
			id := e.word(fmt.Sprintf("known %d", i), 0)
			e.known(e.user, id, fsrs.SkillListen, time.Hour)
		}
	})
	reg := e.word("regressed", 0)
	strug := e.word("struggling", 0)
	e.regressed(e.user, reg, fsrs.SkillRead)
	e.struggling(e.user, strug, fsrs.SkillRead, 4)
	br := e.build("")
	if len(br.StruggleWords) != 2 {
		t.Fatalf("struggle words = %d, want 2 (fewer than the usual 3, so fewer are listed)", len(br.StruggleWords))
	}
	byID := map[int64]StruggleWord{}
	for _, w := range br.StruggleWords {
		byID[w.LexemeID] = w
	}
	if w := byID[reg]; w.State != "regressed" || w.RequiredCount != 3 {
		t.Errorf("regressed word = %+v", w)
	}
	if w := byID[strug]; w.State != "struggling" || w.RequiredCount != 4 {
		t.Errorf("struggling word = %+v", w)
	}
	if br.StruggleWords[0].LexemeID != strug {
		t.Error("the struggling word should outrank the regressed one")
	}
}

func TestStruggleScoreRules(t *testing.T) {
	base := learner.WordState{State: learner.Struggling, Attempts: 10, Accuracy: 0.5}
	if got := StruggleScore(base); got != 75 {
		t.Errorf("base score = %v, want 75 (50 for accuracy, 25 for struggling)", got)
	}
	regressed := base
	regressed.State = learner.Regressed
	if got := StruggleScore(regressed); got != 50 {
		t.Errorf("regressed score = %v, want 50", got)
	}
	withMisses := base
	withMisses.Review.Misses7d = 2
	withMisses.Review.ConfusionKeys = []string{"ב/כ"}
	if got := StruggleScore(withMisses); got != 75+20+5 {
		t.Errorf("score with misses and confusion = %v, want 100", got)
	}
	noAttempts := learner.WordState{State: learner.Struggling}
	if got := StruggleScore(noAttempts); got != 50+25 {
		t.Errorf("no attempts score = %v, want 75", got)
	}
}

func TestStruggleTiesBreakByLexemeID(t *testing.T) {
	e := newEnv(t)
	var made []int64
	for range 4 {
		id := e.word("tie", 0)
		made = append(made, id)
		e.struggling(e.user, id, fsrs.SkillRead, 3)
	}
	br := e.build("")
	if got := ids(br.StruggleWords); !slices.Equal(got, made) {
		t.Errorf("tied words = %v, want ascending ids %v", got, made)
	}
}

func TestFailingSkillAndCause(t *testing.T) {
	ws := func(skills map[string]learner.SkillState) learner.WordState {
		w := learner.WordState{State: learner.Struggling}
		for _, s := range learner.Skills {
			sk := skills[s]
			sk.Skill = s
			if sk.State == "" {
				sk.State = learner.Unseen
			}
			w.Skills = append(w.Skills, sk)
		}
		return w
	}
	tests := []struct {
		name string
		w    learner.WordState
		want string
	}{
		{"the one weak skill", ws(map[string]learner.SkillState{
			fsrs.SkillListen: {State: learner.Known, Accuracy: 1},
			fsrs.SkillRead:   {State: learner.Struggling, Accuracy: 0.4},
		}), fsrs.SkillRead},
		{"the weaker of two weak skills", ws(map[string]learner.SkillState{
			fsrs.SkillListen: {State: learner.Struggling, Accuracy: 0.5},
			fsrs.SkillSpell:  {State: learner.Regressed, Accuracy: 0.2},
		}), fsrs.SkillSpell},
		{"a tie goes to the first skill", ws(map[string]learner.SkillState{
			fsrs.SkillRecall: {State: learner.Struggling, Accuracy: 0.5},
			fsrs.SkillRead:   {State: learner.Struggling, Accuracy: 0.5},
		}), fsrs.SkillRead},
		{"no weak skill falls back to recall", ws(nil), fsrs.SkillRecall},
	}
	for _, tt := range tests {
		if got := FailingSkill(tt.w); got != tt.want {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}

	// From stored evidence: hears it well, cannot read it. The model diagnoses decoding.
	e := newEnv(t)
	id := e.word("decode", 0)
	e.known(e.user, id, fsrs.SkillListen, time.Hour)
	e.struggling(e.user, id, fsrs.SkillRead, 1)
	br := e.build("")
	if len(br.StruggleWords) != 1 {
		t.Fatalf("struggle words = %d", len(br.StruggleWords))
	}
	w := br.StruggleWords[0]
	if w.Skill != fsrs.SkillRead || w.Cause != string(learner.CauseDecoding) || br.FocusCause != "decoding" {
		t.Errorf("word %+v focus cause %q, want read and decoding", w, br.FocusCause)
	}
}

func TestSkillFocus(t *testing.T) {
	mk := func(counts map[string]map[learner.State]int) learner.Summary {
		var sum learner.Summary
		for _, s := range learner.Skills {
			c := counts[s]
			if c == nil {
				c = map[learner.State]int{}
			}
			sum.Skills = append(sum.Skills, learner.SkillSummary{Skill: s, Counts: c})
		}
		return sum
	}
	all := learner.Skills
	weak := map[learner.State]int{learner.Struggling: 3, learner.Known: 3}
	fine := map[learner.State]int{learner.Known: 6}
	thin := map[learner.State]int{learner.Struggling: 2}

	tests := []struct {
		name     string
		sum      learner.Summary
		unlocked []string
		want     string
	}{
		{"no evidence anywhere is listening", mk(nil), all, fsrs.SkillListen},
		{"the weakest skill", mk(map[string]map[learner.State]int{fsrs.SkillListen: fine, fsrs.SkillRecall: weak}), all, fsrs.SkillRecall},
		{"too little evidence is not judged", mk(map[string]map[learner.State]int{fsrs.SkillListen: fine, fsrs.SkillSpell: thin}), all, fsrs.SkillListen},
		{"a locked skill is never the focus", mk(map[string]map[learner.State]int{fsrs.SkillListen: fine, fsrs.SkillRead: weak}),
			[]string{fsrs.SkillListen, fsrs.SkillRecall}, fsrs.SkillListen},
		{"a tie goes to the first skill", mk(map[string]map[learner.State]int{fsrs.SkillRead: weak, fsrs.SkillRecall: weak}), all, fsrs.SkillRead},
		{"learning counts half", mk(map[string]map[learner.State]int{
			fsrs.SkillListen: {learner.Learning: 4, learner.Known: 2},
			fsrs.SkillRead:   {learner.Struggling: 1, learner.Known: 5}}), all, fsrs.SkillListen},
	}
	for _, tt := range tests {
		if got := skillFocus(tt.sum, tt.unlocked); got != tt.want {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}

	// The gate hook is read by Build.
	f := newFixture(t, WithUnlockedSkills(func(context.Context, int64) ([]string, error) {
		return []string{fsrs.SkillListen, fsrs.SkillRecall}, nil
	}))
	if got := f.build("").SkillFocus; got == fsrs.SkillRead {
		t.Errorf("focus = read although read is locked")
	}
}

func TestActivitiesNeverNameFlashcard(t *testing.T) {
	for skill, types := range skillActivities {
		if len(types) == 0 {
			t.Errorf("skill %s has no activity", skill)
		}
		for _, a := range types {
			if a == "flashcard" {
				t.Errorf("skill %s names flashcard", skill)
			}
		}
	}
	for _, s := range learner.Skills {
		if _, ok := skillActivities[s]; !ok {
			t.Errorf("no activities for %s", s)
		}
	}
}

func TestNewWordsTopUpFromNextBand(t *testing.T) {
	e := newEnv(t) // No evidence: a beginner, so 4 new words.
	b0 := []int64{e.word("a", 0), e.word("b", 0)}
	var b1 []int64
	for range 5 {
		b1 = append(b1, e.word("c", 1))
	}
	e.word("d", 2)
	br := e.build("")
	if len(br.NewWords) != 4 {
		t.Fatalf("new words = %d, want 4", len(br.NewWords))
	}
	got := ids(br.NewWords)
	for _, id := range b0 {
		if !in(got, id) {
			t.Errorf("band 0 word %d is missing: the lowest band is taken whole before the next", id)
		}
	}
	for _, w := range br.NewWords[2:] {
		if !in(b1, w.LexemeID) {
			t.Errorf("top-up word %d is not band 1", w.LexemeID)
		}
	}
	if br.Notes != nil && len(br.Notes) != 0 {
		t.Errorf("notes = %v, want none", br.Notes)
	}
}

func TestNewWordsUnbandedFallback(t *testing.T) {
	e := newEnv(t)
	for range 10 {
		e.word("never placed", -1)
	}
	br := e.build("")
	if len(br.NewWords) != 4 {
		t.Fatalf("new words = %d, want 4", len(br.NewWords))
	}
	for _, w := range br.NewWords {
		if w.Band != -1 {
			t.Errorf("word %d band = %d, want -1", w.LexemeID, w.Band)
		}
	}
}

func TestNewWordsFewerAvailableSaysSo(t *testing.T) {
	e := newEnv(t)
	e.word("only", 0)
	e.word("two", 0)
	br := e.build("")
	if len(br.NewWords) != 2 {
		t.Fatalf("new words = %d, want 2", len(br.NewWords))
	}
	if len(br.Notes) != 1 || !strings.Contains(br.Notes[0], "Only 2 unseen words") {
		t.Errorf("notes = %v", br.Notes)
	}
}

func TestThemeFitsNewAndComfortableWords(t *testing.T) {
	f := newFixture(t)
	br := f.build("Food and drink")
	if br.Theme != "Food and drink" {
		t.Fatalf("theme = %q", br.Theme)
	}
	var themed int
	for i, w := range br.NewWords {
		if w.ThemeMatch {
			themed++
			if !in(f.band1Food, w.LexemeID) {
				t.Errorf("word %d is marked themed but is not a food word", w.LexemeID)
			}
			if i >= len(f.band1Food) {
				t.Errorf("themed word at %d came after an unthemed one", i)
			}
		}
	}
	if themed != 3 {
		t.Errorf("themed new words = %d, want all 3 food words first", themed)
	}
	if len(br.Notes) != 1 || !strings.Contains(br.Notes[0], "Only 3 of 6 new words match the theme") {
		t.Errorf("notes = %v", br.Notes)
	}
}

func TestThemeWithNoMatchIsIgnoredAndSaid(t *testing.T) {
	f := newFixture(t)
	plain := f.build("")
	br := f.build("astronomy")
	if !slices.Equal(ids(br.NewWords), ids(plain.NewWords)) {
		t.Error("an unmatched theme changed the new words")
	}
	if len(br.Notes) != 1 || !strings.Contains(br.Notes[0], "theme was ignored") {
		t.Errorf("notes = %v, want the theme-ignored note", br.Notes)
	}
	if br.Theme != "astronomy" {
		t.Errorf("theme = %q, want it kept", br.Theme)
	}
}

func TestThemeTagsBeatGloss(t *testing.T) {
	tags := fakeTags{}
	f := newFixture(t, WithTags(tags)) // Gloss "food" words are band1[0:3]. Tag two other words.
	tagged := []int64{f.band1[10], f.band1[11]}
	tags[tagged[0]] = []string{"Weather"}
	tags[tagged[1]] = []string{"weather", "sky"}
	br := f.build("the weather")
	var got []int64
	for _, w := range br.NewWords {
		if w.ThemeMatch {
			got = append(got, w.LexemeID)
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, tagged) {
		t.Errorf("themed words = %v, want the two tagged words %v", got, tagged)
	}
}

func TestThemeKeywordMatching(t *testing.T) {
	if got := themeKeywords("The Food and Drinks of the city"); !slices.Equal(got, []string{"food", "drink", "city"}) {
		t.Errorf("keywords = %v", got)
	}
	if got := themeKeywords("a b"); len(got) != 0 {
		t.Errorf("short words gave keywords %v", got)
	}
	kw := themeKeywords("houses")
	if !matchesAny(kw, "a house") || matchesAny(kw, "horse") {
		t.Error("plural and exact matching is wrong")
	}
	if matchesAny(themeKeywords("bus"), "business") {
		t.Error("a keyword must match a whole word")
	}
}

func TestThemeValidation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if br := e.build("   "); br.Theme != DefaultTheme {
		t.Errorf("blank theme = %q, want %q", br.Theme, DefaultTheme)
	}
	if br := e.build("  at the market  "); br.Theme != "at the market" {
		t.Errorf("theme = %q, want it trimmed", br.Theme)
	}
	// 200 Hebrew characters are 400 bytes and fit. The column counts characters.
	if br := e.build(strings.Repeat("ש", MaxThemeRunes)); utf8Len(br.Theme) != MaxThemeRunes {
		t.Error("200 characters were not kept")
	}
	for name, theme := range map[string]string{
		"too long":    strings.Repeat("x", MaxThemeRunes+1),
		"newline":     "a\nb",
		"tab":         "a\tb",
		"nul":         "a\x00b",
		"bad utf-8":   "a\xffb",
		"hebrew long": strings.Repeat("ש", MaxThemeRunes+1),
	} {
		if _, err := e.b.Build(ctx, e.user, theme); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	var n int
	conn, _ := e.store.DB()
	if err := conn.QueryRow(`SELECT COUNT(*) FROM lesson_briefs`).Scan(&n); err != nil || n != 3 {
		t.Errorf("rows = %d (%v), want 3: rejected themes must store nothing", n, err)
	}
}

func utf8Len(s string) int { return len([]rune(s)) }

// A learner with no evidence at all gets a beginner brief: new words only.
func TestNewLearnerFallback(t *testing.T) {
	e := newEnv(t)
	for i := range 20 {
		e.word(fmt.Sprintf("w%d", i), i%3)
	}
	br := e.build("")
	if br.Tier.Name != TierBeginner || br.Tier.KnownWords != 0 || br.Tier.MaxTextTokens != 20 || br.Tier.MinKnownShare != 0.60 {
		t.Errorf("tier = %+v", br.Tier)
	}
	if n := len(br.NewWords); n < BeginnerMinNewWords || n > BeginnerMaxNewWords {
		t.Errorf("new words = %d, want %d to %d", n, BeginnerMinNewWords, BeginnerMaxNewWords)
	}
	for _, w := range br.NewWords {
		if w.Band != 0 {
			t.Errorf("word %d band = %d, want the lowest band 0", w.LexemeID, w.Band)
		}
	}
	if len(br.StruggleWords) != 0 || len(br.ComfortableWords) != 0 || br.FocusCause != "" {
		t.Errorf("a new learner has no struggle or comfortable words: %+v", br.Plan)
	}
	if br.SkillFocus != DefaultSkillFocus || !slices.Equal(br.Activities, []string{ActivityListenPick, ActivityAudioMatch}) {
		t.Errorf("focus %q activities %v, want listening", br.SkillFocus, br.Activities)
	}
	payload := storedPayload(t, e, br.ID)
	if strings.Contains(payload, "null") {
		t.Errorf("payload has a null where an empty list belongs: %s", payload)
	}
}

func TestEmptyLexiconStillBuilds(t *testing.T) {
	e := newEnv(t)
	br := e.build("")
	if len(br.NewWords) != 0 || len(br.Notes) != 1 || !strings.Contains(br.Notes[0], "Only 0 unseen words") {
		t.Errorf("brief = %+v", br.Plan)
	}
}

func TestTierAndNewWordCount(t *testing.T) {
	tiers := []struct {
		known int
		name  string
		lo    int
		hi    int
	}{
		{0, TierBeginner, 3, 4}, {99, TierBeginner, 3, 4},
		{100, TierDeveloping, 5, 8}, {299, TierDeveloping, 5, 8},
		{300, TierEstablished, 5, 8}, {5000, TierEstablished, 5, 8},
	}
	for _, tt := range tiers {
		tier := TierFor(tt.known)
		if tier.Name != tt.name || tier.KnownWords != tt.known {
			t.Errorf("TierFor(%d) = %+v", tt.known, tier)
		}
		if got := NewWordCount(tier, 0); got != tt.hi {
			t.Errorf("%d known, no struggle: %d new, want %d", tt.known, got, tt.hi)
		}
		if got := NewWordCount(tier, MaxStruggleWords); got < tt.lo || got > tt.hi {
			t.Errorf("%d known, 5 struggle: %d new, want within %d..%d", tt.known, got, tt.lo, tt.hi)
		}
		for s := 0; s <= 10; s++ {
			if got := NewWordCount(tier, s); got < tt.lo || got > tt.hi {
				t.Errorf("%d known, %d struggle: %d new, outside %d..%d", tt.known, s, got, tt.lo, tt.hi)
			}
		}
	}
	if NewWordCount(TierFor(150), 3) != 8 || NewWordCount(TierFor(150), 4) != 7 || NewWordCount(TierFor(150), 5) != 6 {
		t.Error("the count should fall by one per struggle word past three")
	}
	if NewWordCount(TierFor(10), 5) != 3 {
		t.Error("a beginner with a heavy review load takes the minimum of 3")
	}
}

// A beginner takes 3 to 4 new words, an integrated check of the tier from stored evidence.
func TestBeginnerBriefFromEvidence(t *testing.T) {
	e := newEnv(t)
	for i := range 30 {
		id := e.word(fmt.Sprintf("k%d", i), 0)
		e.known(e.user, id, fsrs.SkillListen, time.Hour)
	}
	for range 10 {
		e.word("u", 1)
	}
	br := e.build("")
	if br.Tier.Name != TierBeginner || br.Tier.KnownWords != 30 {
		t.Fatalf("tier = %+v", br.Tier)
	}
	if n := len(br.NewWords); n != 4 {
		t.Errorf("new words = %d, want 4", n)
	}
	if len(br.ComfortableWords) != 30 {
		t.Errorf("comfortable = %d, want all 30 known words", len(br.ComfortableWords))
	}
}

// ac-3: two calls on the same day with no new evidence choose the same word sets.
func TestAC3SameDaySameWordSets(t *testing.T) {
	f := newFixture(t)
	a, b := f.build("Food"), f.build("Food")
	if a.ID == b.ID {
		t.Fatal("two builds shared an id")
	}
	if !slices.Equal(ids(a.NewWords), ids(b.NewWords)) ||
		!slices.Equal(ids(a.StruggleWords), ids(b.StruggleWords)) ||
		!slices.Equal(ids(a.ComfortableWords), ids(b.ComfortableWords)) {
		t.Error("the same day gave different word sets")
	}
	// Later the same day, still the same.
	f.clk.t = f.clk.t.Add(5 * time.Hour)
	if c := f.build("Food"); !slices.Equal(ids(a.NewWords), ids(c.NewWords)) {
		t.Error("a later time on the same day changed the new words")
	}
	// The whole plan, not only the ids.
	a.Plan.Notes, b.Plan.Notes = nil, nil
	ja, _ := json.Marshal(a.Plan)
	jb, _ := json.Marshal(b.Plan)
	if string(ja) != string(jb) {
		t.Error("the plans differ")
	}
}

func TestSelectionIsSeededByDayUserAndSeed(t *testing.T) {
	f := newFixture(t)
	sets := map[string]bool{}
	for day := range 12 {
		f.clk.t = testNow.AddDate(0, 0, day)
		sets[fmt.Sprint(ids(f.build("").NewWords))] = true
	}
	if len(sets) < 2 {
		t.Error("twelve days chose the same new words every day: the day is not in the seed")
	}

	// A different seed, same day.
	f.clk.t = testNow
	base := ids(f.build("").NewWords)
	different := false
	for seed := uint64(1); seed < 12 && !different; seed++ {
		g := New(f.store, f.b.model, f.clk, WithSeed(seed))
		br, err := g.Build(context.Background(), f.user, "")
		if err != nil {
			t.Fatal(err)
		}
		different = !slices.Equal(base, ids(br.NewWords))
	}
	if !different {
		t.Error("eleven seeds chose the same new words: the seed is not used")
	}

	// The order a learner gets is not the order another learner gets from the same pool.
	differ := 0
	for id := int64(1); id <= 50; id++ {
		if f.b.rank(1, "2026-10-19", id) != f.b.rank(2, "2026-10-19", id) {
			differ++
		}
	}
	if differ != 50 {
		t.Errorf("only %d of 50 ranks changed with the user", differ)
	}
}

// Evidence added between two calls can change the plan.
func TestNewEvidenceChangesThePlan(t *testing.T) {
	f := newFixture(t)
	first := f.build("")
	// The learner now struggles with the first new word.
	f.struggling(f.user, first.NewWords[0].LexemeID, fsrs.SkillListen, 1)
	second := f.build("")
	if slices.Contains(ids(second.NewWords), first.NewWords[0].LexemeID) {
		t.Error("a word that is now struggling is still a new word")
	}
}

// Hard limits.
func TestHardLimits(t *testing.T) {
	f := newFixture(t)
	for _, theme := range []string{"", "Food", "astronomy"} {
		br := f.build(theme)
		if n := len(br.NewWords); n < MinNewWords || n > MaxNewWords {
			t.Errorf("theme %q: new words %d", theme, n)
		}
		if len(br.StruggleWords) > MaxStruggleWords || len(br.ComfortableWords) > MaxComfortableWords {
			t.Errorf("theme %q: struggle %d comfortable %d", theme, len(br.StruggleWords), len(br.ComfortableWords))
		}
		payload := storedPayload(t, f.env, br.ID)
		if len(payload) > MaxPayloadBytes {
			t.Errorf("theme %q: payload %d bytes over %d", theme, len(payload), MaxPayloadBytes)
		}
	}
}

func TestLongFieldsAreCutOrSkipped(t *testing.T) {
	e := newEnv(t)
	long := e.word(strings.Repeat("g", 300), 0)
	e.exec(`INSERT INTO lexemes (text_nfc, gloss_en) VALUES (?, 'too long')`, strings.Repeat("א", MaxTextRunes+1))
	br := e.build("")
	var got *NewWord
	for i := range br.NewWords {
		if br.NewWords[i].LexemeID == long {
			got = &br.NewWords[i]
		}
		if br.NewWords[i].Gloss == "too long" {
			t.Error("a word with over 80 characters of text was offered")
		}
	}
	if got == nil {
		t.Fatal("the word with the long gloss was not offered")
	}
	if utf8Len(got.Gloss) != MaxGlossRunes {
		t.Errorf("gloss = %d characters, want it cut to %d", utf8Len(got.Gloss), MaxGlossRunes)
	}
}

func TestPayloadSizeLimitTrimsComfortableWordsThenFails(t *testing.T) {
	f := newFixture(t)
	full := f.build("")
	size := len(storedPayload(t, f.env, full.ID))

	f.b.maxPayload = size - 1000
	trimmed := f.build("")
	if n := len(trimmed.ComfortableWords); n >= len(full.ComfortableWords) || n == 0 {
		t.Fatalf("comfortable = %d, want fewer than %d and more than none", n, len(full.ComfortableWords))
	}
	if p := storedPayload(t, f.env, trimmed.ID); len(p) > f.b.maxPayload {
		t.Errorf("payload %d bytes over the limit %d", len(p), f.b.maxPayload)
	}
	if !slices.Contains(trimmed.Notes, trimNote) {
		t.Errorf("notes = %v, want the trim note", trimmed.Notes)
	}
	// The kept words are the most preferred: a prefix of the full list.
	if !slices.Equal(ids(trimmed.ComfortableWords), ids(full.ComfortableWords)[:len(trimmed.ComfortableWords)]) {
		t.Error("trimming did not keep the preferred words")
	}

	before := briefCount(t, f.env)
	f.b.maxPayload = 10
	if _, err := f.b.Build(context.Background(), f.user, ""); err != ErrTooLarge {
		t.Errorf("err = %v, want ErrTooLarge", err)
	}
	if briefCount(t, f.env) != before {
		t.Error("a brief that was too large was stored")
	}
}

func TestCoachNotes(t *testing.T) {
	e := newEnv(t)
	other := e.other()
	note := func(user int64, body string, ago time.Duration) {
		e.exec(`INSERT INTO coach_notes (user_id, author, body, created_at) VALUES (?, 'muse', ?, ?)`,
			user, body, db.FormatTime(testNow.Add(-ago)))
	}
	note(e.user, "too old", 40*24*time.Hour)
	for i := range 7 {
		note(e.user, fmt.Sprintf("note %d", i), time.Duration(7-i)*time.Hour)
	}
	note(other, "someone else's", time.Hour)
	br := e.build("")
	if len(br.CoachNotes) != MaxCoachNotes {
		t.Fatalf("notes = %d, want %d", len(br.CoachNotes), MaxCoachNotes)
	}
	for i, n := range br.CoachNotes {
		if want := fmt.Sprintf("note %d", 6-i); n.Body != want || n.Author != "muse" {
			t.Errorf("note %d = %+v, want %q newest first", i, n, want)
		}
	}
	// Notes are stored with the brief.
	got, err := e.b.Get(context.Background(), e.user, br.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.CoachNotes) != MaxCoachNotes || got.CoachNotes[0].Body != "note 6" {
		t.Errorf("stored notes = %+v", got.CoachNotes)
	}
	// A learner with none gets an empty list.
	if n := len(newEnv(t).build("").CoachNotes); n != 0 {
		t.Errorf("notes = %d for a learner with none", n)
	}
}

// The payload is a versioned shape. Its keys are pinned so a change is deliberate.
func TestPayloadSchemaV1(t *testing.T) {
	f := newFixture(t)
	f.exec(`INSERT INTO coach_notes (user_id, author, body, created_at) VALUES (?, 'muse', 'x', ?)`, f.user, db.FormatTime(testNow))
	br := f.build("Food")

	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(storedPayload(t, f.env, br.ID)), &doc); err != nil {
		t.Fatal(err)
	}
	keys := func(m map[string]json.RawMessage) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	want := []string{"activities", "coach_notes", "comfortable_words", "focus_cause", "new_words", "notes",
		"skill_focus", "struggle_words", "targets", "theme", "tier", "v"}
	if got := keys(doc); !slices.Equal(got, want) {
		t.Errorf("payload keys = %v, want %v", got, want)
	}
	if string(doc["v"]) != "1" {
		t.Errorf(`"v" = %s, want 1`, doc["v"])
	}
	first := func(name string) map[string]json.RawMessage {
		var list []map[string]json.RawMessage
		if err := json.Unmarshal(doc[name], &list); err != nil || len(list) == 0 {
			t.Fatalf("%s: %v (%d)", name, err, len(list))
		}
		return list[0]
	}
	check := func(name string, got map[string]json.RawMessage, want ...string) {
		t.Helper()
		if k := keys(got); !slices.Equal(k, want) {
			t.Errorf("%s keys = %v, want %v", name, k, want)
		}
	}
	check("new_words", first("new_words"), "band", "gloss", "lexeme_id", "min_uses", "text", "theme_match")
	check("struggle_words", first("struggle_words"), "activities", "cause", "gloss", "lexeme_id", "required_count", "score", "skill", "state", "text")
	check("comfortable_words", first("comfortable_words"), "gloss", "lexeme_id", "text")
	check("coach_notes", first("coach_notes"), "at", "author", "body")
	var tier, targets map[string]json.RawMessage
	_ = json.Unmarshal(doc["tier"], &tier)
	_ = json.Unmarshal(doc["targets"], &targets)
	check("tier", tier, "known_words", "max_text_tokens", "min_known_share", "name")
	check("targets", targets, "coverage", "new_word_min_uses")

	// The brief a client sees adds the id and the times, and never the learner id.
	raw, _ := json.Marshal(br)
	var top map[string]json.RawMessage
	_ = json.Unmarshal(raw, &top)
	for _, k := range []string{"id", "created_at", "expires_at", "v", "new_words"} {
		if _, ok := top[k]; !ok {
			t.Errorf("brief JSON lacks %q", k)
		}
	}
	for _, k := range []string{"user_id", "UserID"} {
		if _, ok := top[k]; ok {
			t.Errorf("brief JSON has %q", k)
		}
	}
	if _, ok := top["used_by_unit_id"]; ok {
		t.Error("an unused brief has used_by_unit_id in its JSON")
	}
	// The stored payload has neither the id nor the learner.
	if _, ok := doc["id"]; ok {
		t.Error("payload carries the id")
	}
}

func storedPayload(t *testing.T, e *env, id string) string {
	t.Helper()
	conn, err := e.store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var p string
	if err := conn.QueryRow(`SELECT payload_json FROM lesson_briefs WHERE id = ?`, id).Scan(&p); err != nil {
		t.Fatal(err)
	}
	return p
}

func briefCount(t *testing.T, e *env) int {
	t.Helper()
	conn, err := e.store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM lesson_briefs`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
