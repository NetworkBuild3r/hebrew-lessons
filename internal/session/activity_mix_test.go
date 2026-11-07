package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

var richFacts = activity.Facts{HasGloss: true, HasAudio: true, GlossedOthers: 40, VoicedOthers: 40}

func skillView(skill string, state learner.State, accuracy float64, attempts int) activity.SkillView {
	return activity.SkillView{Skill: skill, State: state, Accuracy: accuracy, Attempts: attempts, HasCard: attempts > 0}
}

// listenWeakest is a word whose listen accuracy is the lowest, the other three skills fine.
func listenWeakest() []activity.SkillView {
	return []activity.SkillView{
		skillView("listen", learner.Learning, 0.25, 8),
		skillView("read", learner.Learning, 0.9, 8),
		skillView("recall", learner.Learning, 0.8, 8),
		skillView("spell", learner.Learning, 0.7, 8),
	}
}

func modelCand(id int64, skill string, due time.Time) Candidate {
	c := knownCand(id, due)
	c.Skill = skill
	c.Skills = listenWeakest()
	c.Facts = richFacts
	c.Exposed = true
	return c
}

// INIT-027/SPEC-004: the review mix asks through the card's own skill, not the weakest.
func TestMixAsksThroughTheCardsOwnSkill(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	got := mix([]Candidate{modelCand(1, fsrs.SkillRead, now.Add(-time.Hour))}, TargetSuccess, now, 1)
	if len(got) != 1 {
		t.Fatalf("queue = %+v", got)
	}
	ref := got[0]
	if ref.Activity != activity.ReadPick || ref.AskSkill != fsrs.SkillRead {
		t.Fatalf("ref = %+v, want read_pick through read", ref)
	}
	if ref.MovedUp || ref.OwnActivity != activity.ReadPick {
		t.Fatalf("ref = %+v, own skill is not strong enough to move up", ref)
	}
	if ref.CardID != 1 || ref.Skill != fsrs.SkillRead || ref.LexemeID != 1 {
		t.Fatalf("ref = %+v, want the due read card", ref)
	}
}

func TestMixAsksEachCardThroughItsOwnSkill(t *testing.T) {
	// INIT-027/SPEC-004: review asks the card skill, not the word's weakest skill.
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	due := now.Add(-time.Hour)
	build := func(id int64, skill string) Candidate {
		var views []activity.SkillView
		for _, s := range activity.Skills {
			views = append(views, skillView(s, learner.Learning, 0.5, 8))
		}
		c := knownCand(id, due.Add(time.Duration(id)*time.Minute))
		c.Skill = skill
		c.Skills, c.Facts, c.Exposed = views, richFacts, true
		return c
	}
	got := mix([]Candidate{
		build(1, fsrs.SkillListen), build(2, fsrs.SkillRead),
		build(3, fsrs.SkillRecall), build(4, fsrs.SkillSpell),
	}, TargetSuccess, now, 1)
	want := map[int64]activity.Kind{1: activity.ListenPick, 2: activity.ReadPick, 3: activity.TypeIt, 4: activity.Dictation}
	if len(got) != 4 {
		t.Fatalf("queue = %+v", got)
	}
	for _, ref := range got {
		if ref.Activity != want[ref.CardID] || ref.AskSkill != ref.Skill {
			t.Errorf("card %d: %+v, want activity %q through %q", ref.CardID, ref, want[ref.CardID], ref.Skill)
		}
	}
}

