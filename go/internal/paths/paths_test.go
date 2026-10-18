package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDefaultsUnderHebrewRoot(t *testing.T) {
	root := t.TempDir()
	layout, err := Resolve(root, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if layout.Root != root {
		t.Fatalf("root: got %s", layout.Root)
	}
	if layout.DataDir != filepath.Join(root, "data") {
		t.Fatalf("data: got %s", layout.DataDir)
	}
	if layout.LessonsDir != filepath.Join(root, "worksheets") {
		t.Fatalf("lessons: got %s", layout.LessonsDir)
	}
	if layout.DBPath != filepath.Join(root, "data", "hebrew.sqlite") {
		t.Fatalf("db: got %s", layout.DBPath)
	}
	if layout.MediaDir != filepath.Join(root, "data", "media") {
		t.Fatalf("media: got %s", layout.MediaDir)
	}
}

func TestResolveExplicitDirs(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "var", "app")
	lessons := filepath.Join(root, "var", "worksheets")
	layout, err := Resolve(root, data, lessons)
	if err != nil {
		t.Fatal(err)
	}
	if layout.DataDir != data || layout.LessonsDir != lessons {
		t.Fatalf("layout: %+v", layout)
	}
	if layout.DBPath != filepath.Join(data, "hebrew.sqlite") {
		t.Fatalf("db: %s", layout.DBPath)
	}
	if layout.MediaDir != filepath.Join(data, "media") {
		t.Fatalf("media: %s", layout.MediaDir)
	}
}

func TestRelativeDataDirUsesWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)
	layout, err := Resolve(root, filepath.Join("..", "app-data"), "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "app-data")
	if layout.DataDir != want {
		t.Fatalf("data: got %s want %s", layout.DataDir, want)
	}
}

func TestUnsetHebrewRootUsesWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	layout, err := Resolve("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if layout.Root != root {
		t.Fatalf("root: got %s want %s", layout.Root, root)
	}
	if layout.DataDir != filepath.Join(root, "data") {
		t.Fatalf("data: %s", layout.DataDir)
	}
	if layout.LessonsDir != filepath.Join(root, "worksheets") {
		t.Fatalf("lessons: %s", layout.LessonsDir)
	}
	if layout.DBPath != filepath.Join(root, "data", "hebrew.sqlite") {
		t.Fatalf("db: %s", layout.DBPath)
	}
}

func TestDataDirMustNotBeServed(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		filepath.Join(root, "shared"),
		filepath.Join(root, "shared", "player"),
		filepath.Join(root, "review"),
		filepath.Join(root, "worksheets"),
		filepath.Join(root, "worksheets", "lesson-1"),
	}
	for _, dir := range cases {
		_, err := Resolve(root, dir, "")
		if err == nil {
			t.Fatalf("DATA_DIR %s was accepted", dir)
		}
		if !strings.Contains(err.Error(), "DATA_DIR must not equal or lie inside a served root") {
			t.Fatalf("error: %v", err)
		}
	}
}

func TestDataDirPrefixIsNotContainment(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "shared-backup")
	layout, err := Resolve(root, data, "")
	if err != nil {
		t.Fatal(err)
	}
	if layout.DataDir != data {
		t.Fatalf("data: %s", layout.DataDir)
	}
}
