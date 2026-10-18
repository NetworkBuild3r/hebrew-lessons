package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/app"
)

// configFromEnv reads the process environment into app.Config.
// Unset fields are filled by app.ApplyDefaults.
func configFromEnv() (app.Config, error) {
	cookieSecure, err := parseBool("COOKIE_SECURE", os.Getenv("COOKIE_SECURE"))
	if err != nil {
		return app.Config{}, err
	}
	logLevel, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		return app.Config{}, err
	}
	cfg := app.Config{
		DataDir:           os.Getenv("DATA_DIR"),
		HTTPAddr:          os.Getenv("HTTP_ADDR"),
		MCPAddr:           os.Getenv("MCP_ADDR"),
		PublicOrigin:      strings.TrimSpace(os.Getenv("PUBLIC_ORIGIN")),
		TrustedProxies:    splitList(os.Getenv("TRUSTED_PROXIES")),
		CookieSecure:      cookieSecure,
		LogLevel:          logLevel,
		GoogleAuthURL:     strings.TrimSpace(os.Getenv("GOOGLE_AUTH_URL")),
		GoogleTokenURL:    strings.TrimSpace(os.Getenv("GOOGLE_TOKEN_URL")),
		GoogleUserinfoURL: strings.TrimSpace(os.Getenv("GOOGLE_USERINFO_URL")),
		GoogleJWKSURL:     strings.TrimSpace(os.Getenv("GOOGLE_JWKS_URL")),
		GoogleIssuer:      strings.TrimSpace(os.Getenv("GOOGLE_ISSUER")),
		ElevenLabsAPIKey:  os.Getenv("ELEVENLABS_API_KEY"),
		MCPAuthToken:      os.Getenv("MCP_AUTH_TOKEN"),
	}
	return app.ApplyDefaults(cfg), nil
}

func parseBool(name, raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "0", "false", "no":
		return false, nil
	case "1", "true", "yes":
		return true, nil
	default:
		return false, fmt.Errorf("%s: invalid bool %q", name, raw)
	}
}

func parseLogLevel(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return "", nil
	case "debug", "info", "warn", "error":
		return strings.ToLower(strings.TrimSpace(raw)), nil
	default:
		return "", fmt.Errorf("LOG_LEVEL: invalid %q", raw)
	}
}

func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// warnOptionalConfig logs when sign-in, speech, or MCP OAuth is unset.
// Those settings are not required to boot. /healthz does not use them.
func warnOptionalConfig(cfg app.Config) {
	if os.Getenv("GOOGLE_OAUTH_CLIENT_ID") == "" || os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET") == "" {
		slog.Warn("google sign-in is off", "reason", "GOOGLE_OAUTH_CLIENT_ID or GOOGLE_OAUTH_CLIENT_SECRET is unset")
	}
	if cfg.ElevenLabsAPIKey == "" && cfg.TTS == nil {
		slog.Warn("speech synthesis is off", "reason", "ELEVENLABS_API_KEY is unset")
	}
	if cfg.MCPAuthToken == "" || os.Getenv("MCP_PUBLIC_URL") == "" {
		slog.Warn("mcp oauth is off", "reason", "MCP_AUTH_TOKEN or MCP_PUBLIC_URL is unset")
	}
}

func setLogLevel(level string) {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	slog.SetLogLoggerLevel(slogLevel)
}
