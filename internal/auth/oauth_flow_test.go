package auth

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

const (
	oauthLearner  = "learner@example.com"
	oauthAdmin    = "admin@example.com"
	oauthRedirect = "http://127.0.0.1:9/callback"
)

func TestOAuthDiscoveryDocuments(t *testing.T) {
	env := newOAuthEnv(t, false)
	as := env.get("/.well-known/oauth-authorization-server")
	if as.StatusCode != http.StatusOK {
		t.Fatalf("as %d %s", as.StatusCode, as.Body)
	}
	if as.Header.Get("Content-Type") != "application/json; charset=utf-8" || as.Header.Get("Cache-Control") != "no-store" || as.Header.Get("Pragma") != "no-cache" {
		t.Fatalf("headers %+v", as.Header)
	}
	if strings.HasSuffix(as.Body, "\n") {
		t.Fatalf("trailing newline %q", as.Body)
	}
	wantAS := `{"issuer":"` + env.base + `","authorization_endpoint":"` + env.base + `/authorize","token_endpoint":"` + env.base + `/token","registration_endpoint":"` + env.base + `/register","response_types_supported":["code"],"grant_types_supported":["authorization_code","refresh_token"],"code_challenge_methods_supported":["S256"],"token_endpoint_auth_methods_supported":["none","client_secret_post","client_secret_basic"],"scopes_supported":["mcp"]}`
	if as.Body != wantAS {
		t.Fatalf("as body %s", as.Body)
	}
	oidc := env.get("/.well-known/openid-configuration")
	if oidc.StatusCode != http.StatusOK || oidc.Body != as.Body {
		t.Fatalf("oidc %d %s", oidc.StatusCode, oidc.Body)
	}
	res := env.get("/.well-known/oauth-protected-resource")
	wantPR := `{"resource":"` + env.base + `/mcp","authorization_servers":["` + env.base + `"],"scopes_supported":["mcp"],"bearer_methods_supported":["header"]}`
	if res.StatusCode != http.StatusOK || res.Body != wantPR {
		t.Fatalf("resource %d %s", res.StatusCode, res.Body)
	}
	mcpRes := env.get("/.well-known/oauth-protected-resource/mcp")
	if mcpRes.StatusCode != http.StatusOK || mcpRes.Body != wantPR {
		t.Fatalf("resource mcp %d %s", mcpRes.StatusCode, mcpRes.Body)
	}
	fwd := env.do(http.MethodGet, "/.well-known/oauth-authorization-server", nil, "")
	fwd.Header.Set("X-Forwarded-Host", "evil.example")
	got := env.roundTrip(fwd)
	if !strings.Contains(got.Body, env.base) || strings.Contains(got.Body, "evil.example") {
		t.Fatalf("issuer followed forwarded host %s", got.Body)
	}
	opt := env.do(http.MethodOptions, "/register", nil, "")
	optRes := env.roundTrip(opt)
	if optRes.StatusCode != http.StatusNoContent {
		t.Fatalf("options %d", optRes.StatusCode)
	}
	if optRes.Header.Get("Access-Control-Allow-Origin") != "*" || optRes.Header.Get("Access-Control-Max-Age") != "600" {
		t.Fatalf("cors %+v", optRes.Header)
	}
	if optRes.Header.Get("Access-Control-Allow-Headers") != "Content-Type, Authorization, Accept, MCP-Protocol-Version" {
		t.Fatalf("allow headers %q", optRes.Header.Get("Access-Control-Allow-Headers"))
	}
	missing := env.get("/token")
	if missing.StatusCode != http.StatusNotFound || missing.Body != "Not found" {
		t.Fatalf("wrong method %d %q", missing.StatusCode, missing.Body)
	}
}

func TestOAuthIssuerUsesTrustedPublicHost(t *testing.T) {
	env := startOAuthEnvOpts(t, oauthStartOpts{proxies: []string{"127.0.0.1"}, unsetPublic: true})
	req := env.do(http.MethodGet, "/.well-known/oauth-authorization-server", nil, "")
	req.Header.Set("X-Forwarded-Host", "hebrew-mcp.ibhacked.us")
	req.Header.Set("X-Forwarded-Proto", "https")
	got := env.roundTrip(req)
	if got.StatusCode != http.StatusOK || !strings.Contains(got.Body, `"issuer":"https://hebrew-mcp.ibhacked.us"`) || strings.Contains(got.Body, env.base) {
		t.Fatalf("public host %d %s", got.StatusCode, got.Body)
	}
	ported := env.do(http.MethodGet, "/.well-known/oauth-protected-resource", nil, "")
	ported.Header.Set("X-Forwarded-Host", "hebrew-mcp.ibhacked.us:443")
	ported.Header.Set("X-Forwarded-Proto", "https")
	got = env.roundTrip(ported)
	if got.StatusCode != http.StatusOK || !strings.Contains(got.Body, `"resource":"https://hebrew-mcp.ibhacked.us/mcp"`) || strings.Contains(got.Body, ":443") {
		t.Fatalf("default port %d %s", got.StatusCode, got.Body)
	}
	pod := env.do(http.MethodGet, "/.well-known/oauth-authorization-server", nil, "")
	pod.Header.Set("X-Forwarded-Host", "10.8.8.8")
	pod.Header.Set("X-Forwarded-Proto", "https")
	got = env.roundTrip(pod)
	if got.StatusCode != http.StatusServiceUnavailable || strings.Contains(got.Body, "10.8.8.8") {
		t.Fatalf("pod address %d %s", got.StatusCode, got.Body)
	}
	cluster := env.do(http.MethodGet, "/.well-known/oauth-authorization-server", nil, "")
	cluster.Header.Set("X-Forwarded-Host", "hebrew.default.svc.cluster.local")
	cluster.Header.Set("X-Forwarded-Proto", "https")
	got = env.roundTrip(cluster)
	if got.StatusCode != http.StatusServiceUnavailable || strings.Contains(got.Body, "cluster.local") {
		t.Fatalf("cluster host %d %s", got.StatusCode, got.Body)
	}
}

