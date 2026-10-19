// Package mcp is the authoring server.
//
// This slice serves eight tools (get_context, lookup, validate_unit,
// put_unit, upsert_lexemes, generate_audio, publish_unit,
// get_learner_words) and three resources over streamable HTTP. The
// other three tools (get_unit, unit_history, delete_unit) register on
// the same server later. They are not stubbed here, so tools/list shows
// only what runs.
//
// An OAuth access token is accepted when SessionFromToken is set. That
// function maps the bearer to a session id. Sessions then calls
// auth.Service.Lookup, which re-checks the live allowlist and copies
// users.role. The static MCP_AUTH_TOKEN stays RoleAdmin.
// MCP_READONLY_TOKEN, when set, is a second static bearer. A match is
// RoleLearner bound to MCP_READONLY_LEARNER_ID and may call only
// get_context, lookup, get_learner_words, and validate_unit. get_context
// and get_learner_words are limited to that learner id. The process does
// not create a users row. Point the id at a dedicated smoke learner so
// the smoke does not read a real student. get_context still succeeds
// when that learner has no placement row and reports an empty placement.
// An empty MCP_READONLY_TOKEN leaves the smoke bearer off. Startup
// refuses a set token shorter than 32 characters, equal to
// MCP_AUTH_TOKEN, or paired with a missing, non-integer, or non-positive
// MCP_READONLY_LEARNER_ID. Both bearers work whether OAuth is on or off.
// Discovery and registration are not this handler: the app mounts them
// outside it.
//
// Owned by MCP Builder. The package does not read the environment and
// does not import the database.
package mcp
