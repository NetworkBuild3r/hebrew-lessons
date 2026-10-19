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
	"sync"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
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

	openTimeout   = 15 * time.Second
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
	CookieSecure bool
	// LogLevel is debug, info, warn, or error.
	LogLevel string

	// Google endpoints. Defaults point at Google. Tests point them at a fake server.
	GoogleAuthURL     string
	GoogleTokenURL    string
	GoogleUserinfoURL string
	GoogleJWKSURL     string
	GoogleIssuer      string

	// ElevenLabsAPIKey gates the real TTS client. Empty leaves TTS unset
	// unless TTS is set directly.
	ElevenLabsAPIKey string

	// MCPAuthToken is the bearer secret for the authoring tools.
	// cmd/hebrew copies MCP_AUTH_TOKEN. Empty rejects every MCP request.
	// internal/mcp does not read the environment. GET /healthz stays open.
	MCPAuthToken string

	// Clock stamps database times. Nil uses the system clock.
	Clock clock.Clock
	// TTS overrides the synthesizer. Nil selects ElevenLabs when
	// ElevenLabsAPIKey is set, and selects nothing when it is empty.
	TTS audio.TTS
}

// ApplyDefaults fills zero fields. It does not replace a set field.
func ApplyDefaults(cfg Config) Config {
	if cfg.DataDir == "" {
		cfg.DataDir = defaultDataDir
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
	dataDir, err := prepareDataDir(cfg.DataDir)
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

	tts := cfg.TTS
	if tts == nil && cfg.ElevenLabsAPIKey != "" {
		tts = audio.NewElevenLabs(cfg.ElevenLabsAPIKey)
	}

	a := &App{
		cfg:      cfg,
		store:    store,
		audioDir: audioDir,
		tts:      tts,
	}
	health := a.healthHandler()
	a.web = web.New(health)
	a.mcp = newMCP(cfg, health)
	return a, nil
}

// Web is the learner handler.
func (a *App) Web() http.Handler { return a.web }

// MCP is the authoring handler. GET /healthz is unauthenticated.
// Every other route is the MCP server.
func (a *App) MCP() http.Handler { return a.mcp }

// AudioDir is <DataDir>/audio, where clips are written as <sha256>.mp3.
func (a *App) AudioDir() string { return a.audioDir }

// TTS is the resolved synthesizer. It is nil when no override was set and
// ELEVENLABS_API_KEY was empty.
func (a *App) TTS() audio.TTS { return a.tts }

// Close releases the database. /healthz answers 503 after Close returns.
func (a *App) Close() error {
	a.closeOnce.Do(func() {
		if a.store != nil {
			a.closeErr = a.store.Close()
		}
	})
	return a.closeErr
}

// prepareDataDir uses dir when this process can create a file there.
// Otherwise it logs a warning and uses a directory under the system temp
// path, so a container with no volume still starts.
func prepareDataDir(dir string) (string, error) {
	err := usableDir(dir)
	if err == nil {
		return dir, nil
	}
	fallback := fallbackDataDir()
	if fallbackErr := usableDir(fallback); fallbackErr != nil {
		return "", fmt.Errorf("data dir %s: %w", dir, err)
	}
	slog.Warn("data directory is not writable", "dir", dir, "fallback", fallback, "err", err)
	return fallback, nil
}

func usableDir(dir string) error {
	if dir == "" {
		return fmt.Errorf("path is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
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

func fallbackDataDir() string {
	return filepath.Join(os.TempDir(), "hebrew")
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

func newMCP(cfg Config, health http.Handler) http.Handler {
	// Course, lexicon, and unit stores stay unset until *db.Store
	// grows ListUnits, LatestPlacement, PlacementKnownLexemes,
	// PlacementLearningLexemes, StruggleStats, CompletedSharedUnits,
	// LearnerKnownLexemes,
	// SearchLexemes, PutDraftUnit, and units.Lookup. Tools then
	// answer with isError. LatestPlacement is not on this branch;
	// its pull request is stacked on #60. The adapter belongs here,
	// next to this call, so db does not import mcp.
	authoring := mcp.Handler(mcp.Options{AuthToken: cfg.MCPAuthToken})
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", health)
	mux.Handle("/", authoring)
	return mux
}
