package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCSRFMiddleware(t *testing.T) {
	const token = "tok-1"
	h := newMux(nil, CSRFFunc(func(*http.Request) string { return token }))

	ok := httptest.NewRequest(http.MethodPost, "/review", strings.NewReader(""))
	ok.Host = "learn.example"
	ok.Header.Set("Origin", "https://learn.example")
	ok.Header.Set("X-CSRF-Token", token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, ok)
	if rec.Code == http.StatusForbidden {
		t.Fatal("matching header and origin were rejected")
	}

	form := httptest.NewRequest(http.MethodPost, "/review", strings.NewReader(url.Values{"X-CSRF-Token": {token}}.Encode()))
	form.Host = "learn.example"
	form.Header.Set("Origin", "https://learn.example")
	form.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, form)
	if rec.Code == http.StatusForbidden {
		t.Fatal("matching form field and origin were rejected")
	}

	bad := []struct {
		name   string
		header string
		origin string
		body   string
	}{
		{"wrong token", "nope", "https://learn.example", ""},
		{"missing token", "", "https://learn.example", ""},
		{"foreign origin", token, "https://evil.example", ""},
		{"missing origin", token, "", ""},
	}
	for _, tc := range bad {
		req := httptest.NewRequest(http.MethodPost, "/review", strings.NewReader(tc.body))
		req.Host = "learn.example"
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		if tc.header != "" {
			req.Header.Set("X-CSRF-Token", tc.header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s status = %d", tc.name, rec.Code)
		}
	}

	get := httptest.NewRequest(http.MethodGet, "/review", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, get)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rec.Code)
	}
}
