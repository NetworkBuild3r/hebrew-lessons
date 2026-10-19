package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

func TestMCPRejectsRemovedAllowlistEmail(t *testing.T) {
	clk := &manualClock{t: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	store := openTestStore(t, clk)
	user, err := store.CreateUser(context.Background(), "learner@example.com", "sub-1", RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	access := NewAccess([]string{"learner@example.com"}, nil)
	svc, err := New(Config{
		PublicOrigin: "https://hebrew.example",
		Clock:        clk,
		Store:        store,
		Access:       access,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := svc.createSession(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}

	const oauthToken = "oauth-access"
	const staticToken = "static-secret"
	h := mcp.Handler(mcp.Options{
		AuthToken: staticToken,
		Sessions:  svc,
		SessionFromToken: func(_ context.Context, token string, _ *http.Request) (string, error) {
			if token != oauthToken {
				return "", mcp.ErrUnauthorized
			}
			return raw, nil
		},
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	allowed := postMCPInitialize(t, srv.URL, oauthToken)
	if allowed.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(allowed.Body)
		allowed.Body.Close()
		t.Fatalf("allowed email status %d body %s", allowed.StatusCode, body)
	}
	allowed.Body.Close()

	access.Set(nil, nil)
	removed := postMCPInitialize(t, srv.URL, oauthToken)
	defer removed.Body.Close()
	if removed.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(removed.Body)
		t.Fatalf("removed email status %d body %s", removed.StatusCode, body)
	}
	if !strings.Contains(removed.Header.Get("WWW-Authenticate"), "Bearer") {
		t.Fatalf("challenge %q", removed.Header.Get("WWW-Authenticate"))
	}
	if _, _, err := svc.Lookup(context.Background(), raw); err != ErrNoSession {
		t.Fatalf("session after removal: %v", err)
	}

	still := postMCPInitialize(t, srv.URL, staticToken)
	defer still.Body.Close()
	if still.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(still.Body)
		t.Fatalf("static token status %d body %s", still.StatusCode, body)
	}
}

const mcpInitializeBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`

func postMCPInitialize(t *testing.T, url, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(mcpInitializeBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
