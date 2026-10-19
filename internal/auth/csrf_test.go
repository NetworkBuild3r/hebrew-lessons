package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCSRFAndLogout(t *testing.T) {
	clk := &manualClock{t: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	store := openTestStore(t, clk)
	user, err := store.CreateUser(context.Background(), "learner@example.com", "sub-1", RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := New(Config{
		PublicOrigin:  "https://hebrew.example",
		CookieSecure:  false,
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

	mux := http.NewServeMux()
	svc.Register(mux)
	mux.HandleFunc("/_hx/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /elsewhere", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	h := svc.Middleware(mux)

	call := func(method, path, token, origin string, form bool) *httptest.ResponseRecorder {
		t.Helper()
		var body *strings.Reader
		if form {
			body = strings.NewReader(url.Values{"_csrf": {token}}.Encode())
		} else {
			body = strings.NewReader("")
		}
		req := httptest.NewRequest(method, path, body)
		req.AddCookie(&http.Cookie{Name: cookieSession, Value: raw})
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if token != "" && !form {
			req.Header.Set("X-CSRF-Token", token)
		}
		if form {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Referer", "https://hebrew.example/lessons")
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	if rr := call(http.MethodPost, "/auth/logout", "", "https://hebrew.example", false); rr.Code != http.StatusForbidden {
		t.Fatalf("missing token %d", rr.Code)
	}
	if rr := call(http.MethodPost, "/auth/logout", sess.CSRF, "https://evil.example", false); rr.Code != http.StatusForbidden {
		t.Fatalf("bad origin %d", rr.Code)
	}
	if rr := call(http.MethodPost, "/auth/logout", "not-the-token", "https://hebrew.example", false); rr.Code != http.StatusForbidden {
		t.Fatalf("bad token %d", rr.Code)
	}
	if rr := call(http.MethodPost, "/_hx/ping", "", "https://hebrew.example", false); rr.Code != http.StatusForbidden {
		t.Fatalf("hx missing token %d", rr.Code)
	}
	if rr := call(http.MethodGet, "/_hx/ping", "", "", false); rr.Code != http.StatusNoContent {
		t.Fatalf("hx get %d", rr.Code)
	}
	if rr := call(http.MethodPost, "/elsewhere", "", "", false); rr.Code != http.StatusForbidden {
		t.Fatalf("other post %d", rr.Code)
	}
	if rr := call(http.MethodPost, "/elsewhere", sess.CSRF, "", false); rr.Code != http.StatusForbidden {
		t.Fatalf("token without origin %d", rr.Code)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/elsewhere", strings.NewReader(""))
	req.AddCookie(&http.Cookie{Name: cookieSession, Value: raw})
	req.Header.Set("Origin", "null")
	req.Header.Set("Referer", "https://hebrew.example/lessons")
	req.Header.Set("X-CSRF-Token", sess.CSRF)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("null origin %d", rr.Code)
	}
	if rr := call(http.MethodPost, "/_hx/ping", sess.CSRF, "", true); rr.Code != http.StatusNoContent {
		t.Fatalf("form csrf %d body %s", rr.Code, rr.Body.String())
	}

	rr = call(http.MethodPost, "/auth/logout", sess.CSRF, "https://hebrew.example", false)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("logout %d %s", rr.Code, rr.Body.String())
	}
	if _, _, err := svc.Lookup(context.Background(), raw); err != ErrNoSession {
		t.Fatalf("session after logout: %v", err)
	}
}

func TestRequireUserAndAdmin(t *testing.T) {
	clk := &manualClock{t: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	store := openTestStore(t, clk)
	learner, err := store.CreateUser(context.Background(), "learner@example.com", "sub-l", RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := store.CreateUser(context.Background(), "admin@example.com", "sub-a", RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := New(Config{
		PublicOrigin:  "https://hebrew.example",
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{"learner@example.com"},
		AdminEmails:   []string{"admin@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	learnerRaw, _, err := svc.createSession(context.Background(), learner.ID)
	if err != nil {
		t.Fatal(err)
	}
	adminRaw, _, err := svc.createSession(context.Background(), admin.ID)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := CurrentUser(r.Context())
		w.Header().Set("X-Email", u.Email)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("GET /page", svc.RequireUser(ok))
	mux.Handle("GET /admin", svc.RequireAdmin(ok))
	mux.Handle("GET /_hx/me", svc.RequireUser(ok))
	h := svc.Middleware(mux)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/page", nil))
	if rr.Code != http.StatusFound || rr.Header().Get("Location") != "/auth/google/start" {
		t.Fatalf("html redirect %d %s", rr.Code, rr.Header().Get("Location"))
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/_hx/me", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("hx %d", rr.Code)
	}

	as := func(raw, path string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: cookieSession, Value: raw})
		out := httptest.NewRecorder()
		h.ServeHTTP(out, req)
		return out
	}
	if rr := as(learnerRaw, "/page"); rr.Code != http.StatusNoContent || rr.Header().Get("X-Email") != "learner@example.com" {
		t.Fatalf("learner page %d", rr.Code)
	}
	if rr := as(learnerRaw, "/admin"); rr.Code != http.StatusForbidden {
		t.Fatalf("learner admin %d", rr.Code)
	}
	if rr := as(adminRaw, "/admin"); rr.Code != http.StatusNoContent || rr.Header().Get("X-Email") != "admin@example.com" {
		t.Fatalf("admin page %d %s", rr.Code, rr.Body.String())
	}
}
