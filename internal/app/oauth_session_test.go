package app

import (
	"bytes"
	"context"
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
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

const (
	oauthRedirect     = "http://127.0.0.1:9/callback"
	oauthLearnerEmail = "learner@example.com"
	oauthAdminEmail   = "admin@example.com"
	oauthHello        = "שָׁלוֹם"
	oauthStaticToken  = "static-secret"
	oauthPublicHost   = "hebrew-mcp.ibhacked.us"
	oauthPublicOrigin = "https://hebrew-mcp.ibhacked.us"
)

// oauthRoleEnv signs two allowlisted accounts in through the real OAuth
// authorization-code flow. Role comes only from sessionUsers, which
// copies users.role after Lookup. The admin email is in ADMIN_EMAILS;
// the other allowlisted email is not, so it is a learner.
type oauthRoleEnv struct {
	base      string
	learner   string
	admin     string
	learnerID int64
	mem       *mcp.Memory
	access    *auth.Access
}

func startOAuthRoles(t *testing.T) oauthRoleEnv {
	t.Helper()
	start := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := &stepClock{t: start}
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	access := auth.NewAccess([]string{oauthLearnerEmail}, []string{oauthAdminEmail})

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	fake := &googleFake{
		key:      key,
		kid:      "test-key",
		clientID: "test-client",
		secret:   "test-secret",
		issuer:   "https://issuer.test",
		now:      start,
		verified: true,
	}
	fakeSrv := httptest.NewServer(fake)
	t.Cleanup(fakeSrv.Close)
	svc, err := auth.New(auth.Config{
		PublicOrigin:       "https://hebrew.example",
		StateSecret:        "oauth-test-secret",
		GoogleClientID:     fake.clientID,
		GoogleClientSecret: fake.secret,
		GoogleAuthURL:      fakeSrv.URL + "/authorize",
		GoogleTokenURL:     fakeSrv.URL + "/token",
		GoogleJWKSURL:      fakeSrv.URL + "/jwks",
		GoogleIssuer:       fake.issuer,
		Access:             access,
		Clock:              clk,
		Store:              store,
	})
	if err != nil {
		t.Fatal(err)
	}

	mem := mcp.NewMemory()
	mem.AddLexeme(mcp.Lexeme{TextNFC: oauthHello, Bare: "שלום", Gloss: "hello; peace", POS: "interjection"})
	users := sessionUsers{auth: svc}
	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	base := "http://" + srv.Listener.Addr().String()
	if !svc.RegisterOAuth(mux, base) {
		t.Fatal("oauth did not mount")
	}
	mux.Handle("/", mcp.Handler(mcp.Options{
		AuthToken:        oauthStaticToken,
		Sessions:         users,
		SessionFromToken: users.SessionFromToken,
		Course:           mem,
		Lexicon:          mem,
		Lookup:           mem,
		Units:            mem,
	}))
	srv.Start()
	t.Cleanup(srv.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	learner := signInMCP(t, client, fake, base, "", oauthLearnerEmail, "learner-sub")
	user, err := store.UserByEmail(context.Background(), oauthLearnerEmail)
	if err != nil {
		t.Fatal(err)
	}
	mem.SetPlacement(user.ID, mcp.Placement{Finished: true, Sittings: 1, StopReason: "exhausted"}, []string{oauthHello}, nil)
	admin := signInMCP(t, client, fake, base, "", oauthAdminEmail, "admin-sub")
	return oauthRoleEnv{
		base: base, learner: learner, admin: admin,
		learnerID: user.ID, mem: mem, access: access,
	}
}

func TestOAuthAdminEmailCanPutShared(t *testing.T) {
	env := startOAuthRoles(t)
	saved := postTool(t, env.base, env.admin, "put_unit", map[string]any{
		"scope": mcp.ScopeShared, "position": 0, "doc": map[string]any{
			"title": "Hello", "situation": "Meeting a friend", "concept": "Greeting",
			"newWords": []string{oauthHello},
			"lessons": []map[string]any{{
				"kind": "lesson",
				"activities": []map[string]any{
					{"type": "listen_pick", "text": oauthHello},
					{"type": "recall", "text": oauthHello},
				},
			}},
		},
	})
	if saved.StatusCode != http.StatusOK || strings.Contains(saved.Body, `"isError":true`) {
		t.Fatalf("admin put %d %s", saved.StatusCode, saved.Body)
	}
	units, err := env.mem.ListUnits(context.Background())
	if err != nil || len(units) != 1 || units[0].Scope != mcp.ScopeShared || units[0].Status != "draft" {
		t.Fatalf("units %+v err %v", units, err)
	}
}

func TestOAuthLearnerPutUnitForbidden(t *testing.T) {
	env := startOAuthRoles(t)
	for _, scope := range []string{mcp.ScopeShared, "learner:" + strconv.FormatInt(env.learnerID, 10)} {
		put := postTool(t, env.base, env.learner, "put_unit", map[string]any{
			"scope": scope, "position": 0, "doc": map[string]any{"title": "Hello"},
		})
		if put.StatusCode != http.StatusOK || !strings.Contains(put.Body, `"isError":true`) || !strings.Contains(put.Body, "forbidden: admin only") {
			t.Fatalf("learner put %s %d %s", scope, put.StatusCode, put.Body)
		}
	}
	units, err := env.mem.ListUnits(context.Background())
	if err != nil || len(units) != 0 {
		t.Fatalf("units %+v err %v", units, err)
	}
}

func TestOAuthLearnerGetContextIsSelf(t *testing.T) {
	env := startOAuthRoles(t)
	other := env.learnerID + 1
	cross := postTool(t, env.base, env.learner, "get_context", map[string]any{"position": 0, "learnerId": other})
	if cross.StatusCode != http.StatusOK || !strings.Contains(cross.Body, `"isError":true`) || !strings.Contains(cross.Body, "learnerId must be your own id.") {
		t.Fatalf("other %d %s", cross.StatusCode, cross.Body)
	}
	self := postTool(t, env.base, env.learner, "get_context", map[string]any{"position": 0, "learnerId": env.learnerID})
	if self.StatusCode != http.StatusOK || strings.Contains(self.Body, `"isError":true`) || !strings.Contains(self.Body, oauthHello) {
		t.Fatalf("self %d %s", self.StatusCode, self.Body)
	}
	omitted := postTool(t, env.base, env.learner, "get_context", map[string]any{"position": 0})
	if omitted.StatusCode != http.StatusOK || strings.Contains(omitted.Body, `"isError":true`) || !strings.Contains(omitted.Body, oauthHello) {
		t.Fatalf("default learnerId %d %s", omitted.StatusCode, omitted.Body)
	}
}

func TestOAuthRemovedAllowlistEmailUnauthorized(t *testing.T) {
	env := startOAuthRoles(t)
	env.access.Set(nil, []string{oauthAdminEmail})
	removed := postTool(t, env.base, env.learner, "get_context", map[string]any{"position": 0})
	if removed.StatusCode != http.StatusUnauthorized || !strings.Contains(removed.Header.Get("WWW-Authenticate"), "Bearer") {
		t.Fatalf("removed %d %s %q", removed.StatusCode, removed.Body, removed.Header.Get("WWW-Authenticate"))
	}
	still := postInit(t, env.base, oauthStaticToken)
	if still.StatusCode != http.StatusOK {
		t.Fatalf("static %d %s", still.StatusCode, still.Body)
	}
	adminStill := postTool(t, env.base, env.admin, "get_context", map[string]any{"position": 0})
	if adminStill.StatusCode != http.StatusOK || strings.Contains(adminStill.Body, `"isError":true`) {
		t.Fatalf("admin after removal %d %s", adminStill.StatusCode, adminStill.Body)
	}
}

func signInMCP(t *testing.T, client *http.Client, fake *googleFake, base, publicOrigin, email, sub string) string {
	t.Helper()
	if publicOrigin == "" {
		publicOrigin = base
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client.Jar = jar
	regBody := `{"redirect_uris":["` + oauthRedirect + `"],"client_name":"MCP client"}`
	reg := doOAuth(t, client, http.MethodPost, base+"/register", regBody, "application/json", "")
	if reg.StatusCode != http.StatusCreated {
		t.Fatalf("register %d %s", reg.StatusCode, reg.Body)
	}
	var clientDoc struct {
		ID string `json:"client_id"`
	}
	if err := json.Unmarshal([]byte(reg.Body), &clientDoc); err != nil || clientDoc.ID == "" {
		t.Fatalf("client %s", reg.Body)
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientDoc.ID},
		"redirect_uri":          {oauthRedirect},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {"client-state"},
	}
	start := doOAuth(t, client, http.MethodGet, base+"/authorize?"+q.Encode(), "", "", "")
	if start.StatusCode != http.StatusFound {
		t.Fatalf("authorize %d %s", start.StatusCode, start.Body)
	}
	gu, err := url.Parse(start.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	fake.challenge = gu.Query().Get("code_challenge")
	fake.email = email
	fake.sub = sub
	fake.verified = true
	fake.mint = mintSpec{nonce: gu.Query().Get("nonce")}
	cb := doOAuth(t, client, http.MethodGet, base+"/oauth/google/callback?code=authcode&state="+url.QueryEscape(gu.Query().Get("state")), "", "", "")
	if cb.StatusCode != http.StatusOK {
		t.Fatalf("callback %d %s", cb.StatusCode, cb.Body)
	}
	reqID := hidden(t, cb.Body, "req")
	csrf := hidden(t, cb.Body, "_csrf")
	form := url.Values{"req": {reqID}, "_csrf": {csrf}, "action": {"approve"}}.Encode()
	approved := doOAuth(t, client, http.MethodPost, base+"/authorize", form, "application/x-www-form-urlencoded", publicOrigin)
	if approved.StatusCode != http.StatusSeeOther {
		t.Fatalf("approve %d %s", approved.StatusCode, approved.Body)
	}
	loc, err := url.Parse(approved.Header.Get("Location"))
	if err != nil || loc.Query().Get("code") == "" {
		t.Fatalf("approval %s", approved.Header.Get("Location"))
	}
	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {loc.Query().Get("code")},
		"code_verifier": {verifier},
		"redirect_uri":  {oauthRedirect},
		"client_id":     {clientDoc.ID},
	}.Encode()
	tokRes := doOAuth(t, client, http.MethodPost, base+"/token", tokenForm, "application/x-www-form-urlencoded", "")
	if tokRes.StatusCode != http.StatusOK {
		t.Fatalf("token %d %s", tokRes.StatusCode, tokRes.Body)
	}
	var tok struct {
		Access string `json:"access_token"`
	}
	if err := json.Unmarshal([]byte(tokRes.Body), &tok); err != nil || tok.Access == "" {
		t.Fatalf("access %s", tokRes.Body)
	}
	return tok.Access
}

type oauthResult struct {
	StatusCode int
	Header     http.Header
	Body       string
}

func doOAuth(t *testing.T, client *http.Client, method, rawURL, body, contentType, origin string) oauthResult {
	t.Helper()
	req, err := http.NewRequest(method, rawURL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
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
	return oauthResult{StatusCode: res.StatusCode, Header: res.Header.Clone(), Body: string(b)}
}

func hidden(t *testing.T, body, name string) string {
	t.Helper()
	needle := `name="` + name + `" value="`
	i := strings.Index(body, needle)
	if i < 0 {
		t.Fatalf("missing %s in %s", name, body)
	}
	rest := body[i+len(needle):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		t.Fatal(name)
	}
	return rest[:j]
}

func postTool(t *testing.T, base, token, name string, args any) oauthResult {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatal(err)
	}
	return postMCPBody(t, base, token, raw)
}

func postInit(t *testing.T, base, token string) oauthResult {
	t.Helper()
	return postMCPBody(t, base, token, []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`))
}

func postMCPBody(t *testing.T, base, token string, body []byte) oauthResult {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("X-Forwarded-Host", oauthPublicHost)
	req.Header.Set("X-Forwarded-Proto", "https")
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
