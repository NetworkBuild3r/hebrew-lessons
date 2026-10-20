package units

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Beginner pointed nouns. Genders are the lexicon values the fixtures store.
var vocab8 = []struct{ text, gender string }{
	{"שָׁלוֹם", "m"},
	{"בֹּקֶר", "m"},
	{"תּוֹדָה", "f"},
	{"מַיִם", "m"},
	{"לֶחֶם", "m"},
	{"חָלָב", "m"},
	{"סוּס", "m"},
	{"פָּרָה", "f"},
}

const (
	sus    = "סוּס"
	susim  = "סוּסִים"
	naal   = "נַעַל"
	gelida = "גְּלִידָה"
	kos    = "כּוֹס"
	david  = "דוד"
	bakar  = "בָּקָר"
)

type glex struct {
	ID              int64  `json:"id"`
	POS             string `json:"pos,omitempty"`
	Gender          string `json:"gender,omitempty"`
	Plural          string `json:"plural,omitempty"`
	IrregularPlural bool   `json:"irregularPlural,omitempty"`
	Bare            string `json:"bare,omitempty"`
	Gloss           string `json:"gloss,omitempty"`
}

type goldenLookup struct {
	Known   []string        `json:"known,omitempty"`
	Lexemes map[string]glex `json:"lexemes"`
	Audio   []string        `json:"audio"`
}

type storedIssue struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Pointer  string `json:"pointer"`
}

type goldenFile struct {
	Case        string        `json:"case"`
	Lookup      goldenLookup  `json:"lookup"`
	Issues      []storedIssue `json:"issues"`
	MsgContains []string      `json:"msgContains,omitempty"`
	MsgNot      []string      `json:"msgNot,omitempty"`
	Unit        Unit          `json:"unit"`
}

type goldenCase struct {
	file          string
	name          string
	unit          Unit
	lookup        goldenLookup
	issues        []Issue
	msgContains   []string
	msgNot        []string
	homographBare string
}

type fakeLookup struct {
	known []string
	lex   map[string]glex
	audio map[string]bool
	err   error
}

func (f fakeLookup) KnownLexemes(context.Context, int) (map[string]bool, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make(map[string]bool, len(f.known))
	for _, text := range f.known {
		out[text] = true
	}
	return out, nil
}

func (f fakeLookup) HasAudio(_ context.Context, text string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.audio[normalize(text)], nil
}

func (f fakeLookup) LexemeByText(_ context.Context, text string) (int64, bool, error) {
	if f.err != nil {
		return 0, false, f.err
	}
	e, ok := f.lex[normalize(text)]
	if !ok {
		return 0, false, nil
	}
	return e.ID, true, nil
}

func (f fakeLookup) Facts(_ context.Context, text string) (LexemeFacts, bool, error) {
	if f.err != nil {
		return LexemeFacts{}, false, f.err
	}
	e, ok := f.lex[normalize(text)]
	if !ok {
		return LexemeFacts{}, false, nil
	}
	return LexemeFacts{
		POS:             e.POS,
		Gender:          e.Gender,
		Plural:          e.Plural,
		IrregularPlural: e.IrregularPlural,
		Bare:            e.Bare,
		Gloss:           e.Gloss,
	}, true, nil
}

func (g goldenLookup) fake() fakeLookup {
	audio := map[string]bool{}
	for _, a := range g.Audio {
		audio[normalize(a)] = true
	}
	lex := map[string]glex{}
	for k, v := range g.Lexemes {
		lex[normalize(k)] = v
	}
	return fakeLookup{known: g.Known, lex: lex, audio: audio}
}

func v1() (Unit, fakeLookup) {
	c := validCase()
	return c.unit, c.lookup.fake()
}

func validCase() goldenCase {
	words := wordTexts(vocab8)
	u := assemble(words, [][]int{{0, 1}, {2, 3}, {4, 5}, {6, 7}}, evenRepeats(len(words), 3))
	return goldenCase{
		file:   "valid_8words.json",
		name:   "V1 valid 8-word unit. Four lessons introduce two words each. Three checkpoints follow so every new word can appear four times across three later lessons.",
		unit:   u,
		lookup: lookupFor(vocab8, u, nil),
	}
}

