package placement

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFixture(t *testing.T) {
	if strings.Contains(string(itemsJSON), "pseudo") {
		t.Fatal("items.json still mentions pseudo-words")
	}
	cat, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat) < 5*BlockSize {
		t.Fatalf("items = %d, want at least %d", len(cat), 5*BlockSize)
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
		if it.Verified {
			t.Errorf("%s verified = true", it.ID)
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
		if counts[band] < BlockSize {
			t.Errorf("band %d = %d, want at least %d", band, counts[band], BlockSize)
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
