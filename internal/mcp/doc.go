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
// users.role. The static MCP_AUTH_TOKEN stays RoleAdmin. Discovery and
// registration are not this handler: the app mounts them outside it.
//
// Owned by MCP Builder. The package does not read the environment and
// does not import the database.
package mcp