// The mix never asks a flashcard when a checked activity fits, whatever the word's skills.
func TestMixNeverAsksAFlashcardWhenACheckedActivityFits(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	rng := rand.New(rand.NewPCG(3, 4))
	states := []learner.State{learner.Unseen, learner.Seen, learner.Struggling, learner.Regressed, learner.Learning, learner.Known, learner.Solid}
	for round := 0; round < 300; round++ {
		var cands []Candidate
		n := 1 + rng.IntN(25)
		ownFits := map[int64]bool{}
		for i := 0; i < n; i++ {
			id := int64(i + 1)
			var views []activity.SkillView
			for _, s := range activity.Skills {
				att := rng.IntN(3) * 4
				views = append(views, activity.SkillView{Skill: s, State: states[rng.IntN(len(states))], Accuracy: rng.Float64(), Attempts: att, HasCard: att > 0})
			}
			c := knownCand(id, now.Add(-time.Duration(i+1)*time.Minute))
			c.Skill = []string{"read", "recall", "listen", "spell"}[rng.IntN(4)]
			c.Skills = views
			c.Facts = activity.Facts{HasGloss: rng.IntN(5) > 0, HasAudio: rng.IntN(5) > 0, GlossedOthers: rng.IntN(7), VoicedOthers: rng.IntN(7)}
			c.Exposed = true
			cands = append(cands, c)
			ownFits[id] = len(activity.Fitting(c.Skill, c.Facts)) > 0
		}
		for _, ref := range mix(cands, TargetSuccess, now, int64(round)) {
			if ref.Kind == KindLetter || ref.Kind == KindPattern {
				continue
			}
			if ownFits[ref.CardID] && ref.Activity == activity.Flashcard && !ref.MovedUp {
				t.Fatalf("round %d: card %d is a flashcard although its own skill fits", round, ref.CardID)
			}
			if !ownFits[ref.CardID] && !ref.MovedUp && (ref.Activity != activity.Flashcard || ref.AskSkill != ref.Skill) {
				t.Fatalf("round %d: card %d has %+v but own skill fits nothing", round, ref.CardID, ref)
			}
		}
	}
}

func TestMixOmitsListenWithoutClipKeepsFlashcardSentinel(t *testing.T) {
	// ADR-0028 D6: only listen-without-clip is omitted. Other unaskable skills
	// keep the flashcard sentinel for promoteFlashcard.
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	c := modelCand(1, fsrs.SkillRecall, now.Add(-time.Hour))
	c.Facts = activity.Facts{} // no gloss and no clip
	listen := modelCand(2, fsrs.SkillListen, now.Add(-time.Minute))
	listen.Facts = activity.Facts{HasGloss: true, GlossedOthers: 40} // no clip
	askable := modelCand(3, fsrs.SkillRead, now.Add(-2*time.Minute))
	got, skipped := mixGate([]Candidate{c, listen, askable}, TargetSuccess, now, 1, activity.OpenGate())
	if skipped != 1 {
		t.Fatalf("SkippedNoAudio = %d, want 1 listen without clip", skipped)
	}
	byID := map[int64]CardRef{}
	for _, ref := range got {
		byID[ref.CardID] = ref
	}
	if _, ok := byID[2]; ok {
		t.Fatalf("listen without clip stayed in the queue: %+v", got)
	}
	if ref, ok := byID[1]; !ok || ref.Activity != activity.Flashcard || ref.AskSkill != fsrs.SkillRecall {
		t.Fatalf("no-fit recall = %+v, want flashcard sentinel", ref)
	}
	if ref, ok := byID[3]; !ok || ref.Activity == activity.Flashcard {
		t.Fatalf("askable read = %+v", ref)
	}
}

func TestMixChangesNoCardAndNoOrder(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	var plain, rich []Candidate
	for i := 0; i < 30; i++ {
		due := now.Add(-time.Duration(i+1) * time.Minute)
		var c Candidate
		if i%3 == 0 {
			c = struggleCand(int64(i+1), due)
		} else {
			c = knownCand(int64(i+1), due)
		}
		plain = append(plain, c)
		c.Skills, c.Facts, c.Exposed = listenWeakest(), richFacts, true
		rich = append(rich, c)
	}
	a, b := mix(plain, TargetSuccess, now, 7), mix(rich, TargetSuccess, now, 7)
	if !reflect.DeepEqual(idsOf(a), idsOf(b)) || pattern(a) != pattern(b) {
		t.Fatalf("asking through skills changed the queue:\n%v\n%v", idsOf(a), idsOf(b))
	}
}

func TestMixRotatesTheEverydayAndTheMatchActivity(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	var cands []Candidate
	for i := 1; i <= 9; i++ {
		cands = append(cands, modelCand(int64(i), fsrs.SkillRead, now.Add(-time.Duration(20-i)*time.Minute)))
	}
	got := mix(cands, TargetSuccess, now, 1)
	var kinds []activity.Kind
	for _, ref := range got {
		kinds = append(kinds, ref.Activity)
	}
	// Review alternates everyday and match within the skill run (ADR-0028 D6).
	want := []activity.Kind{
		activity.ReadPick, activity.WordMatch, activity.ReadPick, activity.WordMatch,
		activity.ReadPick, activity.WordMatch, activity.ReadPick, activity.WordMatch,
		activity.ReadPick,
	}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("activities = %v, want %v", kinds, want)
	}
}

