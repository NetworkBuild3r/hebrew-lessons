package lessons

import (
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestImportRefusesNotesAndZip(t *testing.T) {
	for _, in := range []Sheet{
		{Ref: "notes", ClassOn: "2026-11-01", Kind: "notes", Body: "tutor: שלום || peace"},
		{Ref: "pack.zip", ClassOn: "2026-11-01", Kind: "class", Body: "tutor: שלום || peace"},
		{Ref: "page.html", ClassOn: "2026-11-01", Kind: "class", Body: "tutor: שלום || peace"},
	} {
		got, err := ImportSheet(in, time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		if got.Passed {
			t.Fatalf("%s was accepted", in.Ref)
		}
	}
}

func TestImportKeepsTutorAndLearnerApart(t *testing.T) {
	got, err := ImportSheet(Sheet{
		Ref: "class-1", ClassOn: "2026-11-01", Kind: "class", Title: "Morning",
		Body: "tutor: שָׁלוֹם || peace\nlearner: בוקר\nnot a line\n",
	}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Passed || len(got.Lines) != 3 {
		t.Fatalf("import %+v", got)
	}
	if got.Lines[0].Tutor == "" || got.Lines[0].Gloss != "peace" || !got.Lines[0].HasPoints {
		t.Fatalf("tutor line %+v", got.Lines[0])
	}
	if got.Lines[1].Learner == "" || got.Lines[1].Tutor != "" {
		t.Fatalf("learner line %+v", got.Lines[1])
	}
	if !got.Lines[2].Held {
		t.Fatal("a line with no speaker was kept")
	}
}

func TestImportSameModifiedTimeChangesNothing(t *testing.T) {
	when := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	got, err := ImportSheet(Sheet{Ref: "class-1", ClassOn: "2026-11-01", Kind: "class", Modified: when, Body: "tutor: שלום || peace"}, when)
	if err != nil || !got.Unchanged || !got.Passed {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestAssembleWordLessonThenDropsAThinPassage(t *testing.T) {
	peace, water := "שָׁלוֹם", "מַיִם"
	in := AssembleInput{
		Lines: []Line{
			{Tutor: peace + " " + water, Gloss: "peace and water"},
			{Tutor: "מילה חדשה", Gloss: "a line that is not in the lexicon"},
		},
		Lexicon: []Word{noun(1, peace, "peace"), noun(2, water, "water")},
	}
	plan := Assemble(in)
	if plan.Kind != "word" || len(plan.Unit.NewWords) != 2 {
		t.Fatalf("kind %s words %v", plan.Kind, plan.Unit.NewWords)
	}
	if !hasType(plan, units.ActivityProduce) || !hasType(plan, units.ActivityOrder) {
		t.Fatalf("steps missing write or sentence: %+v", plan.Unit.Lessons[0].Activities)
	}
}

func TestAssembleAddsAMissedWordThePassageLeftOut(t *testing.T) {
	peace, water := "שָׁלוֹם", "מַיִם"
	missed := "תּוֹדָה"
	in := AssembleInput{
		Lines:   []Line{{Tutor: peace + " " + water, Gloss: "peace and water"}},
		Lexicon: []Word{noun(1, peace, "peace"), noun(2, water, "water")},
		Known:   Known{Taught: map[string]bool{missed: true}, Missed: map[string]bool{missed: true}},
	}
	plan := Assemble(in)
	found := false
	for _, a := range plan.Unit.Lessons[0].Activities {
		if a.Type == units.ActivityRecall && a.Text == missed {
			found = true
		}
	}
	if !found {
		t.Fatalf("missed word was not added: %+v", plan)
	}
}

func TestHeavyDayIsAnEasyLesson(t *testing.T) {
	peace := "שָׁלוֹם"
	in := AssembleInput{
		Heavy:   true,
		Lines:   []Line{{Tutor: peace}},
		Lexicon: []Word{noun(1, peace, "peace"), noun(2, "מַיִם", "water")},
		Known:   Known{Taught: map[string]bool{peace: true}},
	}
	plan := Assemble(in)
	if plan.Kind != "easy" || len(plan.Unit.NewWords) != 0 {
		t.Fatalf("%+v", plan)
	}
}

func TestCheckStoryRejectsLowCoverageAndAMissingDueWord(t *testing.T) {
	peace := "שָׁלוֹם"
	fails := CheckStory("brand new words", map[string]bool{peace: true}, nil, nil)
	if !hasFailure(fails, "story.coverage") && !hasFailure(fails, "story.no_hebrew") {
		t.Fatalf("%+v", fails)
	}
	fails = CheckStory(peace, map[string]bool{peace: true}, []string{"מַיִם"}, nil)
	if !hasFailure(fails, "story.missing_word") {
		t.Fatalf("due word was not reported: %+v", fails)
	}
	if got := CheckStory(peace+" "+peace, map[string]bool{peace: true}, []string{peace}, nil); len(got) != 0 {
		t.Fatalf("a good story failed: %+v", got)
	}
}

func hasType(p Plan, typ string) bool {
	for _, a := range p.Unit.Lessons[0].Activities {
		if a.Type == typ {
			return true
		}
	}
	return false
}

func hasFailure(list []StoryFailure, id string) bool {
	for _, f := range list {
		if f.ID == id {
			return true
		}
	}
	return false
}

func noun(id int64, text, gloss string) Word {
	return Word{ID: id, Text: text, Gloss: gloss, POS: "noun", Gender: "m"}
}

func TestMatchStripsOnePrefixAndRefusesTwoHits(t *testing.T) {
	peace := noun(1, "שָׁלוֹם", "peace")
	got, ok := Match("ושָׁלוֹם", []Word{peace})
	if !ok || got.ID != 1 {
		t.Fatalf("prefix match %+v %v", got, ok)
	}
	other := noun(2, "שָׁלוֹם", "hello")
	if _, ok := Match("שָׁלוֹם", []Word{peace, other}); ok {
		t.Fatal("two entries for one token were accepted")
	}
}

func TestEntryCheckNamesTheFailingCheck(t *testing.T) {
	w := noun(1, "שלום", "peace")
	if EntryCheck(w) != "entry.points" {
		t.Fatalf("%s", EntryCheck(w))
	}
	w = noun(1, "שָׁלוֹם", "peace <b>")
	if EntryCheck(w) != "entry.gloss" {
		t.Fatalf("%s", EntryCheck(w))
	}
}

func TestSharedGlossIsNotInTheSameLesson(t *testing.T) {
	a, b := noun(1, "שָׁלוֹם", "peace"), noun(2, "הֵי", "peace")
	c := noun(3, "מַיִם", "water")
	got := takeWords(rankWords([]Word{a, b, c}))
	if len(got) != 2 || got[1].Gloss == got[0].Gloss {
		t.Fatalf("%+v", got)
	}
}

func TestTodayOfferFollowsTheReviewCap(t *testing.T) {
	word := Plan{Kind: "word", Unit: units.Unit{Lessons: []units.Lesson{{Activities: []units.Activity{
		{Type: units.ActivityRecall, Text: "a"},
	}}}}}
	easy := Plan{Kind: "easy", Unit: units.Unit{Lessons: []units.Lesson{{Activities: []units.Activity{
		{Type: units.ActivityRecall, Text: "b"},
	}}}}}
	if got := TodayOffer(10, "2026-10-14", "Morning", word, easy); got.Kind != "word" || got.Label != "from class of 14 Oct" {
		t.Fatalf("%+v", got)
	}
	if got := TodayOffer(20, "2026-10-14", "Morning", word, easy); got.Kind != "easy" || !got.Heavy {
		t.Fatalf("%+v", got)
	}
	if got := TodayOffer(20, "2026-10-14", "Morning", Plan{}, Plan{}); got.Kind != "review" || got.Note == "" {
		t.Fatalf("%+v", got)
	}
}

func TestReadoutSaysTooEarlyAndSpeakingPrepListsTheTutor(t *testing.T) {
	lines := Readout(Week{Offered: 1, Started: 1})
	if len(lines) != 5 || !strings.Contains(lines[0], "Too early") {
		t.Fatalf("%v", lines)
	}
	page := SpeakingPrep("2026-10-14", []string{"שָׁלוֹם"}, []string{"שָׁלוֹם בּוֹקֶר"})
	if !strings.Contains(page, "שָׁלוֹם בּוֹקֶר") || !strings.Contains(page, "2026-10-14") {
		t.Fatal(page)
	}
}
