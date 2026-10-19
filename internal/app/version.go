package app

import (
	"encoding/json"
	"net/http"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/buildinfo"
)

// versionHandler serves GET /version. It does not read the database or
// the session, so it stays up when /healthz is 503.
func versionHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(buildinfo.Current())
	})
}

// mountVersion serves GET /version before the learner router. The learner
// router sits behind session middleware, which looks up a cookie. This
// probe has no auth and must not depend on that lookup.
func mountVersion(next http.Handler) http.Handler {
	version := versionHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/version" {
			version.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
