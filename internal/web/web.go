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

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
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
// New does not deal placement. NewWithStore reads Today, Review, and
// Progress from the learner database. NewWithPlacement wires the deck and
// the run store. NewOptions wires those plus the lesson store and the audio
// directory. Tests pass a fake to newMux; production does not. The ungated
// router still serves those pages.
// Audio bytes are served at /audio/<sha256>.mp3 from Options.AudioDir.
func New(health http.Handler, sessions *auth.Service) http.Handler {
	return NewOptions(health, sessions, Options{})
}

// Options is the data the learner pages read. A nil Store leaves Today,
// Review, and Progress on the empty real state, and /lesson on the empty
// published-lesson state. Deck and Runs are the placement deck; a nil pair
// still serves the read-only intro.
type Options struct {
	Store    *db.Store
	AudioDir string
	// Voices is the speaker table of a dialogue (INIT-015/SPEC-026): a line is
	// played from the clip in its speaker's voice. A zero value means the default
	// female voice and no male voice, so a speaker b line is silent.
	Voices audio.Voices
	Deck   placementDeck
	Runs   placementRuns
	// SessionSecret is HEBREW_SESSION_SECRET. The key of the activity item tokens is
	// derived from it (activity.KeyFromSecret), so a token cannot be forged and one
	// issued before a restart still verifies. Empty gives a random key per process.
	SessionSecret string
	// Placement is the placement engine, for the check-in on Today. Nil leaves the
	// check-in off. See PlacementEngine.
	Placement PlacementEngine
	// study replaces the review session service. It is for tests in this package.
	study *session.Service
	// bands replaces where the learner model gets placement bands. For tests.
	bands learner.BandSource
}

// NewOptions is New with the lesson store, the audio directory, and placement.
func NewOptions(health http.Handler, sessions *auth.Service, opts Options) http.Handler {
	return newHandler(health, sessions, opts)
}

// NewWithStore is New with the learner database. Today, Review, and Progress
// read that store. The study queue is built by internal/session.
// The store is also the placement run reader and the lesson player store.
// Writes need NewWithPlacement. Audio stays empty until NewOptions.
func NewWithStore(health http.Handler, sessions *auth.Service, store *db.Store) http.Handler {
	var runs placementRuns
	if store != nil {
		runs = store
	}
	return newHandler(health, sessions, Options{Store: store, Runs: runs})
}

// NewWithPlacement is New plus the placement deck. deck deals cards.
// runs is the store read for ownership and LatestPlacement. Both are
// required for a POST; a nil pair still serves the read-only intro.
// When runs is the process store, Today, Review, Progress, and the lesson
// player use it too.
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
	srv := newServer(opts.Store)
	if opts.Store != nil {
		key, err := activity.KeyFromSecret(opts.SessionSecret)
		if err != nil {
			panic(err)
		}
		srv.useActivityKey(key)
		if opts.study != nil {
			srv.study = opts.study
		}
	}
	srv.audioDir = opts.AudioDir
	srv.voices = opts.Voices
	srv.deck = opts.Deck
	srv.runs = opts.Runs
	srv.recheck = opts.Placement
	srv.bands = opts.bands
	mux := srv.routes(health, tokens, sessions != nil, secure)
	if sessions != nil {
		sessions.Register(mux)
		// auth.Middleware loads the session and checks CSRF, including
		// POST /auth/logout, POST /lesson, and POST /_hx/. The local csrf
		// middleware is not stacked on top: it compares Origin to r.Host and
		// would reject a same-origin post behind a host-rewriting proxy.
		return securityHeaders(sessions.Middleware(mux))
	}
	return securityHeaders(csrfMiddleware(nil, mux))
}

// newMux is the test router. tokens is a fake; production New does not use it.
func newMux(health http.Handler, tokens CSRFToken) http.Handler {
	return securityHeaders(csrfMiddleware(tokens, newServer(nil).routes(health, tokens, false, false)))
}

