package mcpstore

import (
	"context"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// INIT-015/SPEC-026: the store the MCP tools validate against finds a dialogue line's
// clip in its speaker's voice and in no other.
func TestHasAudioForSpeaker(t *testing.T) {
	ctx := context.Background()
	store, raw := openAdapter(t, nil)
	var _ units.SpeakerLookup = store
	if _, err := raw.InsertAudio(ctx, db.Asset{ID: strings.Repeat("a1", 32), TextNFC: hello, Voice: "vf", Model: "eleven_v3", Bytes: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.InsertAudio(ctx, db.Asset{ID: strings.Repeat("b2", 32), TextNFC: coffee, Voice: "vm", Model: "eleven_v3", Bytes: 1}); err != nil {
		t.Fatal(err)
	}
	// Without voices set, a has the default voice (not "vf"), and b has none.
	for _, c := range []struct{ text, sp string }{{hello, "a"}, {hello, "b"}, {coffee, "a"}, {coffee, "b"}} {
		if ok, err := store.HasAudioForSpeaker(ctx, c.text, c.sp); err != nil || ok {
			t.Fatalf("no voices set: %q %q = %v, %v", c.text, c.sp, ok, err)
		}
	}
	if store.WithVoices(audio.Voices{Female: "vf", Male: "vm"}) != store {
		t.Fatal("WithVoices does not return the store")
	}
	for _, c := range []struct {
		text, sp string
		want     bool
	}{
		{hello, "a", true}, {hello, "b", false}, // a clip in the female voice does not serve b
		{coffee, "b", true}, {coffee, "a", false}, // and the other way round
		{book, "a", false}, {hello, "", false}, {hello, "c", false},
	} {
		if ok, err := store.HasAudioForSpeaker(ctx, c.text, c.sp); err != nil || ok != c.want {
			t.Fatalf("%q %q = %v, %v; want %v", c.text, c.sp, ok, err, c.want)
		}
	}
	// HasAudio stays any voice.
	if ok, err := store.HasAudio(ctx, coffee); err != nil || !ok {
		t.Fatalf("HasAudio = %v, %v", ok, err)
	}
	// With the male voice cleared, b has no clips, though the male row is still there.
	store.WithVoices(audio.Voices{Female: "vf"})
	if ok, err := store.HasAudioForSpeaker(ctx, coffee, "b"); err != nil || ok {
		t.Fatalf("b with no male voice = %v, %v", ok, err)
	}
	if ok, err := (*Store)(nil).HasAudioForSpeaker(ctx, coffee, "a"); err == nil || ok {
		t.Fatalf("a nil store = %v, %v", ok, err)
	}
}