func TestMCPPublicURLPrecedesForwardedHost(t *testing.T) {
	const public = "https://hebrew-mcp.ibhacked.us"
	env := startOAuthEnvOpts(t, oauthStartOpts{google: true, proxies: []string{"127.0.0.1"}, public: public})
	req := env.do(http.MethodGet, "/.well-known/oauth-authorization-server", nil, "")
	req.Header.Set("X-Forwarded-Host", "other.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	got := env.roundTrip(req)
	if got.StatusCode != http.StatusOK || !strings.Contains(got.Body, `"issuer":"`+public+`"`) || strings.Contains(got.Body, "other.example") {
		t.Fatalf("issuer %d %s", got.StatusCode, got.Body)
	}
	id := env.registerClient(oauthRedirect, "none")
	pair := pkcePair(t)
	start := env.authorize(id, oauthRedirect, pair.challenge, "S256", "st")
	if start.StatusCode != http.StatusFound {
		t.Fatalf("authorize %d %s", start.StatusCode, start.Body)
	}
	redir := mustParse(t, start.Header.Get("Location")).Query().Get("redirect_uri")
	if redir != public+"/oauth/google/callback" || strings.Contains(redir, "/auth/google/callback") {
		t.Fatalf("google redirect_uri %q", redir)
	}
}

func TestOAuthDynamicRegistration(t *testing.T) {
	env := newOAuthEnv(t, false)
	accepted := []string{
		"cursor://anysphere.cursor-mcp/oauth/callback",
		"https://www.cursor.com/agents/mcp/oauth/callback",
		"https://cursor.com/agents/mcp/oauth/callback",
		"http://localhost:8787/callback",
		"http://127.0.0.1:1/callback",
		"http://localhost:9/callback",
		"http://[::1]:9/callback",
	}
	for _, uri := range accepted {
		res := env.registerRaw(map[string]any{"redirect_uris": []string{uri}, "client_name": "Cursor"})
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("%s status %d %s", uri, res.StatusCode, res.Body)
		}
		var doc registrationResponse
		if err := json.Unmarshal([]byte(res.Body), &doc); err != nil {
			t.Fatal(err)
		}
		if doc.ClientID == "" || doc.TokenEndpointAuthMethod != "none" || doc.ClientSecret != "" || len(doc.RedirectURIs) != 1 || doc.RedirectURIs[0] != uri {
			t.Fatalf("%s doc %+v", uri, doc)
		}
		if !strings.Contains(res.Body, `"grant_types":["authorization_code","refresh_token"]`) {
			t.Fatalf("grants %s", res.Body)
		}
	}
	conf := env.registerRaw(map[string]any{
		"redirect_uris":              []string{"https://cursor.com/agents/mcp/oauth/callback"},
		"token_endpoint_auth_method": "client_secret_post",
		"client_name":                "<script>",
	})
	if conf.StatusCode != http.StatusCreated || !strings.Contains(conf.Body, `"client_secret_expires_at":0`) || !strings.Contains(conf.Body, `"client_secret":"`) {
		t.Fatalf("confidential %d %s", conf.StatusCode, conf.Body)
	}
	rejected := []string{
		"https://client.example/callback",
		"https://client.example:8443/callback",
		"http://client.example/callback",
		"http://127.0.0.1:9/callback#frag",
		"cursor://anysphere.cursor-mcp/oauth/callback#x",
		"https://user:pass@client.example/callback",
		"javascript:alert(1)",
		"http://127.0.0.2/callback",
	}
	for _, uri := range rejected {
		res := env.registerRaw(map[string]any{"redirect_uris": []string{uri}})
		if res.StatusCode != http.StatusBadRequest || !strings.Contains(res.Body, `"error":"invalid_redirect_uri"`) || !strings.Contains(res.Body, "redirect_uri is not allowed by this server: ") {
			t.Fatalf("%s %d %s", uri, res.StatusCode, res.Body)
		}
	}
	bad := []struct {
		body string
		code int
		err  string
		desc string
	}{
		{body: "{", code: http.StatusBadRequest, err: "invalid_client_metadata", desc: "Body must be JSON."},
		{body: `{"redirect_uris":[]}`, code: http.StatusBadRequest, err: "invalid_redirect_uri", desc: "redirect_uris must be 1 to 10 strings."},
		{body: `{"redirect_uris":"https://client.example/cb"}`, code: http.StatusBadRequest, err: "invalid_redirect_uri", desc: "redirect_uris must be 1 to 10 strings."},
	}
	for _, tc := range bad {
		res := env.post("/register", tc.body, "application/json")
		if res.StatusCode != tc.code || !strings.Contains(res.Body, `"error":"`+tc.err+`"`) || !strings.Contains(res.Body, tc.desc) {
			t.Fatalf("body %s -> %d %s", tc.body, res.StatusCode, res.Body)
		}
	}
	uris := make([]string, 11)
	for i := range uris {
		uris[i] = "https://client.example/" + strings.Repeat("a", i+1)
	}
	tooMany := env.registerRaw(map[string]any{"redirect_uris": uris})
	if tooMany.StatusCode != http.StatusBadRequest || !strings.Contains(tooMany.Body, "redirect_uris must be 1 to 10 strings.") {
		t.Fatalf("eleven %d %s", tooMany.StatusCode, tooMany.Body)
	}
	huge := env.post("/register", strings.Repeat("x", oauthMaxBody+1), "application/json")
	if huge.StatusCode != http.StatusRequestEntityTooLarge || !strings.Contains(huge.Body, "Body must be JSON.") {
		t.Fatalf("413 %d %s", huge.StatusCode, huge.Body)
	}
}

func TestOAuthAuthorizationCodeFlow(t *testing.T) {
	env := newOAuthEnv(t, true)
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	code, verifier, clientID := env.authorizationCode(oauthLearner, "learner-sub")
	if strings.Contains(logs.String(), oauthRedirect) {
		t.Fatalf("log leaked redirect %s", logs.String())
	}
	if !strings.Contains(logs.String(), "redirect_uri_present") || !strings.Contains(logs.String(), clientID) {
		t.Fatalf("log %s", logs.String())
	}
	tok := env.exchange(clientID, code, verifier, oauthRedirect, "", "")
	if tok.AccessToken == "" || tok.RefreshToken == "" || tok.TokenType != "Bearer" || tok.ExpiresIn != 3600 || tok.Scope != "mcp" {
		t.Fatalf("tokens %+v", tok)
	}
	raw, err := env.svc.SessionFromAccessToken(context.Background(), tok.AccessToken, nil)
	if err != nil || raw == "" {
		t.Fatalf("session %q %v", raw, err)
	}
	user, _, err := env.svc.Lookup(context.Background(), raw)
	if err != nil || user.Email != oauthLearner || user.Role != RoleLearner {
		t.Fatalf("user %+v err %v", user, err)
	}
	reused := env.tokenForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {oauthRedirect},
		"client_id":     {clientID},
	}, "")
	if reused.StatusCode != http.StatusBadRequest || !strings.Contains(reused.Body, "Code was already used.") {
		t.Fatalf("reuse %d %s", reused.StatusCode, reused.Body)
	}

	longCode, longVerifier, longClient := env.authorizationCode(oauthLearner, "learner-sub")
	longTok := env.exchange(longClient, longCode, longVerifier, oauthRedirect, "", "")
	second, verifier2, client2 := env.authorizationCode(oauthLearner, "learner-sub")
	refreshed := env.exchange(client2, second, verifier2, oauthRedirect, "", "")
	next := env.tokenForm(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshed.RefreshToken},
		"client_id":     {client2},
	}, "")
	if next.StatusCode != http.StatusOK {
		t.Fatalf("refresh %d %s", next.StatusCode, next.Body)
	}
	var rotated tokenResponse
	if err := json.Unmarshal([]byte(next.Body), &rotated); err != nil || rotated.AccessToken == "" || rotated.RefreshToken == refreshed.RefreshToken {
		t.Fatalf("rotated %s err %v", next.Body, err)
	}
	if _, err := env.svc.SessionFromAccessToken(context.Background(), refreshed.AccessToken, nil); err != nil {
		t.Fatal("previous access token was revoked on rotation")
	}
	old := env.tokenForm(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshed.RefreshToken},
		"client_id":     {client2},
	}, "")
	if old.StatusCode != http.StatusBadRequest || !strings.Contains(old.Body, "Refresh token is invalid or expired.") {
		t.Fatalf("old refresh %d %s", old.StatusCode, old.Body)
	}
	if _, err := env.svc.SessionFromAccessToken(context.Background(), rotated.AccessToken, nil); !errors.Is(err, ErrNoSession) {
		t.Fatalf("family reuse err %v", err)
	}

	env.clk.Advance(oauthAccessTTL + time.Second)
	if _, err := env.svc.SessionFromAccessToken(context.Background(), tok.AccessToken, nil); !errors.Is(err, ErrNoSession) {
		t.Fatalf("expired access %v", err)
	}
	env.clk.Advance(oauthRefreshTTL)
	expiredRefresh := env.tokenForm(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {longTok.RefreshToken},
		"client_id":     {longClient},
	}, "")
	if expiredRefresh.StatusCode != http.StatusBadRequest || !strings.Contains(expiredRefresh.Body, "Refresh token is invalid or expired.") {
		t.Fatalf("expired refresh %d %s", expiredRefresh.StatusCode, expiredRefresh.Body)
	}
}

