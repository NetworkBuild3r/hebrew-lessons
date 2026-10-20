package placement

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
)

func TestFixture(t *testing.T) {
	if strings.Contains(string(itemsJSON), "pseudo") {
		t.Fatal("items.json still mentions pseudo-words")
	}
	cat, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat) < 5*BandMax {
		t.Fatalf("items = %d, want at least %d", len(cat), 5*BandMax)
	}
	counts := map[int]int{}
	ids := map[string]struct{}{}
	text := map[string]string{}
	bare := map[string]string{}
	for _, it := range cat {
		if it.ID == "" || strings.Contains(it.ID, "pseudo") || strings.Contains(it.ID, "band") {
			t.Errorf("id %q reveals the fixture or is empty", it.ID)
		}
		if _, ok := ids[it.ID]; ok {
			t.Errorf("duplicate id %s", it.ID)
		}
		ids[it.ID] = struct{}{}
		if !isNFC(it.TextNFC) {
			t.Errorf("%s text is not NFC", it.ID)
		}
		if !it.Verified {
			t.Errorf("%s verified = false", it.ID)
		}
		if it.Translit == "" || it.GlossEN == "" || it.POS == "" {
			t.Errorf("%s missing gloss, translit, or pos", it.ID)
		}
		switch it.Gender {
		case "", "m", "f", "both":
		default:
			t.Errorf("%s gender %q", it.ID, it.Gender)
		}
		if prev, ok := text[it.TextNFC]; ok {
			t.Errorf("%s and %s share pointed text", it.ID, prev)
		}
		text[it.TextNFC] = it.ID
		if it.Band < 0 || it.Band > 4 {
			t.Errorf("%s band %d", it.ID, it.Band)
		}
		counts[it.Band]++
		wantSource := "freq"
		if it.Band == 0 {
			wantSource = "tutor"
		}
		if it.Source != wantSource {
			t.Errorf("%s source %s, want %s", it.ID, it.Source, wantSource)
		}
		key := unpointed(it.TextNFC)
		if prev, ok := bare[key]; ok {
			t.Errorf("%s and %s share unpointed form %s", it.ID, prev, key)
		}
		bare[key] = it.ID
	}
	for band := 0; band <= 4; band++ {
		if counts[band] < BandMax {
			t.Errorf("band %d = %d, want at least %d", band, counts[band], BandMax)
		}
	}
}

func TestDeckLint(t *testing.T) {
	cat, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range cat {
		if !isNFC(it.TextNFC) {
			t.Errorf("%s text is not NFC", it.ID)
		}
		if it.Translit == "" || it.GlossEN == "" {
			t.Errorf("%s missing translit or gloss", it.ID)
		}
		if strings.Contains(strings.ToLower(it.Translit), "ch") {
			t.Errorf("%s translit %q contains ch", it.ID, it.Translit)
		}
		if (it.POS == "verb" || it.POS == "adjective") && it.Gender != "" {
			t.Errorf("%s %s has gender %q", it.ID, it.POS, it.Gender)
		}
	}
}

// testdata/v2-bare-letters.json is the niqqud-stripped text of each item
// on v2 before the deck grew. letterAllow lists bare spellings that may
// differ from that snapshot. It is empty: every v2 item keeps the letters
// it had on v2. Items added after v2 (SPEC-006) are not in the snapshot and
// are checked against the frequency list by gen instead.
var letterAllow = map[string]string{}

func TestBareLettersMatchV2(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "testdata", "v2-bare-letters.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]string
	if err := json.Unmarshal(body, &snapshot); err != nil {
		t.Fatal(err)
	}
	cat, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]struct{}{}
	for _, it := range cat {
		seen[it.ID] = struct{}{}
		got := unpointed(it.TextNFC)
		if want, ok := letterAllow[it.ID]; ok {
			if got != want {
				t.Errorf("%s bare %s, allowlist %s", it.ID, got, want)
			}
			continue
		}
		want, ok := snapshot[it.ID]
		if !ok {
			// Added after v2. gen checked it against the frequency list.
			continue
		}
		if got != want {
			t.Errorf("%s bare %s, v2 snapshot %s", it.ID, got, want)
		}
	}
	for id := range letterAllow {
		if _, ok := seen[id]; !ok {
			t.Errorf("allowlist %s is not in the deck", id)
		}
	}
	for id := range snapshot {
		if _, ok := seen[id]; !ok {
			t.Errorf("v2 snapshot %s is not in the deck", id)
		}
	}
}

