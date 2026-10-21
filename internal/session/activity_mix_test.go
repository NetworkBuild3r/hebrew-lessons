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

// ac-2: the mix asks a word through the skill where its accuracy is lowest.
func TestMixPicksListenPickForTheWeakestListenAccuracy(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	got := mix([]Candidate{modelCand(1, fsrs.SkillRead, now.Add(-time.Hour))}, TargetSuccess, now, 1)
	if len(got) != 1 {
		t.Fatalf("queue = %+v", got)
	}
	ref := got[0]
	if ref.Activity != activity.ListenPick || ref.AskSkill != fsrs.SkillListen {
		t.Fatalf("ref = %+v, want listen_pick through listen", ref)
	}
	// The card the mix chose is still the due read card, which is what a flashcard grades.
	if ref.CardID != 1 || ref.Skill != fsrs.SkillRead || ref.LexemeID != 1 {
		t.Fatalf("ref = %+v, want the due read card", ref)
	}
}

func TestMixAsksEachWordThroughItsOwnWeakestSkill(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	due := now.Add(-time.Hour)
	build := func(id int64, weakest string) Candidate {
		var views []activity.SkillView
		for _, s := range activity.Skills {
			acc := 0.9
			if s == weakest {
				acc = 0.2
			}
			views = append(views, skillView(s, learner.Learning, acc, 8))
		}
		c := knownCand(id, due.Add(time.Duration(id)*time.Minute))
		c.Skills, c.Facts, c.Exposed = views, richFacts, true
		return c
	}
	got := mix([]Candidate{build(1, "listen"), build(2, "read"), build(3, "recall"), build(4, "spell")}, TargetSuccess, now, 1)
	want := map[int64]activity.Kind{1: activity.ListenPick, 2: activity.ReadPick, 3: activity.TypeIt, 4: activity.Dictation}
	if len(got) != 4 {
		t.Fatalf("queue = %+v", got)
	}
	for _, ref := range got {
		if ref.Activity != want[ref.CardID] {
			t.Errorf("card %d: activity %q, want %q", ref.CardID, ref.Activity, want[ref.CardID])
		}
		if spec, _ := activity.SpecOf(ref.Activity); spec.Skill != ref.AskSkill {
			t.Errorf("card %d: skill %q does not match activity %q", ref.CardID, ref.AskSkill, ref.Activity)
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
		fitsAny := map[int64]bool{}
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
			for _, s := range activity.Skills {
				if len(activity.Fitting(s, c.Facts)) > 0 {
					fitsAny[id] = true
				}
			}
		}
		for _, ref := range mix(cands, TargetSuccess, now, int64(round)) {
			if ref.Kind == KindLetter || ref.Kind == KindPattern {
				continue
			}
			if fitsAny[ref.CardID] && ref.Activity == activity.Flashcard {
				t.Fatalf("round %d: card %d is a flashcard although a checked activity fits", round, ref.CardID)
			}
			if !fitsAny[ref.CardID] && (ref.Activity != activity.Flashcard || ref.AskSkill != ref.Skill) {
				t.Fatalf("round %d: card %d has %+v but nothing fits: want a flashcard through its own skill", round, ref.CardID, ref)
			}
		}
	}
}

func TestMixKeepsTheFlashcardWhereNothingCanBeChecked(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	c := modelCand(1, fsrs.SkillRecall, now.Add(-time.Hour))
	c.Facts = activity.Facts{} // no gloss and no clip
	noModel := knownCand(2, now.Add(-time.Minute))
	got := mix([]Candidate{c, noModel}, TargetSuccess, now, 1)
	if len(got) != 2 {
		t.Fatalf("queue = %+v", got)
	}
	for _, ref := range got {
		if ref.Activity != activity.Flashcard || ref.AskSkill != ref.Skill {
			t.Fatalf("ref = %+v, want a flashcard through the card's skill", ref)
		}
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
	want := []activity.Kind{
		activity.ListenPick, activity.ListenPick, activity.AudioMatch,
		activity.ListenPick, activity.ListenPick, activity.AudioMatch,
		activity.ListenPick, activity.ListenPick, activity.AudioMatch,
	}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("activities = %v, want %v", kinds, want)
	}
}

