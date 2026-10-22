package testharness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Reviewer spellings. Audio keys are the exact pointed text, so a missing
// dagesh or shin dot is a different clip.
const (
	pleaseFixed = "בְּבַקָּשָׁה"
	billFixed   = "חֶשְׁבּוֹן"
	familyFixed = "מִשְׁפָּחָה"
	twoFixed    = "שְׁנַיִם"
	eightFixed  = "שְׁמוֹנֶה"

	pleaseBroken = "בְבַקָּשָׁה"
	billBroken   = "חֶשְבּוֹן"
	familyBroken = "מִשְפָּחָה"
	twoBroken    = "שְנַיִם"
	eightBroken  = "שְמוֹנֶה"
)

func TestFixtureNiqqudRepairs(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join(testdataDir(t), "lexemes.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{pleaseFixed, billFixed, familyFixed, twoFixed, eightFixed} {
		if !strings.Contains(text, want) {
			t.Fatalf("lexemes.json missing %q", want)
		}
	}
	root := filepath.Dir(testdataDir(t))
	for _, dir := range []string{
		testdataDir(t),
		filepath.Join(root, "internal", "units", "testdata"),
	} {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			s := string(body)
			for _, bad := range []string{pleaseBroken, billBroken, familyBroken, twoBroken, eightBroken} {
				if strings.Contains(s, bad) {
					rel, _ := filepath.Rel(root, path)
					t.Errorf("%s still has %q", rel, bad)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestPointedShinHasDot requires a shin or sin dot on every ש in a pointed
// fixture word. story.unpointed fixtures are unpointed on purpose.
func TestPointedShinHasDot(t *testing.T) {
	t.Parallel()
	n := 0
	for _, path := range fixtureJSON(t) {
		if strings.Contains(strings.ToLower(filepath.Base(path)), "unpointed") {
			continue
		}
		n += assertPointedShin(t, path)
	}
	if n == 0 {
		t.Fatal("no pointed fixture words")
	}
}

// TestUnitAudioKeysMatchFixtures requires every Hebrew activity string that
// needs a clip to appear byte-identical in audio_present.json or lexemes.json.
func TestUnitAudioKeysMatchFixtures(t *testing.T) {
	t.Parallel()
	keys := map[string]bool{}
	for _, text := range loadAudio(t) {
		keys[text] = true
	}
	for _, item := range loadLexemes(t) {
		keys[item.TextNfc] = true
	}
	dir := filepath.Join(testdataDir(t), "units")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Lessons []struct {
				Activities []struct {
					Text string `json:"text"`
				} `json:"activities"`
			} `json:"lessons"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
		for li, lesson := range doc.Lessons {
			for ai, act := range lesson.Activities {
				if !hebrewLettersPresent(act.Text) {
					continue
				}
				seen++
				if !keys[act.Text] {
					t.Errorf("%s lessons/%d/activities/%d text is not an audio or lexeme key", entry.Name(), li, ai)
				}
			}
		}
	}
	if seen == 0 {
		t.Fatal("unit fixtures have no Hebrew activity text")
	}
}

// TestFixtureHebrewIsNFC checks testdata and the validator goldens with
// Unicode NFC. Audio keys and lexicon rows are compared as stored.
func TestFixtureHebrewIsNFC(t *testing.T) {
	t.Parallel()
	n := 0
	for _, path := range fixtureJSON(t) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		n += assertNormNFC(t, path, v)
	}
	if n == 0 {
		t.Fatal("fixtures have no Hebrew strings")
	}
}

func fixtureJSON(t *testing.T) []string {
	t.Helper()
	root := filepath.Dir(testdataDir(t))
	var out []string
	for _, dir := range []string{testdataDir(t), filepath.Join(root, "internal", "units", "testdata")} {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".json") {
				out = append(out, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(out) == 0 {
		t.Fatal("no fixture json")
	}
	return out
}

func assertPointedShin(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	n := 0
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			for _, word := range hebrewWords(x) {
				if !wordPointed(word) {
					continue
				}
				n++
				if !shinDotted(word) {
					rel, _ := filepath.Rel(filepath.Dir(testdataDir(t)), path)
					t.Errorf("%s pointed word %q has ש without a shin or sin dot", rel, word)
				}
			}
		case []any:
			for _, item := range x {
				walk(item)
			}
		case map[string]any:
			for _, item := range x {
				walk(item)
			}
		}
	}
	walk(v)
	return n
}

func assertNormNFC(t *testing.T, path string, v any) int {
	t.Helper()
	switch x := v.(type) {
	case string:
		if !containsHebrew(x) {
			return 0
		}
		if !norm.NFC.IsNormalString(x) {
			rel, _ := filepath.Rel(filepath.Dir(testdataDir(t)), path)
			t.Errorf("%s is not NFC: %q", rel, x)
		}
		return 1
	case []any:
		n := 0
		for _, item := range x {
			n += assertNormNFC(t, path, item)
		}
		return n
	case map[string]any:
		n := 0
		for _, item := range x {
			n += assertNormNFC(t, path, item)
		}
		return n
	default:
		return 0
	}
}

func hebrewWords(s string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		out = append(out, b.String())
		b.Reset()
	}
	for _, r := range s {
		if hebrewWordRune(r) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

func hebrewWordRune(r rune) bool {
	return (r >= 0x0591 && r <= 0x05C7) || (r >= 0x05D0 && r <= 0x05EA)
}

func wordPointed(word string) bool {
	for _, r := range word {
		if (r >= 0x05B0 && r <= 0x05BD) || r == 0x05C7 {
			return true
		}
	}
	return false
}

func shinDotted(word string) bool {
	rs := []rune(word)
	for i, r := range rs {
		if r != 0x05E9 {
			continue
		}
		dotted := false
		for j := i + 1; j < len(rs); j++ {
			if !unicode.Is(unicode.Mn, rs[j]) {
				break
			}
			if rs[j] == 0x05C1 || rs[j] == 0x05C2 {
				dotted = true
			}
		}
		if !dotted {
			return false
		}
	}
	return true
}

func hebrewLettersPresent(s string) bool {
	for _, r := range s {
		if r >= 0x05D0 && r <= 0x05EA {
			return true
		}
	}
	return false
}
