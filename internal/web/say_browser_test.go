//go:build browser

package web

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// SPEC-003 ac-3. The say step at 390 and 1440, light and dark, from the keyboard,
// and with script turned off. Run with MOTION_PW_DIR set:
//
//	go test -tags browser -run TestSayStepInBrowser ./internal/web
func TestSayStepInBrowser(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	origin, cookie, paths := startSayServer(t)
	shots := t.TempDir()
	raw, err := json.Marshal(paths)
	if err != nil {
		t.Fatal(err)
	}
	out := runNode(t, dir, "say-browser.mjs",
		"ORIGIN="+origin, "SESSION="+cookie, "PATHS="+string(raw), "SHOTS="+shots)
	assertBrowserChecks(t, out, 20)
	for _, name := range []string{
		"say-ask-390-light-pointer.png",
		"say-reveal-390-light-pointer.png",
		"say-ask-1440-dark-pointer.png",
		"say-reveal-1440-dark-pointer.png",
		"say-ask-390-light-keyboard.png",
		"say-reveal-1440-dark-noscript.png",
	} {
		info, err := os.Stat(filepath.Join(shots, name))
		if err != nil || info.Size() == 0 {
			t.Errorf("screenshot %s: %v", name, err)
		}
	}
}

func startSayServer(t *testing.T) (origin, cookie string, paths []string) {
	t.Helper()
	const word = "\u05e9\u05b8\u05dc\u05d5\u05b9\u05dd"
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	ctx := context.Background()
	srv := httptest.NewUnstartedServer(nil)
	origin = "http://" + srv.Listener.Addr().String()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	sp := func(s string) *string { return &s }
	if _, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: word, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
	}); err != nil {
		t.Fatal(err)
	}
	clipID := strings.Repeat("ab", 32)
	if _, err := store.InsertAudio(ctx, db.Asset{
		ID: clipID, TextNFC: units.Normalize(word), Voice: "voice", Model: "eleven_v3", Bytes: 4,
	}); err != nil {
		t.Fatal(err)
	}
	audioDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(audioDir, clipID+".mp3"), silentMP3(8), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		unitID := putLesson(t, store, nil, units.Unit{
			Title: "Greeting", Situation: "A hello.", Concept: "Say hello.", Position: i + 1,
			NewWords: []string{word},
			Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{{
				Type: units.ActivitySay, Text: word, PromptEN: "good morning",
			}}}},
		}, "shared")
		publish(t, store, unitID)
		paths = append(paths, "/lesson/"+itoa(unitID)+"/0")
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin: origin, Clock: clk, Store: store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv.Config.Handler = NewOptions(nil, svc, Options{Store: store, AudioDir: audioDir})
	srv.Start()
	t.Cleanup(srv.Close)
	cookie = mintSession(t, store, user.ID, "brian-csrf", now.Add(time.Hour)).Value
	return origin, cookie, paths
}

func assertBrowserChecks(t *testing.T, out []byte, min int) {
	t.Helper()
	var got struct {
		Checks []struct {
			Name   string `json:"name"`
			OK     bool   `json:"ok"`
			Detail string `json:"detail"`
		} `json:"checks"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("bad output %q: %v", out, err)
	}
	if len(got.Checks) < min {
		t.Fatalf("only %d checks ran: %s", len(got.Checks), out)
	}
	for _, c := range got.Checks {
		if !c.OK {
			t.Errorf("FAIL %s: %s", c.Name, c.Detail)
		}
	}
	for _, e := range got.Errors {
		t.Errorf("page error: %s", e)
	}
}