func TestClientCardShowsFrontAndBack(t *testing.T) {
	cat, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(cat[0].card())
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, hidden := range []string{"\"band\"", "pseudo", "verified", "source", "\"pos\"", "is_pseudo", "false_alarm"} {
		if strings.Contains(s, hidden) {
			t.Fatalf("client card JSON contains %s: %s", hidden, s)
		}
	}
	for _, shown := range []string{"text_nfc", "audio_url", "gloss_en", "translit", "gender"} {
		if !strings.Contains(s, shown) {
			t.Fatalf("client card JSON = %s", s)
		}
	}
}

// The tests below are the SPEC-006 build checks. Run them with
//
//	go test ./internal/placement -run Items

const (
	// minVerifiedPerBand is the floor for verified words in a band. The
	// target is 40; a band may hold 36 to 39 when a word could not be
	// confirmed against Pealim.
	minVerifiedPerBand = 36
	// minPerPOSGroup is the floor for nouns and for non-nouns in a band, so
	// a distractor can always match the part of speech of its target.
	minPerPOSGroup = 12
)

var allowedPOS = map[string]bool{"noun": true, "verb": true, "adjective": true, "adverb": true}

// checkDeck returns one line for each rule the deck breaks. An empty result
// means the deck is sound.
func checkDeck(items []fixtureItem) []string {
	var bad []string
	type tally struct{ verified, nouns, others int }
	perBand := map[int]*tally{}
	glosses := map[string]string{}
	for _, it := range items {
		if strings.TrimSpace(it.GlossEN) == "" {
			bad = append(bad, fmt.Sprintf("%s has no gloss", it.ID))
		}
		switch {
		case strings.TrimSpace(it.POS) == "":
			bad = append(bad, fmt.Sprintf("%s has no part of speech", it.ID))
		case !allowedPOS[it.POS]:
			bad = append(bad, fmt.Sprintf("%s part of speech %q is not one of noun, verb, adjective, adverb", it.ID, it.POS))
		}
		if it.POS == "noun" && it.Gender == "" {
			bad = append(bad, fmt.Sprintf("%s is a noun with no gender", it.ID))
		}
		if !isNFC(it.TextNFC) {
			bad = append(bad, fmt.Sprintf("%s text is not NFC", it.ID))
		}
		if g := strings.ToLower(strings.TrimSpace(it.GlossEN)); g != "" {
			if prev, ok := glosses[g]; ok {
				bad = append(bad, fmt.Sprintf("%s and %s share the gloss %q", it.ID, prev, it.GlossEN))
			}
			glosses[g] = it.ID
		}
		b := perBand[it.Band]
		if b == nil {
			b = &tally{}
			perBand[it.Band] = b
		}
		if it.Verified {
			b.verified++
		}
		if it.POS == "noun" {
			b.nouns++
		} else {
			b.others++
		}
	}
	for band := 0; band < bands; band++ {
		b := perBand[band]
		if b == nil {
			b = &tally{}
		}
		if b.verified < minVerifiedPerBand {
			bad = append(bad, fmt.Sprintf("band %d has %d verified words, want at least %d", band, b.verified, minVerifiedPerBand))
		}
		if b.nouns < minPerPOSGroup {
			bad = append(bad, fmt.Sprintf("band %d has %d nouns, want at least %d", band, b.nouns, minPerPOSGroup))
		}
		if b.others < minPerPOSGroup {
			bad = append(bad, fmt.Sprintf("band %d has %d non-nouns, want at least %d", band, b.others, minPerPOSGroup))
		}
	}
	sort.Strings(bad)
	return bad
}

// auditAudio returns one line for each required clip with no audio asset.
func auditAudio(clips []Clip, available map[string]struct{}) []string {
	var bad []string
	for _, c := range MissingAudio(clips, available) {
		bad = append(bad, fmt.Sprintf("%s %s has no audio asset for %q", c.Kind, c.ID, c.Text))
	}
	return bad
}

func sourceDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Dir(file)
}

func readManifest(t *testing.T) (ClipManifest, []byte) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(sourceDir(t), "audio-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(body)
	if err != nil {
		t.Fatal(err)
	}
	return m, body
}

func TestItemsDeck(t *testing.T) {
	cat, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range checkDeck(cat) {
		t.Error(line)
	}
	counts := map[int]int{}
	for _, it := range cat {
		counts[it.Band]++
	}
	for band := 0; band < bands; band++ {
		t.Logf("band %d: %d words", band, counts[band])
	}
}

// fixtureDeck is a sound deck: five bands of 36 verified words, half nouns.
func fixtureDeck() []fixtureItem {
	var items []fixtureItem
	for band := 0; band < bands; band++ {
		for i := 0; i < 36; i++ {
			it := fixtureItem{
				ID:       fmt.Sprintf("w%d-%d", band, i),
				TextNFC:  fmt.Sprintf("שָׁלוֹם%c", rune('א'+i)),
				Band:     band,
				GlossEN:  fmt.Sprintf("gloss %d %d", band, i),
				Translit: "x",
				POS:      "verb",
				Verified: true,
			}
			if i%2 == 0 {
				it.POS = "noun"
				it.Gender = "m"
			}
			items = append(items, it)
		}
	}
	return items
}

