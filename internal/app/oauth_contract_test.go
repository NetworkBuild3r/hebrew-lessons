package app

import (
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
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

	reg := doOAuth(t, env.client, http.MethodPost, env.base+"/register", `{"redirect_uris":["https://client.example/callback"],"client_name":"cursor"}`, "application/json", "")
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
	missing := postMCPBody(t, env.base, "", []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`))
	// postMCPBody posts to /. The resource clients call is /mcp.
	mcpMissing := postMCPPath(t, env.base, "/mcp", "", []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if mcpMissing.StatusCode != http.StatusUnauthorized || mcpMissing.Header.Get("WWW-Authenticate") != wantChallenge {
		t.Fatalf("mcp challenge %d %q %s", mcpMissing.StatusCode, mcpMissing.Header.Get("WWW-Authenticate"), mcpMissing.Body)
	}
	if missing.StatusCode != http.StatusUnauthorized || missing.Header.Get("WWW-Authenticate") != wantChallenge {
		t.Fatalf("root challenge %d %q", missing.StatusCode, missing.Header.Get("WWW-Authenticate"))
	}

	learner := signInMCP(t, env.client, env.fake, env.base, oauthPublicOrigin, oauthLearnerEmail, "learner-sub")
	listed := postMCPBody(t, env.base, learner, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	if listed.StatusCode != http.StatusOK || strings.Contains(listed.Body, `"isError":true`) || !strings.Contains(listed.Body, "get_context") {
		t.Fatalf("tools/list %d %s", listed.StatusCode, listed.Body)
	}

	env.access.Set(nil, []string{oauthAdminEmail})
	removed := postMCPBody(t, env.base, learner, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	if removed.StatusCode != http.StatusUnauthorized || removed.Header.Get("WWW-Authenticate") != wantChallenge {
		t.Fatalf("removed %d %q %s", removed.StatusCode, removed.Header.Get("WWW-Authenticate"), removed.Body)
	}

	still := postMCPBody(t, env.base, oauthStaticToken, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	if still.StatusCode != http.StatusOK || strings.Contains(still.Body, `"isError":true`) || !strings.Contains(still.Body, "get_context") {
		t.Fatalf("static %d %s", still.StatusCode, still.Body)
	}
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
	req, err := http.NewRequest(http.MethodPost, env.base+"/register", strings.NewReader(`{"redirect_uris":["https://client.example/cb"],"client_name":"cross"}`))
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
