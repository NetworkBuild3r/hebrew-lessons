package auth

import (
	"net/url"
	"strings"
)

const (
	// CookieName is the session cookie when COOKIE_SECURE is not 1.
	CookieName = "hebrew_session"
	// HostCookieName is the session cookie when COOKIE_SECURE=1.
	HostCookieName = "__Host-hebrew_session"
)

// ReadSessionToken returns the raw session token from a Cookie header.
// Both names are accepted (ADR-0016). When cookieSecure is set, the
// __Host- name wins if both are present; otherwise hebrew_session wins.
// An empty value is treated as absent. Parsing matches server/http/cookies.js:
// first occurrence wins, one layer of double quotes is stripped, and
// decodeURIComponent is applied.
func ReadSessionToken(header string, cookieSecure bool) string {
	cookies := parseCookies(header)
	primary, secondary := CookieName, HostCookieName
	if cookieSecure {
		primary, secondary = HostCookieName, CookieName
	}
	if value := cookies[primary]; value != "" {
		return value
	}
	return cookies[secondary]
}

func parseCookies(header string) map[string]string {
	out := map[string]string{}
	if header == "" {
		return out
	}
	for _, part := range strings.Split(header, ";") {
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			continue
		}
		name := strings.TrimSpace(part[:eq])
		if name == "" {
			continue
		}
		if _, exists := out[name]; exists {
			continue
		}
		value := strings.TrimSpace(part[eq+1:])
		if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = value[1 : len(value)-1]
		}
		decoded, err := url.PathUnescape(value)
		if err != nil {
			out[name] = value
			continue
		}
		out[name] = decoded
	}
	return out
}
