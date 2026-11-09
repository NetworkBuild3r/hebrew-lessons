package mcp_test

import (
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// INIT-023/SPEC-006 ac-3: lookup for לִבְחוֹר returns present forms with hasAudio.
func TestLookupReturnsVerbFormsWithHasAudio(t *testing.T) {
	e := newAP(t)
	admin := e.as("static", nil)
	lemma := units.Normalize("לִבְחוֹר")
	bocher := units.Normalize("בּוֹחֵר")
	bocheret := units.Normalize("בּוֹחֶרֶת")
	e.exec(`INSERT INTO lexemes (text_nfc, bare, gloss_en, pos, translit) VALUES (?, ?, 'to choose', 'verb', 'livchor')`,
		lemma, units.StripNiqqud(lemma))
	id := int64(e.count(`SELECT id FROM lexemes WHERE text_nfc = ?`, lemma))

	mustOK(t, admin, "upsert_verb_forms", map[string]any{"forms": []any{
		map[string]any{"lemmaId": id, "tense": "present", "number": "sg", "gender": "m", "formHe": bocher, "translit": "bocher"},
		map[string]any{"lemmaId": id, "tense": "present", "number": "sg", "gender": "f", "formHe": bocheret, "translit": "bocheret"},
		map[string]any{"lemmaId": id, "tense": "infinitive", "formHe": lemma, "translit": "livchor"},
	}})
	audioID := strings.Repeat("cd", 32)
	e.exec(`INSERT INTO audio_assets (id, text_nfc, voice, model, bytes, created_at) VALUES (?, ?, 'v', 'm', 4, ?)`,
		audioID, bocher, db.FormatTime(apNow))

	raw := mustOK(t, admin, "lookup", map[string]any{"query": lemma})
	if !strings.Contains(raw, `"tense":"present"`) || !strings.Contains(raw, bocher) || !strings.Contains(raw, bocheret) {
		t.Fatalf("lookup missing present forms: %s", raw)
	}
	if !strings.Contains(raw, `"hasAudio":true`) || !strings.Contains(raw, `"hasAudio":false`) {
		t.Fatalf("lookup must set hasAudio per form: %s", raw)
	}
	if !strings.Contains(raw, `"he":"`+bocher+`"`) || !strings.Contains(raw, `"translit":"bocher"`) {
		t.Fatalf("lookup form fields: %s", raw)
	}
}
