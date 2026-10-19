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
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
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
// CSRFFunc(auth.CSRFToken). A signed-out GET of /, /review, /progress,
// /lesson, or /placement redirects to /signin (303 for a full page, 401 and
// HX-Redirect for htmx). Reads stay on those page URLs. /_hx/ is for
// htmx writes, and this router does not register read routes there.
// New does not deal placement. NewWithPlacement wires the deck and the
// run store the handlers read for ownership and the Today card.
// NewOptions wires those plus the lesson store and the audio directory.
// Tests pass a fake to newMux; production does not. The ungated router
// still serves those pages.
// Audio bytes are served at /audio/<sha256>.mp3 from Options.AudioDir.
func New(health http.Handler, sessions *auth.Service) http.Handler {
	return NewOptions(health, sessions, Options{})
}

// Options is the data the learner pages read. A nil Store leaves Today
// and /lesson on the empty published-lesson state. Deck and Runs are the
// placement deck; a nil pair still serves the read-only intro.
type Options struct {
	Store    *db.Store
	AudioDir string
	Deck     placementDeck
	Runs     placementRuns
}

// NewOptions is New with the lesson store, the audio directory, and placement.
func NewOptions(health http.Handler, sessions *auth.Service, opts Options) http.Handler {
	return newHandler(health, sessions, opts)
}

// NewWithPlacement is New plus the placement deck. deck deals cards.
// runs is the store read for ownership and LatestPlacement. Both are
// required for a POST; a nil pair still serves the read-only intro.
// When runs is the process store, the lesson player uses it too.
func NewWithPlacement(health http.Handler, sessions *auth.Service, deck placementDeck, runs placementRuns) http.Handler {
	opts := Options{Deck: deck, Runs: runs}
	if store, ok := runs.(*db.Store); ok {
		opts.Store = store
	}
	return newHandler(health, sessions, opts)
}

func newHandler(health http.Handler, sessions *auth.Service, opts Options) http.Handler {
	var tokens CSRFToken
	secure := false
	if sessions != nil {
		tokens = CSRFFunc(auth.CSRFToken)
		// hebrew_lang uses the same COOKIE_SECURE flag as the session cookie.
		secure = sessions.CookieSecure()
	}
	mux := routes(health, tokens, sessions != nil, secure, opts)
	if sessions != nil {
		sessions.Register(mux)
		// auth.Middleware loads the session and checks CSRF, including
		// POST /auth/logout and POST /lesson. The local csrf middleware is
		// not stacked on top: it compares Origin to r.Host and would reject
		// a same-origin post behind a host-rewriting proxy.
		return securityHeaders(sessions.Middleware(mux))
	}
	return securityHeaders(csrfMiddleware(nil, mux))
}

// newMux is the test router. tokens is a fake; production New does not use it.
func newMux(health http.Handler, tokens CSRFToken) http.Handler {
	return securityHeaders(csrfMiddleware(tokens, routes(health, tokens, false, false, Options{})))
}

