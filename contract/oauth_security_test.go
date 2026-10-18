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

const basicRealm = `Basic realm="hebrew-lessons"`

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
	assertClientAuth(t, missing, "")

	empty := cloneValues(postBody)
	empty.Set("client_secret", "")
	assertClientAuth(t, tokenRequest(t, c, empty, nil), "")

	wrong := cloneValues(postBody)
	wrong.Set("client_secret", "not-the-secret")
	assertClientAuth(t, tokenRequest(t, c, wrong, nil), "")

	// A rejected secret does not burn the code.
	ok := cloneValues(postBody)
	ok.Set("client_secret", post.Secret)
	postRefresh := issuedRefresh(t, tokenRequest(t, c, ok, nil))

	postRefreshBody := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {postRefresh},
		"client_id":     {post.ID},
	}
	assertClientAuth(t, tokenRequest(t, c, postRefreshBody, nil), "")
	postRefreshWrong := cloneValues(postRefreshBody)
	postRefreshWrong.Set("client_secret", "not-the-secret")
	assertClientAuth(t, tokenRequest(t, c, postRefreshWrong, nil), "")
	postRefreshEmpty := cloneValues(postRefreshBody)
	postRefreshEmpty.Set("client_secret", "")
	assertClientAuth(t, tokenRequest(t, c, postRefreshEmpty, nil), "")
	// A rejected refresh does not rotate the token.
	postRefreshOK := cloneValues(postRefreshBody)
	postRefreshOK.Set("client_secret", post.Secret)
	assertAccessToken(t, tokenRequest(t, c, postRefreshOK, nil))

	basicClient := registerOAuth(t, c, "client_secret_basic", []string{oauthRedirect})
	basicVerifier, basicChallenge := pkcePair(t)
	basicCode := authorizationCode(t, c, basicClient.ID, basicChallenge, oauthRedirect)
	basicBody := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {basicCode},
		"redirect_uri":  {oauthRedirect},
		"code_verifier": {basicVerifier},
	}

	assertClientAuth(t, tokenRequest(t, c, basicBody, nil), basicRealm)
	assertClientAuth(t, tokenRequest(t, c, basicBody, map[string]string{
		"Authorization": basicHeader(basicClient.ID, ""),
	}), basicRealm)
	assertClientAuth(t, tokenRequest(t, c, basicBody, map[string]string{
		"Authorization": basicHeader(basicClient.ID, "not-the-secret"),
	}), basicRealm)

	basicRefresh := issuedRefresh(t, tokenRequest(t, c, basicBody, map[string]string{
		"Authorization": basicHeader(basicClient.ID, basicClient.Secret),
	}))
	basicRefreshBody := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {basicRefresh},
		"client_id":     {basicClient.ID},
	}
	assertClientAuth(t, tokenRequest(t, c, basicRefreshBody, nil), basicRealm)
	assertClientAuth(t, tokenRequest(t, c, basicRefreshBody, map[string]string{
		"Authorization": basicHeader(basicClient.ID, ""),
	}), basicRealm)
	assertClientAuth(t, tokenRequest(t, c, basicRefreshBody, map[string]string{
		"Authorization": basicHeader(basicClient.ID, "not-the-secret"),
	}), basicRealm)
	assertAccessToken(t, tokenRequest(t, c, basicRefreshBody, map[string]string{
		"Authorization": basicHeader(basicClient.ID, basicClient.Secret),
	}))
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
	assertDescribed(t, stolen, 400, "invalid_grant", "client_id or redirect_uri does not match the code.")

	// A rejected client does not burn the code.
	kept := tokenRequest(t, c, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {owner.ID},
		"redirect_uri":  {oauthRedirect},
		"code_verifier": {verifier},
	}, nil)
	assertAccessToken(t, kept)
}

