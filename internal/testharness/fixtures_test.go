package testharness

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Activities have no audio field, by design. A clip is addressed by the
// exact pointed NFC text. The validator calls Lookup.HasAudio(ctx, textNFC),
// which matches audio_assets.text_nfc. A lexeme points at that row through
// audio_id. testdata/audio_present.json is the text list a fake Lookup uses.
// bad_missing_audio.json is the valid unit plus one listen_pick whose text
// is absent from that list. MCP Builder's rules.go (#55) decides which
// activity types need a clip.

func TestFixtures(t *testing.T) {
	t.Parallel()
	lex := loadLexemes(t)
	byText := map[string]lexeme{}
	for _, item := range lex {
		byText[item.TextNfc] = item
	}
	audio := loadAudio(t)

	t.Run("lexemes", func(t *testing.T) {
		if len(lex) != 40 {
			t.Fatalf("len = %d, want 40", len(lex))
		}
		seen := map[string]bool{}
		glosses := map[string]bool{}
		for _, item := range lex {
			if item.TextNfc == "" || item.GlossEn == "" || item.POS == "" {
				t.Fatalf("empty field: %+v", item)
			}
			if seen[item.TextNfc] || glosses[item.GlossEn] {
				t.Fatalf("duplicate %+v", item)
			}
			seen[item.TextNfc] = true
			glosses[item.GlossEn] = true
			if !posOK[item.POS] {
				t.Fatalf("pos %q", item.POS)
			}
			assertHebrewWord(t, item.TextNfc)
			if item.Bare != hebrewLetters(item.TextNfc) {
				t.Fatalf("bare %q from %q", item.Bare, item.TextNfc)
			}
			assertFinalLetters(t, item.Bare)
			if strings.TrimSpace(item.TextNfc) != item.TextNfc || strings.Contains(item.TextNfc, " ") {
				t.Fatalf("spacing %q", item.TextNfc)
			}
			if item.Root != "" {
				if item.Root != hebrewLetters(item.Root) {
					t.Fatalf("root %q", item.Root)
				}
			}
		}
	})

	valid := loadUnit(t, "valid_8words.json")
	nine := loadUnit(t, "bad_9_new_words.json")
	four := loadUnit(t, "bad_lesson_4_new_words.json")
	low := loadUnit(t, "bad_low_coverage.json")
	missing := loadUnit(t, "bad_missing_audio.json")
	for _, doc := range []units.Unit{valid, nine, four, low, missing} {
		assertUnitBasics(t, doc, byText)
	}

	t.Run("valid", func(t *testing.T) {
		if len(valid.NewWords) != 8 {
			t.Fatalf("new words %d", len(valid.NewWords))
		}
		if got := introCounts(valid); !sameInts(got, []int{2, 2, 2, 2, 0}) {
			t.Fatalf("intros %v", got)
		}
		if kinds(valid) != "lesson,lesson,lesson,lesson,checkpoint" {
			t.Fatalf("kinds %s", kinds(valid))
		}
		if !sameStrings(activityTypes(valid), []string{
			units.ActivityCloze,
			units.ActivityListenPick,
			units.ActivityPattern,
			units.ActivityProduce,
			units.ActivityRecall,
			units.ActivityShadow,
			units.ActivityStory,
		}) {
			t.Fatalf("types %v", activityTypes(valid))
		}
		if ratio := storyNewWordRatio(t, valid); ratio != 1 {
			t.Fatalf("story coverage %v", ratio)
		}
		assertStoryAfterIntro(t, valid)
	})

	t.Run("nineNewWords", func(t *testing.T) {
		if len(nine.NewWords) != 9 {
			t.Fatalf("new words %d", len(nine.NewWords))
		}
		if nine.NewWords[8] != lexemeByGloss(t, lex, "bread") {
			t.Fatalf("ninth word %q", nine.NewWords[8])
		}
		// 3 in the last content lesson is inside the 2–3 band. The fault is the ninth word.
		if got := introCounts(nine); !sameInts(got, []int{2, 2, 2, 3, 0}) {
			t.Fatalf("intros %v", got)
		}
	})

	t.Run("lessonFourNewWords", func(t *testing.T) {
		// Three lessons keep the unit at 8 words, so the only budget fault is one lesson of 4.
		if len(four.NewWords) != 8 {
			t.Fatalf("new words %d", len(four.NewWords))
		}
		if got := introCounts(four); !sameInts(got, []int{4, 2, 2, 0}) {
			t.Fatalf("intros %v", got)
		}
		if kinds(four) != "lesson,lesson,lesson,checkpoint" {
			t.Fatalf("kinds %s", kinds(four))
		}
	})

	t.Run("lowCoverage", func(t *testing.T) {
		if got := introCounts(low); !sameInts(got, []int{2, 2, 2, 2, 0}) {
			t.Fatalf("intros %v", got)
		}
		ratio := storyNewWordRatio(t, low)
		if ratio >= 0.9 {
			t.Fatalf("story coverage %v, want under 0.9", ratio)
		}
		newSet := wordSet(low.NewWords)
		for _, tok := range storyTokens(t, low) {
			if newSet[tok] {
				t.Fatalf("story token %q is a new word", tok)
			}
			if _, ok := byText[tok]; !ok {
				t.Fatalf("story token %q is not in the lexicon", tok)
			}
		}
	})

	t.Run("missingAudio", func(t *testing.T) {
		milk := lexemeByGloss(t, lex, "milk")
		present := map[string]bool{}
		for _, text := range audio {
			if present[text] {
				t.Fatalf("duplicate clip %q", text)
			}
			present[text] = true
			if !strings.Contains(text, " ") && byText[text].TextNfc == "" {
				t.Fatalf("clip %q is not a lexeme", text)
			}
		}
		for _, name := range []string{"valid_8words.json", "bad_9_new_words.json", "bad_lesson_4_new_words.json", "bad_low_coverage.json"} {
			for _, text := range activityTexts(loadUnit(t, name)) {
				if !present[text] {
					t.Fatalf("%s text is not in audio_present.json", name)
				}
			}
		}
		var absent []string
		for _, text := range activityTexts(missing) {
			if !present[text] {
				absent = append(absent, text)
			}
		}
		if len(absent) != 1 || absent[0] != milk {
			t.Fatalf("missing clips %q, want milk", absent)
		}
	})
}

