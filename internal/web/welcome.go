package web

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// The first-run welcome (onboarding frame A). A signed-in learner whose
// IsOnboarded is false and who GETs a landing route is sent to GET /welcome.
// The handler does not combine onboarded_at, finished runs, and lesson
// attempts. "Start from the basics" POSTs here and calls SetOnboarded.
// Finishing a placement sets onboarded_at in the store's transaction; this
// file does not.

// needsWelcome is the gate. It is true only when IsOnboarded is false.
// A read error fails open (the caller does not redirect).
func (s *server) needsWelcome(ctx context.Context, userID int64) (bool, error) {
	if s == nil || s.onboard == nil {
		return false, nil
	}
	on, err := s.onboard.IsOnboarded(ctx, userID)
	if err != nil {
		return false, err
	}
	return !on, nil
}

// welcomeGate redirects a learner who still needs the first-run choice.
// POSTs are not landing routes. Signed-out requests are handled by gated
// before this runs.
func (s *server) welcomeGate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if user, ok := auth.CurrentUser(r.Context()); ok && user != nil {
				need, err := s.needsWelcome(r.Context(), user.ID)
				if err != nil {
					slog.Warn("welcome gate", "err", err)
				} else if need {
					w.Header().Set("Cache-Control", "no-store")
					http.Redirect(w, r, "/welcome", http.StatusSeeOther)
					return
				}
			}
		}
		next(w, r)
	}
}

func (s *server) serveWelcome(w http.ResponseWriter, r *http.Request, secure bool, csrf string) {
	if user, ok := auth.CurrentUser(r.Context()); ok && user != nil {
		need, err := s.needsWelcome(r.Context(), user.ID)
		if err != nil {
			slog.Warn("welcome", "err", err)
		} else if !need {
			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}
	lang := chooseLang(w, r, secure)
	page := views.NewPage(lang, csrf, "welcome")
	page.Title = views.WelcomeTitle
	w.Header().Set("Cache-Control", "no-store")
	render(w, r, views.Welcome(page))
}

// postBasics is POST /onboarding/basics. Both "Start from the basics" paths
// land here: the welcome card and the placement result link. It calls
// SetOnboarded and goes to Today. There is no confirm step.
func (s *server) postBasics(w http.ResponseWriter, r *http.Request, secure bool, _ string) {
	_ = secure
	user, ok := auth.CurrentUser(r.Context())
	if !ok || user == nil || s == nil || s.onboard == nil {
		http.Error(w, "onboarding is not available", http.StatusServiceUnavailable)
		return
	}
	if err := s.onboard.SetOnboarded(r.Context(), user.ID, s.now()); err != nil {
		slog.Warn("onboarding basics", "err", err)
		http.Error(w, "could not save that choice", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
