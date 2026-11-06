package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// zoneEnv is a signed-in handler plus store for POST /_hx/zone tests.
type zoneEnv struct {
	store  *db.Store
	h      http.Handler
	user   db.User
	other  db.User
	cookie *http.Cookie
	csrf   string
	zones  *studyday.Resolver
}

func newZoneEnv(t *testing.T) zoneEnv {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.CreateUser(context.Background(), "zone@example.com", "sub-zone", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateUser(context.Background(), "other-zone@example.com", "sub-other-zone", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	const csrf = "zone-csrf"
	cookie := mintSession(t, store, user.ID, csrf, now.Add(time.Hour))
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		CookieSecure:  false,
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{"zone@example.com", "other-zone@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	appLoc, err := studyday.AppZone("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	zones, err := studyday.NewResolver(store, appLoc)
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store, Zones: zones})
	return zoneEnv{store: store, h: h, user: user, other: other, cookie: cookie, csrf: csrf, zones: zones}
}

func postZone(t *testing.T, env zoneEnv, cookie *http.Cookie, csrf, zone, origin string, extra url.Values) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	if zone != "" || extra != nil {
		form.Set("zone", zone)
	}
	for k, vs := range extra {
		for _, v := range vs {
			form.Add(k, v)
		}
	}
	if csrf != "" {
		form.Set("_csrf", csrf)
	}
	req := httptest.NewRequest(http.MethodPost, "/_hx/zone", strings.NewReader(form.Encode()))
	req.Host = "learn.example"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	rec := httptest.NewRecorder()
	env.h.ServeHTTP(rec, req)
	return rec
}

func zoneOf(t *testing.T, store *db.Store, id int64) (string, bool) {
	t.Helper()
	zone, ok, err := store.UserTimeZone(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return zone, ok
}

// AC: signed-in POST America/Denver with NULL zone stores it; Resolver.For Source learner.
func TestZoneObserveStoresWhenUnset(t *testing.T) {
	env := newZoneEnv(t)
	rec := postZone(t, env, env.cookie, env.csrf, "America/Denver", "https://learn.example", url.Values{
		"userId": {strconv.FormatInt(env.other.ID, 10)},
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204\n%s", rec.Code, rec.Body.String())
	}
	zone, ok := zoneOf(t, env.store, env.user.ID)
	if !ok || zone != "America/Denver" {
		t.Fatalf("stored zone = %q ok=%v", zone, ok)
	}
	otherZone, otherOK := zoneOf(t, env.store, env.other.ID)
	if otherOK || otherZone != "" {
		t.Fatalf("body userId wrote other learner: zone=%q ok=%v", otherZone, otherOK)
	}
	z, err := env.zones.For(context.Background(), env.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if z.Name != "America/Denver" || z.Source != studyday.SourceLearner {
		t.Fatalf("Resolver.For = %+v", z)
	}
}

// AC: second POST America/New_York leaves Denver.
func TestZoneObserveLeavesExisting(t *testing.T) {
	env := newZoneEnv(t)
	if rec := postZone(t, env, env.cookie, env.csrf, "America/Denver", "https://learn.example", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("first status = %d", rec.Code)
	}
	if rec := postZone(t, env, env.cookie, env.csrf, "America/New_York", "https://learn.example", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("second status = %d", rec.Code)
	}
	zone, ok := zoneOf(t, env.store, env.user.ID)
	if !ok || zone != "America/Denver" {
		t.Fatalf("after second POST: zone=%q ok=%v", zone, ok)
	}
}

// AC: after set_time_zone America/Phoenix, device POST Denver leaves Phoenix.
func TestZoneObserveLeavesStatedZone(t *testing.T) {
	env := newZoneEnv(t)
	if err := env.store.SetUserTimeZone(context.Background(), env.user.ID, "America/Phoenix"); err != nil {
		t.Fatal(err)
	}
	if rec := postZone(t, env, env.cookie, env.csrf, "America/Denver", "https://learn.example", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	zone, ok := zoneOf(t, env.store, env.user.ID)
	if !ok || zone != "America/Phoenix" {
		t.Fatalf("after device POST: zone=%q ok=%v", zone, ok)
	}
}

// AC: Local, empty, whitespace, 65 chars, Mars/Base are 400 and write nothing.
func TestZoneObserveRejectsInvalid(t *testing.T) {
	env := newZoneEnv(t)
	bad := []string{"Local", "", "   ", strings.Repeat("a", 65), "Mars/Base"}
	for _, zone := range bad {
		rec := postZone(t, env, env.cookie, env.csrf, zone, "https://learn.example", nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("zone %q status = %d, want 400", zone, rec.Code)
		}
		got, ok := zoneOf(t, env.store, env.user.ID)
		if ok || got != "" {
			t.Fatalf("zone %q wrote %q", zone, got)
		}
	}
}

// AC: cross-origin and missing CSRF are rejected by existing middleware.
func TestZoneObserveCSRFAndOrigin(t *testing.T) {
	env := newZoneEnv(t)
	cross := postZone(t, env, env.cookie, env.csrf, "America/Denver", "https://evil.example", nil)
	if cross.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d, want 403", cross.Code)
	}
	missing := postZone(t, env, env.cookie, "", "America/Denver", "https://learn.example", nil)
	if missing.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d, want 403", missing.Code)
	}
	got, ok := zoneOf(t, env.store, env.user.ID)
	if ok || got != "" {
		t.Fatalf("rejected POSTs wrote zone=%q ok=%v", got, ok)
	}
}

// AC: signed-out POST writes nothing.
func TestZoneObserveSignedOut(t *testing.T) {
	env := newZoneEnv(t)
	rec := postZone(t, env, nil, env.csrf, "America/Denver", "https://learn.example", nil)
	if rec.Code == http.StatusNoContent {
		// Middleware may 403 before the handler; either way, nothing is stored.
	} else if rec.Code != http.StatusForbidden && rec.Code != http.StatusSeeOther && rec.Code != http.StatusUnauthorized {
		t.Fatalf("signed-out status = %d", rec.Code)
	}
	got, ok := zoneOf(t, env.store, env.user.ID)
	if ok || got != "" {
		t.Fatalf("signed-out POST wrote zone=%q ok=%v", got, ok)
	}
}

// Layout includes zone.js when a CSRF token is present; the script has no retry.
func TestZoneScriptInLayout(t *testing.T) {
	const token = "zone-layout-csrf"
	body := get(t, newMux(nil, CSRFFunc(func(*http.Request) string { return token })), "/")
	if !strings.Contains(body, `/static/js/zone.js`) {
		t.Fatal("signed-in layout missing zone.js")
	}
	js, err := os.ReadFile("static/js/zone.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(js)
	if !strings.Contains(src, "keepalive") || !strings.Contains(src, "/_hx/zone") {
		t.Fatal("zone.js must POST /_hx/zone with keepalive")
	}
	if strings.Contains(src, "retry") || strings.Count(src, "fetch(") != 1 {
		t.Fatal("zone.js must fetch once with no retry")
	}
	if strings.Contains(src, `"UTC"`) {
		t.Fatal("zone.js must not default to UTC")
	}
}