func TestGoldens(t *testing.T) {
	dir := filepath.Join("testdata")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	catalog := map[string]bool{}
	for _, r := range Rules {
		catalog[r.ID] = true
	}
	for _, c := range goldenCases() {
		t.Run(c.file, func(t *testing.T) {
			body := marshalGolden(c)
			path := filepath.Join(dir, c.file)
			if os.Getenv("WRITE_GOLDENS") == "1" {
				if err := os.WriteFile(path, body, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden: %v (WRITE_GOLDENS=1 writes it)", err)
			}
			if !bytes.Equal(bytes.TrimSpace(raw), bytes.TrimSpace(body)) {
				t.Fatalf("golden %s drifted from the fixture builder; run WRITE_GOLDENS=1", c.file)
			}
			var file goldenFile
			if err := json.Unmarshal(raw, &file); err != nil {
				t.Fatal(err)
			}
			got := Validate(context.Background(), &file.Unit, file.Lookup.fake())
			if !sameIssues(got, c.issues) {
				t.Fatalf("issues\n got %s\nwant %s", mustJSON(stripMsg(got)), mustJSON(c.issues))
			}
			for _, is := range got {
				if is.Rule == "" || is.Pointer == "" && is.Rule != RuleSchemaLookup || is.Msg == "" {
					t.Fatalf("incomplete issue %#v", is)
				}
				if is.Severity != SeverityBlock && is.Severity != SeverityWarn {
					t.Fatalf("severity %#v", is)
				}
				if !catalog[is.Rule] {
					t.Fatalf("issue rule %s is not in the catalog", is.Rule)
				}
				if !sortedPair(got) {
					t.Fatal("issues are not sorted by pointer, then rule")
				}
			}
			joined := joinMsgs(got)
			for _, sub := range c.msgContains {
				if !strings.Contains(joined, sub) {
					t.Fatalf("messages %q lack %q", joined, sub)
				}
			}
			for _, sub := range c.msgNot {
				if strings.Contains(joined, sub) {
					t.Fatalf("messages %q contain %q", joined, sub)
				}
			}
			if c.homographBare != "" {
				n := 0
				for _, e := range file.Lookup.Lexemes {
					if e.Bare == c.homographBare {
						n++
					}
				}
				if n < 2 {
					t.Fatalf("homograph bare %q is on %d lexemes", c.homographBare, n)
				}
			}
		})
	}
}

func goldenCases() []goldenCase {
	v := validCase()
	return []goldenCase{
		v,
		introducingListen(),
		introducingStory(),
		boundary23(),
		budgetUnit(),
		budgetLesson(),
		recycleCount(),
		recycleSpread(),
		recycleIntro(false),
		recycleIntro(true),
		coverageCase("story_89_9.json", "V8 story coverage 89.9% blocks", ActivityStory, 899, 101, SeverityBlock, "89.9%", "899/1000"),
		coverageCase("story_90.json", "V9 story coverage 90.0% warns", ActivityStory, 9, 1, SeverityWarn, "90.0%", "9/10"),
		coverageCase("story_94_9.json", "V9 story coverage 94.9% warns", ActivityStory, 949, 51, SeverityWarn, "94.9%", "949/1000"),
		coverageCase("story_95.json", "V10 story coverage 95.0% is clean", ActivityStory, 19, 1, "", "", ""),
		coverageCase("listen_89_9.json", "V11 listening coverage 89.9% blocks", ActivityListenPick, 899, 101, SeverityBlock, "89.9%", "899/1000"),
		coverageCase("listen_90.json", "V11 listening coverage 90.0% warns", ActivityListenPick, 9, 1, SeverityWarn, "90.0%", "9/10"),
		coverageCase("listen_94_9.json", "V11 listening coverage 94.9% warns", ActivityListenPick, 949, 51, SeverityWarn, "94.9%", "949/1000"),
		coverageCase("listen_95.json", "V11 listening coverage 95.0% is clean", ActivityListenPick, 19, 1, "", "", ""),
		coverageDetail(),
		unknownLexeme(),
		extraSpaces(),
		missingAudio(),
		schemaCase(),
		checkpointNew(),
		nounGender(),
		nounPlural(false),
		nounPlural(true),
		storyUnpointed(),
		storyUnmapped(),
		storyAcronym(),
	}
}

func introducingListen() goldenCase {
	u, _ := base()
	text := strings.Join(u.NewWords[:2], " ")
	u.Lessons[0].Activities = append([]Activity{{Type: ActivityListenPick, Text: text}}, u.Lessons[0].Activities...)
	return goldenCase{
		file:   "introducing_listen.json",
		name:   "A listen_pick that introduces its new words counts those words as known in that activity. Coverage is not 0%. Listening stays a passage, with the same thresholds as story.",
		unit:   u,
		lookup: lookupFor(vocab8, u, nil),
	}
}

func introducingStory() goldenCase {
	u, _ := base()
	text := strings.Join(u.NewWords[:2], " ")
	u.Lessons[0].Activities = append([]Activity{{Type: ActivityStory, Text: text}}, u.Lessons[0].Activities...)
	return goldenCase{
		file:   "introducing_story.json",
		name:   "A story that introduces its new words counts those words as known in that activity. Coverage is not 0%.",
		unit:   u,
		lookup: lookupFor(vocab8, u, nil),
	}
}

func boundary23() goldenCase {
	words := wordTexts(vocab8)
	u := assemble(words, [][]int{{0, 1, 2}, {3, 4, 5}, {6, 7}}, evenRepeats(len(words), 3))
	return goldenCase{
		file:   "boundary_2_and_3.json",
		name:   "V4 lessons introduce exactly 3, 3, and 2 new words. No budget issue.",
		unit:   u,
		lookup: lookupFor(vocab8, u, nil),
	}
}

func budgetUnit() goldenCase {
	voc := append(append([]struct{ text, gender string }{}, vocab8...), struct{ text, gender string }{kos, "f"})
	words := wordTexts(voc)
	u := assemble(words, [][]int{{0, 1, 2}, {3, 4, 5}, {6, 7, 8}}, evenRepeats(len(words), 3))
	return goldenCase{
		file:   "bad_budget_unit.json",
		name:   "V2 nine new words. Lesson loads stay at 3. The ninth word is the budget hit.",
		unit:   u,
		lookup: lookupFor(voc, u, nil),
		issues: []Issue{iss(RuleBudgetUnit, SeverityBlock, "/newWords/8")},
	}
}

func budgetLesson() goldenCase {
	words := wordTexts(vocab8)
	u := assemble(words, [][]int{{0, 1, 2, 3}, {4}, {5, 6, 7}}, evenRepeats(len(words), 3))
	return goldenCase{
		file:   "bad_budget_lesson.json",
		name:   "V3 one lesson introduces 4 new words (block) and another introduces 1 (warn). Exactly 2 or 3 stay quiet.",
		unit:   u,
		lookup: lookupFor(vocab8, u, nil),
		issues: []Issue{
			iss(RuleBudgetLesson, SeverityBlock, "/lessons/0"),
			iss(RuleBudgetLessonFew, SeverityWarn, "/lessons/1"),
		},
	}
}

func recycleCount() goldenCase {
	words := wordTexts(vocab8)
	reps := evenRepeats(len(words), 3)
	reps[0][0], reps[1][0], reps[2][0] = 1, 1, 1
	u := assemble(words, pairs(4), reps)
	return goldenCase{
		file:   "bad_recycle_count.json",
		name:   "V5 a new word appears 3 times across 3 later lessons.",
		unit:   u,
		lookup: lookupFor(vocab8, u, nil),
		issues: []Issue{iss(RuleRecycleCount, SeverityWarn, "/newWords/0")},
	}
}

func recycleSpread() goldenCase {
	words := wordTexts(vocab8)
	reps := evenRepeats(len(words), 3)
	reps[0][0], reps[1][0], reps[2][0] = 2, 2, 0
	u := assemble(words, pairs(4), reps)
	return goldenCase{
		file:   "bad_recycle_spread.json",
		name:   "V6 four later appearances sit in only two lessons.",
		unit:   u,
		lookup: lookupFor(vocab8, u, nil),
		issues: []Issue{iss(RuleRecycleSpread, SeverityWarn, "/newWords/0")},
	}
}

func recycleIntro(withTitle bool) goldenCase {
	words := wordTexts(vocab8)
	reps := evenRepeats(len(words), 3)
	reps[0][0], reps[1][0], reps[2][0] = 0, 0, 0
	u := assemble(words, pairs(4), reps)
	prependRecalls(&u, 0, words[0], 8)
	name := "V7 eight repeats in the introducing lesson do not count, and there are no later appearances."
	file := "bad_recycle_intro.json"
	issues := []Issue{
		iss(RuleRecycleCount, SeverityWarn, "/newWords/0"),
		iss(RuleRecycleSpread, SeverityWarn, "/newWords/0"),
	}
	if withTitle {
		u.Title = ""
		file = "bad_many.json"
		name = "V18 an empty title plus both recycling warns, sorted by pointer then rule."
		issues = append(issues, iss(RuleSchemaRequired, SeverityBlock, "/title"))
	}
	return goldenCase{
		file:   file,
		name:   name,
		unit:   u,
		lookup: lookupFor(vocab8, u, nil),
		issues: issues,
	}
}

func coverageCase(file, name, typ string, nKnown, nUnknown int, sev, pct, frac string) goldenCase {
	u, lex := base()
	text := repeatJoin(vocab8[0].text, nKnown) + " " + repeatJoin(naal, nUnknown)
	li, ai := addAct(&u, typ, text)
	lex = cloneLex(lex)
	lex[normalize(naal)] = glex{ID: 50, POS: "noun", Gender: "f", Bare: stripNiqqud(naal)}
	var issues []Issue
	var msg []string
	if sev != "" {
		rule := RuleCoverageStory
		if typ == ActivityListenPick {
			rule = RuleCoverageListening
		}
		issues = []Issue{iss(rule, sev, at("lessons", li, "activities", ai))}
		msg = []string{pct, frac}
	}
	return goldenCase{
		file:        file,
		name:        name,
		unit:        u,
		lookup:      lookupFor(vocab8, u, lex),
		issues:      issues,
		msgContains: msg,
	}
}

func coverageDetail() goldenCase {
	u, lex := base()
	text := strings.Join([]string{
		vocab8[0].text, vocab8[0].text, vocab8[0].text, vocab8[0].text, vocab8[0].text,
		vocab8[1].text, vocab8[1].text,
		david, "12", naal,
	}, ", ") + "."
	li, ai := addAct(&u, ActivityStory, text)
	lex = cloneLex(lex)
	lex[normalize(david)] = glex{ID: 60, POS: "name", Bare: david}
	lex[normalize(naal)] = glex{ID: 61, POS: "noun", Gender: "f", Bare: stripNiqqud(naal)}
	return goldenCase{
		file:        "coverage_detail.json",
		name:        "V12 coverage counts known lexemes, earlier new words, a name, and a number. Punctuation is not a token. 9/10 is 90.0%.",
		unit:        u,
		lookup:      lookupFor(vocab8, u, lex),
		issues:      []Issue{iss(RuleCoverageStory, SeverityWarn, at("lessons", li, "activities", ai))},
		msgContains: []string{"9/10", "90.0%"},
	}
}

func unknownLexeme() goldenCase {
	u, _ := base()
	ai := addActAt(&u, 0, ActivityRecall, gelida)
	return goldenCase{
		file:   "bad_lexeme_unknown.json",
		name:   "V13 an activity references a pointed word that is not in the lexicon.",
		unit:   u,
		lookup: lookupFor(vocab8, u, nil),
		issues: []Issue{iss(RuleLexemeUnknown, SeverityBlock, at("lessons", 0, "activities", ai, "text"))},
	}
}

func extraSpaces() goldenCase {
	u, _ := base()
	u.Lessons[0].Activities[3].Text = "  " + vocab8[0].text + "   " + vocab8[1].text + "  "
	return goldenCase{
		file:   "extra_spaces.json",
		name:   "V14 extra spaces are normalized before lookup, so they are not an unknown lexeme.",
		unit:   u,
		lookup: lookupFor(vocab8, u, nil),
	}
}

func missingAudio() goldenCase {
	u, _ := base()
	text := vocab8[0].text + "."
	ai := addActAt(&u, 0, ActivityRecall, text)
	lk := lookupFor(vocab8, u, nil)
	audio := lk.Audio[:0]
	for _, a := range lk.Audio {
		if a != normalize(text) {
			audio = append(audio, a)
		}
	}
	lk.Audio = audio
	return goldenCase{
		file:   "bad_audio_missing.json",
		name:   "V15 a Hebrew activity whose exact text has no audio clip.",
		unit:   u,
		lookup: lk,
		issues: []Issue{iss(RuleAudioMissing, SeverityBlock, at("lessons", 0, "activities", ai, "text"))},
	}
}

func schemaCase() goldenCase {
	u := Unit{
		Title:     "",
		Situation: "cafe",
		Concept:   "greetings",
		NewWords:  []string{vocab8[0].text},
		Lessons: []Lesson{{
			Kind: "quiz",
			Activities: []Activity{{
				Type: "flashcard",
				Text: "",
			}},
		}},
	}
	return goldenCase{
		file:   "bad_schema.json",
		name:   "V16 missing title, unknown activity type, lesson kind other than lesson or checkpoint, and a new word that never appears.",
		unit:   u,
		lookup: lookupFor(vocab8[:1], u, nil),
		issues: []Issue{
			iss(RuleSchemaRequired, SeverityBlock, "/lessons/0/activities/0/text"),
			iss(RuleSchemaActivity, SeverityBlock, "/lessons/0/activities/0/type"),
			iss(RuleSchemaKind, SeverityBlock, "/lessons/0/kind"),
			iss(RuleSchemaRef, SeverityBlock, "/newWords/0"),
			iss(RuleSchemaRequired, SeverityBlock, "/title"),
		},
	}
}

func checkpointNew() goldenCase {
	words := wordTexts(vocab8)
	u := assemble(words, [][]int{{0, 1}, {2, 3}, {4, 5}, {6}}, evenRepeats(len(words), 3))
	return goldenCase{
		file:   "bad_checkpoint_new.json",
		name:   "V17 a checkpoint introduces a new word. The lesson left with one word warns, and the late word cannot fill three later lessons.",
		unit:   u,
		lookup: lookupFor(vocab8, u, nil),
		issues: []Issue{
			iss(RuleBudgetLessonFew, SeverityWarn, "/lessons/3"),
			iss(RuleSchemaCheckpoint, SeverityBlock, "/lessons/4"),
			iss(RuleRecycleCount, SeverityWarn, "/newWords/7"),
			iss(RuleRecycleSpread, SeverityWarn, "/newWords/7"),
		},
	}
}

func nounGender() goldenCase {
	u, lex := base()
	e := lex[normalize(vocab8[0].text)]
	e.Gender = ""
	lex[normalize(vocab8[0].text)] = e
	return goldenCase{
		file:   "bad_noun_gender.json",
		name:   "A new noun has no gender.",
		unit:   u,
		lookup: lookupFor(vocab8, u, lex),
		issues: []Issue{iss(RuleNounGender, SeverityBlock, "/newWords/0")},
	}
}

func nounPlural(given bool) goldenCase {
	u, lex := base()
	e := lex[normalize(sus)]
	e.IrregularPlural = true
	e.Plural = susim
	lex[normalize(sus)] = e
	file := "bad_noun_plural.json"
	name := "A new noun is irregular in the lexicon and the unit never shows that plural."
	var issues []Issue
	if given {
		addAct(&u, ActivityRecall, susim)
		lex[normalize(susim)] = glex{ID: 70, POS: "noun", Gender: "m", Bare: stripNiqqud(susim)}
		file = "noun_plural_given.json"
		name = "The irregular plural appears as a token, so noun.plural stays quiet."
	} else {
		issues = []Issue{iss(RuleNounPlural, SeverityWarn, "/newWords/6")}
	}
	return goldenCase{
		file:   file,
		name:   name,
		unit:   u,
		lookup: lookupFor(vocab8, u, lex),
		issues: issues,
	}
}

func storyUnpointed() goldenCase {
	u, lex := base()
	text := vocab8[0].text + ", " + david + ", 3, בקר."
	li, ai := addAct(&u, ActivityStory, text)
	lex = cloneLex(lex)
	lex[normalize(david)] = glex{ID: 80, POS: "name", Bare: david}
	lex[normalize(bakar)] = glex{ID: 81, POS: "noun", Gender: "m", Bare: stripNiqqud(bakar)}
	return goldenCase{
		file:   "bad_story_unpointed.json",
		name:   "Unpointed Hebrew in a story blocks. The name, the number, and the punctuation do not. בקר is the bare form of two lexemes, and that homograph is not exempt. The bare token is also not a lexicon key, so it is story.unmapped as well as story.unpointed.",
		unit:   u,
		lookup: lookupFor(vocab8, u, lex),
		issues: []Issue{
			iss(RuleStoryUnmapped, SeverityBlock, at("lessons", li, "activities", ai, "text")),
			iss(RuleStoryUnpointed, SeverityBlock, at("lessons", li, "activities", ai, "text")),
		},
		msgContains:   []string{"בקר"},
		msgNot:        []string{david},
		homographBare: stripNiqqud(bakar),
	}
}

func storyAcronym() goldenCase {
	u, _ := base()
	const acronym = "צה״ל"
	text := repeatJoin(vocab8[0].text, 19) + " " + acronym
	li, ai := addAct(&u, ActivityStory, text)
	return goldenCase{
		file:   "bad_story_acronym.json",
		name:   "צה״ל is an acronym, not a gematria numeral, so an unmapped story token blocks as story.unmapped. It is also unpointed. Nineteen known words keep coverage at 95%.",
		unit:   u,
		lookup: lookupFor(vocab8, u, nil),
		issues: []Issue{
			iss(RuleStoryUnmapped, SeverityBlock, at("lessons", li, "activities", ai, "text")),
			iss(RuleStoryUnpointed, SeverityBlock, at("lessons", li, "activities", ai, "text")),
		},
		msgContains: []string{acronym},
	}
}

func storyUnmapped() goldenCase {
	u, lex := base()
	text := strings.Join([]string{gelida, david, "12"}, " ")
	li, ai := addAct(&u, ActivityStory, text)
	ri := addActAt(&u, li, ActivityRecall, gelida)
	lex = cloneLex(lex)
	lex[normalize(david)] = glex{ID: 90, POS: "name", Bare: david}
	lk := lookupFor(vocab8, u, lex)
	lk.Known = []string{gelida}
	return goldenCase{
		file:   "bad_story_unmapped.json",
		name:   "A story content word that is not a lexicon entry blocks as story.unmapped. The name and the number do not. The same word in a recall is lexeme.unknown, and the story token is not reported under both ids. KnownLexemes keeps coverage quiet.",
		unit:   u,
		lookup: lk,
		issues: []Issue{
			iss(RuleStoryUnmapped, SeverityBlock, at("lessons", li, "activities", ai, "text")),
			iss(RuleLexemeUnknown, SeverityBlock, at("lessons", li, "activities", ri, "text")),
		},
		msgContains: []string{gelida},
		msgNot:      []string{david, "12"},
	}
}

func base() (Unit, map[string]glex) {
	c := validCase()
	return c.unit, cloneLex(c.lookup.Lexemes)
}

func wordTexts(voc []struct{ text, gender string }) []string {
	out := make([]string, len(voc))
	for i, w := range voc {
		out[i] = w.text
	}
	return out
}

func pairs(n int) [][]int {
	groups := make([][]int, n)
	for i := 0; i < n; i++ {
		groups[i] = []int{i * 2, i*2 + 1}
	}
	return groups
}

func evenRepeats(nWords, nCP int) [][]int {
	reps := make([][]int, nCP)
	for c := 0; c < nCP; c++ {
		reps[c] = make([]int, nWords)
		for w := 0; w < nWords; w++ {
			if c == 0 {
				reps[c][w] = 2
			} else {
				reps[c][w] = 1
			}
		}
	}
	return reps
}

func assemble(words []string, groups [][]int, repeats [][]int) Unit {
	var lessons []Lesson
	for _, g := range groups {
		var acts []Activity
		for _, wi := range g {
			acts = append(acts, Activity{Type: ActivityRecall, Text: words[wi]})
		}
		if len(g) > 0 {
			text := joinIdx(words, g)
			acts = append(acts,
				Activity{Type: ActivityListenPick, Text: text},
				Activity{Type: ActivityStory, Text: text},
			)
		}
		lessons = append(lessons, Lesson{Kind: KindLesson, Activities: acts})
	}
	for _, row := range repeats {
		var acts []Activity
		for wi, n := range row {
			for k := 0; k < n; k++ {
				acts = append(acts, Activity{Type: ActivityRecall, Text: words[wi]})
			}
		}
		lessons = append(lessons, Lesson{Kind: KindCheckpoint, Activities: acts})
	}
	return Unit{
		Title:     "Cafe morning",
		Situation: "Ordering at a cafe",
		Concept:   "Greetings and food",
		NewWords:  append([]string(nil), words...),
		Lessons:   lessons,
	}
}

func joinIdx(words []string, idx []int) string {
	parts := make([]string, len(idx))
	for i, wi := range idx {
		parts[i] = words[wi]
	}
	return strings.Join(parts, " ")
}

func prependRecalls(u *Unit, lesson int, text string, n int) {
	extra := make([]Activity, n)
	for i := range extra {
		extra[i] = Activity{Type: ActivityRecall, Text: text}
	}
	u.Lessons[lesson].Activities = append(extra, u.Lessons[lesson].Activities...)
}

func addAct(u *Unit, typ, text string) (int, int) {
	li := len(u.Lessons) - 1
	ai := addActAt(u, li, typ, text)
	return li, ai
}

func addActAt(u *Unit, li int, typ, text string) int {
	u.Lessons[li].Activities = append(u.Lessons[li].Activities, Activity{Type: typ, Text: text})
	return len(u.Lessons[li].Activities) - 1
}

func repeatJoin(word string, n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = word
	}
	return strings.Join(parts, " ")
}

