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

// These tests require the fixed MCP OAuth behavior. A confidential client must
// present the secret it registered, and a grant can be redeemed only by the
// client it was issued to. In-memory authorization codes and pending sign-ins
// stay as they are: that storage is not a bug, and these tests do not record
// a golden.

const oauthRedirect = "cursor://anysphere.cursor-mcp/oauth/callback"

const (
	errInvalidClient  = "invalid_client"
	errInvalidGrant   = "invalid_grant"
	descClientAuth    = "Client authentication failed."
	descCodeClient    = "client_id or redirect_uri does not match the code."
	descRefreshClient = "client_id does not match the refresh token."
	basicChallenge    = `Basic realm="hebrew-lessons"`
)

func TestTokenChecksClientSecret(t *testing.T) {
	post := registerOAuthClient(t, "Post client", "client_secret_post")
	postVerifier, postChallenge := pkce(t)
	postCode := authorizeCode(t, post.ID, postChallenge)

	t.Run("client_secret_post_wrong_secret", func(t *testing.T) {
		res := redeemCode(t, post.ID, postCode, oauthRedirect, postVerifier, strPtr("not-the-secret"), nil)
		assertOAuthError(t, res, 401, errInvalidClient, descClientAuth, "")
	})
	t.Run("client_secret_post_missing_secret", func(t *testing.T) {
		res := redeemCode(t, post.ID, postCode, oauthRedirect, postVerifier, nil, nil)
		assertOAuthError(t, res, 401, errInvalidClient, descClientAuth, "")
		empty := ""
		blank := redeemCode(t, post.ID, postCode, oauthRedirect, postVerifier, &empty, nil)
		assertOAuthError(t, blank, 401, errInvalidClient, descClientAuth, "")
	})
	var postRefresh string
	t.Run("client_secret_post_correct_client", func(t *testing.T) {
		res := redeemCode(t, post.ID, postCode, oauthRedirect, postVerifier, &post.Secret, nil)
		_, postRefresh = assertAccessToken(t, res)
	})
	t.Run("client_secret_post_refresh_wrong_secret", func(t *testing.T) {
		res := refreshToken(t, post.ID, postRefresh, strPtr("not-the-secret"), nil)
		assertOAuthError(t, res, 401, errInvalidClient, descClientAuth, "")
	})
	t.Run("client_secret_post_refresh_missing_secret", func(t *testing.T) {
		res := refreshToken(t, post.ID, postRefresh, nil, nil)
		assertOAuthError(t, res, 401, errInvalidClient, descClientAuth, "")
	})
	t.Run("client_secret_post_refresh_correct_client", func(t *testing.T) {
		res := refreshToken(t, post.ID, postRefresh, &post.Secret, nil)
		assertAccessToken(t, res)
	})

	basicClient := registerOAuthClient(t, "Basic client", "client_secret_basic")
	basicVerifier, basicChallengePKCE := pkce(t)
	basicCode := authorizeCode(t, basicClient.ID, basicChallengePKCE)
	wrongBasic := "not-the-secret"

	t.Run("client_secret_basic_wrong_secret", func(t *testing.T) {
		res := redeemCode(t, basicClient.ID, basicCode, oauthRedirect, basicVerifier, nil, &wrongBasic)
		assertOAuthError(t, res, 401, errInvalidClient, descClientAuth, basicChallenge)
	})
	t.Run("client_secret_basic_missing_secret", func(t *testing.T) {
		// Authorization: Basic with an empty secret, and a request that sends no secret at all.
		empty := ""
		viaBasic := redeemCode(t, basicClient.ID, basicCode, oauthRedirect, basicVerifier, nil, &empty)
		assertOAuthError(t, viaBasic, 401, errInvalidClient, descClientAuth, basicChallenge)
		omitted := redeemCode(t, basicClient.ID, basicCode, oauthRedirect, basicVerifier, nil, nil)
		assertOAuthError(t, omitted, 401, errInvalidClient, descClientAuth, basicChallenge)
	})
	var basicRefresh string
	t.Run("client_secret_basic_correct_client", func(t *testing.T) {
		res := redeemCode(t, basicClient.ID, basicCode, oauthRedirect, basicVerifier, nil, &basicClient.Secret)
		_, basicRefresh = assertAccessToken(t, res)
	})
	t.Run("client_secret_basic_refresh_wrong_secret", func(t *testing.T) {
		res := refreshToken(t, basicClient.ID, basicRefresh, nil, &wrongBasic)
		assertOAuthError(t, res, 401, errInvalidClient, descClientAuth, basicChallenge)
	})
	t.Run("client_secret_basic_refresh_missing_secret", func(t *testing.T) {
		empty := ""
		viaBasic := refreshToken(t, basicClient.ID, basicRefresh, nil, &empty)
		assertOAuthError(t, viaBasic, 401, errInvalidClient, descClientAuth, basicChallenge)
		omitted := refreshToken(t, basicClient.ID, basicRefresh, nil, nil)
		assertOAuthError(t, omitted, 401, errInvalidClient, descClientAuth, basicChallenge)
	})
	t.Run("client_secret_basic_refresh_correct_client", func(t *testing.T) {
		res := refreshToken(t, basicClient.ID, basicRefresh, nil, &basicClient.Secret)
		assertAccessToken(t, res)
	})
}

