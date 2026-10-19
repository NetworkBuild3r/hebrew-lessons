package tanakh

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

const (
	// ActionImported means the verses were written.
	ActionImported = "imported"
	// ActionSkipped means the stored content hash matched and nothing was written.
	ActionSkipped = "skipped"
	// ActionRefused means validation, license, or counts failed. The process exits 2.
	ActionRefused = "refused"
	// ActionDryRun means the files validated and nothing was written.
	ActionDryRun = "dry-run"

	versionHe = "Tanach with Nikkud"
	versionEn = "The Holy Scriptures: A New Translation (JPS 1917)"
	licensePD = "Public Domain"
	sourceAPI = "sefaria-api"

	metaContentHash = "content_hash"
	metaSource      = "source"
	metaVersionHe   = "version_he"
	metaVersionEn   = "version_en"
	metaLicenseHe   = "license_he"
	metaLicenseEn   = "license_en"
	metaFetchedAt   = "fetched_at"
	metaBookCount   = "book_count"
	metaVerseCount  = "verse_count"
	metaImportedAt  = "imported_at"

	maxFileBytes = 8 << 20
	maxCorpus    = 32 << 20
)

// Expect is the book and verse total Import requires.
// PerBook, when set, requires those exact per-book counts and no other books.
type Expect struct {
	Books   int
	Verses  int
	PerBook map[string]int
}

// Options controls one import.
// A nil Expect means the shipped corpus: 39 books and 23,206 verses.
type Options struct {
	DryRun bool
	Expect *Expect
}

// Result is the summary of one import attempt.
type Result struct {
	Books  int
	Verses int
	Hash   string
	Action string
	Reason string
}

// RefusalError is a validation, license, or count failure.
// ExitCode is 2. Quiet tells the process not to print a second line
// after the summary log.
type RefusalError struct {
	Msg string
}

func (e *RefusalError) Error() string { return e.Msg }
func (e *RefusalError) ExitCode() int { return 2 }
func (e *RefusalError) Quiet() bool   { return true }

func refuse(format string, args ...any) error {
	return &RefusalError{Msg: fmt.Sprintf(format, args...)}
}

// ProductionExpect is the pinned public-domain corpus.
func ProductionExpect() Expect {
	books := db.TanakhBooks()
	per := make(map[string]int, len(books))
	sum := 0
	for _, b := range books {
		per[b.Slug] = b.Verses
		sum += b.Verses
	}
	return Expect{Books: len(books), Verses: sum, PerBook: per}
}

func expectOf(opt Options) Expect {
	if opt.Expect != nil {
		return *opt.Expect
	}
	return ProductionExpect()
}

type inputFile struct {
	name string
	body []byte
}

type verseRow struct {
	Book    string `json:"book"`
	Chapter int    `json:"chapter"`
	Verse   int    `json:"verse"`
	He      string `json:"he"`
	En      string `json:"en"`
}

type bookFile struct {
	Book      string     `json:"book"`
	Title     string     `json:"title"`
	License   *string    `json:"license"`
	LicenseEn *string    `json:"licenseEn"`
	LicenseHe *string    `json:"licenseHe"`
	HeLicense *string    `json:"heLicense"`
	Rows      []verseRow `json:"rows"`
}

type manifestBook struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	Rows  int    `json:"rows"`
	File  string `json:"file"`
}

type manifestFile struct {
	Source    string         `json:"source"`
	FetchedAt string         `json:"fetchedAt"`
	VersionEn string         `json:"versionEn"`
	VersionHe string         `json:"versionHe"`
	LicenseEn string         `json:"licenseEn"`
	LicenseHe string         `json:"licenseHe"`
	Books     []manifestBook `json:"books"`
}

type corpus struct {
	books   int
	verses  int
	rows    []db.Verse
	meta    map[string]string
	license error
}

// Import reads dir, validates it, and upserts tanakh_verses.
// dir is a directory of .json files, or .json.gz files with the same names.
// The content hash covers the sorted logical file names and the decompressed
// bytes. When the stored content_hash matches and DryRun is false, Import
// writes nothing. DryRun validates and returns counts without writing.
// store must already be open so migrations have run.
func Import(ctx context.Context, store *db.Store, dir string, opt Options) (Result, error) {
	if store == nil {
		return Result{}, errors.New("tanakh: nil store")
	}
	files, err := readDir(dir)
	if err != nil {
		res := Result{}
		if files != nil {
			res.Hash = hashFiles(files)
		}
		var refused *RefusalError
		if errors.As(err, &refused) {
			res.Action = ActionRefused
			res.Reason = err.Error()
			return res, err
		}
		return Result{}, err
	}
	sum := hashFiles(files)
	parsed, err := parseCorpus(files, expectOf(opt))
	if err != nil {
		res := Result{Hash: sum, Action: ActionRefused, Reason: err.Error()}
		if parsed != nil {
			res.Books = parsed.books
			res.Verses = parsed.verses
		}
		return res, err
	}
	res := Result{Books: parsed.books, Verses: parsed.verses, Hash: sum}
	if opt.DryRun {
		res.Action = ActionDryRun
		return res, nil
	}
	have, ok, err := store.TanakhMeta(ctx, metaContentHash)
	if err != nil {
		return Result{}, err
	}
	if ok && have == sum {
		res.Action = ActionSkipped
		return res, nil
	}
	meta := parsed.meta
	meta[metaContentHash] = sum
	meta[metaImportedAt] = db.FormatTime(store.Clock().Now())
	if err := store.ReplaceTanakh(ctx, parsed.rows, meta); err != nil {
		return Result{}, err
	}
	res.Action = ActionImported
	return res, nil
}

