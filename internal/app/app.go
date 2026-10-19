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
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcpstore"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
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
	// key (HKDF-SHA256). When OAuth is enabled, an empty value refuses
	// to start. OAuth is enabled when MCP_PUBLIC_URL is set or Google
	// client credentials are set.
	StateSecret string
	// OAuthRedirectURIs is MCP_OAUTH_REDIRECT_URIS, extra exact redirect
	// URIs for dynamic registration.
	OAuthRedirectURIs []string

	// AllowedEmails is MCP_OAUTH_ALLOWED_EMAILS. AdminEmails is ADMIN_EMAILS.
	// They seed auth.Access when Access is nil. A non-nil Access is the
	// live list: sign-in and every later request read it, and Set changes
	// what the next request allows.
	AllowedEmails []string
	AdminEmails   []string
	Access        *auth.Access

	// MCPAuthToken is the bearer secret for the authoring tools.
	// cmd/hebrew copies MCP_AUTH_TOKEN and trims it. An empty token
	// disables the static bearer: it accepts no request. Startup still
	// proceeds. internal/mcp does not read the environment.
	// GET /healthz stays open.
	// MCPPublicURL is the public MCP origin. When it is set, it is the
	// OAuth issuer. Empty does not stop startup; the trusted proxy's
	// public host is the issuer then. MCPAllowedOrigins are browser
	// Origins allowed to call MCP. A request with no Origin header is
	// allowed. cmd/hebrew reads MCP_ALLOWED_ORIGINS.
	MCPAuthToken      string
	MCPPublicURL      string
	MCPAllowedOrigins []string

	// ElevenLabsAPIKey gates the real TTS client. Empty leaves TTS unset
	// unless TTS is set directly. The key is read at synthesis time:
	// an empty value does not stop New.
	ElevenLabsAPIKey string
	// ElevenLabsVoice is ELEVENLABS_VOICE_ID. Empty uses the legacy voice.
	ElevenLabsVoice string
	// ElevenLabsModel is ELEVENLABS_MODEL. Empty uses the legacy model.
	ElevenLabsModel string

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
	if cfg.ElevenLabsVoice == "" {
		cfg.ElevenLabsVoice = audio.DefaultVoiceID
	}
	if cfg.ElevenLabsModel == "" {
		cfg.ElevenLabsModel = audio.DefaultModel
	}
	return cfg
}

// App is the running process core.
type App struct {
	cfg       Config
	store     *db.Store
	authSvc   *auth.Service
	review    *fsrs.Review
	placement *placement.Service
	audioDir  string
	tts       audio.TTS
	web       http.Handler
	mcp       http.Handler
	stopSweep context.CancelFunc
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
	if oauthEnabled(cfg) && strings.TrimSpace(cfg.StateSecret) == "" {
		return nil, fmt.Errorf("HEBREW_SESSION_SECRET is required when OAuth is enabled")
	}
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
		OAuthRedirectURIs:  cfg.OAuthRedirectURIs,
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
		client := audio.NewElevenLabs(cfg.ElevenLabsAPIKey)
		client.Voice = cfg.ElevenLabsVoice
		client.Model = cfg.ElevenLabsModel
		tts = client
	}

	review := fsrs.New(store, cfg.Clock)
	a := &App{
		cfg:       cfg,
		store:     store,
		authSvc:   svc,
		review:    review,
		placement: placement.New(store, review, cfg.Clock),
		audioDir:  audioDir,
		tts:       tts,
	}
	health := a.healthHandler()
	a.web = web.New(health, svc)
	a.mcp = newMCP(cfg, health, svc, store, tts, audioDir)
	sweepCtx, stop := context.WithCancel(context.Background())
	a.stopSweep = stop
	go sweepOAuth(sweepCtx, store, cfg.Clock)
	return a, nil
}

// oauthEnabled reports that the MCP authorization server or Google
// sign-in will run. Either one seals OAuth state with HEBREW_SESSION_SECRET.
func oauthEnabled(cfg Config) bool {
	if strings.TrimSpace(cfg.MCPPublicURL) != "" {
		return true
	}
	return strings.TrimSpace(cfg.GoogleClientID) != "" && strings.TrimSpace(cfg.GoogleClientSecret) != ""
}

func sweepOAuth(ctx context.Context, store *db.Store, clk clock.Clock) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := clk.Now().UTC()
			if err := store.SweepOAuth(ctx, now, now.Add(-auth.OAuthClientTTL)); err != nil && ctx.Err() == nil {
				slog.Error("mcp oauth sweep", "err", err)
			}
		}
	}
}

// Web is the learner handler.
func (a *App) Web() http.Handler { return a.web }

// MCP is the authoring handler. GET /healthz is unauthenticated.
// /mcp is the streamable MCP server. OAuth routes are mounted beside it.
// Any other path is 404.
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

// Placement is the vocabulary placement deck.
// It uses the clock from Config, the same clock the store and the review service use.
func (a *App) Placement() *placement.Service { return a.placement }

// Close releases the database. /healthz answers 503 after Close returns.
func (a *App) Close() error {
	a.closeOnce.Do(func() {
		if a.stopSweep != nil {
			a.stopSweep()
		}
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

func newMCP(cfg Config, health http.Handler, sessions *auth.Service, store *db.Store, tts audio.TTS, audioDir string) http.Handler {
	// The adapter is the *db.Store behind the authoring tools.
	// It lives in internal/mcpstore so internal/db does not import internal/mcp.
	//
	// AuthToken is Config.MCPAuthToken. cmd/hebrew trimmed MCP_AUTH_TOKEN
	// into that field and logs once when it is empty. An empty value
	// disables the static bearer. This function does not read the
	// environment and does not log that warning again. sessionUsers calls
	// auth.Service.Lookup and copies users.role. SessionFromToken maps an
	// OAuth access token onto that same Lookup path. It does not read the
	// web session cookie. Admin is users.role from ADMIN_EMAILS.
	//
	// OAuth discovery, registration, authorize, and token are mounted on
	// this mux, outside the authoring handler. The authoring handler's
	// bearer middleware and CrossOriginProtection therefore do not see
	// them. MCP_PUBLIC_URL is the issuer when it is set. The trusted
	// proxy's public host is the fallback when it is unset. The pod
	// address is never advertised. Only /mcp reaches the authoring
	// handler. Every other unmatched path is 404.
	adapter := mcpstore.New(store)
	users := sessionUsers{auth: sessions}
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", health)
	if sessions != nil {
		sessions.RegisterOAuth(mux, cfg.MCPPublicURL)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not found"))
	})
	authoring := mcp.Handler(mcp.Options{
		AuthToken:        cfg.MCPAuthToken,
		AllowedOrigins:   cfg.MCPAllowedOrigins,
		Sessions:         users,
		SessionFromToken: users.SessionFromToken,
		MetadataURL: func(r *http.Request) string {
			if sessions == nil {
				return ""
			}
			base, _, ok := sessions.PublicBase(r, cfg.MCPPublicURL)
			if !ok {
				return ""
			}
			return base + "/.well-known/oauth-protected-resource"
		},
		Course:  adapter,
		Lexicon: adapter,
		Lookup:  adapter,
		Units:   adapter,
		Audio: audioGen{
			gen: &audio.Generator{
				TTS:   tts,
				Store: store,
				Dir:   audioDir,
				Voice: cfg.ElevenLabsVoice,
				Model: cfg.ElevenLabsModel,
			},
			store: store,
		},
	})
	mux.Handle("/mcp", authoring)
	mux.Handle("/mcp/{$}", authoring)
	return mux
}
