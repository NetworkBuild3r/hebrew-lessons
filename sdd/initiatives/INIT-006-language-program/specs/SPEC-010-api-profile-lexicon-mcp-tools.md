# SPEC-010 · API: Profile, coverage, lexicon, and library MCP tools; HTTP twins; switch route

## Metadata

```yaml
spec_id: SPEC-010
initiative_id: INIT-006
title: "MCP tools and matching HTTP routes for the learner profile, coverage, lexicon, library; AI-planning switch"
domain: api
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-005, SPEC-007, SPEC-008, SPEC-009]
blocks: [SPEC-015, SPEC-017, SPEC-019]
```

---

## Description

**Context:** MCP tools are registered in `mcp/tools.js` with `register(server, name, meta, handler)` (`:142`–`:145`) and token-gated with `requireMcpAuthToken()` (as `list_users`, `:202`–`:218`). INIT-006 §12 is the parity table. SPEC-001 moved the switch route here from INIT-004 SPEC-011.

**Scope (in):**

- MCP tools in `mcp/tools.js` (a new `registerLearnerTools(server)` block), each a thin adapter:
  - `get_learner_profile({ username })` → `profile.buildProfile(…, { audience: 'mcp' })`.
  - `check_text_coverage({ username, text })` → `coverage.checkCoverage(…, { audience: 'mcp' })`. `text` ≤ 4000 characters.
  - `list_grammar_concepts()`.
  - `list_lexicon({ q, pos, binyan, tense, rootClass, verified, limit, cursor })`, `get_lexicon_entry({ key })`.
  - `upsert_lexicon_entries({ entries, expectedRev })` with actor `mcp`; rows are marked `source: 'mcp'`; `verified` in the input is ignored.
  - `add_library_words({ username, words })`: switch-gated like the profile. `words` are lexicon keys only. If any key is not already in the lexicon, the whole call is refused with `unknown_keys: [key]` and nothing is written (the AI writes the entry with `upsert_lexicon_entries` first; ADR-0009 §6). Then `library.add` with `source: 'ai'`; 50 words per learner per local day; audit `library_add` with actor `mcp`, username, count.
  - Descriptions written for the AI: when to call each, what `planning_off` means, and that the profile must not be pasted into a lesson file.
- HTTP twins: `GET /api/me/profile`, `POST /api/me/coverage`, `GET /api/me/mix` (session); `PUT /api/admin/lexicon` and `PATCH /api/admin/lexicon/:key/verified` (admin); `PATCH /api/admin/users/:id/ai-planning` (admin; audit `ai_planning_switch` with previous and new value, per the ADR-0009 §1 audit table).
- No MCP tool flips the switch or marks a form verified; attempts return 403-equivalent `forbidden`.

**Scope (out):** service logic (SPEC-005, SPEC-007, SPEC-008, SPEC-009); UI (SPEC-015, SPEC-017).

---

## Understood (agent's read of the intent)

- [ ] und-1: Each adapter is a few lines: auth, parse, call, shape. Logic lives in services.
- [ ] und-2: The eight existing tools keep their names and behavior.

## Acceptance Criteria

- [ ] ac-1 (REQ-013): a parity test calls each row of INIT-006 §12 through MCP and HTTP where both exist and asserts both reach the same service function (spy) with equivalent arguments.
- [ ] ac-2 (REQ-007): every new tool refuses without `MCP_AUTH_TOKEN`; `get_learner_profile` with the switch off returns `planning_off`.
- [ ] ac-3 (REQ-007): `PATCH /api/admin/users/:id/ai-planning` returns 401 signed out, 403 for a non-admin, 200 for an admin, and writes the audit row.
- [ ] ac-4: the 51st `add_library_words` word in a learner's local day is refused and nothing past the cap is written. A call naming a key that is not in the lexicon returns `unknown_keys` and writes no `user_words` or `review_items` row.
- [ ] ac-5: `list_users`, `list_words`, and the other existing tools still return their current shapes (existing tests unchanged).

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | `add_library_words` with switch off | Refused with `planning_off`. Writing into a learner's deck is personalization. | resolved | yes — ADR-0009 §1 |

> **Readiness** = 100.

---

## Deliverables

- [ ] Changes to `mcp/tools.js`; `server/api/learner-routes.js`; admin route additions in `server/api/admin-routes.js`
- [ ] Tests: `test/learner-mcp-tools.test.js`, `test/learner-parity.test.js`, `test/ai-planning-switch.test.js`

---

## Technical Requirements (domain-specific)

- [ ] Input schemas with `z` as the existing tools do; string length limits on every field.
- [ ] No profile content in log lines.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** the three test files and `npm test`.
- **Rollback plan:** remove `registerLearnerTools` and the route mounts.

---

## Security

- **Sensitivity:** required. Triggers matched: MCP read of learner data; MCP write into learner data; admin switch.
- **Security acceptance criteria:**
  - [ ] Token required on every new tool; switch and verification unreachable from MCP.
  - [ ] Rate and daily caps enforced before any write.
- **Review depth:** thorough · **focus areas:** authz matrix, audit, caps.

---

## Verification Strategy

- **Claim:** every §12 row is reachable on its adapters, gated as specified, and calls one implementation.
- **Check + executor:** `mechanized` — the three test files.
- **Pass condition:** exit 0; parity spy assertions pass for every row.
- **Test levels:** integration (in-process MCP server and HTTP server, temporary database, no network).

---

## Integration Points

- SPEC-015 calls `/api/me/mix`. SPEC-017 calls `/api/me/profile` and the switch route. SPEC-019 documents the tool names exactly as registered.
- Produces **Change:** learner MCP tools and HTTP twins.
- **Provenance tag:** `INIT-006/SPEC-010`
