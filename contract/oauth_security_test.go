package contract

import "testing"

// These two OAuth behaviors are known security bugs. They must not be frozen
// as the contract. The tests stay skipped, marked expected-to-be-fixed, until
// the server rejects them. In-memory authorization codes and pending sign-ins
// are not in this list: that storage is fine to keep.
//
// Skipped tests do not boot an OAuth flow and do not record a golden.

func TestTokenChecksClientSecret(t *testing.T) {
	t.Skip("expected-to-be-fixed: POST /token does not check client secrets. Do not freeze this. A later OAuth PR asserts the fixed behavior and stays skipped until the server rejects a wrong or missing secret.")
}

func TestAuthCodeIsBoundToTheRegisteringClient(t *testing.T) {
	t.Skip("expected-to-be-fixed: a client that shares a redirect URI can redeem another client's authorization code. Do not freeze this. A later OAuth PR asserts the code is bound to the client that started the grant, and stays skipped until that is true.")
}
