package units

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// INIT-030/SPEC-004 shape rules.

func TestStoryLengthPassAndFail(t *testing.T) {
	words := twelveWords()
	story := strings.Join(words, " ")
	f := BriefFacts{MinStoryTokens: 12}
	pass := Unit{Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
		{Type: ActivityStory, Text: story},
	}}}}
	if got := storyLengthIssues(&pass, &f); len(got) != 0 {
		t.Fatalf("12-word story: %v", ruleSet(got))
	}
	fail := Unit{Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
		{Type: ActivityStory, Text: words[0] + " " + words[1] + " " + words[2]},
	}}}}
	if got := storyLengthIssues(&fail, &f); len(got) != 1 || got[0].Rule != RuleBriefStoryLength {
		t.Fatalf("short story: %v", ruleSet(got))
	}
	// Dialogue lines together meet the floor.
	var lines []DialogueLine
	for i := 0; i < 6; i++ {
		lines = append(lines, DialogueLine{Speaker: SpeakerA, Text: words[i*2] + " " + words[i*2+1]})
	}
	lines[1].Speaker = SpeakerB
	dial := Unit{Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
		{Type: ActivityDialogue, Lines: lines},
	}}}}
	if got := storyLengthIssues(&dial, &f); len(got) != 0 {
		t.Fatalf("12-word dialogue: %v", ruleSet(got))
	}
	// Checkpoint is exempt.
	cp := Unit{Lessons: []Lesson{{Kind: KindCheckpoint, Activities: []Activity{
		{Type: ActivityRecall, Text: words[0]},
	}}}}
	if got := storyLengthIssues(&cp, &f); len(got) != 0 {
		t.Fatalf("checkpoint: %v", ruleSet(got))
	}
}

func TestDrillSharePassAndFail(t *testing.T) {
	w := vocab8[0].text
	pair := vocab8[0].text + " " + vocab8[1].text
	// 1 drill / 3 acts = 33% → pass.
	pass := Unit{Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
		{Type: ActivityShadow, Text: w},
		{Type: ActivityStory, Text: pair},
		{Type: ActivityCloze, Text: pair},
	}}}}
	if got := drillShareIssues(&pass, SeverityBlock); len(got) != 0 {
		t.Fatalf("under cap: %v", ruleSet(got))
	}
	// 3 drills / 4 acts = 75% → fail.
	fail := Unit{Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
		{Type: ActivityShadow, Text: w},
		{Type: ActivityListenPick, Text: w},
		{Type: ActivityRecall, Text: w},
		{Type: ActivityStory, Text: pair},
	}}}}
	if got := drillShareIssues(&fail, SeverityBlock); len(got) != 1 || got[0].Rule != RuleUnitDrillShare {
		t.Fatalf("over cap: %v", ruleSet(got))
	}
	// Without a brief, Validate warns.
	u, lk, _ := briefFixture()
	u.Lessons = fail.Lessons
	got := Validate(context.Background(), &u, lk)
	if !hasRule(got, RuleUnitDrillShare) {
		t.Fatalf("Validate should warn drill_share: %v", ruleSet(got))
	}
	for _, is := range got {
		if is.Rule == RuleUnitDrillShare && is.Severity != SeverityWarn {
			t.Fatalf("want warn without brief, got %s", is.Severity)
		}
	}
}

func TestPatternBriefPassAndFail(t *testing.T) {
	f := BriefFacts{NextPattern: "present-tense"}
	pass := Unit{Pattern: "present-tense"}
	if got := patternBriefIssues(&pass, &f); len(got) != 0 {
		t.Fatalf("matching pattern: %v", ruleSet(got))
	}
	fail := Unit{Pattern: "past-tense"}
	if got := patternBriefIssues(&fail, &f); len(got) != 1 || got[0].Rule != RuleBriefPattern {
		t.Fatalf("mismatch: %v", ruleSet(got))
	}
	cons := BriefFacts{Consolidate: true, ReusePatterns: []string{"present-tense", "past-tense"}}
	if got := patternBriefIssues(&Unit{Pattern: ""}, &cons); len(got) != 0 {
		t.Fatalf("consolidate empty: %v", ruleSet(got))
	}
	if got := patternBriefIssues(&Unit{Pattern: "past-tense"}, &cons); len(got) != 0 {
		t.Fatalf("consolidate reuse: %v", ruleSet(got))
	}
	if got := patternBriefIssues(&Unit{Pattern: "future-tense"}, &cons); len(got) != 1 || got[0].Rule != RuleBriefPattern {
		t.Fatalf("consolidate new pattern: %v", ruleSet(got))
	}
}

