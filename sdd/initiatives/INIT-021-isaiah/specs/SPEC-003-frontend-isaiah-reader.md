# SPEC-003 · frontend: Isaiah reader

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-021
title: "Isaiah reader"
domain: frontend
status: planned
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [deploy]
reversibility_class: remote
security_review: light
estimated_effort: 6 hours
blocked_by: [INIT-018/SPEC-001, SPEC-002]
blocks: []
```

## Description

**Context:** Isaiah study goes verse by verse, each with layers, and feeds speaking through roots and the bridge to modern Hebrew.

**Scope (in):**

- `GET /isaiah`: chapters with at least one checked verse.
- `GET /isaiah/{chapter}/{verse}`: the pointed verse large; word-by-word glosses under each word (tap a word for its gloss and root, the same card pattern as lessons); then the layers as sections — Roots (each linking to its root page when visible), The manner of the Jews, Bridge to modern Hebrew; the JPS English behind a "Show the English" toggle. Previous and next verse.
- Only checked notes and glosses show; a verse with none shows the text and English only.
- The verse orb plays a clip when one exists; no new audio generation here.

**Scope (out):** Other books. Cantillation display.

## Acceptance Criteria

- [ ] ac-1: A draft note never appears on the page. (REQ-013)
- [ ] ac-2: Each word with a checked gloss is tappable and shows gloss and root; the root links only when `roots.Visible`. (REQ-013, REQ-006)
- [ ] ac-3: The page passes at 390 and 1440, light and dark; pointed text keeps its line height; RTL is correct.

## Deliverables

- [ ] `internal/web/isaiah.go`, `views/isaiah.templ`, CSS
- [ ] Render tests; browser test with screenshots

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** deploy. **Rollback:** revert and redeploy.

## Security

- **Sensitivity:** light — read-only; chapter and verse parsed as bounded integers.

## Verification Strategy

- **Claim:** a learner reads Isaiah verse by verse with checked layers only.
- **Check + executor:** mechanized — `go test ./internal/web/...`; browser suite with `-tags browser`.
- **Pass condition:** ac-1 to ac-3 tests and checks pass.

## Integration Points

- INIT-018/SPEC-002's root pages link verses here; INIT-019/SPEC-004 shows Isaiah when content exists.
- **Provenance tag:** `INIT-021/SPEC-003`