func (s *server) routes(health http.Handler, tokens CSRFToken, gate, secure bool) *http.ServeMux {
	mux := http.NewServeMux()
	if health != nil {
		mux.Handle("GET /healthz", health)
	}
	static, err := Static()
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(noDirFS{static}))))
	play := player{store: s.store, audioDir: s.audioDir, voices: s.voices}
	mux.HandleFunc("GET /{$}", gated(gate, secure, func(w http.ResponseWriter, r *http.Request) {
		s.serveHome(w, r, secure, csrfValue(tokens, r))
	}))
	review := gated(gate, secure, func(w http.ResponseWriter, r *http.Request) {
		s.serveReview(w, r, secure, csrfValue(tokens, r))
	})
	progress := gated(gate, secure, func(w http.ResponseWriter, r *http.Request) {
		s.serveProgress(w, r, secure, csrfValue(tokens, r))
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
		serveAudio(s.audioDir, w, r)
	})
	mux.HandleFunc("POST /_hx/review/reveal", gated(gate, secure, s.reveal))
	mux.HandleFunc("POST /_hx/review/grade", gated(gate, secure, s.grade))
	mux.HandleFunc("POST /_hx/review/answer", gated(gate, secure, s.answer))
	mux.HandleFunc("POST /_hx/review/next", gated(gate, secure, s.next))
	mountPlacement(mux, tokens, gate, secure, s.deck, s.runs)
	s.mountShell(mux, tokens, gate, secure)
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
// route exists. frame-ancestors none is part of the CSP. script-src is
// 'self' plus the sha256 of the former inline theme boot (ThemeBootCSP).
// That boot is now /static/js/theme.js, which 'self' already allows. The
// hash stays in the header so this policy string does not change.
func securityHeaders(next http.Handler) http.Handler {
	policy := "default-src 'self'; script-src 'self' '" + views.ThemeBootCSP + "'; style-src 'self'; img-src 'self'; font-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", policy)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// same-origin, not no-referrer: under no-referrer a browser sends "Origin: null" on a native
		// form POST, and the CSRF check refuses that on purpose. same-origin sends the real Origin to
		// this site and sends no referrer at all to any other site, so nothing leaks.
		w.Header().Set("Referrer-Policy", "same-origin")
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

// homeFrom fills Today from the signed-in learner.
// With no learner, or no store, the page is the empty real state: nothing
// due, no next unit, no known words. The placement card stays off unless
// a run reader has a row.
func (s *server) homeFrom(r *http.Request, lang, csrf string) (views.Home, error) {
	vm, gender, situation, err := s.loadHome(r)
	if err != nil {
		return views.Home{}, err
	}
	return homeToView(lang, csrf, gender, situation, vm), nil
}

func (s *server) serveHome(w http.ResponseWriter, r *http.Request, secure bool, csrf string) {
	lang := chooseLang(w, r, secure)
	if s == nil || s.store == nil {
		page := homeToView(lang, csrf, views.AddressMasculine, "", HomeVM{})
		if s == nil || s.runs == nil {
			// No placement store: the card stays off, as on the ungated shell.
			page.Placement = &views.PlacementVM{}
		} else {
			state, err := todayPlacement(r, s.runs)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			if state != nil {
				fillPlacement(&page, &PlacementVM{
					Active:              state.Active,
					Finished:            state.Finished,
					CardsRated:          state.CardsRated,
					Level:               state.Level,
					EstimatedKnownWords: state.EstimatedKnownWords,
					TakenAt:             state.TakenAt,
				})
			}
		}
		s.fillToday(r, &page)
		render(w, r, views.Today(page))
		return
	}
	page, err := s.homeFrom(r, lang, csrf)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err := s.applyCourseLesson(r, &page); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	s.fillToday(r, &page)
	render(w, r, views.Today(page))
}

// applyCourseLesson puts the next published lesson in this learner's scope
// on the Today card. The due count, known count, and placement stay on the
// scheduler's page. No lesson leaves the empty card.
func (s *server) applyCourseLesson(r *http.Request, home *views.Home) error {
	if home == nil || s == nil || s.store == nil {
		return nil
	}
	user, ok := auth.CurrentUser(r.Context())
	if !ok || user == nil {
		home.Lesson = false
		return nil
	}
	lesson, found, err := s.store.NextCourseLesson(r.Context(), user.ID)
	if err != nil {
		return err
	}
	if !found {
		home.Lesson = false
		home.Words = nil
		return nil
	}
	gloss, err := titleGloss(r.Context(), s.store, lesson.Title)
	if err != nil {
		return err
	}
	home.Lesson = true
	home.LessonTitle = lesson.Title
	home.LessonGloss = gloss
	home.LessonHref = lessonPath(lesson.UnitID, lesson.LessonIx)
	home.UI.Situation = lesson.Situation
	home.Words = chipsFor(lesson.NewWords)
	// A lesson the learner has answered something in is one to continue, not to start.
	run, started, err := s.store.InProgressLessonRun(r.Context(), user.ID, lesson.UnitID, lesson.LessonIx)
	if err != nil {
		return err
	}
	if started {
		total := max(len(lesson.Activities), 1)
		home.Today.Step = min(run.ActivityIx+1, total)
		home.Today.Steps = total
	}
	return nil
}

