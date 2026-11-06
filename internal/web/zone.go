package web

import (
	"net/http"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// observeZone is POST /_hx/zone. It stores the device zone for the session
// learner when users.time_zone is still NULL. A user id in the body is ignored.
// Invalid zones are 400 and write nothing. Success is 204 whether or not a row
// changed. INIT-032/SPEC-007 · ADR-0027 D6.
func (s *server) observeZone(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.CurrentUser(r.Context())
	if !ok || user == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	zone := r.PostForm.Get("zone")
	if _, err := studyday.ParseZone(zone); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if s == nil || s.store == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := s.store.SetUserTimeZoneIfUnset(r.Context(), user.ID, zone); err != nil {
		serverError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
