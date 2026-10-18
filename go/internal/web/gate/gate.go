// Package gate stands in for auth.Middleware, auth.RequireSession, and
// httpx.Protector / httpx.CSRFToken from draft PR #36.
//
// Do not import go/internal/auth or go/internal/httpx until that PR merges.
// The dev mount wraps POST /_hx/ as:
//
//	gate.Middleware(gate.RequireSession(gate.Protector(handler)))
//
// which is the same order as auth.Middleware outside the protector.
// The real protector also checks Origin. This stand-in checks the CSRF token.
package gate

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/library"
)

type ctxKey int

const (
	userKey ctxKey = iota
	tokenKey
)

// DevToken is the CSRF token the dev session puts on the request.
// httpx.CSRFToken will replace CSRFToken once PR #36 has merged.
const DevToken = "dev-csrf-token"

// Middleware attaches the dev user and CSRF token. It does not read a cookie.
// auth.Middleware(conn, cookieSecure) replaces it.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), userKey, library.DemoUserID)
		ctx = context.WithValue(ctx, tokenKey, DevToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserID is the user Middleware stored, or 0.
func UserID(r *http.Request) int64 {
	id, _ := r.Context().Value(userKey).(int64)
	return id
}

// CSRFToken is the stand-in for httpx.CSRFToken.
func CSRFToken(r *http.Request) string {
	token, _ := r.Context().Value(tokenKey).(string)
	return token
}

// RequireSession rejects a request with no user. HTML gets 302 to /login/;
// anything else gets 401 {"error":"Unauthorized","code":"no_session"}.
func RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserID(r) <= 0 {
			if wantsHTML(r) {
				http.Redirect(w, r, "/login/?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
				return
			}
			writeCode(w, http.StatusUnauthorized, "Unauthorized", "no_session")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Protector rejects a non-GET whose X-CSRF-Token (or csrf form field) does
// not match CSRFToken. The body is 403 {"error":"Forbidden","code":"csrf"}.
func Protector(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		_ = r.ParseForm()
		got := r.Header.Get("X-CSRF-Token")
		if got == "" {
			got = r.PostFormValue("csrf")
		}
		want := CSRFToken(r)
		if want == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			writeCode(w, http.StatusForbidden, "Forbidden", "csrf")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func wantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if accept == "" || strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*") {
		return r.Header.Get("HX-Request") != "true"
	}
	return false
}

func writeCode(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message, "code": code})
}
