// Package auth implements Google sign-in, browser sessions, and CSRF checks.
//
// Files matching oauth*.go in this package are reserved for the MCP Builder
// (the OAuth 2.1 authorization server). This package takes its clock from
// configuration and does not call time.Now.
//
// Browser sign-in is an OIDC authorization-code flow (state, nonce, PKCE
// S256) with the ID token checked against the configured Google issuer.
// The session cookie carries 32 random bytes; sessions.id_hash stores only
// their SHA-256. CSRF uses the per-session token in sessions.csrf.
package auth
