package lessondata

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type caseMeta struct {
	Label        string `json:"label"`
	OK           bool   `json:"ok"`
	Kind         string `json:"kind"`
	AssignParses bool   `json:"assignParses"`
	Repo         string `json:"repo"`
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

func collectCases(t *testing.T) []string {
	t.Helper()
	var dirs []string
	err := filepath.WalkDir(goldenRoot(t), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "case.json" {
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) == 0 {
		t.Fatal("no goldens; run node scripts/generate-lessondata-goldens.js")
	}
	sort.Strings(dirs)
	return dirs
}

func TestGoldens(t *testing.T) {
	root := repoRoot(t)
	base := goldenRoot(t)
	for _, dir := range collectCases(t) {
		dir := dir
		name, err := filepath.Rel(base, dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			metaBytes, err := os.ReadFile(filepath.Join(dir, "case.json"))
			if err != nil {
				t.Fatal(err)
			}
			var meta caseMeta
			if err := json.Unmarshal(metaBytes, &meta); err != nil {
				t.Fatal(err)
			}
			input, err := os.ReadFile(filepath.Join(dir, "input.js"))
			if err != nil {
				t.Fatal(err)
			}
			if meta.Repo != "" {
				repoBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(meta.Repo)))
				if err != nil {
					t.Fatal(err)
				}
				if string(repoBytes) != string(input) {
					t.Fatalf("golden input drifted from %s\n%s", meta.Repo, diffAt(string(input), string(repoBytes)))
				}
			}
			doc, err := Parse(string(input), meta.Label)
			if !meta.OK {
				if err == nil {
					t.Fatal("expected SyntaxError, parse succeeded")
				}
				var se *SyntaxError
				if !errors.As(err, &se) {
					t.Fatalf("error type %T, want *SyntaxError: %v", err, err)
				}
				want, readErr := os.ReadFile(filepath.Join(dir, "error.json"))
				if readErr != nil {
					t.Fatal(readErr)
				}
				got := quoteJSON(err.Error()) + "\n"
				if got != string(want) {
					t.Fatalf("error text\n%s", diffAt(got, string(want)))
				}
				return
			}
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			compareFile(t, filepath.Join(dir, "written.js"), doc.Write())
			compareFile(t, filepath.Join(dir, "emit.js"), doc.Emit())
			compareFile(t, filepath.Join(dir, "assign.js"), doc.Assign())

			again, err := Parse(doc.Write(), meta.Label)
			if err != nil {
				t.Fatalf("rewrite did not parse: %v", err)
			}
			if again.Write() != doc.Write() {
				t.Fatalf("indented writer is not idempotent\n%s", diffAt(again.Write(), doc.Write()))
			}
			fromEmit, err := Parse(doc.Emit(), meta.Label)
			if err != nil {
				t.Fatalf("emit script did not parse: %v", err)
			}
			if fromEmit.Write() != doc.Write() {
				t.Fatalf("emit script parsed to a different value\n%s", diffAt(fromEmit.Write(), doc.Write()))
			}
			if meta.AssignParses {
				fromAssign, err := Parse(doc.Assign(), meta.Label)
				if err != nil {
					t.Fatalf("assign script did not parse: %v", err)
				}
				if fromAssign.Write() != doc.Write() {
					t.Fatalf("assign script parsed to a different value\n%s", diffAt(fromAssign.Write(), doc.Write()))
				}
			}
		})
	}
}

func TestEveryWorksheetHasGolden(t *testing.T) {
	root := repoRoot(t)
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
	if len(files) < 2 {
		t.Fatalf("expected worksheet files, found %d", len(files))
	}
	for _, rel := range files {
		caseFile := filepath.Join(goldenRoot(t), "files", filepath.FromSlash(rel), "case.json")
		if _, err := os.Stat(caseFile); err != nil {
			t.Errorf("missing golden for %s (%s)", rel, caseFile)
		}
	}
}

func TestMutationMessagesCoverGrammar(t *testing.T) {
	var b strings.Builder
	err := filepath.WalkDir(filepath.Join(goldenRoot(t), "mutations"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == "error.json" {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			b.Write(body)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := b.String()
	if text == "" {
		t.Fatal("no mutation errors")
	}
	// Fragments are matched against JSON.stringify(message), so a backslash
	// or quote in the message is escaped in the file.
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

func compareFile(t *testing.T, path, got string) {
	t.Helper()
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Fatalf("%s\n%s", filepath.Base(path), diffAt(got, string(want)))
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
