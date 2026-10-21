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

// Lesson-clip checks in real Chromium. They stay out of go test ./... (no
// browser tag). Run them with:
//
//	MOTION_PW_DIR=<dir with node_modules/playwright and Chromium> go test -tags browser -run 'TestRecall' ./internal/web
//
// The lead's ruling replaced the strict "the lesson player renders no
// <audio controls>" test. A clip has one play path in each mode:
//   - html.js present: the native bar is display:none or hidden, absent from
//     the accessibility tree, and not on the Tab order. Opacity or a 1px box
//     does not count.
//   - clips.js absent: the native bar is visible and plays, and the orb is
//     neither visible nor focusable.
// Never both. Hero, word, and dialogue orbs are named buttons of at least
// 44px. Starting a second orb stops the first. A failed clip goes to error
// without the stop colour, and the next press loads again.

type recallOrbCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

func TestRecallClipModesInBrowser(t *testing.T) {
	assertRecallOrbChecks(t, "mode-")
}

func TestRecallOrbsHaveNamesAndHitArea(t *testing.T) {
	assertRecallOrbChecks(t, "orb-")
}

func TestRecallOrbSinglePlaybackAndError(t *testing.T) {
	assertRecallOrbChecks(t, "play-")
}

func assertRecallOrbChecks(t *testing.T, prefix string) {
	t.Helper()
	checks, errs := runRecallOrbBrowser(t)
	if len(errs) != 0 {
		t.Fatalf("script errors: %s", strings.Join(errs, "\n"))
	}
	n := 0
	for _, c := range checks {
		if !strings.HasPrefix(c.Name, prefix) {
			continue
		}
		n++
		if c.OK {
			t.Logf("ok   %s: %s", c.Name, c.Detail)
			continue
		}
		t.Errorf("FAIL %s: %s", c.Name, c.Detail)
	}
	if n == 0 {
		t.Fatalf("no checks with prefix %s: %+v", prefix, checks)
	}
}

func runRecallOrbBrowser(t *testing.T) ([]recallOrbCheck, []string) {
	t.Helper()
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	origin, cookie, unitID, failSrc := startRecallOrbServer(t)
	out := runNode(t, dir, "recall-orb-browser.mjs",
		"ORIGIN="+origin,
		"SESSION_COOKIE="+cookie,
		"UNIT="+itoa(unitID),
		"FAIL_SRC="+failSrc,
	)
	var got struct {
		Checks []recallOrbCheck `json:"checks"`
		Errors []string         `json:"errors"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("bad output %s: %v", out, err)
	}
	return got.Checks, got.Errors
}

// startRecallOrbServer publishes one lesson: a dialogue (the .c-orb) and then
// a three-word recall (the hero orb and the word orbs). The middle word's
// clip is in the database and not on disk, so the player renders its URL and
// the browser can fail that one request.
func startRecallOrbServer(t *testing.T) (origin, cookie string, unitID int64, failSrc string) {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	origin = "http://" + srv.Listener.Addr().String()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	audioDir := t.TempDir()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.CreateUser(ctx, "orb@example.com", "sub-orb", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	sp := func(s string) *string { return &s }
	if _, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: rwShalom, Gloss: "hello", Translit: sp("shalom"), POS: sp("noun"), Gender: sp("m")},
		{TextNFC: rwBoker, Gloss: "morning", Translit: sp("boker"), POS: sp("noun"), Gender: sp("m")},
		{TextNFC: rwAni, Gloss: "I", Translit: sp("ani"), POS: sp("pron")},
		{TextNFC: rwGar, Gloss: "lives", Translit: sp("gar"), POS: sp("verb")},
		{TextNFC: rwPo, Gloss: "here", Translit: sp("po"), POS: sp("adv")},
	}); err != nil {
		t.Fatal(err)
	}
	line := rwAni + " " + rwGar + " " + rwPo
	unitID = putLesson(t, store, nil, units.Unit{
		Title: "Orbs", Situation: "A line and a conversation.", Position: 0,
		NewWords: []string{rwGar},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityDialogue, Lines: []units.DialogueLine{
				{Speaker: units.SpeakerA, Text: rwShalom},
				{Speaker: units.SpeakerB, Text: rwBoker},
			}},
			{Type: units.ActivityRecall, Text: line},
		}}},
	}, "shared")
	publish(t, store, unitID)

	const (
		voiceF = "voice-f"
		voiceM = "voice-m"
	)
	type clip struct {
		id, text, voice string
		file            bool
	}
	failID := strings.Repeat("f4", 32)
	failSrc = "/audio/" + failID + ".mp3"
	clips := []clip{
		{strings.Repeat("a1", 32), units.Normalize(rwShalom), voiceF, true},
		{strings.Repeat("b2", 32), units.Normalize(rwBoker), voiceM, true},
		{strings.Repeat("c3", 32), units.Normalize(line), "narrator", true},
		{strings.Repeat("d4", 32), units.Normalize(rwAni), "narrator", true},
		// The file is real. The browser test answers this URL with 404 while
		// JavaScript is on, so the error state can be retried. With clips.js
		// blocked the same file has to play.
		{failID, units.Normalize(rwGar), "narrator", true},
		{strings.Repeat("e5", 32), units.Normalize(rwPo), "narrator", true},
	}
	for _, c := range clips {
		if _, err := store.InsertAudio(ctx, db.Asset{
			ID: c.id, TextNFC: c.text, Voice: c.voice, Model: "eleven_v4", Bytes: 8,
		}); err != nil {
			t.Fatal(err)
		}
		if c.file {
			if err := os.WriteFile(filepath.Join(audioDir, c.id+".mp3"), silentMP3(40), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin: origin, Clock: fixedClock(now), Store: store,
		AllowedEmails: []string{"orb@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv.Config.Handler = NewOptions(nil, svc, Options{
		Store: store, AudioDir: audioDir, Voices: audio.Voices{Female: voiceF, Male: voiceM},
	})
	srv.Start()
	t.Cleanup(srv.Close)
	c := mintSession(t, store, user.ID, "csrf-orb", now.Add(time.Hour))
	return origin, c.Value, unitID, failSrc
}
