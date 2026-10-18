package lessondata

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Corpus floors. Shrinking a category, or dropping a worksheet, fails the
// golden run even if meta.json is rewritten to match.
const (
	minWorksheetFiles = 8
	minMutations      = 81
	minQuirks         = 64
)

type goldenMeta struct {
	Node             string `json:"node"`
	LessonDataSha256 string `json:"lessonDataSha256"`
	Generator        string `json:"generator"`
	Files            int    `json:"files"`
	Mutations        int    `json:"mutations"`
	Quirks           int    `json:"quirks"`
}

type mutationRow struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Input     string `json:"input"`
	ErrorJSON string `json:"errorJSON"`
}

type quirkRow struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Input        string `json:"input"`
	AssignParses bool   `json:"assignParses"`
	Written      string `json:"written"`
	Emit         string `json:"emit"`
	Assign       string `json:"assign"`
}

type worksheetRow struct {
	Repo         string `json:"repo"`
	Label        string `json:"label"`
	AssignParses bool   `json:"assignParses"`
	Written      string `json:"written"`
	Emit         string `json:"emit"`
	Assign       string `json:"assign"`
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func goldenRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata", "golden")
}

func readJSONL(t *testing.T, name string) [][]byte {
	t.Helper()
	f, err := os.Open(filepath.Join(goldenRoot(t), name))
	if err != nil {
		t.Fatalf("%s: %v (run node scripts/generate-lessondata-goldens.js)", name, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	// Worksheet lines carry emit and assign text for a whole lesson.
	sc.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)
	var rows [][]byte
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		rows = append(rows, append([]byte(nil), line...))
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if len(rows) == 0 {
		t.Fatalf("%s is empty", name)
	}
	return rows
}

func TestGoldens(t *testing.T) {
	root := repoRoot(t)
	base := goldenRoot(t)
	meta := readMeta(t)

	mutations := readJSONL(t, "mutations.jsonl")
	quirks := readJSONL(t, "quirks.jsonl")
	worksheets := readJSONL(t, "worksheets.jsonl")
	if len(mutations) < minMutations || meta.Mutations != len(mutations) {
		t.Fatalf("mutations: %d rows, meta %d, floor %d", len(mutations), meta.Mutations, minMutations)
	}
	if len(quirks) < minQuirks || meta.Quirks != len(quirks) {
		t.Fatalf("quirks: %d rows, meta %d, floor %d", len(quirks), meta.Quirks, minQuirks)
	}
	if len(worksheets) < minWorksheetFiles || meta.Files != len(worksheets) {
		t.Fatalf("worksheets: %d rows, meta %d, floor %d", len(worksheets), meta.Files, minWorksheetFiles)
	}

	for _, line := range mutations {
		var row mutationRow
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatalf("mutations.jsonl: %v", err)
		}
		t.Run("mutations/"+row.ID, func(t *testing.T) {
			t.Parallel()
			doc, err := Parse(row.Input, row.Label)
			if err == nil {
				t.Fatalf("expected SyntaxError, parse succeeded (%s)", doc.Write())
			}
			var se *SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("error type %T, want *SyntaxError: %v", err, err)
			}
			// errorJSON is JSON.stringify(message): ASCII, quotes included.
			// Comparing that to quoteJSON keeps a lone surrogate in the
			// message (unexpected emoji) instead of U+FFFD from Unmarshal.
			got := quoteJSON(err.Error())
			if got != row.ErrorJSON {
				t.Fatalf("error text\n%s", diffAt(got, row.ErrorJSON))
			}
		})
	}

	for _, line := range quirks {
		var row quirkRow
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatalf("quirks.jsonl: %v", err)
		}
		t.Run("quirks/"+row.ID, func(t *testing.T) {
			t.Parallel()
			checkRoundTrip(t, row.Label, row.Input, row.Written, row.Emit, row.Assign, row.AssignParses)
		})
	}

	for _, line := range worksheets {
		var row worksheetRow
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatalf("worksheets.jsonl: %v", err)
		}
		t.Run(row.Repo, func(t *testing.T) {
			t.Parallel()
			input, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(row.Repo)))
			if err != nil {
				t.Fatal(err)
			}
			writtenPath, err := writtenFile(base, row.Written)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(writtenPath)
			if err != nil {
				t.Fatal(err)
			}
			checkRoundTrip(t, row.Label, string(input), string(want), row.Emit, row.Assign, row.AssignParses)
		})
	}
}

