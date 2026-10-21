# SPEC-007 Completion Summary — Lesson authoring and MCP assignment

**Initiative:** INIT-002-multi-user-progress
**Spec:** SPEC-007 · Lesson authoring service and MCP lesson assignment
**Status:** complete
**Provenance:** INIT-002/SPEC-007

## Change

Lessons are saved through one path, `mcp/store.upsertLesson`. The AI can list active usernames and assign who sees a lesson. A new lesson with no assignment stays hidden, and the tool reply says so.

## Gates

- Autonomy `auto_tests`. `gate_actions: []`.
- Pre-flight: gate_actions PASS · security_review=required FAIL · domain=service FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false. Tier: full.

## AC verification

Independent of the author. 12 tests passed. Real-tree audio verify: 65 recordings. See `SPEC-007-eval-report.md`.

## Security

Assignment tools refuse unless `MCP_AUTH_TOKEN` is set, and they resolve usernames before any file write. `list_users` returns only username and display name for active accounts. No Critical finding. The token check is inside the tools; requiring the token on the MCP HTTP listener is SPEC-011.

## Degradation

Timer and trace skipped (`scripts/dev` absent). No persona. No commit. `mcp/store.js` was left unchanged.

## Harness

Cursor. Model passed: `cursor-grok-4.5-high-fast`. Worker: [SPEC-007](3c0e894d-982b-47d3-997b-019efb5435b8).
