// Package httpx checks HTMX writes under /_hx/.
//
// A non-GET request under that prefix needs both of these, or the response
// is 403:
//
//   - Origin, or Referer when Origin is absent, matches PUBLIC_ORIGIN or the
//     host origin.
//   - A per-session CSRF token, sent as X-CSRF-Token or a csrf form field,
//     compared in constant time.
//
// GET is passed through and this package has no mutating handler.
//
// Host origin follows ADR-0016's trusted-proxy rules, which Node's
// server/http/security.js does not implement (there is no trusted-proxies.js
// in this tree). The rightmost X-Forwarded-Proto is used only when the
// socket peer is in TRUSTED_PROXIES. CF-Connecting-IP is the client address
// only in that same case. Otherwise the socket address is the client and
// X-Forwarded-Proto is ignored. X-Forwarded-For is never read.
package httpx

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/auth"
)

const (
	// hxPrefix is the path prefix whose non-GET requests require the checks.
	hxPrefix = "/_hx/"
	// maxFormBytes bounds a csrf form body. The token is also accepted as a
	// header, which does not read the body. 1 MiB stays under the largest
	// JSON body Node allows for ordinary writes (ADR-0016).
	maxFormBytes = 1 << 20
)

// Config is the process settings the /_hx/ checks need.
type Config struct {
	// PublicOrigin is PUBLIC_ORIGIN. One trailing slash is removed, matching
	// server/http/security.js. Empty means only the request host origin counts.
	PublicOrigin string
	// CookieSecure is COOKIE_SECURE=1. It selects https as the default
	// scheme when X-Forwarded-Proto is not trusted.
	CookieSecure bool
	// TrustedProxies is the TRUSTED_PROXIES list. Empty trusts nobody.
	TrustedProxies []net.IPNet
	// Key is CSRF_KEY. Nil or empty generates a random key and logs that
	// tokens reset on restart.
	Key []byte
	// Logf receives the startup warning. Nil uses log.Printf.
	Logf func(format string, args ...any)
}

// Protector is the process-wide CSRF key and origin policy.
type Protector struct {
	publicOrigin   string
	cookieSecure   bool
	trustedProxies []net.IPNet
	key            []byte
}

// New loads the CSRF key. A missing key is generated with crypto/rand and a
// warning is logged: tokens reset on restart.
func New(cfg Config) (*Protector, error) {
	key := append([]byte(nil), cfg.Key...)
	if len(bytesTrimSpace(key)) == 0 {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		logf := cfg.Logf
		if logf == nil {
			logf = log.Printf
		}
		logf("warning: CSRF_KEY is unset; generated a random key; tokens reset on restart")
	}
	return &Protector{
		publicOrigin:   stripOneSlash(cfg.PublicOrigin),
		cookieSecure:   cfg.CookieSecure,
		trustedProxies: append([]net.IPNet(nil), cfg.TrustedProxies...),
		key:            key,
	}, nil
}

