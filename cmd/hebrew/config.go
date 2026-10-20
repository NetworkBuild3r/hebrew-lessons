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
	adminToken := strings.TrimSpace(os.Getenv("MCP_AUTH_TOKEN"))
	readOnlyToken, readOnlyLearner, err := app.ConfigureMCPReadOnly(adminToken, os.Getenv("MCP_READONLY_TOKEN"), os.Getenv("MCP_READONLY_LEARNER_ID"))
	if err != nil {
		return app.Config{}, err
	}
	cfg := app.Config{
		DataDir:              dataDir,
		HTTPAddr:             os.Getenv("HTTP_ADDR"),
		MCPAddr:              os.Getenv("MCP_ADDR"),
		PublicOrigin:         strings.TrimSpace(os.Getenv("PUBLIC_ORIGIN")),
		TrustedProxies:       splitList(os.Getenv("TRUSTED_PROXIES")),
		CookieSecure:         cookieSecure,
		Dev:                  dev,
		LogLevel:             logLevel,
		GoogleClientID:       firstSet("GOOGLE_CLIENT_ID", "GOOGLE_OAUTH_CLIENT_ID", "web_client_id", "client_id"),
		GoogleClientSecret:   firstSet("GOOGLE_CLIENT_SECRET", "GOOGLE_OAUTH_CLIENT_SECRET", "web_client_secret", "client_secret"),
		GoogleAuthURL:        firstSet("GOOGLE_AUTH_URL", "GOOGLE_OAUTH_AUTH_URL"),
		GoogleTokenURL:       firstSet("GOOGLE_TOKEN_URL", "GOOGLE_OAUTH_TOKEN_URL"),
		GoogleUserinfoURL:    firstSet("GOOGLE_USERINFO_URL", "GOOGLE_OAUTH_USERINFO_URL"),
		GoogleJWKSURL:        strings.TrimSpace(os.Getenv("GOOGLE_JWKS_URL")),
		GoogleIssuer:         strings.TrimSpace(os.Getenv("GOOGLE_ISSUER")),
		StateSecret:          strings.TrimSpace(os.Getenv("HEBREW_SESSION_SECRET")),
		OAuthRedirectURIs:    splitList(os.Getenv("MCP_OAUTH_REDIRECT_URIS")),
		AllowedEmails:        splitList(os.Getenv("MCP_OAUTH_ALLOWED_EMAILS")),
		AdminEmails:          splitList(os.Getenv("ADMIN_EMAILS")),
		MCPAuthToken:         adminToken,
		MCPReadOnlyToken:     readOnlyToken,
		MCPReadOnlyLearnerID: readOnlyLearner,
		MCPPublicURL:         strings.TrimSpace(os.Getenv("MCP_PUBLIC_URL")),
		MCPAllowedOrigins:    splitList(os.Getenv("MCP_ALLOWED_ORIGINS")),
		ElevenLabsAPIKey:     os.Getenv("ELEVENLABS_API_KEY"),
		ElevenLabsVoice:      strings.TrimSpace(os.Getenv("ELEVENLABS_VOICE_ID")),
		// The male voice for speaker b of a dialogue. No default: an unset value
		// means no male voice, and a speaker b clip fails rather than use the other voice.
		ElevenLabsVoiceMale: strings.TrimSpace(os.Getenv("ELEVENLABS_VOICE_ID_MALE")),
		ElevenLabsModel:     strings.TrimSpace(os.Getenv("ELEVENLABS_MODEL")),
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

// firstSet returns the first non-empty trimmed environment value.
// GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET stay ahead of the old
// app's GOOGLE_OAUTH_* and web_client_*/client_* names.
func firstSet(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
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
	if cfg.MCPAuthToken == "" {
		slog.Warn("mcp static token is disabled", "reason", "MCP_AUTH_TOKEN is empty")
	}
	if cfg.MCPPublicURL == "" {
		slog.Warn("mcp oauth issuer uses the forwarded host", "reason", "MCP_PUBLIC_URL is unset")
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
