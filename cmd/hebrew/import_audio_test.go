package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnknownCommand(t *testing.T) {
	err := run([]string{"nope"})
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error = %v", err)
	}
}

func TestImportCommandRequiresFrom(t *testing.T) {
	err := runImport(nil)
	if err == nil || !strings.Contains(err.Error(), "--from") {
		t.Fatalf("error = %v", err)
	}
}

func TestImportCommand(t *testing.T) {
	data := t.TempDir()
	export := t.TempDir()
	body := []byte("hello-clip")
	if err := os.MkdirAll(filepath.Join(export, "files", "_audio", "words"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(export, "files", "_audio", "words", "a.mp3"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"words":{"a.mp3":{"text":"שָׁלוֹם","voiceId":"v","model":"eleven_v3"}},"lessons":[],"unmapped_word_files":[]}`)
	if err := os.WriteFile(filepath.Join(export, "clip-text-map.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", data)

	if err := runImport([]string{"--dry-run", "--from", export}); err != nil {
		t.Fatal(err)
	}
	if n := countMP3(t, filepath.Join(data, "audio")); n != 0 {
		t.Fatalf("dry-run mp3s = %d", n)
	}
	if err := runImport([]string{"--from", export}); err != nil {
		t.Fatal(err)
	}
	if n := countMP3(t, filepath.Join(data, "audio")); n != 1 {
		t.Fatalf("mp3s = %d", n)
	}
	if _, err := os.Stat(filepath.Join(data, "hebrew.sqlite")); err != nil {
		t.Fatal(err)
	}
	if err := runImport([]string{"--from", export}); err != nil {
		t.Fatal(err)
	}
	if n := countMP3(t, filepath.Join(data, "audio")); n != 1 {
		t.Fatalf("re-run mp3s = %d", n)
	}
}

func TestImportCommandStdin(t *testing.T) {
	data := t.TempDir()
	t.Setenv("DATA_DIR", data)
	tgz := tinyTGZ(t)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = prev })
	go func() {
		_, _ = w.Write(tgz)
		_ = w.Close()
	}()
	if err := runImport([]string{"--from", "-"}); err != nil {
		t.Fatal(err)
	}
	if n := countMP3(t, filepath.Join(data, "audio")); n != 1 {
		t.Fatalf("stdin import mp3s = %d", n)
	}
}

func tinyTGZ(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	clip := []byte("stdin-clip")
	mapBody := []byte(`{"words":{"a.mp3":{"text":"הֵי","voiceId":"v","model":"legacy"}},"lessons":[],"unmapped_word_files":[]}`)
	writeTar := func(name string, body []byte) {
		t.Helper()
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	writeTar("clip-text-map.json", mapBody)
	writeTar("files/_audio/words/a.mp3", clip)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func countMP3(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".mp3") {
			n++
		}
	}
	return n
}