func TestItemsDeckRulesFail(t *testing.T) {
	if bad := checkDeck(fixtureDeck()); len(bad) != 0 {
		t.Fatalf("the sound fixture deck fails: %v", bad)
	}
	cases := []struct {
		name   string
		mutate func(items []fixtureItem) []fixtureItem
		want   string
	}{
		{"no gloss", func(items []fixtureItem) []fixtureItem { items[3].GlossEN = ""; return items }, "has no gloss"},
		{"no part of speech", func(items []fixtureItem) []fixtureItem { items[4].POS = ""; return items }, "has no part of speech"},
		{"unknown part of speech", func(items []fixtureItem) []fixtureItem { items[4].POS = "particle"; return items }, "is not one of"},
		{"noun without gender", func(items []fixtureItem) []fixtureItem { items[0].Gender = ""; return items }, "noun with no gender"},
		{"not NFC", func(items []fixtureItem) []fixtureItem {
			// The shin dot written before the qamats is not canonical order.
			items[5].TextNFC = "ש\u05C1\u05B8"
			return items
		}, "is not NFC"},
		{"shared gloss", func(items []fixtureItem) []fixtureItem { items[7].GlossEN = items[6].GlossEN; return items }, "share the gloss"},
		{"too few nouns", func(items []fixtureItem) []fixtureItem {
			n := 0
			for i := range items {
				if items[i].Band == 2 && items[i].POS == "noun" {
					n++
					if n > 6 {
						items[i].POS = "verb"
						items[i].Gender = ""
					}
				}
			}
			return items
		}, "band 2 has 6 nouns"},
		{"too few non-nouns", func(items []fixtureItem) []fixtureItem {
			n := 0
			for i := range items {
				if items[i].Band == 4 && items[i].POS == "verb" {
					n++
					if n > 11 {
						items[i].POS = "noun"
						items[i].Gender = "f"
					}
				}
			}
			return items
		}, "band 4 has 11 non-nouns"},
		{"band below 36 verified", func(items []fixtureItem) []fixtureItem { items[40].Verified = false; return items }, "band 1 has 35 verified words"},
		{"band missing", func(items []fixtureItem) []fixtureItem {
			var kept []fixtureItem
			for _, it := range items {
				if it.Band != 3 {
					kept = append(kept, it)
				}
			}
			return kept
		}, "band 3 has 0 verified words"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bad := checkDeck(c.mutate(fixtureDeck()))
			if len(bad) == 0 {
				t.Fatal("deck passed, want a failure")
			}
			if !strings.Contains(strings.Join(bad, "\n"), c.want) {
				t.Fatalf("failures = %v, want one containing %q", bad, c.want)
			}
		})
	}
}

func TestItemsNFCRule(t *testing.T) {
	// The NFC rule only means something if the check can tell the forms apart.
	decomposed := "ש\u05C1\u05B8" // shin dot before qamats: not canonical order
	if isNFC(decomposed) {
		t.Fatalf("%q reads as NFC, so the NFC check could not fail", decomposed)
	}
	if !isNFC(audio.NormalizeText(decomposed)) {
		t.Fatalf("NormalizeText(%q) is not NFC", decomposed)
	}
}

func TestItemsLetters(t *testing.T) {
	letters, err := Letters()
	if err != nil {
		t.Fatal(err)
	}
	if len(letters) != 6 {
		t.Fatalf("letter items = %d, want 6", len(letters))
	}
	ids := map[string]struct{}{}
	for _, l := range letters {
		if l.ID == "" || l.Sound == "" || l.LetterNFC == "" || l.SyllableText == "" {
			t.Errorf("%+v has an empty field", l)
		}
		if _, ok := ids[l.ID]; ok {
			t.Errorf("duplicate letter id %s", l.ID)
		}
		ids[l.ID] = struct{}{}
		for _, s := range append([]string{l.LetterNFC, l.SyllableText}, l.Distractors...) {
			if !isNFC(s) {
				t.Errorf("%s: %q is not NFC", l.ID, s)
			}
			if s != audio.NormalizeText(s) {
				t.Errorf("%s: %q is not the audio_assets text form", l.ID, s)
			}
		}
		if len(l.Distractors) != 3 {
			t.Errorf("%s has %d distractors, want 3", l.ID, len(l.Distractors))
		}
		seen := map[string]struct{}{l.LetterNFC: {}}
		for _, d := range l.Distractors {
			if _, ok := seen[d]; ok {
				t.Errorf("%s: distractor %q repeats the answer or another distractor", l.ID, d)
			}
			seen[d] = struct{}{}
		}
		// A letter with its vowel: one base letter and at least one point.
		runes := []rune(l.LetterNFC)
		if len(runes) < 2 || runes[0] < 0x05D0 || runes[0] > 0x05EA {
			t.Errorf("%s: %q is not a letter with a vowel", l.ID, l.LetterNFC)
		}
	}
}

