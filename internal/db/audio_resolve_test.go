package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestResolveLexemeAudio(t *testing.T) {
	when := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	text := "שָׁלוֹם"
	messy := "\u200e " + norm.NFD.String(text) + "  "
	if units.Normalize(messy) != text {
		t.Fatalf("normalize %q = %q", messy, units.Normalize(messy))
	}
	v4 := strings.Repeat("ab", 32)
	v3 := strings.Repeat("cd", 32)
	older := when.Add(-time.Hour)
	newer := when
	if _, err := s.InsertAudio(ctx, Asset{ID: v3, TextNFC: text, Voice: "v", Model: "eleven_v3", Bytes: 3, CreatedAt: newer}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertAudio(ctx, Asset{ID: v4, TextNFC: text, Voice: "v", Model: "eleven_v4", Bytes: 4, CreatedAt: older}); err != nil {
		t.Fatal(err)
	}
	asset, ok, err := s.AudioByText(ctx, text)
	if err != nil || !ok || asset.ID != v4 {
		t.Fatalf("AudioByText = %+v ok %v err %v, want %s", asset, ok, err, v4)
	}

	// Text match wins over a different lexemes.audio_id, and NFD input uses the NFC row.
	got, ok, err := s.ResolveLexemeAudio(ctx, LexemeAudio{Text: messy, AudioID: v3})
	if err != nil || !ok || got != asset.ID {
		t.Fatalf("ResolveLexemeAudio = %q ok %v err %v, want %s", got, ok, err, asset.ID)
	}

	plain := units.Plain(text)
	if plain == text {
		t.Fatal("plain fixture collapsed")
	}
	miss, ok, err := s.ResolveLexemeAudio(ctx, LexemeAudio{Text: plain})
	if err != nil || ok || miss != "" {
		t.Fatalf("unpointed ResolveLexemeAudio = %q ok %v err %v, want none", miss, ok, err)
	}

	// No row for this text: the column is the clip.
	fallback, ok, err := s.ResolveLexemeAudio(ctx, LexemeAudio{Text: "אֵין", AudioID: v3})
	if err != nil || !ok || fallback != v3 {
		t.Fatalf("fallback = %q ok %v err %v", fallback, ok, err)
	}
	none, ok, err := s.ResolveLexemeAudio(ctx, LexemeAudio{Text: "אֵין"})
	if err != nil || ok || none != "" {
		t.Fatalf("empty = %q ok %v err %v", none, ok, err)
	}

	// Same model: newer created_at, then the lower id. AudioByText is the oracle.
	first := strings.Repeat("11", 32)
	second := strings.Repeat("22", 32)
	other := "בֹּקֶר"
	if _, err := s.InsertAudio(ctx, Asset{ID: second, TextNFC: other, Voice: "v", Model: "legacy", Bytes: 1, CreatedAt: older}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertAudio(ctx, Asset{ID: first, TextNFC: other, Voice: "v", Model: "legacy", Bytes: 1, CreatedAt: newer}); err != nil {
		t.Fatal(err)
	}
	want, ok, err := s.AudioByText(ctx, other)
	if err != nil || !ok {
		t.Fatal(err)
	}
	got, ok, err = s.ResolveLexemeAudio(ctx, LexemeAudio{Text: other, AudioID: second})
	if err != nil || !ok || got != want.ID || got != first {
		t.Fatalf("newer = %q want %q (audio %s)", got, first, want.ID)
	}

	tied := "עֶרֶב"
	low := strings.Repeat("0a", 32)
	high := strings.Repeat("0b", 32)
	for _, id := range []string{high, low} {
		if _, err := s.InsertAudio(ctx, Asset{ID: id, TextNFC: tied, Voice: "v", Model: "legacy", Bytes: 1, CreatedAt: when}); err != nil {
			t.Fatal(err)
		}
	}
	want, ok, err = s.AudioByText(ctx, tied)
	if err != nil || !ok {
		t.Fatal(err)
	}
	got, ok, err = s.ResolveLexemeAudio(ctx, LexemeAudio{Text: tied})
	if err != nil || !ok || got != want.ID || got != low {
		t.Fatalf("tie = %q want %q (audio %s)", got, low, want.ID)
	}
}

func TestResolveLexemeAudioBatch(t *testing.T) {
	when := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()

	// One past the chunk size, so a second read has to run.
	const n = audioResolveChunk + 1
	lexemes := make([]LexemeAudio, n)
	want := make([]string, n)
	for i := 0; i < n; i++ {
		text := "w" + strings.Repeat("א", 1) + itoa(i)
		id := strings.Repeat("ab", 31) + twoHex(i)
		if _, err := s.InsertAudio(ctx, Asset{ID: id, TextNFC: text, Voice: "v", Model: "eleven_v3", Bytes: 1, CreatedAt: when}); err != nil {
			t.Fatal(err)
		}
		lexemes[i] = LexemeAudio{Text: " " + text + " ", AudioID: "ignored"}
		want[i] = id
	}
	lexemes = append(lexemes, LexemeAudio{Text: "אֵין", AudioID: want[0]})
	want = append(want, want[0])

	got, err := s.ResolveLexemeAudioBatch(ctx, lexemes)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("len %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d = %q want %q", i, got[i], want[i])
		}
		one, ok, err := s.ResolveLexemeAudio(ctx, lexemes[i])
		if err != nil || !ok || one != got[i] {
			t.Fatalf("single %d = %q ok %v err %v, batch %q", i, one, ok, err, got[i])
		}
	}
	empty, err := s.ResolveLexemeAudioBatch(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty batch = %#v err %v", empty, err)
	}
}

func twoHex(i int) string {
	const hexdigits = "0123456789abcdef"
	return string([]byte{hexdigits[(i>>4)&0xf], hexdigits[i&0xf]})
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [8]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}
