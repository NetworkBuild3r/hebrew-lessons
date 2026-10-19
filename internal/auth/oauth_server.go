package auth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

const (
	oauthAccessTTL  = time.Hour
	oauthRefreshTTL = 30 * 24 * time.Hour
	oauthCodeTTL    = 10 * time.Minute
	oauthRequestTTL = 10 * time.Minute
	// OAuthClientTTL matches Node CLIENT_TTL: 400 days from registration.
	// A client that has been issued a code or token keeps this lifetime.
	OAuthClientTTL = 400 * 24 * time.Hour
	// OAuthIdleClientTTL is how long a registration may sit with no code
	// and no token before sweep deletes it. About one day.
	OAuthIdleClientTTL = 24 * time.Hour
	oauthRateWindow    = 10 * time.Minute
	oauthScope         = "mcp"
	oauthMaxBody       = 64 * 1024

	cookieMCPOAuth     = "hebrew_mcp_google"
	cookieMCPOAuthHost = "__Host-hebrew_mcp_google"

	// oauthSessionLabel is mixed into the HMAC that turns an opaque token
	// into the raw session id Lookup expects. The raw id is not stored.
	oauthSessionLabel = "hebrew-lessons mcp-oauth-session v1\x00"
)

var (
	errBodyTooLarge = errors.New("auth: body too large")

	pkceChallenge = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	pkceVerifier  = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

	// Default redirects are the old app's DEFAULT_REDIRECTS.
	// Anything else must be http loopback or an exact MCP_OAUTH_REDIRECT_URIS entry.
	defaultOAuthRedirects = []string{
		"cursor://anysphere.cursor-mcp/oauth/callback",
		"https://www.cursor.com/agents/mcp/oauth/callback",
		"https://cursor.com/agents/mcp/oauth/callback",
		"http://localhost:8787/callback",
	}

	// Caps bound table growth. Node had no numeric cap; these are the bounds
	// added for the authorization server. Tests lower them. Idle
	// registrations are deleted after OAuthIdleClientTTL, so a burst of
	// unused clients cannot hold every slot for 400 days.
	oauthClientCap = 1000
	oauthRateLimit = 30

	googleErrorCodes = map[string]struct{}{
		"invalid_request":           {},
		"unauthorized_client":       {},
		"access_denied":             {},
		"unsupported_response_type": {},
		"invalid_scope":             {},
		"server_error":              {},
		"temporarily_unavailable":   {},
	}
)

// oauthHandler is the MCP OAuth 2.1 authorization server.
// Sign-in is Google only. The password form is not served.
type oauthHandler struct {
	svc  *Service
	base string
}

// RegisterOAuth mounts the authorization server on mux.
// publicURL is MCP_PUBLIC_URL. When it is set, it is the issuer.
// When it is empty, the issuer is the trusted proxy's public host.
// The redirect_uri sent to Google is {issuer}/oauth/google/callback.
// /auth/google/callback is the same handler and is never that redirect_uri.
// These routes are not behind the MCP bearer middleware.
func (s *Service) RegisterOAuth(mux *http.ServeMux, publicURL string) bool {
	if s == nil || mux == nil {
		return false
	}
	base, _, _ := oauthBase(publicURL)
	h := &oauthHandler{svc: s, base: base}
	mux.HandleFunc("/.well-known/oauth-authorization-server", h.metadata)
	mux.HandleFunc("/.well-known/openid-configuration", h.metadata)
	mux.HandleFunc("/.well-known/oauth-protected-resource", h.protectedResource)
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", h.protectedResource)
	mux.HandleFunc("/.well-known/", h.unknownWellKnown)
	mux.HandleFunc("/register", h.register)
	mux.HandleFunc("/authorize", h.authorize)
	mux.HandleFunc("/token", h.token)
	mux.HandleFunc("/oauth/google/callback", h.googleCallback)
	mux.HandleFunc("/auth/google/callback", h.googleCallback)
	return true
}

// SessionFromAccessToken maps a live access token to the raw session id
// Lookup accepts. It does not resolve the user and does not import the
// MCP package. An unknown, expired, revoked, or non-access token is
// ErrNoSession. The app session adapter turns that into an MCP 401 so
// the static bearer can still be tried. Any other error is a store
// failure and is returned as-is.
func (s *Service) SessionFromAccessToken(ctx context.Context, token string, _ *http.Request) (string, error) {
	if s == nil || token == "" {
		return "", ErrNoSession
	}
	row, err := s.store.OAuthToken(ctx, tokenDigest(token))
	if errors.Is(err, db.ErrNotFound) {
		return "", ErrNoSession
	}
	if err != nil {
		return "", err
	}
	if row.Kind != "access" || row.RevokedAt != nil || !row.ExpiresAt.After(s.now()) {
		return "", ErrNoSession
	}
	raw, err := s.sessionFromOpaque(token)
	if err != nil {
		return "", ErrNoSession
	}
	return raw, nil
}

func (s *Service) sessionFromOpaque(token string) (string, error) {
	if len(s.stateKey) == 0 || token == "" {
		return "", errors.New("auth: oauth session key")
	}
	mac := hmac.New(sha256.New, s.stateKey)
	_, _ = mac.Write([]byte(oauthSessionLabel))
	_, _ = mac.Write([]byte(token))
	sum := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(sum[:sessionIDBytes]), nil
}

func (s *Service) bindTokenSession(ctx context.Context, userID int64, rawToken string, exp time.Time) error {
	raw, err := s.sessionFromOpaque(rawToken)
	if err != nil {
		return err
	}
	hash, err := sessionHash(raw)
	if err != nil {
		return err
	}
	csrf, err := randomToken(sessionIDBytes)
	if err != nil {
		return err
	}
	return s.store.InsertSession(ctx, hash, userID, csrf, exp)
}

// PublicBase is the origin clients use for this request.
// MCP_PUBLIC_URL wins when it is set and its host is not a pod address.
// The trusted proxy's public host is the fallback when that URL is unset.
// A non-loopback IP and a cluster-internal name are refused, so the pod
// address is never advertised.
func (s *Service) PublicBase(r *http.Request, configured string) (base, origin string, ok bool) {
	base, origin, ok = oauthBase(configured)
	if ok && !podHost(hostOf(base)) {
		return base, origin, true
	}
	if s != nil && r != nil {
		if proxiesFrom(r.Context()) == nil {
			r = r.WithContext(context.WithValue(r.Context(), proxyKey{}, s.proxies))
		}
		if host, scheme, good := forwardedPublicHost(r); good {
			return joinOrigin(scheme, host)
		}
	}
	return "", "", false
}

