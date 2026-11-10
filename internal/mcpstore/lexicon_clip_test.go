package mcpstore

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

func TestSearchLexemeAudioPresentUsesEitherClip(t *testing.T) {
	ctx := context.Background()
	store, raw := openAdapter(t, nil)
	idOnly := mustLexeme(t, raw, "כֶּלֶב")
	mustGloss(t, raw, "כֶּלֶב", "dog")
	audioID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := raw.InsertAudio(ctx, db.Asset{
		ID: audioID, TextNFC: "unused-id-clip", Voice: "v", Model: "legacy", Bytes: 4,
		CreatedAt: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `UPDATE lexemes SET audio_id = ? WHERE id = ?`, audioID, idOnly); err != nil {
		t.Fatal(err)
	}
	assetWord := mustLexeme(t, raw, "חָתוּל")
	mustGloss(t, raw, "חָתוּל", "cat")
	if _, err := raw.InsertAudio(ctx, db.Asset{
		ID:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		TextNFC: "חָתוּל", Voice: "v", Model: "legacy", Bytes: 4,
		CreatedAt: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	_ = assetWord
	neither := mustLexeme(t, raw, "דָּג")
	mustGloss(t, raw, "דָּג", "fish-only")
	_ = neither

	dog, _, err := store.SearchLexemes(ctx, "dog", 8)
	if err != nil || len(dog) != 1 || !dog[0].AudioPresent {
		t.Fatalf("audio_id-only %+v, %v", dog, err)
	}
	cat, _, err := store.SearchLexemes(ctx, "cat", 8)
	if err != nil || len(cat) != 1 || !cat[0].AudioPresent {
		t.Fatalf("audio_assets-only %+v, %v", cat, err)
	}
	fish, _, err := store.SearchLexemes(ctx, "fish-only", 8)
	if err != nil || len(fish) != 1 || fish[0].AudioPresent {
		t.Fatalf("clipless %+v, %v", fish, err)
	}
}
