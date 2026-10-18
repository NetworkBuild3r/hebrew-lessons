package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

type manualClock struct {
	mu sync.Mutex
	t  time.Time
}

func (m *manualClock) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.t
}

func (m *manualClock) Advance(d time.Duration) {
	m.mu.Lock()
	m.t = m.t.Add(d)
	m.mu.Unlock()
}

func openTestStore(t *testing.T, clk *manualClock) *db.Store {
	t.Helper()
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestSessionCookieLifecycle(t *testing.T) {
	start := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := &manualClock{t: start}
	store := openTestStore(t, clk)
	user, err := store.CreateUser(context.Background(), "learner@example.com", "sub-1", RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := New(Config{
		PublicOrigin:  "https://hebrew.example",
		CookieSecure:  true,
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{"learner@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, sess, err := svc.createSession(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	id, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(id) != sessionIDBytes {
		t.Fatalf("session id len %d err %v", len(id), err)
	}
	sum := sha256.Sum256(id)
	if hex.EncodeToString(sum[:]) != sess.IDHash {
		t.Fatal("stored hash is not sha256 of the cookie")
	}
	if sess.ExpiresAt.Sub(start) != SessionLifetime {
		t.Fatalf("lifetime %s", sess.ExpiresAt.Sub(start))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /who", func(w http.ResponseWriter, r *http.Request) {
		u, ok := CurrentUser(r.Context())
		if !ok {
			http.Error(w, "signed out", http.StatusUnauthorized)
			return
		}
		w.Header().Set("X-Role", u.Role)
		w.Header().Set("X-CSRF", CSRFToken(r))
		w.WriteHeader(http.StatusNoContent)
	})
	h := svc.Middleware(mux)

	req := httptest.NewRequest(http.MethodGet, "/who", nil)
	req.AddCookie(&http.Cookie{Name: cookieSessionHost, Value: raw})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent || rr.Header().Get("X-Role") != RoleLearner {
		t.Fatalf("who %d %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("X-CSRF") != sess.CSRF || rr.Header().Get("Set-Cookie") != "" {
		t.Fatal("fresh session should not slide yet")
	}

	clk.Advance(2 * time.Minute)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatal(rr.Code)
	}
	c := cookieNamed(t, rr, cookieSessionHost)
	if c.Value != raw || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("cookie %+v", c)
	}
	if c.MaxAge != int(SessionLifetime/time.Second) {
		t.Fatalf("max-age %d", c.MaxAge)
	}

	clk.Advance(SessionLifetime + time.Second)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expired status %d", rr.Code)
	}
	cleared := cookieNamed(t, rr, cookieSessionHost)
	if cleared.Value != "" || cleared.MaxAge > 0 {
		t.Fatalf("expired cookie not cleared: %+v", cleared)
	}
}

func TestSecureModeIgnoresPlainSessionCookie(t *testing.T) {
	clk := &manualClock{t: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	store := openTestStore(t, clk)
	user, err := store.CreateUser(context.Background(), "learner@example.com", "sub-1", RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := New(Config{
		PublicOrigin:  "https://hebrew.example",
		CookieSecure:  true,
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{"learner@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := svc.createSession(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /who", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := CurrentUser(r.Context()); !ok {
			http.Error(w, "signed out", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	h := svc.Middleware(mux)

	plain := httptest.NewRequest(http.MethodGet, "/who", nil)
	plain.AddCookie(&http.Cookie{Name: cookieSession, Value: raw})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, plain)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("plain cookie was honoured: %d", rr.Code)
	}

	host := httptest.NewRequest(http.MethodGet, "/who", nil)
	host.AddCookie(&http.Cookie{Name: cookieSessionHost, Value: raw})
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, host)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("host cookie %d %s", rr.Code, rr.Body.String())
	}
}

func cookieNamed(t *testing.T, rr *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range rr.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("cookie %s missing", name)
	return nil
}
