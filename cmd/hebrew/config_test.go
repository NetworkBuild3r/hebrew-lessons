package main

import "testing"

func TestConfigFromEnv(t *testing.T) {
	for _, key := range []string{
		"DATA_DIR", "HTTP_ADDR", "MCP_ADDR", "PUBLIC_ORIGIN", "COOKIE_SECURE",
		"TRUSTED_PROXIES", "LOG_LEVEL", "GOOGLE_AUTH_URL", "GOOGLE_TOKEN_URL",
		"GOOGLE_USERINFO_URL", "GOOGLE_JWKS_URL", "GOOGLE_ISSUER", "ELEVENLABS_API_KEY",
	} {
		t.Setenv(key, "")
	}
	cfg, err := configFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != "/data" || cfg.HTTPAddr != ":8080" || cfg.MCPAddr != ":8787" {
		t.Fatalf("defaults: %+v", cfg)
	}
	if cfg.CookieSecure {
		t.Fatal("COOKIE_SECURE default is true")
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("log level %q", cfg.LogLevel)
	}
	if cfg.GoogleIssuer != "https://accounts.google.com" || cfg.GoogleJWKSURL != "https://www.googleapis.com/oauth2/v3/certs" {
		t.Fatalf("google defaults: %+v", cfg)
	}
	if cfg.Clock == nil {
		t.Fatal("clock is nil")
	}

	t.Setenv("DATA_DIR", "/tmp/hebrew-data")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("MCP_ADDR", ":9797")
	t.Setenv("PUBLIC_ORIGIN", "https://hebrew.example")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("TRUSTED_PROXIES", "10.42.0.0/16, 127.0.0.1")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("GOOGLE_ISSUER", "http://127.0.0.1:9")
	t.Setenv("GOOGLE_JWKS_URL", "http://127.0.0.1:9/jwks")
	t.Setenv("ELEVENLABS_API_KEY", "secret")
	cfg, err = configFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != "/tmp/hebrew-data" || cfg.HTTPAddr != ":9090" || cfg.MCPAddr != ":9797" {
		t.Fatalf("overrides: %+v", cfg)
	}
	if !cfg.CookieSecure || cfg.LogLevel != "debug" || cfg.ElevenLabsAPIKey != "secret" {
		t.Fatalf("flags: %+v", cfg)
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
	if cfg.GoogleAuthURL != "https://accounts.google.com/o/oauth2/v2/auth" {
		t.Fatalf("unset google url was not defaulted: %s", cfg.GoogleAuthURL)
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
