package httpx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/auth"
)

func TestCSRFTable(t *testing.T) {
	key := []byte("test-csrf-key")
	proxies := mustCIDR(t, "10.0.0.0/8")
	p, err := New(Config{
		PublicOrigin:   "https://hebrew.example/",
		CookieSecure:   true,
		TrustedProxies: proxies,
		Key:            key,
	})
	if err != nil {
		t.Fatal(err)
	}
	hash := auth.HashToken("session-token")
	user := &auth.User{ID: 7, Role: "user"}
	wantToken := hmacHex(key, hash)

	var sawToken string
	protected := p.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawToken = CSRFToken(r)
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("X-Mutated", "1")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	call := func(method, path, origin, referer, token, remote, forwarded, form string) *httptest.ResponseRecorder {
		t.Helper()
		var body io.Reader
		if form != "" {
			body = strings.NewReader(form)
		}
		req := httptest.NewRequest(method, "https://hebrew.example"+path, body)
		req.Host = "hebrew.example"
		req.RemoteAddr = remote
		req = req.WithContext(auth.NewContext(req.Context(), user, hash))
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if referer != "" {
			req.Header.Set("Referer", referer)
		}
		if token != "" {
			req.Header.Set("X-CSRF-Token", token)
		}
		if forwarded != "" {
			req.Header.Set("X-Forwarded-Proto", forwarded)
		}
		if form != "" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		rec := httptest.NewRecorder()
		protected.ServeHTTP(rec, req)
		return rec
	}

	ok := call(http.MethodPost, "/_hx/library/suspend", "https://hebrew.example", "", wantToken, "10.1.2.3:443", "http, https", "")
	if ok.Code != http.StatusNoContent {
		t.Fatalf("valid post: %d %s", ok.Code, ok.Body.String())
	}
	if ok.Header().Get("X-Mutated") != "1" {
		t.Fatal("handler did not run")
	}
	if sawToken != wantToken {
		t.Fatalf("token %s want %s", sawToken, wantToken)
	}

	cases := []struct {
		name       string
		method     string
		path       string
		origin     string
		referer    string
		token      string
		remote     string
		forwarded  string
		form       string
		wantStatus int
	}{
		{name: "get skips checks", method: http.MethodGet, path: "/_hx/library", remote: "203.0.113.4:1234", wantStatus: http.StatusNoContent},
		{name: "head is not get", method: http.MethodHead, path: "/_hx/library", remote: "10.1.2.3:443", wantStatus: http.StatusForbidden},
		{name: "outside prefix", method: http.MethodPost, path: "/api/me/library", remote: "203.0.113.4:1234", wantStatus: http.StatusNoContent},
		{name: "missing origin", method: http.MethodPost, path: "/_hx/library", token: wantToken, remote: "10.1.2.3:443", wantStatus: http.StatusForbidden},
		{name: "foreign origin", method: http.MethodPost, path: "/_hx/library", origin: "https://evil.example", token: wantToken, remote: "10.1.2.3:443", wantStatus: http.StatusForbidden},
		{name: "referer fallback", method: http.MethodPost, path: "/_hx/library", referer: "https://hebrew.example/words/", token: wantToken, remote: "10.1.2.3:443", wantStatus: http.StatusNoContent},
		{name: "origin null uses referer", method: http.MethodPost, path: "/_hx/library", origin: "null", referer: "https://hebrew.example/words/", token: wantToken, remote: "10.1.2.3:443", wantStatus: http.StatusNoContent},
		{name: "bad token", method: http.MethodPost, path: "/_hx/library", origin: "https://hebrew.example", token: "00", remote: "10.1.2.3:443", wantStatus: http.StatusForbidden},
		{name: "form field", method: http.MethodPost, path: "/_hx/library", origin: "https://hebrew.example/", form: "csrf=" + wantToken, remote: "10.1.2.3:443", wantStatus: http.StatusNoContent},
		{name: "untrusted proto ignored", method: http.MethodPost, path: "/_hx/library", origin: "http://hebrew.example", token: wantToken, remote: "203.0.113.9:80", forwarded: "http", wantStatus: http.StatusForbidden},
		{name: "trusted rightmost proto", method: http.MethodPost, path: "/_hx/words", origin: "http://hebrew.example", token: wantToken, remote: "10.0.0.8:443", forwarded: "https, http", wantStatus: http.StatusNoContent},
		{name: "host origin when public differs", method: http.MethodPost, path: "/_hx/words", origin: "https://127.0.0.1:8090", token: wantToken, remote: "203.0.113.9:80", wantStatus: http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := call(tc.method, tc.path, tc.origin, tc.referer, tc.token, tc.remote, tc.forwarded, tc.form)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
			}
			if tc.wantStatus == http.StatusForbidden {
				if strings.TrimSpace(rec.Body.String()) != `{"error":"Forbidden","code":"csrf"}` {
					t.Fatalf("body %s", rec.Body.String())
				}
				if rec.Header().Get("X-Mutated") != "" {
					t.Fatal("rejected request mutated")
				}
			}
			if tc.method == http.MethodGet && rec.Header().Get("X-Mutated") != "" {
				t.Fatal("GET mutated")
			}
		})
	}
}

