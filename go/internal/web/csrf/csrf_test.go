package csrf

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIssueAndCheck(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	tok := Issue(rec, req)
	if len(tok) != 64 {
		t.Fatalf("token %q", tok)
	}
	post := httptest.NewRequest(http.MethodPost, "http://example.com/_hx/units/unit-3/gate-ack", nil)
	post.Header.Set("Origin", "http://example.com")
	post.Header.Set("X-CSRF-Token", tok)
	post.AddCookie(rec.Result().Cookies()[0])
	if err := Check(post); err != nil {
		t.Fatal(err)
	}
}

func TestCheckRejects(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	tok := Issue(rec, req)
	cookie := rec.Result().Cookies()[0]
	cases := []struct {
		name   string
		origin string
		token  string
		refer  string
	}{
		{"no origin", "", tok, ""},
		{"wrong origin", "http://evil.example", tok, ""},
		{"wrong token", "http://example.com", tok + "aa", ""},
		{"missing token", "http://example.com", "", ""},
	}
	for _, tc := range cases {
		post := httptest.NewRequest(http.MethodPost, "http://example.com/_hx/units/unit-3/gate-ack", nil)
		if tc.origin != "" {
			post.Header.Set("Origin", tc.origin)
		}
		if tc.refer != "" {
			post.Header.Set("Referer", tc.refer)
		}
		if tc.token != "" {
			post.Header.Set("X-CSRF-Token", tc.token)
		}
		post.AddCookie(cookie)
		if err := Check(post); err == nil {
			t.Fatalf("%s was allowed", tc.name)
		}
	}
	get := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	get.Header.Set("Origin", "http://example.com")
	get.Header.Set("X-CSRF-Token", tok)
	get.AddCookie(cookie)
	if err := Check(get); err == nil {
		t.Fatal("GET was treated as a write")
	}
}

func TestRefererFallback(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	tok := Issue(rec, req)
	body := strings.NewReader("csrf=" + tok)
	post := httptest.NewRequest(http.MethodPost, "http://example.com/_hx/units/unit-3/gate-ack", body)
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Set("Referer", "http://example.com/lessons/")
	post.AddCookie(rec.Result().Cookies()[0])
	if err := Check(post); err != nil {
		t.Fatal(err)
	}
}
