package units

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestTranslationRulesAreCataloged(t *testing.T) {
	doc, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		RuleUnitTranslationMissing:     SeverityBlock,
		RuleUnitTranslationLang:        SeverityBlock,
		RuleUnitTranslationLength:      SeverityWarn,
		RuleUnitTranslationDistractors: SeverityWarn,
	}
	var order []string
	seen := map[string]Rule{}
	for _, r := range Rules {
		order = append(order, r.ID)
		if _, ok := seen[r.ID]; ok {
			t.Fatalf("duplicate rule %s", r.ID)
		}
		seen[r.ID] = r
	}
	if !slices.IsSorted(order) {
		t.Fatalf("catalog is not sorted: %v", order)
	}
	for id, sev := range want {
		r, ok := seen[id]
		if !ok {
			t.Errorf("catalog has no %s", id)
			continue
		}
		if r.Severity != sev {
			t.Errorf("%s severity %s, want %s", id, r.Severity, sev)
		}
		if !strings.Contains(string(doc), id) {
			t.Errorf("doc.go does not list %s", id)
		}
	}
}

func TestTranslationMissing(t *testing.T) {
	ctx := context.Background()
	u, lk := v1()
	if got := byRule(Validate(ctx, &u, lk), RuleUnitTranslationMissing); len(got) != 0 {
		t.Fatalf("fixture %s", mustJSON(got))
	}
	li, ai := firstSentence(t, u)
	line := &u.Lessons[li].Activities[ai]
	line.Translation = ""
	wantPtr := at("lessons", li, "activities", ai, "translation")
	wantMsg := fmt.Sprintf("activity %d: this listen_pick has several words in the lexicon; set translation to the English of the whole line.", ai+1)

	got := byRule(Validate(ctx, &u, lk), RuleUnitTranslationMissing)
	if len(got) != 1 || got[0].Severity != SeverityBlock || got[0].Pointer != wantPtr || got[0].Msg != wantMsg {
		t.Fatalf("missing %#v", got)
	}
	pub := byRule(ValidateIn(ctx, &u, lk, ModePublished), RuleUnitTranslationMissing)
	if len(pub) != 1 || pub[0].Severity != SeverityWarn || pub[0].Pointer != wantPtr || pub[0].Msg != wantMsg {
		t.Fatalf("published recheck %#v", pub)
	}
	for _, is := range ValidateIn(ctx, &u, lk, ModePublished) {
		if is.Severity == SeverityBlock {
			t.Fatalf("published recheck blocked: %+v", is)
		}
	}

	line.Translation = "  \t"
	got = byRule(Validate(ctx, &u, lk), RuleUnitTranslationMissing)
	if len(got) != 1 || got[0].Severity != SeverityBlock {
		t.Fatalf("whitespace %#v", got)
	}

	line.Translation = "good morning"
	if got := byRule(Validate(ctx, &u, lk), RuleUnitTranslationMissing); len(got) != 0 {
		t.Fatalf("filled %#v", got)
	}

	// One lexicon token, even beside an unknown word or a numeral, needs no translation.
	line.Text = vocab8[0].text + " גָּר"
	line.Target = vocab8[0].text
	line.Translation = ""
	if got := byRule(Validate(ctx, &u, lk), RuleUnitTranslationMissing); len(got) != 0 {
		t.Fatalf("one lexicon token %#v", got)
	}
	line.Text = vocab8[0].text + " 12"
	if got := byRule(Validate(ctx, &u, lk), RuleUnitTranslationMissing); len(got) != 0 {
		t.Fatalf("numeral %#v", got)
	}

	// The unpointed spelling does not count, the same as a graded target.
	u, lk = v1()
	li, ai = firstSentence(t, u)
	act := u.Lessons[li].Activities[ai]
	toks := tokenize(act.Text)
	if len(toks) < 2 {
		t.Fatal(toks)
	}
	lk.lex = map[string]glex{}
	for i, tok := range toks[:2] {
		bare := stripNiqqud(tok)
		lk.lex[bare] = glex{ID: int64(i + 1), POS: "noun", Gender: "m", Bare: bare}
	}
	act.Translation = ""
	u.Lessons[li].Activities[ai] = act
	if got := byRule(Validate(ctx, &u, lk), RuleUnitTranslationMissing); len(got) != 0 {
		t.Fatalf("unpointed lexicon %#v", got)
	}
}

