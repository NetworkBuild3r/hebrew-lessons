// Package app composes the process: configuration, the database, and the
// two HTTP handlers.
//
// New is the constructor tests and embeds call. It returns the learner and
// MCP handlers. NewApp is the same startup and also returns *App so the
// caller can Close the database. New wraps NewApp. cmd/hebrew uses NewApp
// because it has to release the database on shutdown. Handlers returned by
// New stay open for the process lifetime.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web"
)

const (
	defaultDataDir  = "/data"
	defaultHTTPAddr = ":8080"
	defaultMCPAddr  = ":8787"
	defaultLogLevel = "info"

	defaultGoogleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultGoogleTokenURL    = "https://oauth2.googleapis.com/token"
	defaultGoogleUserinfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
	defaultGoogleJWKSURL     = "https://www.googleapis.com/oauth2/v3/certs"
	defaultGoogleIssuer      = "https://accounts.google.com"

	// Race-instrumented sqlite on a busy CI runner can spend most of a
	// shorter window on the first schema statement.
	openTimeout   = 60 * time.Second
	healthTimeout = 2 * time.Second
)

// Config is the process configuration. Zero values are filled by ApplyDefaults.
// Tests set DataDir to a temp directory and may replace Clock and TTS.
type Config struct {
	// DataDir holds hebrew.sqlite. Audio files live in DataDir/audio.
	DataDir string
	// HTTPAddr is the learner listener, default :8080.
	HTTPAddr string
	// MCPAddr is the authoring listener, default :8787.
	MCPAddr string
	// PublicOrigin is the external origin, used later for redirects and cookies.
	PublicOrigin string
	// TrustedProxies are the CIDRs allowed to supply a forwarded client IP.
	TrustedProxies []string
	// CookieSecure sets the Secure flag on session cookies.
	// cmd/hebrew defaults an unset COOKIE_SECURE to true.
	CookieSecure bool
	// Dev is DEV=1. It allows a missing DATA_DIR to be created and silences
	// the warning when COOKIE_SECURE is off. It does not choose another directory.
	Dev bool
	// LogLevel is debug, info, warn, or error.
	LogLevel string

	// Google sign-in. Empty client id or secret leaves the routes at 503.
	GoogleClientID     string
	GoogleClientSecret string
	// Google endpoints. Defaults point at Google. Tests point them at a fake server.
	GoogleAuthURL  string
	GoogleTokenURL string
	// GoogleUserinfoURL is accepted from the environment so a deployment
	// can name the endpoint. Sign-in reads email from the ID token and
	// does not call userinfo.
	GoogleUserinfoURL string
	GoogleJWKSURL     string
	GoogleIssuer      string
	// StateSecret is HEBREW_SESSION_SECRET. It derives the OAuth state HMAC
	// key (HKDF-SHA256). Empty falls back to a random per-process key.
	StateSecret string

	// AllowedEmails is MCP_OAUTH_ALLOWED_EMAILS. AdminEmails is ADMIN_EMAILS.
	// They seed auth.Access when Access is nil. A non-nil Access is the
	// live list: sign-in and every later request read it, and Set changes
	// what the next request allows.
	AllowedEmails []string
	AdminEmails   []string
	Access        *auth.Access

	// MCPAuthToken and MCPPublicURL are recorded for the MCP builder.
	// Empty values do not stop startup.
	MCPAuthToken string
	MCPPublicURL string

	// ElevenLabsAPIKey gates the real TTS client. Empty leaves TTS unset
	// unless TTS is set directly.
	ElevenLabsAPIKey string

	// Clock stamps database times. Nil uses the system clock.
	Clock clock.Clock
	// TTS overrides the synthesizer. Nil selects ElevenLabs when
	// ElevenLabsAPIKey is set, and selects nothing when it is empty.
	TTS audio.TTS
}

// ApplyDefaults fills zero fields. It does not replace a set field.
func ApplyDefaults(cfg Config) Config {
	if cfg.DataDir == "" {
		if cfg.Dev {
			cfg.DataDir = "data"
		} else {
			cfg.DataDir = defaultDataDir
		}
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = defaultHTTPAddr
	}
	if cfg.MCPAddr == "" {
		cfg.MCPAddr = defaultMCPAddr
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = defaultLogLevel
	}
	if cfg.GoogleAuthURL == "" {
		cfg.GoogleAuthURL = defaultGoogleAuthURL
	}
	if cfg.GoogleTokenURL == "" {
		cfg.GoogleTokenURL = defaultGoogleTokenURL
	}
	if cfg.GoogleUserinfoURL == "" {
		cfg.GoogleUserinfoURL = defaultGoogleUserinfoURL
	}
	if cfg.GoogleJWKSURL == "" {
		cfg.GoogleJWKSURL = defaultGoogleJWKSURL
	}
	if cfg.GoogleIssuer == "" {
		cfg.GoogleIssuer = defaultGoogleIssuer
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.System()
	}
	return cfg
}

// App is the running process core.
type App struct {
	cfg       Config
	store     *db.Store
	authSvc   *auth.Service
	review    *fsrs.Review
	audioDir  string
	tts       audio.TTS
	web       http.Handler
	mcp       http.Handler
	closeOnce sync.Once
	closeErr  error
}

// New opens the database, applies migrations, and returns the two handlers.
// GET /healthz on either handler is unauthenticated. Missing Google,
// ElevenLabs, and OAuth settings do not fail New; the process logs those
// separately. Cleanup is NewApp's Close. New wraps NewApp.
func New(cfg Config) (web, mcp http.Handler, err error) {
	a, err := NewApp(cfg)
	if err != nil {
		return nil, nil, err
	}
	return a.Web(), a.MCP(), nil
}

