# SPEC-002 completion summary

**Initiative:** INIT-017 · **Spec:** SPEC-002 (api) · **Status:** complete · **Date:** 2026-10-23

## Change

An admin can write patterns and explainers over MCP. A learner token can list patterns and cannot write them. An explainer is parsed to an allow-listed tree and refused, without a write, when it uses raw HTML, an image, or an off-site link. `validate_unit` blocks a shared unit that needs a pattern taught later, and names that unit. `check_path_order` lists every such break and a stable order that keeps class order where it can.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestPatternToolsAdminLearnerAndPath`: the admin writes, the learner is refused, and the audit actor is the token. |
| ac-2 | PASS | The same test refuses `<script>`, an image, and an external link, and the explainer table stays empty. `TestParseRefusesOutsideTheGrammar` is the parser table. |
| ac-3 | PASS | `validate_unit` on a unit placed before its prerequisite names the later unit. |
| ac-4 | PASS | Two violations come back, and the proposed order is filler, then the teaching unit, then the two that depend on it. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- No security-review registry is available. The security checks are in the code and the test: the body is parsed and refused rather than stripped, both writes are admin-only, and each write is a `publish_audit` row for the token's actor.
- The four tools live in `internal/mcp/tool_patterns.go`. The spec named three files.
