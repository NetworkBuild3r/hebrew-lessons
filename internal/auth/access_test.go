package auth

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

func TestAccessRecheckedEachRequest(t *testing.T) {
	clk := &manualClock{t: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	store := openTestStore(t, clk)
	admin, err := store.CreateUser(context.Background(), "admin@example.com", "sub-a", RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	access := NewAccess([]string{"admin@example.com"}, []string{"admin@example.com"})
	svc, err := New(Config{
		PublicOrigin: "https://hebrew.example",
		Clock:        clk,
		Store:        store,
		Access:       access,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := svc.createSession(context.Background(), admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := svc.createSession(context.Background(), admin.ID)
	if err != nil {
		t.Fatal(err)
	}

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := CurrentUser(r.Context())
		w.Header().Set("X-Role", u.Role)
		w.WriteHeader(http.StatusNoContent)
	})
	mux := http.NewServeMux()
	mux.Handle("GET /page", svc.RequireUser(ok))
	mux.Handle("GET /admin", svc.RequireAdmin(ok))
	h := svc.Middleware(mux)

	call := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: cookieSession, Value: raw})
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	if rr := call("/admin"); rr.Code != http.StatusNoContent || rr.Header().Get("X-Role") != RoleAdmin {
		t.Fatalf("admin %d %s", rr.Code, rr.Header().Get("X-Role"))
	}

	access.Set([]string{"admin@example.com"}, nil)
	if rr := call("/admin"); rr.Code != http.StatusForbidden {
		t.Fatalf("demoted admin %d", rr.Code)
	}
	if rr := call("/page"); rr.Code != http.StatusNoContent || rr.Header().Get("X-Role") != RoleLearner {
		t.Fatalf("demoted page %d %s", rr.Code, rr.Header().Get("X-Role"))
	}
	row, err := store.UserByEmail(context.Background(), "admin@example.com")
	if err != nil || row.Role != RoleLearner {
		t.Fatalf("stored role %+v err %v", row, err)
	}

	access.Set(nil, nil)
	if rr := call("/page"); rr.Code != http.StatusFound {
		t.Fatalf("removed %d", rr.Code)
	}
	if _, _, err := svc.Lookup(context.Background(), raw); err != ErrNoSession {
		t.Fatalf("current session: %v", err)
	}
	if _, _, err := svc.Lookup(context.Background(), other); err != ErrNoSession {
		t.Fatalf("other session: %v", err)
	}
}

func TestStateKeyIsNotTheClientSecret(t *testing.T) {
	clk := &manualClock{t: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	makeSvc := func() *Service {
		t.Helper()
		store := openTestStore(t, clk)
		svc, err := New(Config{
			PublicOrigin:       "https://hebrew.example",
			GoogleClientID:     "test-client",
			GoogleClientSecret: "shared-secret",
			GoogleAuthURL:      "https://issuer.test/auth",
			GoogleTokenURL:     "https://issuer.test/token",
			GoogleJWKSURL:      "https://issuer.test/jwks",
			GoogleIssuer:       "https://issuer.test",
			Clock:              clk,
			Store:              store,
		})
		if err != nil {
			t.Fatal(err)
		}
		return svc
	}
	a := makeSvc()
	b := makeSvc()
	sealed, err := a.sealState(oidcState{
		State: "state", Nonce: "nonce", Verifier: "verifier",
		Exp: db.FormatTime(a.now().Add(time.Minute)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.openState(sealed); err == nil {
		t.Fatal("another process accepted a state cookie sealed with the same client secret")
	}
	if _, err := a.openState(sealed); err != nil {
		t.Fatal(err)
	}
}

func TestStateSecretSharedAcrossServices(t *testing.T) {
	clk := &manualClock{t: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	newSvc := func(secret string) *Service {
		t.Helper()
		store := openTestStore(t, clk)
		svc, err := New(Config{
			PublicOrigin: "https://hebrew.example",
			StateSecret:  secret,
			Clock:        clk,
			Store:        store,
		})
		if err != nil {
			t.Fatal(err)
		}
		return svc
	}
	a := newSvc("shared-session-secret")
	b := newSvc("shared-session-secret")
	c := newSvc("other-session-secret")
	sealed, err := a.sealState(oidcState{
		State: "state", Nonce: "nonce", Verifier: "verifier",
		Exp: db.FormatTime(a.now().Add(time.Minute)),
	})
	if err != nil {
		t.Fatal(err)
	}
	opened, err := b.openState(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if opened.State != "state" || opened.Nonce != "nonce" || opened.Verifier != "verifier" {
		t.Fatalf("opened %+v", opened)
	}
	if _, err := c.openState(sealed); err == nil {
		t.Fatal("a different HEBREW_SESSION_SECRET accepted the state cookie")
	}
}

func TestUnsetStateSecretWarnsOnce(t *testing.T) {
	warnStateSecretOnce = sync.Once{}
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(prev)
		warnStateSecretOnce = sync.Once{}
	})

	clk := &manualClock{t: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	for range 2 {
		store := openTestStore(t, clk)
		if _, err := New(Config{Clock: clk, Store: store}); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Count(buf.String(), "HEBREW_SESSION_SECRET is unset"); got != 1 {
		t.Fatalf("warning count %d in %s", got, buf.String())
	}
}
