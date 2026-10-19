package app

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
)

// TestOAuthContract is the production MCP mux. Discovery and registration
// are outside the bearer middleware. The issuer is the trusted proxy's
// public host, not the listener address.
func TestOAuthContract(t *testing.T) {
	env := startOAuthContract(t)

	wantAS := `{"issuer":"` + oauthPublicOrigin + `","authorization_endpoint":"` + oauthPublicOrigin + `/authorize","token_endpoint":"` + oauthPublicOrigin + `/token","registration_endpoint":"` + oauthPublicOrigin + `/register","response_types_supported":["code"],"grant_types_supported":["authorization_code","refresh_token"],"code_challenge_methods_supported":["S256"],"token_endpoint_auth_methods_supported":["none","client_secret_post","client_secret_basic"],"scopes_supported":["mcp"]}`
	as := doOAuth(t, env.client, http.MethodGet, env.base+"/.well-known/oauth-authorization-server", "", "", "")
	if as.StatusCode != http.StatusOK || as.Body != wantAS {
		t.Fatalf("authorization server %d %s", as.StatusCode, as.Body)
	}
	if strings.Contains(as.Body, env.listener) {
		t.Fatalf("issuer used the listener %s", as.Body)
	}
	wantPR := `{"resource":"` + oauthPublicOrigin + `/mcp","authorization_servers":["` + oauthPublicOrigin + `"],"scopes_supported":["mcp"],"bearer_methods_supported":["header"]}`
	for _, path := range []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp"} {
		res := doOAuth(t, env.client, http.MethodGet, env.base+path, "", "", "")
		if res.StatusCode != http.StatusOK || res.Body != wantPR || strings.Contains(res.Body, env.listener) {
			t.Fatalf("%s %d %s", path, res.StatusCode, res.Body)
		}
	}

	reg := doOAuth(t, env.client, http.MethodPost, env.base+"/register", `{"redirect_uris":["cursor://anysphere.cursor-mcp/oauth/callback"],"client_name":"cursor"}`, "application/json", "")
	if reg.StatusCode != http.StatusCreated || !strings.Contains(reg.Body, `"client_id"`) {
		t.Fatalf("register %d %s", reg.StatusCode, reg.Body)
	}
	cross := crossSiteRegister(t, env)
	if cross.StatusCode != http.StatusCreated {
		t.Fatalf("cross-site register %d %s", cross.StatusCode, cross.Body)
	}
	token := doOAuth(t, env.client, http.MethodPost, env.base+"/token", "", "application/x-www-form-urlencoded", "")
	if token.StatusCode == http.StatusForbidden || token.StatusCode == http.StatusUnauthorized {
		t.Fatalf("token post blocked %d %s", token.StatusCode, token.Body)
	}

	wantChallenge := `Bearer realm="hebrew-lessons", resource_metadata="` + oauthPublicOrigin + `/.well-known/oauth-protected-resource"`
	root := doOAuth(t, env.client, http.MethodPost, env.base+"/", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`, "application/json", "")
	if root.StatusCode != http.StatusNotFound || root.Body != "Not found" || root.Header.Get("WWW-Authenticate") != "" {
		t.Fatalf("root %d %q %s", root.StatusCode, root.Header.Get("WWW-Authenticate"), root.Body)
	}
	unknown := doOAuth(t, env.client, http.MethodGet, env.base+"/.well-known/oauth-authorization-server/mcp", "", "", "")
	if unknown.StatusCode != http.StatusNotFound || unknown.Header.Get("WWW-Authenticate") != "" {
		t.Fatalf("unknown well-known %d %q %s", unknown.StatusCode, unknown.Header.Get("WWW-Authenticate"), unknown.Body)
	}
	opt := doOAuth(t, env.client, http.MethodOptions, env.base+"/.well-known/oauth-authorization-server/mcp", "", "", "")
	if opt.StatusCode != http.StatusNoContent || opt.Header.Get("Access-Control-Allow-Origin") != "*" || opt.Header.Get("Access-Control-Max-Age") != "600" {
		t.Fatalf("options %d %+v", opt.StatusCode, opt.Header)
	}
	mcpMissing := postMCPPath(t, env.base, "/mcp", "", []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if mcpMissing.StatusCode != http.StatusUnauthorized || mcpMissing.Header.Get("WWW-Authenticate") != wantChallenge || mcpMissing.Body != "Unauthorized" {
		t.Fatalf("mcp challenge %d %q %s", mcpMissing.StatusCode, mcpMissing.Header.Get("WWW-Authenticate"), mcpMissing.Body)
	}

	learner := signInMCP(t, env.client, env.fake, env.base, oauthPublicOrigin, oauthLearnerEmail, "learner-sub")
	listed := postMCPPath(t, env.base, "/mcp", learner, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	if listed.StatusCode != http.StatusOK || strings.Contains(listed.Body, `"isError":true`) || !strings.Contains(listed.Body, "get_context") || !strings.Contains(listed.Body, "get_learner_words") || !strings.Contains(listed.Body, "mixTarget") {
		t.Fatalf("tools/list %d %s", listed.StatusCode, listed.Body)
	}

	env.access.Set(nil, []string{oauthAdminEmail})
	removed := postMCPPath(t, env.base, "/mcp", learner, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	if removed.StatusCode != http.StatusUnauthorized || removed.Header.Get("WWW-Authenticate") != wantChallenge {
		t.Fatalf("removed %d %q %s", removed.StatusCode, removed.Header.Get("WWW-Authenticate"), removed.Body)
	}

	still := postMCPPath(t, env.base, "/mcp", oauthStaticToken, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	if still.StatusCode != http.StatusOK || strings.Contains(still.Body, `"isError":true`) || !strings.Contains(still.Body, "get_context") || !strings.Contains(still.Body, "generate_audio") || !strings.Contains(still.Body, "get_learner_words") || !strings.Contains(still.Body, "cardsTruncated") {
		t.Fatalf("static %d %s", still.StatusCode, still.Body)
	}
}

// TestOAuthGoogleCallbackSkipsBearer is the production mux. /oauth/ and
// the /auth/google/callback alias are the OAuth handler. A request with
// no Authorization header must not be answered by the bearer middleware.
func TestOAuthGoogleCallbackSkipsBearer(t *testing.T) {
	env := startOAuthContract(t)
	for _, path := range []string{"/oauth/google/callback", "/auth/google/callback"} {
		for _, suffix := range []string{"", "?state=not-a-state"} {
			res := getCallbackNoAuth(t, env, path+suffix)
			if res.StatusCode == http.StatusUnauthorized || res.Header.Get("WWW-Authenticate") != "" {
				t.Fatalf("%s%s bearer %d %q %s", path, suffix, res.StatusCode, res.Header.Get("WWW-Authenticate"), res.Body)
			}
			if res.StatusCode != http.StatusBadRequest || !strings.Contains(res.Body, "Sign-in expired") {
				t.Fatalf("%s%s %d %s", path, suffix, res.StatusCode, res.Body)
			}
		}
		cb, redir := validGoogleCallback(t, env, path)
		if redir != oauthPublicOrigin+"/oauth/google/callback" || strings.Contains(redir, "/auth/google/callback") {
			t.Fatalf("redirect_uri %q", redir)
		}
		if cb.StatusCode == http.StatusUnauthorized || cb.Header.Get("WWW-Authenticate") != "" {
			t.Fatalf("%s valid bearer %d %q %s", path, cb.StatusCode, cb.Header.Get("WWW-Authenticate"), cb.Body)
		}
		if cb.StatusCode != http.StatusOK || !strings.Contains(cb.Body, "Approve") {
			t.Fatalf("%s valid %d %s", path, cb.StatusCode, cb.Body)
		}
	}
}

func getCallbackNoAuth(t *testing.T, env oauthContractEnv, path string) oauthResult {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, env.base+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Authorization") != "" {
		t.Fatal("callback request carried Authorization")
	}
	req.Header.Set("X-Forwarded-Host", oauthPublicHost)
	req.Header.Set("X-Forwarded-Proto", "https")
	res, err := env.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return oauthResult{StatusCode: res.StatusCode, Header: res.Header.Clone(), Body: string(b)}
}

func validGoogleCallback(t *testing.T, env oauthContractEnv, callbackPath string) (oauthResult, string) {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	reg := doOAuth(t, client, http.MethodPost, env.base+"/register", `{"redirect_uris":["http://127.0.0.1:9/callback"],"client_name":"callback"}`, "application/json", "")
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register %d %s", reg.StatusCode, reg.Body)
	}
	var doc struct {
		ID string `json:"client_id"`
	}
	if err := json.Unmarshal([]byte(reg.Body), &doc); err != nil || doc.ID == "" {
		t.Fatalf("client %s", reg.Body)
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(base64.RawURLEncoding.EncodeToString(buf)))
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {doc.ID},
		"redirect_uri":          {"http://127.0.0.1:9/callback"},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
		"state":                 {"client-state"},
	}
	start := doOAuth(t, client, http.MethodGet, env.base+"/authorize?"+q.Encode(), "", "", "")
	if start.StatusCode != http.StatusFound {
		t.Fatalf("authorize %d %s", start.StatusCode, start.Body)
	}
	gu, err := url.Parse(start.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	redir := gu.Query().Get("redirect_uri")
	env.fake.challenge = gu.Query().Get("code_challenge")
	env.fake.email = oauthLearnerEmail
	env.fake.sub = "callback-sub"
	env.fake.verified = true
	env.fake.mint = mintSpec{nonce: gu.Query().Get("nonce")}
	req, err := http.NewRequest(http.MethodGet, env.base+callbackPath+"?code=authcode&state="+url.QueryEscape(gu.Query().Get("state")), nil)
	if err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Authorization") != "" {
		t.Fatal("callback request carried Authorization")
	}
	req.Header.Set("X-Forwarded-Host", oauthPublicHost)
	req.Header.Set("X-Forwarded-Proto", "https")
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return oauthResult{StatusCode: res.StatusCode, Header: res.Header.Clone(), Body: string(b)}, redir
}

func TestOAuthDiscoveryMissesBearerWhenHostUnknown(t *testing.T) {
	a := newTestApp(t, Config{DataDir: t.TempDir(), MCPAuthToken: "static-secret"})
	rr := httptest.NewRecorder()
	a.MCP().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil))
	if rr.Code == http.StatusUnauthorized || strings.Contains(rr.Body.String(), "no bearer token") {
		t.Fatalf("discovery reached the bearer middleware %d %s", rr.Code, rr.Body.String())
	}
	if rr.Code != http.StatusServiceUnavailable || strings.Contains(rr.Body.String(), "192.0.2.1") {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}

type oauthContractEnv struct {
	base     string
	listener string
	client   *http.Client
	fake     *googleFake
	access   *auth.Access
}

func startOAuthContract(t *testing.T) oauthContractEnv {
	t.Helper()
	start := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := &stepClock{t: start}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	fake := &googleFake{
		key: key, kid: "test-key", clientID: "test-client", secret: "test-secret",
		issuer: "https://issuer.test", now: start, verified: true,
	}
	fakeSrv := httptest.NewServer(fake)
	t.Cleanup(fakeSrv.Close)
	access := auth.NewAccess([]string{oauthLearnerEmail}, []string{oauthAdminEmail})
	app := newTestApp(t, Config{
		DataDir:            t.TempDir(),
		PublicOrigin:       "https://hebrew.example",
		TrustedProxies:     []string{"127.0.0.1"},
		StateSecret:        "oauth-test-secret",
		GoogleClientID:     fake.clientID,
		GoogleClientSecret: fake.secret,
		GoogleAuthURL:      fakeSrv.URL + "/authorize",
		GoogleTokenURL:     fakeSrv.URL + "/token",
		GoogleJWKSURL:      fakeSrv.URL + "/jwks",
		GoogleIssuer:       fake.issuer,
		Access:             access,
		Clock:              clk,
		MCPAuthToken:       oauthStaticToken,
	})
	srv := httptest.NewServer(app.MCP())
	t.Cleanup(srv.Close)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return oauthContractEnv{
		base:     srv.URL,
		listener: srv.Listener.Addr().String(),
		client: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
		fake:   fake,
		access: access,
	}
}

func crossSiteRegister(t *testing.T, env oauthContractEnv) oauthResult {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, env.base+"/register", strings.NewReader(`{"redirect_uris":["http://127.0.0.1:9/callback"],"client_name":"cross"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("X-Forwarded-Host", oauthPublicHost)
	req.Header.Set("X-Forwarded-Proto", "https")
	res, err := env.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return oauthResult{StatusCode: res.StatusCode, Header: res.Header.Clone(), Body: string(b)}
}

func postMCPPath(t *testing.T, base, path, token string, body []byte) oauthResult {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	req.Header.Set("X-Forwarded-Host", oauthPublicHost)
	req.Header.Set("X-Forwarded-Proto", "https")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return oauthResult{StatusCode: res.StatusCode, Header: res.Header.Clone(), Body: string(b)}
}
