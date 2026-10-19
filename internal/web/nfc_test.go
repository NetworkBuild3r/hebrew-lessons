package web

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// TestHebrewSourcesAreNFC fails when a view, a generated templ file, or a
// template or fixture holds text that is not NFC. Hebrew dagesh and shin
// dots sort after the vowel in canonical order; a typed dagesh-first
// literal does not match the keys the rest of the repo stores.
func TestHebrewSourcesAreNFC(t *testing.T) {
	root := repoRoot(t)
	seen := map[string]bool{}
	var paths []string
	add := func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}

	views := filepath.Join(root, "internal", "web", "views")
	err := filepath.WalkDir(views, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".templ", ".go":
			add(path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	webDir := filepath.Join(root, "internal", "web")
	entries, err := os.ReadDir(webDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ".go" {
			add(filepath.Join(webDir, entry.Name()))
		}
	}

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(rel, string(os.PathSeparator))
		if len(parts) < 2 {
			return nil
		}
		inTree := false
		for _, part := range parts[:len(parts)-1] {
			if part == "templates" || part == "fixtures" || part == "testdata" {
				inTree = true
				break
			}
		}
		if !inTree {
			return nil
		}
		switch filepath.Ext(path) {
		case ".templ", ".go", ".json", ".html", ".txt", ".md":
			add(path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("nfc walk found no files")
	}

	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !utf8.Valid(b) {
			t.Errorf("%s is not valid UTF-8", path)
			continue
		}
		s := string(b)
		if norm.NFC.IsNormalString(s) {
			continue
		}
		fixed := norm.NFC.String(s)
		at := 0
		for at < len(s) && at < len(fixed) && s[at] == fixed[at] {
			at++
		}
		start := at - 12
		if start < 0 {
			start = 0
		}
		end := at + 12
		if end > len(s) {
			end = len(s)
		}
		wantEnd := at + 12
		if wantEnd > len(fixed) {
			wantEnd = len(fixed)
		}
		rel, _ := filepath.Rel(root, path)
		t.Errorf("%s is not NFC near byte %d: got %q want %q", rel, at, s[start:end], fixed[start:wantEnd])
	}
}

func TestRenderedPagesAreNFC(t *testing.T) {
	h := New(nil, nil)
	for _, path := range []string{
		"/", "/?lang=en", "/?due=0", "/?next=0",
		"/?lang=en&due=0", "/?lang=en&next=0",
		"/review", "/progress", "/lesson", "/signin",
		"/review?lang=en", "/progress?lang=en", "/lesson?lang=en", "/signin?lang=en",
	} {
		body := get(t, h, path)
		if !norm.NFC.IsNormalString(body) {
			t.Errorf("%s body is not NFC", path)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}
