//go:build browser

package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"
)

// SPEC-018 in a real browser. These stay out of go test ./... (no browser tag), like the shell,
// motion, and activity tests. Run them with:
//
//	MOTION_PW_DIR=<dir with node_modules/playwright and Chromium> go test -tags browser -run 'TestProgressInBrowser|TestProgressScreenshots' ./internal/web
//
// TestProgressInBrowser checks what a render test cannot see: the three charts are drawn and sit inside
// the screen at 390 and 1440, in light and dark; the skill choice works from the keyboard, with a
// mouse, and with no script; every tab stop has a visible ring and a 44px target; the chart
// colours really are the tokens' (and never green or red); the text and the ticks have contrast; the
// table opens from the keyboard; Hebrew carries lang and dir; the empty state shows its picture and
// its next step; and reduced motion leaves nothing sliding.
//
// TestProgressScreenshots (SHOTS_DIR set) saves Progress with data, with evidence but no snapshots,
// and empty, at 390 and 1440, light and dark.

// startProgressServer runs the app with one learner for each state the page can be in.
func startProgressServer(t *testing.T) (origin string, cookies map[string]string) {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	origin = "http://" + srv.Listener.Addr().String()
	clk := fixedClock(progNow)
	store, app := openShellAt(t, clk, origin, nil, nil)
	srv.Config.Handler = app
	srv.Start()
	t.Cleanup(srv.Close)

	cookies = map[string]string{}
	data, words := seedProgressLearner(t, store, clk, "learner1@example.com", "sub-data")
	cookies["data"] = data.cookie.Value

	empty := addLearner(t, store, "learner2@example.com", "sub-empty", progNow)
	cookies["empty"] = empty.cookie.Value

	evidence := addLearner(t, store, "learner3@example.com", "sub-evidence", progNow)
	cookies["evidence"] = evidence.cookie.Value
	strugglingWord(t, store, clk, evidence.user.ID, words[0].id, "listen", 4, 1)
	strugglingWord(t, store, clk, evidence.user.ID, words[1].id, "read", 1, 5)

	// A learner with one snapshot, for the single dot.
	one := addLearner(t, store, "learner4@example.com", "sub-one", progNow)
	cookies["one"] = one.cookie.Value
	seedProgressSnapshots(t, store, one.user.ID, 1, progEst)
	return origin, cookies
}

func TestProgressInBrowser(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	origin, cookies := startProgressServer(t)
	raw, _ := json.Marshal(cookies)
	out := runNode(t, dir, "progress-browser.mjs", "ORIGIN="+origin, "COOKIES="+string(raw))
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
	if len(got.Checks) < 60 {
		t.Fatalf("only %d checks ran: %s", len(got.Checks), out)
	}
	for _, c := range got.Checks {
		if c.OK {
			t.Logf("ok   %s", c.Name)
			continue
		}
		t.Errorf("FAIL %s: %s", c.Name, c.Detail)
	}
	for _, e := range got.Errors {
		t.Errorf("page error: %s", e)
	}
}

func TestProgressScreenshots(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	shots := os.Getenv("SHOTS_DIR")
	if dir == "" || shots == "" {
		t.Skip("set MOTION_PW_DIR and SHOTS_DIR")
	}
	origin, cookies := startProgressServer(t)
	raw, _ := json.Marshal(cookies)
	out := runNode(t, dir, "progress-shots.mjs", "ORIGIN="+origin, "COOKIES="+string(raw), "OUT="+shots)
	t.Logf("%s", out)
}
