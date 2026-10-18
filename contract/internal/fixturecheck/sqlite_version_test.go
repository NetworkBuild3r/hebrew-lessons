package fixturecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSQLiteVersionMatchesGoModule fails when contract/go.mod and go/go.mod
// pin different modernc.org/sqlite versions. The repo carries one version.
func TestSQLiteVersionMatchesGoModule(t *testing.T) {
	contractDir := moduleRoot(t)
	contractMod := filepath.Join(contractDir, "go.mod")
	goMod := filepath.Join(filepath.Dir(contractDir), "go", "go.mod")
	contractVer := sqliteModuleVersion(t, contractMod)
	goVer := sqliteModuleVersion(t, goMod)
	if contractVer != goVer {
		t.Fatalf("modernc.org/sqlite contract/go.mod %s != go/go.mod %s", contractVer, goVer)
	}
}

func sqliteModuleVersion(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var found string
	for _, line := range strings.Split(string(body), "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "//") {
			continue
		}
		fields := strings.Fields(trim)
		for i, field := range fields {
			if field != "modernc.org/sqlite" || i+1 >= len(fields) {
				continue
			}
			ver := fields[i+1]
			if found != "" && found != ver {
				t.Fatalf("%s has modernc.org/sqlite %s and %s", path, found, ver)
			}
			found = ver
		}
	}
	if found == "" {
		t.Fatalf("%s does not require modernc.org/sqlite", path)
	}
	return found
}