func TestTranslationLang(t *testing.T) {
	ctx := context.Background()
	u, lk := v1()
	li, ai := firstSentence(t, u)
	ptr := at("lessons", li, "activities", ai, "translation")
	for _, bad := range []string{"שָׁלוֹם", "a\u05B0", "<", ">", "javascript:alert(1)", "JAVASCRIPT:x", "\uFF1C"} {
		u.Lessons[li].Activities[ai].Translation = bad
		got := byRule(Validate(ctx, &u, lk), RuleUnitTranslationLang)
		if len(got) != 1 || got[0].Severity != SeverityBlock || got[0].Pointer != ptr || !strings.HasSuffix(got[0].Msg, ".") {
			t.Fatalf("%q %#v", bad, got)
		}
		pub := byRule(ValidateIn(ctx, &u, lk, ModePublished), RuleUnitTranslationLang)
		if len(pub) != 1 || pub[0].Severity != SeverityBlock {
			t.Fatalf("published %q %#v", bad, pub)
		}
	}
	u.Lessons[li].Activities[ai].Translation = "good morning"
	if got := byRule(Validate(ctx, &u, lk), RuleUnitTranslationLang); len(got) != 0 {
		t.Fatalf("english %#v", got)
	}

	u.Lessons[0].Activities = append(u.Lessons[0].Activities, Activity{
		Type: ActivityDialogue,
		Lines: []DialogueLine{
			{Speaker: SpeakerA, Text: vocab8[0].text, Translation: "hello ש"},
			{Speaker: SpeakerB, Text: vocab8[1].text, Translation: "morning"},
		},
	})
	di := len(u.Lessons[0].Activities) - 1
	got := byRule(Validate(ctx, &u, lk), RuleUnitTranslationLang)
	want := at("lessons", 0, "activities", di, "lines", 0, "translation")
	if len(got) != 1 || got[0].Pointer != want || got[0].Severity != SeverityBlock {
		t.Fatalf("line %#v", got)
	}

	// A recall may carry a translation, and Hebrew there is the same block.
	u, lk = v1()
	u.Lessons[0].Activities[0].Translation = "ש"
	got = byRule(Validate(ctx, &u, lk), RuleUnitTranslationLang)
	if len(got) != 1 || got[0].Pointer != at("lessons", 0, "activities", 0, "translation") {
		t.Fatalf("recall %#v", got)
	}
}

func TestTranslationLength(t *testing.T) {
	ctx := context.Background()
	u, lk := v1()
	li, ai := firstSentence(t, u)
	ptr := at("lessons", li, "activities", ai, "translation")
	u.Lessons[li].Activities[ai].Translation = strings.Repeat("a", TranslationMaxLen)
	if got := byRule(Validate(ctx, &u, lk), RuleUnitTranslationLength); len(got) != 0 {
		t.Fatalf("120 %#v", got)
	}
	u.Lessons[li].Activities[ai].Translation = strings.Repeat("a", TranslationMaxLen+1)
	got := byRule(Validate(ctx, &u, lk), RuleUnitTranslationLength)
	if len(got) != 1 || got[0].Severity != SeverityWarn || got[0].Pointer != ptr || !strings.Contains(got[0].Msg, "120") {
		t.Fatalf("121 %#v", got)
	}
	// Hebrew over the limit is both rules.
	u.Lessons[li].Activities[ai].Translation = strings.Repeat("א", TranslationMaxLen+1)
	if len(byRule(Validate(ctx, &u, lk), RuleUnitTranslationLength)) != 1 || len(byRule(Validate(ctx, &u, lk), RuleUnitTranslationLang)) != 1 {
		t.Fatal("hebrew over the limit should warn and block")
	}

	u, lk = v1()
	u.Lessons[0].Activities = append(u.Lessons[0].Activities, Activity{
		Type: ActivityDialogue,
		Lines: []DialogueLine{
			{Speaker: SpeakerA, Text: vocab8[0].text, Translation: strings.Repeat("b", TranslationMaxLen+1)},
			{Speaker: SpeakerB, Text: vocab8[1].text, Translation: "ok"},
		},
	})
	di := len(u.Lessons[0].Activities) - 1
	got = byRule(Validate(ctx, &u, lk), RuleUnitTranslationLength)
	want := at("lessons", 0, "activities", di, "lines", 0, "translation")
	if len(got) != 1 || got[0].Severity != SeverityWarn || got[0].Pointer != want {
		t.Fatalf("line length %#v", got)
	}
}

