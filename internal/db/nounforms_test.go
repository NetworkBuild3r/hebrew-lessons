package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// INIT-023/SPEC-005 ac-2: an upsert with one missing lemma writes nothing.
func TestUpsertNounFormsMissingLemmaWritesNothing(t *testing.T) {
	when := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	house := units.Normalize("בַּיִת")
	res, err := s.conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en, pos) VALUES (?, 'house', 'noun')`, house)
	if err != nil {
		t.Fatal(err)
	}
	lemma, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	err = s.UpsertNounForms(ctx, []NounFormInput{
		{LemmaID: lemma, Kind: "singular", FormHe: house, Translit: "bayit"},
		{LemmaID: 99999, Kind: "plural", Number: "pl", FormHe: units.Normalize("בָּתִּים"), Translit: "batim"},
	})
	var missing MissingLemmaError
	if !errors.As(err, &missing) || missing.ID != 99999 {
		t.Fatalf("err = %v", err)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM noun_forms`); n != 0 {
		t.Fatalf("a missing lemma wrote rows: %d", n)
	}
}

// INIT-023/SPEC-005 ac-3: a repeat of the same key replaces form_he.
func TestUpsertNounFormsReplaceFormHe(t *testing.T) {
	when := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	house := units.Normalize("בַּיִת")
	res, err := s.conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en, pos) VALUES (?, 'house', 'noun')`, house)
	if err != nil {
		t.Fatal(err)
	}
	lemma, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	first := units.Normalize("בַּיִת")
	second := units.Normalize("הַבַּיִת")
	if err := s.UpsertNounForms(ctx, []NounFormInput{
		{LemmaID: lemma, Kind: "definite", FormHe: first, Translit: "bayit"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertNounForms(ctx, []NounFormInput{
		{LemmaID: lemma, Kind: "definite", FormHe: second, Translit: "habayit"},
	}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM noun_forms`); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
	forms, err := s.NounFormsFor(ctx, lemma)
	if err != nil {
		t.Fatal(err)
	}
	if len(forms) != 1 || forms[0].FormHe != second || forms[0].Translit != "habayit" {
		t.Fatalf("forms = %#v", forms)
	}
}

