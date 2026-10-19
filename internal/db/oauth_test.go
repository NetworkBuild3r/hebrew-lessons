package db

import (
	"context"
	"errors"
	"io/fs"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

func TestOAuthIssuedBackfillSurvivesSweep(t *testing.T) {
	ctx := context.Background()
	conn := openRaw(t)
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") || strings.HasPrefix(entry.Name(), "007") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if len(names) != 6 || names[len(names)-1] != "006_learner_tz.sql" {
		t.Fatalf("migrations before 007 = %v", names)
	}
	var versions []int
	for _, name := range names {
		version, err := versionOf(name)
		if err != nil {
			t.Fatal(err)
		}
		versions = append(versions, version)
		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		for _, stmt := range statements(string(body)) {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
	}
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for _, version := range versions {
		if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, version, FormatTime(created)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO oauth_clients (
			client_id, client_name, redirect_uris, token_endpoint_auth_method,
			grant_types, response_types, created_at
		) VALUES ('pre-007', 'old', '["https://app.example/cb"]', 'none', '["authorization_code"]', '["code"]', ?)`,
		FormatTime(created)); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(ctx, conn, fixedClock{created}); err != nil {
		t.Fatal(err)
	}
	var issued int
	if err := conn.QueryRowContext(ctx, `SELECT issued FROM oauth_clients WHERE client_id = 'pre-007'`).Scan(&issued); err != nil {
		t.Fatal(err)
	}
	if issued != 1 {
		t.Fatalf("issued = %d, want 1", issued)
	}
	var codes, tokens int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM oauth_codes WHERE client_id = 'pre-007'`).Scan(&codes); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM oauth_tokens WHERE client_id = 'pre-007'`).Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	if codes != 0 || tokens != 0 {
		t.Fatalf("pre-007 client has codes=%d tokens=%d", codes, tokens)
	}

	sweepAt := created.Add(25 * time.Hour)
	s := &Store{conn: conn, clock: fixedClock{sweepAt}}
	if err := s.SweepOAuth(ctx, sweepAt, sweepAt.Add(-400*24*time.Hour), sweepAt.Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, `SELECT issued FROM oauth_clients WHERE client_id = 'pre-007'`).Scan(&issued); err != nil {
		t.Fatalf("sweep removed the pre-007 client: %v", err)
	}
	if issued != 1 {
		t.Fatalf("issued after sweep = %d", issued)
	}
}

func TestOAuthCodeAndTokenInsertsSetIssued(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user, err := s.CreateUser(ctx, "learner@example.com", "sub", "learner")
	if err != nil {
		t.Fatal(err)
	}
	insert := func(id string) {
		t.Helper()
		if err := s.InsertOAuthClient(ctx, OAuthClient{
			ID: id, Name: id, RedirectURIs: []string{"https://app.example/cb"}, AuthMethod: "none",
			GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"code"},
			CreatedAt: when.Add(-25 * time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
		if got := oauthIssued(t, s, id); got != 0 {
			t.Fatalf("%s issued = %d, want 0", id, got)
		}
	}
	insert("code-client")
	if err := s.InsertOAuthCode(ctx, OAuthCode{
		Hash: "code-hash", ClientID: "code-client", UserID: user.ID,
		CodeChallenge: strings.Repeat("a", 43), ExpiresAt: when.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if got := oauthIssued(t, s, "code-client"); got != 1 {
		t.Fatalf("InsertOAuthCode issued = %d, want 1", got)
	}

	insert("token-client")
	if _, err := s.ExecContext(ctx, `
		INSERT INTO oauth_codes (code_hash, client_id, user_id, code_challenge, expires_at)
		VALUES ('redeem-me', 'token-client', ?, ?, ?)`,
		user.ID, strings.Repeat("b", 43), FormatTime(when.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	if got := oauthIssued(t, s, "token-client"); got != 0 {
		t.Fatalf("raw code issued = %d, want 0", got)
	}
	ok, err := s.RedeemOAuthCode(ctx, "redeem-me", when, OAuthToken{
		Hash: "access-hash", Kind: "access", ClientID: "token-client", UserID: user.ID,
		FamilyID: "fam", ExpiresAt: when.Add(time.Hour), CreatedAt: when,
	}, OAuthToken{
		Hash: "refresh-hash", Kind: "refresh", ClientID: "token-client", UserID: user.ID,
		FamilyID: "fam", ExpiresAt: when.Add(24 * time.Hour), CreatedAt: when,
	})
	if err != nil || !ok {
		t.Fatalf("redeem ok=%v err=%v", ok, err)
	}
	if got := oauthIssued(t, s, "token-client"); got != 1 {
		t.Fatalf("RedeemOAuthCode issued = %d, want 1", got)
	}
}

func TestIdleOAuthClientWithLiveRequestSurvives(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	if err := s.InsertOAuthClient(ctx, OAuthClient{
		ID: "waiting", Name: "waiting", RedirectURIs: []string{"https://app.example/cb"}, AuthMethod: "none",
		GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"code"},
		CreatedAt: when.Add(-25 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertOAuthRequest(ctx, OAuthRequest{
		ID: "live-req", ClientID: "waiting", RedirectURI: "https://app.example/cb",
		CodeChallenge: strings.Repeat("c", 43), ExpiresAt: when.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SweepOAuth(ctx, when, when.Add(-400*24*time.Hour), when.Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OAuthClient(ctx, "waiting"); err != nil {
		t.Fatalf("live request did not keep the client: %v", err)
	}
	if _, err := s.OAuthRequest(ctx, "live-req"); err != nil {
		t.Fatalf("live request was deleted: %v", err)
	}
	if got := oauthIssued(t, s, "waiting"); got != 0 {
		t.Fatalf("issued = %d, want 0", got)
	}
}

func TestIssueOAuthCodeConsumesRequestAndMarksIssued(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user, err := s.CreateUser(ctx, "learner@example.com", "sub", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.InsertOAuthClient(ctx, OAuthClient{
		ID: "consent", Name: "consent", RedirectURIs: []string{"https://app.example/cb"}, AuthMethod: "none",
		GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"code"},
		CreatedAt: when.Add(-25 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertOAuthRequest(ctx, OAuthRequest{
		ID: "req-1", ClientID: "consent", RedirectURI: "https://app.example/cb", State: "st",
		CodeChallenge: strings.Repeat("d", 43), Scope: "mcp", ExpiresAt: when.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.IssueOAuthCode(ctx, "req-1", when, OAuthCode{
		Hash: "issued-code", UserID: user.ID, CodeChallenge: strings.Repeat("d", 43),
		ExpiresAt: when.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientID != "consent" || got.State != "st" {
		t.Fatalf("request %+v", got)
	}
	if _, err := s.OAuthRequest(ctx, "req-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("request still present: %v", err)
	}
	if _, err := s.OAuthCode(ctx, "issued-code"); err != nil {
		t.Fatal(err)
	}
	if issued := oauthIssued(t, s, "consent"); issued != 1 {
		t.Fatalf("issued = %d, want 1", issued)
	}
	if err := s.SweepOAuth(ctx, when, when.Add(-400*24*time.Hour), when.Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OAuthClient(ctx, "consent"); err != nil {
		t.Fatalf("issued client was swept: %v", err)
	}
}

func oauthIssued(t *testing.T, s *Store, id string) int {
	t.Helper()
	var n int
	if err := s.conn.QueryRowContext(context.Background(), `SELECT issued FROM oauth_clients WHERE client_id = ?`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
