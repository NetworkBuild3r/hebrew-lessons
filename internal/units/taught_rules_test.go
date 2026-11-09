package units

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetaTextAuthoringPhraseAndCleanEnglish(t *testing.T) {
	if !ContainsMetaText("Correct form first; incorrect form second.") {
		t.Fatal("authoring note must match")
	}
	if ContainsMetaText("I live.") {
		t.Fatal(`"I live." must not match`)
	}

	raw, err := os.ReadFile(filepath.Join("testdata", "meta_text_23_units.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		Phrase string `json:"phrase"`
		Count  int    `json:"count"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Count != 23 || meta.Phrase == "" {
		t.Fatalf("fixture: count=%d phrase=%q", meta.Count, meta.Phrase)
	}
	for i := 0; i < meta.Count; i++ {
		u := Unit{
			Title: "clone", Situation: "s", Concept: "c",
			Lessons: []Lesson{{Kind: KindCheckpoint, Activities: []Activity{
				{Type: ActivityPattern, Text: "אֲנִי", Translation: meta.Phrase},
			}}},
		}
		got := metaTextIssues(&u, ModeWrite)
		if len(got) != 1 || got[0].Rule != RuleUnitMetaText || got[0].Severity != SeverityBlock {
			t.Fatalf("unit %d of 23: %v", i+1, got)
		}
		pub := metaTextIssues(&u, ModePublished)
		if len(pub) != 1 || pub[0].Severity != SeverityWarn {
			t.Fatalf("unit %d ModePublished: %v", i+1, pub)
		}
	}

	clean := Unit{
		Title: "clean", Situation: "s", Concept: "c",
		Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
			{Type: ActivityCloze, Text: "אֲנִי גָּר", Target: "גָּר", Translation: "I live."},
			{Type: ActivitySay, Text: "אֲנִי גָּר", PromptEN: "Say: I live."},
		}}},
	}
	if got := metaTextIssues(&clean, ModeWrite); len(got) != 0 {
		t.Fatalf("clean English: %v", got)
	}
}

func TestMetaTextUnits1To11Fixtures(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "units_1_to_11_english.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Lines []string `json:"lines"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Lines) == 0 {
		t.Fatal("empty units 1–11 English fixtures")
	}
	// Justified exceptions: none. Every listed line is learner meaning English.
	var hits []string
	for _, line := range file.Lines {
		if ContainsMetaText(line) {
			hits = append(hits, line)
		}
		u := Unit{Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
			{Type: ActivityCloze, Text: "אֲנִי", Translation: line},
		}}}}
		if got := metaTextIssues(&u, ModeWrite); len(got) != 0 {
			hits = append(hits, line+" via issues")
		}
	}
	if len(hits) != 0 {
		t.Fatalf("unit.meta_text on units 1–11 fixtures (justify each hit in this test): %v", hits)
	}
}

func TestTargetUntaughtUnit12SteakCheckpoint(t *testing.T) {
	u := loadCloneFixture(t, "unit12_steak_checkpoint.json")
	const (
		lehachin = "לְהָכִין"
		steak    = "סְטֵייק"
		laasot   = "לַעֲשׂוֹת"
		ani      = "אֲנִי"
	)
	lex := map[string]glex{
		normalize(lehachin): {ID: 1, POS: "verb", Gloss: "to prepare", Bare: stripNiqqud(lehachin)},
		normalize(steak):    {ID: 2, POS: "noun", Gender: "m", Gloss: "steak", Bare: stripNiqqud(steak)},
		normalize(laasot):   {ID: 3, POS: "verb", Gloss: "to do", Bare: stripNiqqud(laasot)},
		normalize(ani):      {ID: 4, POS: "pronoun", Gloss: "I", Bare: stripNiqqud(ani)},
	}
	audio := []string{normalize(lehachin), normalize(steak), normalize(laasot), normalize(ani),
		normalize(lehachin + " " + steak + " — " + laasot + " " + steak),
		normalize(ani + " " + lehachin + " " + steak)}
	lk := goldenLookup{Lexemes: lex, Audio: audio}.fake()

	// Learner has no card for לְהָכִין (id 1). Steak and אני are known.
	f := BriefFacts{
		Tier:              "established",
		MinKnownShare:     0.95,
		BriefLexemeIDs:    map[int64]bool{},
		KnownLexemeIDs:    map[int64]bool{2: true, 4: true},
		LearningLexemeIDs: map[int64]bool{},
	}

	got := ValidateWithBriefIn(context.Background(), &u, lk, &f, ModeWrite)
	if !hasRule(got, RuleUnitTargetUntaught) {
		t.Fatalf("want unit.target_untaught, got %v", ruleSet(got))
	}
	for _, is := range got {
		if is.Rule == RuleUnitTargetUntaught && is.Severity != SeverityBlock {
			t.Fatalf("ModeWrite severity: %v", is)
		}
	}
	pub := targetUntaughtIssues(context.Background(), &u, lk, &f, ModePublished)
	if len(pub) != 1 || pub[0].Severity != SeverityWarn {
		t.Fatalf("ModePublished: %v", pub)
	}

	// Without a brief the rule does not run.
	noBrief := ValidateIn(context.Background(), &u, lk, ModeWrite)
	if hasRule(noBrief, RuleUnitTargetUntaught) {
		t.Fatalf("without brief: %v", ruleSet(noBrief))
	}
}

