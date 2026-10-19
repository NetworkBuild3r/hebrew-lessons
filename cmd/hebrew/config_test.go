package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/app"
)

func TestConfigFromEnv(t *testing.T) {
	for _, key := range []string{
		"DATA_DIR", "HTTP_ADDR", "MCP_ADDR", "PUBLIC_ORIGIN", "COOKIE_SECURE",
		"TRUSTED_PROXIES", "LOG_LEVEL", "DEV",
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET",
		"GOOGLE_AUTH_URL", "GOOGLE_TOKEN_URL",
		"GOOGLE_USERINFO_URL", "GOOGLE_JWKS_URL", "GOOGLE_ISSUER",
		"HEBREW_SESSION_SECRET",
		"MCP_OAUTH_ALLOWED_EMAILS", "ADMIN_EMAILS",
		"MCP_AUTH_TOKEN", "MCP_PUBLIC_URL",
		"ELEVENLABS_API_KEY",
	} {
		t.Setenv(key, "")
	}
	if _, err := configFromEnv(); err == nil {
		t.Fatal("expected DATA_DIR error")
	}
	t.Setenv("DATA_DIR", "/data")
	cfg, err := configFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != "/data" || cfg.HTTPAddr != ":8080" || cfg.MCPAddr != ":8787" {
		t.Fatalf("defaults: %+v", cfg)
	}
	if !cfg.CookieSecure {
		t.Fatal("COOKIE_SECURE default is false")
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("log level %q", cfg.LogLevel)
	}
	if cfg.GoogleIssuer != "https://accounts.google.com" || cfg.GoogleJWKSURL != "https://www.googleapis.com/oauth2/v3/certs" {
		t.Fatalf("google defaults: %+v", cfg)
	}
	if cfg.MCPAuthToken != "" {
		t.Fatalf("MCP auth token %q", cfg.MCPAuthToken)
	}
	if cfg.Clock == nil {
		t.Fatal("clock is nil")
	}

	t.Setenv("DATA_DIR", "/tmp/hebrew-data")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("MCP_ADDR", ":9797")
	t.Setenv("PUBLIC_ORIGIN", "https://hebrew.example")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("DEV", "1")
	t.Setenv("TRUSTED_PROXIES", "10.42.0.0/16, 127.0.0.1")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("GOOGLE_CLIENT_ID", "client")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret-value")
	t.Setenv("GOOGLE_ISSUER", "http://127.0.0.1:9")
	t.Setenv("GOOGLE_JWKS_URL", "http://127.0.0.1:9/jwks")
	t.Setenv("HEBREW_SESSION_SECRET", "state-secret")
	t.Setenv("MCP_OAUTH_ALLOWED_EMAILS", "learner@example.com, admin@example.com")
	t.Setenv("ADMIN_EMAILS", "admin@example.com")
	t.Setenv("MCP_AUTH_TOKEN", "mcp-token")
	t.Setenv("MCP_PUBLIC_URL", "https://hebrew.example/mcp")
	t.Setenv("ELEVENLABS_API_KEY", "secret")
	cfg, err = configFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != "/tmp/hebrew-data" || cfg.HTTPAddr != ":9090" || cfg.MCPAddr != ":9797" {
		t.Fatalf("overrides: %+v", cfg)
	}
	if cfg.CookieSecure || !cfg.Dev || cfg.LogLevel != "debug" || cfg.ElevenLabsAPIKey != "secret" {
		t.Fatalf("flags: %+v", cfg)
	}
	if cfg.GoogleClientID != "client" || cfg.GoogleClientSecret != "secret-value" {
		t.Fatalf("google client: %+v", cfg)
	}
	if cfg.MCPAuthToken != "mcp-token" || cfg.MCPPublicURL != "https://hebrew.example/mcp" {
		t.Fatalf("mcp: %+v", cfg)
	}
	if len(cfg.AllowedEmails) != 2 || cfg.AllowedEmails[0] != "learner@example.com" || len(cfg.AdminEmails) != 1 {
		t.Fatalf("emails: %+v", cfg)
	}
	if cfg.PublicOrigin != "https://hebrew.example" {
		t.Fatalf("origin %q", cfg.PublicOrigin)
	}
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0] != "10.42.0.0/16" || cfg.TrustedProxies[1] != "127.0.0.1" {
		t.Fatalf("proxies %#v", cfg.TrustedProxies)
	}
	if cfg.GoogleIssuer != "http://127.0.0.1:9" || cfg.GoogleJWKSURL != "http://127.0.0.1:9/jwks" {
		t.Fatalf("google override lost: %+v", cfg)
	}
	if cfg.StateSecret != "state-secret" {
		t.Fatalf("state secret %q", cfg.StateSecret)
	}
	if cfg.GoogleAuthURL != "https://accounts.google.com/o/oauth2/v2/auth" {
		t.Fatalf("unset google url was not defaulted: %s", cfg.GoogleAuthURL)
	}
}

func TestDevAllowsUnsetDataDir(t *testing.T) {
	t.Setenv("DEV", "1")
	t.Setenv("DATA_DIR", "")
	t.Setenv("COOKIE_SECURE", "")
	cfg, err := configFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != "data" || !cfg.Dev || !cfg.CookieSecure {
		t.Fatalf("%+v", cfg)
	}
}

func TestWarnOptionalConfigUsesStruct(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "from-env")
	t.Setenv("GOOGLE_CLIENT_SECRET", "from-env")
	t.Setenv("MCP_AUTH_TOKEN", "from-env")
	t.Setenv("MCP_PUBLIC_URL", "https://from-env.example")
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	warnOptionalConfig(app.Config{})
	log := buf.String()
	if !strings.Contains(log, "GOOGLE_CLIENT_ID") || !strings.Contains(log, "MCP_AUTH_TOKEN") {
		t.Fatalf("warnings: %s", log)
	}
	buf.Reset()
	warnOptionalConfig(app.Config{
		GoogleClientID:     "id",
		GoogleClientSecret: "secret",
		MCPAuthToken:       "token",
		MCPPublicURL:       "https://hebrew.example/mcp",
		ElevenLabsAPIKey:   "key",
	})
	if buf.Len() != 0 {
		t.Fatalf("configured settings still warned: %s", buf.String())
	}
}

func TestConfigRejectsBadEnv(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "maybe")
	if _, err := configFromEnv(); err == nil {
		t.Fatal("expected COOKIE_SECURE error")
	}
	t.Setenv("COOKIE_SECURE", "1")
	t.Setenv("LOG_LEVEL", "loud")
	if _, err := configFromEnv(); err == nil {
		t.Fatal("expected LOG_LEVEL error")
	}
}
