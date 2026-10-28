package mcpstore

import (
	"context"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// INIT-023/SPEC-006: mcpstore form reads and lookup attach forms.
func TestFormReadsAndLookupForms(t *testing.T) {
	ctx := context.Background()
	store, raw := openAdapter(t, nil)
	lemma := units.Normalize("לִבְחוֹר")
	bocher := units.Normalize("בּוֹחֵר")
	house := units.Normalize("בַּיִת")
	batim := units.Normalize("בָּתִּים")

	verbID := mustLexeme(t, raw, lemma)
	if _, err := raw.ExecContext(ctx, `UPDATE lexemes SET pos = 'verb', gloss_en = 'to choose' WHERE id = ?`, verbID); err != nil {
		t.Fatal(err)
	}
	nounID := mustLexeme(t, raw, house)
	if _, err := raw.ExecContext(ctx, `UPDATE lexemes SET pos = 'noun', gloss_en = 'house' WHERE id = ?`, nounID); err != nil {
		t.Fatal(err)
	}

	if err := store.UpsertVerbForms(ctx, []db.VerbFormInput{
		{LemmaID: verbID, Tense: "present", Number: "sg", Gender: "m", FormHe: bocher, Translit: "bocher"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertNounForms(ctx, []db.NounFormInput{
		{LemmaID: nounID, Kind: "singular", FormHe: house, Translit: "bayit"},
		{LemmaID: nounID, Kind: "plural", Number: "pl", FormHe: batim, Translit: "batim"},
	}); err != nil {
		t.Fatal(err)
	}

	verbs, err := store.VerbFormsFor(ctx, verbID)
	if err != nil || len(verbs) != 1 || verbs[0].FormHe != bocher {
		t.Fatalf("VerbFormsFor %+v %v", verbs, err)
	}
	nouns, err := store.NounFormsFor(ctx, nounID)
	if err != nil || len(nouns) != 2 {
		t.Fatalf("NounFormsFor %+v %v", nouns, err)
	}

	hits, _, err := store.SearchLexemes(ctx, lemma, 8)
	if err != nil || len(hits) != 1 || len(hits[0].Forms) != 1 {
		t.Fatalf("verb lookup %+v %v", hits, err)
	}
	if hits[0].Forms[0].Tense != "present" || hits[0].Forms[0].He != bocher || hits[0].Forms[0].HasAudio {
		t.Fatalf("verb form %+v", hits[0].Forms[0])
	}

	nounHits, _, err := store.SearchLexemes(ctx, house, 8)
	if err != nil || len(nounHits) != 1 || len(nounHits[0].Forms) != 2 {
		t.Fatalf("noun lookup %+v %v", nounHits, err)
	}
	if nounHits[0].Forms[0].Kind != "singular" || nounHits[0].Forms[1].Kind != "plural" {
		t.Fatalf("noun form order %+v", nounHits[0].Forms)
	}
}
