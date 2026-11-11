package auth

import (
	"log/slog"
	"net/http"
)

// LocalLearnerEmail is the only account POST /auth/local can sign in.
// It exists only when the process was started with DEV=1.
const LocalLearnerEmail = "local@localhost"

// handleLocalSignIn signs in LocalLearnerEmail. It is not registered unless
// LocalDev is set. Origin must match the configured public origin.
func (s *Service) handleLocalSignIn(w http.ResponseWriter, r *http.Request) {
	if !s.localDev {
		http.NotFound(w, r)
		return
	}
	got, ok := requestOrigin(r)
	if !ok || got != s.origin {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	role, allowed := s.roleFor(LocalLearnerEmail)
	if !allowed {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	user, err := s.store.UpsertGoogleUser(r.Context(), LocalLearnerEmail, "local-dev", role)
	if err != nil {
		slog.Error("local user", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	raw, sess, err := s.createSession(r.Context(), user.ID)
	if err != nil {
		slog.Error("local session", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	s.setSessionCookie(w, raw, sess.ExpiresAt)
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
