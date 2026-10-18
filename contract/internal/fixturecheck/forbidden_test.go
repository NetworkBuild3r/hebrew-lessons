package fixturecheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContractTreeOmitsAdultLesson(t *testing.T) {
	root := moduleRoot(t)
	Names(t, root)
	Dir(t, root)
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