func routes(health http.Handler, tokens CSRFToken, gate, secure bool, opts Options) *http.ServeMux {
	mux := http.NewServeMux()
	if health != nil {
		mux.Handle("GET /healthz", health)
	}
	static, err := Static()
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(noDirFS{static}))))
	play := player{store: opts.Store, audioDir: opts.AudioDir}
	mux.HandleFunc("GET /{$}", gated(gate, secure, func(w http.ResponseWriter, r *http.Request) {
		lang := chooseLang(w, r, secure)
		state, err := todayPlacement(r, opts.Runs)
		if err != nil {
			serverError(w)
			return
		}
		home, err := homeFrom(r, lang, csrfValue(tokens, r), opts.Store, state)
		if err != nil {
			serverError(w)
			return
		}
		render(w, r, views.Today(home))
	}))
	review := gated(gate, secure, func(w http.ResponseWriter, r *http.Request) {
		render(w, r, views.Review(views.NewPage(chooseLang(w, r, secure), csrfValue(tokens, r), "review")))
	})
	progress := gated(gate, secure, func(w http.ResponseWriter, r *http.Request) {
		render(w, r, views.Progress(views.NewPage(chooseLang(w, r, secure), csrfValue(tokens, r), "progress")))
	})
	lesson := gated(gate, secure, func(w http.ResponseWriter, r *http.Request) {
		play.lessonIndex(w, r, chooseLang(w, r, secure), csrfValue(tokens, r))
	})
	lessonAt := gated(gate, secure, func(w http.ResponseWriter, r *http.Request) {
		play.lessonAt(w, r, chooseLang(w, r, secure), csrfValue(tokens, r))
	})
	lessonPost := gated(gate, secure, func(w http.ResponseWriter, r *http.Request) {
		play.lessonSubmit(w, r, chooseLang(w, r, secure), csrfValue(tokens, r))
	})
	mux.HandleFunc("GET /review", review)
	mux.HandleFunc("GET /progress", progress)
	mux.HandleFunc("GET /lesson", lesson)
	mux.HandleFunc("GET /lesson/{unitID}/{lessonIx}", lessonAt)
	mux.HandleFunc("POST /lesson/{unitID}/{lessonIx}", lessonPost)
	mux.HandleFunc("GET /audio/{name}", func(w http.ResponseWriter, r *http.Request) {
		serveAudio(opts.AudioDir, w, r)
	})
	mountPlacement(mux, tokens, gate, secure, opts.Deck, opts.Runs)
	mux.HandleFunc("GET /signin", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.CurrentUser(r.Context()); ok {
			w.Header().Set("Cache-Control", "no-store")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		render(w, r, views.SignIn(views.NewPage(chooseLang(w, r, secure), csrfValue(tokens, r), "signin")))
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

// homeFrom builds Today. The due count is still the placeholder: ?due=0
// clears it. ?next=0 hides the lesson card. Otherwise the card is the
// next published lesson in this learner's scope, or the empty state when
// nothing is waiting. A missing store or a signed-out request is empty.
// placement is copied through. Nil means the learner has never taken the check.
func homeFrom(r *http.Request, lang, csrf string, store *db.Store, placement *views.PlacementVM) (views.Home, error) {
	q := r.URL.Query()
	ui := views.UIFor(lang, views.FakeLearner().AddressGender)
	due := 12
	if q.Get("due") == "0" {
		due = 0
	}
	home := views.Home{
		UI:        ui,
		CSRF:      csrf,
		Due:       due,
		DueText:   strconv.Itoa(due),
		Placement: placement,
	}
	if q.Get("next") == "0" || store == nil {
		return home, nil
	}
	user, ok := auth.CurrentUser(r.Context())
	if !ok || user == nil {
		return home, nil
	}
	lesson, ok, err := store.NextCourseLesson(r.Context(), user.ID)
	if err != nil || !ok {
		return home, err
	}
	gloss, err := titleGloss(r.Context(), store, lesson.Title)
	if err != nil {
		return views.Home{}, err
	}
	home.Lesson = true
	home.LessonTitle = lesson.Title
	home.LessonGloss = gloss
	home.LessonHref = lessonPath(lesson.UnitID, lesson.LessonIx)
	home.UI.Situation = lesson.Situation
	home.Words = chipsFor(lesson.NewWords)
	return home, nil
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

// langCookieName remembers an explicit UI choice. A missing cookie is
// English, including for an existing session that never opted in.
const langCookieName = "hebrew_lang"

// chooseLang resolves the shell language. ?lang=he or ?lang=en is a choice
// and is stored on hebrew_lang. With no query, that cookie wins. With
// neither, the shell is English. secure is COOKIE_SECURE: the same flag
// the session cookie uses. The ungated router passes false.
func chooseLang(w http.ResponseWriter, r *http.Request, secure bool) string {
	q := r.URL.Query().Get("lang")
	if q == "he" || q == "en" {
		http.SetCookie(w, &http.Cookie{
			Name:     langCookieName,
			Value:    q,
			Path:     "/",
			MaxAge:   400 * 24 * 60 * 60,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secure,
		})
		return q
	}
	if c, err := r.Cookie(langCookieName); err == nil && (c.Value == "he" || c.Value == "en") {
		return c.Value
	}
	return "en"
}

func signInPath(w http.ResponseWriter, r *http.Request, secure bool) string {
	if chooseLang(w, r, secure) == "he" {
		return "/signin?lang=he"
	}
	if r.URL.Query().Get("lang") == "en" {
		return "/signin?lang=en"
	}
	return "/signin"
}

// gated sends a signed-out request to /signin when the router has a
// session service. A full page is 303. htmx gets 401 and HX-Redirect so
// the shell does not swap the sign-in page into #app. The ungated test
// router passes gate false and serves the page. secure is forwarded to
// chooseLang so a ?lang choice on the redirect still sets hebrew_lang.
func gated(gate, secure bool, next http.HandlerFunc) http.HandlerFunc {
	if !gate {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.CurrentUser(r.Context()); !ok {
			w.Header().Set("Cache-Control", "no-store")
			loc := signInPath(w, r, secure)
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", loc)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, loc, http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func csrfValue(tokens CSRFToken, r *http.Request) string {
	if tokens == nil {
		return ""
	}
	return tokens.CSRFToken(r)
}
