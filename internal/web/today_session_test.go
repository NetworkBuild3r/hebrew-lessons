package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// SPEC-011: with the switches off, the side-feature routes are 404 and Today
// does not link them. With each switch on, that route answers again.
func TestSideFeatureSwitchesHideRoutes(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)

	store, off := openShell(t, clk, dueEngine(), nil)
	l := addLearner(t, store, "a@example.com", "sub-off", now)
	finishPlacement(t, store, l.user.ID, now.Add(-40*24*time.Hour))
	for _, path := range []string{"/roots", "/isaiah", "/placement", "/placement/review"} {
		if code := codeFor(t, off, l, path); code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, code)
		}
	}
	if rec := formPost(t, off, l, "/_hx/placement/recheck", url.Values{"_csrf": {l.csrf}}, nil); rec.Code != http.StatusNotFound {
		t.Errorf("check-in = %d, want 404", rec.Code)
	}
	home := getLearner(t, off, l, "/")
	for _, href := range []string{`href="/roots"`, `href="/isaiah"`, `href="/placement"`, "Take the placement", "Start the check-in"} {
		if strings.Contains(home, href) {
			t.Errorf("Today still shows %s", href)
		}
	}

	rootStore, roots := openShellFlags(t, clk, nil, nil, Options{ShowRoots: true})
	rootLearner := addLearner(t, rootStore, "a@example.com", "sub-roots", now)
	if code := codeFor(t, roots, rootLearner, "/roots"); code != http.StatusOK {
		t.Fatalf("SHOW_ROOTS GET /roots = %d", code)
	}

	placeStore, place := openShellFlags(t, clk, nil, nil, Options{ShowPlacement: true})
	placeLearner := addLearner(t, placeStore, "a@example.com", "sub-place", now)
	if code := codeFor(t, place, placeLearner, "/placement"); code != http.StatusOK {
		t.Fatalf("SHOW_PLACEMENT GET /placement = %d", code)
	}

	checkStore, check := openShellFlags(t, clk, dueEngine(), nil, Options{ShowCheckIn: true, ShowPlacement: true})
	who := addLearner(t, checkStore, "a@example.com", "sub-check", now)
	finishPlacement(t, checkStore, who.user.ID, now.Add(-40*24*time.Hour))
	rec := formPost(t, check, who, "/_hx/placement/recheck", url.Values{"_csrf": {who.csrf}}, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/placement" {
		t.Fatalf("check-in on = %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestPlacementRouteIs404WhenTheSwitchIsOff(t *testing.T) {
	h := New(nil, nil)
	for _, path := range []string{"/placement", "/placement/review"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("ungated GET %s = %d, want 404", path, rec.Code)
		}
	}
}
