// Package web is the learner HTTP router, templ views, and embedded static
// assets. Owned by Frontend.
//
// Views are templ sources in views/. Static files (css, icons, vendored
// htmx) live in static/. Both trees are embedded here.
package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
)

// Assets is the embedded views/ and static/ trees.
//
//go:embed static views
var Assets embed.FS

// New returns the learner mux. health handles GET /healthz.
// sessions, when non-nil, registers Google sign-in and logout and wraps
// the mux with the session and CSRF middleware.
func New(health http.Handler, sessions *auth.Service) http.Handler {
	mux := http.NewServeMux()
	if health != nil {
		mux.Handle("GET /healthz", health)
	}
	if sessions != nil {
		sessions.Register(mux)
		return sessions.Middleware(mux)
	}
	return mux
}

// Static returns the embedded static files.
func Static() (fs.FS, error) {
	return fs.Sub(Assets, "static")
}
