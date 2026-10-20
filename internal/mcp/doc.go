// Package mcp is the authoring server.
//
// This slice serves thirteen tools and four resources over streamable HTTP.
// The authoring tools are get_context, lookup, validate_unit, put_unit,
// upsert_lexemes, generate_audio, publish_unit, and get_learner_words.
// The learner tools (INIT-015/SPEC-021) are get_learner_model, get_progress,
// get_lesson_brief, add_coach_note, and unpublish_unit. The resources are
// hebrew://schema/unit, hebrew://catalog/activities, hebrew://rules, and
// hebrew://authoring. The other three tools (get_unit, unit_history,
// delete_unit) register on the same server later. They are not stubbed
// here, so tools/list shows only what runs.
//
// The learner tools read a store that a CourseReader may also implement
// (LearnerData), found with a type assertion, so Options has no field for it.
// Every one takes its learner from the principal (learnerFor): a learner
// account is its own learner and may not name another, and an admin, including
// the static token, must name one. get_lesson_brief and add_coach_note are
// admin-only. Calls to them, get_learner_model, get_progress, unpublish_unit,
// and validate_unit with a briefId share a limit of 60 a minute for each caller.
//
// An OAuth access token is accepted when SessionFromToken is set. That
// function maps the bearer to a session id. Sessions then calls
// auth.Service.Lookup, which re-checks the live allowlist and copies
// users.role. The static MCP_AUTH_TOKEN stays RoleAdmin.
// MCP_READONLY_TOKEN, when set, is a second static bearer. A match is
// RoleLearner bound to MCP_READONLY_LEARNER_ID. Its actor is
// mcp-readonly; the admin static token stays mcp. It may call only the
// read tools: get_context, lookup, get_learner_words, get_learner_model,
// get_progress and validate_unit. It may not call unpublish_unit or any
// admin tool. The tools that take a learner id are limited to that id. The process does
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
