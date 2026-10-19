package placement

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFixture(t *testing.T) {
	cat, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat) != 80 {
		t.Fatalf("items = %d, want 80", len(cat))
	}
	counts := map[int]int{}
	var pseudo int
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
		if prev, ok := text[it.TextNFC]; ok {
			t.Errorf("%s and %s share pointed text", it.ID, prev)
		}
		text[it.TextNFC] = it.ID
		switch {
		case it.Pseudo:
			pseudo++
			if it.Band != nil || it.Source != "pseudo" {
				t.Errorf("%s: pseudo item band=%v source=%s", it.ID, it.Band, it.Source)
			}
		case it.Band == nil:
			t.Errorf("%s: real item has a null band", it.ID)
		default:
			if *it.Band < 0 || *it.Band > 4 {
				t.Errorf("%s band %d", it.ID, *it.Band)
			}
			counts[*it.Band]++
			wantSource := "freq"
			if *it.Band == 0 {
				wantSource = "tutor"
			}
			if it.Source != wantSource {
				t.Errorf("%s source %s, want %s", it.ID, it.Source, wantSource)
			}
			if it.GlossEN == "" || it.POS == "" {
				t.Errorf("%s missing gloss or pos", it.ID)
			}
			key := unpointed(it.TextNFC)
			if prev, ok := bare[key]; ok {
				t.Errorf("%s and %s share unpointed form %s", it.ID, prev, key)
			}
			bare[key] = it.ID
		}
	}
	for band := 0; band <= 4; band++ {
		if counts[band] != 12 {
			t.Errorf("band %d = %d, want 12", band, counts[band])
		}
	}
	if pseudo != 20 {
		t.Fatalf("pseudo = %d, want 20", pseudo)
	}
	for _, it := range cat {
		if !it.Pseudo {
			continue
		}
		if prev, ok := text[it.TextNFC]; ok && prev != it.ID {
			t.Errorf("pseudo %s matches real text %s", it.ID, prev)
		}
		if prev, ok := bare[unpointed(it.TextNFC)]; ok {
			t.Errorf("pseudo %s matches real unpointed form of %s", it.ID, prev)
		}
	}
}

func TestClientItemHidesBand(t *testing.T) {
	cat, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(cat[0].card())
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, hidden := range []string{"\"band\"", "pseudo", "verified", "source", "\"pos\"", "is_pseudo"} {
		if strings.Contains(s, hidden) {
			t.Fatalf("client card JSON contains %s: %s", hidden, s)
		}
	}
	if !strings.Contains(s, "gloss_en") || !strings.Contains(s, "audio_url") {
		t.Fatalf("client card JSON = %s", s)
	}
}