func TestRefreshTokenIsBoundToTheIssuingClient(t *testing.T) {
	c := httpx.New()
	owner := registerOAuth(t, c, "", []string{oauthRedirect})
	other := registerOAuth(t, c, "", []string{oauthRedirect})
	verifier, challenge := pkcePair(t)
	code := authorizationCode(t, c, owner.ID, challenge, oauthRedirect)
	refresh := issuedRefresh(t, tokenRequest(t, c, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {owner.ID},
		"redirect_uri":  {oauthRedirect},
		"code_verifier": {verifier},
	}, nil))

	stolen := tokenRequest(t, c, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refresh},
		"client_id":     {other.ID},
	}, nil)
	assertDescribed(t, stolen, 400, "invalid_grant", "client_id does not match the refresh token.")

	// A rejected client does not rotate the refresh token.
	kept := tokenRequest(t, c, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refresh},
		"client_id":     {owner.ID},
	}, nil)
	assertAccessToken(t, kept)
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
	assertDescribed(t, mismatched, 400, "invalid_grant", "client_id or redirect_uri does not match the code.")

	// A rejected redirect does not burn the code.
	kept := tokenRequest(t, c, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {client.ID},
		"redirect_uri":  {oauthRedirect},
		"code_verifier": {verifier},
	}, nil)
	assertAccessToken(t, kept)
}

func TestTokenRejectsAnEmptyRedirectURI(t *testing.T) {
	c := httpx.New()
	client := registerOAuth(t, c, "", []string{oauthRedirect})
	verifier, challenge := pkcePair(t)
	code := authorizationCode(t, c, client.ID, challenge, oauthRedirect)
	empty := tokenRequest(t, c, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {client.ID},
		"redirect_uri":  {""},
		"code_verifier": {verifier},
	}, nil)
	assertDescribed(t, empty, 400, "invalid_grant", "client_id or redirect_uri does not match the code.")

	kept := tokenRequest(t, c, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {client.ID},
		"redirect_uri":  {oauthRedirect},
		"code_verifier": {verifier},
	}, nil)
	assertAccessToken(t, kept)
}

func TestTokenRejectsABadVerifierWhenRedirectURIIsOmitted(t *testing.T) {
	c := httpx.New()
	client := registerOAuth(t, c, "", []string{oauthRedirect})
	verifier, challenge := pkcePair(t)
	code := authorizationCode(t, c, client.ID, challenge, oauthRedirect)
	wrong, _ := pkcePair(t)

	bad := tokenRequest(t, c, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {client.ID},
		"code_verifier": {wrong},
	}, nil)
	assertDescribed(t, bad, 400, "invalid_grant", "PKCE verification failed.")

	missing := tokenRequest(t, c, url.Values{
		"grant_type": {"authorization_code"},
		"code":       {code},
		"client_id":  {client.ID},
	}, nil)
	assertDescribed(t, missing, 400, "invalid_grant", "PKCE verification failed.")

	// Neither failure burns the code.
	kept := tokenRequest(t, c, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {client.ID},
		"redirect_uri":  {oauthRedirect},
		"code_verifier": {verifier},
	}, nil)
	assertAccessToken(t, kept)
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

func assertDescribed(t *testing.T, res *httpx.Exchange, status int, code, description string) {
	t.Helper()
	assertOAuth(t, res, status, code)
	var body struct {
		Description string `json:"error_description"`
	}
	if err := json.Unmarshal(res.Body, &body); err != nil {
		t.Fatalf("body %s: %v", res.Body, err)
	}
	if body.Description != description {
		t.Fatalf("error_description\n got %q\nwant %q", body.Description, description)
	}
}

func assertClientAuth(t *testing.T, res *httpx.Exchange, wwwAuthenticate string) {
	t.Helper()
	assertDescribed(t, res, 401, "invalid_client", "Client authentication failed.")
	if got := res.Header.Get("WWW-Authenticate"); got != wwwAuthenticate {
		t.Fatalf("WWW-Authenticate\n got %q\nwant %q", got, wwwAuthenticate)
	}
}

func issuedRefresh(t *testing.T, res *httpx.Exchange) string {
	t.Helper()
	assertAccessToken(t, res)
	var body struct {
		Refresh string `json:"refresh_token"`
	}
	if err := json.Unmarshal(res.Body, &body); err != nil || body.Refresh == "" {
		t.Fatalf("refresh token: %s", res.Body)
	}
	return body.Refresh
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
