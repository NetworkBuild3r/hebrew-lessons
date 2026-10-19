package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// OAuthClient is a dynamically registered MCP client.
// SecretHash is empty for a public client. The raw secret is never stored.
type OAuthClient struct {
	ID            string
	SecretHash    string
	Name          string
	RedirectURIs  []string
	AuthMethod    string
	GrantTypes    []string
	ResponseTypes []string
	Scope         string
	CreatedAt     time.Time
}

// OAuthRequest is an in-flight authorization, before a code is issued.
type OAuthRequest struct {
	ID            string
	ClientID      string
	RedirectURI   string
	State         string
	CodeChallenge string
	Scope         string
	Resource      string
	ExpiresAt     time.Time
}

// OAuthCode is an authorization code. Hash is the digest of the raw code.
// Email is the user's address, joined for the allowlist check.
type OAuthCode struct {
	Hash          string
	ClientID      string
	UserID        int64
	Email         string
	RedirectURI   string
	CodeChallenge string
	Scope         string
	Resource      string
	ExpiresAt     time.Time
	UsedAt        *time.Time
}

// OAuthToken is an access or refresh token. Hash is the digest of the raw token.
type OAuthToken struct {
	Hash      string
	Kind      string
	ClientID  string
	UserID    int64
	Scope     string
	Resource  string
	FamilyID  string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// InsertOAuthClient writes one registered client.
func (s *Store) InsertOAuthClient(ctx context.Context, c OAuthClient) error {
	if err := s.ready(); err != nil {
		return err
	}
	uris, err := json.Marshal(c.RedirectURIs)
	if err != nil {
		return err
	}
	grants, err := json.Marshal(c.GrantTypes)
	if err != nil {
		return err
	}
	responses, err := json.Marshal(c.ResponseTypes)
	if err != nil {
		return err
	}
	var secret any
	if c.SecretHash != "" {
		secret = c.SecretHash
	}
	_, err = s.exec(ctx, `
		INSERT INTO oauth_clients (
			client_id, client_secret_hash, client_name, redirect_uris,
			token_endpoint_auth_method, grant_types, response_types, scope, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, secret, c.Name, string(uris), c.AuthMethod, string(grants), string(responses), c.Scope, FormatTime(c.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("db: insert oauth client: %w", err)
	}
	return nil
}

// OAuthClient loads one client. ErrNotFound means it is not registered.
func (s *Store) OAuthClient(ctx context.Context, id string) (OAuthClient, error) {
	if err := s.ready(); err != nil {
		return OAuthClient{}, err
	}
	row := s.queryRow(ctx, `
		SELECT client_id, client_secret_hash, client_name, redirect_uris,
		       token_endpoint_auth_method, grant_types, response_types, scope, created_at
		FROM oauth_clients WHERE client_id = ?`, id)
	var c OAuthClient
	var secret sql.NullString
	var uris, grants, responses, created string
	err := row.Scan(&c.ID, &secret, &c.Name, &uris, &c.AuthMethod, &grants, &responses, &c.Scope, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return OAuthClient{}, ErrNotFound
	}
	if err != nil {
		return OAuthClient{}, fmt.Errorf("db: scan oauth client: %w", err)
	}
	if secret.Valid {
		c.SecretHash = secret.String
	}
	if err := json.Unmarshal([]byte(uris), &c.RedirectURIs); err != nil {
		return OAuthClient{}, fmt.Errorf("db: oauth client redirects: %w", err)
	}
	if err := json.Unmarshal([]byte(grants), &c.GrantTypes); err != nil {
		return OAuthClient{}, fmt.Errorf("db: oauth client grants: %w", err)
	}
	if err := json.Unmarshal([]byte(responses), &c.ResponseTypes); err != nil {
		return OAuthClient{}, fmt.Errorf("db: oauth client responses: %w", err)
	}
	c.CreatedAt, err = ParseTime(created)
	if err != nil {
		return OAuthClient{}, fmt.Errorf("db: oauth client created_at: %w", err)
	}
	return c, nil
}

// InsertOAuthRequest writes one in-flight authorization.
func (s *Store) InsertOAuthRequest(ctx context.Context, req OAuthRequest) error {
	if err := s.ready(); err != nil {
		return err
	}
	_, err := s.exec(ctx, `
		INSERT INTO oauth_requests (
			id, client_id, redirect_uri, state, code_challenge, code_challenge_method,
			scope, resource, expires_at
		) VALUES (?, ?, ?, ?, ?, 'S256', ?, ?, ?)`,
		req.ID, req.ClientID, nullString(req.RedirectURI), nullString(req.State), req.CodeChallenge,
		nullString(req.Scope), nullString(req.Resource), FormatTime(req.ExpiresAt),
	)
	if err != nil {
		return fmt.Errorf("db: insert oauth request: %w", err)
	}
	return nil
}

// OAuthRequest loads one authorization request. ErrNotFound means it is gone.
func (s *Store) OAuthRequest(ctx context.Context, id string) (OAuthRequest, error) {
	if err := s.ready(); err != nil {
		return OAuthRequest{}, err
	}
	row := s.queryRow(ctx, `
		SELECT id, client_id, redirect_uri, state, code_challenge, scope, resource, expires_at
		FROM oauth_requests WHERE id = ?`, id)
	return scanOAuthRequest(row)
}

// ConsumeOAuthRequest deletes the request and returns it.
// ErrNotFound means it was already consumed or never existed.
// An expired row is deleted and reported as ErrNotFound.
func (s *Store) ConsumeOAuthRequest(ctx context.Context, id string, now time.Time) (OAuthRequest, error) {
	if err := s.ready(); err != nil {
		return OAuthRequest{}, err
	}
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return OAuthRequest{}, fmt.Errorf("db: begin oauth request: %w", err)
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, `
		SELECT id, client_id, redirect_uri, state, code_challenge, scope, resource, expires_at
		FROM oauth_requests WHERE id = ?`, id)
	req, err := scanOAuthRequest(row)
	if err != nil {
		return OAuthRequest{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM oauth_requests WHERE id = ?`, id); err != nil {
		return OAuthRequest{}, fmt.Errorf("db: delete oauth request: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return OAuthRequest{}, fmt.Errorf("db: commit oauth request: %w", err)
	}
	if !req.ExpiresAt.After(now) {
		return OAuthRequest{}, ErrNotFound
	}
	return req, nil
}

// InsertOAuthCode writes one authorization code. Hash is the digest.
func (s *Store) InsertOAuthCode(ctx context.Context, c OAuthCode) error {
	if err := s.ready(); err != nil {
		return err
	}
	_, err := s.exec(ctx, `
		INSERT INTO oauth_codes (
			code_hash, client_id, user_id, redirect_uri, code_challenge, scope, resource, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Hash, c.ClientID, c.UserID, nullString(c.RedirectURI), c.CodeChallenge,
		nullString(c.Scope), nullString(c.Resource), FormatTime(c.ExpiresAt),
	)
	if err != nil {
		return fmt.Errorf("db: insert oauth code: %w", err)
	}
	return nil
}

// OAuthCode loads one code and the user's email. ErrNotFound means no row.
func (s *Store) OAuthCode(ctx context.Context, hash string) (OAuthCode, error) {
	if err := s.ready(); err != nil {
		return OAuthCode{}, err
	}
	row := s.queryRow(ctx, `
		SELECT c.code_hash, c.client_id, c.user_id, u.email, c.redirect_uri, c.code_challenge,
		       c.scope, c.resource, c.expires_at, c.used_at
		FROM oauth_codes AS c
		JOIN users AS u ON u.id = c.user_id
		WHERE c.code_hash = ?`, hash)
	return scanOAuthCode(row)
}

// RedeemOAuthCode marks the code used and inserts the access and refresh tokens.
// ok is false when the code is missing, expired, or already used. The code is
// left unused when ok is false.
func (s *Store) RedeemOAuthCode(ctx context.Context, hash string, now time.Time, access, refresh OAuthToken) (bool, error) {
	if err := s.ready(); err != nil {
		return false, err
	}
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("db: begin oauth redeem: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		UPDATE oauth_codes SET used_at = ?
		WHERE code_hash = ? AND used_at IS NULL AND expires_at > ?`,
		FormatTime(now), hash, FormatTime(now),
	)
	if err != nil {
		return false, fmt.Errorf("db: use oauth code: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("db: use oauth code: %w", err)
	}
	if n == 0 {
		return false, tx.Commit()
	}
	if err := insertOAuthTokenTx(ctx, tx, access); err != nil {
		return false, err
	}
	if err := insertOAuthTokenTx(ctx, tx, refresh); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("db: commit oauth redeem: %w", err)
	}
	return true, nil
}

// OAuthToken loads one token by its digest. ErrNotFound means no row.
func (s *Store) OAuthToken(ctx context.Context, hash string) (OAuthToken, error) {
	if err := s.ready(); err != nil {
		return OAuthToken{}, err
	}
	row := s.queryRow(ctx, `
		SELECT token_hash, kind, client_id, user_id, scope, resource, family_id, expires_at, revoked_at, created_at
		FROM oauth_tokens WHERE token_hash = ?`, hash)
	return scanOAuthToken(row)
}

// RotateOAuthRefresh revokes the presented refresh token and inserts the
// replacement access and refresh tokens in the same family.
// ok is false when the refresh row is missing, expired, or already revoked.
func (s *Store) RotateOAuthRefresh(ctx context.Context, hash string, now time.Time, access, refresh OAuthToken) (bool, error) {
	if err := s.ready(); err != nil {
		return false, err
	}
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("db: begin oauth refresh: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		UPDATE oauth_tokens SET revoked_at = ?
		WHERE token_hash = ? AND kind = 'refresh' AND revoked_at IS NULL AND expires_at > ?`,
		FormatTime(now), hash, FormatTime(now),
	)
	if err != nil {
		return false, fmt.Errorf("db: revoke oauth refresh: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("db: revoke oauth refresh: %w", err)
	}
	if n == 0 {
		return false, tx.Commit()
	}
	if err := insertOAuthTokenTx(ctx, tx, access); err != nil {
		return false, err
	}
	if err := insertOAuthTokenTx(ctx, tx, refresh); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("db: commit oauth refresh: %w", err)
	}
	return true, nil
}

// RevokeOAuthFamily revokes every live token in the family.
// A reuse of an already-revoked refresh token calls this.
func (s *Store) RevokeOAuthFamily(ctx context.Context, familyID string, now time.Time) error {
	if err := s.ready(); err != nil {
		return err
	}
	_, err := s.exec(ctx, `
		UPDATE oauth_tokens SET revoked_at = ?
		WHERE family_id = ? AND revoked_at IS NULL`, FormatTime(now), familyID)
	if err != nil {
		return fmt.Errorf("db: revoke oauth family: %w", err)
	}
	return nil
}

func insertOAuthTokenTx(ctx context.Context, tx *sql.Tx, t OAuthToken) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO oauth_tokens (
			token_hash, kind, client_id, user_id, scope, resource, family_id, expires_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Hash, t.Kind, t.ClientID, t.UserID, nullString(t.Scope), nullString(t.Resource),
		t.FamilyID, FormatTime(t.ExpiresAt), FormatTime(t.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("db: insert oauth token: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanOAuthRequest(row scanner) (OAuthRequest, error) {
	var req OAuthRequest
	var redirect, state, scope, resource, exp sql.NullString
	err := row.Scan(&req.ID, &req.ClientID, &redirect, &state, &req.CodeChallenge, &scope, &resource, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return OAuthRequest{}, ErrNotFound
	}
	if err != nil {
		return OAuthRequest{}, fmt.Errorf("db: scan oauth request: %w", err)
	}
	req.RedirectURI = redirect.String
	req.State = state.String
	req.Scope = scope.String
	req.Resource = resource.String
	if !exp.Valid {
		return OAuthRequest{}, fmt.Errorf("db: oauth request expires_at is null")
	}
	req.ExpiresAt, err = ParseTime(exp.String)
	if err != nil {
		return OAuthRequest{}, fmt.Errorf("db: oauth request expires_at: %w", err)
	}
	return req, nil
}

func scanOAuthCode(row scanner) (OAuthCode, error) {
	var c OAuthCode
	var redirect, scope, resource, exp sql.NullString
	var used sql.NullString
	err := row.Scan(&c.Hash, &c.ClientID, &c.UserID, &c.Email, &redirect, &c.CodeChallenge, &scope, &resource, &exp, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return OAuthCode{}, ErrNotFound
	}
	if err != nil {
		return OAuthCode{}, fmt.Errorf("db: scan oauth code: %w", err)
	}
	c.RedirectURI = redirect.String
	c.Scope = scope.String
	c.Resource = resource.String
	if !exp.Valid {
		return OAuthCode{}, fmt.Errorf("db: oauth code expires_at is null")
	}
	c.ExpiresAt, err = ParseTime(exp.String)
	if err != nil {
		return OAuthCode{}, fmt.Errorf("db: oauth code expires_at: %w", err)
	}
	if used.Valid {
		t, err := ParseTime(used.String)
		if err != nil {
			return OAuthCode{}, fmt.Errorf("db: oauth code used_at: %w", err)
		}
		c.UsedAt = &t
	}
	return c, nil
}

func scanOAuthToken(row scanner) (OAuthToken, error) {
	var t OAuthToken
	var scope, resource, exp, revoked, created sql.NullString
	err := row.Scan(&t.Hash, &t.Kind, &t.ClientID, &t.UserID, &scope, &resource, &t.FamilyID, &exp, &revoked, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return OAuthToken{}, ErrNotFound
	}
	if err != nil {
		return OAuthToken{}, fmt.Errorf("db: scan oauth token: %w", err)
	}
	t.Scope = scope.String
	t.Resource = resource.String
	if !exp.Valid || !created.Valid {
		return OAuthToken{}, fmt.Errorf("db: oauth token time is null")
	}
	t.ExpiresAt, err = ParseTime(exp.String)
	if err != nil {
		return OAuthToken{}, fmt.Errorf("db: oauth token expires_at: %w", err)
	}
	t.CreatedAt, err = ParseTime(created.String)
	if err != nil {
		return OAuthToken{}, fmt.Errorf("db: oauth token created_at: %w", err)
	}
	if revoked.Valid {
		rt, err := ParseTime(revoked.String)
		if err != nil {
			return OAuthToken{}, fmt.Errorf("db: oauth token revoked_at: %w", err)
		}
		t.RevokedAt = &rt
	}
	return t, nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