func TestGeneratedKeyWarns(t *testing.T) {
	var logged string
	if _, err := New(Config{Logf: func(format string, args ...any) {
		logged = format
	}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logged, "tokens reset on restart") {
		t.Fatalf("log %q", logged)
	}
}

func TestClientIPAndProto(t *testing.T) {
	trusted := mustCIDR(t, "10.0.0.0/8", "192.168.0.0/16")
	req := httptest.NewRequest(http.MethodGet, "http://hebrew.example/", nil)
	req.Host = "hebrew.example"
	req.RemoteAddr = "10.1.2.3:443"
	req.Header.Set("CF-Connecting-IP", "203.0.113.50")
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Forwarded-Proto", "http, https")
	if got := ClientIP(req, trusted); got != "203.0.113.50" {
		t.Fatalf("client ip %q", got)
	}
	if got := ForwardedProto(req, trusted, false); got != "https" {
		t.Fatalf("proto %q", got)
	}

	req.RemoteAddr = "203.0.113.8:1234"
	if got := ClientIP(req, trusted); got != "203.0.113.8" {
		t.Fatalf("untrusted client %q", got)
	}
	if got := ForwardedProto(req, trusted, false); got != "http" {
		t.Fatalf("untrusted proto %q", got)
	}
	if got := ForwardedProto(req, trusted, true); got != "https" {
		t.Fatalf("secure default %q", got)
	}

	req.RemoteAddr = "10.9.9.9:1"
	req.Header.Set("CF-Connecting-IP", "203.0.113.1, 10.9.9.9")
	if got := ClientIP(req, trusted); got != "10.9.9.9" {
		t.Fatalf("list header fell back to %q", got)
	}
	req.Header.Set("CF-Connecting-IP", "not-an-ip")
	if got := ClientIP(req, trusted); got != "10.9.9.9" {
		t.Fatalf("bad header fell back to %q", got)
	}

	mapped := httptest.NewRequest(http.MethodGet, "http://hebrew.example/", nil)
	mapped.RemoteAddr = "[::ffff:10.2.3.4]:443"
	if !PeerTrusted(mapped.RemoteAddr, trusted) {
		t.Fatal("v4-mapped peer was not trusted")
	}
	if len(mustCIDR(t)) != 0 {
		t.Fatal("empty list")
	}
	if PeerTrusted("10.1.1.1:1", nil) {
		t.Fatal("empty trust list trusted a peer")
	}
}

func TestOriginMatchesNode(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "golden", "origin.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var n int
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var item struct {
			Kind         string            `json:"kind"`
			ID           string            `json:"id"`
			PublicOrigin string            `json:"publicOrigin"`
			CookieSecure bool              `json:"cookieSecure"`
			Headers      map[string]string `json:"headers"`
			Allowed      bool              `json:"allowed"`
			Origin       any               `json:"origin"`
		}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatal(err)
		}
		if item.Kind != "case" {
			continue
		}
		n++
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Host = item.Headers["host"]
		req.RemoteAddr = "203.0.113.4:1234"
		for name, value := range item.Headers {
			if name == "host" {
				continue
			}
			req.Header.Set(name, value)
		}
		allowed := OriginAllowed(req, item.PublicOrigin, nil, item.CookieSecure)
		if allowed != item.Allowed {
			t.Errorf("%s allowed go %v node %v", item.ID, allowed, item.Allowed)
		}
		origin, ok := RequestOrigin(req)
		nodeOrigin := ""
		if s, isString := item.Origin.(string); isString {
			nodeOrigin = s
		}
		if ok != (nodeOrigin != "") || (ok && origin != nodeOrigin) {
			t.Errorf("%s origin go %q (%v) node %#v", item.ID, origin, ok, item.Origin)
		}
	}
	if n == 0 {
		t.Fatal("origin.jsonl has no cases")
	}
}

func hmacHex(key []byte, tokenHash string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(tokenHash))
	return hex.EncodeToString(mac.Sum(nil))
}

func mustCIDR(t *testing.T, cidrs ...string) []net.IPNet {
	t.Helper()
	var out []net.IPNet
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, *network)
	}
	return out
}