func (h *oauthHandler) requestBase(r *http.Request) (base, origin string, ok bool) {
	if h == nil || h.svc == nil {
		return "", "", false
	}
	return h.svc.PublicBase(r, h.base)
}

func (h *oauthHandler) noPublicHost(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = io.WriteString(w, "public host is not configured")
}

// forwardedPublicHost reads the public host from a trusted proxy.
// X-Forwarded-Host wins. The Host header is used when the proxy rewrote
// it to a DNS name. An IP is not a public host.
func forwardedPublicHost(r *http.Request) (host, scheme string, ok bool) {
	if r == nil {
		return "", "", false
	}
	peer := peerIP(r)
	if peer == nil || !ipTrusted(proxiesFrom(r.Context()), peer) {
		return "", "", false
	}
	scheme = ForwardedProto(r)
	if name := rightmostForwardedHost(r.Header.Values("X-Forwarded-Host")); dnsPublicHost(name) {
		return trimDefaultPort(scheme, name), scheme, true
	}
	if dnsPublicHost(r.Host) {
		return trimDefaultPort(scheme, r.Host), scheme, true
	}
	return "", "", false
}

func rightmostForwardedHost(values []string) string {
	var last string
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				last = part
			}
		}
	}
	return last
}

func dnsPublicHost(hostport string) bool {
	host := hostOf(hostport)
	if host == "" || strings.ContainsAny(host, " /\\@") {
		return false
	}
	if net.ParseIP(host) != nil {
		return false
	}
	if podHost(host) {
		return false
	}
	return host == "localhost" || strings.Contains(host, ".")
}

func podHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsLoopback()
	}
	return strings.HasSuffix(host, ".cluster.local") || strings.HasSuffix(host, ".svc")
}

func hostOf(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return ""
		}
		return strings.ToLower(u.Hostname())
	}
	host := raw
	if h, _, err := net.SplitHostPort(raw); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	return strings.ToLower(host)
}

func trimDefaultPort(scheme, hostport string) string {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		return host
	}
	return hostport
}

func joinOrigin(scheme, hostport string) (base, origin string, ok bool) {
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if scheme != "http" && scheme != "https" {
		return "", "", false
	}
	hostport = strings.TrimSpace(hostport)
	if hostport == "" || strings.Contains(hostport, "/") {
		return "", "", false
	}
	u, err := url.Parse(scheme + "://" + hostport)
	if err != nil || u.Host == "" || u.User != nil || u.Hostname() == "" {
		return "", "", false
	}
	if podHost(u.Hostname()) {
		return "", "", false
	}
	base = scheme + "://" + strings.ToLower(u.Host)
	return base, base, true
}

func oauthBase(raw string) (base, origin string, ok bool) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return "", "", false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", "", false
	}
	return raw, scheme + "://" + strings.ToLower(u.Host), true
}

func (h *oauthHandler) withReq(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), proxyKey{}, h.svc.proxies))
}

func (h *oauthHandler) metadata(w http.ResponseWriter, r *http.Request) {
	r = h.withReq(r)
	if h.preflight(w, r) {
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.notFound(w)
		return
	}
	base, _, ok := h.requestBase(r)
	if !ok {
		h.noPublicHost(w)
		return
	}
	h.writeJSON(w, http.StatusOK, asMetadata{
		Issuer:                            base,
		AuthorizationEndpoint:             base + "/authorize",
		TokenEndpoint:                     base + "/token",
		RegistrationEndpoint:              base + "/register",
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none", "client_secret_post", "client_secret_basic"},
		ScopesSupported:                   []string{oauthScope},
	})
}

func (h *oauthHandler) protectedResource(w http.ResponseWriter, r *http.Request) {
	r = h.withReq(r)
	if h.preflight(w, r) {
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.notFound(w)
		return
	}
	base, _, ok := h.requestBase(r)
	if !ok {
		h.noPublicHost(w)
		return
	}
	h.writeJSON(w, http.StatusOK, protectedMetadata{
		Resource:               base + "/mcp",
		AuthorizationServers:   []string{base},
		ScopesSupported:        []string{oauthScope},
		BearerMethodsSupported: []string{"header"},
	})
}

type asMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
}

type protectedMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	ScopesSupported        []string `json:"scopes_supported"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
}

type registrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientSecretExpiresAt   *int64   `json:"client_secret_expires_at,omitempty"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func (h *oauthHandler) register(w http.ResponseWriter, r *http.Request) {
	r = h.withReq(r)
	if h.preflight(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		h.notFound(w)
		return
	}
	h.sweep(r)
	if h.limited(w, r) {
		return
	}
	body, err := readBody(r, oauthMaxBody)
	if errors.Is(err, errBodyTooLarge) {
		h.oauthError(w, http.StatusRequestEntityTooLarge, "invalid_client_metadata", "Body must be JSON.")
		return
	}
	if err != nil {
		h.oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "Body must be JSON.")
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		body = []byte("{}")
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		h.oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "Body must be JSON.")
		return
	}
	uris, ok := redirectList(doc["redirect_uris"])
	if !ok {
		h.oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect_uris must be 1 to 10 strings.")
		return
	}
	for _, uri := range uris {
		if !h.redirectAllowed(uri) {
			slog.Info("mcp oauth register rejected redirect_uri")
			h.oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect_uri is not allowed by this server: "+clip(uri, 120))
			return
		}
	}
	name := "MCP client"
	if raw, ok := doc["client_name"]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" {
			name = clip(s, 80)
		}
	}
	method := "none"
	if raw, ok := doc["token_endpoint_auth_method"]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil && (s == "none" || s == "client_secret_post" || s == "client_secret_basic") {
			method = s
		}
	}
	id, err := newClientID()
	if err != nil {
		h.fail(w, err)
		return
	}
	var secret string
	var secretHash string
	if method != "none" {
		secret, err = randomHex(24)
		if err != nil {
			h.fail(w, err)
			return
		}
		secretHash = tokenDigest(secret)
	}
	now := h.svc.now()
	n, err := h.svc.store.CountOAuthClients(r.Context(), now.Add(-OAuthClientTTL))
	if err != nil {
		h.fail(w, err)
		return
	}
	if n >= oauthClientCap {
		h.oauthError(w, http.StatusTooManyRequests, "invalid_client_metadata", "Too many registered clients.")
		return
	}
	if err := h.svc.store.InsertOAuthClient(r.Context(), db.OAuthClient{
		ID:            id,
		SecretHash:    secretHash,
		Name:          name,
		RedirectURIs:  uris,
		AuthMethod:    method,
		GrantTypes:    []string{"authorization_code", "refresh_token"},
		ResponseTypes: []string{"code"},
		CreatedAt:     now,
	}); err != nil {
		h.fail(w, err)
		return
	}
	slog.Info("mcp oauth register", "client_name", oneLine(name), "client_id", id, "method", method)
	reply := registrationResponse{
		ClientID:                id,
		ClientIDIssuedAt:        now.Unix(),
		ClientName:              name,
		RedirectURIs:            uris,
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: method,
	}
	if secret != "" {
		reply.ClientSecret = secret
		never := int64(0)
		reply.ClientSecretExpiresAt = &never
	}
	h.writeJSON(w, http.StatusCreated, reply)
}