// INIT-023/SPEC-005 ac-4: NounFormsFor returns the fixed order for every kind.
func TestNounFormsForFixedOrder(t *testing.T) {
	when := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	house := units.Normalize("בַּיִת")
	res, err := s.conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en, pos) VALUES (?, 'house', 'noun')`, house)
	if err != nil {
		t.Fatal(err)
	}
	lemma, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	// Insert out of reading order so ORDER BY is what we are testing.
	rows := []NounFormInput{
		{LemmaID: lemma, Kind: "agreement", Number: "pl", Gender: "f", FormHe: units.Normalize("טוֹבוֹת"), Translit: "tovot"},
		{LemmaID: lemma, Kind: "possessive", Person: "3", Gender: "f", OwnerNumber: "sg", Number: "sg", FormHe: units.Normalize("בֵּיתָהּ"), Translit: "beita"},
		{LemmaID: lemma, Kind: "construct", Number: "pl", FormHe: units.Normalize("בָּתֵּי"), Translit: "batei"},
		{LemmaID: lemma, Kind: "plural", Number: "pl", FormHe: units.Normalize("בָּתִּים"), Translit: "batim"},
		{LemmaID: lemma, Kind: "possessive", Person: "1", OwnerNumber: "sg", Number: "sg", FormHe: units.Normalize("בֵּיתִי"), Translit: "beiti"},
		{LemmaID: lemma, Kind: "agreement", Number: "sg", Gender: "m", FormHe: units.Normalize("טוֹב"), Translit: "tov"},
		{LemmaID: lemma, Kind: "definite", FormHe: units.Normalize("הַבַּיִת"), Translit: "habayit"},
		{LemmaID: lemma, Kind: "construct", Number: "sg", FormHe: units.Normalize("בֵּית"), Translit: "beit"},
		{LemmaID: lemma, Kind: "agreement", Number: "pl", Gender: "m", FormHe: units.Normalize("טוֹבִים"), Translit: "tovim"},
		{LemmaID: lemma, Kind: "possessive", Person: "2", Gender: "m", OwnerNumber: "sg", Number: "sg", FormHe: units.Normalize("בֵּיתְךָ"), Translit: "beitkha"},
		{LemmaID: lemma, Kind: "singular", FormHe: house, Translit: "bayit"},
		{LemmaID: lemma, Kind: "agreement", Number: "sg", Gender: "f", FormHe: units.Normalize("טוֹבָה"), Translit: "tova"},
		{LemmaID: lemma, Kind: "possessive", Person: "1", OwnerNumber: "pl", Number: "sg", FormHe: units.Normalize("בֵּיתֵנוּ"), Translit: "beiteinu"},
	}
	if err := s.UpsertNounForms(ctx, rows); err != nil {
		t.Fatal(err)
	}
	forms, err := s.NounFormsFor(ctx, lemma)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		Kind, Number, Person, Gender, OwnerNumber string
	}{
		{"singular", "", "", "", ""},
		{"definite", "", "", "", ""},
		{"plural", "pl", "", "", ""},
		{"construct", "sg", "", "", ""},
		{"construct", "pl", "", "", ""},
		{"possessive", "sg", "1", "", "sg"},
		{"possessive", "sg", "1", "", "pl"},
		{"possessive", "sg", "2", "m", "sg"},
		{"possessive", "sg", "3", "f", "sg"},
		{"agreement", "sg", "", "m", ""},
		{"agreement", "sg", "", "f", ""},
		{"agreement", "pl", "", "m", ""},
		{"agreement", "pl", "", "f", ""},
	}
	if len(forms) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(forms), len(want), forms)
	}
	for i, w := range want {
		got := forms[i]
		if got.Kind != w.Kind || got.Number != w.Number || got.Person != w.Person || got.Gender != w.Gender || got.OwnerNumber != w.OwnerNumber {
			t.Fatalf("forms[%d] = kind=%s number=%s person=%s gender=%s owner=%s, want %#v",
				i, got.Kind, got.Number, got.Person, got.Gender, got.OwnerNumber, w)
		}
	}
}

// INIT-023/SPEC-005 ac-5: a form with a clip returns its audio id; without, empty.
func TestNounAndVerbFormsAudioID(t *testing.T) {
	when := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()

	house := units.Normalize("בַּיִת")
	batim := units.Normalize("בָּתִּים")
	hear := units.Normalize("שָׁמַע")
	past := units.Normalize("שָׁמַעְתִּי")
	present := units.Normalize("שׁוֹמֵעַ")

	clipHouse := "a3f1c0d2e4b6a8c0d2e4f6a8b0c2d4e6f8a0b2c4d6e8f0a2b4c6d8e0f2a4b6c8"
	clipPast := "b4e2d1c3f5a7b9c1d3e5f7a9b1c3d5e7f9a1b3c5d7e9f1a3b5c7d9e1f3a5b7c9"
	ok, err := s.InsertAudio(ctx, Asset{ID: clipHouse, TextNFC: house, Voice: "v", Model: "m", CreatedAt: when})
	if err != nil || !ok {
		t.Fatalf("house clip: ok=%v err=%v", ok, err)
	}
	ok, err = s.InsertAudio(ctx, Asset{ID: clipPast, TextNFC: past, Voice: "v", Model: "m", CreatedAt: when})
	if err != nil || !ok {
		t.Fatalf("past clip: ok=%v err=%v", ok, err)
	}

	res, err := s.conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en, pos) VALUES (?, 'house', 'noun')`, house)
	if err != nil {
		t.Fatal(err)
	}
	houseID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.conn.ExecContext(ctx, `UPDATE lexemes SET audio_id = ? WHERE id = ?`, clipHouse, houseID); err != nil {
		t.Fatal(err)
	}
	res, err = s.conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en, pos) VALUES (?, 'houses', 'noun')`, batim)
	if err != nil {
		t.Fatal(err)
	}
	batimID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	res, err = s.conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en, pos) VALUES (?, 'hear', 'verb')`, hear)
	if err != nil {
		t.Fatal(err)
	}
	verbID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpsertNounForms(ctx, []NounFormInput{
		{LemmaID: houseID, Kind: "singular", FormHe: house, Translit: "bayit", FormLexemeID: houseID},
		{LemmaID: houseID, Kind: "plural", Number: "pl", FormHe: batim, Translit: "batim", FormLexemeID: batimID},
	}); err != nil {
		t.Fatal(err)
	}
	nouns, err := s.NounFormsFor(ctx, houseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(nouns) != 2 {
		t.Fatalf("nouns = %#v", nouns)
	}
	if nouns[0].AudioID != clipHouse {
		t.Fatalf("singular audio = %q, want %q", nouns[0].AudioID, clipHouse)
	}
	if nouns[1].AudioID != "" {
		t.Fatalf("plural without clip audio = %q, want empty", nouns[1].AudioID)
	}

	if err := s.UpsertVerbForms(ctx, []VerbFormInput{
		{LemmaID: verbID, Tense: "past", Person: "1", Number: "sg", FormHe: past, Translit: "shamati"},
		{LemmaID: verbID, Tense: "present", Person: "1", Number: "sg", Gender: "m", FormHe: present, Translit: "shomea"},
	}); err != nil {
		t.Fatal(err)
	}
	verbs, err := s.VerbFormsFor(ctx, verbID)
	if err != nil {
		t.Fatal(err)
	}
	if len(verbs) != 2 {
		t.Fatalf("verbs = %#v", verbs)
	}
	if verbs[0].Tense != "present" || verbs[0].AudioID != "" {
		t.Fatalf("present without clip = %#v", verbs[0])
	}
	if verbs[1].Tense != "past" || verbs[1].AudioID != clipPast {
		t.Fatalf("past with clip = %#v", verbs[1])
	}
}