func TestRecyclePreviousPassAndFail(t *testing.T) {
	prev := []string{vocab8[0].text, vocab8[1].text, vocab8[2].text, vocab8[3].text, vocab8[4].text}
	f := BriefFacts{PreviousUnits: []BriefPreviousUnit{{NewWords: prev}}}
	story := strings.Join(prev[:4], " ")
	pass := Unit{Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
		{Type: ActivityStory, Text: story},
	}}}}
	if got := recyclePreviousIssues(&pass, &f); len(got) != 0 {
		t.Fatalf("four recycled: %v", ruleSet(got))
	}
	fail := Unit{Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
		{Type: ActivityStory, Text: prev[0] + " " + prev[1]},
	}}}}
	if got := recyclePreviousIssues(&fail, &f); len(got) != 1 || got[0].Rule != RuleBriefRecyclePrevious {
		t.Fatalf("two recycled: %v", ruleSet(got))
	}
	// Fewer than four previous words: all must appear.
	few := BriefFacts{PreviousUnits: []BriefPreviousUnit{{NewWords: prev[:2]}}}
	if got := recyclePreviousIssues(&fail, &few); len(got) != 0 {
		t.Fatalf("both of two: %v", ruleSet(got))
	}
	none := Unit{Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
		{Type: ActivityStory, Text: prev[0]},
	}}}}
	if got := recyclePreviousIssues(&none, &few); len(got) != 1 {
		t.Fatalf("one of two: %v", ruleSet(got))
	}
}

func TestConsolidationPassAndFail(t *testing.T) {
	f := BriefFacts{Consolidate: true}
	if got := consolidationIssues(&Unit{NewWords: nil}, &f); len(got) != 0 {
		t.Fatalf("empty: %v", ruleSet(got))
	}
	if got := consolidationIssues(&Unit{NewWords: []string{vocab8[0].text}}, &f); len(got) != 1 || got[0].Rule != RuleBriefConsolidation {
		t.Fatalf("with new words: %v", ruleSet(got))
	}
}

func TestSameShapePassAndFail(t *testing.T) {
	types := []string{ActivityShadow, ActivityStory, ActivityListenPick, ActivityRecall, ActivityCloze}
	f := BriefFacts{PreviousUnits: []BriefPreviousUnit{{ActivityTypes: types}}}
	// Identical sequence → warn (INIT-037/SPEC-005; ADR-0032 D5).
	fail := Unit{Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
		{Type: ActivityShadow}, {Type: ActivityStory}, {Type: ActivityListenPick},
		{Type: ActivityRecall}, {Type: ActivityCloze},
	}}}}
	if got := sameShapeIssues(&fail, &f); len(got) != 1 || got[0].Rule != RuleUnitSameShape || got[0].Severity != SeverityWarn {
		t.Fatalf("identical: %v", ruleSet(got))
	}
	// Replace every type → similarity 0 → no warn.
	pass := Unit{Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
		{Type: ActivityProduce}, {Type: ActivityProduce}, {Type: ActivityProduce},
		{Type: ActivityProduce}, {Type: ActivityProduce},
	}}}}
	if got := sameShapeIssues(&pass, &f); len(got) != 0 {
		t.Fatalf("different shape: %v", ruleSet(got))
	}
}

