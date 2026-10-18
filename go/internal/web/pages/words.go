package pages

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/gate"
	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/library"
	wordview "github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/views/pages"
)

// Words serves GET /words/ and POST /_hx/words/{key}/suspend.
// There is no database query here. Mount passes library.NewDemo until
// go/internal/learner/library is on this branch. Gloss and transliteration
// edits are not on this page.
type Words struct {
	lib     library.Lister
	shelf   library.Suspender
	user    func(*http.Request) (int64, error)
	token   func(*http.Request) string
	Session func(http.Handler) http.Handler
	Require func(http.Handler) http.Handler
	Protect func(http.Handler) http.Handler
}

// NewWords returns the words handler. A nil lister uses the fixed demo shelf.
// A lister that also implements Suspender gets a suspend button.
func NewWords(lib library.Lister) Words {
	if lib == nil {
		lib = library.Demo{}
	}
	h := Words{lib: lib, user: demoUser, token: gate.CSRFToken}
	if shelf, ok := lib.(library.Suspender); ok {
		h.shelf = shelf
	}
	return h
}

// UseDevGate installs the stand-ins for auth.Middleware, auth.RequireSession,
// and httpx.Protector. Middleware is the outermost wrapper.
func (h *Words) UseDevGate() {
	h.Session = gate.Middleware
	h.Require = gate.RequireSession
	h.Protect = gate.Protector
}

func demoUser(*http.Request) (int64, error) {
	return library.DemoUserID, nil
}

// Register mounts GET /words/ and POST /_hx/words/{key}/suspend.
// When UseDevGate was called, the POST handler is
// Middleware(RequireSession(Protector(suspend))).
func (h Words) Register(mux *http.ServeMux) {
	get := chain(http.HandlerFunc(h.serve), h.Session)
	mux.Handle("GET /words", get)
	mux.Handle("GET /words/{$}", get)
	mux.Handle("GET /words/index.html", get)
	mux.Handle("POST /_hx/words/{key}/suspend", chain(http.HandlerFunc(h.suspend), h.Session, h.Require, h.Protect))
}

func chain(h http.Handler, outer ...func(http.Handler) http.Handler) http.Handler {
	for i := len(outer) - 1; i >= 0; i-- {
		if outer[i] != nil {
			h = outer[i](h)
		}
	}
	return h
}

func (h Words) serve(w http.ResponseWriter, r *http.Request) {
	q, kind := parseWordsQuery(r)
	if kind != "" {
		h.write(w, r, http.StatusBadRequest, wordview.Assemble(q.filters, q.word, q.cursor, "error", kind, nil, ""))
		return
	}
	hx := r.Header.Get("HX-Request") == "true"
	if !hx && !q.canonical {
		http.Redirect(w, r, q.canonicalURL, http.StatusFound)
		return
	}
	uid, err := h.currentUser(r)
	if err != nil {
		h.write(w, r, http.StatusInternalServerError, wordview.Assemble(q.filters, q.word, q.cursor, "error", "load", nil, ""))
		return
	}
	items, next, err := h.lib.List(r.Context(), uid, q.libFilter, q.cursor, q.limit)
	if library.Is(err, "invalid_cursor") {
		// A bad cursor is not a failed load. Drop it and show the first page.
		q.cursor = ""
		q.canonicalURL = wordview.CanonicalPath(q.filters, q.word, "")
		q.canonical = false
		if !hx {
			http.Redirect(w, r, q.canonicalURL, http.StatusFound)
			return
		}
		items, next, err = h.lib.List(r.Context(), uid, q.libFilter, "", q.limit)
	}
	if err != nil {
		status := http.StatusInternalServerError
		kind := "load"
		if library.Is(err, "invalid_filter") {
			status = http.StatusBadRequest
			kind = "invalid_filter"
		}
		h.write(w, r, status, wordview.Assemble(q.filters, q.word, q.cursor, "error", kind, nil, ""))
		return
	}
	state := "ready"
	if len(items) == 0 {
		state = "empty"
	}
	view := wordview.Assemble(q.filters, q.word, q.cursor, state, "", items, next)
	h.decorate(&view, r, false)
	if r.Header.Get("X-Words-Sync") == "1" {
		view.SyncFilters = true
	}
	if hx && !q.canonical && !q.rowsOnly {
		w.Header().Set("HX-Push-Url", q.canonicalURL)
	}
	if hx && q.rowsOnly {
		writeHTML(w, r, http.StatusOK, wordview.WordsRows(view), wordview.WordsRows(view))
		return
	}
	h.write(w, r, http.StatusOK, view)
}

func (h Words) write(w http.ResponseWriter, r *http.Request, status int, view wordview.WordsView) {
	writeHTML(w, r, status, wordview.WordsPage(view), wordview.WordsFragment(view))
}

// writeNoSession answers an expired session with 401. A plain browser
// navigation still goes to the login page.
func writeNoSession(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized", "code": "no_session"})
		return
	}
	http.Redirect(w, r, "/login/?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
}

