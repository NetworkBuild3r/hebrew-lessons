package db

import (
	"context"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestCanonicalBookAliases(t *testing.T) {
	books := TanakhBooks()
	if len(books) != 39 {
		t.Fatalf("books = %d", len(books))
	}
	sum := 0
	for _, b := range books {
		sum += b.Verses
		for _, name := range []string{b.Slug, b.Title, strings.ToUpper(b.Title)} {
			got, ok := CanonicalBook(name)
			if !ok || got != b.Slug {
				t.Fatalf("CanonicalBook(%q) = %q, %v; want %s", name, got, ok, b.Slug)
			}
		}
	}
	if sum != 23206 {
		t.Fatalf("verses = %d", sum)
	}
	for _, tc := range []struct{ name, slug string }{
		{"Gen", "genesis"},
		{"gen.", "genesis"},
		{"Bereshit", "genesis"},
		{"בראשית", "genesis"},
		{"1 Samuel", "i-samuel"},
		{"I Samuel", "i-samuel"},
		{"i-samuel", "i-samuel"},
		{"2 Kgs", "ii-kings"},
		{"Ps", "psalms"},
		{"Psalm", "psalms"},
		{"Song of Solomon", "song-of-songs"},
		{"קהלת", "ecclesiastes"},
	} {
		got, ok := CanonicalBook(tc.name)
		if !ok || got != tc.slug {
			t.Fatalf("CanonicalBook(%q) = %q, %v; want %s", tc.name, got, ok, tc.slug)
		}
	}
	if _, ok := CanonicalBook("not a book"); ok {
		t.Fatal("unknown book resolved")
	}
}

func TestVersesByRefAndContaining(t *testing.T) {
	s := openStore(t, nil)
	ctx := context.Background()
	pointed := "בְּרֵאשִׁ֖ית בָּרָא אֱלֹהִים"
	plain := units.Plain(pointed)
	if strings.ContainsRune(plain, '\u0596') || strings.ContainsRune(plain, '\u05B8') {
		t.Fatalf("plain %q still has cantillation or qamats", plain)
	}
	if !strings.ContainsRune(plain, '\u05C1') {
		t.Fatalf("plain %q dropped the shin dot", plain)
	}
	if plain != "בראשׁית ברא אלהים" {
		t.Fatalf("plain = %q", plain)
	}
	rows := []Verse{
		{Book: "genesis", Chapter: 1, Verse: 1, He: pointed, HePlain: plain, En: "In the beginning God created the heaven and the earth.", SourceVersion: "v", License: "Public Domain"},
		{Book: "genesis", Chapter: 1, Verse: 2, He: "וְהָאָרֶץ", HePlain: "והארץ", En: "Now the earth.", SourceVersion: "v", License: "Public Domain"},
		{Book: "genesis", Chapter: 1, Verse: 3, He: "עַל־פְּנֵי הַמָּיִם", HePlain: "על־פני המים", En: "over the face of the waters.", SourceVersion: "v", License: "Public Domain"},
		{Book: "psalms", Chapter: 23, Verse: 1, He: "מִזְמוֹר בָּרָא", HePlain: "מזמור ברא", En: "A Psalm.", SourceVersion: "v", License: "Public Domain"},
		{Book: "i-samuel", Chapter: 1, Verse: 1, He: "אִישׁ", HePlain: "איש", En: "a man.", SourceVersion: "v", License: "Public Domain"},
	}
	if err := s.ReplaceTanakh(ctx, rows, map[string]string{"content_hash": "abc"}); err != nil {
		t.Fatal(err)
	}

	got, err := s.VersesByRef(ctx, "Gen", 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Verse != 1 || got[1].Verse != 2 || got[0].HePlain != plain {
		t.Fatalf("range = %+v", got)
	}
	one, err := s.VersesByRef(ctx, "בראשית", 1, 2, 2)
	if err != nil || len(one) != 1 || one[0].Verse != 2 {
		t.Fatalf("alias range = %+v, %v", one, err)
	}
	none, err := s.VersesByRef(ctx, "Genesis", 9, 1, 3)
	if err != nil || len(none) != 0 {
		t.Fatalf("empty range = %+v, %v", none, err)
	}
	sam, err := s.VersesByRef(ctx, "1 Samuel", 1, 1, 1)
	if err != nil || len(sam) != 1 || sam[0].Book != "i-samuel" {
		t.Fatalf("1 Samuel = %+v, %v", sam, err)
	}
	if _, err := s.VersesByRef(ctx, "nowhere", 1, 1, 1); err == nil {
		t.Fatal("unknown book succeeded")
	}
	if _, err := s.VersesByRef(ctx, "genesis", 1, 3, 1); err == nil {
		t.Fatal("reversed range succeeded")
	}
	if _, err := s.VersesByRef(ctx, "genesis", 0, 1, 1); err == nil {
		t.Fatal("chapter 0 succeeded")
	}

	hits, err := s.VersesContaining(ctx, "ברא", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].Book != "genesis" || hits[1].Book != "psalms" {
		t.Fatalf("ברא hits = %+v", hits)
	}
	first, err := s.VersesContaining(ctx, "ברא", 1)
	if err != nil || len(first) != 1 || first[0].Book != "genesis" {
		t.Fatalf("limit = %+v %v", first, err)
	}
	for _, miss := range []string{"רא", "בראש", "אשית", "פנ", "עלפני"} {
		got, err := s.VersesContaining(ctx, miss, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("%q matched %+v", miss, got)
		}
	}
	face, err := s.VersesContaining(ctx, "על", 10)
	if err != nil || len(face) != 1 || face[0].Verse != 3 {
		t.Fatalf("maqaf token על = %+v, %v", face, err)
	}
	waters, err := s.VersesContaining(ctx, "פני", 10)
	if err != nil || len(waters) != 1 {
		t.Fatalf("פני = %+v, %v", waters, err)
	}
	if got, err := s.VersesContaining(ctx, "על־פני", 10); err != nil || len(got) != 0 {
		t.Fatalf("phrase query = %+v, %v", got, err)
	}
	pointedQuery, err := s.VersesContaining(ctx, "בָּרָא", 10)
	if err != nil || len(pointedQuery) != 2 {
		t.Fatalf("pointed query = %+v, %v", pointedQuery, err)
	}
	if _, err := s.VersesContaining(ctx, "ברא", 0); err == nil {
		t.Fatal("limit 0 succeeded")
	}

	// Orphan delete and update.
	kept := rows[:1]
	kept[0].En = "changed"
	if err := s.ReplaceTanakh(ctx, kept, map[string]string{"content_hash": "def"}); err != nil {
		t.Fatal(err)
	}
	n, err := s.CountTanakhVerses(ctx)
	if err != nil || n != 1 {
		t.Fatalf("after replace count = %d, %v", n, err)
	}
	again, err := s.VersesByRef(ctx, "genesis", 1, 1, 1)
	if err != nil || len(again) != 1 || again[0].En != "changed" {
		t.Fatalf("updated = %+v, %v", again, err)
	}
	hash, ok, err := s.TanakhMeta(ctx, "content_hash")
	if err != nil || !ok || hash != "def" {
		t.Fatalf("meta = %q %v %v", hash, ok, err)
	}
}

func TestPlainStripsNiqqudAndCantillation(t *testing.T) {
	in := "בָּרָ\u0596א שִׁיר שָׂר"
	got := units.Plain(in)
	if got != "ברא שׁיר שׂר" {
		t.Fatalf("plain = %q", got)
	}
	if units.Plain("שׁ") == units.Plain("שׂ") {
		t.Fatal("shin and sin dots collapsed")
	}
}
