package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

func TestMCPRejectsRemovedAllowlistEmail(t *testing.T) {
	clk := fixedClock{t: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	store := openAuthStore(t, clk)
	user, err := store.CreateUser(context.Background(), "learner@example.com", "sub-1", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	access := auth.NewAccess([]string{"learner@example.com"}, nil)
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://hebrew.example",
		Clock:        clk,
		Store:        store,
		Access:       access,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := insertSession(t, store, user.ID, clk.Now().Add(time.Hour))

	const oauthToken = "oauth-access"
	const staticToken = "static-secret"
	h := mcp.Handler(mcp.Options{
		AuthToken: staticToken,
		Sessions:  sessionUsers{auth: svc},
		SessionFromToken: func(_ context.Context, token string, _ *http.Request) (string, error) {
			if token != oauthToken {
				return "", mcp.ErrUnauthorized
			}
			return raw, nil
		},
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	allowed := postMCP(t, srv.URL, oauthToken)
	allowedBody := readAll(t, allowed)
	if allowed.StatusCode != http.StatusOK {
		t.Fatalf("allowed email status %d body %s", allowed.StatusCode, allowedBody)
	}

	access.Set(nil, nil)
	removed := postMCP(t, srv.URL, oauthToken)
	removedBody := readAll(t, removed)
	if removed.StatusCode != http.StatusUnauthorized {
		t.Fatalf("removed email status %d body %s", removed.StatusCode, removedBody)
	}
	if !strings.Contains(removed.Header.Get("WWW-Authenticate"), "Bearer") {
		t.Fatalf("challenge %q", removed.Header.Get("WWW-Authenticate"))
	}
	if strings.Contains(removedBody, "store is closed") {
		t.Fatalf("leaked body %s", removedBody)
	}
	if _, _, err := svc.Lookup(context.Background(), raw); err != auth.ErrNoSession {
		t.Fatalf("session after removal: %v", err)
	}

	still := postMCP(t, srv.URL, staticToken)
	stillBody := readAll(t, still)
	if still.StatusCode != http.StatusOK {
		t.Fatalf("static token status %d body %s", still.StatusCode, stillBody)
	}
}

func openAuthStore(t *testing.T, clk fixedClock) *db.Store {
	t.Helper()
	store, err := db.Open(context.Background(), t.TempDir()+"/hebrew.sqlite", clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func insertSession(t *testing.T, store *db.Store, userID int64, exp time.Time) string {
	t.Helper()
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf)
	if err := store.InsertSession(context.Background(), hex.EncodeToString(sum[:]), userID, "csrf-token", exp); err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

const mcpInitializeBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`

func postMCP(t *testing.T, url, token string) *http.Response {
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

func readAll(t *testing.T, res *http.Response) string {
	t.Helper()
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