// TestTestdataHebrewIsNFC checks every Hebrew string under testdata.
// golang.org/x/text/unicode/norm is not in go.mod. Hebrew letters have no
// precomposed form, so a string is NFC when each letter's points are already
// in non-decreasing combining-class order. The check does not strip niqqud.
func TestTestdataHebrewIsNFC(t *testing.T) {
	t.Parallel()
	root := testdataDir(t)
	n := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		n += assertHebrewNFC(t, v)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("testdata has no Hebrew strings")
	}
}

func TestAudioPresentLookup(t *testing.T) {
	t.Parallel()
	lex := loadLexemes(t)
	texts := make([]string, len(lex))
	for i, item := range lex {
		texts[i] = item.TextNfc
	}
	lk := NewFixtureLookup(loadAudio(t), texts)
	ctx := context.Background()
	hello := lexemeByGloss(t, lex, "hello; peace")
	milk := lexemeByGloss(t, lex, "milk")
	ok, err := lk.HasAudio(ctx, hello)
	if err != nil || !ok {
		t.Fatalf("HasAudio(%q) = %v, %v", hello, ok, err)
	}
	ok, err = lk.HasAudio(ctx, milk)
	if err != nil || ok {
		t.Fatalf("HasAudio(milk) = %v, %v", ok, err)
	}
	bare := hebrewLetters(hello)
	if bare == hello {
		t.Fatal("bare hello still has points")
	}
	ok, err = lk.HasAudio(ctx, bare)
	if err != nil || ok {
		t.Fatalf("HasAudio(bare) = %v, %v", ok, err)
	}
	id, found, err := lk.LexemeByText(ctx, hello)
	if err != nil || !found || id == 0 {
		t.Fatalf("LexemeByText = %d, %v, %v", id, found, err)
	}
	if _, found, err := lk.LexemeByText(ctx, bare); err != nil || found {
		t.Fatalf("bare lexeme found=%v err=%v", found, err)
	}
}

func assertHebrewNFC(t *testing.T, v any) int {
	t.Helper()
	switch x := v.(type) {
	case string:
		if !containsHebrew(x) {
			return 0
		}
		assertNFC(t, x)
		return 1
	case []any:
		n := 0
		for _, item := range x {
			n += assertHebrewNFC(t, item)
		}
		return n
	case map[string]any:
		n := 0
		for _, item := range x {
			n += assertHebrewNFC(t, item)
		}
		return n
	default:
		return 0
	}
}

func containsHebrew(s string) bool {
	for _, r := range s {
		if r >= 0x0591 && r <= 0x05F4 {
			return true
		}
	}
	return false
}

