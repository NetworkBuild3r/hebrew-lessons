package db

import (
	"context"
	"errors"
	"io/fs"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

func TestMistakesMigrationKeepsExistingRows(t *testing.T) {
	ctx := context.Background()
	conn := openRaw(t)
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	when := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, name := range names {
		version, err := versionOf(name)
		if err != nil {
			t.Fatal(err)
		}
		if version >= 14 {
			continue
		}
		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := apply(ctx, conn, name, version, string(body), when); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('keep14@example.com', 'sub-keep14', 'learner', ?)`, FormatTime(when))
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc, gloss_en, pos) VALUES (?, 'hear', 'verb')`, units.Normalize("\u05e9\u05b8\u05c1\u05de\u05b7\u05e2"))
	if err := m6Migrate(conn); err != nil {
		t.Fatal(err)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM users WHERE email = 'keep14@example.com'`); n != 1 {
		t.Fatalf("users = %d", n)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM lexemes`); n != 1 {
		t.Fatalf("lexemes = %d", n)
	}
	for _, table := range []string{"learner_mistakes", "common_mistakes", "verb_forms", "verse_notes"} {
		if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = '`+table+`'`); n != 1 {
			t.Fatalf("missing %s", table)
		}
	}
	cols := m6Dump(t, conn, `SELECT name FROM pragma_table_info('common_mistakes') ORDER BY name`)
	for _, row := range cols {
		if strings.Contains(row[0], "learner") {
			t.Fatalf("common_mistakes has a learner column: %v", cols)
		}
	}
}

func TestMistakeRepeatStaysOneRowPerLearner(t *testing.T) {
	when := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	a, err := s.InsertUser(ctx, "a@example.com", "sub-a", "learner")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.InsertUser(ctx, "b@example.com", "sub-b", "learner")
	if err != nil {
		t.Fatal(err)
	}
	wrong := units.Normalize("\u05d0\u05b2\u05e0\u05b4\u05d9 \u05d4\u05d5\u05b9\u05dc\u05b6\u05db\u05b6\u05ea")
	right := units.Normalize("\u05d0\u05b2\u05e0\u05b4\u05d9 \u05d4\u05d5\u05b9\u05dc\u05b6\u05db\u05b6\u05ea")
	// The two forms differ by one letter so a repeat is the same pair.
	right = units.Normalize("\u05d0\u05b2\u05e0\u05b4\u05d9 \u05d4\u05d5\u05b9\u05dc\u05b6\u05db\u05b6\u05ea")
	wrong = units.Normalize("\u05d0\u05b2\u05e0\u05b4\u05d9 \u05d4\u05d5\u05b9\u05dc\u05b5\u05da\u05b0")
	row := MistakeInput{Wrong: wrong, Right: right, Note: "agreement", Source: "tutor"}
	if err := s.RecordMistakes(ctx, a, []MistakeInput{row}); err != nil {
		t.Fatal(err)
	}
	s.clock = fixedClock{when.Add(time.Hour)}
	if err := s.RecordMistakes(ctx, a, []MistakeInput{row}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordMistakes(ctx, b, []MistakeInput{row}); err != nil {
		t.Fatal(err)
	}
	got, err := s.MistakesFor(ctx, a, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Times != 2 || got[0].Wrong != wrong || got[0].Right != right {
		t.Fatalf("learner a = %#v", got)
	}
	if got[0].LastAt == got[0].FirstAt {
		t.Fatal("a repeat did not move last_at")
	}
	other, err := s.MistakesFor(ctx, b, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 1 || other[0].Times != 1 || other[0].ID == got[0].ID {
		t.Fatalf("learner b = %#v", other)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM common_mistakes`); n != 0 {
		t.Fatalf("a personal mistake was copied into the shared list: %d", n)
	}
}

func TestCommonMistakesHaveNoLearnerAndVerbFormsOrder(t *testing.T) {
	when := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	if err := s.UpsertPatterns(ctx, []PatternInput{{
		Slug: "present-tense", Title: "Present", Summary: "The verb agrees.",
	}}); err != nil {
		t.Fatal(err)
	}
	hear := units.Normalize("\u05e9\u05b8\u05c1\u05de\u05b7\u05e2")
	past := units.Normalize("\u05e9\u05b8\u05c1\u05de\u05b7\u05e2\u05b0\u05ea\u05bc\u05b4\u05d9")
	present := units.Normalize("\u05e9\u05c1\u05d5\u05b9\u05de\u05b5\u05e2\u05b7")
	res, err := s.conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en, pos) VALUES (?, 'hear', 'verb')`, hear)
	if err != nil {
		t.Fatal(err)
	}
	lemma, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertCommonMistakes(ctx, []CommonMistakeInput{{
		Pattern: "present-tense", Wrong: past, Right: present, Explain: "Present, not past.", Status: "published",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertCommonMistakes(ctx, []CommonMistakeInput{{
		Pattern: "present-tense", Wrong: past, Right: present, Explain: "The present form.", Status: "draft",
	}}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM common_mistakes`); n != 1 {
		t.Fatalf("common rows = %d, want 1", n)
	}
	pub, err := s.CommonMistakesFor(ctx, "present-tense", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != 0 {
		t.Fatalf("a draft was returned as published: %#v", pub)
	}
	if err := s.UpsertCommonMistakes(ctx, []CommonMistakeInput{{
		Pattern: "present-tense", Wrong: past, Right: present, Explain: "The present form.", Status: "published",
	}}); err != nil {
		t.Fatal(err)
	}
	pub, err = s.CommonMistakesFor(ctx, "present-tense", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != 1 || pub[0].Explain != "The present form." {
		t.Fatalf("published = %#v", pub)
	}

	future := units.Normalize("\u05d0\u05b6\u05e9\u05b0\u05c1\u05de\u05b7\u05e2")
	if err := s.UpsertVerbForms(ctx, []VerbFormInput{
		{LemmaID: lemma, Tense: "future", Person: "1", Number: "sg", FormHe: future, Translit: "eshma"},
		{LemmaID: lemma, Tense: "present", Person: "1", Number: "sg", Gender: "m", FormHe: present, Translit: "shomea"},
		{LemmaID: lemma, Tense: "past", Person: "1", Number: "sg", FormHe: past, Translit: "shamati"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertVerbForms(ctx, []VerbFormInput{
		{LemmaID: lemma, Tense: "present", Person: "1", Number: "sg", Gender: "m", FormHe: present, Translit: "shomea"},
	}); err != nil {
		t.Fatal(err)
	}
	forms, err := s.VerbFormsFor(ctx, lemma)
	if err != nil {
		t.Fatal(err)
	}
	if len(forms) != 3 || forms[0].Tense != "present" || forms[1].Tense != "past" || forms[2].Tense != "future" {
		t.Fatalf("order = %#v", forms)
	}
	if forms[0].Person != "1" {
		t.Fatalf("person = %s", forms[0].Person)
	}

	err = s.UpsertVerbForms(ctx, []VerbFormInput{
		{LemmaID: lemma, Tense: "infinitive", FormHe: hear, Translit: "lishmoa"},
		{LemmaID: 99999, Tense: "infinitive", FormHe: hear},
	})
	var missing MissingLemmaError
	if !errors.As(err, &missing) || missing.ID != 99999 {
		t.Fatalf("err = %v", err)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM verb_forms`); n != 3 {
		t.Fatalf("a missing lemma wrote rows: %d", n)
	}
}