// Two due cards of one word are each asked through their own skill.
func TestMixAsksARepeatedWordThroughEachCardsSkill(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	he := modelCand(1, fsrs.SkillRead, now.Add(-2*time.Hour))
	en := modelCand(2, fsrs.SkillRecall, now.Add(-time.Hour))
	en.LexemeID = he.LexemeID
	got := mix([]Candidate{he, en}, TargetSuccess, now, 1)
	if len(got) != 2 {
		t.Fatalf("queue = %+v", got)
	}
	if got[0].AskSkill != fsrs.SkillRead || got[1].AskSkill != fsrs.SkillRecall {
		t.Fatalf("skills = %q then %q, want read then recall", got[0].AskSkill, got[1].AskSkill)
	}
}

func TestMixHonoursTheGate(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	c := modelCand(1, fsrs.SkillRecall, now.Add(-time.Hour))
	c.Skills = []activity.SkillView{
		skillView("listen", learner.Known, 0.95, 8), skillView("read", learner.Struggling, 0.1, 8),
		skillView("recall", learner.Struggling, 0.1, 8), skillView("spell", learner.Struggling, 0.1, 8),
	}
	closed, _ := mixGate([]Candidate{c}, TargetSuccess, now, 1, activity.Gate{})
	if len(closed) != 1 || closed[0].Activity != activity.Flashcard || closed[0].AskSkill != fsrs.SkillRecall {
		t.Fatalf("closed gate: %+v, want a flashcard through recall", closed)
	}
	opened, _ := mixGate([]Candidate{c}, TargetSuccess, now, 1, activity.OpenGate())
	if len(opened) != 1 || opened[0].Activity != activity.TypeIt || opened[0].AskSkill != fsrs.SkillRecall {
		t.Fatalf("open gate: %+v, want type_it through recall", opened)
	}
}

// New words open with an exposure before their first check.
func TestMixMarksNewWordsThatNeedAnIntro(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	fresh := modelCand(1, fsrs.SkillRead, now.Add(-time.Hour))
	fresh.HasRating, fresh.Exposed = false, false
	shown := modelCand(2, fsrs.SkillRead, now.Add(-time.Hour))
	shown.HasRating, shown.Exposed = false, true
	reviewed := modelCand(3, fsrs.SkillRead, now.Add(-time.Hour))
	reviewed.Exposed = false
	got := mix([]Candidate{fresh, shown, reviewed}, TargetSuccess, now, 1)
	intro := map[int64]bool{}
	isNew := map[int64]bool{}
	for _, ref := range got {
		intro[ref.CardID], isNew[ref.CardID] = ref.Intro, ref.New
	}
	if !isNew[1] || !isNew[2] || isNew[3] {
		t.Fatalf("new = %v", isNew)
	}
	if !intro[1] {
		t.Error("a new word that was never shown must open with an exposure")
	}
	if intro[2] {
		t.Error("a new word that was already shown does not need another")
	}
	if intro[3] {
		t.Error("a reviewed word is not new")
	}
}

func TestMixLeavesDrillsWithoutAnActivity(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	cands := []Candidate{modelCand(1, fsrs.SkillRead, now.Add(-time.Hour))}
	cands = append(cands, drillCandidates([]db.ConfusionPair{{Kind: fsrs.MissLetterPair, A: "ב", B: "כ", Count: 3}})...)
	got := mix(cands, TargetSuccess, now, 1)
	drills := 0
	for _, ref := range got {
		if ref.Kind == KindLetter {
			drills++
			if ref.Activity != "" || ref.AskSkill != "" || ref.Intro {
				t.Fatalf("drill step carries an activity: %+v", ref)
			}
		}
	}
	if drills != DrillSteps {
		t.Fatalf("%d drill steps, want %d", drills, DrillSteps)
	}
}

