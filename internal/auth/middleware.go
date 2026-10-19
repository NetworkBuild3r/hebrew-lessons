package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

type (
	userKey    struct{}
	sessionKey struct{}
)

// CurrentUser returns the signed-in user Middleware stored on the request.
// The bool is false when there is no session.
func CurrentUser(ctx context.Context) (*User, bool) {
	if ctx == nil {
		return nil, false
	}
	u, ok := ctx.Value(userKey{}).(*User)
	if !ok || u == nil {
		return nil, false
	}
	return u, true
}

func sessionFrom(ctx context.Context) (Session, bool) {
	if ctx == nil {
		return Session{}, false
	}
	sess, ok := ctx.Value(sessionKey{}).(Session)
	if !ok || sess.IDHash == "" {
		return Session{}, false
	}
	return sess, true
}

// Middleware loads the session, slides a live expiry, and checks CSRF on
// POST /auth/logout and on every non-GET/HEAD/OPTIONS request under /_hx/.
// GET /healthz is passed through without a session lookup so a database
// probe does not depend on the cookie.
func (s *Service) Middleware(next http.Handler) http.Handler {
	if next == nil {
		next = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		r, err := s.attach(w, r)
		if err != nil {
			slog.Error("session", "err", err)
			writeJSON(w, http.StatusInternalServerError, "internal")
			return
		}
		if csrfRequired(r) && !s.csrfOK(r) {
			writeJSON(w, http.StatusForbidden, "csrf")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Service) attach(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	ctx := context.WithValue(r.Context(), proxyKey{}, s.proxies)
	r = r.WithContext(ctx)
	raw := s.readSessionCookie(r)
	if raw == "" {
		return r, nil
	}
	user, sess, err := s.Lookup(r.Context(), raw)
	if errors.Is(err, ErrNoSession) {
		s.clearSessionCookies(w)
		return r, nil
	}
	if err != nil {
		return r, err
	}
	exp, renewed, err := s.renew(r.Context(), sess)
	if err != nil {
		return r, err
	}
	if renewed {
		sess.ExpiresAt = exp
		s.setSessionCookie(w, raw, exp)
	}
	ctx = context.WithValue(r.Context(), userKey{}, &user)
	ctx = context.WithValue(ctx, sessionKey{}, sess)
	return r.WithContext(ctx), nil
}

// RequireUser redirects an HTML request that has no session to
// /auth/google/start. /_hx/ and requests that ask for JSON get 401.
func (s *Service) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := CurrentUser(r.Context()); !ok {
			if wantsJSON(r) {
				writeJSON(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, r, "/auth/google/start", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin allows an admin through. A signed-out request is handled
// like RequireUser. A signed-in non-admin gets 403.
func (s *Service) RequireAdmin(next http.Handler) http.Handler {
	return s.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := CurrentUser(r.Context())
		if !ok || u.Role != RoleAdmin {
			writeJSON(w, http.StatusForbidden, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func wantsJSON(r *http.Request) bool {
	p := r.URL.Path
	if p == "/_hx" || strings.HasPrefix(p, "/_hx/") {
		return true
	}
	accept := r.Header.Get("Accept")
	jsonAt := strings.Index(accept, "application/json")
	if jsonAt < 0 {
		return false
	}
	htmlAt := strings.Index(accept, "text/html")
	return htmlAt < 0 || jsonAt < htmlAt
}

// Register adds Google sign-in and logout on mux.
// Middleware must wrap the mux: it loads the session and checks CSRF.
func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/google/start", s.handleGoogleStart)
	mux.HandleFunc("GET /auth/google/callback", s.handleGoogleCallback)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
}

func writeJSON(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{Error: code})
}