type lexeme struct {
	TextNfc string `json:"textNfc"`
	Bare    string `json:"bare"`
	GlossEn string `json:"glossEn"`
	POS     string `json:"pos"`
	Root    string `json:"root"`
}

var posOK = map[string]bool{
	"noun":         true,
	"verb":         true,
	"adjective":    true,
	"adverb":       true,
	"pronoun":      true,
	"numeral":      true,
	"interjection": true,
}

// hebrewPointClass is the Unicode combining class of each Hebrew point.
// golang.org/x/text/unicode/norm is not in go.mod. Hebrew has no
// precomposed letters, so a string whose points are already in
// non-decreasing class order is NFC. Niqqud stays on the letter.
var hebrewPointClass = map[rune]int{
	0x05B0: 10,
	0x05B1: 11,
	0x05B2: 12,
	0x05B3: 13,
	0x05B4: 14,
	0x05B5: 15,
	0x05B6: 16,
	0x05B7: 17,
	0x05B8: 18,
	0x05B9: 19,
	0x05BA: 19,
	0x05BB: 20,
	0x05BC: 21,
	0x05BD: 22,
	0x05BF: 23,
	0x05C1: 24,
	0x05C2: 25,
	0x05C4: 230,
	0x05C5: 220,
	0x05C7: 18,
}

func loadLexemes(t *testing.T) []lexeme {
	t.Helper()
	var doc struct {
		Lexemes []lexeme `json:"lexemes"`
	}
	decodeFixture(t, "lexemes.json", &doc)
	return doc.Lexemes
}

func loadAudio(t *testing.T) []string {
	t.Helper()
	var doc struct {
		Texts []string `json:"texts"`
	}
	decodeFixture(t, "audio_present.json", &doc)
	return doc.Texts
}

func loadUnit(t *testing.T, name string) units.Unit {
	t.Helper()
	var doc units.Unit
	decodeFixture(t, filepath.Join("units", name), &doc)
	return doc
}

func lexemeByGloss(t *testing.T, lex []lexeme, gloss string) string {
	t.Helper()
	var found string
	for _, item := range lex {
		if item.GlossEn != gloss {
			continue
		}
		if found != "" {
			t.Fatalf("duplicate gloss %q", gloss)
		}
		found = item.TextNfc
	}
	if found == "" {
		t.Fatalf("no lexeme gloss %q", gloss)
	}
	return found
}

func decodeFixture(t *testing.T, name string, dest any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(testdataDir(t), name))
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if dec.More() {
		t.Fatalf("%s: extra JSON", name)
	}
}

func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate testdata")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

func assertUnitBasics(t *testing.T, doc units.Unit, lex map[string]lexeme) {
	t.Helper()
	if doc.Title == "" || doc.Situation == "" || doc.Concept == "" {
		t.Fatalf("empty heading %+v", doc)
	}
	seen := map[string]bool{}
	for _, word := range doc.NewWords {
		if seen[word] {
			t.Fatalf("duplicate new word %q", word)
		}
		seen[word] = true
		if lex[word].TextNfc == "" {
			t.Fatalf("new word %q is not in the lexicon", word)
		}
		assertHebrewWord(t, word)
	}
	if len(doc.Lessons) == 0 {
		t.Fatal("no lessons")
	}
	for _, lesson := range doc.Lessons {
		if lesson.Kind != units.KindLesson && lesson.Kind != units.KindCheckpoint {
			t.Fatalf("kind %q", lesson.Kind)
		}
		if len(lesson.Activities) == 0 {
			t.Fatal("lesson has no activities")
		}
		for _, act := range lesson.Activities {
			if !activityTypeOK(act.Type) || act.Text == "" {
				t.Fatalf("activity %+v", act)
			}
			assertNFC(t, act.Text)
			for _, tok := range tokens(act.Text) {
				assertHebrewWord(t, tok)
			}
		}
	}
	if sum(introCounts(doc)) != len(doc.NewWords) {
		t.Fatalf("not every new word is introduced: %v vs %d", introCounts(doc), len(doc.NewWords))
	}
}

func assertHebrewWord(t *testing.T, word string) {
	t.Helper()
	assertNFC(t, word)
	assertFinalLetters(t, word)
	if hebrewLetters(word) == word {
		t.Fatalf("%q has no points", word)
	}
}