func TestClassifyListenAndSpellCards(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	due, later := now.Add(-time.Minute), now.Add(time.Hour)
	for _, skill := range []string{fsrs.SkillListen, fsrs.SkillSpell} {
		base := Candidate{Skill: skill, Due: due, HasRating: true, Latest: int(fsrs.Good)}
		for name, tc := range map[string]struct {
			edit func(*Candidate)
			want kind
		}{
			"good, known by its own rating": {func(*Candidate) {}, kindKnown},
			"good later":                    {func(c *Candidate) { c.Due = later }, kindSkip},
			"again":                         {func(c *Candidate) { c.Latest = int(fsrs.Again) }, kindStruggle},
			"lapses":                        {func(c *Candidate) { c.Lapses = StruggleLapses }, kindStruggle},
			"confused":                      {func(c *Candidate) { c.Confused = true }, kindStruggle},
			"unreviewed":                    {func(c *Candidate) { c.HasRating = false }, kindNew},
			"recent lapse":                  {func(c *Candidate) { c.RecentLapse = true }, kindStruggle},
		} {
			c := base
			tc.edit(&c)
			if got := classify(c, now); got != tc.want {
				t.Errorf("%s card, %s: kind %d, want %d", skill, name, got, tc.want)
			}
		}
	}
	if got := classify(Candidate{Skill: "speak", Due: due, HasRating: true, Latest: int(fsrs.Good)}, now); got != kindSkip {
		t.Errorf("an unknown skill is kind %d, want skip", got)
	}
}

// ---- BuildSession over a real database --------------------------------------------------

