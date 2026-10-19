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

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
	"github.com/a-h/templ"
)

// Assets is the embedded static/ tree. Views are compiled, not embedded.
//
//go:embed static
var Assets embed.FS

// New returns the learner mux. health handles GET /healthz.
//
// sessions, when non-nil, registers Google sign-in and logout and wraps
// the mux with the session middleware. Page tokens come from
// CSRFFunc(auth.CSRFToken). A signed-out GET / redirects to /signin.
// Tests pass a fake to newMux; production does not.
// Audio bytes are served at /audio/<sha256>.mp3; this router does not
// register that path.
func New(health http.Handler, sessions *auth.Service) http.Handler {
	var tokens CSRFToken
	if sessions != nil {
		tokens = CSRFFunc(auth.CSRFToken)
	}
	mux := routes(health, tokens, sessions != nil)
	if sessions != nil {
		sessions.Register(mux)
		// auth.Middleware loads the session and checks CSRF, including
		// POST /auth/logout. The local csrf middleware is not stacked on
		// top: it compares Origin to r.Host and would reject a same-origin
		// post behind a host-rewriting proxy.
		return securityHeaders(sessions.Middleware(mux))
	}
	return securityHeaders(csrfMiddleware(nil, mux))
}

// newMux is the test router. tokens is a fake; production New does not use it.
func newMux(health http.Handler, tokens CSRFToken) http.Handler {
	return securityHeaders(csrfMiddleware(tokens, routes(health, tokens, false)))
}

func routes(health http.Handler, tokens CSRFToken, gate bool) *http.ServeMux {
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
		if gate {
			if _, ok := auth.CurrentUser(r.Context()); !ok {
				w.Header().Set("Cache-Control", "no-store")
				http.Redirect(w, r, signInPath(r), http.StatusSeeOther)
				return
			}
		}
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
	mux.HandleFunc("GET /signin", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.CurrentUser(r.Context()); ok {
			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		render(w, r, views.SignIn(views.NewPage(r.URL.Query().Get("lang"), csrfValue(tokens, r), "signin")))
	})
	return mux
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

func signInPath(r *http.Request) string {
	if r.URL.Query().Get("lang") == "en" {
		return "/signin?lang=en"
	}
	return "/signin"
}

func csrfValue(tokens CSRFToken, r *http.Request) string {
	if tokens == nil {
		return ""
	}
	return tokens.CSRFToken(r)
}