func TestAuthCodeIsBoundToTheRegisteringClient(t *testing.T) {
	owner := registerOAuthClient(t, "Code owner", "")
	other := registerOAuthClient(t, "Code other", "")
	if owner.ID == other.ID {
		t.Fatal("clients must be distinct")
	}
	if owner.Secret != "" || other.Secret != "" {
		t.Fatal("public clients have no secret")
	}
	verifier, challenge := pkce(t)
	code := authorizeCode(t, owner.ID, challenge)

	var refresh string
	t.Run("authorization_code", func(t *testing.T) {
		stolen := redeemCode(t, other.ID, code, oauthRedirect, verifier, nil, nil)
		assertOAuthError(t, stolen, 400, errInvalidGrant, descCodeClient, "")
		// The issuing client can still redeem. A rejected client does not burn the code.
		kept := redeemCode(t, owner.ID, code, oauthRedirect, verifier, nil, nil)
		_, refresh = assertAccessToken(t, kept)
	})
	t.Run("refresh_token", func(t *testing.T) {
		stolen := refreshToken(t, other.ID, refresh, nil, nil)
		assertOAuthError(t, stolen, 400, errInvalidGrant, descRefreshClient, "")
		ok := refreshToken(t, owner.ID, refresh, nil, nil)
		assertAccessToken(t, ok)
	})
}

type oauthClient struct {
	ID     string
	Secret string
	Method string
}

func registerOAuthClient(t *testing.T, name, method string) oauthClient {
	t.Helper()
	body := map[string]any{
		"client_name":   name,
		"redirect_uris": []string{oauthRedirect},
	}
	if method != "" {
		body["token_endpoint_auth_method"] = method
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	res := oauthDo(t, "POST", "/register", map[string]string{
		"Content-Type": "application/json",
	}, raw)
	if res.Status != 201 {
		t.Fatalf("register %s: %d %s", method, res.Status, res.Body)
	}
	var doc struct {
		ID     string `json:"client_id"`
		Secret string `json:"client_secret"`
		Method string `json:"token_endpoint_auth_method"`
	}
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		t.Fatalf("register body %s: %v", res.Body, err)
	}
	wantMethod := method
	if wantMethod == "" {
		wantMethod = "none"
	}
	if doc.ID == "" || doc.Method != wantMethod {
		t.Fatalf("registration %#v", doc)
	}
	if wantMethod == "none" && doc.Secret != "" {
		t.Fatalf("public client received a secret")
	}
	if wantMethod != "none" && doc.Secret == "" {
		t.Fatal("confidential client has no secret")
	}
	return oauthClient{ID: doc.ID, Secret: doc.Secret, Method: doc.Method}
}

