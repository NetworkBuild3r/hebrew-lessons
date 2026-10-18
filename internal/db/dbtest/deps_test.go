package dbtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNotLinkedIntoHebrewBinary runs `go list -deps ./cmd/hebrew` and fails
// if this package is in the server binary's dependency list.
func TestNotLinkedIntoHebrewBinary(t *testing.T) {
	root := moduleRoot(t)
	cmd := exec.Command("go", "list", "-deps", "./cmd/hebrew")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps ./cmd/hebrew: %v\n%s", err, out)
	}
	const pkg = "github.com/NetworkBuild3r/hebrew-lessons/internal/db/dbtest"
	for _, line := range strings.Split(string(out), "\n") {
		if line == pkg {
			t.Fatalf("%s is linked into ./cmd/hebrew\n%s", pkg, out)
		}
	}
	if !strings.Contains(string(out), "github.com/NetworkBuild3r/hebrew-lessons/cmd/hebrew") {
		t.Fatalf("go list -deps did not list the command:\n%s", out)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
