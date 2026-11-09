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
	// The newer row has the greater id, so a sort that ignores created_at
	// and keeps the lower id picks the old clip.
	first := strings.Repeat("ff", 32)
	second := strings.Repeat("11", 32)
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
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	// An empty batch does not need a connection. A missing word does.
	closed, err := s.ResolveLexemeAudioBatch(ctx, nil)
	if err != nil || len(closed) != 0 {
		t.Fatalf("closed empty batch = %#v err %v", closed, err)
	}
	if _, err := s.ResolveLexemeAudioBatch(ctx, []LexemeAudio{{Text: "שָׁלוֹם"}}); err == nil {
		t.Fatal("closed store resolved a word")
	}
}

// TestResolveLexemeAudioLegs names each fallback the review drills rely on.
func TestResolveLexemeAudioLegs(t *testing.T) {
	when := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	text := "שָׁלוֹם"
	older, newer := when.Add(-time.Hour), when

	t.Run("audio assets", func(t *testing.T) {
		id := strings.Repeat("a1", 32)
		if _, err := s.InsertAudio(ctx, Asset{ID: id, TextNFC: text, Voice: "v", Model: "eleven_v4", Bytes: 1, CreatedAt: older}); err != nil {
			t.Fatal(err)
		}
		got, ok, err := s.ResolveLexemeAudio(ctx, LexemeAudio{Text: "\u200e " + norm.NFD.String(text) + " ", AudioID: "column"})
		if err != nil || !ok || got != id {
			t.Fatalf("got %q ok %v err %v", got, ok, err)
		}
	})

	t.Run("audio id fallback", func(t *testing.T) {
		got, ok, err := s.ResolveLexemeAudio(ctx, LexemeAudio{Text: "אֵין", AudioID: "column"})
		if err != nil || !ok || got != "column" {
			t.Fatalf("got %q ok %v err %v", got, ok, err)
		}
	})

	t.Run("model preference", func(t *testing.T) {
		// An unknown model is newer and must still lose to legacy.
		// legacy is newer and must still lose to eleven_v3.
		word := "מַיִם"
		unknown := strings.Repeat("ab", 32)
		legacy := strings.Repeat("cd", 32)
		v3 := strings.Repeat("ef", 32)
		if _, err := s.InsertAudio(ctx, Asset{ID: unknown, TextNFC: word, Voice: "v", Model: "studio", Bytes: 1, CreatedAt: newer}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.InsertAudio(ctx, Asset{ID: legacy, TextNFC: word, Voice: "v", Model: "legacy", Bytes: 1, CreatedAt: older}); err != nil {
			t.Fatal(err)
		}
		got, ok, err := s.ResolveLexemeAudio(ctx, LexemeAudio{Text: word})
		if err != nil || !ok || got != legacy {
			t.Fatalf("legacy over unknown = %q ok %v err %v", got, ok, err)
		}
		if _, err := s.InsertAudio(ctx, Asset{ID: v3, TextNFC: word, Voice: "v", Model: "eleven_v3", Bytes: 1, CreatedAt: older.Add(-time.Hour)}); err != nil {
			t.Fatal(err)
		}
		got, ok, err = s.ResolveLexemeAudio(ctx, LexemeAudio{Text: word, AudioID: unknown})
		if err != nil || !ok || got != v3 {
			t.Fatalf("v3 over legacy = %q ok %v err %v", got, ok, err)
		}
	})

	t.Run("missing clip", func(t *testing.T) {
		got, ok, err := s.ResolveLexemeAudio(ctx, LexemeAudio{Text: "כִּסֵּא"})
		if err != nil || ok || got != "" {
			t.Fatalf("missing = %q ok %v err %v", got, ok, err)
		}
		// Empty and duplicate text: the blank word keeps its column, and
		// both spellings of the same NFC share the one asset.
		blank := LexemeAudio{Text: " \u200e ", AudioID: "kept"}
		again := LexemeAudio{Text: text}
		messy := LexemeAudio{Text: "  " + text + "  ", AudioID: "ignored"}
		gotBatch, err := s.ResolveLexemeAudioBatch(ctx, []LexemeAudio{blank, again, messy})
		if err != nil {
			t.Fatal(err)
		}
		asset := strings.Repeat("a1", 32)
		if len(gotBatch) != 3 || gotBatch[0] != "kept" || gotBatch[1] != asset || gotBatch[2] != asset {
			t.Fatalf("batch = %#v", gotBatch)
		}
	})

	t.Run("empty batch", func(t *testing.T) {
		var closed *Store
		for _, in := range [][]LexemeAudio{nil, {}} {
			got, err := closed.ResolveLexemeAudioBatch(ctx, in)
			if err != nil || len(got) != 0 {
				t.Fatalf("nil store empty %#v err %v", got, err)
			}
		}
		if _, _, err := closed.ResolveLexemeAudio(ctx, LexemeAudio{Text: text}); err == nil {
			t.Fatal("nil store resolved a word")
		}
	})
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