func TestOAuthPKCEAndRedirectMatch(t *testing.T) {
	env := newOAuthEnv(t, true)
	id := env.registerClient(oauthRedirect, "none")
	plain := pkcePair(t)
	res := env.authorize(id, oauthRedirect, plain.challenge, "plain", "st")
	loc := mustRedirect(t, res, http.StatusSeeOther)
	q := mustParse(t, loc).Query()
	if q.Get("error") != "invalid_request" || q.Get("error_description") != "PKCE with S256 is required." || q.Get("state") != "st" {
		t.Fatalf("plain %s", loc)
	}
	other := env.get("/authorize?" + url.Values{
		"response_type":         {"token"},
		"client_id":             {id},
		"redirect_uri":          {oauthRedirect},
		"code_challenge":        {plain.challenge},
		"code_challenge_method": {"S256"},
		"state":                 {"st"},
	}.Encode())
	loc = mustRedirect(t, other, http.StatusSeeOther)
	if mustParse(t, loc).Query().Get("error") != "unsupported_response_type" {
		t.Fatalf("response type %s", loc)
	}
	target := env.get("/authorize?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {id},
		"redirect_uri":          {oauthRedirect},
		"code_challenge":        {plain.challenge},
		"code_challenge_method": {"S256"},
		"resource":              {"https://evil.example/mcp"},
	}.Encode())
	if mustParse(t, mustRedirect(t, target, http.StatusSeeOther)).Query().Get("error") != "invalid_target" {
		t.Fatalf("resource %s", target.Body)
	}
	unknown := env.get("/authorize?response_type=code&client_id=missing&redirect_uri=" + url.QueryEscape(oauthRedirect))
	if unknown.StatusCode != http.StatusBadRequest || !strings.Contains(unknown.Body, "Unknown or expired client. Register again.") {
		t.Fatalf("unknown %d %s", unknown.StatusCode, unknown.Body)
	}
	mismatch := env.get("/authorize?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {id},
		"redirect_uri":          {"http://127.0.0.1:9/other"},
		"code_challenge":        {plain.challenge},
		"code_challenge_method": {"S256"},
	}.Encode())
	if mismatch.StatusCode != http.StatusBadRequest || !strings.Contains(mismatch.Body, "redirect_uri does not match this client.") {
		t.Fatalf("mismatch %d %s", mismatch.StatusCode, mismatch.Body)
	}

	code, verifier, clientID := env.authorizationCode(oauthLearner, "learner-sub")
	wrong := env.tokenForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier + "x"},
		"redirect_uri":  {oauthRedirect},
		"client_id":     {clientID},
	}, "")
	if wrong.StatusCode != http.StatusBadRequest || !strings.Contains(wrong.Body, "PKCE verification failed.") {
		t.Fatalf("verifier %d %s", wrong.StatusCode, wrong.Body)
	}
	missing := env.tokenForm(url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {oauthRedirect},
		"client_id":    {clientID},
	}, "")
	if missing.StatusCode != http.StatusBadRequest || !strings.Contains(missing.Body, "PKCE verification failed.") {
		t.Fatalf("missing verifier %d %s", missing.StatusCode, missing.Body)
	}
	otherURI := env.tokenForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {"http://127.0.0.1:9/other"},
		"client_id":     {clientID},
	}, "")
	if otherURI.StatusCode != http.StatusBadRequest || !strings.Contains(otherURI.Body, "client_id or redirect_uri does not match the code.") {
		t.Fatalf("redirect %d %s", otherURI.StatusCode, otherURI.Body)
	}
	emptyURI := env.tokenForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {""},
		"client_id":     {clientID},
	}, "")
	if emptyURI.StatusCode != http.StatusBadRequest || !strings.Contains(emptyURI.Body, "client_id or redirect_uri does not match the code.") {
		t.Fatalf("empty redirect %d %s", emptyURI.StatusCode, emptyURI.Body)
	}
	omitted := env.exchange(clientID, code, verifier, "", "", "")
	if omitted.AccessToken == "" {
		t.Fatal("omitted redirect_uri was rejected")
	}

	held, verifierHeld, id2 := env.authorizationCode(oauthLearner, "learner-sub")
	sum := sha256.Sum256([]byte(verifierHeld))
	sentChallenge := env.tokenForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {held},
		"code_verifier": {base64.RawURLEncoding.EncodeToString(sum[:])},
		"client_id":     {id2},
	}, "")
	if sentChallenge.StatusCode != http.StatusBadRequest || !strings.Contains(sentChallenge.Body, "PKCE verification failed.") {
		t.Fatalf("challenge as verifier %d %s", sentChallenge.StatusCode, sentChallenge.Body)
	}
}

func TestOAuthClientAuthentication(t *testing.T) {
	env := newOAuthEnv(t, true)
	code, verifier, clientID, secret := env.authorizationCodeMethod(oauthLearner, "learner-sub", "client_secret_basic")
	noSecret := env.tokenForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {oauthRedirect},
		"client_id":     {clientID},
	}, "")
	if noSecret.StatusCode != http.StatusUnauthorized || !strings.Contains(noSecret.Body, "Client authentication failed.") {
		t.Fatalf("no secret %d %s", noSecret.StatusCode, noSecret.Body)
	}
	if noSecret.Header.Get("WWW-Authenticate") != `Basic realm="hebrew-lessons"` {
		t.Fatalf("challenge %q", noSecret.Header.Get("WWW-Authenticate"))
	}
	bad := env.tokenForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {oauthRedirect},
	}, basicHeader(clientID, "wrong-secret"))
	if bad.StatusCode != http.StatusUnauthorized || bad.Header.Get("WWW-Authenticate") != `Basic realm="hebrew-lessons"` {
		t.Fatalf("bad secret %d %s %q", bad.StatusCode, bad.Body, bad.Header.Get("WWW-Authenticate"))
	}
	conflict := env.tokenForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {oauthRedirect},
		"client_id":     {"other-client"},
	}, basicHeader(clientID, secret))
	if conflict.StatusCode != http.StatusUnauthorized {
		t.Fatalf("conflict %d %s", conflict.StatusCode, conflict.Body)
	}
	ok := env.tokenForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {oauthRedirect},
	}, basicHeader(clientID, secret))
	if ok.StatusCode != http.StatusOK {
		t.Fatalf("basic ok %d %s", ok.StatusCode, ok.Body)
	}

	postCode, postVerifier, postID, postSecret := env.authorizationCodeMethod(oauthLearner, "learner-sub", "client_secret_post")
	basicOnPost := env.tokenForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {postCode},
		"code_verifier": {postVerifier},
		"redirect_uri":  {oauthRedirect},
	}, basicHeader(postID, postSecret))
	if basicOnPost.StatusCode != http.StatusUnauthorized || basicOnPost.Header.Get("WWW-Authenticate") != `Basic realm="hebrew-lessons"` {
		t.Fatalf("basic on post client %d %s %q", basicOnPost.StatusCode, basicOnPost.Body, basicOnPost.Header.Get("WWW-Authenticate"))
	}
	posted := env.tokenForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {postCode},
		"code_verifier": {postVerifier},
		"redirect_uri":  {oauthRedirect},
		"client_id":     {postID},
		"client_secret": {postSecret},
	}, "")
	if posted.StatusCode != http.StatusOK {
		t.Fatalf("post secret %d %s", posted.StatusCode, posted.Body)
	}
	unsupported := env.tokenForm(url.Values{"grant_type": {"password"}}, "")
	if unsupported.StatusCode != http.StatusBadRequest || !strings.Contains(unsupported.Body, "Use authorization_code or refresh_token.") {
		t.Fatalf("grant %d %s", unsupported.StatusCode, unsupported.Body)
	}
}

