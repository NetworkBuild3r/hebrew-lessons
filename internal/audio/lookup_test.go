package audio

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestStoreAudioHasAudio(t *testing.T) {
	data := t.TempDir()
	store := openStoreAt(t, data, time.Date(2026, 10, 18, 3, 0, 0, 0, time.UTC))
	ctx := context.Background()
	text := "שָׁלוֹם"
	inserted, err := store.InsertAudio(ctx, db.Asset{
		ID:        strings.Repeat("ab", 32),
		TextNFC:   text,
		Voice:     "voice-a",
		Model:     "eleven_v4",
		Bytes:     4,
		CreatedAt: time.Date(2026, 10, 18, 3, 0, 0, 0, time.UTC),
	})
	if err != nil || !inserted {
		t.Fatalf("insert inserted=%v err=%v", inserted, err)
	}
	lk := StoreAudio{Store: store}
	var _ units.Lookup = lk

	ok, err := lk.HasAudio(ctx, "  "+text+" \u200e")
	if err != nil || !ok {
		t.Fatalf("HasAudio(normalized) = %v, %v", ok, err)
	}
	ok, err = lk.HasAudio(ctx, "אֵין")
	if err != nil || ok {
		t.Fatalf("HasAudio(missing) = %v, %v", ok, err)
	}
	known, err := lk.KnownLexemes(ctx, 1)
	if err != nil || len(known) != 0 {
		t.Fatalf("KnownLexemes = %v, %v", known, err)
	}
	if _, ok, err := lk.LexemeByText(ctx, text); err != nil || ok {
		t.Fatalf("LexemeByText ok=%v err=%v", ok, err)
	}
	if _, ok, err := lk.Facts(ctx, text); err != nil || ok {
		t.Fatalf("Facts ok=%v err=%v", ok, err)
	}
}
