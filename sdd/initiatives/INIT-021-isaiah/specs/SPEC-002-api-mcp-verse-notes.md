# SPEC-002 · api: MCP verse notes, drafted and checked

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-021
title: "MCP: verse notes, drafted and checked"
domain: api
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: [.claude/agents/security-specialist/AGENT.md]
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 3 hours
blocked_by: [SPEC-001, INIT-017/SPEC-002]
blocks: [SPEC-003]
```

## Description

**Context:** An Isaiah agent drafts one verse at a time; a person who reads Hebrew checks it (D10). The check must be a human act, not something the drafting agent can do for itself.

**Scope (in):**

- `upsert_verse_notes` (admin): notes and word glosses for one verse; always stored as `draft`; bodies checked with INIT-017/SPEC-002's grammar.
- `get_verse` (any token): the verse text, English, and all notes with their status (drafts included for admins, checked only for learners).
- Checking is **not** an MCP tool. It is a button on an admin-only web page (`/admin/isaiah/{chapter}/{verse}`) that a signed-in admin presses, recorded with their user id.

**Scope (out):** The learner reader (SPEC-003).

## Acceptance Criteria

- [ ] ac-1: A note written through MCP is always `draft`, whatever the input says. (REQ-013)
- [ ] ac-2: No MCP tool can set `checked`; the admin page can, and records the checker. (REQ-013)
- [ ] ac-3: A learner token's `get_verse` returns checked notes only.

## Deliverables

- [ ] `internal/mcp/tool_verse_notes.go`; `internal/web/admin_isaiah.go` and a small template
- [ ] Tests

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** none. **Rollback:** revert.

## Security

- **Sensitivity:** required — agent-written content reaches learners only through a human gate.
- **Security acceptance criteria:**
  - [ ] The check endpoint requires an admin web session and CSRF; it is not reachable with an MCP token.
  - [ ] Bodies are parsed with the restricted grammar at write time.
- **Review depth:** standard · **focus areas:** gate bypass, authorization, stored injection.

## Verification Strategy

- **Claim:** agents can draft and only a person can publish.
- **Check + executor:** mechanized — `go test ./internal/mcp/... ./internal/web/...`.
- **Pass condition:** ac-1 to ac-3 tests and the "MCP cannot check" test pass.

## Integration Points

- SPEC-003 reads checked notes.
- **Provenance tag:** `INIT-021/SPEC-002`