func assertNFC(t *testing.T, s string) {
	t.Helper()
	marks := false
	prev := 0
	for _, r := range s {
		if !unicode.Is(unicode.Mn, r) {
			marks = false
			prev = 0
			continue
		}
		class, ok := hebrewPointClass[r]
		if !ok {
			t.Fatalf("%q has combining mark %U", s, r)
		}
		if marks && class < prev {
			t.Fatalf("%q is not NFC at %U", s, r)
		}
		marks = true
		prev = class
	}
}

func assertFinalLetters(t *testing.T, word string) {
	t.Helper()
	letters := []rune(hebrewLetters(word))
	if len(letters) == 0 {
		t.Fatalf("%q has no Hebrew letters", word)
	}
	finalOf := map[rune]rune{'כ': 'ך', 'מ': 'ם', 'נ': 'ן', 'פ': 'ף', 'צ': 'ץ'}
	isFinal := map[rune]bool{'ך': true, 'ם': true, 'ן': true, 'ף': true, 'ץ': true}
	for i, r := range letters {
		if i < len(letters)-1 && isFinal[r] {
			t.Fatalf("%q has a final letter before the end", word)
		}
		if i == len(letters)-1 {
			if repl, ok := finalOf[r]; ok {
				t.Fatalf("%q ends with %c, want %c", word, r, repl)
			}
		}
	}
}

func hebrewLetters(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x05D0 && r <= 0x05EA {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func introCounts(doc units.Unit) []int {
	seen := map[string]bool{}
	newSet := wordSet(doc.NewWords)
	counts := make([]int, len(doc.Lessons))
	for i, lesson := range doc.Lessons {
		for _, act := range lesson.Activities {
			for _, tok := range tokens(act.Text) {
				if newSet[tok] && !seen[tok] {
					seen[tok] = true
					counts[i]++
				}
			}
		}
	}
	return counts
}

func storyNewWordRatio(t *testing.T, doc units.Unit) float64 {
	t.Helper()
	toks := storyTokens(t, doc)
	newSet := wordSet(doc.NewWords)
	hit := 0
	for _, tok := range toks {
		if newSet[tok] {
			hit++
		}
	}
	return float64(hit) / float64(len(toks))
}

func storyTokens(t *testing.T, doc units.Unit) []string {
	t.Helper()
	var story string
	n := 0
	for _, lesson := range doc.Lessons {
		for _, act := range lesson.Activities {
			if act.Type != units.ActivityStory {
				continue
			}
			story = act.Text
			n++
		}
	}
	if n != 1 {
		t.Fatalf("story activities = %d", n)
	}
	toks := tokens(story)
	if len(toks) == 0 {
		t.Fatal("story has no tokens")
	}
	return toks
}

func assertStoryAfterIntro(t *testing.T, doc units.Unit) {
	t.Helper()
	seen := map[string]bool{}
	for _, lesson := range doc.Lessons {
		for _, act := range lesson.Activities {
			toks := tokens(act.Text)
			if act.Type == units.ActivityStory {
				for _, tok := range toks {
					if !seen[tok] {
						t.Fatalf("story token was not introduced earlier")
					}
				}
				continue
			}
			for _, tok := range toks {
				seen[tok] = true
			}
		}
	}
}

func activityTexts(doc units.Unit) []string {
	var out []string
	for _, lesson := range doc.Lessons {
		for _, act := range lesson.Activities {
			out = append(out, act.Text)
		}
	}
	return out
}

func activityTypes(doc units.Unit) []string {
	seen := map[string]bool{}
	var out []string
	for _, lesson := range doc.Lessons {
		for _, act := range lesson.Activities {
			if !seen[act.Type] {
				seen[act.Type] = true
				out = append(out, act.Type)
			}
		}
	}
	return sortedCopy(out)
}

func activityTypeOK(typ string) bool {
	switch typ {
	case units.ActivityListenPick, units.ActivityRecall, units.ActivityProduce,
		units.ActivityCloze, units.ActivityPattern, units.ActivityStory, units.ActivityShadow:
		return true
	default:
		return false
	}
}

func kinds(doc units.Unit) string {
	parts := make([]string, len(doc.Lessons))
	for i, lesson := range doc.Lessons {
		parts[i] = lesson.Kind
	}
	return strings.Join(parts, ",")
}

func tokens(s string) []string {
	var out []string
	for _, field := range strings.Fields(s) {
		field = strings.TrimFunc(field, unicode.IsPunct)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func wordSet(words []string) map[string]bool {
	out := make(map[string]bool, len(words))
	for _, word := range words {
		out[word] = true
	}
	return out
}

func sum(ns []int) int {
	n := 0
	for _, v := range ns {
		n += v
	}
	return n
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
