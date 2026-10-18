package audio

import (
	"context"
	"strings"
	"testing"
)

func TestElevenLabsStub(t *testing.T) {
	_, err := NewElevenLabs("test-key").Synthesize(context.Background(), "שָׁלוֹם", "voice", "eleven_v3")
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("implemented stub returned %v", err)
	}
	_, err = (*ElevenLabs)(nil).Synthesize(context.Background(), "שָׁלוֹם", "voice", "eleven_v3")
	if err == nil || !strings.Contains(err.Error(), "api key") {
		t.Fatalf("nil client returned %v", err)
	}
}
