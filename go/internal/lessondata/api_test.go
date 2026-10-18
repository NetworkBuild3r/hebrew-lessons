package lessondata

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "worksheet.js")
	body := "window.X = {a:1};\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Parse(body, path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Write() != want.Write() {
		t.Fatalf("Load wrote %q, Parse wrote %q", doc.Write(), want.Write())
	}
	if _, err := Load(filepath.Join(dir, "missing.js")); err == nil {
		t.Fatal("missing file returned nil error")
	}
}

func TestEmptyFileName(t *testing.T) {
	_, err := Parse("window.X = +1;", "")
	if err == nil || !strings.HasPrefix(err.Error(), "<data>:") {
		t.Fatalf("got %v", err)
	}
}

func TestFormatNumberNonFinite(t *testing.T) {
	if formatNumber(math.NaN()) != "null" || formatNumber(math.Inf(1)) != "null" || formatNumber(math.Inf(-1)) != "null" {
		t.Fatalf("non-finite: %q %q %q", formatNumber(math.NaN()), formatNumber(math.Inf(1)), formatNumber(math.Inf(-1)))
	}
	if stringify(Value{Kind: 99}, "", "") != "null" {
		t.Fatalf("unknown kind: %q", stringify(Value{Kind: 99}, "", ""))
	}
}