func TestEveryWorksheetHasGolden(t *testing.T) {
	root := repoRoot(t)
	base := goldenRoot(t)
	var files []string
	err := filepath.WalkDir(filepath.Join(root, "worksheets"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".js") {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < minWorksheetFiles {
		t.Fatalf("expected worksheet files, found %d", len(files))
	}
	got := map[string]string{}
	for _, line := range readJSONL(t, "worksheets.jsonl") {
		var row worksheetRow
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatal(err)
		}
		got[row.Repo] = row.Written
	}
	for _, rel := range files {
		written, ok := got[rel]
		if !ok {
			t.Errorf("missing golden row for %s", rel)
			continue
		}
		want := "written/" + strings.TrimSuffix(rel, ".js") + ".written.js"
		if written != want {
			t.Errorf("%s written path %s, want %s", rel, written, want)
		}
		path, err := writtenFile(base, written)
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing written bytes for %s (%s)", rel, path)
		}
	}
}

func TestMutationMessagesCoverGrammar(t *testing.T) {
	var b strings.Builder
	for _, line := range readJSONL(t, "mutations.jsonl") {
		var row mutationRow
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatal(err)
		}
		// Same text as JSON.stringify(message): a backslash in the message
		// is two backslash characters here.
		b.WriteString(row.ErrorJSON)
		b.WriteByte('\n')
	}
	text := b.String()
	for _, frag := range []string{
		"unterminated comment",
		"newline inside string literal",
		"template interpolation is not allowed",
		"unterminated string literal",
		`malformed \\u escape`,
		`malformed \\x escape`,
		"line continuation is not allowed",
		"unexpected character",
		"unexpected end of file",
		"non-finite number",
		"is not a literal value",
		"unexpected token",
		"object key must be an identifier or string",
		`expected \"`,
		"merge source must be an object literal",
		"only window.NAME = <literal> statements are allowed",
		"expected a global name after window.",
	} {
		if !strings.Contains(text, frag) {
			t.Errorf("mutation corpus missing %q", frag)
		}
	}
}

func readMeta(t *testing.T) goldenMeta {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(goldenRoot(t), "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta goldenMeta
	if err := json.Unmarshal(body, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Node == "" || meta.LessonDataSha256 == "" || meta.Generator == "" {
		t.Fatalf("meta.json missing provenance: %+v", meta)
	}
	return meta
}

func writtenFile(base, rel string) (string, error) {
	if rel == "" || strings.HasPrefix(rel, "/") || strings.Contains(rel, "\\") || strings.Contains(rel, "..") {
		return "", errors.New("written path " + strconv.Quote(rel))
	}
	return filepath.Join(base, filepath.FromSlash(rel)), nil
}

func checkRoundTrip(t *testing.T, label, input, written, emit, assign string, assignParses bool) {
	t.Helper()
	doc, err := Parse(input, label)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Write() != written {
		t.Fatalf("written\n%s", diffAt(doc.Write(), written))
	}
	if doc.Emit() != emit {
		t.Fatalf("emit\n%s", diffAt(doc.Emit(), emit))
	}
	if doc.Assign() != assign {
		t.Fatalf("assign\n%s", diffAt(doc.Assign(), assign))
	}
	again, err := Parse(doc.Write(), label)
	if err != nil {
		t.Fatalf("rewrite did not parse: %v", err)
	}
	if again.Write() != doc.Write() {
		t.Fatalf("indented writer is not idempotent\n%s", diffAt(again.Write(), doc.Write()))
	}
	fromEmit, err := Parse(doc.Emit(), label)
	if err != nil {
		t.Fatalf("emit script did not parse: %v", err)
	}
	if fromEmit.Write() != doc.Write() {
		t.Fatalf("emit script parsed to a different value\n%s", diffAt(fromEmit.Write(), doc.Write()))
	}
	if assignParses {
		fromAssign, err := Parse(doc.Assign(), label)
		if err != nil {
			t.Fatalf("assign script did not parse: %v", err)
		}
		if fromAssign.Write() != doc.Write() {
			t.Fatalf("assign script parsed to a different value\n%s", diffAt(fromAssign.Write(), doc.Write()))
		}
	}
}

func diffAt(got, want string) string {
	n := len(got)
	if len(want) < n {
		n = len(want)
	}
	for i := 0; i < n; i++ {
		if got[i] != want[i] {
			return "first difference at byte " + strconv.Itoa(i) +
				": got " + strconv.Quote(window(got, i)) +
				" want " + strconv.Quote(window(want, i)) +
				" (lens " + strconv.Itoa(len(got)) + " vs " + strconv.Itoa(len(want)) + ")"
		}
	}
	return "common prefix, lens " + strconv.Itoa(len(got)) + " vs " + strconv.Itoa(len(want))
}

func window(s string, i int) string {
	start := i - 20
	if start < 0 {
		start = 0
	}
	end := i + 40
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}