func TestOAuthConsentCSRFCookiesAndProxies(t *testing.T) {
	env := newOAuthEnv(t, true)
	id := env.registerClient(oauthRedirect, "none")
	pair := pkcePair(t)
	start := env.authorize(id, oauthRedirect, pair.challenge, "S256", "st")
	if start.StatusCode != http.StatusFound {
		t.Fatalf("start %d %s", start.StatusCode, start.Body)
	}
	googleCookie := cookieByName(t, start.Cookies, "hebrew_mcp_google")
	if !googleCookie.HttpOnly || googleCookie.Secure || googleCookie.Path != "/" || googleCookie.Domain != "" || googleCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("state cookie %+v", googleCookie)
	}
	page := env.finishGoogle(start, oauthLearner, "learner-sub", true)
	if page.StatusCode != http.StatusOK || strings.Contains(page.Body, "password") || !strings.Contains(page.Body, oauthRedirect) {
		t.Fatalf("consent %d %s", page.StatusCode, page.Body)
	}
	if !strings.Contains(page.Body, "MCP client") {
		t.Fatalf("client name missing %s", page.Body)
	}
	reqID, csrf := consentFields(t, page.Body)
	badOrigin := env.consent(reqID, csrf, "approve", "https://evil.example")
	if badOrigin.StatusCode != http.StatusForbidden || !strings.Contains(badOrigin.Body, "could not be confirmed") {
		t.Fatalf("origin %d %s", badOrigin.StatusCode, badOrigin.Body)
	}
	badToken := env.consent(reqID, "not-the-token", "approve", env.base)
	if badToken.StatusCode != http.StatusForbidden {
		t.Fatalf("csrf %d %s", badToken.StatusCode, badToken.Body)
	}
	denied := env.consent(reqID, csrf, "deny", env.base)
	loc := mustRedirect(t, denied, http.StatusSeeOther)
	if mustParse(t, loc).Query().Get("error") != "access_denied" {
		t.Fatalf("deny %s", loc)
	}

	secure := newOAuthEnvSecure(t)
	sid := secure.registerClient(oauthRedirect, "none")
	sp := pkcePair(t)
	sstart := secure.authorize(sid, oauthRedirect, sp.challenge, "S256", "st")
	hostCookie := cookieByName(t, sstart.Cookies, "__Host-hebrew_mcp_google")
	if !hostCookie.Secure || !hostCookie.HttpOnly || hostCookie.Path != "/" || hostCookie.Domain != "" {
		t.Fatalf("host cookie %+v", hostCookie)
	}
	sstart.Header.Set("Cookie", hostCookie.Name+"="+hostCookie.Value)
	spage := secure.finishGoogle(sstart, oauthLearner, "learner-sub", true)
	session := cookieByName(t, spage.Cookies, "__Host-hebrew_session")
	if !session.Secure || !session.HttpOnly || session.Path != "/" || session.Domain != "" || session.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie %+v", session)
	}

	proxied := newOAuthEnvProxied(t)
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(prev)
	req := proxied.do(http.MethodGet, "/authorize?client_id=missing&redirect_uri="+url.QueryEscape(oauthRedirect), nil, "")
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 127.0.0.1")
	req.Header.Set("X-Forwarded-Proto", "https")
	_ = proxied.roundTrip(req)
	if !strings.Contains(logs.String(), "203.0.113.9") || !strings.Contains(logs.String(), "scheme=https") {
		t.Fatalf("trusted proxy log %s", logs.String())
	}
	logs.Reset()
	plain := newOAuthEnv(t, false)
	req = plain.do(http.MethodGet, "/authorize?client_id=missing", nil, "")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("X-Forwarded-Proto", "https")
	_ = plain.roundTrip(req)
	if strings.Contains(logs.String(), "203.0.113.9") || strings.Contains(logs.String(), "scheme=https") {
		t.Fatalf("untrusted proxy log %s", logs.String())
	}
}

func TestOAuthGoogleAndAllowlistFailures(t *testing.T) {
	env := newOAuthEnv(t, true)
	off := newOAuthEnv(t, false)
	id := off.registerClient(oauthRedirect, "none")
	pair := pkcePair(t)
	res := off.authorize(id, oauthRedirect, pair.challenge, "S256", "st")
	if res.StatusCode != http.StatusServiceUnavailable || !strings.Contains(res.Body, "Google sign-in is not configured") || strings.Contains(res.Body, "password") {
		t.Fatalf("google off %d %s", res.StatusCode, res.Body)
	}

	start := env.authorize(env.registerClient(oauthRedirect, "none"), oauthRedirect, pkcePair(t).challenge, "S256", "st")
	unverified := env.finishGoogle(start, oauthLearner, "learner-sub", false)
	loc := mustRedirect(t, unverified, http.StatusSeeOther)
	if mustParse(t, loc).Query().Get("error") != "access_denied" {
		t.Fatalf("unverified %s", loc)
	}
	start = env.authorize(env.registerClient(oauthRedirect, "none"), oauthRedirect, pkcePair(t).challenge, "S256", "st")
	blocked := env.finishGoogle(start, "stranger@example.com", "stranger-sub", true)
	if mustParse(t, mustRedirect(t, blocked, http.StatusSeeOther)).Query().Get("error") != "access_denied" {
		t.Fatalf("stranger %s", blocked.Body)
	}

	code, verifier, clientID := env.authorizationCode(oauthLearner, "learner-sub")
	env.access.Set([]string{oauthAdmin}, []string{oauthAdmin})
	rejected := env.tokenForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {oauthRedirect},
		"client_id":     {clientID},
	}, "")
	if rejected.StatusCode != http.StatusBadRequest || !strings.Contains(rejected.Body, "That account can no longer sign in.") {
		t.Fatalf("removed at token %d %s", rejected.StatusCode, rejected.Body)
	}
	again := env.tokenForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {oauthRedirect},
		"client_id":     {clientID},
	}, "")
	if again.StatusCode != http.StatusBadRequest || !strings.Contains(again.Body, "Code was already used.") {
		t.Fatalf("code was not burned %d %s", again.StatusCode, again.Body)
	}

	env.access.Set([]string{oauthLearner}, []string{oauthAdmin})
	agedCode, agedVerifier, agedClient := env.authorizationCode(oauthLearner, "learner-sub")
	env.clk.Advance(oauthCodeTTL + time.Second)
	expired := env.tokenForm(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {agedCode},
		"code_verifier": {agedVerifier},
		"redirect_uri":  {oauthRedirect},
		"client_id":     {agedClient},
	}, "")
	if expired.StatusCode != http.StatusBadRequest || !strings.Contains(expired.Body, "Code is invalid or expired.") {
		t.Fatalf("expired code %d %s", expired.StatusCode, expired.Body)
	}
}

type oauthHTTP struct {
	StatusCode int
	Header     http.Header
	Body       string
	Cookies    []*http.Cookie
}

type pkce struct {
	verifier  string
	challenge string
}

type oauthEnv struct {
	t      *testing.T
	clk    *manualClock
	store  *db.Store
	access *Access
	svc    *Service
	srv    *httptest.Server
	base   string
	fake   *oauthGoogle
	client *http.Client
}