func TestTranslationDistractors(t *testing.T) {
	ctx := context.Background()
	_, lk := v1()
	w1, w2 := vocab8[0].text, vocab8[1].text
	sentence := Activity{Type: ActivityListenPick, Text: w1 + " " + w2, Target: w1, Translation: "good morning"}
	extra := func(n int) []Activity {
		out := make([]Activity, n)
		for i := range out {
			out[i] = Activity{Type: ActivityRecall, Text: w1, Translation: fmt.Sprintf("hello %d", i+1)}
		}
		return out
	}
	unit := func(acts ...Activity) Unit {
		return Unit{
			Title: "Cafe", Situation: "Ordering", Concept: "Greeting",
			NewWords: []string{w1, w2},
			Lessons:  []Lesson{{Kind: KindLesson, Activities: acts}},
		}
	}

	three := unit(append([]Activity{sentence}, extra(2)...)...)
	got := byRule(Validate(ctx, &three, lk), RuleUnitTranslationDistractors)
	ptr := at("lessons", 0, "activities", 0, "translation")
	if len(got) != 1 || got[0].Severity != SeverityWarn || got[0].Pointer != ptr || !strings.Contains(got[0].Msg, "4") {
		t.Fatalf("three lines %#v", got)
	}
	if len(byRule(Validate(ctx, &three, lk), RuleUnitTranslationMissing)) != 0 {
		t.Fatal("a filled sentence should not be missing")
	}

	four := unit(append([]Activity{sentence}, extra(3)...)...)
	if got := byRule(Validate(ctx, &four, lk), RuleUnitTranslationDistractors); len(got) != 0 {
		t.Fatalf("four lines %#v", got)
	}

	// Dialogue line translations count, including the sentence itself.
	withLines := unit(
		sentence,
		Activity{Type: ActivityDialogue, Lines: []DialogueLine{
			{Speaker: SpeakerA, Text: w1, Translation: "hello"},
			{Speaker: SpeakerB, Text: w2, Translation: "morning"},
			{Speaker: SpeakerA, Text: w1, Translation: "again"},
		}},
	)
	if got := byRule(Validate(ctx, &withLines, lk), RuleUnitTranslationDistractors); len(got) != 0 {
		t.Fatalf("dialogue lines %#v", got)
	}
	withLines.Lessons[0].Activities[1].Lines[2].Translation = ""
	got = byRule(Validate(ctx, &withLines, lk), RuleUnitTranslationDistractors)
	if len(got) != 1 || got[0].Pointer != ptr {
		t.Fatalf("dropped line %#v", got)
	}

	oneWord := unit(Activity{Type: ActivityListenPick, Text: w1})
	if got := byRule(Validate(ctx, &oneWord, lk), RuleUnitTranslationDistractors); len(got) != 0 {
		t.Fatalf("one word %#v", got)
	}
	if got := byRule(Validate(ctx, &oneWord, lk), RuleUnitTranslationMissing); len(got) != 0 {
		t.Fatalf("one word missing %#v", got)
	}

	empty := sentence
	empty.Translation = ""
	bare := unit(empty)
	if len(byRule(Validate(ctx, &bare, lk), RuleUnitTranslationMissing)) != 1 {
		t.Fatal("empty sentence should be missing")
	}
	if len(byRule(Validate(ctx, &bare, lk), RuleUnitTranslationDistractors)) != 1 {
		t.Fatal("empty sentence should also warn about distractors")
	}

	// Four copies of the same English, once punctuation and case are folded, are one line.
	folded := unit(
		sentence,
		Activity{Type: ActivityRecall, Text: w1, Translation: "Hello!"},
		Activity{Type: ActivityRecall, Text: w1, Translation: "hello"},
		Activity{Type: ActivityRecall, Text: w1, Translation: " HELLO "},
	)
	got = byRule(Validate(ctx, &folded, lk), RuleUnitTranslationDistractors)
	if len(got) != 1 || got[0].Severity != SeverityWarn || got[0].Pointer != ptr {
		t.Fatalf("folded duplicates %#v", got)
	}
}