func TestUnits27And34SameShapeAndStoryRepeat(t *testing.T) {
	// INIT-037/SPEC-005: fixtures shaped like units 27 and 34 warn unit.same_shape
	// against each other; unit 34's story against unit 27's blocks unit.story_repeat.
	u27 := loadCloneFixture(t, "unit27_clone.json")
	u34 := loadCloneFixture(t, "unit34_clone.json")
	seq27 := activityTypeSeq(&u27)
	seq34 := activityTypeSeq(&u34)
	if len(seq27) == 0 || len(seq27) != len(seq34) {
		t.Fatalf("fixture sequences: 27=%v 34=%v", seq27, seq34)
	}
	for i := range seq27 {
		if seq27[i] != seq34[i] {
			t.Fatalf("fixture sequences differ at %d: %q vs %q", i, seq27[i], seq34[i])
		}
	}
	story27 := FirstStoryText(&u27)
	story34 := FirstStoryText(&u34)
	if story27 == "" || normalize(story27) != normalize(story34) {
		t.Fatalf("fixture stories must match: %q vs %q", story27, story34)
	}

	f34 := BriefFacts{PreviousUnits: []BriefPreviousUnit{{
		ActivityTypes: seq27,
		FirstStory:    story27,
	}}}
	if got := sameShapeIssues(&u34, &f34); len(got) != 1 || got[0].Rule != RuleUnitSameShape {
		t.Fatalf("unit 34 vs 27 same_shape: %v", ruleSet(got))
	}
	if got := sameShapeIssues(&u27, &BriefFacts{PreviousUnits: []BriefPreviousUnit{{ActivityTypes: seq34}}}); len(got) != 1 || got[0].Rule != RuleUnitSameShape {
		t.Fatalf("unit 27 vs 34 same_shape: %v", ruleSet(got))
	}
	varied := Unit{Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
		{Type: ActivityProduce}, {Type: ActivityProduce}, {Type: ActivityProduce},
		{Type: ActivityProduce}, {Type: ActivityProduce}, {Type: ActivityProduce},
		{Type: ActivityProduce},
	}}}}
	if got := sameShapeIssues(&varied, &f34); len(got) != 0 {
		t.Fatalf("varied sequence: %v", ruleSet(got))
	}

	got := StoryRepeatIssues(&u34, []string{story27}, ModeWrite)
	if len(got) != 1 || got[0].Rule != RuleUnitStoryRepeat || got[0].Severity != SeverityBlock {
		t.Fatalf("story_repeat ModeWrite: %v", got)
	}
	pub := StoryRepeatIssues(&u34, []string{story27}, ModePublished)
	if len(pub) != 1 || pub[0].Severity != SeverityWarn {
		t.Fatalf("story_repeat ModePublished: %v", pub)
	}
	// Without a brief: EarlierFirstStories on the unit still fires.
	u34.EarlierFirstStories = []string{story27}
	if issues := storyRepeatIssues(&u34, ModeWrite); len(issues) != 1 || issues[0].Rule != RuleUnitStoryRepeat {
		t.Fatalf("without brief: %v", issues)
	}
	// With a brief: FirstStory on PreviousUnits feeds the same check.
	u34.EarlierFirstStories = nil
	got = ValidateWithBriefIn(context.Background(), &u34, nil, &f34, ModeWrite)
	if !hasRule(got, RuleUnitStoryRepeat) {
		t.Fatalf("with brief story_repeat: %v", ruleSet(got))
	}
}

