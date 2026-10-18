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

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
	"github.com/a-h/templ"
)

// Assets is the embedded views/ and static/ trees.
//
//go:embed static views
var Assets embed.FS

// New returns the learner mux. health handles GET /healthz.
//
// The mux does not take a CSRF token yet: auth.CSRFToken is not on this
// branch. newMux is what tests call with a CSRFFunc fake. Learner writes
// belong under /_hx/ and pass through csrfMiddleware. Audio bytes are served
// at /audio/<sha256>.mp3; this router does not register that path.
func New(health http.Handler) http.Handler {
	return newMux(health, nil)
}

func newMux(health http.Handler, tokens CSRFToken) http.Handler {
	mux := http.NewServeMux()
	if health != nil {
		mux.Handle("GET /healthz", health)
	}
	static, err := Static()
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	mux.HandleFunc("GET /{$}", handle(tokens, views.Today))
	mux.HandleFunc("GET /review", handle(tokens, views.Review))
	mux.HandleFunc("GET /progress", handle(tokens, views.Progress))
	mux.HandleFunc("GET /lesson", handle(tokens, views.Lesson))
	return csrfMiddleware(tokens, mux)
}

// Static returns the embedded static files.
func Static() (fs.FS, error) {
	return fs.Sub(Assets, "static")
}

func handle(tokens CSRFToken, page func(string) templ.Component) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := page(csrfValue(tokens, r)).Render(r.Context(), w); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}
}

func csrfValue(tokens CSRFToken, r *http.Request) string {
	if tokens == nil {
		return ""
	}
	return tokens.CSRFToken(r)
}