func addVoicedLexeme(t *testing.T, store *db.Store, text, gloss string, audio bool) int64 {
	t.Helper()
	var audioID any
	if audio {
		sum := sha256.Sum256([]byte("clip:" + text))
		id := hex.EncodeToString(sum[:])
		if _, err := store.InsertAudio(context.Background(), db.Asset{
			ID: id, TextNFC: text, Voice: "v", Model: "m", Bytes: 1, CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatal(err)
		}
		audioID = id
	}
	res, err := store.ExecContext(context.Background(),
		`INSERT INTO lexemes (text_nfc, gloss_en, pos, audio_id) VALUES (?, ?, 'noun', ?)`, text, gloss, audioID)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// fillPool adds n words with a clip and a gloss, so every activity can be built.
func fillPool(t *testing.T, store *db.Store, n int) {
	t.Helper()
	letters := []rune("אבגדהוזחטיכלמנסעפצקרשת")
	for i := 0; i < n; i++ {
		text := string([]rune{letters[i%len(letters)], letters[(i/len(letters)+3)%len(letters)], 'ה', letters[(i*7)%len(letters)]}) + fmt.Sprint(i)
		addVoicedLexeme(t, store, text, fmt.Sprintf("pool%c%c", 'a'+rune(i%26), 'a'+rune(i/26)), true)
	}
}

func addCardFor(t *testing.T, store *db.Store, userID, lexemeID int64, skill string, due time.Time, rating int, reviewed time.Time) int64 {
	t.Helper()
	id, err := store.InsertCard(context.Background(), db.Card{
		UserID: userID, LexemeID: lexemeID, Skill: skill, State: fsrs.StateReview, Due: due,
		Stability: 1, Difficulty: 5, Reps: 1, LastReview: reviewed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rating != 0 {
		insertReview(t, store, id, rating, reviewed)
	}
	return id
}

// lessonAttempts stores outcomes for a word and skill without touching any card.
func lessonAttempts(t *testing.T, svc *Service, userID, lexemeID int64, skill string, right, wrong int) {
	t.Helper()
	rec := evidence.New(svc.store, nil, svc.clock)
	for i := 0; i < right+wrong; i++ {
		if _, err := rec.Attempt(context.Background(), evidence.Attempt{
			UserID: userID, LexemeID: lexemeID, Skill: skill, Activity: "story_question",
			Source: evidence.SourceLesson, Prompt: "p", Response: "r", Correct: i < right, SkipSchedule: true,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBuildSessionAsksThroughTheCardsOwnSkill(t *testing.T) {
	// INIT-027/SPEC-004: weakest listen no longer steals the due read card.
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	fillPool(t, store, 30)
	word := addVoicedLexeme(t, store, "ספר", "book", true)
	due, reviewed := clk.Now().Add(-time.Hour), clk.Now().Add(-48*time.Hour)
	readCard := addCardFor(t, store, userID, word, fsrs.SkillRead, due, int(fsrs.Good), reviewed)
	addCardFor(t, store, userID, word, fsrs.SkillRecall, clk.Now().Add(time.Hour), int(fsrs.Good), reviewed)
	lessonAttempts(t, svc, userID, word, fsrs.SkillRead, 6, 0)
	lessonAttempts(t, svc, userID, word, fsrs.SkillRecall, 6, 0)
	lessonAttempts(t, svc, userID, word, fsrs.SkillListen, 1, 5)

	sess := mustBuild(t, svc, userID)
	got := drain(t, sess)
	var ref *CardRef
	for i := range got {
		if got[i].CardID == readCard {
			ref = &got[i]
		}
	}
	if ref == nil {
		t.Fatalf("the due read card is not in the queue: %+v", got)
	}
	if ref.AskSkill == fsrs.SkillListen {
		t.Fatalf("ref = %+v: review must not route to the weakest skill", *ref)
	}
	if ref.Skill != fsrs.SkillRead || ref.LexemeID != word || ref.Intro {
		t.Fatalf("ref = %+v", *ref)
	}
}

func TestBuildSessionKeepsFlashcardsWhereTheLexiconCannotCheck(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	due, reviewed := clk.Now().Add(-time.Hour), clk.Now().Add(-48*time.Hour)
	// Words with no gloss and no clip: flashcard sentinel for promoteFlashcard
	// (not omitted — only listen-without-clip is).
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		res, err := store.ExecContext(ctx, `INSERT INTO lexemes (text_nfc) VALUES (?)`, fmt.Sprintf("bare-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		lex, err := res.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		addCardFor(t, store, userID, lex, fsrs.SkillRead, due, int(fsrs.Good), reviewed)
	}
	for _, ref := range drain(t, mustBuild(t, svc, userID)) {
		if ref.Activity != activity.Flashcard || ref.AskSkill != fsrs.SkillRead {
			t.Fatalf("ref = %+v, want a flashcard through read", ref)
		}
	}
}

func TestBuildSessionFollowsTheGate(t *testing.T) {
	svc, st, clk := openSvc(t, 1)
	svc.gate = func(context.Context, int64) (activity.Gate, error) { return activity.Gate{}, nil }
	userID := mustUser(t, st)
	fillPool(t, st, 30)
	word := addVoicedLexeme(t, st, "ספר", "book", true)
	addCardFor(t, st, userID, word, fsrs.SkillRecall, clk.Now().Add(-time.Hour), int(fsrs.Good), clk.Now().Add(-48*time.Hour))
	lessonAttempts(t, svc, userID, word, fsrs.SkillRecall, 1, 5)
	for _, ref := range drain(t, mustBuild(t, svc, userID)) {
		if ref.LexemeID == word && (ref.Activity != activity.Flashcard || ref.AskSkill != fsrs.SkillRecall) {
			t.Fatalf("ref = %+v: closed gate keeps a flashcard through the card skill", ref)
		}
	}
	// WithGate is the way a caller gives the real gate.
	asked := false
	svc2 := New(st, clk, 1, WithGate(func(context.Context, int64) (activity.Gate, error) {
		asked = true
		return activity.OpenGate(), nil
	}))
	for _, ref := range drain(t, mustBuild(t, svc2, userID)) {
		if ref.LexemeID == word && ref.AskSkill != fsrs.SkillRecall {
			t.Fatalf("ref = %+v: with the gate open the failing skill is asked", ref)
		}
	}
	if !asked {
		t.Fatal("the gate was not consulted")
	}
	failing := New(st, clk, 1, WithGate(func(context.Context, int64) (activity.Gate, error) {
		return activity.Gate{}, errors.New("gate down")
	}))
	if _, err := failing.BuildSession(context.Background(), userID); err == nil {
		t.Fatal("a failing gate must fail the build, not open everything")
	}
}

// Confusion rows of kind other feed the mix's confusion signal, through the learner model.
func TestBuildSessionReadsConfusionsFromTheLearnerModel(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	fillPool(t, store, 30)
	due, reviewed := clk.Now().Add(-time.Minute), clk.Now().Add(-time.Hour)
	heLex := addVoicedLexeme(t, store, "מבולבל", "confused", false)
	steadyLex := addVoicedLexeme(t, store, "יציב2", "steady", false)
	he := addCardFor(t, store, userID, heLex, fsrs.SkillRead, due, int(fsrs.Good), reviewed)
	steady := addCardFor(t, store, userID, steadyLex, fsrs.SkillRead, due, int(fsrs.Good), reviewed)
	if err := store.InsertConfusionMiss(ctx, userID, fsrs.MissOther, "אחר/מבולבל", heLex, clk.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	flags := map[int64]bool{}
	for _, ref := range drain(t, mustBuild(t, svc, userID)) {
		flags[ref.CardID] = ref.Struggling
	}
	if !flags[he] || flags[steady] {
		t.Fatalf("struggling = %v, want only the confused word", flags)
	}
	// A confusion about another learner's word does not count.
	other, err := store.InsertUser(ctx, "other@example.com", "sub-other", "learner")
	if err != nil {
		t.Fatal(err)
	}
	flags = map[int64]bool{}
	for _, ref := range drain(t, mustBuild(t, svc, other)) {
		flags[ref.CardID] = ref.Struggling
	}
	if len(flags) != 0 {
		t.Fatalf("the other learner has cards in the queue: %v", flags)
	}
}

func TestBuildSessionQueuesListenAndSpellCards(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	due, reviewed := clk.Now().Add(-time.Hour), clk.Now().Add(-48*time.Hour)
	fillPool(t, store, 30)
	listenWord := addVoicedLexeme(t, store, "שמיעה", "hearing", true)
	spellWord := addVoicedLexeme(t, store, "איות", "spelling", true)
	listen := addCardFor(t, store, userID, listenWord, fsrs.SkillListen, due, int(fsrs.Again), reviewed)
	spell := addCardFor(t, store, userID, spellWord, fsrs.SkillSpell, due, int(fsrs.Good), reviewed)
	sess := mustBuild(t, svc, userID)
	got := map[int64]CardRef{}
	for _, ref := range drain(t, sess) {
		got[ref.CardID] = ref
	}
	if got[listen].Skill != fsrs.SkillListen || !got[listen].Struggling {
		t.Fatalf("listen card = %+v, want queued as struggling", got[listen])
	}
	if got[spell].Skill != fsrs.SkillSpell || got[spell].Struggling || got[spell].Activity != activity.Dictation {
		t.Fatalf("spell card = %+v, want dictation", got[spell])
	}
	if sess.SkippedNoAudio != 0 {
		t.Fatalf("SkippedNoAudio = %d, want 0 when clips exist", sess.SkippedNoAudio)
	}
}

func TestBuildSessionMarksIntroForNewWordsNotYetShown(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	fillPool(t, store, 30)
	due := clk.Now().Add(-time.Hour)
	freshLex := addVoicedLexeme(t, store, "חדש", "new", false)
	shownLex := addVoicedLexeme(t, store, "מוכר", "familiar", false)
	fresh := addCardFor(t, store, userID, freshLex, fsrs.SkillRead, due, 0, time.Time{})
	shownCard := addCardFor(t, store, userID, shownLex, fsrs.SkillRead, due, 0, time.Time{})
	if err := svc.record.Expose(ctx, userID, []int64{shownLex}, evidence.ContextLessonIntro, 0); err != nil {
		t.Fatal(err)
	}
	got := map[int64]CardRef{}
	for _, ref := range drain(t, mustBuild(t, svc, userID)) {
		got[ref.CardID] = ref
	}
	if !got[fresh].New || !got[fresh].Intro {
		t.Fatalf("fresh = %+v, want a new card that needs its intro", got[fresh])
	}
	if !got[shownCard].New || got[shownCard].Intro {
		t.Fatalf("shown = %+v, want a new card with the intro already done", got[shownCard])
	}
}

func TestAnsweredClosesACheckedVisit(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	fillPool(t, store, 30)
	var ids []int64
	for i := 0; i < 4; i++ {
		w := addVoicedLexeme(t, store, fmt.Sprintf("מילה-%d", i), fmt.Sprintf("meaning%c", 'a'+rune(i)), true)
		ids = append(ids, addCardFor(t, store, userID, w, fsrs.SkillRead, clk.Now().Add(-time.Duration(i+1)*time.Hour), int(fsrs.Good), clk.Now().Add(-48*time.Hour)))
	}
	sess := mustBuild(t, svc, userID)
	first, ok := sess.Next()
	if !ok || first.Activity == "" || first.Activity == activity.Flashcard {
		t.Fatalf("first = %+v, %v: want a checked activity", first, ok)
	}
	if _, err := sess.Answered(ctx, first.CardID+1000, activity.Result{Correct: true}); !errors.Is(err, ErrNotServed) {
		t.Fatalf("err = %v, want ErrNotServed for a card that was not served", err)
	}
	if _, err := sess.Answered(ctx, first.CardID, activity.Result{Correct: true, Rating: fsrs.Good}); err != nil {
		t.Fatal(err)
	}
	if _, err := sess.Answered(ctx, first.CardID, activity.Result{Correct: true, Rating: fsrs.Good}); !errors.Is(err, ErrNotServed) {
		t.Fatalf("second Answered err = %v, want ErrNotServed", err)
	}
	_, total, _ := sess.Place()
	if total != len(ids) {
		t.Fatalf("total = %d after a right answer, want %d", total, len(ids))
	}

	// A wrong answer closes the visit. It is not asked again in this sitting
	// (ADR-0024 D3).
	second, _ := sess.Next()
	if _, err := sess.Answered(ctx, second.CardID, activity.Result{Correct: false}); err != nil {
		t.Fatal(err)
	}
	if _, total, _ = sess.Place(); total != len(ids) {
		t.Fatalf("total = %d after a wrong answer, want %d (no requeue)", total, len(ids))
	}
	rest := drainAll(t, sess)
	again := 0
	for _, ref := range rest {
		if ref.CardID == second.CardID {
			again++
		}
	}
	if again != 0 {
		t.Fatalf("the missed card came back %d times, want 0", again)
	}
	// In a fresh queue each card is asked once, however often it is answered wrong.
	sess2 := mustBuild(t, svc, userID)
	visits := map[int64]int{}
	for {
		next, ok := sess2.Next()
		if !ok {
			break
		}
		visits[next.CardID]++
		if _, err := sess2.Answered(ctx, next.CardID, activity.Result{Correct: false}); err != nil {
			t.Fatal(err)
		}
	}
	for id, n := range visits {
		if n != 1 {
			t.Fatalf("card %d was served %d times when always answered wrong, want 1", id, n)
		}
	}
	if len(visits) != len(ids) {
		t.Fatalf("%d cards served, want %d", len(visits), len(ids))
	}
	_ = ctx
}

func TestAnsweredRefusesFlashcards(t *testing.T) {
	// Review omits unaskable cards (ADR-0028 D6). Answered still refuses a
	// flashcard visit if one is present on the session.
	sess := &Session{
		userID: 1,
		last: CardRef{
			CardID: 7, LexemeID: 7, Skill: fsrs.SkillRead,
			Activity: activity.Flashcard, AskSkill: fsrs.SkillRead,
		},
		hasLast: true,
	}
	if _, err := sess.Answered(context.Background(), 7, activity.Result{Correct: true}); !errors.Is(err, ErrNotChecked) {
		t.Fatalf("err = %v, want ErrNotChecked", err)
	}
}

// A checked ref can still be graded as a flashcard, so a screen that does not know the
// activities yet keeps working.
func TestCheckedRefsCanStillBeGradedAsFlashcards(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	fillPool(t, store, 30)
	w := addVoicedLexeme(t, store, "ספר", "book", true)
	card := addCardFor(t, store, userID, w, fsrs.SkillRead, clk.Now().Add(-time.Hour), int(fsrs.Good), clk.Now().Add(-48*time.Hour))
	sess := mustBuild(t, svc, userID)
	ref, _ := sess.Next()
	if ref.CardID != card || ref.Activity == activity.Flashcard {
		t.Fatalf("ref = %+v", ref)
	}
	revealAndRecord(t, sess, context.Background(), card, fsrs.Good)
	logs := mustReviewLogs(t, store, card)
	if len(logs) != 2 {
		t.Fatalf("review_log = %+v", logs)
	}
}
