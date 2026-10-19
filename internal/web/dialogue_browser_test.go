//go:build browser

package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// INIT-015/SPEC-026 ac-6 in real Chromium. It stays out of go test ./... (no browser tag),
// like the motion and activity browser tests. Run it with:
//
//	MOTION_PW_DIR=<dir with node_modules/playwright and Chromium> go test -tags browser -run TestDialogueInBrowser ./internal/web
//
// A published dialogue unit is played for real. The test records every audio request the
// page makes, and the audio_assets row behind each one tells the voice. It proves:
//   - "Play the conversation" requests the line clips in order, and exactly two voice ids
//     are asked for, one for each speaker;
//   - the line being heard is marked in order;
//   - replaying one line plays only that line;
//   - pressing "Play the conversation" again stops it after the line that is playing.
func TestDialogueInBrowser(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	srv := httptest.NewUnstartedServer(nil)
	origin := "http://" + srv.Listener.Addr().String()
	audioDir := t.TempDir()
	env := newDialogueEnvAt(t, true, origin, audioDir)
	for _, id := range env.ids {
		if err := os.WriteFile(filepath.Join(audioDir, id+".mp3"), silentMP3(20), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	srv.Config.Handler = env.h
	srv.Start()
	t.Cleanup(srv.Close)

	out := runNode(t, dir, "dialogue-browser.mjs",
		"ORIGIN="+origin, "SESSION_COOKIE="+env.cookie.Value, "UNIT="+itoa(env.unitID))
	var got struct {
		Decodable    bool     `json:"decodable"`
		Conversation []string `json:"conversation"`
		ConvPlayed   []int    `json:"conversationPlayed"`
		Current      []int    `json:"current"`
		Replay       []string `json:"replay"`
		Played       []int    `json:"played"`
		Stopped      []int    `json:"stopped"`
		Errors       []string `json:"errors"`
		Overflow     bool     `json:"overflow"`
		OrbPx        float64  `json:"orbPx"`
		NoAutoplay   bool     `json:"noAutoplay"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("bad output %q: %v", out, err)
	}
	if len(got.Errors) != 0 {
		t.Fatalf("script errors: %v", got.Errors)
	}
	if !got.NoAutoplay {
		t.Error("a clip was requested before the learner pressed anything")
	}
	if !got.Decodable {
		t.Error("Chromium could not decode the test clip, so the end-of-clip chaining was not exercised")
	}
	id := func(u string) string {
		u = strings.TrimPrefix(u, "/audio/")
		return strings.TrimSuffix(u, ".mp3")
	}
	voiceOf := func(u string) string {
		var v string
		conn, err := env.store.DB()
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(`SELECT voice FROM audio_assets WHERE id = ?`, id(u)).Scan(&v); err != nil {
			t.Fatalf("no clip for %s: %v", u, err)
		}
		return v
	}

	wantIDs := []string{env.ids["femaleHello"], env.ids["maleMorning"]}
	if len(got.Conversation) != 2 || id(got.Conversation[0]) != wantIDs[0] || id(got.Conversation[1]) != wantIDs[1] {
		t.Fatalf("the conversation requested %v, want the clips %v in order", got.Conversation, wantIDs)
	}
	v0, v1 := voiceOf(got.Conversation[0]), voiceOf(got.Conversation[1])
	if v0 != dlgFemale || v1 != dlgMale || v0 == v1 {
		t.Fatalf("voices requested, in line order: %q then %q; want %q then %q", v0, v1, dlgFemale, dlgMale)
	}
	if len(got.ConvPlayed) != 2 || got.ConvPlayed[0] != 0 || got.ConvPlayed[1] != 1 {
		t.Fatalf("lines played by the conversation, in order: %v", got.ConvPlayed)
	}
	if len(got.Current) != 2 || got.Current[0] != 0 || got.Current[1] != 1 {
		t.Fatalf("lines marked as heard, in order: %v", got.Current)
	}
	// The clip is already loaded from the conversation, so the page may not ask the server again.
	// If it does, it asks for that line's clip and no other.
	for _, u := range got.Replay {
		if id(u) != env.ids["maleMorning"] {
			t.Fatalf("replaying line 2 requested %v, want only its clip", got.Replay)
		}
	}
	if len(got.Played) != 1 || got.Played[0] != 1 {
		t.Fatalf("replaying line 2 played lines %v, want [1]", got.Played)
	}
	if len(got.Stopped) != 1 || got.Stopped[0] != 0 {
		t.Fatalf("stopping the conversation after line 1 played lines %v, want [0]", got.Stopped)
	}
	if got.Overflow {
		t.Error("the dialogue overflows a 390px screen")
	}
	if got.OrbPx < 44 {
		t.Errorf("a play control is %vpx, under the 44px tap size", got.OrbPx)
	}
}

// silentMP3 is n frames of MPEG-1 Layer III, 44.1 kHz, 128 kbit/s, stereo, all silence.
// Each frame is 417 bytes and 26 ms, so a clip of n=20 is about half a second.
func silentMP3(n int) []byte {
	frame := make([]byte, 417)
	copy(frame, []byte{0xFF, 0xFB, 0x90, 0x00})
	var out []byte
	for i := 0; i < n; i++ {
		out = append(out, frame...)
	}
	return out
}
