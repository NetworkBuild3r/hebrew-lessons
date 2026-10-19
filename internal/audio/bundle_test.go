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
	if first.Unmapped != 47 || first.HashMismatch != 0 || first.Conflict != 0 || first.MissingFile != 0 {
		t.Fatalf("first run: %+v", first)
	}
	// Nine lesson texts are NFC, while the manifest hashed a different
	// combining-mark order. text.trim() of the stored text does not equal
	// textHash, so those clips are refused.
	if first.Imported != 492 || first.Duplicate != 0 || first.TextMismatch != 9 || first.Alias != 33 {
		t.Fatalf("first run counts: imported=%d duplicate=%d text-mismatch=%d alias=%d",
			first.Imported, first.Duplicate, first.TextMismatch, first.Alias)
	}
	if first.ByModel["eleven_v4"] != 484 || first.ByModel["eleven_v3"] != 8 {
		t.Fatalf("models: %+v", first.ByModel)
	}
	second := run()
	if second.Imported != 0 || second.Duplicate != 492 || second.Unmapped != 47 || second.HashMismatch != 0 || second.TextMismatch != 9 || second.Conflict != 0 || second.Alias != 33 || second.MissingFile != 0 {
		t.Fatalf("second run: %+v", second)
	}
}