func TestTranslationMarkupAlsoNoMarkup(t *testing.T) {
	u, lk, f := briefFixture()
	li, ai := firstSentence(t, u)
	u.Lessons[li].Activities[ai].Translation = "a < b"
	ptr := at("lessons", li, "activities", ai, "translation")
	var got []string
	for _, is := range validateBrief(&u, lk, &f) {
		if is.Pointer == ptr && (is.Rule == RuleUnitNoMarkup || is.Rule == RuleUnitTranslationLang) {
			got = append(got, is.Rule)
		}
	}
	slices.Sort(got)
	want := []string{RuleUnitNoMarkup, RuleUnitTranslationLang}
	if !slices.Equal(got, want) {
		t.Fatalf("markup rules %v", got)
	}
}

// TestTargetAndTranslationShareMultiWord checks that activity.target and
// unit.translation_missing use one count: two or more exact lexicon entries.
func TestTargetAndTranslationShareMultiWord(t *testing.T) {
	ctx := context.Background()
	const (
		one = "שָׁלוֹם"
		two = "קָפֶה"
		gar = "גָּר"
	)
	bare := stripNiqqud(gar)
	lk := fakeLookup{lex: map[string]glex{
		normalize(one):  {ID: 1, POS: "noun", Gender: "m", Gloss: "hello", Bare: stripNiqqud(one)},
		normalize(two):  {ID: 2, POS: "noun", Gender: "m", Gloss: "coffee", Bare: stripNiqqud(two)},
		normalize(bare): {ID: 3, POS: "verb", Gloss: "lives", Bare: bare},
	}}
	s := &scan{ctx: ctx, lk: lk, cache: map[string]*probe{}}
	cases := []struct {
		text  string
		multi bool
	}{
		{one + " " + two, true},
		{one, false},
		{one + " " + gar, false},
		{one + " 12", false},
		{one + " אַלְמוֹנִי", false},
	}
	for _, c := range cases {
		n := lexemeTokenCount(s, Activity{Text: c.text})
		if s.err != nil {
			t.Fatal(s.err)
		}
		if (n > 1) != c.multi {
			t.Fatalf("%q count %d, want multi %v", c.text, n, c.multi)
		}
		base := Unit{
			Title: "Cafe", Situation: "Ordering", Concept: "Greeting",
			NewWords: []string{one, two},
			Lessons:  []Lesson{{Kind: KindLesson}},
		}
		for _, typ := range []string{ActivityRecall, ActivityProduce, ActivityCloze} {
			doc := base
			doc.Lessons[0].Activities = []Activity{{Type: typ, Text: c.text, Translation: "a line"}}
			if got := len(targetOnly(Validate(ctx, &doc, lk))) > 0; got != c.multi {
				t.Fatalf("%s %q target required %v, want %v", typ, c.text, got, c.multi)
			}
		}
		doc := base
		doc.Lessons[0].Activities = []Activity{{Type: ActivityListenPick, Text: c.text}}
		if got := len(byRule(Validate(ctx, &doc, lk), RuleUnitTranslationMissing)) > 0; got != c.multi {
			t.Fatalf("listen_pick %q translation required %v, want %v", c.text, got, c.multi)
		}
		if got := targetOnly(Validate(ctx, &doc, lk)); len(got) != 0 {
			t.Fatalf("listen_pick %q target %#v", c.text, got)
		}
	}
}

// TestSingleLexemeListenPickOmitsTargetAndTranslation checks a listen_pick
// of one lexicon word with neither field set. That word is credited, so
// activity.target stays quiet, and translation_missing applies only at two
// or more lexicon words.
func TestSingleLexemeListenPickOmitsTargetAndTranslation(t *testing.T) {
	ctx := context.Background()
	const word = "שָׁלוֹם"
	lk := fakeLookup{lex: map[string]glex{
		normalize(word): {ID: 1, POS: "noun", Gender: "m", Gloss: "hello", Bare: stripNiqqud(word)},
	}}
	doc := Unit{
		Title: "Cafe", Situation: "Ordering", Concept: "Greeting",
		NewWords: []string{word},
		Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
			{Type: ActivityListenPick, Text: word},
		}}},
	}
	got := Validate(ctx, &doc, lk)
	if issues := targetOnly(got); len(issues) != 0 {
		t.Fatalf("activity.target %#v", issues)
	}
	if issues := byRule(got, RuleUnitTranslationMissing); len(issues) != 0 {
		t.Fatalf("translation_missing %#v", issues)
	}
}

