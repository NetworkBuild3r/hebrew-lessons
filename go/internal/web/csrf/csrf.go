// Package csrf checks HTMX writes the way ADR-0016 describes them.
// A mutating /_hx/ request needs an Origin (or Referer) that matches this
// request's origin, and a per-browser token compared in constant time.
// Sessions are not in this server yet. The token lives in a SameSite=Lax
// cookie that this package sets, which is the stand-in for the session token.
package csrf

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
)

const cookieName = "hebrew_csrf"

// ErrForbidden is an origin or token mismatch. Callers answer 403.
var ErrForbidden = errors.New("csrf: forbidden")

// Issue returns the browser token, setting the cookie when it is missing.
func Issue(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(cookieName); err == nil && validToken(c.Value) {
		return c.Value
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	tok := hex.EncodeToString(buf)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
	return tok
}

// Check allows a write only when the origin matches and the token matches.
// GET must not call Check; this package does not mutate on read.
func Check(r *http.Request) error {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return ErrForbidden
	}
	if !originOK(r) {
		return ErrForbidden
	}
	c, err := r.Cookie(cookieName)
	if err != nil || !validToken(c.Value) {
		return ErrForbidden
	}
	sent := r.Header.Get("X-CSRF-Token")
	if sent == "" {
		sent = r.FormValue("csrf")
	}
	if !match(c.Value, sent) {
		return ErrForbidden
	}
	return nil
}

func validToken(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func match(a, b string) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func originOK(r *http.Request) bool {
	got := requestOrigin(r)
	if got == "" {
		return false
	}
	return got == siteOrigin(r)
}

func requestOrigin(r *http.Request) string {
	if o := r.Header.Get("Origin"); o != "" {
		u, err := url.Parse(o)
		if err != nil || u.Host == "" || u.Scheme == "" {
			return ""
		}
		return u.Scheme + "://" + u.Host
	}
	ref := r.Header.Get("Referer")
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil || u.Host == "" || u.Scheme == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func siteOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