func (h *oauthHandler) authorize(w http.ResponseWriter, r *http.Request) {
	r = h.withReq(r)
	if h.preflight(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.sweep(r)
		if h.limited(w, r) {
			return
		}
		h.authorizeGet(w, r)
	case http.MethodPost:
		h.sweep(r)
		if h.limited(w, r) {
			return
		}
		h.authorizePost(w, r)
	default:
		// HEAD writes no row. Only GET starts an authorization.
		h.notFound(w)
	}
}

func (h *oauthHandler) authorizeGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	slog.Info("mcp oauth authorize",
		"redirect_uri_present", q.Get("redirect_uri") != "",
		"client_id", oneLine(q.Get("client_id")),
		"client_ip", ClientIP(r),
		"scheme", ForwardedProto(r),
	)
	base, _, _ := h.requestBase(r)
	client, fatal, errCode, desc := h.resolveAuthorize(r.Context(), q, base)
	if fatal != "" {
		slog.Info("mcp oauth authorize fatal", "reason", fatal)
		h.page(w, http.StatusBadRequest, "Cannot sign in", "<h1>Cannot sign in</h1><p class=\"err\">"+esc(fatal)+"</p>")
		return
	}
	if errCode != "" {
		slog.Info("mcp oauth authorize", "error", errCode)
		h.redirectError(w, r, q.Get("redirect_uri"), q.Get("state"), errCode, desc)
		return
	}
	if h.svc.googleOn() && h.svc.access.allowlistEmpty() {
		h.page(w, http.StatusServiceUnavailable, "Sign-in is not configured",
			"<h1>Sign-in is not configured</h1><p class=\"err\">No allowed email is set.</p>")
		return
	}
	reqID, err := randomToken(24)
	if err != nil {
		h.fail(w, err)
		return
	}
	exp := h.svc.now().Add(oauthRequestTTL)
	if err := h.svc.store.InsertOAuthRequest(r.Context(), db.OAuthRequest{
		ID:            reqID,
		ClientID:      q.Get("client_id"),
		RedirectURI:   q.Get("redirect_uri"),
		State:         q.Get("state"),
		CodeChallenge: q.Get("code_challenge"),
		Scope:         oauthScope,
		Resource:      q.Get("resource"),
		ExpiresAt:     exp,
	}); err != nil {
		h.fail(w, err)
		return
	}
	if user, sess, ok := h.browserSession(r); ok {
		h.consentPage(w, client, reqID, q.Get("redirect_uri"), sess.CSRF, http.StatusOK, "")
		_ = user
		return
	}
	h.startGoogle(w, r, reqID)
}

func (h *oauthHandler) authorizePost(w http.ResponseWriter, r *http.Request) {
	kind, user, vals, err := h.readConsent(r)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errBodyTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		h.page(w, status, "Bad request", "<p class=\"err\">Bad request.</p>")
		return
	}
	if kind == consentNoSession {
		h.page(w, http.StatusBadRequest, "Expired", "<h1>Sign-in expired</h1><p>Start again from the app.</p>")
		return
	}
	if kind == consentCSRF {
		h.page(w, http.StatusForbidden, "Forbidden", "<h1>Forbidden</h1><p class=\"err\">The sign-in form could not be confirmed.</p>")
		return
	}
	pending, err := h.svc.store.ConsumeOAuthRequest(r.Context(), vals.Get("req"), h.svc.now())
	if errors.Is(err, db.ErrNotFound) {
		h.page(w, http.StatusBadRequest, "Expired", "<h1>Sign-in expired</h1><p>Start again from the app.</p>")
		return
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	q := url.Values{
		"client_id":             {pending.ClientID},
		"redirect_uri":          {pending.RedirectURI},
		"code_challenge":        {pending.CodeChallenge},
		"code_challenge_method": {"S256"},
		"state":                 {pending.State},
		"response_type":         {"code"},
		"resource":              {pending.Resource},
	}
	base, _, _ := h.requestBase(r)
	if _, fatal, errCode, _ := h.resolveAuthorize(r.Context(), q, base); fatal != "" || errCode != "" {
		h.page(w, http.StatusBadRequest, "Cannot sign in", "<h1>Cannot sign in</h1><p class=\"err\">Invalid request.</p>")
		return
	}
	if vals.Get("action") == "deny" {
		h.redirectError(w, r, pending.RedirectURI, pending.State, "access_denied", "")
		return
	}
	if vals.Get("action") != "approve" {
		h.page(w, http.StatusBadRequest, "Bad request", "<p class=\"err\">Bad request.</p>")
		return
	}
	if _, ok := h.svc.roleFor(user.Email); !ok {
		h.redirectError(w, r, pending.RedirectURI, pending.State, "access_denied", "")
		return
	}
	raw, err := randomToken(sessionIDBytes)
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.svc.store.InsertOAuthCode(r.Context(), db.OAuthCode{
		Hash:          tokenDigest(raw),
		ClientID:      pending.ClientID,
		UserID:        user.ID,
		RedirectURI:   pending.RedirectURI,
		CodeChallenge: pending.CodeChallenge,
		Scope:         oauthScope,
		Resource:      pending.Resource,
		ExpiresAt:     h.svc.now().Add(oauthCodeTTL),
	}); err != nil {
		h.fail(w, err)
		return
	}
	slog.Info("mcp oauth authorize approved", "redirect_uri_present", pending.RedirectURI != "", "client_id", oneLine(pending.ClientID))
	h.redirectQuery(w, r, pending.RedirectURI, url.Values{"code": {raw}, "state": {pending.State}}, false)
}