func newOAuthEnv(t *testing.T, google bool) *oauthEnv {
	t.Helper()
	return startOAuthEnv(t, google, false, nil)
}

func newOAuthEnvSecure(t *testing.T) *oauthEnv {
	t.Helper()
	return startOAuthEnv(t, true, true, nil)
}

func newOAuthEnvProxied(t *testing.T) *oauthEnv {
	t.Helper()
	return startOAuthEnv(t, false, false, []string{"127.0.0.1"})
}

type oauthStartOpts struct {
	google      bool
	secure      bool
	proxies     []string
	public      string
	unsetPublic bool
}

func startOAuthEnv(t *testing.T, google, secure bool, proxies []string) *oauthEnv {
	t.Helper()
	return startOAuthEnvOpts(t, oauthStartOpts{google: google, secure: secure, proxies: proxies})
}

func startOAuthEnvOpts(t *testing.T, opts oauthStartOpts) *oauthEnv {
	t.Helper()
	clk := &manualClock{t: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	store := openTestStore(t, clk)
	access := NewAccess([]string{oauthLearner}, []string{oauthAdmin})
	env := &oauthEnv{t: t, clk: clk, store: store, access: access}
	cfg := Config{
		PublicOrigin:   "https://hebrew.example",
		CookieSecure:   opts.secure,
		StateSecret:    "oauth-test-secret",
		Access:         access,
		Clock:          clk,
		Store:          store,
		TrustedProxies: opts.proxies,
	}
	if opts.google {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		fake := &oauthGoogle{key: key, kid: "test-key", clientID: "test-client", secret: "test-secret", issuer: "https://issuer.test", now: clk.Now()}
		fakeSrv := httptest.NewServer(fake)
		t.Cleanup(fakeSrv.Close)
		fake.srv = fakeSrv
		env.fake = fake
		cfg.GoogleClientID = fake.clientID
		cfg.GoogleClientSecret = fake.secret
		cfg.GoogleAuthURL = fakeSrv.URL + "/authorize"
		cfg.GoogleTokenURL = fakeSrv.URL + "/token"
		cfg.GoogleJWKSURL = fakeSrv.URL + "/jwks"
		cfg.GoogleIssuer = fake.issuer
	}
	svc, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	env.svc = svc
	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	env.base = "http://" + srv.Listener.Addr().String()
	public := env.base
	if opts.unsetPublic {
		public = ""
	} else if opts.public != "" {
		public = opts.public
	}
	if !svc.RegisterOAuth(mux, public) {
		t.Fatal("oauth did not mount")
	}
	srv.Start()
	t.Cleanup(srv.Close)
	env.srv = srv
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	env.client = &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return env
}

func (e *oauthEnv) get(path string) oauthHTTP {
	e.t.Helper()
	return e.roundTrip(e.do(http.MethodGet, path, nil, ""))
}

func (e *oauthEnv) post(path, body, contentType string) oauthHTTP {
	e.t.Helper()
	req := e.do(http.MethodPost, path, strings.NewReader(body), "")
	req.Header.Set("Content-Type", contentType)
	return e.roundTrip(req)
}

func (e *oauthEnv) do(method, path string, body io.Reader, authz string) *http.Request {
	e.t.Helper()
	req, err := http.NewRequest(method, e.base+path, body)
	if err != nil {
		e.t.Fatal(err)
	}
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	return req
}

func (e *oauthEnv) roundTrip(req *http.Request) oauthHTTP {
	e.t.Helper()
	res, err := e.client.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		e.t.Fatal(err)
	}
	return oauthHTTP{StatusCode: res.StatusCode, Header: res.Header.Clone(), Body: string(b), Cookies: res.Cookies()}
}

func (e *oauthEnv) registerRaw(doc any) oauthHTTP {
	e.t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		e.t.Fatal(err)
	}
	return e.post("/register", string(raw), "application/json")
}

func (e *oauthEnv) registerClient(redirect, method string) (id string) {
	e.t.Helper()
	id, _ = e.registerClientSecret(redirect, method)
	return id
}

func (e *oauthEnv) registerClientSecret(redirect, method string) (string, string) {
	e.t.Helper()
	res := e.registerRaw(map[string]any{
		"redirect_uris":              []string{redirect},
		"token_endpoint_auth_method": method,
		"client_name":                "MCP client",
	})
	if res.StatusCode != http.StatusCreated {
		e.t.Fatalf("register %d %s", res.StatusCode, res.Body)
	}
	var doc registrationResponse
	if err := json.Unmarshal([]byte(res.Body), &doc); err != nil {
		e.t.Fatal(err)
	}
	return doc.ClientID, doc.ClientSecret
}

func (e *oauthEnv) authorize(clientID, redirect, challenge, method, state string) oauthHTTP {
	e.t.Helper()
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirect},
		"code_challenge":        {challenge},
		"code_challenge_method": {method},
		"state":                 {state},
	}
	return e.get("/authorize?" + q.Encode())
}

func (e *oauthEnv) authorizationCode(email, sub string) (code, verifier, clientID string) {
	e.t.Helper()
	code, verifier, clientID, _ = e.authorizationCodeMethod(email, sub, "none")
	return code, verifier, clientID
}

func (e *oauthEnv) authorizationCodeMethod(email, sub, method string) (code, verifier, clientID, secret string) {
	e.t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		e.t.Fatal(err)
	}
	e.client.Jar = jar
	clientID, secret = e.registerClientSecret(oauthRedirect, method)
	pair := pkcePair(e.t)
	start := e.authorize(clientID, oauthRedirect, pair.challenge, "S256", "client-state")
	page := e.finishGoogle(start, email, sub, true)
	if page.StatusCode != http.StatusOK {
		e.t.Fatalf("consent %d %s", page.StatusCode, page.Body)
	}
	reqID, csrf := consentFields(e.t, page.Body)
	approved := e.consent(reqID, csrf, "approve", e.base)
	loc := mustRedirect(e.t, approved, http.StatusSeeOther)
	q := mustParse(e.t, loc).Query()
	if q.Get("state") != "client-state" || q.Get("code") == "" || q.Get("error") != "" {
		e.t.Fatalf("approval %s", loc)
	}
	return q.Get("code"), pair.verifier, clientID, secret
}

func (e *oauthEnv) finishGoogle(start oauthHTTP, email, sub string, verified bool) oauthHTTP {
	e.t.Helper()
	if start.StatusCode != http.StatusFound {
		e.t.Fatalf("google start %d %s", start.StatusCode, start.Body)
	}
	gu := mustParse(e.t, start.Header.Get("Location"))
	q := gu.Query()
	redir := q.Get("redirect_uri")
	if redir != e.base+"/oauth/google/callback" {
		e.t.Fatalf("google redirect_uri %q", redir)
	}
	e.fake.prepare(q.Get("code_challenge"), q.Get("nonce"), email, sub, verified)
	req := e.do(http.MethodGet, "/oauth/google/callback?code=authcode&state="+url.QueryEscape(q.Get("state")), nil, "")
	for _, c := range start.Cookies {
		req.AddCookie(c)
	}
	return e.roundTrip(req)
}

func (e *oauthEnv) consent(reqID, csrf, action, origin string) oauthHTTP {
	e.t.Helper()
	form := url.Values{"req": {reqID}, "_csrf": {csrf}, "action": {action}}.Encode()
	req := e.do(http.MethodPost, "/authorize", strings.NewReader(form), "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", origin)
	return e.roundTrip(req)
}

