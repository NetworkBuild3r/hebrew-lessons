package tanakh

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"golang.org/x/text/unicode/norm"
)

type testClock time.Time

func (c testClock) Now() time.Time { return time.Time(c) }

func openStore(t *testing.T, dir string, at time.Time) *db.Store {
	t.Helper()
	s, err := db.Open(context.Background(), filepath.Join(dir, "hebrew.sqlite"), testClock(at))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

type fixRow struct {
	Book    string
	Chapter int
	Verse   int
	He      string
	En      string
}

func writeCorpus(t *testing.T, dir, licenseEn string, gzipped bool, rows []fixRow) {
	t.Helper()
	if licenseEn == "" {
		licenseEn = licensePD
	}
	type jr struct {
		Book    string `json:"book"`
		Chapter int    `json:"chapter"`
		Verse   int    `json:"verse"`
		He      string `json:"he"`
		En      string `json:"en"`
	}
	byBook := map[string][]jr{}
	order := []string{}
	for _, row := range rows {
		if _, ok := byBook[row.Book]; !ok {
			order = append(order, row.Book)
		}
		byBook[row.Book] = append(byBook[row.Book], jr(row))
	}
	var listed []manifestBook
	for _, slug := range order {
		title := slug
		for _, b := range db.TanakhBooks() {
			if b.Slug == slug {
				title = b.Title
			}
		}
		payload := map[string]any{
			"book":  slug,
			"title": title,
			"rows":  byBook[slug],
		}
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		name := slug + ".json"
		if gzipped {
			name += ".gz"
			body, err = gzipBytes(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
		listed = append(listed, manifestBook{Slug: slug, Title: title, Rows: len(byBook[slug]), File: slug + ".json"})
	}
	manifest := manifestFile{
		Source:    sourceAPI,
		FetchedAt: "2026-10-18T23:41:40.840Z",
		VersionEn: versionEn,
		VersionHe: versionHe,
		LicenseEn: licenseEn,
		LicenseHe: licensePD,
		Books:     listed,
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	name := "manifest.json"
	if gzipped {
		name += ".gz"
		body, err = gzipBytes(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func sampleRows() []fixRow {
	return []fixRow{
		{Book: "genesis", Chapter: 1, Verse: 1, He: norm.NFD.String("בְּרֵאשִׁ֖ית בָּרָא"), En: "In the beginning."},
		{Book: "genesis", Chapter: 1, Verse: 2, He: "וְהָאָרֶץ הָיְתָה", En: "the earth."},
	}
}

func fixtureExpect() *Expect {
	return &Expect{Books: 1, Verses: 2}
}

func TestFixtureImportRerunAndChange(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, "", false, sampleRows())
	data := t.TempDir()
	ctx := context.Background()
	firstAt := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, data, firstAt)
	res, err := Import(ctx, s, dir, Options{Expect: fixtureExpect()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionImported || res.Books != 1 || res.Verses != 2 || len(res.Hash) != 64 {
		t.Fatalf("first = %+v", res)
	}
	got, err := s.VersesByRef(ctx, "Genesis", 1, 1, 1)
	if err != nil || len(got) != 1 {
		t.Fatalf("lookup = %+v %v", got, err)
	}
	if !norm.NFC.IsNormalString(got[0].He) {
		t.Fatalf("he is not NFC: %q", got[0].He)
	}
	if strings.ContainsRune(got[0].He, '\u0596') == false {
		t.Fatalf("stored he dropped cantillation: %q", got[0].He)
	}
	if got[0].HePlain != units.Plain(got[0].He) || strings.ContainsRune(got[0].HePlain, '\u0596') || strings.ContainsRune(got[0].HePlain, '\u05B0') {
		t.Fatalf("he_plain = %q", got[0].HePlain)
	}
	if got[0].License != licensePD || !strings.Contains(got[0].SourceVersion, versionHe) {
		t.Fatalf("provenance = %+v", got[0])
	}
	importedAt, ok, err := s.TanakhMeta(ctx, metaImportedAt)
	if err != nil || !ok || importedAt != db.FormatTime(firstAt) {
		t.Fatalf("imported_at = %q %v %v", importedAt, ok, err)
	}
	_ = s.Close()

	later := firstAt.Add(time.Hour)
	s = openStore(t, data, later)
	again, err := Import(ctx, s, dir, Options{Expect: fixtureExpect()})
	if err != nil {
		t.Fatal(err)
	}
	if again.Action != ActionSkipped || again.Hash != res.Hash {
		t.Fatalf("rerun = %+v", again)
	}
	stamp, _, err := s.TanakhMeta(ctx, metaImportedAt)
	if err != nil || stamp != importedAt {
		t.Fatalf("no-op rewrote imported_at to %q", stamp)
	}

	rows := sampleRows()
	rows[1].En = "the earth was."
	rows[1].He = "וְהָאָרֶץ"
	writeCorpus(t, dir, "", false, rows)
	changed, err := Import(ctx, s, dir, Options{Expect: fixtureExpect()})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Action != ActionImported || changed.Hash == res.Hash {
		t.Fatalf("changed = %+v", changed)
	}
	verse, err := s.VersesByRef(ctx, "gen", 1, 2, 2)
	if err != nil || len(verse) != 1 || verse[0].En != "the earth was." || verse[0].HePlain != units.Plain("וְהָאָרֶץ") {
		t.Fatalf("updated verse = %+v %v", verse, err)
	}
	stamp, _, err = s.TanakhMeta(ctx, metaImportedAt)
	if err != nil || stamp != db.FormatTime(later) {
		t.Fatalf("reimport imported_at = %q", stamp)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, "", false, sampleRows())
	s := openStore(t, t.TempDir(), time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC))
	res, err := Import(context.Background(), s, dir, Options{DryRun: true, Expect: fixtureExpect()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionDryRun || res.Verses != 2 {
		t.Fatalf("dry-run = %+v", res)
	}
	n, err := s.CountTanakhVerses(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("verses = %d %v", n, err)
	}
	if _, ok, err := s.TanakhMeta(context.Background(), metaContentHash); err != nil || ok {
		t.Fatalf("dry-run stored a hash: ok=%v %v", ok, err)
	}
}

func TestNonPublicDomainRefused(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, "CC-BY-NC", false, sampleRows())
	s := openStore(t, t.TempDir(), time.Now())
	res, err := Import(context.Background(), s, dir, Options{Expect: fixtureExpect()})
	var refused *RefusalError
	if !errors.As(err, &refused) || refused.ExitCode() != 2 || !refused.Quiet() {
		t.Fatalf("err = %v", err)
	}
	if res.Action != ActionRefused || !strings.Contains(res.Reason, "license") {
		t.Fatalf("result = %+v", res)
	}
	n, err := s.CountTanakhVerses(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("verses = %d %v", n, err)
	}
}

func TestWrongCountRefused(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, "", false, sampleRows())
	s := openStore(t, t.TempDir(), time.Now())
	res, err := Import(context.Background(), s, dir, Options{})
	var refused *RefusalError
	if !errors.As(err, &refused) || refused.ExitCode() != 2 {
		t.Fatalf("err = %v", err)
	}
	if res.Action != ActionRefused || !strings.Contains(res.Reason, "count") || res.Verses != 2 || res.Books != 1 {
		t.Fatalf("result = %+v", res)
	}
	n, nerr := s.CountTanakhVerses(context.Background())
	if nerr != nil || n != 0 {
		t.Fatalf("verses = %d %v", n, nerr)
	}
}

func TestGzipMatchesPlainHash(t *testing.T) {
	plain := t.TempDir()
	zipped := t.TempDir()
	rows := sampleRows()
	writeCorpus(t, plain, "", false, rows)
	writeCorpus(t, zipped, "", true, rows)
	s1 := openStore(t, t.TempDir(), time.Now())
	s2 := openStore(t, t.TempDir(), time.Now())
	a, err := Import(context.Background(), s1, plain, Options{DryRun: true, Expect: fixtureExpect()})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Import(context.Background(), s2, zipped, Options{Expect: fixtureExpect()})
	if err != nil {
		t.Fatal(err)
	}
	if a.Hash != b.Hash || b.Action != ActionImported {
		t.Fatalf("plain %s %s gzip %s %s", a.Hash, a.Action, b.Hash, b.Action)
	}
	both := t.TempDir()
	writeCorpus(t, both, "", false, rows)
	body, err := os.ReadFile(filepath.Join(both, "genesis.json"))
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzipBytes(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(both, "genesis.json.gz"), gz, 0o644); err != nil {
		t.Fatal(err)
	}
	s3 := openStore(t, t.TempDir(), time.Now())
	res, err := Import(context.Background(), s3, both, Options{Expect: fixtureExpect()})
	if err == nil || res.Action != ActionRefused {
		t.Fatalf("both forms = %+v %v", res, err)
	}
}

func TestRealCorpusCounts(t *testing.T) {
	dir := corpusDir(t)
	s := openStore(t, t.TempDir(), time.Now())
	res, err := Import(context.Background(), s, dir, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionDryRun || res.Books != 39 || res.Verses != 23206 {
		t.Fatalf("corpus = %+v", res)
	}
	n, err := s.CountTanakhVerses(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("dry-run wrote %d verses (%v)", n, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "genesis.json"))
	if err != nil {
		t.Fatal(err)
	}
	var book bookFile
	if err := json.Unmarshal(raw, &book); err != nil {
		t.Fatal(err)
	}
	if book.Rows[0].En != "In the beginning God created the heaven and the earth." {
		t.Fatalf("genesis 1:1 en = %q", book.Rows[0].En)
	}
	if units.Plain(book.Rows[0].He) == book.Rows[0].He {
		t.Fatal("genesis 1:1 hebrew has no niqqud to strip")
	}
}

func TestRealCorpusImport(t *testing.T) {
	dir := corpusDir(t)
	data := t.TempDir()
	ctx := context.Background()
	firstAt := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, data, firstAt)
	res, err := Import(ctx, s, dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionImported || res.Books != 39 || res.Verses != 23206 {
		t.Fatalf("import = %+v", res)
	}
	n, err := s.CountTanakhVerses(ctx)
	if err != nil || n != 23206 {
		t.Fatalf("stored verses = %d %v", n, err)
	}
	got, err := s.VersesByRef(ctx, "Gen", 1, 1, 1)
	if err != nil || len(got) != 1 {
		t.Fatalf("genesis 1:1 = %+v %v", got, err)
	}
	if got[0].En != "In the beginning God created the heaven and the earth." {
		t.Fatalf("en = %q", got[0].En)
	}
	if got[0].HePlain != units.Plain(got[0].He) || got[0].License != licensePD {
		t.Fatalf("row = %+v", got[0])
	}
	hits, err := s.VersesContaining(ctx, "ברא", 5)
	if err != nil || len(hits) == 0 || hits[0].Book != "genesis" || hits[0].Chapter != 1 || hits[0].Verse != 1 {
		t.Fatalf("containing ברא = %+v %v", hits, err)
	}
	_ = s.Close()

	s = openStore(t, data, firstAt.Add(time.Hour))
	again, err := Import(ctx, s, dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if again.Action != ActionSkipped || again.Hash != res.Hash {
		t.Fatalf("rerun = %+v", again)
	}
	stamp, ok, err := s.TanakhMeta(ctx, metaImportedAt)
	if err != nil || !ok || stamp != db.FormatTime(firstAt) {
		t.Fatalf("no-op imported_at = %q %v", stamp, err)
	}
}

func corpusDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		cand := filepath.Join(dir, "data", "tanakh", "manifest.json")
		if _, err := os.Stat(cand); err == nil {
			return filepath.Join(dir, "data", "tanakh")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("data/tanakh/manifest.json not found")
		}
		dir = parent
	}
}
