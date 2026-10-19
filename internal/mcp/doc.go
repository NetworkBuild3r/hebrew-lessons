// Package mcp is the authoring server.
//
// This slice serves five tools (get_context, lookup, validate_unit,
// put_unit, publish_unit) and three resources over streamable HTTP.
// The other five tools (upsert_lexemes, get_unit, generate_audio,
// unit_history, delete_unit) register on the same server later. They
// are not stubbed here, so tools/list shows only what runs.
//
// An OAuth access token is accepted when SessionFromToken is set. That
// function maps the bearer to a session id. Sessions then calls
// auth.Service.Lookup, which re-checks the live allowlist and copies
// users.role. The static MCP_AUTH_TOKEN stays RoleAdmin. Discovery and
// registration are not this handler: the app mounts them outside it.
//
// Owned by MCP Builder. The package does not read the environment and
// does not import the database.
package mcp