func (e *oauthEnv) exchange(clientID, code, verifier, redirect, secret, basic string) tokenResponse {
	e.t.Helper()
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"client_id":     {clientID},
	}
	if redirect != "" {
		form.Set("redirect_uri", redirect)
	}
	if secret != "" {
		form.Set("client_secret", secret)
	}
	res := e.tokenForm(form, basic)
	if res.StatusCode != http.StatusOK {
		e.t.Fatalf("token %d %s", res.StatusCode, res.Body)
	}
	var tok tokenResponse
	if err := json.Unmarshal([]byte(res.Body), &tok); err != nil {
		e.t.Fatal(err)
	}
	return tok
}

func (e *oauthEnv) exchangeCode(code, verifier, clientID string) tokenResponse {
	e.t.Helper()
	return e.exchange(clientID, code, verifier, oauthRedirect, "", "")
}

func (e *oauthEnv) tokenForm(form url.Values, authz string) oauthHTTP {
	e.t.Helper()
	req := e.do(http.MethodPost, "/token", strings.NewReader(form.Encode()), authz)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return e.roundTrip(req)
}

func pkcePair(t *testing.T) pkce {
	t.Helper()
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	return pkce{verifier: verifier, challenge: base64.RawURLEncoding.EncodeToString(sum[:])}
}

func mustRedirect(t *testing.T, res oauthHTTP, status int) string {
	t.Helper()
	if res.StatusCode != status {
		t.Fatalf("status %d body %s", res.StatusCode, res.Body)
	}
	loc := res.Header.Get("Location")
	if loc == "" {
		t.Fatalf("missing location body %s", res.Body)
	}
	return loc
}

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func consentFields(t *testing.T, body string) (reqID, csrf string) {
	t.Helper()
	reqID = attr(t, body, "req")
	csrf = attr(t, body, "_csrf")
	return reqID, csrf
}

func attr(t *testing.T, body, name string) string {
	t.Helper()
	needle := `name="` + name + `" value="`
	i := strings.Index(body, needle)
	if i < 0 {
		t.Fatalf("missing %s in %s", name, body)
	}
	rest := body[i+len(needle):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		t.Fatalf("unclosed %s", name)
	}
	return rest[:j]
}

func cookieByName(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("cookie %s missing in %+v", name, cookies)
	return nil
}

func basicHeader(id, secret string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(id+":"+secret))
}

type oauthGoogle struct {
	key      *rsa.PrivateKey
	kid      string
	clientID string
	secret   string
	issuer   string
	now      time.Time
	srv      *httptest.Server

	mu        sync.Mutex
	challenge string
	email     string
	sub       string
	verified  bool
	nonce     string
}

func (f *oauthGoogle) prepare(challenge, nonce, email, sub string, verified bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.challenge = challenge
	f.nonce = nonce
	f.email = email
	f.sub = sub
	f.verified = verified
}

func (f *oauthGoogle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/token":
		f.serveToken(w, r)
	case "/jwks":
		f.serveJWKS(w)
	default:
		http.NotFound(w, r)
	}
}

func (f *oauthGoogle) serveToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form", http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	challenge, nonce, email, sub, verified := f.challenge, f.nonce, f.email, f.sub, f.verified
	f.mu.Unlock()
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != challenge || r.PostForm.Get("client_id") != f.clientID || r.PostForm.Get("client_secret") != f.secret {
		http.Error(w, "client", http.StatusBadRequest)
		return
	}
	claims := map[string]any{
		"iss":            f.issuer,
		"aud":            f.clientID,
		"sub":            sub,
		"email":          email,
		"email_verified": verified,
		"nonce":          nonce,
		"exp":            f.now.Add(time.Hour).Unix(),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id_token": signOAuthIDToken(f.key, f.kid, claims)})
}

func (f *oauthGoogle) serveJWKS(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA",
			"use": "sig",
			"alg": "RS256",
			"kid": f.kid,
			"n":   base64.RawURLEncoding.EncodeToString(f.key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(f.key.E)).Bytes()),
		}},
	})
}

func signOAuthIDToken(key *rsa.PrivateKey, kid string, claims map[string]any) string {
	head, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": kid, "typ": "JWT"})
	body, _ := json.Marshal(claims)
	h := base64.RawURLEncoding.EncodeToString(head)
	b := base64.RawURLEncoding.EncodeToString(body)
	sum := sha256.Sum256([]byte(h + "." + b))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		panic(err)
	}
	return h + "." + b + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestOAuthExtraRedirectURI(t *testing.T) {
	env := newOAuthEnv(t, false)
	const extra = "https://client.example/callback"
	env.svc.oauthRedirects = []string{extra}
	ok := env.registerRaw(map[string]any{"redirect_uris": []string{extra}, "client_name": "extra"})
	if ok.StatusCode != http.StatusCreated {
		t.Fatalf("extra %d %s", ok.StatusCode, ok.Body)
	}
	frag := env.registerRaw(map[string]any{"redirect_uris": []string{extra + "#frag"}})
	if frag.StatusCode != http.StatusBadRequest || !strings.Contains(frag.Body, "redirect_uri is not allowed by this server: ") {
		t.Fatalf("fragment %d %s", frag.StatusCode, frag.Body)
	}
}

func TestOAuthCursorSchemeEndToEnd(t *testing.T) {
	const redirect = "cursor://anysphere.cursor-mcp/oauth/callback"
	env := newOAuthEnv(t, true)
	id := env.registerClient(redirect, "none")
	pair := pkcePair(t)
	start := env.authorize(id, redirect, pair.challenge, "S256", "client-state")
	page := env.finishGoogle(start, oauthLearner, "learner-sub", true)
	if page.StatusCode != http.StatusOK || !strings.Contains(page.Body, "(cursor:)") {
		t.Fatalf("consent %d %s", page.StatusCode, page.Body)
	}
	reqID, csrf := consentFields(t, page.Body)
	approved := env.consent(reqID, csrf, "approve", env.base)
	if approved.StatusCode != http.StatusOK || approved.Header.Get("Location") != "" {
		t.Fatalf("approve %d location %q %s", approved.StatusCode, approved.Header.Get("Location"), approved.Body)
	}
	if approved.Header.Get("Content-Security-Policy") != "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'" {
		t.Fatalf("csp %q", approved.Header.Get("Content-Security-Policy"))
	}
	if !strings.Contains(approved.Body, "Signed in. Returning to the app.") || !strings.Contains(approved.Body, "Return to the app") {
		t.Fatalf("page %s", approved.Body)
	}
	code := codeFromCustomPage(t, approved.Body)
	tok := env.exchange(id, code, pair.verifier, redirect, "", "")
	if tok.AccessToken == "" || tok.TokenType != "Bearer" {
		t.Fatalf("token %+v", tok)
	}

	deniedStart := env.authorize(id, redirect, pkcePair(t).challenge, "S256", "deny-state")
	if deniedStart.StatusCode != http.StatusOK || !strings.Contains(deniedStart.Body, "(cursor:)") {
		t.Fatalf("second consent %d %s", deniedStart.StatusCode, deniedStart.Body)
	}
	denyID, denyCSRF := consentFields(t, deniedStart.Body)
	denied := env.consent(denyID, denyCSRF, "deny", env.base)
	if denied.StatusCode != http.StatusOK || !strings.Contains(denied.Body, "Access was not granted.") {
		t.Fatalf("deny %d %s", denied.StatusCode, denied.Body)
	}
}

