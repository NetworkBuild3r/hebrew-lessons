package app

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
)

func TestReportMissingPlacementAudio(t *testing.T) {
	manifest, err := placement.EmbeddedManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Clips) == 0 {
		t.Fatal("manifest has no clips")
	}
	letter := letterClip(t, manifest)

	t.Run("missing", func(t *testing.T) {
		store := openAudioStore(t)
		got := collectAudioLog(t, context.Background(), store)
		if !strings.Contains(got, "level=WARN") || !strings.Contains(got, fmt.Sprintf("missing=%d", len(manifest.Clips))) {
			t.Fatalf("log %q", got)
		}
		if strings.Count(got, "placement audio") != 1 {
			t.Fatalf("log %q", got)
		}
		assertNoClipText(t, got, manifest)
	})

	t.Run("complete", func(t *testing.T) {
		store := openAudioStore(t)
		insertManifestClips(t, store, manifest, "")
		got := collectAudioLog(t, context.Background(), store)
		if !strings.Contains(got, "level=INFO") || !strings.Contains(got, "missing=0") || strings.Contains(got, "level=WARN") {
			t.Fatalf("log %q", got)
		}
		assertNoClipText(t, got, manifest)
	})

	t.Run("one missing", func(t *testing.T) {
		store := openAudioStore(t)
		insertManifestClips(t, store, manifest, letter.Text)
		want := 0
		for _, c := range manifest.Clips {
			if c.Text == letter.Text {
				want++
			}
		}
		if want == 0 {
			t.Fatal("letter text is not in the manifest")
		}
		got := collectAudioLog(t, context.Background(), store)
		if !strings.Contains(got, "level=WARN") || !strings.Contains(got, fmt.Sprintf("missing=%d", want)) {
			t.Fatalf("log %q, want missing=%d", got, want)
		}
		assertNoClipText(t, got, manifest)
	})

	t.Run("canceled", func(t *testing.T) {
		store := openAudioStore(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got := collectAudioLog(t, ctx, store)
		if got != "" {
			t.Fatalf("cancel logged %q", got)
		}
	})

	t.Run("store closed", func(t *testing.T) {
		store := openAudioStore(t)
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		got := collectAudioLog(t, context.Background(), store)
		if !strings.Contains(got, "level=WARN") || !strings.Contains(got, "err=") {
			t.Fatalf("log %q", got)
		}
		if strings.Contains(got, "missing=") {
			t.Fatalf("closed store logged a count: %q", got)
		}
		assertNoClipText(t, got, manifest)
	})
}

func TestNewAppLogsMissingPlacementClips(t *testing.T) {
	manifest, err := placement.EmbeddedManifest()
	if err != nil {
		t.Fatal(err)
	}
	var buf lockedBuffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	_ = newTestApp(t, Config{DataDir: t.TempDir()})
	got := buf.waitFor(t, "placement audio")
	if !strings.Contains(got, "level=WARN") || !strings.Contains(got, fmt.Sprintf("missing=%d", len(manifest.Clips))) {
		t.Fatalf("log %q", got)
	}
	assertNoClipText(t, got, manifest)
}

func letterClip(t *testing.T, manifest placement.ClipManifest) placement.Clip {
	t.Helper()
	for _, c := range manifest.Clips {
		if c.Kind == placement.ClipLetter {
			return c
		}
	}
	t.Fatal("manifest has no letter clip")
	return placement.Clip{}
}

func openAudioStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock{time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// insertManifestClips stores one row per distinct manifest text, except skip.
func insertManifestClips(t *testing.T, store *db.Store, manifest placement.ClipManifest, skip string) {
	t.Helper()
	ctx := context.Background()
	seen := map[string]bool{}
	i := 0
	for _, c := range manifest.Clips {
		if c.Text == skip || seen[c.Text] {
			continue
		}
		seen[c.Text] = true
		i++
		if _, err := store.InsertAudio(ctx, db.Asset{
			ID:      fmt.Sprintf("%064x", i),
			TextNFC: c.Text,
			Voice:   "voice",
			Model:   "model",
			Bytes:   1,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func collectAudioLog(t *testing.T, ctx context.Context, store *db.Store) string {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	reportMissingPlacementAudio(ctx, store, logger)
	return buf.String()
}

func assertNoClipText(t *testing.T, log string, manifest placement.ClipManifest) {
	t.Helper()
	for _, c := range manifest.Clips {
		if c.Text != "" && strings.Contains(log, c.Text) {
			t.Fatalf("log contains clip text %q: %s", c.Text, log)
		}
		// Letter ids are long enough to match as themselves. Short word ids
		// such as "lev" occur inside English words in the log line.
		if strings.HasPrefix(c.ID, "letter-") && strings.Contains(log, c.ID) {
			t.Fatalf("log contains letter id %q: %s", c.ID, log)
		}
	}
}

// lockedBuffer is a bytes.Buffer the startup goroutine can write while the test reads.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func (l *lockedBuffer) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b.Reset()
}

func (l *lockedBuffer) waitFor(t *testing.T, sub string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := l.String()
		if strings.Contains(got, sub) {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("log %q missing %q", got, sub)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