// TestListenPickCreditPhrases pins the player wording. A target-less line of
// two or more lexicon words grades the first new word, or the first lexicon
// word, and logs no exposure. Whole-sentence choices are planned with #139.
func TestListenPickCreditPhrases(t *testing.T) {
	phrases := []string{
		"only that word gets credit",
		"exactly one lexicon word credits that word",
		"first new word",
		"first lexicon word if there is no new word",
		"logs no exposure",
		"Planned with #139 and migration 015",
		"credits nothing and logs a lesson_listen exposure",
	}
	var target string
	for _, r := range Rules {
		if r.ID == RuleActivityTarget {
			target = r.Description
		}
	}
	for _, phrase := range phrases {
		if !strings.Contains(target, phrase) {
			t.Errorf("activity.target description missing %q", phrase)
		}
	}
	if strings.Contains(target, "logged as an exposure") {
		t.Error("activity.target description still says an exposure is logged now")
	}
	files := []string{
		"doc.go",
		"rules.go",
		"../mcp/schema.go",
		"../mcp/tools.go",
		"../mcp/server.go",
		"../mcp/tool_publish_unit.go",
		"../mcp/resources/authoring.md",
		"../../README.md",
		"../../.cursor/skills/hebrew-lesson-author/SKILL.md",
	}
	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, phrase := range phrases {
			if !strings.Contains(text, phrase) {
				t.Errorf("%s missing %q", path, phrase)
			}
		}
		if strings.Contains(text, "logged as an exposure") || strings.Contains(text, "picks the meaning of the whole line") || strings.Contains(text, "grades the whole sentence") {
			t.Errorf("%s still describes the old listen_pick player", path)
		}
	}
	purpose := []string{
		"lexicon-gloss buttons",
		"collected and validated now",
		"whole-sentence choices are planned with #139",
	}
	for _, path := range []string{"../mcp/schema.go", "../mcp/resources/authoring.md", "../../.cursor/skills/hebrew-lesson-author/SKILL.md"} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, phrase := range purpose {
			if !strings.Contains(string(body), phrase) {
				t.Errorf("%s missing %q", path, phrase)
			}
		}
	}
	logic, err := os.ReadFile("validate_translation.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logic), "Hebrew Dev's ruling of 2026-10-20") {
		t.Error("validate_translation.go does not cite Hebrew Dev's ruling of 2026-10-20")
	}
}

// TestTranslationLangCompatibilityForms kills an NFKC-to-NFC swap.
// U+FB2A and U+FB4B produce unit.translation_lang. In x/text v0.32.0 those
// two also decompose under NFC, so the swap that the check can see is a
// compatibility-only letter: U+FB20 stays outside the Hebrew block under
// NFC and maps to ayin under NFKC.
func TestTranslationLangCompatibilityForms(t *testing.T) {
	ctx := context.Background()
	u, lk := v1()
	li, ai := firstSentence(t, u)
	ptr := at("lessons", li, "activities", ai, "translation")
	for _, bad := range []string{"\uFB2A", "\uFB4B", "\uFB20", "good \uFB20 morning"} {
		u.Lessons[li].Activities[ai].Translation = bad
		got := byRule(Validate(ctx, &u, lk), RuleUnitTranslationLang)
		if len(got) != 1 || got[0].Severity != SeverityBlock || got[0].Pointer != ptr {
			t.Fatalf("%U %#v", []rune(bad)[0], got)
		}
	}
}

// TestTranslationFoldPunctuation counts 'Good morning!' and 'good morning' as one line.
func TestTranslationFoldPunctuation(t *testing.T) {
	if foldTranslation("Good morning!") != "good morning" {
		t.Fatalf("fold %q", foldTranslation("Good morning!"))
	}
	assertDistractorWarn(t, "good morning", "Good morning!", "hello", "thanks")
}

// TestTranslationFoldWhitespace counts a double space and a tab as 'good morning'.
func TestTranslationFoldWhitespace(t *testing.T) {
	if foldTranslation("good  morning") != "good morning" || foldTranslation("good\tmorning") != "good morning" {
		t.Fatalf("double %q tab %q", foldTranslation("good  morning"), foldTranslation("good\tmorning"))
	}
	assertDistractorWarn(t, "good morning", "good  morning", "good\tmorning", "hello")
}

// TestTranslationFoldTrim drops a leading or trailing space so the line matches.
func TestTranslationFoldTrim(t *testing.T) {
	if foldTranslation("  hello") != "hello" || foldTranslation("hello ") != "hello" || foldTranslation("hello") != "hello" {
		t.Fatalf("lead %q trail %q", foldTranslation("  hello"), foldTranslation("hello "))
	}
	assertDistractorWarn(t, "good morning", "hello", "  hello", "hello ")
}

