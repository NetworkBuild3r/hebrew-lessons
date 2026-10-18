package path

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/db"
)

// seedFixture migrates a database with Node and inserts the JSON body.
// The Go service is not the writer. ids maps lowercased usernames to users.id.
func seedFixture(t *testing.T, body any) (string, map[string]int64, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", filepath.Join("testdata", "seed.js"), dir)
	cmd.Stdin = bytes.NewReader(payload)
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("seed: %v\n%s\n%s", err, stderr, out)
	}
	var seeded struct {
		Users map[string]int64 `json:"users"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &seeded); err != nil {
		t.Fatalf("seed stdout %s: %v", out, err)
	}
	conn, err := db.OpenReadOnly(filepath.Join(dir, "hebrew.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return dir, seeded.Users, conn
}

func nodeEnv(tz string) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, item := range os.Environ() {
		if len(item) >= 3 && item[:3] == "TZ=" {
			continue
		}
		env = append(env, item)
	}
	return append(env, "TZ="+tz)
}
