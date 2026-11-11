package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLocalSignInCreatesASession(t *testing.T) {
	clk := &manualClock{t: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)}
	store := openTestStore(t, clk)
	svc, err := New(Config{
		PublicOrigin:  "http://127.0.0.1:8787",
		CookieSecure:  false,
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{LocalLearnerEmail},
		LocalDev:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	svc.Register(mux)
	handler := svc.Middleware(mux)

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8787/auth/local", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8787")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Fatalf("location %q", loc)
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "hebrew_session" {
			cookie = c
		}
	}
	if cookie == nil || cookie.Value == "" {
		t.Fatal("no session cookie")
	}

	home := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8787/who", nil)
	home.AddCookie(cookie)
	who := httptest.NewRecorder()
	mux.HandleFunc("GET /who", func(w http.ResponseWriter, r *http.Request) {
		user, ok := CurrentUser(r.Context())
		if !ok || user.Email != LocalLearnerEmail {
			http.Error(w, "no user", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	handler.ServeHTTP(who, home)
	if who.Code != http.StatusOK {
		t.Fatalf("signed-in request %d", who.Code)
	}
}

func TestLocalSignInAbsentWithoutDev(t *testing.T) {
	clk := &manualClock{t: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)}
	store := openTestStore(t, clk)
	svc, err := New(Config{
		PublicOrigin: "http://127.0.0.1:8787",
		Clock:        clk,
		Store:        store,
		LocalDev:     false,
	})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	svc.Register(mux)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8787/auth/local", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8787")
	rec := httptest.NewRecorder()
	svc.Middleware(mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}
