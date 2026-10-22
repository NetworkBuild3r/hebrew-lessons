// Package testharness boots the app in-process for tests.
//
// Each Env has its own data directory, clock, speech synthesizer, and
// identity provider, so tests can call t.Parallel. Start serves the learner
// and MCP handlers on two httptest servers and registers Cleanup with t.
package testharness

import (
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/app"
)

// MCPToken is the static bearer Start installs on the authoring server.
// An empty token leaves the MCP tools closed, so the harness sets one.
const MCPToken = "testharness-mcp"

// Account is a user a seed helper would insert.
// ID stays zero until the app exposes a way to insert one.
type Account struct {
	ID    int64
	Email string
	Role  string
}

// Env is one running app and the fakes it was given.
type Env struct {
	WebURL  string
	MCPURL  string
	DataDir string
	App     *app.App
	TTS     *FakeTTS
	Clock   *FixedClock
	IDP     *IDP

	once sync.Once
	web  *httptest.Server
	mcp  *httptest.Server
}

// Start opens the app on a temp data directory and two localhost servers.
// The Google auth, token, userinfo, JWKS, and issuer fields all point at IDP.
// TTS is the fake, so the ElevenLabs client is not constructed.
// Cleanup runs when the test finishes. It is also safe to call directly.
func Start(t *testing.T) *Env {
	t.Helper()
	dir := t.TempDir()
	clk := NewFixedClock(Epoch())
	speech := NewFakeTTS()
	idp, err := newIDP(clk)
	if err != nil {
		t.Fatalf("idp: %v", err)
	}
	application, err := app.NewApp(app.Config{
		DataDir:           dir,
		GoogleAuthURL:     idp.AuthURL,
		GoogleTokenURL:    idp.TokenURL,
		GoogleUserinfoURL: idp.UserinfoURL,
		GoogleJWKSURL:     idp.JWKSURL,
		GoogleIssuer:      idp.Issuer,
		Clock:             clk,
		TTS:               speech,
		MCPAuthToken:      MCPToken,
	})
	if err != nil {
		idp.Close()
		t.Fatalf("app: %v", err)
	}
	webSrv := httptest.NewServer(application.Web())
	mcpSrv := httptest.NewServer(application.MCP())
	env := &Env{
		WebURL:  webSrv.URL,
		MCPURL:  mcpSrv.URL,
		DataDir: dir,
		App:     application,
		TTS:     speech,
		Clock:   clk,
		IDP:     idp,
		web:     webSrv,
		mcp:     mcpSrv,
	}
	t.Cleanup(env.Cleanup)
	return env
}

// Cleanup stops both servers, the identity provider, and the database.
func (e *Env) Cleanup() {
	if e == nil {
		return
	}
	e.once.Do(func() {
		if e.web != nil {
			e.web.Close()
		}
		if e.mcp != nil {
			e.mcp.Close()
		}
		if e.IDP != nil {
			e.IDP.Close()
		}
		if e.App != nil {
			_ = e.App.Close()
		}
	})
}

// SeedLearner would insert a learner.
//
// TODO: Backend Dev's auth PR adds App.Store and Store.CreateUser.
// Until those exist, there is no app API to insert a user.
func (e *Env) SeedLearner(t *testing.T, email string) Account {
	t.Helper()
	t.Skipf("TODO: app does not expose a store; cannot seed learner %q", email)
	return Account{Email: email, Role: "learner"}
}

// SeedAdmin would insert an admin.
//
// TODO: Backend Dev's auth PR adds App.Store and Store.CreateUser.
// Until those exist, there is no app API to insert a user.
func (e *Env) SeedAdmin(t *testing.T, email string) Account {
	t.Helper()
	t.Skipf("TODO: app does not expose a store; cannot seed admin %q", email)
	return Account{Email: email, Role: "admin"}
}
