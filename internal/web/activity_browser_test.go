//go:build browser

package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// SPEC-016 in a real browser. These stay out of go test ./... (no browser tag), like the
// motion and placement shots tests. Run them with:
//
//	MOTION_PW_DIR=<dir with node_modules/playwright and Chromium> go test -tags browser -run 'TestActivityInBrowser|TestActivityScreenshots' ./internal/web
//
// TestActivityInBrowser is ac-4 (word_match and audio_match are fully operable by keyboard,
// every tile has an accessible name) and the first-tap rule: after an htmx swap, the first
// tap on an option, on Check's successor Continue, or on a tile registers, because no view
// transition runs (motion.js cancels it for c-answer and data-no-transition).
//
// TestActivityScreenshots (SHOTS_DIR set) saves a PNG of every screen at phone and desktop
// size, light and dark.

// fixtureScreen serves one screen from the fixtures in activity_test.go, with no session:
// /__screen/<kind>.<ask|right|wrong>, /__screen/flash.<front|back>, and
// /__screen/flash-recall.<front|back>. It is only mounted by these tests.
func fixtureScreen(w http.ResponseWriter, r *http.Request) {
	lang := "en"
	if r.URL.Query().Get("lang") == "he" {
		lang = "he"
	}
	kind, state, _ := strings.Cut(r.PathValue("name"), ".")
	var card views.ReviewCard
	switch kind {
	case "flash", "flash-recall":
		k := "read"
		if kind == "flash-recall" {
			k = "recall"
		}
		card = views.ReviewCard{
			Lang: lang, CSRF: "t", CardID: 7, Kind: k, Index: 7, Total: 18, Revealed: state == "back",
			Pointed: "לְהַזְמִין", Unpointed: "להזמין", Translit: "le-hazmin", AudioURL: "/audio/" + clipID('a') + ".mp3",
			Gloss: "to invite · to order", Gender: "m",
		}
		if k == "recall" && state != "back" {
			card.Pointed, card.Unpointed, card.Translit, card.AudioURL, card.Gender = "", "", "", "", ""
		}
	default:
		var found *screen
		for _, s := range screens() {
			if string(s.item.Kind) == kind {
				s := s
				found = &s
			}
		}
		if found == nil {
			http.NotFound(w, r)
			return
		}
		st := map[string]*answerState{"ask": nil, "right": found.right, "wrong": found.wrong}[state]
		vm := ReviewCardVM{CardID: 7, Kind: "read", Index: 7, Total: 18, Item: &found.item, Answer: st}
		card = reviewView(lang, "t", "lede", vm)
	}
	render(w, r, views.Review(views.NewPage(lang, "t", "review"), card))
}

// startActivityServer runs the app and the fixtures on one origin. The auth service is
// told that origin, because a POST must carry it.
func startActivityServer(t *testing.T) (origin string, session string) {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	origin = "http://" + srv.Listener.Addr().String()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, app := openReview(t, fixedClock(now), origin)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	words := seedLexicon(t, store, now, 8, true, true)
	giveCards(t, store, learner.user.ID, now, words)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /__screen/{name}", fixtureScreen)
	mux.Handle("/", app)
	srv.Config.Handler = mux
	srv.Start()
	t.Cleanup(srv.Close)
	return origin, learner.cookie.Value
}

func runNode(t *testing.T, dir, script string, env ...string) []byte {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", script))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, script), src, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("node: %v\n%s\n%s", err, out, stderr.String())
	}
	return out
}

func TestActivityInBrowser(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	origin, session := startActivityServer(t)
	out := runNode(t, dir, "activity-browser.mjs", "ORIGIN="+origin, "SESSION_COOKIE="+session)
	var got struct {
		Checks []struct {
			Name   string `json:"name"`
			OK     bool   `json:"ok"`
			Detail string `json:"detail"`
		} `json:"checks"`
		Errors []string `json:"errors"`
		Net    []string `json:"net"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("bad output %q: %v", out, err)
	}
	t.Logf("posts: %v", got.Net)
	if len(got.Checks) < 30 {
		t.Fatalf("only %d checks ran: %s", len(got.Checks), out)
	}
	for _, c := range got.Checks {
		if c.OK {
			t.Logf("ok   %s", c.Name)
			continue
		}
		t.Errorf("FAIL %s: %s", c.Name, c.Detail)
	}
}

func TestActivityScreenshots(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	shots := os.Getenv("SHOTS_DIR")
	if dir == "" || shots == "" {
		t.Skip("set MOTION_PW_DIR and SHOTS_DIR")
	}
	origin, session := startActivityServer(t)
	var names []string
	for _, s := range screens() {
		for _, state := range []string{"ask", "wrong", "right"} {
			names = append(names, string(s.item.Kind)+"."+state)
		}
	}
	names = append(names, "flash.front", "flash.back", "flash-recall.front", "flash-recall.back")
	list, _ := json.Marshal(names)
	out := runNode(t, dir, "activity-shots.mjs", "ORIGIN="+origin, "SESSION_COOKIE="+session, "OUT="+shots, "NAMES="+string(list))
	t.Logf("%s", out)
}