// TestListenPickLookupFailure is one schema.lookup and no translation_missing
// when a sentence listen_pick cannot read the lexicon.
func TestListenPickLookupFailure(t *testing.T) {
	ctx := context.Background()
	_, base := v1()
	w1, w2, w3 := vocab8[0].text, vocab8[1].text, vocab8[2].text
	act := Activity{Type: ActivityListenPick, Text: w1 + " " + w2 + " " + w3}
	s := &scan{ctx: ctx, lk: base, cache: map[string]*probe{}}
	if n := lexemeTokenCount(s, act); n < 3 || s.err != nil {
		t.Fatalf("fixture count %d err %v", n, s.err)
	}
	doc := Unit{
		Title: "Cafe", Situation: "Ordering", Concept: "Greeting",
		NewWords: []string{w1, w2, w3},
		Lessons:  []Lesson{{Kind: KindLesson, Activities: []Activity{act}}},
	}
	got := Validate(ctx, &doc, failLexeme{fakeLookup: base, fail: w3})
	if n := len(byRule(got, RuleSchemaLookup)); n != 1 {
		t.Fatalf("schema.lookup count %d: %#v", n, got)
	}
	if missing := byRule(got, RuleUnitTranslationMissing); len(missing) != 0 {
		t.Fatalf("translation_missing %#v", missing)
	}
}

// TestTranslationDistractorPointerIsFirstSentence points the warn at the first
// of two sentence listen_picks.
func TestTranslationDistractorPointerIsFirstSentence(t *testing.T) {
	ctx := context.Background()
	_, lk := v1()
	w1, w2 := vocab8[0].text, vocab8[1].text
	line := func(translation string) Activity {
		return Activity{Type: ActivityListenPick, Text: w1 + " " + w2, Translation: translation}
	}
	doc := Unit{
		Title: "Cafe", Situation: "Ordering", Concept: "Greeting",
		NewWords: []string{w1, w2},
		Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
			line("good morning"),
			line("hello"),
		}}},
	}
	got := byRule(Validate(ctx, &doc, lk), RuleUnitTranslationDistractors)
	want := "/lessons/0/activities/0/translation"
	if len(got) != 1 || got[0].Severity != SeverityWarn || got[0].Pointer != want {
		t.Fatalf("pointer %#v", got)
	}
}

func assertDistractorWarn(t *testing.T, translations ...string) {
	t.Helper()
	ctx := context.Background()
	_, lk := v1()
	w1, w2 := vocab8[0].text, vocab8[1].text
	acts := make([]Activity, len(translations))
	acts[0] = Activity{Type: ActivityListenPick, Text: w1 + " " + w2, Translation: translations[0]}
	for i, translation := range translations[1:] {
		acts[i+1] = Activity{Type: ActivityRecall, Text: w1, Translation: translation}
	}
	doc := Unit{
		Title: "Cafe", Situation: "Ordering", Concept: "Greeting",
		NewWords: []string{w1, w2},
		Lessons:  []Lesson{{Kind: KindLesson, Activities: acts}},
	}
	got := byRule(Validate(ctx, &doc, lk), RuleUnitTranslationDistractors)
	if len(got) != 1 || got[0].Severity != SeverityWarn {
		t.Fatalf("folded lines %#v", got)
	}
}

// failLexeme answers the lexicon, except one spelling, which fails the lookup.
type failLexeme struct {
	fakeLookup
	fail string
}

func (f failLexeme) LexemeByText(ctx context.Context, text string) (int64, bool, error) {
	if normalize(text) == normalize(f.fail) {
		return 0, false, errors.New("lexicon down")
	}
	return f.fakeLookup.LexemeByText(ctx, text)
}

func firstSentence(t *testing.T, u Unit) (int, int) {
	t.Helper()
	for i, les := range u.Lessons {
		for j, act := range les.Activities {
			if act.Type == ActivityListenPick && fixtureHebrewWords(act) > 1 {
				return i, j
			}
		}
	}
	t.Fatal("fixture has no sentence listen_pick")
	return 0, 0
}

func byRule(issues []Issue, rule string) []Issue {
	var out []Issue
	for _, is := range issues {
		if is.Rule == rule {
			out = append(out, is)
		}
	}
	return out
}
