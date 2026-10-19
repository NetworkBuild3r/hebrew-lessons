// Package mcp is the authoring server.
//
// This slice serves four tools (get_context, lookup, validate_unit,
// put_unit) and three resources over streamable HTTP. The other six
// tools (upsert_lexemes, get_unit, generate_audio, publish_unit,
// unit_history, delete_unit) and the OAuth bearer check register on
// the same server later. They are not stubbed here, so tools/list
// shows only what runs.
//
// Owned by MCP Builder. The package does not read the environment and
// does not import the database.
package mcp