func TestItemsAudioManifestMatchesDeck(t *testing.T) {
	_, committed := readManifest(t)
	want, err := BuildManifest(itemsJSON, lettersJSON)
	if err != nil {
		t.Fatal(err)
	}
	body, err := want.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != string(committed) {
		t.Fatal("audio-manifest.json is out of date with items.json and letters.json; run: go run ./internal/placement/gen")
	}
	m, _ := readManifest(t)
	if m.Schema != ManifestSchema || m.Count != len(m.Clips) {
		t.Fatalf("manifest schema %q count %d with %d clips", m.Schema, m.Count, len(m.Clips))
	}
	texts := map[string]string{}
	for _, c := range m.Clips {
		if c.Kind != ClipWord && c.Kind != ClipLetter {
			t.Errorf("%s: kind %q", c.ID, c.Kind)
		}
		if c.Text == "" || c.Text != audio.NormalizeText(c.Text) || !isNFC(c.Text) {
			t.Errorf("%s: text %q is not the audio_assets text form", c.ID, c.Text)
		}
		if prev, ok := texts[c.Text]; ok {
			t.Errorf("%s and %s need the same clip text %q", c.ID, prev, c.Text)
		}
		texts[c.Text] = c.ID
	}
	cat, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	letters, err := Letters()
	if err != nil {
		t.Fatal(err)
	}
	if m.Count != len(cat)+len(letters) {
		t.Fatalf("manifest has %d clips, deck needs %d", m.Count, len(cat)+len(letters))
	}
}

func TestItemsAudioSilentWordFails(t *testing.T) {
	m, _ := readManifest(t)
	var texts []string
	for _, c := range m.Clips {
		texts = append(texts, c.Text)
	}
	if bad := auditAudio(m.Clips, AvailableSet(texts)); len(bad) != 0 {
		t.Fatalf("every clip is available, yet the audit says %v", bad)
	}

	// One silent word: drop its clip from the available set.
	silent := m.Clips[0]
	if silent.Kind != ClipWord {
		t.Fatalf("first clip is %s, want a word", silent.Kind)
	}
	var without []string
	for _, c := range m.Clips {
		if c.ID != silent.ID {
			without = append(without, c.Text)
		}
	}
	bad := auditAudio(m.Clips, AvailableSet(without))
	if len(bad) != 1 || !strings.Contains(bad[0], silent.ID) {
		t.Fatalf("audit = %v, want exactly the silent word %s", bad, silent.ID)
	}

	// A letter item with no clip fails the same way.
	var letterClip Clip
	for _, c := range m.Clips {
		if c.Kind == ClipLetter {
			letterClip = c
			break
		}
	}
	var withoutLetter []string
	for _, c := range m.Clips {
		if c.ID != letterClip.ID {
			withoutLetter = append(withoutLetter, c.Text)
		}
	}
	bad = auditAudio(m.Clips, AvailableSet(withoutLetter))
	if len(bad) != 1 || !strings.Contains(bad[0], letterClip.ID) {
		t.Fatalf("audit = %v, want exactly the silent letter %s", bad, letterClip.ID)
	}

	// Available text is matched after normalization, as audio_assets stores it.
	if bad := auditAudio([]Clip{{Kind: ClipWord, ID: "x", Text: audio.NormalizeText("שָׁלוֹם")}}, AvailableSet([]string{" שָׁלוֹם "})); len(bad) != 0 {
		t.Fatalf("a padded available text did not match: %v", bad)
	}
}

// availableAudioFile lists the clip texts that have an audio_assets row.
// It is committed in audio phase 2, after the clips are generated and
// imported. Its shape is {"texts": ["<pointed text>", ...]}.
const availableAudioFile = "audio-available.json"

func TestItemsAudioRealDeck(t *testing.T) {
	path := filepath.Join(sourceDir(t), "testdata", availableAudioFile)
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skipf("audio phase 2 is pending: testdata/%s is not committed yet, so the real deck cannot be checked for silent words. Generate and import the clips in audio-manifest.json, then commit that file", availableAudioFile)
	}
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Texts []string `json:"texts"`
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Texts) == 0 {
		t.Fatalf("%s lists no clip texts", availableAudioFile)
	}
	m, _ := readManifest(t)
	if bad := auditAudio(m.Clips, AvailableSet(doc.Texts)); len(bad) != 0 {
		t.Fatalf("%d clips are silent:\n%s", len(bad), strings.Join(bad, "\n"))
	}
}
