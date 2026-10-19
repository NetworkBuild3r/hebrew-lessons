package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/tanakh"
)

func captureTanakhLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func writeCmdCorpus(t *testing.T, dir, licenseEn string) {
	t.Helper()
	if licenseEn == "" {
		licenseEn = "Public Domain"
	}
	rows := []map[string]any{
		{"book": "genesis", "chapter": 1, "verse": 1, "he": "בְּרֵאשִׁית", "en": "In the beginning."},
	}
	body, err := json.Marshal(map[string]any{"book": "genesis", "title": "Genesis", "rows": rows})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "genesis.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{
		"source":    "sefaria-api",
		"fetchedAt": "2026-10-18T00:00:00Z",
		"versionEn": "The Holy Scriptures: A New Translation (JPS 1917)",
		"versionHe": "Tanach with Nikkud",
		"licenseEn": licenseEn,
		"licenseHe": "Public Domain",
		"books": []map[string]any{
			{"slug": "genesis", "title": "Genesis", "rows": 1, "file": "genesis.json"},
		},
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func oneExpect() *tanakh.Expect {
	return &tanakh.Expect{Books: 1, Verses: 1}
}

func TestImportTanakhCommand(t *testing.T) {
	buf := captureTanakhLog(t)
	data := t.TempDir()
	dir := t.TempDir()
	writeCmdCorpus(t, dir, "")
	t.Setenv("DATA_DIR", data)

	if err := importTanakh(nil, oneExpect()); err == nil || !strings.Contains(err.Error(), "--from") {
		t.Fatalf("missing --from: %v", err)
	}

	buf.Reset()
	if err := importTanakh([]string{"--dry-run", "--from", dir}, oneExpect()); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(buf.String(), "msg=tanakh"); n != 1 {
		t.Fatalf("dry-run log lines = %d\n%s", n, buf.String())
	}
	if !strings.Contains(buf.String(), "action=dry-run") || !strings.Contains(buf.String(), "verses=1") {
		t.Fatalf("log = %s", buf.String())
	}
	s := openCmdDB(t, data)
	n, err := s.CountTanakhVerses(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("dry-run verses = %d %v", n, err)
	}
	_ = s.Close()

	buf.Reset()
	if err := importTanakh([]string{"--from", dir}, oneExpect()); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(buf.String(), "msg=tanakh"); n != 1 || !strings.Contains(buf.String(), "action=imported") {
		t.Fatalf("import log = %s", buf.String())
	}
	buf.Reset()
	if err := importTanakh([]string{"--from", dir}, oneExpect()); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(buf.String(), "msg=tanakh"); n != 1 || !strings.Contains(buf.String(), "action=skipped") {
		t.Fatalf("skip log = %s", buf.String())
	}

	bad := t.TempDir()
	writeCmdCorpus(t, bad, "CC-BY-NC")
	buf.Reset()
	err = importTanakh([]string{"--from", bad}, oneExpect())
	var refused *tanakh.RefusalError
	if !errors.As(err, &refused) || refused.ExitCode() != 2 {
		t.Fatalf("non-PD err = %v", err)
	}
	code, quiet := statusCode(err)
	if code != 2 || !quiet {
		t.Fatalf("status = %d quiet=%v", code, quiet)
	}
	if n := strings.Count(buf.String(), "msg=tanakh"); n != 1 || !strings.Contains(buf.String(), "action=refused") || !strings.Contains(buf.String(), "license") {
		t.Fatalf("refuse log = %s", buf.String())
	}

	buf.Reset()
	err = runImportTanakh([]string{"--from", dir})
	if !errors.As(err, &refused) || refused.ExitCode() != 2 {
		t.Fatalf("wrong count err = %v", err)
	}
	if !strings.Contains(buf.String(), "action=refused") || !strings.Contains(buf.String(), "count") {
		t.Fatalf("count log = %s", buf.String())
	}
	if code, _ := statusCode(err); code != 2 {
		t.Fatalf("count status = %d", code)
	}

	missing := filepath.Join(t.TempDir(), "nope")
	err = importTanakh([]string{"--from", missing}, oneExpect())
	if err == nil {
		t.Fatal("missing dir succeeded")
	}
	if code, quiet := statusCode(err); code != 1 || quiet {
		t.Fatalf("io status = %d quiet=%v err=%v", code, quiet, err)
	}
}

func TestStatusCodeIO(t *testing.T) {
	if code, quiet := statusCode(nil); code != 0 || !quiet {
		t.Fatalf("nil = %d %v", code, quiet)
	}
	if code, quiet := statusCode(errors.New("disk")); code != 1 || quiet {
		t.Fatalf("io = %d %v", code, quiet)
	}
}

func openCmdDB(t *testing.T, data string) *db.Store {
	t.Helper()
	s, err := db.Open(context.Background(), filepath.Join(data, "hebrew.sqlite"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
