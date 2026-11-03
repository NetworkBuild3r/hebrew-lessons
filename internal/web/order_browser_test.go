//go:build browser

package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

// INIT-029/SPEC-004 AC3: place chips by drag and tap, check, see right-order
// reveal on a wrong answer. Needs Playwright:
//
//	MOTION_PW_DIR=<dir> go test -tags browser -run TestOrderScreenInBrowser ./internal/web
func TestOrderScreenInBrowser(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	f := setupOrderLesson(t, []string{orderAcceptLine()})
	srv := httptest.NewUnstartedServer(f.h)
	srv.Start()
	t.Cleanup(srv.Close)

	_, ask := f.openAsk(t)
	wrong := wrongPermutation(ask.OrderChips)
	var parts []string
	for _, p := range wrong {
		parts = append(parts, strconv.Itoa(p))
	}

	out := runNode(t, dir, "order-browser.mjs",
		"ORIGIN="+srv.URL,
		"SESSION_COOKIE="+f.cookie.Value,
		"PATH_START="+f.path,
		"WRONG_ORDER="+strings.Join(parts, ","),
	)
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
	if len(got.Checks) < 8 {
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