// A word that comes up twice is asked through two skills, not the same one twice.
func TestMixAsksARepeatedWordThroughAnotherSkill(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	he := modelCand(1, fsrs.SkillRead, now.Add(-2*time.Hour))
	en := modelCand(2, fsrs.SkillRecall, now.Add(-time.Hour))
	en.LexemeID = he.LexemeID
	got := mix([]Candidate{he, en}, TargetSuccess, now, 1)
	if len(got) != 2 {
		t.Fatalf("queue = %+v", got)
	}
	if got[0].AskSkill == got[1].AskSkill {
		t.Fatalf("one word asked through %q twice: %+v", got[0].AskSkill, got)
	}
	if got[0].AskSkill != fsrs.SkillListen || got[1].AskSkill != fsrs.SkillSpell {
		t.Fatalf("skills = %q then %q, want weakest first: listen then spell", got[0].AskSkill, got[1].AskSkill)
	}
}

func TestMixHonoursTheGate(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	c := modelCand(1, fsrs.SkillRecall, now.Add(-time.Hour))
	// Read is the weakest skill, but the letters check has not been passed.
	c.Skills = []activity.SkillView{
		skillView("listen", learner.Known, 0.95, 8), skillView("read", learner.Struggling, 0.1, 8),
		skillView("recall", learner.Struggling, 0.1, 8), skillView("spell", learner.Struggling, 0.1, 8),
	}
	closed := mixGate([]Candidate{c}, TargetSuccess, now, now, 1, activity.Gate{})
	if closed[0].Activity != activity.ListenPick || closed[0].AskSkill != fsrs.SkillListen {
		t.Fatalf("before the gate: %+v, want listening only", closed[0])
	}
	opened := mixGate([]Candidate{c}, TargetSuccess, now, now, 1, activity.OpenGate())
	if opened[0].AskSkill == fsrs.SkillListen {
		t.Fatalf("after the gate: %+v, want a failing skill", opened[0])
	}
	// With no clip, a closed gate leaves nothing to check: the card is a flashcard.
	c.Facts.HasAudio = false
	flash := mixGate([]Candidate{c}, TargetSuccess, now, now, 1, activity.Gate{})
	if flash[0].Activity != activity.Flashcard || flash[0].AskSkill != fsrs.SkillRecall {
		t.Fatalf("no clip, closed gate: %+v", flash[0])
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
			"good later":                    {func(c *Candidate) { c.Due = later }, kindKnown},
			"again":                         {func(c *Candidate) { c.Latest = int(fsrs.Again) }, kindStruggle},
			"lapses":                        {func(c *Candidate) { c.Lapses = StruggleLapses }, kindStruggle},
			"confused":                      {func(c *Candidate) { c.Confused = true }, kindStruggle},
			"unreviewed":                    {func(c *Candidate) { c.HasRating = false }, kindNew},
			"recent lapse":                  {func(c *Candidate) { c.RecentLapse = true }, kindSkip},
		} {
			c := base
			tc.edit(&c)
			if got := classify(c, now, now); got != tc.want {
				t.Errorf("%s card, %s: kind %d, want %d", skill, name, got, tc.want)
			}
		}
	}
	if got := classify(Candidate{Skill: "speak", Due: due, HasRating: true, Latest: int(fsrs.Good)}, now, now); got != kindSkip {
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

func TestBuildSessionAsksThroughTheWeakestSkill(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	fillPool(t, store, 30)
	word := addVoicedLexeme(t, store, "ספר", "book", true)
	due, reviewed := clk.Now().Add(-time.Hour), clk.Now().Add(-48*time.Hour)
	readCard := addCardFor(t, store, userID, word, fsrs.SkillRead, due, int(fsrs.Good), reviewed)
	// The recall card is not due yet. It has one good review, as the read card has.
	addCardFor(t, store, userID, word, fsrs.SkillRecall, clk.Now().Add(time.Hour), int(fsrs.Good), reviewed)
	// Reading and recall are fine. Listening is failing: one right of six.
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
	if ref.Activity != activity.ListenPick || ref.AskSkill != fsrs.SkillListen {
		t.Fatalf("ref = %+v, want listen_pick through listen", *ref)
	}
	if ref.Skill != fsrs.SkillRead || ref.LexemeID != word || ref.Intro {
		t.Fatalf("ref = %+v", *ref)
	}
}

func TestBuildSessionKeepsFlashcardsWhereTheLexiconCannotCheck(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	due, reviewed := clk.Now().Add(-time.Hour), clk.Now().Add(-48*time.Hour)
	// Words with no gloss and no clip, as every v2 word is.
	var cards []int64
	for i := 0; i < 5; i++ {
		cards = append(cards, addCard(t, store, userID, fmt.Sprintf("bare-%d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false))
	}
	for _, ref := range drain(t, mustBuild(t, svc, userID)) {
		if ref.Activity != activity.Flashcard || ref.AskSkill != fsrs.SkillRead {
			t.Fatalf("ref = %+v, want a flashcard through read", ref)
		}
	}
	_ = cards
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
		if ref.LexemeID == word && (ref.Activity != activity.ListenPick || ref.AskSkill != fsrs.SkillListen) {
			t.Fatalf("ref = %+v: before the letters check only listening is asked", ref)
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
	due, reviewed := clk.Now().Add(-time.Minute), clk.Now().Add(-time.Hour)
	he := addCard(t, store, userID, "מבולבל", fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)
	steady := addCard(t, store, userID, "יציב", fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)
	lexeme := lexemeOf(t, store, he)
	if err := store.InsertConfusionMiss(ctx, userID, fsrs.MissOther, "אחר/מבולבל", lexeme, clk.Now().Add(-time.Hour)); err != nil {
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
	listen := addCard(t, store, userID, "שמיעה", fsrs.SkillListen, due, 0, int(fsrs.Again), reviewed, false)
	spell := addCard(t, store, userID, "איות", fsrs.SkillSpell, due, 0, int(fsrs.Good), reviewed, false)
	got := map[int64]CardRef{}
	for _, ref := range drain(t, mustBuild(t, svc, userID)) {
		got[ref.CardID] = ref
	}
	if got[listen].Skill != fsrs.SkillListen || !got[listen].Struggling {
		t.Fatalf("listen card = %+v, want queued as struggling", got[listen])
	}
	if got[spell].Skill != fsrs.SkillSpell || got[spell].Struggling {
		t.Fatalf("spell card = %+v", got[spell])
	}
}

func TestBuildSessionMarksIntroForNewWordsNotYetShown(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	due := clk.Now().Add(-time.Hour)
	fresh := addCard(t, store, userID, "חדש", fsrs.SkillRead, due, 0, 0, time.Time{}, false)
	shownCard := addCard(t, store, userID, "מוכר", fsrs.SkillRead, due, 0, 0, time.Time{}, false)
	if err := svc.record.Expose(ctx, userID, []int64{lexemeOf(t, store, shownCard)}, evidence.ContextLessonIntro, 0); err != nil {
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
	if err := sess.Answered(first.CardID+1000, true); !errors.Is(err, ErrNotServed) {
		t.Fatalf("err = %v, want ErrNotServed for a card that was not served", err)
	}
	if err := sess.Answered(first.CardID, true); err != nil {
		t.Fatal(err)
	}
	if err := sess.Answered(first.CardID, true); !errors.Is(err, ErrNotServed) {
		t.Fatalf("second Answered err = %v, want ErrNotServed", err)
	}
	_, total, _ := sess.Place()
	if total != len(ids) {
		t.Fatalf("total = %d after a right answer, want %d", total, len(ids))
	}

	// A wrong answer puts the visit back once.
	second, _ := sess.Next()
	if err := sess.Answered(second.CardID, false); err != nil {
		t.Fatal(err)
	}
	if _, total, _ = sess.Place(); total != len(ids)+1 {
		t.Fatalf("total = %d after a wrong answer, want one more visit", total)
	}
	rest := drainAll(t, sess)
	again := 0
	for _, ref := range rest {
		if ref.CardID == second.CardID {
			again++
		}
	}
	if again != 1 {
		t.Fatalf("the missed card came back %d times, want once", again)
	}
	// Answering it wrong a second time does not queue it again: in a fresh queue the card is
	// served twice at most, however often it is answered wrong.
	sess2 := mustBuild(t, svc, userID)
	visits := map[int64]int{}
	for {
		next, ok := sess2.Next()
		if !ok {
			break
		}
		visits[next.CardID]++
		if err := sess2.Answered(next.CardID, false); err != nil {
			t.Fatal(err)
		}
	}
	for id, n := range visits {
		if n != 2 {
			t.Fatalf("card %d was served %d times when always answered wrong, want 2", id, n)
		}
	}
	if len(visits) != len(ids) {
		t.Fatalf("%d cards served, want %d", len(visits), len(ids))
	}
	_ = ctx
}

func TestAnsweredRefusesFlashcards(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	addCard(t, store, userID, "בלי-תשובה", fsrs.SkillRead, clk.Now().Add(-time.Hour), 0, int(fsrs.Good), clk.Now().Add(-48*time.Hour), false)
	sess := mustBuild(t, svc, userID)
	card, _ := sess.Next()
	if card.Activity != activity.Flashcard {
		t.Fatalf("card = %+v", card)
	}
	if err := sess.Answered(card.CardID, true); !errors.Is(err, ErrNotChecked) {
		t.Fatalf("err = %v, want ErrNotChecked", err)
	}
	// The serve is untouched: the flashcard can still be graded as before.
	revealAndRecord(t, sess, context.Background(), card.CardID, fsrs.Good)
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
