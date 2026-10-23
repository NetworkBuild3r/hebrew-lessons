# SPEC-002 · frontend: Roots hub and the word-card link

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-018
title: "Roots hub and the word-card link"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [deploy]
reversibility_class: remote
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-001]
blocks: [INIT-019/SPEC-004]
```

## Description

**Context:** Each root gets a page that ties words, forms, lessons and verses together, and a word card should lead to it.

**Scope (in):**

- `GET /roots`: visible roots, each with its letters, a gloss of the core meaning (from its most common word), and counts.
- `GET /roots/{root}`: words as word cards (the `wordCard` form, the learner's state as a quiet badge), verb forms table when present, "In your lessons", "In Isaiah" (verses with the root's words marked), each verse linking to the Isaiah reader when INIT-021/SPEC-003 exists and to nothing before.
- Word card back: a small "Root שׁ־מ־ע" link when `roots.Visible` is true.
- A root page that is not visible returns 404.

**Scope (out):** Editing roots. Search.

## Acceptance Criteria

- [ ] ac-1: `/roots` lists exactly `roots.List`; an invisible root 404s. (REQ-006)
- [ ] ac-2: A word card shows the root link only for a visible root, and the link goes to that root's page. (REQ-006)
- [ ] ac-3: Pages pass at 390 and 1440, light and dark; Hebrew is in `bdi` with `lang` and `dir`.

## Deliverables

- [ ] `internal/web/roots.go`, `views/roots.templ`, the card back change in `player.templ`, CSS in `flip.css`
- [ ] Render tests; browser test with screenshots

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** deploy. **Rollback:** revert and redeploy.

## Security

- **Sensitivity:** light — read-only pages; the root path segment is validated as Hebrew letters and maqaf before lookup.

## Verification Strategy

- **Claim:** every visible root has a page, invisible ones do not, and cards link to them.
- **Check + executor:** mechanized — `go test ./internal/web/...`; browser suite with `-tags browser`.
- **Pass condition:** ac-1 to ac-3 tests and checks pass.

## Integration Points

- INIT-019/SPEC-004 adds Roots to navigation. INIT-021/SPEC-003 links verses back here.
- **Provenance tag:** `INIT-018/SPEC-002`