func TestEmptyAllowlistDoesNotRedirectToGoogle(t *testing.T) {
	env := newOAuthEnv(t, true)
	env.access.Set(nil, []string{oauthAdmin})
	id := env.registerClient(oauthRedirect, "none")
	res := env.authorize(id, oauthRedirect, pkcePair(t).challenge, "S256", "st")
	if res.StatusCode != http.StatusServiceUnavailable || !strings.Contains(res.Body, "No allowed email is set.") {
		t.Fatalf("empty allowlist %d %s", res.StatusCode, res.Body)
	}
	if res.Header.Get("Location") != "" {
		t.Fatalf("redirected to %s", res.Header.Get("Location"))
	}
	n, err := env.store.CountOAuthRequests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("authorize wrote %d requests", n)
	}
}

func TestTokenBurnsCodeWhenAllowlistRemovesUser(t *testing.T) {
	env := newOAuthEnv(t, true)
	code, verifier, clientID := env.authorizationCode(oauthLearner, "learner-sub")
	env.access.Set(nil, nil)
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {oauthRedirect},
		"client_id":     {clientID},
	}
	rejected := env.tokenForm(form, "")
	if rejected.StatusCode != http.StatusBadRequest || !strings.Contains(rejected.Body, "That account can no longer sign in.") {
		t.Fatalf("removed %d %s", rejected.StatusCode, rejected.Body)
	}
	env.access.Set([]string{oauthLearner}, []string{oauthAdmin})
	again := env.tokenForm(form, "")
	if again.StatusCode != http.StatusBadRequest || !strings.Contains(again.Body, "Code was already used.") {
		t.Fatalf("code survived %d %s", again.StatusCode, again.Body)
	}
}

func TestMCPSessionCookieIsHostOnly(t *testing.T) {
	env := newOAuthEnvSecure(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	env.client.Jar = jar
	id := env.registerClient(oauthRedirect, "none")
	start := env.authorize(id, oauthRedirect, pkcePair(t).challenge, "S256", "st")
	page := env.finishGoogle(start, oauthLearner, "learner-sub", true)
	session := cookieByName(t, page.Cookies, "__Host-hebrew_session")
	if session.Domain != "" || !session.Secure || !session.HttpOnly || session.Path != "/" || session.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie %+v", session)
	}
	for _, raw := range page.Header.Values("Set-Cookie") {
		if strings.Contains(strings.ToLower(raw), "domain=") {
			t.Fatalf("domain attribute %s", raw)
		}
	}
}

func TestAuthorizeHeadWritesNoRow(t *testing.T) {
	env := newOAuthEnv(t, true)
	id := env.registerClient(oauthRedirect, "none")
	pair := pkcePair(t)
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {id},
		"redirect_uri":          {oauthRedirect},
		"code_challenge":        {pair.challenge},
		"code_challenge_method": {"S256"},
		"state":                 {"st"},
	}
	res := env.roundTrip(env.do(http.MethodHead, "/authorize?"+q.Encode(), nil, ""))
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("head %d %s", res.StatusCode, res.Body)
	}
	n, err := env.store.CountOAuthRequests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("head wrote %d requests", n)
	}
}

