package normalize

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

// Placeholders written into goldens. The original volatile bytes are not kept.
const (
	maskUUID      = "<uuid>"
	maskTimestamp = "<timestamp>"
	maskEpoch     = "<epoch>"
	maskToken     = "<token>"
)

var (
	uuidRE = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	timeRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$`)
	// UUIDs and timestamps inside a longer sentence (domain error text).
	uuidInTextRE = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	timeInTextRE = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`)
	hexTokenRE   = regexp.MustCompile(`^(?:[0-9a-fA-F]{32}|[0-9a-fA-F]{48}|[0-9a-fA-F]{64})$`)
	opaqueRE     = regexp.MustCompile(`^[A-Za-z0-9_-]{20,}$`)
)

// Prose and schema-contract keys are not rewritten. A description that quotes
// an id is part of the frozen text. Instance fields such as id and token are.
var exemptKeys = map[string]bool{
	"description": true,
	"title":       true,
	"name":        true,
	"enum":        true,
	"const":       true,
	"default":     true,
	"pattern":     true,
	"format":      true,
	"$schema":     true,
	"type":        true,
}

var epochKeys = map[string]bool{
	"exp":                      true,
	"iat":                      true,
	"nbf":                      true,
	"at":                       true,
	"timestamp":                true,
	"applied_at":               true,
	"expires_at":               true,
	"expiresAt":                true,
	"createdAt":                true,
	"updatedAt":                true,
	"created_at":               true,
	"updated_at":               true,
	"client_id_issued_at":      true,
	"client_secret_expires_at": true,
}

var tokenKeys = map[string]bool{
	"access_token":  true,
	"refresh_token": true,
	"id_token":      true,
	"client_secret": true,
	"token":         true,
	"authorization": true,
}

// Mask replaces volatile JSON values with stable placeholders.
// Numeric revision counters are left alone: a bare integer is ambiguous,
// and those goldens land with the domain-error cases.
func Mask(v any) any {
	return maskValue("", v)
}

// MaskString rewrites ids and timestamps inside a sentence.
func MaskString(s string) string {
	s = uuidInTextRE.ReplaceAllString(s, maskUUID)
	s = timeInTextRE.ReplaceAllString(s, maskTimestamp)
	return s
}

func maskValue(key string, v any) any {
	if exemptKeys[key] {
		return v
	}
	switch n := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(n))
		for k, child := range n {
			out[k] = maskValue(k, child)
		}
		return out
	case []any:
		if key == "enum" {
			return n
		}
		out := make([]any, len(n))
		for i, el := range n {
			out[i] = maskValue(key, el)
		}
		return out
	case string:
		return maskString(key, n)
	case float64, int, int64, json.Number:
		if epochKeys[key] {
			return maskEpoch
		}
		return n
	default:
		return n
	}
}

func maskString(key, s string) string {
	if epochKeys[key] && (timeRE.MatchString(s) || isAllDigits(s)) {
		if timeRE.MatchString(s) {
			return maskTimestamp
		}
		return maskEpoch
	}
	if tokenKeys[key] || (key == "code" && looksLikeToken(s)) {
		return maskToken
	}
	if uuidRE.MatchString(s) {
		return maskUUID
	}
	if timeRE.MatchString(s) {
		return maskTimestamp
	}
	// Domain messages quote an id inside a sentence. Exact-match above
	// handles a value that is only an id or a timestamp.
	return MaskString(s)
}

func looksLikeToken(s string) bool {
	if uuidRE.MatchString(s) || timeRE.MatchString(s) {
		return false
	}
	if hexTokenRE.MatchString(s) {
		return true
	}
	return opaqueRE.MatchString(s)
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// MaskHeaders drops hop-by-hop and proxy headers and hides cookie values.
// Cookie attributes (Path, HttpOnly, SameSite) stay.
func MaskHeaders(h http.Header) map[string]string {
	out := map[string]string{}
	for k, vals := range h {
		if dropHeader(k) {
			continue
		}
		joined := strings.Join(vals, "\n")
		if strings.EqualFold(k, "Set-Cookie") {
			parts := make([]string, len(vals))
			for i, v := range vals {
				parts[i] = maskCookie(v)
			}
			joined = strings.Join(parts, "\n")
		}
		out[k] = joined
	}
	return out
}

func dropHeader(k string) bool {
	switch strings.ToLower(k) {
	case "date", "connection", "keep-alive", "transfer-encoding", "content-length":
		return true
	}
	return strings.HasPrefix(strings.ToLower(k), "cf-")
}

func maskCookie(cookie string) string {
	name, rest, ok := strings.Cut(cookie, "=")
	if !ok {
		return cookie
	}
	value, attrs, hasAttrs := strings.Cut(rest, ";")
	if strings.TrimSpace(value) == "" {
		return cookie
	}
	if hasAttrs {
		return name + "=" + maskToken + ";" + attrs
	}
	return name + "=" + maskToken
}
