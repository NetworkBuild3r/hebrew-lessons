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
		"GOOGLE_OAUTH_CLIENT_ID", "GOOGLE_OAUTH_CLIENT_SECRET",
		"web_client_id", "client_id", "web_client_secret", "client_secret",
		"GOOGLE_AUTH_URL", "GOOGLE_TOKEN_URL",
		"GOOGLE_OAUTH_AUTH_URL", "GOOGLE_OAUTH_TOKEN_URL",
		"GOOGLE_USERINFO_URL", "GOOGLE_OAUTH_USERINFO_URL",
		"GOOGLE_JWKS_URL", "GOOGLE_ISSUER",
		"MCP_OAUTH_REDIRECT_URIS",
		"HEBREW_SESSION_SECRET",
		"MCP_OAUTH_ALLOWED_EMAILS", "ADMIN_EMAILS",
		"MCP_AUTH_TOKEN", "MCP_PUBLIC_URL", "MCP_ALLOWED_ORIGINS",
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
	t.Setenv("MCP_AUTH_TOKEN", " mcp-token\n")
	t.Setenv("MCP_PUBLIC_URL", "https://hebrew.example/mcp")
	t.Setenv("MCP_ALLOWED_ORIGINS", "https://hebrew.example, https://builder.example")
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
	if len(cfg.MCPAllowedOrigins) != 2 || cfg.MCPAllowedOrigins[0] != "https://hebrew.example" || cfg.MCPAllowedOrigins[1] != "https://builder.example" {
		t.Fatalf("origins: %+v", cfg.MCPAllowedOrigins)
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

func TestGoogleEnvAliases(t *testing.T) {
	t.Setenv("DATA_DIR", "/data")
	for _, key := range []string{
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET",
		"GOOGLE_OAUTH_CLIENT_ID", "GOOGLE_OAUTH_CLIENT_SECRET",
		"web_client_id", "client_id", "web_client_secret", "client_secret",
		"GOOGLE_AUTH_URL", "GOOGLE_TOKEN_URL", "GOOGLE_OAUTH_AUTH_URL", "GOOGLE_OAUTH_TOKEN_URL",
		"GOOGLE_USERINFO_URL", "GOOGLE_OAUTH_USERINFO_URL",
		"MCP_OAUTH_REDIRECT_URIS",
	} {
		t.Setenv(key, "")
	}

	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "oauth-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "oauth-secret")
	t.Setenv("GOOGLE_OAUTH_AUTH_URL", "https://alias.example/auth")
	t.Setenv("GOOGLE_OAUTH_TOKEN_URL", "https://alias.example/token")
	t.Setenv("GOOGLE_OAUTH_USERINFO_URL", "https://alias.example/userinfo")
	cfg, err := configFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GoogleClientID != "oauth-id" || cfg.GoogleClientSecret != "oauth-secret" {
		t.Fatalf("oauth alias %+v", cfg)
	}
	if cfg.GoogleAuthURL != "https://alias.example/auth" || cfg.GoogleTokenURL != "https://alias.example/token" || cfg.GoogleUserinfoURL != "https://alias.example/userinfo" {
		t.Fatalf("url alias %+v", cfg)
	}

	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")
	t.Setenv("web_client_id", "web-id")
	t.Setenv("web_client_secret", "web-secret")
	cfg, err = configFromEnv()
	if err != nil || cfg.GoogleClientID != "web-id" || cfg.GoogleClientSecret != "web-secret" {
		t.Fatalf("web alias %+v %v", cfg, err)
	}

	t.Setenv("web_client_id", "")
	t.Setenv("web_client_secret", "")
	t.Setenv("client_id", "plain-id")
	t.Setenv("client_secret", "plain-secret")
	cfg, err = configFromEnv()
	if err != nil || cfg.GoogleClientID != "plain-id" || cfg.GoogleClientSecret != "plain-secret" {
		t.Fatalf("client alias %+v %v", cfg, err)
	}

	t.Setenv("GOOGLE_CLIENT_ID", "primary-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "primary-secret")
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "oauth-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "oauth-secret")
	t.Setenv("GOOGLE_AUTH_URL", "https://primary.example/auth")
	t.Setenv("GOOGLE_OAUTH_AUTH_URL", "https://alias.example/auth")
	t.Setenv("web_client_id", "web-id")
	t.Setenv("client_id", "plain-id")
	cfg, err = configFromEnv()
	if err != nil || cfg.GoogleClientID != "primary-id" || cfg.GoogleClientSecret != "primary-secret" || cfg.GoogleAuthURL != "https://primary.example/auth" {
		t.Fatalf("GOOGLE_CLIENT_* wins over GOOGLE_OAUTH_* %+v %v", cfg, err)
	}

	t.Setenv("GOOGLE_CLIENT_ID", "  ")
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "fallback-id")
	cfg, err = configFromEnv()
	if err != nil || cfg.GoogleClientID != "fallback-id" {
		t.Fatalf("blank primary %+v %v", cfg, err)
	}

	t.Setenv("MCP_OAUTH_REDIRECT_URIS", "https://client.example/cb, cursor://anysphere.cursor-mcp/oauth/callback")
	cfg, err = configFromEnv()
	if err != nil || len(cfg.OAuthRedirectURIs) != 2 || cfg.OAuthRedirectURIs[0] != "https://client.example/cb" {
		t.Fatalf("redirects %+v %v", cfg.OAuthRedirectURIs, err)
	}
}

func TestMCPAuthTokenBlankDisablesStaticToken(t *testing.T) {
	t.Setenv("DATA_DIR", "/data")
	t.Setenv("MCP_AUTH_TOKEN", " \n\t ")
	cfg, err := configFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPAuthToken != "" {
		t.Fatalf("token %q", cfg.MCPAuthToken)
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
