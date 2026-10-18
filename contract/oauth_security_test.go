package contract

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"html"
	"net/url"
	"os"
	"regexp"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/httpx"
)

// Live black-box checks for the OAuth token endpoint. They do not record a
// golden. In-memory authorization codes and pending sign-ins stay out of the
// suite's recorded files.
//
// The Go server does not implement these routes yet. The contract workflow
// already marks that leg continue-on-error, and this file does not add a skip.

const oauthRedirect = "cursor://anysphere.cursor-mcp/oauth/callback"

func TestTokenChecksClientSecret(t *testing.T) {
	c := httpx.New()
	post := registerOAuth(t, c, "client_secret_post", []string{oauthRedirect})
	postVerifier, postChallenge := pkcePair(t)
	postCode := authorizationCode(t, c, post.ID, postChallenge, oauthRedirect)
	postBody := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {postCode},
		"client_id":     {post.ID},
		"redirect_uri":  {oauthRedirect},
		"code_verifier": {postVerifier},
	}

	missing := tokenRequest(t, c, postBody, nil)
	assertOAuth(t, missing, 401, "invalid_client")
	if got := missing.Header.Get("WWW-Authenticate"); got != "" {
		t.Fatalf("client_secret_post WWW-Authenticate %q", got)
	}

	wrong := cloneValues(postBody)
	wrong.Set("client_secret", "not-the-secret")
	assertOAuth(t, tokenRequest(t, c, wrong, nil), 401, "invalid_client")

	basicClient := registerOAuth(t, c, "client_secret_basic", []string{oauthRedirect})
	basicVerifier, basicChallenge := pkcePair(t)
	basicCode := authorizationCode(t, c, basicClient.ID, basicChallenge, oauthRedirect)
	basicBody := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {basicCode},
		"redirect_uri":  {oauthRedirect},
		"code_verifier": {basicVerifier},
	}
	const wantChallenge = `Basic realm="hebrew-lessons"`

	noHeader := tokenRequest(t, c, basicBody, nil)
	assertOAuth(t, noHeader, 401, "invalid_client")
	if got := noHeader.Header.Get("WWW-Authenticate"); got != wantChallenge {
		t.Fatalf("WWW-Authenticate\n got %q\nwant %q", got, wantChallenge)
	}

	badBasic := tokenRequest(t, c, basicBody, map[string]string{
		"Authorization": basicHeader(basicClient.ID, "not-the-secret"),
	})
	assertOAuth(t, badBasic, 401, "invalid_client")
	if got := badBasic.Header.Get("WWW-Authenticate"); got != wantChallenge {
		t.Fatalf("WWW-Authenticate\n got %q\nwant %q", got, wantChallenge)
	}
}

func TestAuthCodeIsBoundToTheRegisteringClient(t *testing.T) {
	c := httpx.New()
	owner := registerOAuth(t, c, "", []string{oauthRedirect})
	other := registerOAuth(t, c, "", []string{oauthRedirect})
	verifier, challenge := pkcePair(t)
	code := authorizationCode(t, c, owner.ID, challenge, oauthRedirect)

	stolen := tokenRequest(t, c, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {other.ID},
		"redirect_uri":  {oauthRedirect},
		"code_verifier": {verifier},
	}, nil)
	assertOAuth(t, stolen, 400, "invalid_grant")
}

func TestRefreshTokenIsBoundToTheIssuingClient(t *testing.T) {
	c := httpx.New()
	owner := registerOAuth(t, c, "", []string{oauthRedirect})
	other := registerOAuth(t, c, "", []string{oauthRedirect})
	verifier, challenge := pkcePair(t)
	code := authorizationCode(t, c, owner.ID, challenge, oauthRedirect)
	issued := tokenRequest(t, c, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {owner.ID},
		"redirect_uri":  {oauthRedirect},
		"code_verifier": {verifier},
	}, nil)
	if issued.Status != 200 {
		t.Fatalf("token %d %s", issued.Status, issued.Body)
	}
	var tokens struct {
		Refresh string `json:"refresh_token"`
	}
	if err := json.Unmarshal(issued.Body, &tokens); err != nil || tokens.Refresh == "" {
		t.Fatalf("refresh token: %s", issued.Body)
	}

	stolen := tokenRequest(t, c, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tokens.Refresh},
		"client_id":     {other.ID},
	}, nil)
	assertOAuth(t, stolen, 400, "invalid_grant")
}

func TestTokenAcceptsTheExactAuthorizationRedirectURI(t *testing.T) {
	c := httpx.New()
	client := registerOAuth(t, c, "", []string{oauthRedirect})
	verifier, challenge := pkcePair(t)
	code := authorizationCode(t, c, client.ID, challenge, oauthRedirect)
	res := tokenRequest(t, c, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {client.ID},
		"redirect_uri":  {oauthRedirect},
		"code_verifier": {verifier},
	}, nil)
	assertAccessToken(t, res)
}

func TestTokenRejectsADifferentRedirectURI(t *testing.T) {
	c := httpx.New()
	const other = "http://localhost:8787/callback"
	client := registerOAuth(t, c, "", []string{oauthRedirect, other})
	verifier, challenge := pkcePair(t)
	code := authorizationCode(t, c, client.ID, challenge, oauthRedirect)
	mismatched := tokenRequest(t, c, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {client.ID},
		"redirect_uri":  {other},
		"code_verifier": {verifier},
	}, nil)
	assertOAuth(t, mismatched, 400, "invalid_grant")
}

