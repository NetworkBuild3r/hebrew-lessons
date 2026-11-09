package mcp_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// INIT-023/SPEC-006 ac-1, ac-2, ac-5
func TestUpsertNounFormsAdminOnlyAndMissingLemma(t *testing.T) {
	e := newAP(t)
	learner := e.as("learner-a", nil)
	admin := e.as("static", nil)
	house := units.Normalize("בַּיִת")
	batim := units.Normalize("בָּתִּים")
	lemma := e.ids[apShalom]

	msg := mustFail(t, learner, "upsert_noun_forms", map[string]any{"forms": []any{
		map[string]any{"lemmaId": lemma, "kind": "singular", "formHe": house},
	}})
	if !strings.Contains(msg, "admin") {
		t.Fatalf("learner upsert_noun_forms = %s", msg)
	}
	if e.count(`SELECT COUNT(*) FROM noun_forms`) != 0 {
		t.Fatal("learner wrote a noun form")
	}

	errText := mustFail(t, admin, "upsert_noun_forms", map[string]any{"forms": []any{
		map[string]any{"lemmaId": lemma, "kind": "singular", "formHe": house},
		map[string]any{"lemmaId": 99999, "kind": "plural", "number": "pl", "formHe": batim},
	}})
	if e.count(`SELECT COUNT(*) FROM noun_forms`) != 0 {
		t.Fatalf("missing lemma wrote rows: %s", errText)
	}
	if !strings.Contains(errText, "99999") || !strings.Contains(errText, "nothing was written") {
		t.Fatalf("missing lemma must be named: %s", errText)
	}

	mustOK(t, admin, "upsert_noun_forms", map[string]any{"forms": []any{
		map[string]any{"lemmaId": lemma, "kind": "singular", "formHe": house, "translit": "bayit"},
		map[string]any{"lemmaId": lemma, "kind": "plural", "number": "pl", "formHe": batim, "translit": "batim"},
		map[string]any{
			"lemmaId": lemma, "kind": "possessive", "person": "1", "ownerNumber": "sg", "number": "sg",
			"formHe": units.Normalize("בֵּיתִי"), "translit": "beiti",
		},
		map[string]any{
			"lemmaId": lemma, "kind": "agreement", "number": "sg", "gender": "f",
			"formHe": units.Normalize("טוֹבָה"), "translit": "tova",
		},
	}})
	if e.count(`SELECT COUNT(*) FROM noun_forms`) != 4 {
		t.Fatal("valid noun forms were not stored")
	}

	tools, err := admin.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range tools.Tools {
		if tool.Name != "upsert_noun_forms" {
			continue
		}
		found = true
		if tool.Annotations == nil || tool.Annotations.ReadOnlyHint {
			t.Fatalf("upsert_noun_forms annotations %#v", tool.Annotations)
		}
		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		raw := string(schema)
		for _, need := range []string{
			`"lemmaId"`, `"kind"`, `"number"`, `"person"`, `"gender"`, `"ownerNumber"`,
			`"formHe"`, `"translit"`, `"formLexemeId"`, "singular", "possessive", "agreement",
		} {
			if !strings.Contains(raw, need) {
				t.Fatalf("upsert_noun_forms schema missing %s: %s", need, raw)
			}
		}
		if !strings.Contains(tool.Description, "סֵפֶר") || !strings.Contains(tool.Description, "רַע") {
			t.Fatalf("description needs original possessive and agreement examples: %s", tool.Description)
		}
	}
	if !found {
		t.Fatal("upsert_noun_forms missing from tool list")
	}
}