func TestCheckpointCoverageBlocksUntaughtPatternLines(t *testing.T) {
	u := loadCloneFixture(t, "checkpoint_bad_coverage.json")
	words := []string{"זָר", "אֶחָד", "שְׁנַיִם", "שְׁלוֹשָׁה", "אַרְבַּע", "חָמֵשׁ"}
	lex := map[string]glex{}
	audio := []string{}
	for i, w := range words {
		lex[normalize(w)] = glex{ID: int64(i + 1), POS: "noun", Gender: "m", Bare: stripNiqqud(w), Gloss: "g"}
		audio = append(audio, normalize(w))
	}
	for _, les := range u.Lessons {
		for _, act := range les.Activities {
			audio = append(audio, normalize(act.Text))
		}
	}
	lk := goldenLookup{Lexemes: lex, Audio: audio}.fake()
	f := BriefFacts{
		Tier:              "established",
		MinKnownShare:     0.95,
		BriefLexemeIDs:    map[int64]bool{},
		KnownLexemeIDs:    map[int64]bool{},
		LearningLexemeIDs: map[int64]bool{},
	}

	got := ValidateWithBriefIn(context.Background(), &u, lk, &f, ModeWrite)
	if !hasRule(got, RuleBriefCheckpointCoverage) {
		t.Fatalf("want brief.checkpoint_coverage, got %v", ruleSet(got))
	}
	pub := checkpointCoverageIssues(context.Background(), &u, lk, &f, ModePublished)
	if len(pub) != 1 || pub[0].Severity != SeverityWarn {
		t.Fatalf("ModePublished: %v", pub)
	}
	if hasRule(ValidateIn(context.Background(), &u, lk, ModeWrite), RuleBriefCheckpointCoverage) {
		t.Fatal("without brief checkpoint_coverage must not run")
	}
}

func TestSkippedWithoutBriefListsTaughtRules(t *testing.T) {
	got := SkippedWithoutBrief()
	want := map[string]bool{RuleUnitTargetUntaught: true, RuleBriefCheckpointCoverage: true}
	if len(got) != len(want) {
		t.Fatalf("skipped = %v", got)
	}
	for _, id := range got {
		if !want[id] {
			t.Fatalf("unexpected skipped %s", id)
		}
	}
}

func TestTargetUntaughtAcceptsIntroducedNewWordAndPrevious(t *testing.T) {
	const (
		word = "לֶחֶם"
		ani  = "אֲנִי"
	)
	lex := map[string]glex{
		normalize(word): {ID: 1, POS: "noun", Gender: "m", Gloss: "bread", Bare: stripNiqqud(word)},
		normalize(ani):  {ID: 2, POS: "pronoun", Gloss: "I", Bare: stripNiqqud(ani)},
	}
	u := Unit{
		Title: "t", Situation: "s", Concept: "c",
		NewWords: []string{word},
		Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{
			{Type: ActivityStory, Text: ani + " " + word},
			{Type: ActivityCloze, Text: ani + " " + word, Target: word, Translation: "I bread."},
		}}},
	}
	audio := []string{normalize(word), normalize(ani), normalize(ani + " " + word)}
	lk := goldenLookup{Lexemes: lex, Audio: audio}.fake()
	f := BriefFacts{
		MinKnownShare: 0.60, BriefLexemeIDs: map[int64]bool{1: true},
		KnownLexemeIDs: map[int64]bool{2: true}, LearningLexemeIDs: map[int64]bool{},
	}
	if got := targetUntaughtIssues(context.Background(), &u, lk, &f, ModeWrite); len(got) != 0 {
		t.Fatalf("introduced newWord: %v", got)
	}

	prevOnly := Unit{
		Title: "t", Situation: "s", Concept: "c",
		Lessons: []Lesson{{Kind: KindCheckpoint, Activities: []Activity{
			{Type: ActivityRecall, Text: word},
		}}},
	}
	f2 := BriefFacts{
		MinKnownShare: 0.60, BriefLexemeIDs: map[int64]bool{},
		KnownLexemeIDs: map[int64]bool{}, LearningLexemeIDs: map[int64]bool{},
		PreviousUnits: []BriefPreviousUnit{{NewWords: []string{word}}},
	}
	lk2 := goldenLookup{Lexemes: lex, Audio: []string{normalize(word)}}.fake()
	if got := targetUntaughtIssues(context.Background(), &prevOnly, lk2, &f2, ModeWrite); len(got) != 0 {
		t.Fatalf("previous unit newWord: %v", got)
	}

	learning := BriefFacts{
		MinKnownShare: 0.60, BriefLexemeIDs: map[int64]bool{},
		KnownLexemeIDs: map[int64]bool{}, LearningLexemeIDs: map[int64]bool{1: true},
	}
	if got := targetUntaughtIssues(context.Background(), &prevOnly, lk2, &learning, ModeWrite); len(got) != 0 {
		t.Fatalf("learning word: %v", got)
	}
}

func TestAuthoringMetaPhrasesStartWithRequiredList(t *testing.T) {
	want := []string{
		"correct form", "incorrect form", "wrong form", "first;", "second.",
		"target", "distractor", "checkpoint",
	}
	if len(AuthoringMetaPhrases) < len(want) {
		t.Fatalf("AuthoringMetaPhrases too short: %v", AuthoringMetaPhrases)
	}
	for i, p := range want {
		if !strings.EqualFold(AuthoringMetaPhrases[i], p) {
			t.Fatalf("phrase %d: got %q want %q", i, AuthoringMetaPhrases[i], p)
		}
	}
}