func TestOAuthSweepTTLCapAndRateLimit(t *testing.T) {
	env := newOAuthEnv(t, false)
	id := env.registerClient(oauthRedirect, "none")
	if err := env.store.InsertOAuthRequest(context.Background(), db.OAuthRequest{
		ID: "old-req", ClientID: id, RedirectURI: oauthRedirect,
		CodeChallenge: strings.Repeat("a", 43), ExpiresAt: env.clk.Now().Add(-time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.store.InsertOAuthClient(context.Background(), db.OAuthClient{
		ID: "old-client", Name: "old", RedirectURIs: []string{oauthRedirect}, AuthMethod: "none",
		GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"code"},
		CreatedAt: env.clk.Now().Add(-OAuthClientTTL - time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	stale := env.authorize("old-client", oauthRedirect, pkcePair(t).challenge, "S256", "st")
	if stale.StatusCode != http.StatusBadRequest || !strings.Contains(stale.Body, "Unknown or expired client") {
		t.Fatalf("ttl %d %s", stale.StatusCode, stale.Body)
	}
	if err := env.store.SweepOAuth(context.Background(), env.clk.Now(), env.clk.Now().Add(-OAuthClientTTL), env.clk.Now().Add(-OAuthIdleClientTTL)); err != nil {
		t.Fatal(err)
	}
	if _, err := env.store.OAuthRequest(context.Background(), "old-req"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("request sweep %v", err)
	}
	if _, err := env.store.OAuthClient(context.Background(), "old-client"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("client sweep %v", err)
	}

	prevCap := oauthClientCap
	oauthClientCap = 1
	t.Cleanup(func() { oauthClientCap = prevCap })
	capped := env.registerRaw(map[string]any{"redirect_uris": []string{oauthRedirect}})
	if capped.StatusCode != http.StatusTooManyRequests || !strings.Contains(capped.Body, "Too many registered clients.") {
		t.Fatalf("cap %d %s", capped.StatusCode, capped.Body)
	}
	oauthClientCap = prevCap

	prevRate := oauthRateLimit
	oauthRateLimit = 2
	t.Cleanup(func() { oauthRateLimit = prevRate })
	limited := newOAuthEnv(t, false)
	for i := 0; i < 2; i++ {
		ok := limited.registerRaw(map[string]any{"redirect_uris": []string{oauthRedirect}})
		if ok.StatusCode != http.StatusCreated {
			t.Fatalf("rate %d %d %s", i, ok.StatusCode, ok.Body)
		}
	}
	blocked := limited.registerRaw(map[string]any{"redirect_uris": []string{oauthRedirect}})
	if blocked.StatusCode != http.StatusTooManyRequests || !strings.Contains(blocked.Body, "Too many requests from this address.") {
		t.Fatalf("rate limit %d %s", blocked.StatusCode, blocked.Body)
	}
}

func TestLoadClientRejectsExpiredTTL(t *testing.T) {
	env := newOAuthEnv(t, false)
	live := env.registerClient(oauthRedirect, "none")
	h := &oauthHandler{svc: env.svc}
	if _, err := h.loadClient(context.Background(), live); err != nil {
		t.Fatal(err)
	}
	if err := env.store.InsertOAuthClient(context.Background(), db.OAuthClient{
		ID: "aged", Name: "aged", RedirectURIs: []string{oauthRedirect}, AuthMethod: "none",
		GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"code"},
		CreatedAt: env.clk.Now().Add(-OAuthClientTTL),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.loadClient(context.Background(), "aged"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("past ttl %v", err)
	}
	if err := env.store.InsertOAuthClient(context.Background(), db.OAuthClient{
		ID: "inside", Name: "inside", RedirectURIs: []string{oauthRedirect}, AuthMethod: "none",
		GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"code"},
		CreatedAt: env.clk.Now().Add(-OAuthClientTTL + time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.loadClient(context.Background(), "inside"); err != nil {
		t.Fatalf("inside ttl %v", err)
	}
}

func TestOAuthIdleClientPrune(t *testing.T) {
	env := newOAuthEnv(t, false)
	ctx := context.Background()
	now := env.clk.Now()
	user, err := env.store.CreateUser(ctx, oauthLearner, "learner-sub", RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	insertClient := func(id string, age time.Duration) {
		t.Helper()
		if err := env.store.InsertOAuthClient(ctx, db.OAuthClient{
			ID: id, Name: id, RedirectURIs: []string{oauthRedirect}, AuthMethod: "none",
			GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"code"},
			CreatedAt: now.Add(-age),
		}); err != nil {
			t.Fatal(err)
		}
	}
	insertClient("idle", 25*time.Hour)
	insertClient("coded", 25*time.Hour)
	insertClient("tokened", 25*time.Hour)
	insertClient("young", 23*time.Hour)
	insertClient("ancient", OAuthClientTTL+time.Second)
	// Raw rows leave issued at 0. Sweep has to notice the code and token
	// before it deletes those expired rows.
	if _, err := env.store.ExecContext(ctx, `
		INSERT INTO oauth_codes (code_hash, client_id, user_id, code_challenge, expires_at)
		VALUES ('code-hash', 'coded', ?, ?, ?)`,
		user.ID, strings.Repeat("a", 43), db.FormatTime(now.Add(-time.Minute))); err != nil {
		t.Fatal(err)
	}
	if _, err := env.store.ExecContext(ctx, `
		INSERT INTO oauth_tokens (token_hash, kind, client_id, user_id, family_id, expires_at, created_at)
		VALUES ('token-hash', 'access', 'tokened', ?, 'fam', ?, ?)`,
		user.ID, db.FormatTime(now.Add(-time.Minute)), db.FormatTime(now.Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	if _, err := env.store.ExecContext(ctx, `
		INSERT INTO oauth_codes (code_hash, client_id, user_id, code_challenge, expires_at)
		VALUES ('ancient-code', 'ancient', ?, ?, ?)`,
		user.ID, strings.Repeat("b", 43), db.FormatTime(now.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	if err := env.store.SweepOAuth(ctx, now, now.Add(-OAuthClientTTL), now.Add(-OAuthIdleClientTTL)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"idle", "ancient"} {
		if _, err := env.store.OAuthClient(ctx, id); !errors.Is(err, db.ErrNotFound) {
			t.Fatalf("%s survived: %v", id, err)
		}
	}
	for _, id := range []string{"coded", "tokened", "young"} {
		if _, err := env.store.OAuthClient(ctx, id); err != nil {
			t.Fatalf("%s missing: %v", id, err)
		}
	}
	if _, err := env.store.OAuthCode(ctx, "code-hash"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("expired code kept: %v", err)
	}
	if _, err := env.store.OAuthToken(ctx, "token-hash"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("expired token kept: %v", err)
	}
	if _, err := env.store.OAuthCode(ctx, "ancient-code"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("ancient code kept: %v", err)
	}

	prev := oauthClientCap
	oauthClientCap = 1
	t.Cleanup(func() { oauthClientCap = prev })
	blocked := newOAuthEnv(t, false)
	if err := blocked.store.InsertOAuthClient(context.Background(), db.OAuthClient{
		ID: "recent-idle", Name: "recent", RedirectURIs: []string{oauthRedirect}, AuthMethod: "none",
		GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"code"},
		CreatedAt: blocked.clk.Now().Add(-23 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if res := blocked.registerRaw(map[string]any{"redirect_uris": []string{oauthRedirect}}); res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("23h idle still holds a slot %d %s", res.StatusCode, res.Body)
	}
	freed := newOAuthEnv(t, false)
	if err := freed.store.InsertOAuthClient(context.Background(), db.OAuthClient{
		ID: "dead-idle", Name: "dead", RedirectURIs: []string{oauthRedirect}, AuthMethod: "none",
		GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"code"},
		CreatedAt: freed.clk.Now().Add(-25 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if res := freed.registerRaw(map[string]any{"redirect_uris": []string{oauthRedirect}}); res.StatusCode != http.StatusCreated {
		t.Fatalf("25h idle lockout %d %s", res.StatusCode, res.Body)
	}
	if _, err := freed.store.OAuthClient(context.Background(), "dead-idle"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("dead registration kept: %v", err)
	}
}

func TestOAuthRateLimitUsesTrustedClientIP(t *testing.T) {
	prev := oauthRateLimit
	oauthRateLimit = 1
	t.Cleanup(func() { oauthRateLimit = prev })

	trusted := newOAuthEnvProxied(t)
	first := trusted.registerForwarded("203.0.113.10, 127.0.0.1")
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first forwarded %d %s", first.StatusCode, first.Body)
	}
	same := trusted.registerForwarded("203.0.113.10, 127.0.0.1")
	if same.StatusCode != http.StatusTooManyRequests || !strings.Contains(same.Body, "Too many requests from this address.") {
		t.Fatalf("same forwarded ip %d %s", same.StatusCode, same.Body)
	}
	other := trusted.registerForwarded("203.0.113.11, 127.0.0.1")
	if other.StatusCode != http.StatusCreated {
		t.Fatalf("second forwarded ip %d %s", other.StatusCode, other.Body)
	}
	if keys := trusted.rateKeys(); len(keys) != 2 || keys[0] != "203.0.113.10" || keys[1] != "203.0.113.11" {
		t.Fatalf("trusted buckets %v", keys)
	}

	plain := newOAuthEnv(t, false)
	spoofed := plain.registerForwarded("203.0.113.10")
	if spoofed.StatusCode != http.StatusCreated {
		t.Fatalf("untrusted first %d %s", spoofed.StatusCode, spoofed.Body)
	}
	if keys := plain.rateKeys(); len(keys) != 1 || keys[0] != "127.0.0.1" {
		t.Fatalf("untrusted key %v", keys)
	}
	otherSpoof := plain.registerForwarded("198.51.100.20")
	if otherSpoof.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("untrusted header opened a second bucket %d %s", otherSpoof.StatusCode, otherSpoof.Body)
	}
	if keys := plain.rateKeys(); len(keys) != 1 || keys[0] != "127.0.0.1" {
		t.Fatalf("untrusted keys after second header %v", keys)
	}
}

func TestOAuthRateMapShrinksAfterWindow(t *testing.T) {
	env := newOAuthEnv(t, false)
	now := env.clk.Now()
	if !env.svc.rateAllow("203.0.113.1", now) || !env.svc.rateAllow("203.0.113.2", now) {
		t.Fatal("rate allow")
	}
	if keys := env.rateKeys(); len(keys) != 2 {
		t.Fatalf("filled %v", keys)
	}
	env.clk.Advance(oauthRateWindow - time.Second)
	env.svc.PruneRateLimits(env.clk.Now())
	if keys := env.rateKeys(); len(keys) != 2 {
		t.Fatalf("window still open %v", keys)
	}
	env.clk.Advance(time.Second)
	res := env.registerRaw(map[string]any{"redirect_uris": []string{oauthRedirect}})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register after window %d %s", res.StatusCode, res.Body)
	}
	if keys := env.rateKeys(); len(keys) != 1 || keys[0] != "127.0.0.1" {
		t.Fatalf("sweep left %v", keys)
	}
}

func (e *oauthEnv) registerForwarded(forwarded string) oauthHTTP {
	e.t.Helper()
	raw, err := json.Marshal(map[string]any{"redirect_uris": []string{oauthRedirect}})
	if err != nil {
		e.t.Fatal(err)
	}
	req := e.do(http.MethodPost, "/register", strings.NewReader(string(raw)), "")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", forwarded)
	return e.roundTrip(req)
}

func (e *oauthEnv) rateKeys() []string {
	e.svc.rateMu.Lock()
	defer e.svc.rateMu.Unlock()
	keys := make([]string, 0, len(e.svc.rate))
	for k := range e.svc.rate {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func codeFromCustomPage(t *testing.T, body string) string {
	t.Helper()
	const needle = "code="
	i := strings.Index(body, needle)
	if i < 0 {
		t.Fatalf("no code in %s", body)
	}
	rest := body[i+len(needle):]
	end := strings.IndexAny(rest, "&\"")
	if end < 0 {
		t.Fatalf("code tail %s", rest)
	}
	return rest[:end]
}
