# SPEC-007 · Service: Learner profile contract and text coverage

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-006
title: "ADR-0009 §1 profile projection, switch, rate, audit; known-word coverage check"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-006, INIT-004/SPEC-006]
blocks: [SPEC-010]
```

---

## Description

**Context:** INIT-004 SPEC-010 planned a minimum snapshot under ADR-0004 §1. ADR-0009 §1 supersedes that with an aggregate profile; SPEC-001 trimmed INIT-004 SPEC-010 to personal-plan checks. The switch column `users.ai_planning` comes from INIT-004 SPEC-006. The rate limit, the role-by-route matrix, and the audit rows for these tools are ADR-0009 §1, which replaces ADR-0004 §4 and §10 for them.

**Scope (in):** `server/learner/profile.js` and `server/learner/coverage.js`:

- `buildProfile(userId, { now, audience })` with `audience` `mcp` or `self`:
  - Switch off and `audience: 'mcp'`: `{ error: 'planning_off' }`, no other key.
  - Otherwise exactly the ADR-0009 §1 returned fields, built from SPEC-006 functions through an explicit field-by-field projection (no spreading of model objects).
  - `audience: 'self'` returns the same object plus `ai_planning: true|false`, and no audit row and no rate count.
- `checkCoverage(userId, text, { audience })`: tokenizes Hebrew text (split on whitespace and punctuation, NFC), classifies each token as known, unknown, or non-Hebrew. A token is known only when (1) its `exactKey`, or its unique vowel-stripped form (ADR-0001 A1), matches a word the learner has passed (`learning` or `longTerm` in either direction) or has in the library, or (2) it is a lexicon `form` entry whose `lemma_key` is such a word. There is **no prefix stripping** (ADR-0009 §1, "Coverage tool"; ADR-0001 A1 and A2): a prefixed token is unknown until a `form` entry records it. `text` is at most 4000 characters. Returns `{ coverage, known_count, running_words, unknown: [token] (deduped, max 100) }`. Same switch gate for `mcp`.
- Rate limiter: 60 per clock hour for the MCP credential, shared by profile and coverage, in memory, reset on the hour. Over the limit: `{ error: 'rate_limited' }`.
- Audit: `learner_profile_read` / `learner_coverage_read` with actor `mcp`, learner username, field names included or `none`, counts (`word_count`, `concept_count`, `lesson_count` for the profile; `running_words`, `unknown_count` for coverage). A `planning_off` or `rate_limited` refusal is audited too, with `fields: none`. No word keys and no text in the audit row.
- Constant `NEVER_RETURNED` in `server/learner/profile-contract.js`, copied once from ADR-0009 §1, exported for tests.

**Scope (out):** adapters (SPEC-010); UI (SPEC-017).

---

## Understood (agent's read of the intent)

- [ ] und-1: The learner sees exactly what the AI would see. Transparency is the mitigation for sending more than ADR-0004 allowed.
- [ ] und-2: Coverage for `mcp` reveals which words are unknown, so it is learner data and is gated like the profile.

## Acceptance Criteria

- [ ] ac-1 (REQ-007): with the switch on, the profile JSON contains every ADR-0009 §1 field and a recursive key and value walk finds no name in `NEVER_RETURNED`, no ISO timestamp string, and no numeric user id.
- [ ] ac-2 (REQ-007): switch off yields `{ error: 'planning_off' }` only, and an audit row with `fields: none`.
- [ ] ac-3 (REQ-007): the 61st MCP read in a clock hour (profile and coverage combined) returns `rate_limited` and writes an audit row.
- [ ] ac-4 (REQ-008): `audience: 'self'` returns a deep-equal object to the `mcp` result (minus `ai_planning`) for the same fixture, and writes no audit row.
- [ ] ac-5 (REQ-009): on a fixture text of 20 Hebrew words with 18 known, coverage is 0.9 and `unknown` lists the other two. A known word with a leading ו is in `unknown` and lowers coverage, until a `form` entry whose `lemma_key` is the known word exists, after which it is known. The result has no `matched_by` field.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | INIT-004 SPEC-006 not yet run | This spec does not add `users.ai_planning` itself. It is blocked until that column exists. | resolved | yes — §4 external edge |
| aud-2 | Which accounts the MCP may profile | Any active account by username, subject to that account's switch. Disabled accounts: `not_found`. | resolved | yes — ADR-0009 §1 matrix |

> **Readiness** = 100 once INIT-004 SPEC-006 is complete.

---

## Deliverables

- [ ] `server/learner/profile.js`, `server/learner/profile-contract.js`, `server/learner/coverage.js`
- [ ] Tests: `test/learner-profile.test.js`, `test/learner-coverage.test.js`

---

## Technical Requirements (domain-specific)

- [ ] Projection is an explicit list. A test fails if a new key appears in the output that is not in the ADR-0009 returned-fields list.
- [ ] No `SELECT *`. Profile queries never select `password_hash`, `display_name`, or `sessions`.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** both test files and `npm test`.
- **Rollback plan:** delete the modules; no persistent state besides audit rows.

---

## Security

- **Sensitivity:** required. Triggers matched: learner data leaving toward a third-party model.
- **Security acceptance criteria:**
  - [ ] Never-returned walk passes; no timestamps; no ids.
  - [ ] Switch-off and rate-limit responses carry no profile field.
  - [ ] Audit rows hold counts and field names only.
- **Review depth:** thorough · **focus areas:** data minimization, switch authz, audit completeness.

---

## Verification Strategy

- **Claim:** the profile and coverage outputs match ADR-0009 §1 exactly, are gated, rate-limited, and audited.
- **Check + executor:** `mechanized` — `node --test test/learner-profile.test.js test/learner-coverage.test.js`.
- **Pass condition:** exit 0; walk finds nothing; key set equals the contract list.
- **Test levels:** integration on a temporary database. No network.

---

## Integration Points

- SPEC-010 is the only adapter for `mcp`; `GET /api/me/profile` and `POST /api/me/coverage` call with `audience: 'self'`.
- Produces **Change:** profile contract.
- **Provenance tag:** `INIT-006/SPEC-007`
