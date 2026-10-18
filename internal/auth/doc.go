// Package auth implements Google sign-in, browser sessions, and CSRF checks.
//
// Files matching oauth*.go in this package are reserved for the MCP Builder
// (the OAuth 2.1 authorization server). This package takes its clock from
// configuration and does not call time.Now.
package auth
