// Package web is the learner HTTP router, templ views, and embedded static
// assets. Owned by Frontend.
//
// Views are templ sources in views/, compiled into this package. Static
// files (css, icons, htmx, shell.js) live in static/ and are the only tree
// embedded here.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
	"github.com/a-h/templ"
)

// Assets is the embedded static/ tree. Views are compiled, not embedded.
//
//go:embed static
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
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(noDirFS{static}))))
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
	return securityHeaders(csrfMiddleware(tokens, mux))
}

// securityHeaders sets the policy that has to be in place before a write
// route exists. frame-ancestors none is part of the CSP. The inline theme
// script is allowed by its sha256 hash; every other script is a file under
// /static/js/.
func securityHeaders(next http.Handler) http.Handler {
	policy := "default-src 'self'; script-src 'self' '" + views.ThemeBootCSP + "'; style-src 'self'; img-src 'self'; font-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", policy)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// noDirFS hides directories so FileServer cannot list them.
type noDirFS struct{ fs.FS }

func (n noDirFS) Open(name string) (fs.File, error) {
	f, err := n.FS.Open(name)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if stat.IsDir() {
		f.Close()
		return nil, fs.ErrNotExist
	}
	return f, nil
}

// homeFrom is placeholder Today data until the review queue is wired.
// ?lang=en flips the shell. ?due=0 clears the queue. ?next=0 hides the lesson.
func homeFrom(r *http.Request, csrf string) views.Home {
	q := r.URL.Query()
	ui := views.UIFor(q.Get("lang"), views.FakeLearner().AddressGender)
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
