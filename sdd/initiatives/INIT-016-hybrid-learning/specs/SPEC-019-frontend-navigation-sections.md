# SPEC-019 · frontend: Navigation around the sections

## Metadata

```yaml
spec_id: SPEC-019
initiative_id: INIT-016
title: "Navigation around the sections"
domain: frontend
status: planned
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [deploy]
reversibility_class: remote
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-009, SPEC-011, SPEC-015]
blocks: []
```

## Description

**Context:** The site is organized around what you are learning: Path, Learn, Roots, Isaiah, My stuff. The dock and rail today hold Today, Lessons, Review, Progress, Me.

**Scope (in):**

- Path keeps Today first; Lessons and Review live under it.
- Learn, Roots and Isaiah appear only when they have content (at least one published explainer, one visible root, one checked verse note).
- My stuff replaces Me and holds words, verbs, mistakes, "You can now say", and account settings.
- The phone dock stays at five items; desktop rail may show more. The existing shell browser tests are updated, not weakened.

**Scope (out):** Practice-by-worksheet-type browse (follows SPEC-023 once units carry a type).

## Acceptance Criteria

- [ ] ac-1: With no explainers, roots or verse notes, those three items are absent; adding one makes its item appear. (REQ-012)
- [ ] ac-2: The dock fits 320 to 390 px with full labels, as the shell suite already checks. (REQ-012)
- [ ] ac-3: Every existing route remains reachable within two taps from Today.

## Deliverables

- [ ] `views/layout.templ` shell, `internal/web/shell.go`, CSS
- [ ] Shell browser test updates; render tests for conditional items

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** deploy. **Rollback:** revert and redeploy.

## Security

- **Sensitivity:** light.

## Verification Strategy

- **Claim:** navigation follows the sections and hides empty ones.
- **Check + executor:** mechanized — `go test ./internal/web/...`; `TestShellInBrowser` with `-tags browser`.
- **Pass condition:** ac-1 to ac-3 tests and checks pass.

## Integration Points

- Links SPEC-006, 009, 011, 015, 022 pages.
- **Provenance tag:** `INIT-016/SPEC-019`
