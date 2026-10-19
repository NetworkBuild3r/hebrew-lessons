package audio

import (
	"context"
	"os"
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
	if first.MissingFile != 0 || first.Conflict != 0 {
		t.Fatalf("first run: %+v", first)
	}
	second := run()
	if second.Imported != 0 || second.Duplicate != first.Imported || second.Unmapped != first.Unmapped || second.HashMismatch != first.HashMismatch || second.TextMismatch != first.TextMismatch || second.Conflict != 0 || second.Alias != first.Alias || second.MissingFile != 0 {
		t.Fatalf("second run: %+v after %+v", second, first)
	}
}
