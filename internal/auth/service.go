package auth

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
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
	// AllowedEmails is MCP_OAUTH_ALLOWED_EMAILS. Anyone else is refused,
	// except addresses in AdminEmails.
	AllowedEmails []string
	// AdminEmails is ADMIN_EMAILS. Those addresses sign in as admin.
	AdminEmails []string
	Clock       clock.Clock
	Store       *db.Store
}

// Service is Google sign-in, sessions, and CSRF for one process.
type Service struct {
	store        *db.Store
	clock        clock.Clock
	origin       string
	cookieSecure bool
	proxies      []net.IPNet
	clientID     string
	clientSecret string
	authURL      string
	tokenURL     string
	jwksURL      string
	issuer       string
	allowed      map[string]struct{}
	admins       map[string]struct{}
	httpClient   *http.Client
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
	return &Service{
		store:        cfg.Store,
		clock:        cfg.Clock,
		origin:       origin,
		cookieSecure: cfg.CookieSecure,
		proxies:      nets,
		clientID:     strings.TrimSpace(cfg.GoogleClientID),
		clientSecret: strings.TrimSpace(cfg.GoogleClientSecret),
		authURL:      strings.TrimSpace(cfg.GoogleAuthURL),
		tokenURL:     strings.TrimSpace(cfg.GoogleTokenURL),
		jwksURL:      strings.TrimSpace(cfg.GoogleJWKSURL),
		issuer:       strings.TrimSpace(cfg.GoogleIssuer),
		allowed:      emailSet(cfg.AllowedEmails),
		admins:       emailSet(cfg.AdminEmails),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (s *Service) now() time.Time {
	return s.clock.Now().UTC().Truncate(time.Second)
}

func (s *Service) googleOn() bool {
	return s.clientID != "" && s.clientSecret != "" && s.origin != "" &&
		s.authURL != "" && s.tokenURL != "" && s.jwksURL != "" && s.issuer != ""
}

func (s *Service) roleFor(email string) (string, bool) {
	email = normalizeEmail(email)
	if _, ok := s.admins[email]; ok {
		return RoleAdmin, true
	}
	if _, ok := s.allowed[email]; ok {
		return RoleLearner, true
	}
	return "", false
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

func emailSet(list []string) map[string]struct{} {
	out := make(map[string]struct{}, len(list))
	for _, e := range list {
		e = normalizeEmail(e)
		if e == "" {
			continue
		}
		out[e] = struct{}{}
	}
	return out
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