// NewApp opens the database, applies migrations, and builds both handlers.
// Close releases the database. /healthz answers 503 after Close returns.
func NewApp(cfg Config) (*App, error) {
	cfg = ApplyDefaults(cfg)
	if auth.HTTPSOriginWithoutProxies(cfg.TrustedProxies, cfg.PublicOrigin) {
		slog.Warn("TRUSTED_PROXIES is empty and PUBLIC_ORIGIN is https",
			"public_origin", cfg.PublicOrigin,
			"hint", "set TRUSTED_PROXIES to the proxy CIDR")
	}
	if !cfg.CookieSecure && !cfg.Dev {
		slog.Warn("COOKIE_SECURE is off",
			"hint", "set COOKIE_SECURE=true, or DEV=1 to silence this outside production")
	}
	dataDir, err := prepareDataDir(cfg.DataDir, cfg.Dev)
	if err != nil {
		return nil, err
	}
	cfg.DataDir = dataDir
	audioDir := filepath.Join(cfg.DataDir, "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), openTimeout)
	defer cancel()
	store, err := db.Open(ctx, filepath.Join(cfg.DataDir, "hebrew.sqlite"), cfg.Clock)
	if err != nil {
		return nil, err
	}

	svc, err := auth.New(auth.Config{
		PublicOrigin:       cfg.PublicOrigin,
		CookieSecure:       cfg.CookieSecure,
		TrustedProxies:     cfg.TrustedProxies,
		GoogleClientID:     cfg.GoogleClientID,
		GoogleClientSecret: cfg.GoogleClientSecret,
		GoogleAuthURL:      cfg.GoogleAuthURL,
		GoogleTokenURL:     cfg.GoogleTokenURL,
		GoogleJWKSURL:      cfg.GoogleJWKSURL,
		GoogleIssuer:       cfg.GoogleIssuer,
		StateSecret:        cfg.StateSecret,
		AllowedEmails:      cfg.AllowedEmails,
		AdminEmails:        cfg.AdminEmails,
		Access:             cfg.Access,
		Clock:              cfg.Clock,
		Store:              store,
	})
	if err != nil {
		store.Close()
		return nil, err
	}

	tts := cfg.TTS
	if tts == nil && cfg.ElevenLabsAPIKey != "" {
		tts = audio.NewElevenLabs(cfg.ElevenLabsAPIKey)
	}

	a := &App{
		cfg:      cfg,
		store:    store,
		authSvc:  svc,
		review:   fsrs.New(store, cfg.Clock),
		audioDir: audioDir,
		tts:      tts,
	}
	health := a.healthHandler()
	a.web = web.New(health, svc)
	a.mcp = newMCP(health)
	return a, nil
}

// Web is the learner handler.
func (a *App) Web() http.Handler { return a.web }

// MCP is the authoring handler. This skeleton serves /healthz only.
func (a *App) MCP() http.Handler { return a.mcp }

// Store is the process database. QA and MCP Builder use it for users and sessions.
func (a *App) Store() *db.Store { return a.store }

// AudioDir is <DataDir>/audio, where clips are written as <sha256>.mp3.
func (a *App) AudioDir() string { return a.audioDir }

// TTS is the resolved synthesizer. It is nil when no override was set and
// ELEVENLABS_API_KEY was empty.
func (a *App) TTS() audio.TTS { return a.tts }

// Review is the spaced-repetition service.
// It uses the clock from Config, the same clock the store was opened with.
func (a *App) Review() *fsrs.Review { return a.review }

// Close releases the database. /healthz answers 503 after Close returns.
func (a *App) Close() error {
	a.closeOnce.Do(func() {
		if a.store != nil {
			a.closeErr = a.store.Close()
		}
	})
	return a.closeErr
}

// prepareDataDir refuses an unset, missing, or unwritable directory.
// DEV=1 creates a missing directory and, when dir is empty, uses "data".
// It does not substitute another path when the chosen directory cannot be written.
func prepareDataDir(dir string, dev bool) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		if !dev {
			return "", fmt.Errorf("DATA_DIR is unset")
		}
		dir = "data"
	}
	info, err := os.Stat(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("DATA_DIR %s: %w", dir, err)
		}
		if !dev {
			return "", fmt.Errorf("DATA_DIR %s does not exist", dir)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("DATA_DIR %s: %w", dir, err)
		}
	} else if !info.IsDir() {
		return "", fmt.Errorf("DATA_DIR %s is not a directory", dir)
	}
	if err := writableDir(dir); err != nil {
		return "", fmt.Errorf("DATA_DIR %s is not writable: %w", dir, err)
	}
	return dir, nil
}

func writableDir(dir string) error {
	f, err := os.CreateTemp(dir, ".write-check-*")
	if err != nil {
		return err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Remove(name)
}

func (a *App) healthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No session, cookie, or bearer check. Probes call this with an empty request.
		ctx, cancel := context.WithTimeout(r.Context(), healthTimeout)
		defer cancel()
		err := a.store.Ping(ctx)
		status := http.StatusOK
		if err != nil {
			status = http.StatusServiceUnavailable
			slog.Info("healthz db ping failed", "err", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(struct {
			OK bool `json:"ok"`
		}{OK: err == nil})
	})
}

func newMCP(health http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", health)
	return mux
}
