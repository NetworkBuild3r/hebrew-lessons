//go:build browser

package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	mrand "math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
)

// SPEC-012 in a real browser. These stay out of go test ./... (no browser tag). Run them with:
//
//	MOTION_PW_DIR=<dir with node_modules/playwright and Chromium> go test -tags browser -run 'TestPlacementInBrowser|TestPlacementScreenshots' ./internal/web
//
// TestPlacementInBrowser is ac-4 (the clip plays on load, and a clip that 404s is replaced by the server)
// and ac-5 (keys 1 to 4, ?, Space; focus order and a visible focus ring), plus the picked state, the
// close dialog, the first-tap rule on every answer screen, the typing part and the review's clips.
// TestPlacementScreenshots (SHOTS_DIR set) saves a PNG of every screen at phone and desktop size,
// light and dark.
//
// The server is the real app wiring: the v3 engine on a real store, every clip a row in audio_assets
// and a short silent MP3 on disk. The learners are put in the state a screen needs by driving the
// engine over HTTP first (the same driver the render tests use), then the browser takes it from there.

// silentMP3 (frames of silent MPEG-1 Layer III audio) is shared with dialogue_browser_test.go.

type plBrowserServer struct {
	t      *testing.T
	origin string
	h      http.Handler
	store  *db.Store
	now    time.Time
	n      int

	mu          sync.Mutex
	letters     map[string]bool
	broken      map[string]bool
	breakFirst  bool
	brokenFirst string
}

// startPlacementServer runs the app on one origin. users is how many learners the test may make.
// With breakFirst, the first clip a browser asks for that is not a letter is a 404, and stays one.
func startPlacementServer(t *testing.T, users int, breakFirst bool) *plBrowserServer {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	s := &plBrowserServer{
		t:          t,
		origin:     "http://" + srv.Listener.Addr().String(),
		now:        time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC),
		letters:    map[string]bool{},
		broken:     map[string]bool{},
		breakFirst: breakFirst,
	}
	clk := fixedClock(s.now)
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	s.store = store
	seedAudio(t, store)

	audioDir := t.TempDir()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := conn.QueryContext(context.Background(), `SELECT id FROM audio_assets`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(audioDir, id+".mp3"), silentMP3(120), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rows.Close()
	letters, err := placement.Letters()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range letters {
		sum := sha256.Sum256([]byte("placement-audio:" + l.SyllableText))
		s.letters[hex.EncodeToString(sum[:])+".mp3"] = true
	}

	var emails []string
	for i := 0; i < users; i++ {
		emails = append(emails, fmt.Sprintf("b%d@example.com", i))
	}
	svc, err := auth.New(auth.Config{PublicOrigin: s.origin, CookieSecure: false, Clock: clk, Store: store, AllowedEmails: emails})
	if err != nil {
		t.Fatal(err)
	}
	engine := placement.NewEngine(store, evidence.New(store, fsrs.New(store, clk), clk), clk, placement.WithSource(mrand.NewPCG(1, 7)))
	s.h = NewOptions(nil, svc, Options{Store: store, Runs: store, Deck: engine, AudioDir: audioDir, SessionSecret: "spec-012-test-secret"})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /audio/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		s.mu.Lock()
		if s.breakFirst && s.brokenFirst == "" && !s.letters[name] {
			s.brokenFirst = name
			s.broken[name] = true
		}
		gone := s.broken[name]
		s.mu.Unlock()
		if gone {
			http.NotFound(w, r)
			return
		}
		s.h.ServeHTTP(w, r)
	})
	mux.Handle("/", s.h)
	srv.Config.Handler = mux
	srv.Start()
	t.Cleanup(srv.Close)
	return s
}

// learner makes a user and puts them in a state. The states are the screens a test needs.
func (s *plBrowserServer) learner(state string) placedUser {
	s.t.Helper()
	i := s.n
	s.n++
	u := insertPlacedUser(s.t, s.store, fmt.Sprintf("b%d@example.com", i), fmt.Sprintf("sub-b%d", i), fmt.Sprintf("csrf-b%d", i), s.now)
	u.origin = s.origin
	d := newDriver(s.t, s.h, s.store, u)
	switch state {
	case "intro", "letters":
	case "listening", "break", "reading", "offer":
		d.toScreen(state)
	case "results":
		d.run(mixedScript())
	default:
		s.t.Fatalf("no state %q", state)
	}
	return u
}

func cookiesJSON(t *testing.T, m map[string]placedUser) string {
	t.Helper()
	out := map[string]string{}
	for k, u := range m {
		out[k] = u.cookie.Value
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPlacementInBrowser(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	s := startPlacementServer(t, 24, true)
	users := map[string]placedUser{}
	// The broken clip must be the first non-letter clip the browser asks for, so that user goes first in the script.
	for _, state := range []string{"intro", "listening", "listening2", "listening3", "listening4", "break", "offer", "results"} {
		users[state] = s.learner(strings.TrimRight(state, "234"))
	}
	out := runNode(t, dir, "placement-browser.mjs", "ORIGIN="+s.origin, "COOKIES="+cookiesJSON(t, users))
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
	if len(got.Checks) < 40 {
		t.Fatalf("only %d checks ran: %s", len(got.Checks), out)
	}
	for _, c := range got.Checks {
		if c.OK {
			t.Logf("ok   %s", c.Name)
			continue
		}
		t.Errorf("FAIL %s: %s", c.Name, c.Detail)
	}
	s.mu.Lock()
	broken := s.brokenFirst
	s.mu.Unlock()
	if broken == "" {
		t.Error("the server never broke a clip, so the replacement was not exercised")
	}
}