func lookupFor(voc []struct{ text, gender string }, u Unit, lex map[string]glex) goldenLookup {
	if lex == nil {
		lex = map[string]glex{}
		for i, w := range voc {
			lex[normalize(w.text)] = glex{
				ID:     int64(i + 1),
				POS:    "noun",
				Gender: w.gender,
				Bare:   stripNiqqud(w.text),
			}
		}
	}
	return goldenLookup{Lexemes: lex, Audio: audiosOf(u)}
}

func audiosOf(u Unit) []string {
	m := map[string]bool{}
	for _, les := range u.Lessons {
		for _, act := range les.Activities {
			n := normalize(act.Text)
			if n == "" {
				continue
			}
			m[n] = true
		}
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func cloneLex(m map[string]glex) map[string]glex {
	out := make(map[string]glex, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func iss(rule, sev, pointer string) Issue {
	return Issue{Rule: rule, Severity: sev, Pointer: pointer}
}

func stripMsg(in []Issue) []Issue {
	if len(in) == 0 {
		return nil
	}
	out := make([]Issue, len(in))
	for i, is := range in {
		out[i] = Issue{Rule: is.Rule, Severity: is.Severity, Pointer: is.Pointer}
	}
	return out
}

func sameIssues(got, want []Issue) bool {
	g := stripMsg(got)
	if len(g) == 0 && len(want) == 0 {
		return true
	}
	return reflect.DeepEqual(g, want)
}

func sortedPair(in []Issue) bool {
	for i := 1; i < len(in); i++ {
		if in[i].Pointer < in[i-1].Pointer {
			return false
		}
		if in[i].Pointer == in[i-1].Pointer && in[i].Rule < in[i-1].Rule {
			return false
		}
	}
	return true
}

func joinMsgs(in []Issue) string {
	var b strings.Builder
	for _, is := range in {
		b.WriteString(is.Msg)
		b.WriteByte('\n')
	}
	return b.String()
}

func marshalGolden(c goldenCase) []byte {
	file := goldenFile{
		Case:        c.name,
		Lookup:      c.lookup,
		Issues:      make([]storedIssue, 0, len(c.issues)),
		MsgContains: c.msgContains,
		MsgNot:      c.msgNot,
		Unit:        c.unit,
	}
	for _, is := range c.issues {
		file.Issues = append(file.Issues, storedIssue{Rule: is.Rule, Severity: is.Severity, Pointer: is.Pointer})
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		panic(err)
	}
	return append(raw, '\n')
}

func mustJSON(v any) string {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(raw)
}
