package auth

import (
	"crypto/hmac"
	"net/http"
	"net/url"
	"strings"
)

// CSRFToken is the per-session token for templates and HTMX.
// It is empty when r has no session.
func CSRFToken(r *http.Request) string {
	if r == nil {
		return ""
	}
	sess, ok := sessionFrom(r.Context())
	if !ok {
		return ""
	}
	return sess.CSRF
}

func (s *Service) csrfOK(r *http.Request) bool {
	got, ok := requestOrigin(r)
	if !ok || got != s.origin {
		return false
	}
	sess, has := sessionFrom(r.Context())
	if !has || sess.CSRF == "" {
		return false
	}
	return tokensEqual(providedCSRF(r), sess.CSRF)
}

func requestOrigin(r *http.Request) (string, bool) {
	if raw := strings.TrimSpace(r.Header.Get("Origin")); raw != "" && !strings.EqualFold(raw, "null") {
		return parseAbsoluteOrigin(raw, false)
	}
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = r.Header.Get("Referrer")
	}
	if strings.TrimSpace(ref) == "" {
		return "", false
	}
	return parseAbsoluteOrigin(ref, true)
}

// parseAbsoluteOrigin returns scheme://host. allowPath is for a Referer,
// which has a path. An Origin header with a path or query is rejected.
func parseAbsoluteOrigin(raw string, allowPath bool) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.Opaque != "" {
		return "", false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	if !allowPath {
		if u.RawQuery != "" || u.User != nil || (u.Path != "" && u.Path != "/") {
			return "", false
		}
	}
	return scheme + "://" + strings.ToLower(u.Host), true
}

func canonicalOrigin(raw string) (string, bool) {
	return parseAbsoluteOrigin(strings.TrimRight(strings.TrimSpace(raw), "/"), false)
}

func providedCSRF(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-CSRF-Token")); v != "" {
		return v
	}
	if r.Body != nil {
		r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	}
	if err := r.ParseForm(); err != nil {
		return ""
	}
	return r.PostForm.Get("_csrf")
}

func tokensEqual(got, want string) bool {
	if got == "" || want == "" {
		return false
	}
	return hmac.Equal([]byte(got), []byte(want))
}

func csrfRequired(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	p := r.URL.Path
	if p == "/auth/logout" {
		return true
	}
	return p == "/_hx" || strings.HasPrefix(p, "/_hx/")
}
