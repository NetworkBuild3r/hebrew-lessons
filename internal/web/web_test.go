package web

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedTrees(t *testing.T) {
	static, err := Static()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(static, "README.txt"); err != nil {
		t.Fatal(err)
	}
	_, err = fs.Stat(Assets, "views")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("views tree should not be embedded, stat err = %v", err)
	}
	_, err = fs.Stat(Assets, "views/README.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("views sources should not be embedded, stat err = %v", err)
	}
}

func TestStaticDoesNotListDirectories(t *testing.T) {
	h := New(nil, nil)
	for _, path := range []string{"/static/", "/static/css/", "/static/js/", "/static/icons/", "/static/fonts/"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, body %q", path, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "<a ") {
			t.Fatalf("%s listed a directory", path)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := New(nil, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assertSecurityHeaders(t, rec)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	req.Host = "learn.example"
	req.Header.Set("Origin", "https://learn.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("write without a token status = %d", rec.Code)
	}
	assertSecurityHeaders(t, rec)
}

func assertSecurityHeaders(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	csp := rec.Header().Get("Content-Security-Policy")
	for _, part := range []string{
		"default-src 'self'",
		"script-src 'self';",
		"frame-ancestors 'none'",
		"object-src 'none'",
	} {
		if !strings.Contains(csp, part) {
			t.Fatalf("CSP %q missing %s", csp, part)
		}
	}
	if strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "unsafe-eval") {
		t.Fatalf("CSP allows inline or eval scripts: %s", csp)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}
	// same-origin keeps referrers off other sites and lets a native form POST carry its real Origin.
	// no-referrer would make the browser send "Origin: null", which the CSRF check refuses.
	if got := rec.Header().Get("Referrer-Policy"); got != "same-origin" {
		t.Fatalf("Referrer-Policy = %q, want same-origin", got)
	}
}

func TestHealthRoute(t *testing.T) {
	h := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d", rr.Code)
	}
}