func loadCloneFixture(t *testing.T, name string) Unit {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Unit Unit `json:"unit"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	return file.Unit
}

func TestConsolidationBriefBlocksNewWordsAndPassesEmptyWithStory(t *testing.T) {
	// AC: consolidation brief blocks new words; passes empty newWords with a 12-word story.
	words := twelveWords()
	story := strings.Join(words, " ")
	lex := map[string]glex{}
	audio := []string{}
	for i, w := range words {
		lex[normalize(w)] = glex{ID: int64(i + 1), POS: "noun", Gender: "m", Bare: stripNiqqud(w), Gloss: "g"}
		audio = append(audio, normalize(w))
	}
	audio = append(audio, normalize(story))
	f := BriefFacts{
		Tier: "beginner", MinKnownShare: 0.60, MinStoryTokens: 12,
		Consolidate: true, BriefLexemeIDs: map[int64]bool{}, KnownLexemeIDs: map[int64]bool{},
	}
	for i := range words {
		f.KnownLexemeIDs[int64(i+1)] = true
		f.BriefLexemeIDs[int64(i+1)] = true
	}
	lk := goldenLookup{Lexemes: lex, Audio: audio}.fake()

	bad := Unit{
		Title: "Review", Situation: "At home", Concept: "Practise",
		NewWords: []string{words[0]},
		Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
			{Type: ActivityShadow, Text: words[0]},
			{Type: ActivityStory, Text: story},
			{Type: ActivityListenPick, Text: story, Translation: "A long review story."},
			{Type: ActivityCloze, Text: story, Target: words[0]},
			{Type: ActivitySay, Text: story, PromptEN: "Say the story."},
		}}},
	}
	got := ValidateWithBrief(context.Background(), &bad, lk, &f)
	if !hasRule(got, RuleBriefConsolidation) {
		t.Fatalf("expected consolidation block: %v", ruleSet(got))
	}

	good := bad
	good.NewWords = nil
	good.Lessons = []Lesson{{Kind: KindLesson, Activities: []Activity{
		{Type: ActivityStory, Text: story},
		{Type: ActivityListenPick, Text: story, Translation: "A long review story."},
		{Type: ActivityCloze, Text: story, Target: words[0]},
		{Type: ActivitySay, Text: story, PromptEN: "Say the story."},
		{Type: ActivityOrder, Text: strings.Join(words[:4], " "), Translation: "Four words."},
	}}}
	got = ValidateWithBrief(context.Background(), &good, lk, &f)
	for _, rule := range []string{RuleBriefConsolidation, RuleBriefStoryLength, RuleUnitDrillShare, RuleBudgetLessonFew} {
		if hasRule(got, rule) {
			t.Fatalf("consolidation pass hit %s: %v", rule, ruleSet(got))
		}
	}
}

func TestLearnerStyleUnitsFailStoryLengthAndDrillShare(t *testing.T) {
	// AC: fixtures shaped like learner 1 units 30 and 34 (short story, mostly
	// bare-word shadow/listen_pick/recall) fail brief.story_length and unit.drill_share.
	w := []string{vocab8[0].text, vocab8[1].text, vocab8[2].text}
	for _, name := range []string{"unit30", "unit34"} {
		t.Run(name, func(t *testing.T) {
			u := Unit{
				Title: name, Situation: "cafe", Concept: "drill",
				NewWords: []string{w[0], w[1]},
				Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
					{Type: ActivityShadow, Text: w[0]},
					{Type: ActivityShadow, Text: w[1]},
					{Type: ActivityListenPick, Text: w[0]},
					{Type: ActivityListenPick, Text: w[1]},
					{Type: ActivityRecall, Text: w[0]},
					{Type: ActivityRecall, Text: w[1]},
					{Type: ActivityProduce, Text: w[0]},
					{Type: ActivityStory, Text: w[0] + " " + w[1] + " " + w[2]}, // 3 words, under floor
				}}},
			}
			lex := map[string]glex{}
			audio := []string{}
			for i, text := range w {
				lex[normalize(text)] = glex{ID: int64(i + 1), POS: "noun", Gender: "m", Bare: stripNiqqud(text), Gloss: "g"}
				audio = append(audio, normalize(text))
			}
			for _, act := range u.Lessons[0].Activities {
				audio = append(audio, normalize(act.Text))
			}
			f := BriefFacts{
				Tier: "beginner", MinKnownShare: 0.60, MinStoryTokens: 12,
				NextPattern: "present-tense",
				NewWords: []BriefNewWord{
					{LexemeID: 1, Text: w[0], MinUses: 3},
					{LexemeID: 2, Text: w[1], MinUses: 3},
				},
				BriefLexemeIDs: map[int64]bool{1: true, 2: true, 3: true},
				KnownLexemeIDs: map[int64]bool{3: true},
			}
			u.Pattern = "present-tense"
			lk := goldenLookup{Lexemes: lex, Audio: audio}.fake()
			got := ValidateWithBrief(context.Background(), &u, lk, &f)
			if !hasRule(got, RuleBriefStoryLength) {
				t.Fatalf("want story_length: %v", ruleSet(got))
			}
			if !hasRule(got, RuleUnitDrillShare) {
				t.Fatalf("want drill_share: %v", ruleSet(got))
			}
			for _, is := range got {
				if is.Rule == RuleBriefStoryLength || is.Rule == RuleUnitDrillShare {
					if is.Severity != SeverityBlock {
						t.Fatalf("%s severity %s", is.Rule, is.Severity)
					}
				}
			}
		})
	}
}

func TestBudgetLessonFewSuppressedOnConsolidate(t *testing.T) {
	words := twelveWords()
	story := strings.Join(words, " ")
	lex := map[string]glex{}
	audio := []string{normalize(story)}
	for i, w := range words {
		lex[normalize(w)] = glex{ID: int64(i + 1), POS: "noun", Gender: "m", Bare: stripNiqqud(w), Gloss: "g"}
		audio = append(audio, normalize(w))
	}
	f := BriefFacts{
		Tier: "beginner", MinKnownShare: 0.60, MinStoryTokens: 12,
		Consolidate: true, BriefLexemeIDs: map[int64]bool{}, KnownLexemeIDs: map[int64]bool{},
	}
	for i := range words {
		f.KnownLexemeIDs[int64(i+1)] = true
		f.BriefLexemeIDs[int64(i+1)] = true
	}
	u := Unit{
		Title: "Review", Situation: "home", Concept: "practise",
		Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
			{Type: ActivityStory, Text: story},
			{Type: ActivityListenPick, Text: story, Translation: "The review story."},
			{Type: ActivityCloze, Text: story, Target: words[0]},
			{Type: ActivitySay, Text: story, PromptEN: "Say it."},
			{Type: ActivityOrder, Text: strings.Join(words[:4], " "), Translation: "Four words."},
		}}},
	}
	lk := goldenLookup{Lexemes: lex, Audio: audio}.fake()
	got := ValidateWithBrief(context.Background(), &u, lk, &f)
	if hasRule(got, RuleBudgetLessonFew) {
		t.Fatalf("consolidation should suppress budget.lesson.few: %v", ruleSet(got))
	}
}

func twelveWords() []string {
	// Two copies of vocab8 yield 16; take 12.
	out := make([]string, 0, 12)
	for _, w := range vocab8 {
		out = append(out, w.text)
		if len(out) == 12 {
			break
		}
	}
	for len(out) < 12 {
		out = append(out, vocab8[len(out)%len(vocab8)].text)
	}
	return out[:12]
}
