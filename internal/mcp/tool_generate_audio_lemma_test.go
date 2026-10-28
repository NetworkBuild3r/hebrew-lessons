package mcp_test

import (
	"path/filepath"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// INIT-023/SPEC-006 ac-4: generate_audio with lemmaId clips forms that had none and skips the rest.
func TestGenerateAudioLemmaID(t *testing.T) {
	e := newAP(t)
	lemma := units.Normalize("לִבְחוֹר")
	bocher := units.Normalize("בּוֹחֵר")
	bocheret := units.Normalize("בּוֹחֶרֶת")
	e.exec(`INSERT INTO lexemes (text_nfc, bare, gloss_en, pos) VALUES (?, ?, 'to choose', 'verb')`,
		lemma, units.StripNiqqud(lemma))
	lemmaID := int64(e.count(`SELECT id FROM lexemes WHERE text_nfc = ?`, lemma))

	admin := e.as("static", nil)
	mustOK(t, admin, "upsert_verb_forms", map[string]any{"forms": []any{
		map[string]any{"lemmaId": lemmaID, "tense": "present", "number": "sg", "gender": "m", "formHe": bocher, "translit": "bocher"},
		map[string]any{"lemmaId": lemmaID, "tense": "present", "number": "sg", "gender": "f", "formHe": bocheret, "translit": "bocheret"},
	}})

	tts := &e2eTTS{calls: map[string]int{}}
	gen := &audio.Generator{
		TTS: tts, Store: e.store, Dir: filepath.Join(t.TempDir(), "audio"),
		Voice: audio.DefaultVoiceID, Model: audio.DefaultModel,
	}
	pre, err := gen.Generate(t.Context(), []string{bocher})
	if err != nil || len(pre) != 1 || pre[0].Status != audio.StatusGenerated {
		t.Fatalf("pre %+v %v", pre, err)
	}
	tts.calls = map[string]int{}

	session := e.as("static", e2eAudio{gen: gen})
	raw := mustOK(t, session, "generate_audio", map[string]any{"lemmaId": lemmaID})
	type item struct {
		TextNFC string `json:"textNfc"`
		Status  string `json:"status"`
		AudioID string `json:"audioId"`
	}
	type result struct {
		Results []item `json:"results"`
	}
	got := decodeInto[result](t, raw)
	if len(got.Results) != 2 {
		t.Fatalf("results %s", raw)
	}
	byText := map[string]item{}
	for _, it := range got.Results {
		byText[it.TextNFC] = it
	}
	if byText[bocher].Status != audio.StatusSkipped || byText[bocher].AudioID == "" {
		t.Fatalf("existing form should skip %+v in %s", byText[bocher], raw)
	}
	if byText[bocheret].Status != audio.StatusGenerated || byText[bocheret].AudioID == "" || tts.calls[bocheret] != 1 {
		t.Fatalf("missing form should generate %+v calls %v in %s", byText[bocheret], tts.calls, raw)
	}
	if tts.calls[bocher] != 0 {
		t.Fatalf("provider called for skipped form: %d", tts.calls[bocher])
	}
}