func (h *oauthHandler) startGoogle(w http.ResponseWriter, r *http.Request, reqID string) {
	if h.svc.access.allowlistEmpty() {
		h.page(w, http.StatusServiceUnavailable, "Sign-in is not configured",
			"<h1>Sign-in is not configured</h1><p class=\"err\">No allowed email is set.</p>")
		return
	}
	if !h.svc.googleOn() {
		h.page(w, http.StatusServiceUnavailable, "Sign-in is not configured",
			"<h1>Sign-in is not configured</h1><p class=\"err\">Google sign-in is not configured.</p>")
		return
	}
	verifier, challenge, err := newPKCE()
	if err != nil {
		h.fail(w, err)
		return
	}
	nonce, err := randomToken(sessionIDBytes)
	if err != nil {
		h.fail(w, err)
		return
	}
	sealed, err := h.svc.sealState(oidcState{
		State:    reqID,
		Nonce:    nonce,
		Verifier: verifier,
		Exp:      db.FormatTime(h.svc.now().Add(oidcStateTTL)),
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	base, _, ok := h.requestBase(r)
	if !ok {
		h.noPublicHost(w)
		return
	}
	h.svc.setCookie(w, h.googleCookieName(), sealed, h.svc.now().Add(oidcStateTTL), int(oidcStateTTL/time.Second))
	loc, err := h.svc.mcpGoogleURL(base+"/oauth/google/callback", reqID, nonce, challenge)
	if err != nil {
		h.page(w, http.StatusInternalServerError, "Sign-in is not configured",
			"<h1>Sign-in is not configured</h1><p class=\"err\">The Google sign-in address is invalid.</p>")
		return
	}
	h.cors(w.Header())
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, loc, http.StatusFound)
}

func (h *oauthHandler) googleCallback(w http.ResponseWriter, r *http.Request) {
	r = h.withReq(r)
	if h.preflight(w, r) {
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.notFound(w)
		return
	}
	if !h.svc.googleOn() {
		h.page(w, http.StatusNotFound, "Not found", "<p>Not found.</p>")
		return
	}
	sealed := readCookie(r, h.googleCookieName())
	h.svc.clearCookie(w, cookieMCPOAuth)
	h.svc.clearCookie(w, cookieMCPOAuthHost)
	st, err := h.svc.openState(sealed)
	q := r.URL.Query()
	if err != nil || !tokensEqual(q.Get("state"), st.State) || st.State == "" {
		slog.Info("mcp google callback", "result", "expired")
		h.page(w, http.StatusBadRequest, "Expired", "<h1>Sign-in expired</h1><p>Start again from the app.</p>")
		return
	}
	pending, err := h.svc.store.OAuthRequest(r.Context(), st.State)
	if errors.Is(err, db.ErrNotFound) || (err == nil && !pending.ExpiresAt.After(h.svc.now())) {
		slog.Info("mcp google callback", "result", "expired")
		h.page(w, http.StatusBadRequest, "Expired", "<h1>Sign-in expired</h1><p>Start again from the app.</p>")
		return
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	aq := url.Values{
		"client_id":             {pending.ClientID},
		"redirect_uri":          {pending.RedirectURI},
		"code_challenge":        {pending.CodeChallenge},
		"code_challenge_method": {"S256"},
		"response_type":         {"code"},
		"resource":              {pending.Resource},
	}
	base, _, baseOK := h.requestBase(r)
	client, fatal, errCode, _ := h.resolveAuthorize(r.Context(), aq, base)
	if fatal != "" || errCode != "" {
		slog.Info("mcp google callback", "result", "invalid")
		_, _ = h.svc.store.ConsumeOAuthRequest(r.Context(), pending.ID, h.svc.now())
		h.page(w, http.StatusBadRequest, "Cannot sign in", "<h1>Cannot sign in</h1><p class=\"err\">Invalid request.</p>")
		return
	}
	if gerr := q.Get("error"); gerr != "" {
		_, _ = h.svc.store.ConsumeOAuthRequest(r.Context(), pending.ID, h.svc.now())
		mapped := "server_error"
		if _, ok := googleErrorCodes[gerr]; ok {
			mapped = gerr
		}
		slog.Info("mcp google callback", "result", "rejected", "error", mapped)
		h.redirectError(w, r, pending.RedirectURI, pending.State, mapped, "")
		return
	}
	if q.Get("code") == "" {
		h.page(w, http.StatusBadRequest, "Cannot sign in", "<h1>Cannot sign in</h1><p class=\"err\">Google did not return a code.</p>")
		return
	}
	if !baseOK {
		h.noPublicHost(w)
		return
	}
	rawToken, err := h.svc.exchangeAt(r.Context(), q.Get("code"), st.Verifier, base+"/oauth/google/callback")
	if err != nil {
		slog.Info("mcp google callback", "result", "identity_failed")
		h.page(w, http.StatusBadGateway, "Google sign-in failed", "<h1>Google sign-in failed</h1><p class=\"err\">Try again from the app.</p>")
		return
	}
	id, err := h.svc.verify(r.Context(), rawToken, st.Nonce)
	if err != nil {
		slog.Info("mcp google callback", "result", "identity_failed")
		if errors.Is(err, errGoogle) {
			h.page(w, http.StatusBadGateway, "Google sign-in failed", "<h1>Google sign-in failed</h1><p class=\"err\">Try again from the app.</p>")
			return
		}
		_, _ = h.svc.store.ConsumeOAuthRequest(r.Context(), pending.ID, h.svc.now())
		if errors.Is(err, errUnverified) {
			h.redirectError(w, r, pending.RedirectURI, pending.State, "access_denied", "")
			return
		}
		h.page(w, http.StatusBadRequest, "Cannot sign in", "<h1>Cannot sign in</h1><p class=\"err\">Invalid request.</p>")
		return
	}
	role, ok := h.svc.roleFor(id.Email)
	if !ok {
		_, _ = h.svc.store.ConsumeOAuthRequest(r.Context(), pending.ID, h.svc.now())
		slog.Info("mcp google callback", "result", "rejected", "error", "access_denied")
		h.redirectError(w, r, pending.RedirectURI, pending.State, "access_denied", "")
		return
	}
	user, err := h.svc.store.UpsertGoogleUser(r.Context(), id.Email, id.Sub, role)
	if err != nil {
		h.fail(w, err)
		return
	}
	raw, sess, err := h.svc.createSession(r.Context(), user.ID)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.svc.setSessionCookie(w, raw, sess.ExpiresAt)
	slog.Info("mcp google callback", "result", "ok", "redirect_uri_present", pending.RedirectURI != "", "client_id", oneLine(pending.ClientID))
	h.consentPage(w, client, pending.ID, pending.RedirectURI, sess.CSRF, http.StatusOK, "")
}

func (h *oauthHandler) token(w http.ResponseWriter, r *http.Request) {
	r = h.withReq(r)
	if h.preflight(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		h.notFound(w)
		return
	}
	h.sweep(r)
	body, err := readBody(r, oauthMaxBody)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errBodyTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		slog.Info("mcp token", "result", "invalid_request")
		h.oauthError(w, status, "invalid_request", "Bad body.")
		return
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		slog.Info("mcp token", "result", "invalid_request")
		h.oauthError(w, http.StatusBadRequest, "invalid_request", "Bad body.")
		return
	}
	creds := credentialsFrom(form, r)
	switch form.Get("grant_type") {
	case "authorization_code":
		h.tokenCode(w, r, form, creds)
	case "refresh_token":
		h.tokenRefresh(w, r, form, creds)
	default:
		slog.Info("mcp token", "result", "unsupported_grant_type")
		h.oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "Use authorization_code or refresh_token.")
	}
}

