package auth

import (
	"context"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

const (
	// RoleLearner is the default signed-in role.
	RoleLearner = "learner"
	// RoleAdmin is the role for addresses listed in ADMIN_EMAILS.
	RoleAdmin = "admin"

	// SessionLifetime is the old app's session length
	// (server/auth/sessions.js LIFETIME_MS, server/http/cookies.js MAX_AGE_SEC).
	SessionLifetime = 30 * 24 * time.Hour
	// SessionRenewAfter is the old app's minimum gap between session writes
	// (server/auth/sessions.js LAST_SEEN_MIN_INTERVAL_MS). v2 has no
	// last_seen_at column, so a request at least this long after the last
	// expiry was chosen slides expires_at forward by SessionLifetime.
	SessionRenewAfter = time.Minute

	sessionIDBytes = 32
	oidcStateTTL   = 10 * time.Minute

	// stateKeyInfo is the HKDF info label for the OAuth state HMAC key.
	// HEBREW_SESSION_SECRET is the input keying material.
	stateKeyInfo = "hebrew-lessons oauth-state v1"

	// jwksCacheTTL is how long a successful JWKS response is reused.
	jwksCacheTTL = 5 * time.Minute
	// jwksUnknownKidMinInterval is the minimum gap between JWKS refetches
	// caused by an ID token whose kid is not in the cache. A burst of
	// tokens with unknown kids cannot fetch more often than this.
	jwksUnknownKidMinInterval = time.Minute
)

// ErrNoSession means the cookie value is missing, unknown, or expired.
var ErrNoSession = errors.New("auth: no session")

// Config is the slice of process configuration auth needs.
// Clock and Store are required; a nil Clock uses the system clock.
type Config struct {
	PublicOrigin       string
	CookieSecure       bool
	TrustedProxies     []string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleAuthURL      string
	GoogleTokenURL     string
	GoogleJWKSURL      string
	GoogleIssuer       string
	// StateSecret is HEBREW_SESSION_SECRET. New derives the OAuth state
	// HMAC key from it with HKDF-SHA256 and stateKeyInfo. An empty value
	// uses a random per-process key, and New logs that fallback once.
	StateSecret string
	// AllowedEmails is MCP_OAUTH_ALLOWED_EMAILS. Anyone else is refused,
	// except addresses in AdminEmails. These slices are copied at New
	// when Access is nil.
	AllowedEmails []string
	// AdminEmails is ADMIN_EMAILS. Those addresses sign in as admin.
	AdminEmails []string
	// Access is the live allowlist. When set, it is read on every
	// authenticated request and AllowedEmails and AdminEmails are ignored.
	// Mutate it with Set. It is not global state.
	Access *Access
	// OAuthRedirectURIs is MCP_OAUTH_REDIRECT_URIS. These exact strings
	// are allowed at dynamic registration, in addition to the built-in
	// Cursor redirects and http loopback.
	OAuthRedirectURIs []string
	Clock             clock.Clock
	Store             *db.Store
}

// Service is Google sign-in, sessions, and CSRF for one process.
type Service struct {
	store            *db.Store
	clock            clock.Clock
	origin           string
	cookieSecure     bool
	proxies          []net.IPNet
	clientID         string
	clientSecret     string
	authURL          string
	tokenURL         string
	jwksURL          string
	issuer           string
	access           *Access
	oauthRedirects   []string
	stateKey         []byte
	rateMu           sync.Mutex
	rate             map[string]rateWindow
	httpClient       *http.Client
	jwksMu           sync.Mutex
	jwksKeys         []rsaPublic
	jwksUntil        time.Time
	jwksUnknownKidAt time.Time
}

// New builds the auth service. Invalid TRUSTED_PROXIES entries are logged
// and ignored. They do not stop startup.
func New(cfg Config) (*Service, error) {
	if cfg.Store == nil {
		return nil, errors.New("auth: nil store")
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.System()
	}
	origin, _ := canonicalOrigin(cfg.PublicOrigin)
	nets, invalid := parseProxies(cfg.TrustedProxies)
	for _, bad := range invalid {
		slog.Warn("ignoring invalid TRUSTED_PROXIES entry", "entry", bad)
	}
	access := cfg.Access
	if access == nil {
		access = NewAccess(cfg.AllowedEmails, cfg.AdminEmails)
	}
	stateKey, err := stateKeyFromSecret(cfg.StateSecret)
	if err != nil {
		return nil, err
	}
	return &Service{
		store:          cfg.Store,
		clock:          cfg.Clock,
		origin:         origin,
		cookieSecure:   cfg.CookieSecure,
		proxies:        nets,
		clientID:       strings.TrimSpace(cfg.GoogleClientID),
		clientSecret:   strings.TrimSpace(cfg.GoogleClientSecret),
		authURL:        strings.TrimSpace(cfg.GoogleAuthURL),
		tokenURL:       strings.TrimSpace(cfg.GoogleTokenURL),
		jwksURL:        strings.TrimSpace(cfg.GoogleJWKSURL),
		issuer:         strings.TrimSpace(cfg.GoogleIssuer),
		access:         access,
		oauthRedirects: append([]string(nil), cfg.OAuthRedirectURIs...),
		stateKey:       stateKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

var warnStateSecretOnce sync.Once

// stateKeyFromSecret derives the 32-byte OAuth state HMAC key.
// An empty secret falls back to a random per-process key and logs once.
func stateKeyFromSecret(secret string) ([]byte, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		warnStateSecretOnce.Do(func() {
			slog.Warn("HEBREW_SESSION_SECRET is unset; oauth state is sealed with a random per-process key")
		})
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		return key, nil
	}
	return hkdf.Key(sha256.New, []byte(secret), nil, stateKeyInfo, 32)
}

func (s *Service) now() time.Time {
	return s.clock.Now().UTC()
}

func (s *Service) googleOn() bool {
	return s.clientID != "" && s.clientSecret != "" && s.origin != "" &&
		s.authURL != "" && s.tokenURL != "" && s.jwksURL != "" && s.issuer != ""
}

func (s *Service) roleFor(email string) (string, bool) {
	return s.access.role(email)
}

// admit applies the live allowlist. The role on the returned user is the
// role computed for this request, not the role stored at login. When that
// differs from users.role, the row is updated so a later read matches.
// An address that is no longer allowed loses every session row, and the
// request is unauthenticated.
func (s *Service) admit(ctx context.Context, user User) (User, bool, error) {
	role, ok := s.roleFor(user.Email)
	if !ok {
		if err := s.store.DeleteUserSessions(ctx, user.ID); err != nil {
			return User{}, false, err
		}
		return User{}, false, nil
	}
	if user.Role != role {
		if err := s.store.UpdateUserRole(ctx, user.ID, role); err != nil {
			return User{}, false, err
		}
		user.Role = role
	}
	return user, true, nil
}

// UserByEmail is the signed-in user row for an email address.
func (s *Service) UserByEmail(ctx context.Context, email string) (User, error) {
	u, err := s.store.UserByEmail(ctx, normalizeEmail(email))
	if err != nil {
		return User{}, err
	}
	return mapUser(u), nil
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// User is a signed-in account.
type User struct {
	ID        int64
	Email     string
	GoogleSub string
	Role      string
	CreatedAt time.Time
}

// Session is a browser session. IDHash is what the database stores.
type Session struct {
	IDHash    string
	UserID    int64
	CSRF      string
	ExpiresAt time.Time
}

func mapUser(u db.User) User {
	return User{
		ID:        u.ID,
		Email:     u.Email,
		GoogleSub: u.GoogleSub,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
	}
}

func mapSession(s db.Session) Session {
	return Session{
		IDHash:    s.IDHash,
		UserID:    s.UserID,
		CSRF:      s.CSRF,
		ExpiresAt: s.ExpiresAt,
	}
}
