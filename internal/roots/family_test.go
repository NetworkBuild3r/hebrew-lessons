package roots

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestFamilyVisibilityAndScope(t *testing.T) {
	ctx := context.Background()
	s := openRoots(t)
	when := db.FormatTime(time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC))
	must(t, s, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('a@example.com', 'sub-a', 'learner', ?)`, when)
	must(t, s, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('b@example.com', 'sub-b', 'learner', ?)`, when)
	var a, b int64
	if err := connOf(t, s).QueryRowContext(ctx, `SELECT id FROM users WHERE email = 'a@example.com'`).Scan(&a); err != nil {
		t.Fatal(err)
	}
	if err := connOf(t, s).QueryRowContext(ctx, `SELECT id FROM users WHERE email = 'b@example.com'`).Scan(&b); err != nil {
		t.Fatal(err)
	}
	must(t, s, `INSERT INTO lexemes (text_nfc, gloss_en, pos, root, binyan) VALUES ('שָׁמַע', 'hear', 'verb', 'שׁ־מ־ע', 'paal')`)
	must(t, s, `INSERT INTO lexemes (text_nfc, gloss_en, pos, root) VALUES ('שְׁמוּעָה', 'news', 'noun', 'שׁ־מ־ע')`)
	must(t, s, `INSERT INTO lexemes (text_nfc, gloss_en, pos, root) VALUES ('לֵב', 'heart', 'noun', 'ל־ב')`)
	var hear, news, heart int64
	scanID(t, s, &hear, `SELECT id FROM lexemes WHERE text_nfc = 'שָׁמַע'`)
	scanID(t, s, &news, `SELECT id FROM lexemes WHERE text_nfc = 'שְׁמוּעָה'`)
	scanID(t, s, &heart, `SELECT id FROM lexemes WHERE text_nfc = 'לֵב'`)

	doc := `{"title":"T","situation":"s","concept":"c","lessons":[{"kind":"lesson","activities":[{"type":"story","text":"שָׁמַע"}]}]}`
	must(t, s, `INSERT INTO units (position, title, situation, concept, status, rev, doc, scope) VALUES (1, 'Shared', 's', 'c', 'published', 1, ?, 'shared')`, doc)
	must(t, s, `INSERT INTO units (position, title, situation, concept, status, rev, doc, scope) VALUES (2, 'Mine', 's', 'c', 'published', 1, ?, ?)`, doc, db.LearnerScope(a))
	must(t, s, `INSERT INTO units (position, title, situation, concept, status, rev, doc, scope) VALUES (3, 'Theirs', 's', 'c', 'published', 1, ?, ?)`, doc, db.LearnerScope(b))
	var shared, mine, theirs int64
	scanID(t, s, &shared, `SELECT id FROM units WHERE title = 'Shared'`)
	scanID(t, s, &mine, `SELECT id FROM units WHERE title = 'Mine'`)
	scanID(t, s, &theirs, `SELECT id FROM units WHERE title = 'Theirs'`)
	must(t, s, `INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill) VALUES (?, 0, ?, 'new', 'story')`, shared, hear)
	must(t, s, `INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill) VALUES (?, 0, ?, 'new', 'story')`, mine, news)
	must(t, s, `INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill) VALUES (?, 0, ?, 'new', 'story')`, theirs, hear)

	// One word and no verse stays off the index. Two words are on it.
	list, err := List(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Root != "שׁ־מ־ע" || list[0].Words != 2 {
		t.Fatalf("list %#v", list)
	}
	ok, err := Visible(ctx, s, "ל־ב")
	if err != nil || ok {
		t.Fatalf("lonely root visible=%v %v", ok, err)
	}
	ok, err = Visible(ctx, s, "שׁ־מ־ע")
	if err != nil || !ok {
		t.Fatalf("family visible=%v %v", ok, err)
	}
	if ok, _ := Visible(ctx, s, "not-a-root"); ok {
		t.Fatal("bad path was visible")
	}

	fam, err := Family(ctx, s, learner.New(s, s.Clock()), "שׁ־מ־ע", a)
	if err != nil {
		t.Fatal(err)
	}
	if len(fam.Forms) != 0 {
		t.Fatalf("forms %#v", fam.Forms)
	}
	if len(fam.Words) != 2 || fam.Words[0].State != "unseen" {
		t.Fatalf("words %#v", fam.Words)
	}
	if fam.CoreGloss != "hear" && fam.CoreGloss != "news" {
		t.Fatalf("gloss %q", fam.CoreGloss)
	}
	titles := map[string]bool{}
	for _, les := range fam.Lessons {
		titles[les.Title] = true
	}
	if !titles["Shared"] || !titles["Mine"] || titles["Theirs"] {
		t.Fatalf("lessons %#v", fam.Lessons)
	}
}

func TestVersesIsaiahFirstAndCapped(t *testing.T) {
	ctx := context.Background()
	s := openRoots(t)
	must(t, s, `INSERT INTO lexemes (text_nfc, gloss_en, root) VALUES ('שָׁמַע', 'hear', 'שׁ־מ־ע')`)
	must(t, s, `INSERT INTO lexemes (text_nfc, gloss_en, root) VALUES ('שֹׁמֵעַ', 'hearing', 'שׁ־מ־ע')`)
	seedVerse(t, s, "genesis", 1, 1, "שָׁמַע", "In the beginning")
	seedVerse(t, s, "isaiah", 1, 1, "שָׁמַע", "Hear")
	for i := 2; i <= 12; i++ {
		seedVerse(t, s, "isaiah", 1, i, "שָׁמַע", "again")
	}
	start := time.Now()
	verses, err := s.VersesForWords(ctx, []string{units.Plain("שָׁמַע")}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("verse lookup took %s", time.Since(start))
	}
	if len(verses) != 10 || verses[0].Book != "isaiah" {
		t.Fatalf("got %d first %s", len(verses), verses[0].Book)
	}
	for _, v := range verses {
		if v.Book == "genesis" {
			t.Fatal("genesis outranked Isaiah inside the cap")
		}
	}
}

func openRoots(t *testing.T) *db.Store {
	t.Helper()
	s, err := db.Open(context.Background(), t.TempDir()+"/hebrew.sqlite", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func connOf(t *testing.T, s *db.Store) *sql.DB {
	t.Helper()
	c, err := s.DB()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func must(t *testing.T, s *db.Store, q string, args ...any) {
	t.Helper()
	if _, err := connOf(t, s).ExecContext(context.Background(), q, args...); err != nil {
		t.Fatal(err)
	}
}

func scanID(t *testing.T, s *db.Store, dest *int64, q string) {
	t.Helper()
	if err := connOf(t, s).QueryRowContext(context.Background(), q).Scan(dest); err != nil {
		t.Fatal(err)
	}
}

func seedVerse(t *testing.T, s *db.Store, book string, chapter, verse int, he, en string) {
	t.Helper()
	must(t, s, `
		INSERT INTO tanakh_verses (book, chapter, verse, he, he_plain, en, source_version, license)
		VALUES (?, ?, ?, ?, ?, ?, 'test', 'Public Domain')`,
		book, chapter, verse, he, units.Plain(he), en)
}
