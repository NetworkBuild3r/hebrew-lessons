package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeSessions struct {
	user Principal
	err  error
	raw  string
}

func (f *fakeSessions) ResolveSession(_ context.Context, raw string) (Principal, error) {
	f.raw = raw
	if f.err != nil {
		return Principal{}, f.err
	}
	return f.user, nil
}

func TestOAuthSessionsUsesResolver(t *testing.T) {
	users := &fakeSessions{user: Principal{Actor: "7", UserID: 7, Source: "oauth"}}
	v := OAuthSessions{
		Tokens: func(_ context.Context, token string, _ *http.Request) (string, error) {
			if token != "access" {
				return "", ErrUnauthorized
			}
			return "session-raw", nil
		},
		Users: users,
	}
	p, err := v.Validate(context.Background(), "access", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.UserID != 7 || p.Actor != "7" || users.raw != "session-raw" {
		t.Fatalf("principal %+v raw %q", p, users.raw)
	}
	if _, err := v.Validate(context.Background(), "other", nil); err != ErrUnauthorized {
		t.Fatalf("unknown token: %v", err)
	}

	users.err = ErrUnauthorized
	if _, err := v.Validate(context.Background(), "access", nil); err != ErrUnauthorized {
		t.Fatalf("removed user: %v", err)
	}

	bare := OAuthSessions{}
	if _, err := bare.Validate(context.Background(), "access", nil); err != ErrUnauthorized {
		t.Fatalf("unwired oauth: %v", err)
	}
}

func TestRemovedSessionIsHTTP401(t *testing.T) {
	h := Handler(Options{
		AuthToken: testToken,
		Sessions:  &fakeSessions{err: ErrUnauthorized},
		SessionFromToken: func(context.Context, string, *http.Request) (string, error) {
			return "session-raw", nil
		},
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	res := postInitialize(t, srv.URL, "oauth-access")
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
}
