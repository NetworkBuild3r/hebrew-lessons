package auth

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// Middleware reads the session cookie and stores the user on the request
// context. HTML and JSON routes both use it. A missing or rejected token
// leaves the context without a user; RequireSession decides the response.
// Database errors are a 500 JSON body and the next handler is not called.
func Middleware(conn *sql.DB, cookieSecure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := ReadSessionToken(r.Header.Get("Cookie"), cookieSecure)
			if token != "" {
				user, err := Resolve(r.Context(), conn, token, time.Now())
				if err != nil {
					log.Printf("auth: %v", err)
					writeJSONError(w, http.StatusInternalServerError, "Error", "server_error")
					return
				}
				if user != nil {
					r = r.WithContext(NewContext(r.Context(), user, HashToken(token)))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireSession rejects a request that Middleware did not attach a user to.
// An HTML navigation (Accept contains text/html, or the path is empty, "/",
// ends in "/", or ends in ".html" — the same cases as server/web.js wantsHtml)
// is a 302 to /login/?next= with SafeNextPath applied to the path.
// Every other request is 401 JSON: {"error":"Unauthorized","code":"no_session"}.
func RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFromContext(r.Context()) == nil {
			if wantsHTML(r) {
				writeRedirect(w, "/login/?next="+encodeURIComponent(SafeNextPath(r.URL.Path)))
				return
			}
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized", "no_session")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func wantsHTML(r *http.Request) bool {
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		return true
	}
	path := r.URL.Path
	if path == "" || path == "/" || strings.HasSuffix(path, "/") || strings.HasSuffix(path, ".html") {
		return true
	}
	return false
}

func writeRedirect(w http.ResponseWriter, location string) {
	h := w.Header()
	h.Set("Location", location)
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Content-Length", "0")
	setSecurityHeaders(h)
	w.WriteHeader(http.StatusFound)
}

func writeJSONError(w http.ResponseWriter, status int, message, code string) {
	body, err := json.Marshal(struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}{Error: message, Code: code})
	if err != nil {
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	setSecurityHeaders(h)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func setSecurityHeaders(h http.Header) {
	if h.Get("X-Content-Type-Options") == "" {
		h.Set("X-Content-Type-Options", "nosniff")
	}
	if h.Get("Referrer-Policy") == "" {
		h.Set("Referrer-Policy", "same-origin")
	}
	if h.Get("X-Frame-Options") == "" {
		h.Set("X-Frame-Options", "DENY")
	}
}

// encodeURIComponent matches JavaScript encodeURIComponent for a path that
// is already Unicode text: every byte outside the unreserved set is %HH.
func encodeURIComponent(s string) string {
	const hexdigits = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isURIUnreserved(c) {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hexdigits[c>>4])
		b.WriteByte(hexdigits[c&0x0f])
	}
	return b.String()
}

func isURIUnreserved(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '-', '_', '.', '!', '~', '*', '\'', '(', ')':
		return true
	default:
		return false
	}
}