// currentUser prefers the id auth.Middleware stored. demoUser remains until
// that middleware is the one on the request.
func (h Words) currentUser(r *http.Request) (int64, error) {
	if id := gate.UserID(r); id > 0 {
		return id, nil
	}
	if h.user == nil {
		return 0, errors.New("no user")
	}
	return h.user(r)
}

// decorate fills the suspend control. blocked is the 404 not_found state:
// the word is personal-only and has no review row, so pause is disabled.
func (h Words) decorate(view *wordview.WordsView, r *http.Request, blocked bool) {
	if view == nil || view.Selected == nil || h.shelf == nil {
		return
	}
	d := view.Selected
	d.CanSuspend = true
	token := ""
	if h.token != nil {
		token = h.token(r)
	}
	d.CSRF = token
	raw, err := json.Marshal(map[string]string{"X-CSRF-Token": token})
	if err != nil {
		raw = []byte(`{"X-CSRF-Token":""}`)
	}
	d.CSRFHeader = string(raw)
	d.PostURL = "/_hx/words/" + url.PathEscape(d.Word) + "/suspend"
	// Neither direction means there is no review row. The card already
	// knows, so the lock note replaces the pause button before a POST.
	if !d.HeEn.Show && !d.EnHe.Show {
		blocked = true
	}
	if blocked {
		d.Blocked = true
		d.SuspendValue = "1"
		d.SuspendHe, d.SuspendEn = "השהו חזרות", "Pause reviews"
		return
	}
	if d.Suspended {
		d.SuspendValue = "0"
		d.SuspendHe, d.SuspendEn = "המשיכו חזרות", "Resume reviews"
		return
	}
	d.SuspendValue = "1"
	d.SuspendHe, d.SuspendEn = "השהו חזרות", "Pause reviews"
}

// suspend is POST /_hx/words/{key}/suspend. The action field is pause, because
// the included filter form already has a control named suspended.
// pause=1 suspends and pause=0 resumes. A 404 not_found redraws the card
// with the control disabled. The protector answers 403 csrf before this runs.
func (h Words) suspend(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.write(w, r, http.StatusBadRequest, wordview.Assemble(wordview.Filters{}, "", "", "error", "invalid_filter", nil, ""))
		return
	}
	filters, kind := filtersFromForm(r)
	if kind != "" {
		h.write(w, r, http.StatusBadRequest, wordview.Assemble(filters, "", "", "error", kind, nil, ""))
		return
	}
	key := r.PathValue("key")
	on, ok := pauseOn(r.PostFormValue("pause"))
	if !ok {
		h.write(w, r, http.StatusBadRequest, wordview.Assemble(filters, "", "", "error", "invalid_filter", nil, ""))
		return
	}
	if h.shelf == nil {
		h.write(w, r, http.StatusInternalServerError, wordview.Assemble(filters, "", "", "error", "load", nil, ""))
		return
	}
	uid, err := h.currentUser(r)
	if err != nil || uid <= 0 {
		writeNoSession(w, r)
		return
	}
	parsed, limit, err := library.ParseQuery(filterMap(filters))
	if err != nil {
		h.write(w, r, http.StatusBadRequest, wordview.Assemble(filters, "", "", "error", "invalid_filter", nil, ""))
		return
	}
	if _, err = h.shelf.SetSuspended(r.Context(), uid, key, on); err != nil {
		if library.Is(err, "not_found") {
			h.writeNotFound(w, r, uid, filters, parsed, limit, key)
			return
		}
		status := http.StatusInternalServerError
		kind := "load"
		if library.Is(err, "invalid_key") || library.Is(err, "bad_request") {
			status = http.StatusBadRequest
			kind = "invalid_filter"
		}
		h.write(w, r, status, wordview.Assemble(filters, "", "", "error", kind, nil, ""))
		return
	}
	items, next, err := h.lib.List(r.Context(), uid, parsed, "", limit)
	if err != nil {
		h.write(w, r, http.StatusInternalServerError, wordview.Assemble(filters, key, "", "error", "load", nil, ""))
		return
	}
	state := "ready"
	if len(items) == 0 {
		state = "empty"
	}
	view := wordview.Assemble(filters, key, "", state, "", items, next)
	if view.Selected == nil {
		if item, ok := h.recoverHidden(r.Context(), uid, parsed, key); ok {
			wordview.SelectItem(&view, item)
		}
	}
	if view.Selected != nil {
		view.Selected.FocusAction = true
	}
	h.decorate(&view, r, false)
	if r.Header.Get("HX-Request") != "true" {
		http.Redirect(w, r, wordview.CanonicalPath(filters, key, ""), http.StatusSeeOther)
		return
	}
	w.Header().Set("HX-Push-Url", wordview.CanonicalPath(filters, key, ""))
	h.write(w, r, http.StatusOK, view)
}

// recoverHidden finds a word the active filter just dropped, so the detail
// stays open and the button can undo the pause.
func (h Words) recoverHidden(ctx context.Context, uid int64, parsed library.Filter, key string) (library.Item, bool) {
	parsed.Suspended = nil
	cursor := ""
	for page := 0; page < 4; page++ {
		items, next, err := h.lib.List(ctx, uid, parsed, cursor, library.PageSize)
		if err != nil {
			return library.Item{}, false
		}
		for _, item := range items {
			if item.Word == key {
				return item, true
			}
		}
		if next == "" {
			break
		}
		cursor = next
	}
	return library.Item{}, false
}

