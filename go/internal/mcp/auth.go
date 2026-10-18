package mcp

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Bearer check from mcp/http.js. The static MCP_AUTH_TOKEN is compared as
// SHA-256 digests. An OAuth access token is accepted the same way Node's
// verifyAccessToken accepts one. The authorization-server routes are not
// served; those paths 404.

var bearerRE = regexp.MustCompile(`(?i)^Bearer\s+(\S+)$`)

type principal struct {
	userID   int64
	username string
	hasUser  bool
}

func (a *App) authenticate(header string) (principal, bool) {
	expected := os.Getenv("MCP_AUTH_TOKEN")
	if expected == "" {
		return principal{}, false
	}
	m := bearerRE.FindStringSubmatch(header)
	if m == nil {
		return principal{}, false
	}
	given := sha256.Sum256([]byte(m[1]))
	wanted := sha256.Sum256([]byte(expected))
	if subtle.ConstantTimeCompare(given[:], wanted[:]) == 1 {
		return principal{}, true
	}
	access := a.verifyAccessToken(m[1])
	if access == nil {
		return principal{}, false
	}
	if access.uid == nil {
		return principal{}, true
	}
	user, ok := a.activeUser(*access.uid)
	if !ok || (access.un != "" && user != access.un) {
		return principal{}, false
	}
	return principal{userID: *access.uid, username: user, hasUser: true}, true
}

type accessPayload struct {
	uid *int64
	un  string
	exp int64
}

func (a *App) verifyAccessToken(token string) *accessPayload {
	if !oauthEnabled() || token == "" {
		return nil
	}
	store := loadOAuthStore()
	h := tokenHash(token)
	if rec, ok := store.access[h]; ok && rec.exp > oauthNow() {
		return a.resolvePayload(rec)
	}
	payload := verifySigned(token, "access")
	if payload == nil {
		return nil
	}
	return a.resolvePayload(*payload)
}

func (a *App) resolvePayload(p accessPayload) *accessPayload {
	if p.uid == nil {
		return &p
	}
	user, ok := a.activeUser(*p.uid)
	if !ok || (p.un != "" && user != p.un) {
		return nil
	}
	p.un = user
	return &p
}

func (a *App) activeUser(id int64) (string, bool) {
	if a.db == nil {
		return "", false
	}
	var username, status string
	err := a.db.QueryRow(`SELECT username, status FROM users WHERE id = ?`, id).Scan(&username, &status)
	if err != nil || status != "active" {
		return "", false
	}
	return username, true
}

func oauthEnabled() bool {
	return oauthBase() != "" && os.Getenv("MCP_AUTH_TOKEN") != ""
}

func oauthBase() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("MCP_PUBLIC_URL")), "/")
}

func resourceMetadata() string {
	return `Bearer realm="hebrew-lessons", resource_metadata="` + oauthBase() + `/.well-known/oauth-protected-resource"`
}

func plainAuthenticate() string {
	return `Bearer realm="hebrew-lessons"`
}

func oauthKey() []byte {
	mac := hmac.New(sha256.New, []byte(os.Getenv("MCP_AUTH_TOKEN")))
	mac.Write([]byte("hebrew-mcp-oauth-v1"))
	return mac.Sum(nil)
}

func tokenHash(raw string) string {
	mac := hmac.New(sha256.New, oauthKey())
	mac.Write([]byte(raw))
	return hexEncode(mac.Sum(nil))
}

func hexEncode(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = hexdigits[c>>4]
		out[i*2+1] = hexdigits[c&0x0f]
	}
	return string(out)
}

func verifySigned(token, typ string) *accessPayload {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil
	}
	mac := hmac.New(sha256.New, oauthKey())
	mac.Write([]byte(parts[0]))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(got) != len(want) || subtle.ConstantTimeCompare(got, want) != 1 {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil
	}
	var body struct {
		T   string          `json:"t"`
		Exp float64         `json:"exp"`
		UID json.RawMessage `json:"uid"`
		UN  string          `json:"un"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil
	}
	if body.T != typ || !(int64(body.Exp) > oauthNow() || body.Exp > float64(oauthNow())) {
		return nil
	}
	out := &accessPayload{un: body.UN, exp: int64(body.Exp)}
	if len(body.UID) > 0 && string(body.UID) != "null" {
		var n float64
		if err := json.Unmarshal(body.UID, &n); err != nil {
			return nil
		}
		id := int64(n)
		out.uid = &id
	}
	return out
}

func oauthNow() int64 { return time.Now().Unix() }

type oauthStore struct {
	access map[string]accessPayload
}

func loadOAuthStore() oauthStore {
	out := oauthStore{access: map[string]accessPayload{}}
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		return out
	}
	raw, err := os.ReadFile(filepath.Join(dir, "mcp-oauth-state.json"))
	if err != nil {
		return out
	}
	var body struct {
		Access map[string]struct {
			Exp float64         `json:"exp"`
			UID json.RawMessage `json:"uid"`
			UN  string          `json:"un"`
		} `json:"access"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return out
	}
	for k, rec := range body.Access {
		p := accessPayload{un: rec.UN, exp: int64(rec.Exp)}
		if rec.Exp > float64(p.exp) {
			// keep the truncated second; Node compares exp > now() in seconds
		}
		if len(rec.UID) > 0 && string(rec.UID) != "null" {
			var n float64
			if err := json.Unmarshal(rec.UID, &n); err == nil {
				id := int64(n)
				p.uid = &id
			}
		}
		out.access[k] = p
	}
	return out
}
