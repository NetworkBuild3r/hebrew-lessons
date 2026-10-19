package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
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
	file := wordFileName("v", "eleven_v3", "שָׁלוֹם")
	if err := os.MkdirAll(filepath.Join(export, "_audio", "words"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(export, "_audio", "words", file), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(export, "clip-text-map.json"), wordMap(file, body, "שָׁלוֹם", "v", "eleven_v3"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeWordsManifest(t, export, []wordEntry{{file: file, text: "שָׁלוֹם", voice: "v", model: "eleven_v3"}})
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
	file := wordFileName("v", "legacy", "הֵי")
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
	writeTar("clip-text-map.json", wordMap(file, clip, "הֵי", "v", "legacy"))
	writeTar("_audio/words/manifest.json", wordsManifestJSON([]wordEntry{{file: file, text: "הֵי", voice: "v", model: "legacy"}}))
	writeTar("_audio/words/"+file, clip)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func wordFileName(voice, model, text string) string {
	sum := sha256.Sum256([]byte(voice + "\x00" + model + "\x00" + strings.TrimSpace(text)))
	return hex.EncodeToString(sum[:])[:24] + ".mp3"
}

func wordMap(file string, body []byte, text, voice, model string) []byte {
	sum := sha256.Sum256(body)
	raw, err := json.Marshal(map[string]any{
		"schema":  "hebrew-clip-text-map/v2",
		"lessons": []any{},
		"words": []any{map[string]any{
			"source":   "word",
			"file":     file,
			"path":     "_audio/words/" + file,
			"text":     text,
			"voiceId":  voice,
			"model":    model,
			"sha256":   hex.EncodeToString(sum[:]),
			"verified": true,
		}},
		"unmapped": []any{},
	})
	if err != nil {
		panic(err)
	}
	return raw
}

func TestImportCommandExitCode(t *testing.T) {
	t.Run("text-mismatch", func(t *testing.T) {
		data := t.TempDir()
		t.Setenv("DATA_DIR", data)
		export := lessonExport(t, "קָם", "שָׁלוֹם")
		out, err := captureStdout(t, func() error {
			return runImport([]string{"--from", export})
		})
		if err == nil || !strings.Contains(err.Error(), "text-mismatch=1") {
			t.Fatalf("error = %v", err)
		}
		if !strings.Contains(out, "text-mismatch=1") || !strings.Contains(out, "imported=0") {
			t.Fatalf("summary %q", out)
		}
		if n := countMP3(t, filepath.Join(data, "audio")); n != 0 {
			t.Fatalf("mp3s = %d", n)
		}
	})

	t.Run("missing", func(t *testing.T) {
		data := t.TempDir()
		t.Setenv("DATA_DIR", data)
		export := t.TempDir()
		file := wordFileName("v", "eleven_v3", "שָׁלוֹם")
		if err := os.WriteFile(filepath.Join(export, "clip-text-map.json"), wordMap(file, []byte("gone"), "שָׁלוֹם", "v", "eleven_v3"), 0o644); err != nil {
			t.Fatal(err)
		}
		writeWordsManifest(t, export, []wordEntry{{file: file, text: "שָׁלוֹם", voice: "v", model: "eleven_v3"}})
		out, err := captureStdout(t, func() error {
			return runImport([]string{"--from", export})
		})
		if err == nil || !strings.Contains(err.Error(), "missing-file=1") {
			t.Fatalf("error = %v", err)
		}
		if !strings.Contains(out, "missing-file=1") {
			t.Fatalf("summary %q", out)
		}
	})

	t.Run("conflict", func(t *testing.T) {
		data := t.TempDir()
		t.Setenv("DATA_DIR", data)
		export := t.TempDir()
		body := []byte("same-bytes")
		if err := os.MkdirAll(filepath.Join(export, "_audio", "words"), 0o755); err != nil {
			t.Fatal(err)
		}
		words := []any{}
		for _, text := range []string{"שָׁלוֹם", "קָם"} {
			file := wordFileName("v", "eleven_v4", text)
			if err := os.WriteFile(filepath.Join(export, "_audio", "words", file), body, 0o644); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(body)
			words = append(words, map[string]any{
				"source":  "word",
				"file":    file,
				"path":    "_audio/words/" + file,
				"text":    text,
				"voiceId": "v",
				"model":   "eleven_v4",
				"sha256":  hex.EncodeToString(sum[:]),
			})
		}
		raw, err := json.Marshal(map[string]any{
			"schema": "hebrew-clip-text-map/v2", "lessons": []any{}, "words": words, "unmapped": []any{},
		})
		entries := make([]wordEntry, 0, len(words))
		for _, item := range words {
			obj := item.(map[string]any)
			entries = append(entries, wordEntry{
				file: obj["file"].(string), text: obj["text"].(string), voice: "v", model: "eleven_v4",
			})
		}
		writeWordsManifest(t, export, entries)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(export, "clip-text-map.json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := captureStdout(t, func() error {
			return runImport([]string{"--from", export})
		})
		if err == nil || !strings.Contains(err.Error(), "conflict=1") {
			t.Fatalf("error = %v\n%s", err, out)
		}
		if !strings.Contains(out, "conflict=1") {
			t.Fatalf("summary %q", out)
		}
	})
}

// lessonExport is a lesson whose map text and textHash were changed, while
// audio/manifest.json still names the honest text.
func lessonExport(t *testing.T, forged, honest string) string {
	t.Helper()
	export := t.TempDir()
	const voice, model = "v", "eleven_v4"
	body := []byte("lesson-bytes")
	clip := "audio/story/line-1.mp3"
	if err := os.MkdirAll(filepath.Join(export, "demo", "audio", "story"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(export, "demo", clip), body, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	raw, err := json.Marshal(map[string]any{
		"schema": "hebrew-clip-text-map/v2",
		"lessons": []any{map[string]any{
			"source":   "lesson",
			"lesson":   "demo",
			"clip":     clip,
			"path":     "demo/" + clip,
			"text":     forged,
			"voiceId":  voice,
			"model":    model,
			"textHash": textHash(voice, model, forged),
			"sha256":   hex.EncodeToString(sum[:]),
		}},
		"words":    []any{},
		"unmapped": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(export, "clip-text-map.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := json.Marshal(map[string]any{
		"version": 1,
		"clips":   map[string]string{clip: textHash(voice, model, honest)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(export, "demo", "audio", "manifest.json"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	return export
}

type wordEntry struct {
	file, text, voice, model string
}

func writeWordsManifest(t *testing.T, dir string, entries []wordEntry) {
	t.Helper()
	manifestDir := filepath.Join(dir, "_audio", "words")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manifestDir, "manifest.json"), wordsManifestJSON(entries), 0o644); err != nil {
		t.Fatal(err)
	}
}

func wordsManifestJSON(entries []wordEntry) []byte {
	out := map[string]any{}
	voice, model := "v", "eleven_v4"
	for _, ent := range entries {
		voice, model = ent.voice, ent.model
		out[ent.voice+"|"+ent.model+"|"+ent.text] = map[string]any{
			"text": ent.text, "file": ent.file, "voiceId": ent.voice, "model": ent.model,
		}
	}
	raw, err := json.Marshal(map[string]any{
		"version": 1, "voiceId": voice, "model": model, "entries": out,
	})
	if err != nil {
		panic(err)
	}
	return raw
}

func textHash(voice, model, text string) string {
	sum := sha256.Sum256([]byte(voice + "\x00" + model + "\x00" + strings.TrimSpace(text)))
	return hex.EncodeToString(sum[:])
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = prev }()
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	callErr := fn()
	_ = w.Close()
	<-done
	_ = r.Close()
	return buf.String(), callErr
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
