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

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// SPEC-004 ac-3. One side of a dialogue at 390 and 1440, light and dark, from the
// keyboard, and with script turned off.
func TestDialogueSideInBrowser(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	origin, cookie, paths := startSideServer(t)
	shots := t.TempDir()
	raw, err := json.Marshal(paths)
	if err != nil {
		t.Fatal(err)
	}
	out := runNode(t, dir, "dialogue-side-browser.mjs",
		"ORIGIN="+origin, "SESSION="+cookie, "PATHS="+string(raw), "SHOTS="+shots)
	assertBrowserChecks(t, out, 24)
	for _, name := range []string{
		"side-other-390-light-pointer.png",
		"side-ask-390-light-pointer.png",
		"side-reveal-390-light-pointer.png",
		"side-ask-1440-dark-pointer.png",
		"side-reveal-1440-dark-pointer.png",
		"side-ask-390-light-keyboard.png",
		"side-reveal-1440-dark-noscript.png",
	} {
		info, err := os.Stat(filepath.Join(shots, name))
		if err != nil || info.Size() == 0 {
			t.Errorf("screenshot %s: %v", name, err)
		}
	}
}

func startSideServer(t *testing.T) (origin, cookie string, paths []string) {
	t.Helper()
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
		{TextNFC: dlgHello, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: dlgMorning, Gloss: "morning", Translit: sp("boker"), Gender: sp("m"), POS: sp("noun")},
	}); err != nil {
		t.Fatal(err)
	}
	audioDir := t.TempDir()
	for _, c := range []struct{ id, text, voice string }{
		{strings.Repeat("a1", 32), dlgHello, dlgFemale},
		{strings.Repeat("b2", 32), dlgMorning, dlgMale},
	} {
		if _, err := store.InsertAudio(ctx, db.Asset{ID: c.id, TextNFC: c.text, Voice: c.voice, Model: "eleven_v3", Bytes: 4}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(audioDir, c.id+".mp3"), silentMP3(8), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 6; i++ {
		unitID := putLesson(t, store, nil, units.Unit{
			Title: "Greeting", Situation: "Two people meet.", Concept: "A greeting.", Position: i + 1,
			NewWords: []string{dlgHello, dlgMorning},
			Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{{
				Type: units.ActivityDialogue,
				Play: units.SpeakerB,
				Lines: []units.DialogueLine{
					{Speaker: units.SpeakerA, Text: dlgHello},
					{Speaker: units.SpeakerB, Text: dlgMorning, PromptEN: "good morning"},
				},
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
	srv.Config.Handler = NewOptions(nil, svc, Options{
		Store: store, AudioDir: audioDir, Voices: audio.Voices{Female: dlgFemale, Male: dlgMale},
	})
	srv.Start()
	t.Cleanup(srv.Close)
	cookie = mintSession(t, store, user.ID, "brian-csrf", now.Add(time.Hour)).Value
	return origin, cookie, paths
}
