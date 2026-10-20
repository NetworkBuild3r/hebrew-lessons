package db

import (
	"context"
	"strings"
	"testing"
	"time"
)

// INIT-015/SPEC-026: a clip is found by text and by voice. The same text can have a
// clip in each voice, and a lookup for one voice never returns the other's.
func TestAudioByTextVoice(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, fixedClock{time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)})
	const text = "text-key"
	at := func(h int) time.Time { return time.Date(2026, 10, 19, h, 0, 0, 0, time.UTC) }
	rows := []Asset{
		{ID: strings.Repeat("a1", 32), TextNFC: text, Voice: "voice-f", Model: "legacy", Bytes: 1, CreatedAt: at(1)},
		{ID: strings.Repeat("a2", 32), TextNFC: text, Voice: "voice-f", Model: "eleven_v3", Bytes: 1, CreatedAt: at(2)},
		{ID: strings.Repeat("a3", 32), TextNFC: text, Voice: "voice-m", Model: "eleven_v3", Bytes: 1, CreatedAt: at(3)},
		{ID: strings.Repeat("a4", 32), TextNFC: "other-text", Voice: "voice-m", Model: "eleven_v3", Bytes: 1, CreatedAt: at(4)},
	}
	for _, r := range rows {
		if ok, err := s.InsertAudio(ctx, r); err != nil || !ok {
			t.Fatalf("insert %s: %v %v", r.ID[:2], ok, err)
		}
	}
	f, ok, err := s.AudioByTextVoice(ctx, text, "voice-f")
	if err != nil || !ok || f.ID != rows[1].ID || f.Voice != "voice-f" {
		t.Fatalf("female: %+v %v %v", f, ok, err)
	}
	m, ok, err := s.AudioByTextVoice(ctx, text, "voice-m")
	if err != nil || !ok || m.ID != rows[2].ID || m.Voice != "voice-m" {
		t.Fatalf("male: %+v %v %v", m, ok, err)
	}
	for _, c := range []struct{ text, voice string }{
		{text, "voice-x"}, // a voice with no clip of this text
		{text, ""},        // an empty voice matches nothing
		{text, "VOICE-F"}, // exact, not case-folded
		{"no-such-text", "voice-f"},
		{"other-text", "voice-f"}, // that text has a clip, in the other voice only
	} {
		if a, ok, err := s.AudioByTextVoice(ctx, c.text, c.voice); err != nil || ok {
			t.Fatalf("AudioByTextVoice(%q, %q) = %+v %v %v, want none", c.text, c.voice, a, ok, err)
		}
	}
	// The any-voice lookup is unchanged: newest known model wins, whatever the voice.
	any, ok, err := s.AudioByText(ctx, text)
	if err != nil || !ok || (any.ID != rows[2].ID && any.ID != rows[1].ID) {
		t.Fatalf("AudioByText: %+v %v %v", any, ok, err)
	}
}
