package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
)

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

type fakeTTS struct{}

func (fakeTTS) Synthesize(context.Context, string, string, string) ([]byte, error) {
	return []byte("mp3"), nil
}

func TestHealthzBothHandlers(t *testing.T) {
	dir := t.TempDir()
	// No Google, ElevenLabs, or OAuth fields. New must still boot.
	webH, mcpH, err := New(Config{DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "audio")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "hebrew.sqlite")); err != nil {
		t.Fatal(err)
	}

	for name, h := range map[string]http.Handler{"web": webH, "mcp": mcpH} {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Del("Authorization")
		body := getHealthRequest(t, h, req)
		if !body.OK {
			t.Fatalf("%s health = %+v", name, body)
		}
	}
}

func TestDataDirFallback(t *testing.T) {
	parent := t.TempDir()
	blocked := filepath.Join(parent, "not-a-directory")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := NewApp(Config{DataDir: blocked})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	if a.cfg.DataDir == blocked {
		t.Fatal("startup used the blocked data dir")
	}
	getHealth(t, a.Web())
}

func TestHealthzWhenDBPingFails(t *testing.T) {
	a := newTestApp(t, Config{DataDir: t.TempDir()})
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	for _, h := range []http.Handler{a.Web(), a.MCP()} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
		}
		var body struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.OK {
			t.Fatal("ok = true, want false")
		}
	}
}

func TestConfigSeams(t *testing.T) {
	fake := fakeTTS{}
	when := time.Date(2026, 10, 18, 16, 0, 0, 0, time.UTC)
	a := newTestApp(t, Config{
		DataDir:           t.TempDir(),
		PublicOrigin:      "https://hebrew.example",
		TrustedProxies:    []string{"10.42.0.0/16"},
		CookieSecure:      true,
		GoogleIssuer:      "http://127.0.0.1:9",
		GoogleAuthURL:     "http://127.0.0.1:9/auth",
		GoogleTokenURL:    "http://127.0.0.1:9/token",
		GoogleUserinfoURL: "http://127.0.0.1:9/userinfo",
		GoogleJWKSURL:     "http://127.0.0.1:9/jwks",
		ElevenLabsAPIKey:  "ignored-because-tts-is-set",
		Clock:             fixedClock{when},
		TTS:               fake,
	})
	if a.cfg.GoogleIssuer != "http://127.0.0.1:9" || a.cfg.GoogleJWKSURL != "http://127.0.0.1:9/jwks" {
		t.Fatalf("google endpoints were replaced: %+v", a.cfg)
	}
	if !a.cfg.CookieSecure || len(a.cfg.TrustedProxies) != 1 || a.cfg.PublicOrigin != "https://hebrew.example" {
		t.Fatalf("config not kept: %+v", a.cfg)
	}
	if got := a.TTS(); got != fake {
		t.Fatalf("tts = %#v, want the override", got)
	}
	if !a.store.Clock().Now().Equal(when) {
		t.Fatal("store clock was not the injected clock")
	}
}

func TestTTSFromAPIKey(t *testing.T) {
	withKey := newTestApp(t, Config{DataDir: t.TempDir(), ElevenLabsAPIKey: "test-key"})
	el, ok := withKey.TTS().(*audio.ElevenLabs)
	if !ok || el.APIKey != "test-key" {
		t.Fatalf("tts = %#v, want *audio.ElevenLabs", withKey.TTS())
	}

	unset := newTestApp(t, Config{DataDir: t.TempDir()})
	if unset.TTS() != nil {
		t.Fatalf("tts = %#v, want nil without a key", unset.TTS())
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := ApplyDefaults(Config{})
	if cfg.DataDir != "/data" || cfg.HTTPAddr != ":8080" || cfg.MCPAddr != ":8787" {
		t.Fatalf("addrs: %+v", cfg)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("log level %q", cfg.LogLevel)
	}
	if cfg.GoogleAuthURL != defaultGoogleAuthURL || cfg.GoogleTokenURL != defaultGoogleTokenURL ||
		cfg.GoogleUserinfoURL != defaultGoogleUserinfoURL || cfg.GoogleJWKSURL != defaultGoogleJWKSURL ||
		cfg.GoogleIssuer != defaultGoogleIssuer {
		t.Fatalf("google defaults: %+v", cfg)
	}
	if cfg.Clock == nil {
		t.Fatal("clock default is nil")
	}
}

func TestMCPHealthOpenAndToolsClosed(t *testing.T) {
	a := newTestApp(t, Config{DataDir: t.TempDir(), MCPAuthToken: "test-token"})
	getHealth(t, a.MCP())

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rr := httptest.NewRecorder()
	a.MCP().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Header().Get("WWW-Authenticate"), "Bearer") {
		t.Fatalf("challenge %q", rr.Header().Get("WWW-Authenticate"))
	}
}

func newTestApp(t *testing.T, cfg Config) *App {
	t.Helper()
	a, err := NewApp(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func getHealth(t *testing.T, h http.Handler) struct {
	OK bool `json:"ok"`
} {
	t.Helper()
	return getHealthRequest(t, h, httptest.NewRequest(http.MethodGet, "/healthz", nil))
}

func getHealthRequest(t *testing.T, h http.Handler, req *http.Request) struct {
	OK bool `json:"ok"`
} {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type %q", ct)
	}
	var body struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		b, _ := io.ReadAll(rr.Body)
		t.Fatalf("json: %v (%s)", err, b)
	}
	return body
}
