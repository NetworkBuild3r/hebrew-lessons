package pages

import (
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/library"
	wordview "github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/views/pages"
)

// Words serves GET /words/ from a Lister. There is no database query here.
// Mount passes library.Demo until go/internal/learner/library is on this
// branch. The types match that package, so the swap is the import path.
// Writes (add, edit, suspend) are a later PR.
type Words struct {
	lib  library.Lister
	user func(*http.Request) (int64, error)
}

// NewWords returns the read-only words handler. A nil lister uses the demo fixture.
func NewWords(lib library.Lister) Words {
	if lib == nil {
		lib = library.Demo{}
	}
	return Words{lib: lib, user: demoUser}
}

func demoUser(*http.Request) (int64, error) {
	return library.DemoUserID, nil
}

// Register mounts GET /words/ and the slash-less and index.html aliases.
func (h Words) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /words", h.serve)
	mux.HandleFunc("GET /words/{$}", h.serve)
	mux.HandleFunc("GET /words/index.html", h.serve)
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
	uid, err := h.user(r)
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
