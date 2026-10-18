// Package golden compares canonical JSON to files under golden/.
// Rewrite them with UPDATE_GOLDEN=1 or, from this module's root package,
// go test -update. A golden change is a contract change and goes through review.
package golden

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/normalize"
)

var updateFlag bool

func init() {
	flag.BoolVar(&updateFlag, "update", false, "rewrite contract golden files instead of comparing")
}

// Update reports whether this run should overwrite goldens.
func Update() bool {
	if updateFlag {
		return true
	}
	v := os.Getenv("UPDATE_GOLDEN")
	return v == "1" || strings.EqualFold(v, "true")
}

// Marshal emits stable JSON: sorted object keys, no HTML escaping, trailing newline.
func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Compare checks got against golden/<rel>. In update mode it writes the file
// and the test still passes.
func Compare(t *testing.T, rel string, got any) {
	t.Helper()
	body, err := Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(Root(t), rel)
	if Update() {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated golden %s", rel)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (set UPDATE_GOLDEN=1 to create it)", rel, err)
	}
	if bytes.Equal(want, body) {
		return
	}
	t.Fatalf("golden mismatch %s\n%s", rel, firstDiff(want, body))
}

// CompareRaw checks a live JSON payload against rel.
// Volatile values are masked on both sides, then both are encoded with sorted
// object keys. Key order in the file is not part of the contract. $schema,
// empty properties, and the rest of the wire message stay. In update mode the
// file is rewritten from the pretty-printed live bytes (server key order
// preserved) and the test still passes.
func CompareRaw(t *testing.T, rel string, body []byte) {
	t.Helper()
	if Update() {
		writePretty(t, rel, body)
		return
	}
	got, err := NormalizeRaw(body)
	if err != nil {
		t.Fatalf("live raw %s: %v", rel, err)
	}
	path := filepath.Join(Root(t), rel)
	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (set UPDATE_GOLDEN=1 to create it)", rel, err)
	}
	want, err := NormalizeRaw(wantBytes)
	if err != nil {
		t.Fatalf("golden raw %s: %v", rel, err)
	}
	if bytes.Equal(want, got) {
		return
	}
	t.Fatalf("golden mismatch %s\n%s", rel, firstDiff(want, got))
}

// NormalizeRaw masks volatile JSON values and emits stable JSON.
func NormalizeRaw(body []byte) ([]byte, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, fmt.Errorf("empty payload")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return Marshal(normalize.Mask(v))
}

// writePretty saves informational bytes in update mode. rel is relative to
// the contract module root. Key order in body is preserved.
func writePretty(t *testing.T, rel string, body []byte) {
	t.Helper()
	if !Update() {
		return
	}
	path := filepath.Join(Root(t), rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	pretty := body
	var buf bytes.Buffer
	if err := json.Indent(&buf, body, "", "  "); err == nil {
		buf.WriteByte('\n')
		pretty = buf.Bytes()
	}
	if err := os.WriteFile(path, pretty, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("updated raw %s", rel)
}

// Root is the contract module directory.
func Root(t *testing.T) string {
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

func firstDiff(want, got []byte) string {
	ws := strings.Split(string(want), "\n")
	gs := strings.Split(string(got), "\n")
	n := len(ws)
	if len(gs) > n {
		n = len(gs)
	}
	for i := 0; i < n; i++ {
		var w, g string
		if i < len(ws) {
			w = ws[i]
		}
		if i < len(gs) {
			g = gs[i]
		}
		if w != g {
			return fmt.Sprintf("line %d\n got %s\nwant %s", i+1, g, w)
		}
	}
	return fmt.Sprintf("length got %d want %d", len(got), len(want))
}