func TestTokenAcceptsMissingRedirectURIWhenPKCEVerifies(t *testing.T) {
	c := httpx.New()
	client := registerOAuth(t, c, "", []string{oauthRedirect})
	verifier, challenge := pkcePair(t)
	code := authorizationCode(t, c, client.ID, challenge, oauthRedirect)
	res := tokenRequest(t, c, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {client.ID},
		"code_verifier": {verifier},
	}, nil)
	assertAccessToken(t, res)
}

// /authorize requires S256 PKCE, so this server does not issue a code with no
// code_challenge. There is no live authorization code on which to redeem a
// missing redirect_uri without PKCE.
func TestAuthorizeDoesNotIssueACodeWithoutPKCE(t *testing.T) {
	c := httpx.New()
	client := registerOAuth(t, c, "", []string{oauthRedirect})
	q := url.Values{
		"response_type": {"code"},
		"client_id":     {client.ID},
		"redirect_uri":  {oauthRedirect},
		"state":         {"st8"},
	}
	res, err := c.Do(testCtx(t), "GET", publicURL(t)+"/authorize?"+q.Encode(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw := res.Header.Get("Location")
	if raw == "" {
		raw = html.UnescapeString(firstMatch(t, res.Body, `url=([^"]+)"`))
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("error") != "invalid_request" {
		t.Fatalf("error %q body %s", u.Query().Get("error"), res.Body)
	}
	if u.Query().Get("code") != "" {
		t.Fatalf("authorize issued a code without PKCE: %s", raw)
	}
}

type registeredClient struct {
	ID     string `json:"client_id"`
	Secret string `json:"client_secret"`
}

func registerOAuth(t *testing.T, c *httpx.Client, method string, redirects []string) registeredClient {
	t.Helper()
	payload := map[string]any{
		"client_name":   "Contract",
		"redirect_uris": redirects,
	}
	if method != "" {
		payload["token_endpoint_auth_method"] = method
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Do(testCtx(t), "POST", publicURL(t)+"/register", map[string]string{
		"Content-Type": "application/json",
	}, raw)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != 201 {
		t.Fatalf("register %d %s", res.Status, res.Body)
	}
	var client registeredClient
	if err := json.Unmarshal(res.Body, &client); err != nil || client.ID == "" {
		t.Fatalf("register body %s", res.Body)
	}
	if method != "" && client.Secret == "" {
		t.Fatalf("register omitted client_secret: %s", res.Body)
	}
	return client
}

func authorizationCode(t *testing.T, c *httpx.Client, clientID, challenge, redirect string) string {
	t.Helper()
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirect},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {"st8"},
	}
	page, err := c.Do(testCtx(t), "GET", publicURL(t)+"/authorize?"+q.Encode(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if page.Status != 200 {
		t.Fatalf("authorize %d %s", page.Status, page.Body)
	}
	req := firstMatch(t, page.Body, `name="req" value="([^"]+)"`)
	approved, err := c.Do(testCtx(t), "POST", publicURL(t)+"/authorize", map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}, []byte(url.Values{
		"req":      {html.UnescapeString(req)},
		"password": {os.Getenv("CONTRACT_MCP_TOKEN")},
		"action":   {"approve"},
	}.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	return codeFromRedirect(t, approved)
}

func tokenRequest(t *testing.T, c *httpx.Client, body url.Values, headers map[string]string) *httpx.Exchange {
	t.Helper()
	h := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	for k, v := range headers {
		h[k] = v
	}
	res, err := c.Do(testCtx(t), "POST", publicURL(t)+"/token", h, []byte(body.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func assertOAuth(t *testing.T, res *httpx.Exchange, status int, code string) {
	t.Helper()
	if res.Status != status {
		t.Fatalf("status %d body %s", res.Status, res.Body)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(res.Body, &body); err != nil {
		t.Fatalf("body %s: %v", res.Body, err)
	}
	if body.Error != code {
		t.Fatalf("error %q body %s", body.Error, res.Body)
	}
}

func assertAccessToken(t *testing.T, res *httpx.Exchange) {
	t.Helper()
	if res.Status != 200 {
		t.Fatalf("status %d body %s", res.Status, res.Body)
	}
	var body struct {
		TokenType   string `json:"token_type"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(res.Body, &body); err != nil {
		t.Fatalf("body %s: %v", res.Body, err)
	}
	if body.TokenType != "Bearer" || body.AccessToken == "" {
		t.Fatalf("token response %s", res.Body)
	}
}

func codeFromRedirect(t *testing.T, res *httpx.Exchange) string {
	t.Helper()
	raw := res.Header.Get("Location")
	if raw == "" {
		raw = html.UnescapeString(firstMatch(t, res.Body, `url=([^"]+)"`))
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	code := u.Query().Get("code")
	if code == "" {
		t.Fatalf("no code in %d %s", res.Status, raw)
	}
	return code
}

func firstMatch(t *testing.T, body []byte, pattern string) string {
	t.Helper()
	m := regexp.MustCompile(pattern).FindSubmatch(body)
	if m == nil {
		t.Fatalf("no %s in %s", pattern, body)
	}
	return string(m[1])
}

func pkcePair(t *testing.T) (verifier, challenge string) {
	t.Helper()
	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge
}

func basicHeader(id, secret string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(id+":"+secret))
}

func cloneValues(v url.Values) url.Values {
	out := url.Values{}
	for k, vals := range v {
		out[k] = append([]string(nil), vals...)
	}
	return out
}