func readDir(dir string) ([]inputFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("tanakh: read %s: %w", dir, err)
	}
	byName := map[string]inputFile{}
	total := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		logical, ok := logicalName(name)
		if !ok {
			continue
		}
		if _, exists := byName[logical]; exists {
			return nil, refuse("tanakh: schema: %s and another file share the name %s", name, logical)
		}
		body, err := readMaybeGzip(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("tanakh: read %s: %w", name, err)
		}
		total += len(body)
		if total > maxCorpus {
			return nil, fmt.Errorf("tanakh: read %s: corpus exceeds %d bytes", dir, maxCorpus)
		}
		byName[logical] = inputFile{name: logical, body: body}
	}
	files := make([]inputFile, 0, len(byName))
	for _, f := range byName {
		files = append(files, f)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	if _, ok := byName["manifest.json"]; !ok {
		return files, refuse("tanakh: schema: manifest.json is missing")
	}
	return files, nil
}

func logicalName(name string) (string, bool) {
	switch {
	case strings.HasSuffix(name, ".json.gz"):
		return strings.TrimSuffix(name, ".gz"), true
	case strings.HasSuffix(name, ".json"):
		return name, true
	default:
		return "", false
	}
}

func readMaybeGzip(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		zr, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		r = zr
	}
	body, err := io.ReadAll(io.LimitReader(r, maxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxFileBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", maxFileBytes)
	}
	return body, nil
}

// hashFiles hashes sorted logical names and decompressed bytes.
// A .json and a .json.gz of the same bytes share a hash.
func hashFiles(files []inputFile) string {
	h := sha256.New()
	for _, f := range files {
		h.Write([]byte(f.name))
		h.Write([]byte{0})
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(f.body)))
		h.Write(n[:])
		h.Write(f.body)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func parseCorpus(files []inputFile, expect Expect) (*corpus, error) {
	var manifestBody []byte
	books := map[string]inputFile{}
	for _, f := range files {
		if f.name == "manifest.json" {
			manifestBody = f.body
			continue
		}
		books[f.name] = f
	}
	var manifest manifestFile
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		return nil, refuse("tanakh: schema: manifest.json: %v", err)
	}
	if err := checkManifestIdentity(manifest); err != nil {
		return nil, err
	}
	if len(manifest.Books) == 0 {
		return nil, refuse("tanakh: schema: manifest lists no books")
	}
	seenFile := map[string]bool{}
	out := &corpus{meta: map[string]string{
		metaSource:    manifest.Source,
		metaVersionHe: manifest.VersionHe,
		metaVersionEn: manifest.VersionEn,
		metaLicenseHe: manifest.LicenseHe,
		metaLicenseEn: manifest.LicenseEn,
		metaFetchedAt: manifest.FetchedAt,
	}}
	sourceVersion := manifest.VersionHe + " / " + manifest.VersionEn
	refs := map[string]bool{}
	for _, listed := range manifest.Books {
		if listed.File == "" || listed.Slug == "" || listed.Title == "" {
			return nil, refuse("tanakh: schema: manifest book is missing slug, title, or file")
		}
		if filepath.Base(listed.File) != listed.File || listed.File == "manifest.json" {
			return nil, refuse("tanakh: schema: manifest file %q is not a book file name", listed.File)
		}
		if seenFile[listed.File] {
			return nil, refuse("tanakh: schema: manifest lists %s twice", listed.File)
		}
		seenFile[listed.File] = true
		f, ok := books[listed.File]
		if !ok {
			return nil, refuse("tanakh: schema: manifest lists %s but the file is missing", listed.File)
		}
		var book bookFile
		if err := json.Unmarshal(f.body, &book); err != nil {
			return nil, refuse("tanakh: schema: %s: %v", listed.File, err)
		}
		if book.Book != listed.Slug || book.Title != listed.Title {
			return nil, refuse("tanakh: schema: %s book %q %q does not match the manifest", listed.File, book.Book, book.Title)
		}
		if _, known := db.CanonicalBook(book.Book); !known || book.Book != slugOf(book.Book) {
			return nil, refuse("tanakh: schema: %s book %q is not a canonical slug", listed.File, book.Book)
		}
		if err := checkLicensePtr(listed.File, "license", book.License); err != nil {
			out.license = err
		}
		if err := checkLicensePtr(listed.File, "licenseEn", book.LicenseEn); err != nil && out.license == nil {
			out.license = err
		}
		if err := checkLicensePtr(listed.File, "licenseHe", book.LicenseHe); err != nil && out.license == nil {
			out.license = err
		}
		if err := checkLicensePtr(listed.File, "heLicense", book.HeLicense); err != nil && out.license == nil {
			out.license = err
		}
		if book.Rows == nil {
			return nil, refuse("tanakh: schema: %s has no rows", listed.File)
		}
		if len(book.Rows) != listed.Rows {
			return nil, refuse("tanakh: schema: %s has %d rows, manifest says %d", listed.File, len(book.Rows), listed.Rows)
		}
		for i, row := range book.Rows {
			if err := validateRow(listed.File, book.Book, i, row); err != nil {
				return nil, err
			}
			key := row.Book + "\x00" + strconv.Itoa(row.Chapter) + "\x00" + strconv.Itoa(row.Verse)
			if refs[key] {
				return nil, refuse("tanakh: schema: duplicate %s %d:%d", row.Book, row.Chapter, row.Verse)
			}
			refs[key] = true
			he := units.Normalize(row.He)
			en := units.Normalize(row.En)
			plain := units.Plain(row.He)
			if he == "" || plain == "" || en == "" {
				return nil, refuse("tanakh: schema: %s %d:%d is empty after normalization", row.Book, row.Chapter, row.Verse)
			}
			out.rows = append(out.rows, db.Verse{
				Book:          row.Book,
				Chapter:       row.Chapter,
				Verse:         row.Verse,
				He:            he,
				HePlain:       plain,
				En:            en,
				SourceVersion: sourceVersion,
				License:       licensePD,
			})
		}
		out.books++
	}
	for name := range books {
		if !seenFile[name] {
			return nil, refuse("tanakh: schema: %s is not listed in the manifest", name)
		}
	}
	out.verses = len(out.rows)
	out.meta[metaBookCount] = strconv.Itoa(out.books)
	out.meta[metaVerseCount] = strconv.Itoa(out.verses)
	if out.license != nil {
		return out, out.license
	}
	if manifest.LicenseEn != licensePD || manifest.LicenseHe != licensePD {
		return out, refuse("tanakh: license: manifest licenseEn=%q licenseHe=%q, want %q", manifest.LicenseEn, manifest.LicenseHe, licensePD)
	}
	if err := checkCounts(out, expect); err != nil {
		return out, err
	}
	return out, nil
}

