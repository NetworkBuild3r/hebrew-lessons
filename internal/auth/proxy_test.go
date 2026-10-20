package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTrustedProxies(t *testing.T) {
	if !HTTPSOriginWithoutProxies(nil, "https://hebrew.example") {
		t.Fatal("expected https warning")
	}
	if HTTPSOriginWithoutProxies([]string{"10.0.0.0/8"}, "https://hebrew.example") {
		t.Fatal("proxies set")
	}
	if HTTPSOriginWithoutProxies(nil, "http://hebrew.example") {
		t.Fatal("http origin")
	}

	clk := &manualClock{t: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	store := openTestStore(t, clk)
	svc, err := New(Config{
		PublicOrigin:   "https://hebrew.example",
		TrustedProxies: []string{"10.0.0.0/8", "not-a-cidr", "192.0.2.1"},
		Clock:          clk,
		Store:          store,
	})
	if err != nil {
		t.Fatal(err)
	}

	var gotIP, gotProto string
	h := svc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = ClientIP(r)
		gotProto = ForwardedProto(r)
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:443"
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.9.9.9")
	req.Header.Set("X-Forwarded-Proto", "http, https")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if gotIP != "203.0.113.9" || gotProto != "https" {
		t.Fatalf("trusted ip %q proto %q", gotIP, gotProto)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.4:443"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("X-Forwarded-Proto", "https")
	h.ServeHTTP(rr, req)
	if gotIP != "198.51.100.4" || gotProto != "http" {
		t.Fatalf("untrusted ip %q proto %q", gotIP, gotProto)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[::ffff:10.0.0.5]:443"
	req.Header.Set("X-Forwarded-For", "198.51.100.8, 10.0.0.2")
	h.ServeHTTP(rr, req)
	if gotIP != "198.51.100.8" {
		t.Fatalf("mapped peer ip %q", gotIP)
	}
}

func TestClientIPWithoutMiddleware(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.4:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.1")
	req = req.WithContext(context.Background())
	if ClientIP(req) != "203.0.113.4" {
		t.Fatalf("ip %q", ClientIP(req))
	}
}
