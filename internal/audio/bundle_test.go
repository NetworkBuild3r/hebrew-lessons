package audio

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestImportVerifiedBundle runs the real audio-import.tgz when
// HEBREW_AUDIO_BUNDLE points at it. CI does not have that archive.
func TestImportVerifiedBundle(t *testing.T) {
	bundle := os.Getenv("HEBREW_AUDIO_BUNDLE")
	if bundle == "" {
		t.Skip("HEBREW_AUDIO_BUNDLE is not set")
	}
	data := t.TempDir()
	store := openStoreAt(t, data, time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC))
	run := func() Report {
		t.Helper()
		rep, err := Import(context.Background(), store, data, bundle, Options{})
		if err != nil {
			t.Fatal(err)
		}
		t.Log(rep.Summary(false))
		return rep
	}
	first := run()
	if first.Imported != 520 || first.Unmapped != 28 || first.HashMismatch != 0 || first.TextMismatch != 0 || first.Conflict != 0 || first.MissingFile != 0 {
		t.Fatalf("first run: %+v", first)
	}
	var lessons, words int
	for _, clip := range readManifest(t, first.ManifestPath) {
		if strings.HasPrefix(clip.Path, "_audio/words/") {
			words++
		} else {
			lessons++
		}
	}
	if lessons != 397 || words != 123 {
		t.Fatalf("manifest lessons=%d words=%d", lessons, words)
	}
	if first.Alias != 61 || len(first.ByModel) != 2 || first.ByModel["eleven_v4"] != 493 || first.ByModel["eleven_v3"] != 27 {
		t.Fatalf("alias=%d models=%v", first.Alias, first.ByModel)
	}
	t.Logf("imported=%d unmapped=%d alias=%d models=%v", first.Imported, first.Unmapped, first.Alias, first.ByModel)
	second := run()
	if second.Imported != 0 || second.Duplicate != 520 || second.Unmapped != 28 || second.HashMismatch != 0 || second.TextMismatch != 0 || second.Conflict != 0 || second.Alias != first.Alias || second.MissingFile != 0 {
		t.Fatalf("second run: %+v after %+v", second, first)
	}
}