func (h *oauthHandler) tokenCode(w http.ResponseWriter, r *http.Request, form url.Values, creds credentials) {
	_, redirectPresent := form["redirect_uri"]
	slog.Info("mcp token", "grant_type", "authorization_code", "redirect_uri_present", redirectPresent, "client_id", oneLine(creds.clientID))
	code, err := h.svc.store.OAuthCode(r.Context(), tokenDigest(form.Get("code")))
	if errors.Is(err, db.ErrNotFound) || (err == nil && !code.ExpiresAt.After(h.svc.now())) {
		slog.Info("mcp token", "grant_type", "authorization_code", "result", "invalid_grant")
		h.oauthError(w, http.StatusBadRequest, "invalid_grant", "Code is invalid or expired.")
		return
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	if code.UsedAt != nil {
		slog.Info("mcp token", "grant_type", "authorization_code", "result", "used_code")
		h.oauthError(w, http.StatusBadRequest, "invalid_grant", "Code was already used.")
		return
	}
	if creds.conflict {
		h.invalidClient(w, true)
		return
	}
	if code.ClientID != "" && creds.clientID != "" && creds.clientID != code.ClientID {
		slog.Info("mcp token", "grant_type", "authorization_code", "result", "client_or_redirect")
		h.oauthError(w, http.StatusBadRequest, "invalid_grant", "client_id or redirect_uri does not match the code.")
		return
	}
	clientID := creds.clientID
	if clientID == "" {
		clientID = code.ClientID
	}
	client, err := h.loadClient(r.Context(), clientID)
	if errors.Is(err, db.ErrNotFound) {
		client = db.OAuthClient{}
		err = nil
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	var clientPtr *db.OAuthClient
	if client.ID != "" {
		clientPtr = &client
	}
	if h.clientAuthFailed(w, clientPtr, creds) {
		return
	}
	clientOK := clientPtr != nil && slices.Contains(client.RedirectURIs, code.RedirectURI)
	if !clientOK || (redirectPresent && form.Get("redirect_uri") != code.RedirectURI) || (!redirectPresent && code.CodeChallenge == "") {
		slog.Info("mcp token", "grant_type", "authorization_code", "result", "client_or_redirect")
		h.oauthError(w, http.StatusBadRequest, "invalid_grant", "client_id or redirect_uri does not match the code.")
		return
	}
	if !pkceS256(form.Get("code_verifier"), code.CodeChallenge) {
		slog.Info("mcp token", "grant_type", "authorization_code", "result", "pkce")
		h.oauthError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed.")
		return
	}
	user, userErr := h.svc.store.UserByEmail(r.Context(), code.Email)
	allowed := userErr == nil && user.ID == code.UserID
	if allowed {
		_, allowed = h.svc.roleFor(user.Email)
	}
	if !allowed {
		if userErr != nil && !errors.Is(userErr, db.ErrNotFound) {
			h.fail(w, userErr)
			return
		}
		if err := h.svc.store.BurnOAuthCode(r.Context(), code.Hash, h.svc.now()); err != nil {
			h.fail(w, err)
			return
		}
		slog.Info("mcp token", "grant_type", "authorization_code", "result", "account")
		h.oauthError(w, http.StatusBadRequest, "invalid_grant", "That account can no longer sign in.")
		return
	}
	pair, err := h.newPair(r.Context(), clientID, code.UserID, code.Resource)
	if err != nil {
		h.fail(w, err)
		return
	}
	ok, err := h.svc.store.RedeemOAuthCode(r.Context(), code.Hash, h.svc.now(), pair.access, pair.refresh)
	if err != nil {
		h.fail(w, err)
		return
	}
	if !ok {
		slog.Info("mcp token", "grant_type", "authorization_code", "result", "used_code")
		h.oauthError(w, http.StatusBadRequest, "invalid_grant", "Code was already used.")
		return
	}
	slog.Info("mcp token", "grant_type", "authorization_code", "result", "ok", "user_id", code.UserID)
	h.writeTokens(w, pair)
}

func (h *oauthHandler) tokenRefresh(w http.ResponseWriter, r *http.Request, form url.Values, creds credentials) {
	presented := form.Get("refresh_token")
	row, err := h.svc.store.OAuthToken(r.Context(), tokenDigest(presented))
	if errors.Is(err, db.ErrNotFound) || (err == nil && (row.Kind != "refresh" || !row.ExpiresAt.After(h.svc.now()))) {
		slog.Info("mcp token", "grant_type", "refresh_token", "result", "invalid_grant")
		h.oauthError(w, http.StatusBadRequest, "invalid_grant", "Refresh token is invalid or expired.")
		return
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	if row.RevokedAt != nil {
		_ = h.svc.store.RevokeOAuthFamily(r.Context(), row.FamilyID, h.svc.now())
		slog.Info("mcp token", "grant_type", "refresh_token", "result", "reused")
		h.oauthError(w, http.StatusBadRequest, "invalid_grant", "Refresh token is invalid or expired.")
		return
	}
	if creds.conflict {
		h.invalidClient(w, true)
		return
	}
	if row.ClientID != "" && creds.clientID != "" && creds.clientID != row.ClientID {
		slog.Info("mcp token", "grant_type", "refresh_token", "result", "client")
		h.oauthError(w, http.StatusBadRequest, "invalid_grant", "client_id does not match the refresh token.")
		return
	}
	clientID := creds.clientID
	if clientID == "" {
		clientID = row.ClientID
	}
	if clientID == "" {
		slog.Info("mcp token", "grant_type", "refresh_token", "result", "client")
		h.oauthError(w, http.StatusBadRequest, "invalid_grant", "client_id does not match the refresh token.")
		return
	}
	client, err := h.loadClient(r.Context(), clientID)
	if errors.Is(err, db.ErrNotFound) {
		client = db.OAuthClient{}
		err = nil
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	var clientPtr *db.OAuthClient
	if client.ID != "" {
		clientPtr = &client
	}
	if row.ClientID == "" && clientPtr == nil {
		slog.Info("mcp token", "grant_type", "refresh_token", "result", "client")
		h.oauthError(w, http.StatusBadRequest, "invalid_grant", "client_id does not match the refresh token.")
		return
	}
	if h.clientAuthFailed(w, clientPtr, creds) {
		return
	}
	rawSession, err := h.svc.sessionFromOpaque(presented)
	if err != nil {
		h.fail(w, err)
		return
	}
	if _, _, err := h.svc.Lookup(r.Context(), rawSession); err != nil {
		if errors.Is(err, ErrNoSession) {
			slog.Info("mcp token", "grant_type", "refresh_token", "result", "account")
			h.oauthError(w, http.StatusBadRequest, "invalid_grant", "That account can no longer sign in.")
			return
		}
		h.fail(w, err)
		return
	}
	pair, err := h.newPair(r.Context(), clientID, row.UserID, row.Resource)
	if err != nil {
		h.fail(w, err)
		return
	}
	pair.access.FamilyID = row.FamilyID
	pair.refresh.FamilyID = row.FamilyID
	ok, err := h.svc.store.RotateOAuthRefresh(r.Context(), row.Hash, h.svc.now(), pair.access, pair.refresh)
	if err != nil {
		h.fail(w, err)
		return
	}
	if !ok {
		slog.Info("mcp token", "grant_type", "refresh_token", "result", "invalid_grant")
		h.oauthError(w, http.StatusBadRequest, "invalid_grant", "Refresh token is invalid or expired.")
		return
	}
	slog.Info("mcp token", "grant_type", "refresh_token", "result", "ok", "user_id", row.UserID)
	h.writeTokens(w, pair)
}

type issuedPair struct {
	accessRaw  string
	refreshRaw string
	access     db.OAuthToken
	refresh    db.OAuthToken
}

func (h *oauthHandler) newPair(ctx context.Context, clientID string, userID int64, resource string) (issuedPair, error) {
	family, err := randomToken(16)
	if err != nil {
		return issuedPair{}, err
	}
	accessRaw, err := randomToken(sessionIDBytes)
	if err != nil {
		return issuedPair{}, err
	}
	refreshRaw, err := randomToken(sessionIDBytes)
	if err != nil {
		return issuedPair{}, err
	}
	now := h.svc.now()
	accessExp := now.Add(oauthAccessTTL)
	refreshExp := now.Add(oauthRefreshTTL)
	if err := h.svc.bindTokenSession(ctx, userID, accessRaw, accessExp); err != nil {
		return issuedPair{}, err
	}
	if err := h.svc.bindTokenSession(ctx, userID, refreshRaw, refreshExp); err != nil {
		return issuedPair{}, err
	}
	return issuedPair{
		accessRaw:  accessRaw,
		refreshRaw: refreshRaw,
		access: db.OAuthToken{
			Hash: tokenDigest(accessRaw), Kind: "access", ClientID: clientID, UserID: userID,
			Scope: oauthScope, Resource: resource, FamilyID: family, ExpiresAt: accessExp, CreatedAt: now,
		},
		refresh: db.OAuthToken{
			Hash: tokenDigest(refreshRaw), Kind: "refresh", ClientID: clientID, UserID: userID,
			Scope: oauthScope, Resource: resource, FamilyID: family, ExpiresAt: refreshExp, CreatedAt: now,
		},
	}, nil
}

func (h *oauthHandler) writeTokens(w http.ResponseWriter, pair issuedPair) {
	h.writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  pair.accessRaw,
		TokenType:    "Bearer",
		ExpiresIn:    int(oauthAccessTTL / time.Second),
		RefreshToken: pair.refreshRaw,
		Scope:        oauthScope,
	})
}

func (h *oauthHandler) resolveAuthorize(ctx context.Context, q url.Values, base string) (client db.OAuthClient, fatal, errCode, desc string) {
	loaded, err := h.loadClient(ctx, q.Get("client_id"))
	if errors.Is(err, db.ErrNotFound) || q.Get("client_id") == "" {
		return db.OAuthClient{}, "Unknown or expired client. Register again.", "", ""
	}
	if err != nil {
		return db.OAuthClient{}, "Unknown or expired client. Register again.", "", ""
	}
	redirect := q.Get("redirect_uri")
	if redirect == "" || !slices.Contains(loaded.RedirectURIs, redirect) || !h.redirectAllowed(redirect) {
		return loaded, "redirect_uri does not match this client.", "", ""
	}
	if q.Get("response_type") != "code" {
		return loaded, "", "unsupported_response_type", ""
	}
	if q.Get("code_challenge_method") != "S256" || !pkceChallenge.MatchString(q.Get("code_challenge")) {
		return loaded, "", "invalid_request", "PKCE with S256 is required."
	}
	if resource := q.Get("resource"); resource != "" && (base == "" || !resourceMatches(base, resource)) {
		return loaded, "", "invalid_target", ""
	}
	return loaded, "", "", ""
}

func resourceMatches(base, resource string) bool {
	resource = strings.TrimRight(resource, "/")
	base = strings.TrimRight(base, "/")
	return resource == base+"/mcp" || resource == base
}

// redirectAllowed matches Node: reject a fragment first, then an exact
// member of the built-in list or MCP_OAUTH_REDIRECT_URIS, then http
// loopback on any port. A random https URI is refused. cursor:// is in
// the built-in list.
func (h *oauthHandler) redirectAllowed(uri string) bool {
	if strings.ContainsAny(uri, "\r\n") {
		return false
	}
	u, err := url.Parse(uri)
	if err != nil || u.Scheme == "" {
		return false
	}
	if u.Fragment != "" || strings.Contains(uri, "#") {
		return false
	}
	if slices.Contains(defaultOAuthRedirects, uri) || (h != nil && h.svc != nil && slices.Contains(h.svc.oauthRedirects, uri)) {
		return true
	}
	if u.Scheme != "http" {
		return false
	}
	host := u.Hostname()
	return strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1" || host == "[::1]"
}

func (h *oauthHandler) loadClient(ctx context.Context, id string) (db.OAuthClient, error) {
	c, err := h.svc.store.OAuthClient(ctx, id)
	if err != nil {
		return db.OAuthClient{}, err
	}
	if !c.CreatedAt.Add(OAuthClientTTL).After(h.svc.now()) {
		return db.OAuthClient{}, db.ErrNotFound
	}
	return c, nil
}

func (h *oauthHandler) sweep(r *http.Request) {
	if h == nil || h.svc == nil {
		return
	}
	now := h.svc.now()
	h.svc.PruneRateLimits(now)
	if err := h.svc.store.SweepOAuth(r.Context(), now, now.Add(-OAuthClientTTL), now.Add(-OAuthIdleClientTTL)); err != nil {
		slog.Error("mcp oauth sweep", "err", err)
	}
}

func (h *oauthHandler) limited(w http.ResponseWriter, r *http.Request) bool {
	if h.svc.rateAllow(ClientIP(r), h.svc.now()) {
		return false
	}
	if r.URL.Path == "/register" {
		h.oauthError(w, http.StatusTooManyRequests, "invalid_request", "Too many requests from this address.")
		return true
	}
	h.page(w, http.StatusTooManyRequests, "Too many requests", "<h1>Too many requests</h1><p class=\"err\">Wait and try again.</p>")
	return true
}

func (h *oauthHandler) unknownWellKnown(w http.ResponseWriter, r *http.Request) {
	r = h.withReq(r)
	if h.preflight(w, r) {
		return
	}
	h.notFound(w)
}

type rateWindow struct {
	start time.Time
	n     int
}

// PruneRateLimits drops per-IP windows whose interval has elapsed.
// The OAuth sweep calls this so the map cannot grow without bound.
func (s *Service) PruneRateLimits(now time.Time) {
	if s == nil {
		return
	}
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	for ip, w := range s.rate {
		if w.start.IsZero() || now.Sub(w.start) >= oauthRateWindow {
			delete(s.rate, ip)
		}
	}
}

func (s *Service) rateAllow(ip string, now time.Time) bool {
	if s == nil {
		return true
	}
	if ip == "" {
		ip = "unknown"
	}
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	if s.rate == nil {
		s.rate = map[string]rateWindow{}
	}
	w := s.rate[ip]
	if w.start.IsZero() || now.Sub(w.start) >= oauthRateWindow {
		w = rateWindow{start: now}
	}
	w.n++
	s.rate[ip] = w
	return w.n <= oauthRateLimit
}

func redirectList(raw json.RawMessage) ([]string, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var items []any
	if json.Unmarshal(raw, &items) != nil || len(items) < 1 || len(items) > 10 {
		return nil, false
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

type credentials struct {
	conflict     bool
	clientID     string
	basicAttempt bool
	basicSecret  string
	postSecret   string
}

func credentialsFrom(form url.Values, r *http.Request) credentials {
	bodyID := form.Get("client_id")
	headerID, headerSecret, basic := basicAuth(r)
	post := ""
	if vs, ok := form["client_secret"]; ok && len(vs) > 0 && vs[0] != "" {
		post = vs[0]
	}
	return credentials{
		conflict:     bodyID != "" && headerID != "" && bodyID != headerID,
		clientID:     firstNonEmpty(bodyID, headerID),
		basicAttempt: basic,
		basicSecret:  headerSecret,
		postSecret:   post,
	}
}

func (h *oauthHandler) clientAuthFailed(w http.ResponseWriter, client *db.OAuthClient, creds credentials) bool {
	method := ""
	if client != nil && (client.AuthMethod == "client_secret_basic" || client.AuthMethod == "client_secret_post") {
		method = client.AuthMethod
	}
	if method == "" {
		return false
	}
	if method == "client_secret_basic" {
		if creds.basicAttempt && creds.postSecret == "" && secretMatches(creds.basicSecret, client.SecretHash) {
			return false
		}
		h.invalidClient(w, true)
		return true
	}
	if !creds.basicAttempt && creds.postSecret != "" && secretMatches(creds.postSecret, client.SecretHash) {
		return false
	}
	h.invalidClient(w, creds.basicAttempt)
	return true
}

func (h *oauthHandler) invalidClient(w http.ResponseWriter, basic bool) {
	slog.Info("mcp token", "result", "invalid_client")
	var extra http.Header
	if basic {
		extra = http.Header{"WWW-Authenticate": {`Basic realm="hebrew-lessons"`}}
	}
	h.oauthErrorHeader(w, http.StatusUnauthorized, "invalid_client", "Client authentication failed.", extra)
}

func basicAuth(r *http.Request) (id, secret string, ok bool) {
	h := r.Header.Get("Authorization")
	const prefix = "basic "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", "", false
	}
	enc := strings.TrimSpace(h[len(prefix):])
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(enc)
	}
	if err != nil {
		return "", "", false
	}
	s := string(raw)
	i := strings.IndexByte(s, ':')
	if i < 0 {
		id, err = url.QueryUnescape(s)
		if err != nil {
			return "", "", false
		}
		return id, "", true
	}
	id, err = url.QueryUnescape(s[:i])
	if err != nil {
		return "", "", false
	}
	secret, err = url.QueryUnescape(s[i+1:])
	if err != nil {
		return "", "", false
	}
	return id, secret, true
}

func pkceS256(verifier, challenge string) bool {
	if !pkceVerifier.MatchString(verifier) || challenge == "" {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	if len(got) != len(challenge) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

func secretMatches(presented, storedHash string) bool {
	if presented == "" || storedHash == "" {
		return false
	}
	got := tokenDigest(presented)
	if len(got) != len(storedHash) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(storedHash)) == 1
}

func tokenDigest(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

const (
	consentOK = iota
	consentNoSession
	consentCSRF
)

func (h *oauthHandler) readConsent(r *http.Request) (int, User, url.Values, error) {
	body, err := readBody(r, oauthMaxBody)
	if err != nil {
		return 0, User{}, nil, err
	}
	vals, err := url.ParseQuery(string(body))
	if err != nil {
		return 0, User{}, nil, err
	}
	raw := h.svc.readSessionCookie(r)
	user, sess, err := h.svc.Lookup(r.Context(), raw)
	if err != nil {
		if errors.Is(err, ErrNoSession) {
			return consentNoSession, User{}, vals, nil
		}
		return 0, User{}, nil, err
	}
	if !h.originOK(r) || !tokensEqual(consentCSRFValue(r, vals), sess.CSRF) {
		return consentCSRF, user, vals, nil
	}
	return consentOK, user, vals, nil
}

func (h *oauthHandler) originOK(r *http.Request) bool {
	got, ok := requestOrigin(r)
	_, origin, baseOK := h.requestBase(r)
	return ok && baseOK && got == origin
}

func consentCSRFValue(r *http.Request, vals url.Values) string {
	if v := strings.TrimSpace(r.Header.Get("X-CSRF-Token")); v != "" {
		return v
	}
	return vals.Get("_csrf")
}

func (h *oauthHandler) browserSession(r *http.Request) (User, Session, bool) {
	user, sess, err := h.svc.Lookup(r.Context(), h.svc.readSessionCookie(r))
	if err != nil {
		return User{}, Session{}, false
	}
	return user, sess, true
}

func (h *oauthHandler) consentPage(w http.ResponseWriter, client db.OAuthClient, reqID, redirectURI, csrf string, status int, message string) {
	host := redirectTargetLabel(redirectURI)
	msg := ""
	if message != "" {
		msg = "<p class=\"err\">" + esc(message) + "</p>"
	}
	body := "<h1>Sign in</h1>" +
		"<p><strong>" + esc(client.Name) + "</strong> wants to read and write Hebrew lessons on this server.</p>" +
		"<p class=\"dim\">It will be sent back to <code>" + esc(redirectURI) + "</code> (" + esc(host) + "). Continue only if you started this from your own app.</p>" +
		msg +
		"<form method=\"post\" action=\"/authorize\">" +
		"<input type=\"hidden\" name=\"req\" value=\"" + esc(reqID) + "\">" +
		"<input type=\"hidden\" name=\"_csrf\" value=\"" + esc(csrf) + "\">" +
		"<button type=\"submit\" name=\"action\" value=\"approve\">Approve</button>" +
		"<button type=\"submit\" name=\"action\" value=\"deny\">Deny</button></form>"
	h.page(w, status, "Sign in to Hebrew lessons MCP", body)
}

func (h *oauthHandler) redirectError(w http.ResponseWriter, r *http.Request, redirectURI, state, errCode, desc string) {
	q := url.Values{}
	if errCode != "" {
		q.Set("error", errCode)
	}
	if desc != "" {
		q.Set("error_description", desc)
	}
	if state != "" {
		q.Set("state", state)
	}
	h.redirectQuery(w, r, redirectURI, q, true)
}

func (h *oauthHandler) redirectQuery(w http.ResponseWriter, r *http.Request, redirectURI string, q url.Values, failed bool) {
	u, err := url.Parse(redirectURI)
	if err != nil || u.Scheme == "" {
		h.page(w, http.StatusBadRequest, "Cannot sign in", "<h1>Cannot sign in</h1><p class=\"err\">Invalid request.</p>")
		return
	}
	existing := u.Query()
	for k, vs := range q {
		for _, v := range vs {
			if v != "" {
				existing.Set(k, v)
			}
		}
	}
	u.RawQuery = existing.Encode()
	target := u.String()
	h.cors(w.Header())
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	web := strings.HasPrefix(strings.ToLower(target), "http://") || strings.HasPrefix(strings.ToLower(target), "https://")
	if web {
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	title := "Signed in. Returning to the app."
	if failed {
		title = "Access was not granted."
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "<!doctype html><html><head><meta charset=\"utf-8\"><meta http-equiv=\"refresh\" content=\"0;url="+esc(target)+
		"\"><title>"+esc(title)+"</title></head><body style=\"font:16px system-ui,sans-serif;max-width:30rem;margin:12vh auto;padding:0 1rem\">"+
		"<p>"+esc(title)+"</p><p><a href=\""+esc(target)+"\">Return to the app</a></p></body></html>")
}

// redirectTargetLabel is the parenthetical on the consent page.
// A custom scheme shows the scheme. An http(s) URI shows the host.
func redirectTargetLabel(redirectURI string) string {
	u, err := url.Parse(redirectURI)
	if err != nil || u.Scheme == "" {
		return redirectURI
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return u.Scheme + ":"
	}
	if u.Host != "" {
		return u.Host
	}
	return redirectURI
}

func (s *Service) mcpGoogleURL(redirectURI, state, nonce, challenge string) (string, error) {
	u, err := url.Parse(s.authURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New("auth: google auth url")
	}
	q := u.Query()
	q.Set("client_id", s.clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s *Service) exchangeAt(ctx context.Context, code, verifier, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")
	form.Set("code_verifier", verifier)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errGoogle
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		slog.Warn("google token exchange failed", "err", err)
		return "", errGoogle
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || resp.StatusCode != http.StatusOK {
		slog.Warn("google token exchange failed", "status", resp.StatusCode)
		return "", errGoogle
	}
	var parsed struct {
		IDToken string `json:"id_token"`
	}
	if json.Unmarshal(body, &parsed) != nil || parsed.IDToken == "" {
		return "", errGoogle
	}
	return parsed.IDToken, nil
}

func (h *oauthHandler) googleCookieName() string {
	if h.svc.cookieSecure {
		return cookieMCPOAuthHost
	}
	return cookieMCPOAuth
}

func (h *oauthHandler) preflight(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodOptions {
		return false
	}
	h.cors(w.Header())
	w.Header().Set("Access-Control-Max-Age", "600")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
	return true
}

func (h *oauthHandler) cors(header http.Header) {
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, MCP-Protocol-Version")
	header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

func (h *oauthHandler) writeJSON(w http.ResponseWriter, status int, body any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		h.fail(w, err)
		return
	}
	h.cors(w.Header())
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	_, _ = w.Write(bytes.TrimRight(buf.Bytes(), "\n"))
}

func (h *oauthHandler) oauthError(w http.ResponseWriter, status int, code, description string) {
	h.oauthErrorHeader(w, status, code, description, nil)
}

func (h *oauthHandler) oauthErrorHeader(w http.ResponseWriter, status int, code, description string, extra http.Header) {
	h.cors(w.Header())
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	for k, vs := range extra {
		for _, v := range vs {
			w.Header().Set(k, v)
		}
	}
	var payload any
	if code != "" {
		payload = map[string]string{"error": code, "error_description": description}
	}
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return
	}
	_, _ = w.Write(bytes.TrimRight(buf.Bytes(), "\n"))
}

func (h *oauthHandler) page(w http.ResponseWriter, status int, title, bodyHTML string) {
	h.cors(w.Header())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\">"+
		"<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">"+
		"<title>"+esc(title)+"</title><style>"+
		"body{font:16px system-ui,sans-serif;max-width:30rem;margin:12vh auto;padding:0 1rem;color:#1b1b1b}"+
		"button{padding:.6rem 1.2rem;font-size:1rem;margin-right:.5rem}.err{color:#b00020}.dim{color:#555;font-size:.9rem}"+
		"code{background:#eee;padding:0 .25rem;word-break:break-all}"+
		"</style></head><body>"+bodyHTML+"</body></html>")
}

func (h *oauthHandler) notFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	_, _ = io.WriteString(w, "Not found")
}

func (h *oauthHandler) fail(w http.ResponseWriter, err error) {
	slog.Error("mcp oauth", "err", err)
	if w.Header().Get("Content-Type") != "" {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = io.WriteString(w, "OAuth request failed")
}

func readBody(r *http.Request, max int) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, int64(max)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > max {
		return nil, errBodyTooLarge
	}
	return b, nil
}

func newClientID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, s)
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func esc(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&#34;",
		"'", "&#39;",
	).Replace(s)
}
