// Package fixturecheck keeps adult lesson content out of the contract tree.
package fixturecheck

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ForbiddenSlug is the adult worksheet that must not appear in any contract
// fixture, golden, catalog entry, or seeded database row. It is assembled so
// this source file does not itself contain the slug.
func ForbiddenSlug() string {
	return "anal-" + "sex-vocab"
}

// Dir fails when any file under root contains ForbiddenSlug.
func Dir(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(body, []byte(ForbiddenSlug())) {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s contains forbidden lesson slug", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// File fails when path contains ForbiddenSlug. A missing sidecar is ignored.
func File(t *testing.T, path string) {
	t.Helper()
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		body, err := os.ReadFile(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatal(err)
		}
		if bytes.Contains(body, []byte(ForbiddenSlug())) {
			t.Errorf("%s contains forbidden lesson slug", candidate)
		}
	}
}

// Names fails when a directory entry is the forbidden slug.
func Names(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(d.Name(), ForbiddenSlug()) {
			t.Errorf("path %s", path)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
