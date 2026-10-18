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
)

// Assets is the embedded views/ and static/ trees.
//
//go:embed static views
var Assets embed.FS

// New returns the learner mux. health handles GET /healthz.
func New(health http.Handler) http.Handler {
	mux := http.NewServeMux()
	if health != nil {
		mux.Handle("GET /healthz", health)
	}
	return mux
}

// Static returns the embedded static files.
func Static() (fs.FS, error) {
	return fs.Sub(Assets, "static")
}
