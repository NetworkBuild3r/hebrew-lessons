//go:build browser

package web

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
)

// The listening step and the Hebrew spotlight in real Chromium: the answers are Hebrew lines, big,
// centered, one to a row; the play control is big and replays; the answered word and the recall
// word are big and centered with the play control below. Run it with:
//
//	MOTION_PW_DIR=<dir with node_modules/playwright and Chromium> go test -tags browser -run TestLessonListenInBrowser ./internal/web
//
// With SHOTS_DIR set it also saves a PNG of each state, light and dark, at 390 and 1440 px.
func TestLessonListenInBrowser(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	for _, size := range []struct{ w, h int }{{390, 844}, {1440, 900}} {
		for _, scheme := range []string{"light", "dark"} {
			srv := httptest.NewUnstartedServer(nil)
			origin := "http://" + srv.Listener.Addr().String()
			fx, unitID, _, _ := listenFixtureAt(t, origin, lsHello, lsMorning, lsNight, lsEvening, lsThanks)
			srv.Config.Handler = fx.h
			srv.Start()
			t.Cleanup(srv.Close)
			cookie, _ := json.Marshal(map[string]string{"name": fx.cookie.Name, "value": fx.cookie.Value})
			out := runNode(t, dir, "lesson-listen-browser.mjs",
				"ORIGIN="+srv.URL, "COOKIE="+string(cookie), "PATH_START=/lesson/"+itoa(unitID)+"/0",
				fmt.Sprintf("WIDTH=%d", size.w), fmt.Sprintf("HEIGHT=%d", size.h), "SCHEME="+scheme, "OUT="+os.Getenv("SHOTS_DIR"))
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
			if len(got.Checks) < 16 {
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
	}
}