func slugOf(name string) string {
	slug, ok := db.CanonicalBook(name)
	if !ok {
		return ""
	}
	return slug
}

func checkManifestIdentity(m manifestFile) error {
	if strings.TrimSpace(m.Source) != sourceAPI {
		return refuse("tanakh: schema: manifest source %q, want %q", m.Source, sourceAPI)
	}
	if strings.TrimSpace(m.FetchedAt) == "" {
		return refuse("tanakh: schema: manifest fetchedAt is empty")
	}
	if m.VersionHe != versionHe || m.VersionEn != versionEn {
		return refuse("tanakh: schema: manifest versions he=%q en=%q, want the pinned public-domain editions", m.VersionHe, m.VersionEn)
	}
	return nil
}

func checkLicensePtr(file, field string, value *string) error {
	if value == nil {
		return nil
	}
	if *value != licensePD {
		return refuse("tanakh: license: %s %s is %q, want %q", file, field, *value, licensePD)
	}
	return nil
}

func validateRow(file, book string, i int, row verseRow) error {
	if row.Book != book {
		return refuse("tanakh: schema: %s row %d book %q, want %q", file, i, row.Book, book)
	}
	if row.Chapter < 1 || row.Verse < 1 {
		return refuse("tanakh: schema: %s row %d reference %d:%d is invalid", file, i, row.Chapter, row.Verse)
	}
	if strings.TrimSpace(row.He) == "" || strings.TrimSpace(row.En) == "" {
		return refuse("tanakh: schema: %s %d:%d is missing hebrew or english", row.Book, row.Chapter, row.Verse)
	}
	return nil
}

func checkCounts(c *corpus, expect Expect) error {
	if expect.PerBook != nil {
		got := map[string]int{}
		for _, row := range c.rows {
			got[row.Book]++
		}
		if len(got) != len(expect.PerBook) {
			return refuse("tanakh: count: books=%d, want %d", len(got), len(expect.PerBook))
		}
		for slug, want := range expect.PerBook {
			if got[slug] != want {
				return refuse("tanakh: count: %s verses=%d, want %d", slug, got[slug], want)
			}
		}
	}
	if expect.Books > 0 && c.books != expect.Books {
		return refuse("tanakh: count: books=%d verses=%d, want books=%d verses=%d", c.books, c.verses, expect.Books, expect.Verses)
	}
	if expect.Verses > 0 && c.verses != expect.Verses {
		return refuse("tanakh: count: books=%d verses=%d, want books=%d verses=%d", c.books, c.verses, expect.Books, expect.Verses)
	}
	return nil
}

// gzipBytes compresses body. Tests use it to show a .json.gz imports
// as the same corpus as the uncompressed JSON.
func gzipBytes(body []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(body); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
