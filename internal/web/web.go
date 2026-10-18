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
	"strconv"

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
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		render(w, r, views.Today(homeFrom(r, csrfValue(tokens, r))))
	})
	mux.HandleFunc("GET /review", func(w http.ResponseWriter, r *http.Request) {
		render(w, r, views.Review(views.NewPage(r.URL.Query().Get("lang"), csrfValue(tokens, r), "review")))
	})
	mux.HandleFunc("GET /progress", func(w http.ResponseWriter, r *http.Request) {
		render(w, r, views.Progress(views.NewPage(r.URL.Query().Get("lang"), csrfValue(tokens, r), "progress")))
	})
	mux.HandleFunc("GET /lesson", func(w http.ResponseWriter, r *http.Request) {
		render(w, r, views.Lesson(views.NewPage(r.URL.Query().Get("lang"), csrfValue(tokens, r), "lesson")))
	})
	return csrfMiddleware(tokens, mux)
}

// homeFrom is placeholder Today data until the review queue is wired.
// ?lang=en flips the shell. ?due=0 clears the queue. ?next=0 hides the lesson.
func homeFrom(r *http.Request, csrf string) views.Home {
	q := r.URL.Query()
	ui := views.UIFor(q.Get("lang"))
	due := 12
	if q.Get("due") == "0" {
		due = 0
	}
	return views.Home{
		UI:      ui,
		CSRF:    csrf,
		Due:     due,
		DueText: strconv.Itoa(due),
		Lesson:  q.Get("next") != "0",
		Words:   views.SampleWords(),
	}
}

// Static returns the embedded static files.
func Static() (fs.FS, error) {
	return fs.Sub(Assets, "static")
}

func render(w http.ResponseWriter, r *http.Request, page templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := page.Render(r.Context(), w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func csrfValue(tokens CSRFToken, r *http.Request) string {
	if tokens == nil {
		return ""
	}
	return tokens.CSRFToken(r)
}
