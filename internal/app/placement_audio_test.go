package app

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
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

	a := newTestApp(t, Config{DataDir: t.TempDir()})
	// The done signal, not a poll. Waiting here does not cancel, so the
	// count still logs. Close cancels first and would suppress the line.
	a.clipWG.Wait()
	got := buf.String()
	if !strings.Contains(got, "level=WARN") || !strings.Contains(got, fmt.Sprintf("missing=%d", len(manifest.Clips))) {
		t.Fatalf("log %q", got)
	}
	assertNoClipText(t, got, manifest)

	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	assertNoLogAfterClose(t, &buf)
}

// TestCloseJoinsPlacementAudio closes during the manifest count. Close must
// join that goroutine before it clears Store.conn, and nothing may log after
// Close returns. go.uber.org/goleak is not a module dependency.
//
// One thread makes the count read Store.conn after Close has cleared it,
// unless Wait orders the two. With more threads the read often finishes
// inside the query and the race detector misses it.
func TestCloseJoinsPlacementAudio(t *testing.T) {
	prevProcs := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(prevProcs) })
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	for i := 0; i < 8; i++ {
		var buf lockedBuffer
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
		a, err := NewApp(Config{DataDir: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		if err := a.Close(); err != nil {
			t.Fatal(err)
		}
		assertNoLogAfterClose(t, &buf)
	}
}

// assertNoLogAfterClose fails when a goroutine writes a log line after Close
// has returned. The pause gives a leaked count time to reach the handler.
func assertNoLogAfterClose(t *testing.T, buf *lockedBuffer) {
	t.Helper()
	before := buf.String()
	buf.Seal()
	for range 5 {
		runtime.Gosched()
	}
	time.Sleep(20 * time.Millisecond)
	if late := buf.Late(); late != "" {
		t.Fatalf("logged after Close: %q", late)
	}
	if got := buf.String(); got != before {
		t.Fatalf("log changed after Close: %q", got)
	}
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
// Seal marks Close as returned. A later Write is kept apart so the test can see it.
type lockedBuffer struct {
	mu     sync.Mutex
	b      bytes.Buffer
	late   bytes.Buffer
	sealed bool
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.sealed {
		return l.late.Write(p)
	}
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
	l.late.Reset()
	l.sealed = false
}

func (l *lockedBuffer) Seal() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sealed = true
}

func (l *lockedBuffer) Late() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.late.String()
}
