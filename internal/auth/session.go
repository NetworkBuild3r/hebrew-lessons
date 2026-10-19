package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

const (
	cookieSession     = "hebrew_session"
	cookieSessionHost = "__Host-hebrew_session"
)

// LookupSession resolves rawID, the session cookie value, at now.
// MCP Builder can call this with the store and the process clock.
// Expired rows are deleted. LookupSession does not slide the expiry;
// the HTTP middleware does that. ErrNoSession means the id is missing,
// malformed, unknown, or expired.
func LookupSession(ctx context.Context, store *db.Store, rawID string, now time.Time) (User, Session, error) {
	if store == nil {
		return User{}, Session{}, errors.New("auth: nil store")
	}
	now = now.UTC()
	if err := store.DeleteExpiredSessions(ctx, now); err != nil {
		return User{}, Session{}, err
	}
	hash, err := sessionHash(rawID)
	if err != nil {
		return User{}, Session{}, ErrNoSession
	}
	row, user, err := store.SessionUser(ctx, hash)
	if errors.Is(err, db.ErrNotFound) {
		return User{}, Session{}, ErrNoSession
	}
	if err != nil {
		return User{}, Session{}, err
	}
	if !row.ExpiresAt.After(now) {
		_ = store.DeleteSession(ctx, hash)
		return User{}, Session{}, ErrNoSession
	}
	return mapUser(user), mapSession(row), nil
}

// Lookup resolves the cookie value using this service's store and clock,
// then re-checks the live allowlist. A removed address loses its session
// rows and returns ErrNoSession. A demoted admin comes back as a learner.
func (s *Service) Lookup(ctx context.Context, rawID string) (User, Session, error) {
	user, sess, err := LookupSession(ctx, s.store, rawID, s.now())
	if err != nil {
		return User{}, Session{}, err
	}
	user, ok, err := s.admit(ctx, user)
	if err != nil {
		return User{}, Session{}, err
	}
	if !ok {
		return User{}, Session{}, ErrNoSession
	}
	return user, sess, nil
}

func (s *Service) createSession(ctx context.Context, userID int64) (raw string, sess Session, err error) {
	if err = s.store.DeleteExpiredSessions(ctx, s.now()); err != nil {
		return "", Session{}, err
	}
	idBytes := make([]byte, sessionIDBytes)
	if _, err = rand.Read(idBytes); err != nil {
		return "", Session{}, err
	}
	csrfBytes := make([]byte, sessionIDBytes)
	if _, err = rand.Read(csrfBytes); err != nil {
		return "", Session{}, err
	}
	raw = base64.RawURLEncoding.EncodeToString(idBytes)
	csrf := base64.RawURLEncoding.EncodeToString(csrfBytes)
	exp := s.now().Add(SessionLifetime)
	hash := hashBytes(idBytes)
	if err = s.store.InsertSession(ctx, hash, userID, csrf, exp); err != nil {
		return "", Session{}, err
	}
	return raw, Session{IDHash: hash, UserID: userID, CSRF: csrf, ExpiresAt: exp}, nil
}

func (s *Service) renew(ctx context.Context, sess Session) (time.Time, bool, error) {
	last := sess.ExpiresAt.Add(-SessionLifetime)
	if s.now().Sub(last) < SessionRenewAfter {
		return sess.ExpiresAt, false, nil
	}
	exp := s.now().Add(SessionLifetime)
	if err := s.store.UpdateSessionExpiry(ctx, sess.IDHash, exp); err != nil {
		return time.Time{}, false, err
	}
	return exp, true, nil
}

func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) {
	if sess, ok := sessionFrom(r.Context()); ok {
		_ = s.store.DeleteSession(r.Context(), sess.IDHash)
	}
	_ = s.store.DeleteExpiredSessions(r.Context(), s.now())
	s.clearSessionCookies(w)
	w.Header().Set("Cache-Control", "no-store")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/signin")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/signin", http.StatusSeeOther)
}

func sessionHash(cookie string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cookie)
	if err != nil || len(raw) != sessionIDBytes {
		return "", ErrNoSession
	}
	return hashBytes(raw), nil
}

func hashBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (s *Service) sessionCookieName() string {
	if s.cookieSecure {
		return cookieSessionHost
	}
	return cookieSession
}

func (s *Service) readSessionCookie(r *http.Request) string {
	// Secure mode reads only the __Host- name. A plain hebrew_session
	// cookie can be planted by a sibling host or a non-HTTPS response,
	// so it is ignored entirely. Logout still clears both names.
	return readCookie(r, s.sessionCookieName())
}

func readCookie(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

func (s *Service) setSessionCookie(w http.ResponseWriter, raw string, expires time.Time) {
	s.setCookie(w, s.sessionCookieName(), raw, expires, int(SessionLifetime/time.Second))
}

func (s *Service) clearSessionCookies(w http.ResponseWriter) {
	s.clearCookie(w, cookieSession)
	s.clearCookie(w, cookieSessionHost)
}

func (s *Service) setCookie(w http.ResponseWriter, name, value string, expires time.Time, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cookieSecure,
	})
}

func (s *Service) clearCookie(w http.ResponseWriter, name string) {
	secure := s.cookieSecure
	if len(name) >= 7 && name[:7] == "__Host-" {
		secure = true
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}