func (h Words) writeNotFound(w http.ResponseWriter, r *http.Request, uid int64, filters wordview.Filters, parsed library.Filter, limit int, key string) {
	items, next, err := h.lib.List(r.Context(), uid, parsed, "", limit)
	if err != nil {
		h.write(w, r, http.StatusInternalServerError, wordview.Assemble(filters, key, "", "error", "load", nil, ""))
		return
	}
	state := "ready"
	if len(items) == 0 {
		state = "empty"
	}
	view := wordview.Assemble(filters, key, "", state, "", items, next)
	if view.Selected == nil {
		detail := wordview.WordDetail{Word: key, CloseURL: wordview.CanonicalPath(filters, "", "")}
		view.Selected = &detail
		view.SelectedKey = key
		view.Word = key
	}
	if view.Selected != nil {
		view.Selected.FocusAction = true
	}
	h.decorate(&view, r, true)
	h.write(w, r, http.StatusNotFound, view)
}

func filtersFromForm(r *http.Request) (wordview.Filters, string) {
	var f wordview.Filters
	for _, key := range wordview.FilterKeys {
		value, kind := canonicalFilter(key, r.FormValue(key))
		if kind != "" {
			return f, kind
		}
		f = f.With(key, value)
	}
	return f, ""
}

func pauseOn(raw string) (bool, bool) {
	switch raw {
	case "1", "true":
		return true, true
	case "0", "false":
		return false, true
	default:
		return false, false
	}
}

type wordsQuery struct {
	filters      wordview.Filters
	libFilter    library.Filter
	limit        int
	word, cursor string
	canonicalURL string
	canonical    bool
	rowsOnly     bool
}

func parseWordsQuery(r *http.Request) (wordsQuery, string) {
	values := r.URL.Query()
	var q wordsQuery
	for _, key := range wordview.FilterKeys {
		raw := values.Get(key)
		value, kind := canonicalFilter(key, raw)
		if kind != "" {
			return q, kind
		}
		q.filters = q.filters.With(key, value)
	}
	q.word = values.Get(wordview.QueryWord)
	if q.word != "" {
		if !utf8.ValidString(q.word) || utf8.RuneCountInString(q.word) > 80 || hasControl(q.word) {
			return q, "invalid_filter"
		}
	}
	q.cursor = values.Get(wordview.QueryCursor)
	if values.Get(wordview.QueryFragment) == wordview.FragmentRows {
		q.rowsOnly = true
	}
	parsed, limit, err := library.ParseQuery(filterMap(q.filters))
	if err != nil {
		return q, "invalid_filter"
	}
	q.libFilter = parsed
	q.limit = limit
	q.canonicalURL = wordview.CanonicalPath(q.filters, q.word, q.cursor)
	q.canonical = r.URL.RawQuery == wordview.CanonicalQuery(q.filters, q.word, q.cursor)
	return q, ""
}

func filterMap(f wordview.Filters) map[string]string {
	m := map[string]string{}
	put := func(key, value string) {
		if value != "" {
			m[key] = value
		}
	}
	put("q", f.Q)
	put("status", f.Status)
	put("weak", f.Weak)
	put("pos", f.POS)
	put("binyan", f.Binyan)
	put("tense", f.Tense)
	put("source", f.Source)
	put("suspended", f.Suspended)
	return m
}

func canonicalFilter(key, raw string) (string, string) {
	if strings.TrimSpace(raw) == "" {
		return "", ""
	}
	if !utf8.ValidString(raw) || hasControl(raw) || strings.ContainsAny(raw, "<>") {
		return "", "invalid_filter"
	}
	switch key {
	case "q":
		if utf8.RuneCountInString(raw) > 80 {
			return "", "invalid_filter"
		}
		return raw, ""
	case "status":
		if raw != "new" && raw != "learning" && raw != "longTerm" {
			return "", "invalid_filter"
		}
		return raw, ""
	case "weak":
		switch raw {
		case "1", "true":
			return "1", ""
		case "0", "false":
			return "", ""
		case "weak", "very_weak":
			return raw, ""
		default:
			return "", "invalid_filter"
		}
	case "pos":
		if !wordview.KnownPOS(raw) {
			return "", "invalid_filter"
		}
		return raw, ""
	case "binyan":
		if !wordview.KnownBinyan(raw) {
			return "", "invalid_filter"
		}
		return raw, ""
	case "tense":
		if !wordview.KnownTense(raw) {
			return "", "invalid_filter"
		}
		return raw, ""
	case "source":
		if raw != "lesson" && raw != "self" && raw != "ai" {
			return "", "invalid_filter"
		}
		return raw, ""
	case "suspended":
		switch raw {
		case "1", "true":
			return "1", ""
		case "0", "false":
			return "0", ""
		default:
			return "", "invalid_filter"
		}
	default:
		return "", "invalid_filter"
	}
}

func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f || unicode.Is(unicode.Cc, r) {
			return true
		}
	}
	return false
}