func (s *server) serveReview(w http.ResponseWriter, r *http.Request, secure bool, csrf string) {
	lang := chooseLang(w, r, secure)
	card, err := s.currentReview(r, lang, csrf)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	gender := views.AddressMasculine
	if user, ok := auth.CurrentUser(r.Context()); ok {
		gender, err = s.addressGender(r.Context(), user.ID)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
	ui := views.UIFor(lang, gender)
	page := views.Page{UI: ui, Title: ui.DocTitle("review"), CSRF: csrf, Current: "review"}
	render(w, r, views.Review(page, card))
}

func (s *server) currentReview(r *http.Request, lang, csrf string) (views.ReviewCard, error) {
	lede := views.UIFor(lang, views.AddressMasculine).ReviewLede
	key, user, ok := s.sessionKey(r)
	if !ok || user == nil {
		return reviewView(lang, csrf, lede, ReviewCardVM{Done: true}), nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	q, err := s.queueLocked(r.Context(), key, user.ID)
	if err != nil {
		return views.ReviewCard{}, err
	}
	vm, err := s.cardVM(r.Context(), q)
	if err != nil {
		return views.ReviewCard{}, err
	}
	card := reviewView(lang, csrf, lede, vm)
	if err := s.attachReviewClips(r.Context(), &card); err != nil {
		return views.ReviewCard{}, err
	}
	return card, nil
}

// reveal records that this serve was shown the answer, then returns the
// back and the grade buttons. CSRF and Origin are the checks on /_hx/.
func (s *server) reveal(w http.ResponseWriter, r *http.Request) {
	cardID, ok := reviewCardID(w, r)
	if !ok {
		return
	}
	key, user, ok := s.sessionKey(r)
	if !ok || user == nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	lang := reviewLang(r)
	s.mu.Lock()
	q, err := s.queueLocked(r.Context(), key, user.ID)
	if err != nil {
		s.mu.Unlock()
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	// A checked activity is answered through /_hx/review/answer, never self-graded.
	if q == nil || q.sess == nil || !q.has || q.cur.CardID != cardID || q.item != nil {
		s.mu.Unlock()
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if err := q.sess.Reveal(cardID); err != nil {
		s.mu.Unlock()
		http.Error(w, http.StatusText(reviewStatus(err)), reviewStatus(err))
		return
	}
	q.revealed = true
	vm, err := s.cardVM(r.Context(), q)
	s.mu.Unlock()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	s.finishReview(w, r, lang, vm)
}

// grade records the served card and returns the next card, or Done.
// Session.Record writes the FSRS review through ApplyReview, once per serve.
// peeked is true only when this serve was revealed. A grade with no reveal
// is 400 and writes nothing. A drill step has no card, so Record keeps it
// in the queue only, after Reveal. CSRF and Origin are the checks on /_hx/
// in front of this handler. A database error is 500. An unknown card, or a
// card that was not revealed, is 400. A plain form post, including Reveal,
// is a 303 back to /review.
func (s *server) grade(w http.ResponseWriter, r *http.Request) {
	cardID, ok := reviewCardID(w, r)
	if !ok {
		return
	}
	rating, err := strconv.Atoi(r.PostForm.Get("rating"))
	if err != nil || rating < int(fsrs.Again) || rating > int(fsrs.Easy) {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	key, user, ok := s.sessionKey(r)
	if !ok || user == nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	lang := reviewLang(r)
	s.mu.Lock()
	q := s.queues[key]
	if q == nil || q.userID != user.ID || q.sess == nil || q.done || !q.has || q.cur.CardID != cardID || q.item != nil || !sameStudyDay(q.builtAt, s.now()) {
		s.mu.Unlock()
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	// A positive id that belongs to someone else is not the card this
	// session served. The id check above rejects it. Record then applies
	// the review once; a second grade of the same serve does not match.
	if err := q.sess.Record(r.Context(), cardID, fsrs.Rating(rating)); err != nil {
		s.mu.Unlock()
		status := reviewStatus(err)
		http.Error(w, http.StatusText(status), status)
		return
	}
	q.answered++
	if rating >= int(fsrs.Good) {
		q.correct++
	}
	if err := s.advance(r.Context(), q); err != nil {
		s.mu.Unlock()
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	vm, err := s.cardVM(r.Context(), q)
	s.mu.Unlock()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	s.finishReview(w, r, lang, vm)
}

func reviewCardID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return 0, false
	}
	cardID, err := strconv.ParseInt(r.PostForm.Get("card_id"), 10, 64)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return 0, false
	}
	return cardID, true
}

// finishReview returns the card partial for htmx. A plain form post gets the
// full Review page on the next GET, not the partial by itself.
func (s *server) finishReview(w http.ResponseWriter, r *http.Request, lang string, vm ReviewCardVM) {
	if r.Header.Get("HX-Request") != "true" {
		loc := "/review"
		if lang == "he" {
			loc = "/review?lang=he"
		}
		http.Redirect(w, r, loc, http.StatusSeeOther)
		return
	}
	lede := views.UIFor(lang, views.AddressMasculine).ReviewLede
	card := reviewView(lang, auth.CSRFToken(r), lede, vm)
	if err := s.attachReviewClips(r.Context(), &card); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	render(w, r, views.ReviewBody(card))
}

func reviewLang(r *http.Request) string {
	if c, err := r.Cookie(langCookieName); err == nil && (c.Value == "he" || c.Value == "en") {
		return c.Value
	}
	return "en"
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
