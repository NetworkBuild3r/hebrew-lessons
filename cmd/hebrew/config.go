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
	cookieSecure, err := cookieSecureFromEnv(os.Getenv("COOKIE_SECURE"))
	if err != nil {
		return app.Config{}, err
	}
	logLevel, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		return app.Config{}, err
	}
	dev, err := parseDev(os.Getenv("DEV"))
	if err != nil {
		return app.Config{}, err
	}
	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dataDir == "" && !dev {
		return app.Config{}, fmt.Errorf("DATA_DIR is unset")
	}
	cfg := app.Config{
		DataDir:            dataDir,
		HTTPAddr:           os.Getenv("HTTP_ADDR"),
		MCPAddr:            os.Getenv("MCP_ADDR"),
		PublicOrigin:       strings.TrimSpace(os.Getenv("PUBLIC_ORIGIN")),
		TrustedProxies:     splitList(os.Getenv("TRUSTED_PROXIES")),
		CookieSecure:       cookieSecure,
		Dev:                dev,
		LogLevel:           logLevel,
		GoogleClientID:     strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleAuthURL:      strings.TrimSpace(os.Getenv("GOOGLE_AUTH_URL")),
		GoogleTokenURL:     strings.TrimSpace(os.Getenv("GOOGLE_TOKEN_URL")),
		GoogleUserinfoURL:  strings.TrimSpace(os.Getenv("GOOGLE_USERINFO_URL")),
		GoogleJWKSURL:      strings.TrimSpace(os.Getenv("GOOGLE_JWKS_URL")),
		GoogleIssuer:       strings.TrimSpace(os.Getenv("GOOGLE_ISSUER")),
		StateSecret:        strings.TrimSpace(os.Getenv("HEBREW_SESSION_SECRET")),
		AllowedEmails:      splitList(os.Getenv("MCP_OAUTH_ALLOWED_EMAILS")),
		AdminEmails:        splitList(os.Getenv("ADMIN_EMAILS")),
		MCPAuthToken:       os.Getenv("MCP_AUTH_TOKEN"),
		MCPPublicURL:       strings.TrimSpace(os.Getenv("MCP_PUBLIC_URL")),
		ElevenLabsAPIKey:   os.Getenv("ELEVENLABS_API_KEY"),
	}
	return app.ApplyDefaults(cfg), nil
}

// cookieSecureFromEnv defaults an unset COOKIE_SECURE to true.
// Only an explicit false, 0, or no turns it off.
func cookieSecureFromEnv(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return true, nil
	case "0", "false", "no":
		return false, nil
	case "1", "true", "yes":
		return true, nil
	default:
		return false, fmt.Errorf("COOKIE_SECURE: invalid bool %q", raw)
	}
}

func parseDev(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "0", "false", "no":
		return false, nil
	case "1", "true", "yes":
		return true, nil
	default:
		return false, fmt.Errorf("DEV: invalid bool %q", raw)
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
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
		slog.Warn("google sign-in is off", "reason", "GOOGLE_CLIENT_ID or GOOGLE_CLIENT_SECRET is unset")
	}
	if cfg.ElevenLabsAPIKey == "" && cfg.TTS == nil {
		slog.Warn("speech synthesis is off", "reason", "ELEVENLABS_API_KEY is unset")
	}
	if cfg.MCPAuthToken == "" || cfg.MCPPublicURL == "" {
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