// Handler attaches the protector to the request and enforces the /_hx/ checks.
// Put auth.Middleware outside this handler so the session token hash is
// already on the context. GET, including GET /_hx/, is not checked and this
// handler does not write.
func (p *Protector) Handler(next http.Handler) http.Handler {
	if p == nil {
		panic("httpx: nil protector")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), protectorContextKey{}, p))
		if !needsCSRF(r) {
			next.ServeHTTP(w, r)
			return
		}
		if !p.allow(r) {
			writeForbidden(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func needsCSRF(r *http.Request) bool {
	if r.Method == http.MethodGet {
		return false
	}
	path := r.URL.Path
	return path == "/_hx" || strings.HasPrefix(path, hxPrefix)
}

func (p *Protector) allow(r *http.Request) bool {
	if !OriginAllowed(r, p.publicOrigin, p.trustedProxies, p.cookieSecure) {
		return false
	}
	want := CSRFToken(r)
	if want == "" {
		return false
	}
	got, ok := providedToken(r)
	if !ok {
		return false
	}
	return tokensEqual(got, want)
}

// CSRFToken is the per-session token for templates.
// It is hex(HMAC-SHA256(CSRF_KEY, session token hash)), where the session
// token hash is the SHA-256 hex stored in sessions.token_hash.
// The result is empty when the protector or the session hash is missing,
// so a signed-out page does not embed a usable token.
func CSRFToken(r *http.Request) string {
	if r == nil {
		return ""
	}
	p := protectorFrom(r.Context())
	hash := auth.TokenHashFromContext(r.Context())
	if p == nil || hash == "" {
		return ""
	}
	mac := hmac.New(sha256.New, p.key)
	_, _ = mac.Write([]byte(hash))
	return hex.EncodeToString(mac.Sum(nil))
}

func providedToken(r *http.Request) (string, bool) {
	if v := r.Header.Get("X-CSRF-Token"); v != "" {
		return v, true
	}
	if err := readForm(r); err != nil {
		return "", false
	}
	v := r.PostFormValue("csrf")
	if v == "" {
		return "", false
	}
	return v, true
}

func readForm(r *http.Request) error {
	// A nil ResponseWriter keeps MaxBytesReader from writing its own 413.
	// A body over the limit fails this check, and the caller returns 403.
	if r.Body != nil {
		r.Body = http.MaxBytesReader(nil, r.Body, maxFormBytes)
	}
	ct := r.Header.Get("Content-Type")
	media := strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	if media == "multipart/form-data" {
		return r.ParseMultipartForm(maxFormBytes)
	}
	return r.ParseForm()
}

func tokensEqual(got, want string) bool {
	gb := []byte(got)
	wb := []byte(want)
	if len(gb) != len(wb) {
		subtle.ConstantTimeCompare(wb, wb)
		return false
	}
	return subtle.ConstantTimeCompare(gb, wb) == 1
}

func writeForbidden(w http.ResponseWriter) {
	body := []byte(`{"error":"Forbidden","code":"csrf"}`)
	h := w.Header()
	if h.Get("Content-Type") == "" {
		h.Set("Content-Type", "application/json; charset=utf-8")
	}
	h.Set("Cache-Control", "no-store")
	if h.Get("X-Content-Type-Options") == "" {
		h.Set("X-Content-Type-Options", "nosniff")
	}
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write(body)
}

// RequestOrigin is Origin, with one trailing slash removed, or the Referer
// URL's origin when Origin is missing or the literal "null".
// It matches server/http/security.js requestOrigin. A foreign or missing
// value is ("", false).
func RequestOrigin(r *http.Request) (string, bool) {
	if r == nil {
		return "", false
	}
	origin := r.Header.Get("Origin")
	if origin != "" && origin != "null" {
		return stripOneSlash(origin), true
	}
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = r.Header.Get("Referrer")
	}
	if referer == "" {
		return "", false
	}
	return urlOrigin(referer)
}

// HostOrigin is scheme://host for the request.
// The scheme is the rightmost X-Forwarded-Proto value when the socket peer
// is in trusted. Otherwise, and when that header is absent, the scheme is
// https if cookieSecure and http if not. An empty Host is ("", false) as "".
func HostOrigin(r *http.Request, trusted []net.IPNet, cookieSecure bool) string {
	if r == nil {
		return ""
	}
	host := r.Host
	if host == "" {
		host = r.Header.Get("Host")
	}
	if host == "" {
		return ""
	}
	return ForwardedProto(r, trusted, cookieSecure) + "://" + host
}

// ForwardedProto is the scheme used for HostOrigin.
func ForwardedProto(r *http.Request, trusted []net.IPNet, cookieSecure bool) string {
	if r != nil && PeerTrusted(r.RemoteAddr, trusted) {
		if proto, ok := rightmostForwardedProto(r); ok {
			return proto
		}
	}
	if cookieSecure {
		return "https"
	}
	return "http"
}

// OriginAllowed reports whether RequestOrigin matches publicOrigin or the
// host origin. publicOrigin may still have one trailing slash; it is stripped.
// Missing or foreign origins are false, matching server/http/security.js
// originAllowed, except for the trusted-proxy scheme rule in HostOrigin.
func OriginAllowed(r *http.Request, publicOrigin string, trusted []net.IPNet, cookieSecure bool) bool {
	got, ok := RequestOrigin(r)
	if !ok || got == "" {
		return false
	}
	pub := stripOneSlash(publicOrigin)
	if pub != "" && got == pub {
		return true
	}
	host := HostOrigin(r, trusted, cookieSecure)
	return host != "" && got == host
}

// ClientIP is the client address for a request.
// When the socket peer is in trusted and CF-Connecting-IP is one IP, that
// header is returned. Otherwise the socket host is returned, without the
// port. X-Forwarded-For is not read.
func ClientIP(r *http.Request, trusted []net.IPNet) string {
	if r == nil {
		return ""
	}
	if PeerTrusted(r.RemoteAddr, trusted) {
		if ip := singleIP(r.Header.Get("CF-Connecting-IP")); ip != "" {
			return ip
		}
	}
	return socketHost(r.RemoteAddr)
}

// PeerTrusted reports whether remoteAddr is inside trusted.
// An empty list trusts nobody.
func PeerTrusted(remoteAddr string, trusted []net.IPNet) bool {
	if len(trusted) == 0 {
		return false
	}
	ip := socketIP(remoteAddr)
	return ip != nil && containsIP(trusted, ip)
}

func rightmostForwardedProto(r *http.Request) (string, bool) {
	vals := r.Header.Values("X-Forwarded-Proto")
	if len(vals) == 0 {
		return "", false
	}
	parts := strings.Split(strings.Join(vals, ","), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p != "" {
			return p, true
		}
	}
	return "", false
}

func urlOrigin(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	host := u.Host
	if name, port, err := net.SplitHostPort(host); err == nil {
		if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
			host = name
		}
	}
	return u.Scheme + "://" + host, true
}

func stripOneSlash(value string) string {
	return strings.TrimSuffix(value, "/")
}

func socketHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func socketIP(remoteAddr string) net.IP {
	return net.ParseIP(socketHost(remoteAddr))
}

func singleIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, ",") {
		return ""
	}
	if net.ParseIP(value) == nil {
		return ""
	}
	return value
}

func containsIP(nets []net.IPNet, ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

type protectorContextKey struct{}

func protectorFrom(ctx context.Context) *Protector {
	if ctx == nil {
		return nil
	}
	p, _ := ctx.Value(protectorContextKey{}).(*Protector)
	return p
}