func authorizeCode(t *testing.T, clientID, challenge string) string {
	t.Helper()
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", oauthRedirect)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", "st8")
	page := oauthDo(t, "GET", "/authorize?"+q.Encode(), nil, nil)
	if page.Status != 200 {
		t.Fatalf("authorize %d %s", page.Status, page.Body)
	}
	req := regexp.MustCompile(`name="req" value="([^"]+)"`).FindSubmatch(page.Body)
	if req == nil {
		t.Fatalf("consent page has no req field: %s", trimBody(page.Body))
	}
	form := url.Values{}
	form.Set("req", html.UnescapeString(string(req[1])))
	form.Set("password", os.Getenv("CONTRACT_MCP_TOKEN"))
	form.Set("action", "approve")
	approved := oauthDo(t, "POST", "/authorize", map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}, []byte(form.Encode()))
	if approved.Status != 200 {
		t.Fatalf("approve %d %s", approved.Status, trimBody(approved.Body))
	}
	return codeFrom(t, approved.Body)
}

func redeemCode(t *testing.T, clientID, code, redirect, verifier string, postSecret, basicSecret *string) *httpx.Exchange {
	t.Helper()
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("redirect_uri", redirect)
	form.Set("code_verifier", verifier)
	return tokenCall(t, form, postSecret, basicSecret)
}

func refreshToken(t *testing.T, clientID, refresh string, postSecret, basicSecret *string) *httpx.Exchange {
	t.Helper()
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refresh)
	form.Set("client_id", clientID)
	return tokenCall(t, form, postSecret, basicSecret)
}

func tokenCall(t *testing.T, form url.Values, postSecret, basicSecret *string) *httpx.Exchange {
	t.Helper()
	if postSecret != nil {
		form.Set("client_secret", *postSecret)
	}
	headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	if basicSecret != nil {
		headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(form.Get("client_id")+":"+*basicSecret))
	}
	return oauthDo(t, "POST", "/token", headers, []byte(form.Encode()))
}

func assertOAuthError(t *testing.T, res *httpx.Exchange, status int, code, description, wwwAuthenticate string) {
	t.Helper()
	if res.Status != status {
		t.Fatalf("status %d body %s", res.Status, res.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(res.Body, &body); err != nil {
		t.Fatalf("body %s: %v", res.Body, err)
	}
	if body["error"] != code {
		t.Fatalf("error %#v body %s", body["error"], res.Body)
	}
	if body["error_description"] != description {
		t.Fatalf("error_description %#v", body["error_description"])
	}
	if got := res.Header.Get("WWW-Authenticate"); got != wwwAuthenticate {
		t.Fatalf("WWW-Authenticate\n got %q\nwant %q", got, wwwAuthenticate)
	}
}

func assertAccessToken(t *testing.T, res *httpx.Exchange) (access, refresh string) {
	t.Helper()
	if res.Status != 200 {
		t.Fatalf("status %d body %s", res.Status, res.Body)
	}
	var body struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
		Type    string `json:"token_type"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(res.Body, &body); err != nil {
		t.Fatalf("body %s: %v", res.Body, err)
	}
	if body.Error != "" || body.Type != "Bearer" || body.Access == "" || body.Refresh == "" {
		t.Fatalf("token response %#v", body)
	}
	return body.Access, body.Refresh
}

func oauthDo(t *testing.T, method, path string, headers map[string]string, body []byte) *httpx.Exchange {
	t.Helper()
	res, err := httpx.New().Do(testCtx(t), method, originOf(t, mcpURL(t))+path, headers, body)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func pkce(t *testing.T) (verifier, challenge string) {
	t.Helper()
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge
}

func codeFrom(t *testing.T, body []byte) string {
	t.Helper()
	m := regexp.MustCompile(`url=([^"]+)"`).FindSubmatch(body)
	if m == nil {
		t.Fatalf("no redirect url in %s", trimBody(body))
	}
	target, err := url.Parse(html.UnescapeString(string(m[1])))
	if err != nil {
		t.Fatal(err)
	}
	code := target.Query().Get("code")
	if code == "" {
		t.Fatalf("redirect has no code: %s", target)
	}
	if target.Query().Get("state") != "st8" {
		t.Fatalf("state %#v", target.Query().Get("state"))
	}
	return code
}

func strPtr(s string) *string { return &s }

func trimBody(b []byte) string {
	if len(b) > 400 {
		b = b[:400]
	}
	return string(b)
}
