package web

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
)

// CSRFToken is the slice of auth this shell renders and, in tests, checks.
// Production New passes CSRFFunc(auth.CSRFToken) and lets auth.Middleware
// check the token. This package does not implement sessions, and it does
// not call RequireUser.
type CSRFToken interface {
	CSRFToken(*http.Request) string
}

// CSRFFunc adapts a function such as auth.CSRFToken to CSRFToken.
// Tests pass a fake. Production New passes nil until that function exists.
type CSRFFunc func(*http.Request) string

// CSRFToken implements CSRFToken.
func (f CSRFFunc) CSRFToken(r *http.Request) string {
	if f == nil {
		return ""
	}
	return f(r)
}

// csrfHeader is the name htmx sends. csrfField is the no-JS form field,
// the same name auth.Middleware reads (_csrf). The header stays
// X-CSRF-Token so boosted requests do not change.
const (
	csrfHeader = "X-CSRF-Token"
	csrfField  = "_csrf"
)

// csrfMiddleware checks mutating requests for the X-CSRF-Token header or
// the _csrf form field, and for an Origin whose host matches the request.
// Safe methods pass through. A nil token source fails closed on writes.
//
// Production New does not stack this middleware under auth.Middleware.
// That middleware accepts a Referer fallback and does not require the
// Origin host to equal r.Host, so running both rejects a same-origin post
// behind a host-rewriting proxy. newMux still applies this check so tests
// can pass a fake token without a session.
func csrfMiddleware(tokens CSRFToken, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if !csrfOK(tokens, r) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func csrfOK(tokens CSRFToken, r *http.Request) bool {
	if !originOK(r) {
		return false
	}
	return tokenMatch(providedToken(r), csrfValue(tokens, r))
}

func providedToken(r *http.Request) string {
	if v := r.Header.Get(csrfHeader); v != "" {
		return v
	}
	if err := r.ParseForm(); err != nil {
		return ""
	}
	return r.PostForm.Get(csrfField)
}

func tokenMatch(got, want string) bool {
	if want == "" || len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func originOK(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" || strings.EqualFold(origin, "null") {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}
