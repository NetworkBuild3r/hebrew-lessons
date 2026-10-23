# SPEC-001 · service: Root family read model

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-018
title: "Root family read model"
domain: service
status: planned
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [INIT-016/SPEC-002]
blocks: [SPEC-002, INIT-021/SPEC-003]
```

## Description

**Context:** Roots tie the sections together. A root page shows only when it has at least two words or a verse (D7). `Store.VersesContaining` already searches Tanakh verses by a plain word.

**Scope (in):** a `roots` package:

- `Family(ctx, root, userID) (Family, error)`: the root; its words (text, translit, gloss, pos, binyan, the learner's state for each); verb forms when INIT-019/SPEC-001 has loaded them (empty until then); lessons that use any of the words (from `lesson_items`, published units visible to this learner); up to 10 verses containing any of the words, Isaiah first.
- `List(ctx, userID) ([]Summary, error)`: roots that pass the rule (≥ 2 words, or ≥ 1 word and ≥ 1 verse), with counts.
- `Visible(root)` for a single root, so a link is only drawn when the page exists.

**Scope (out):** Pages (SPEC-002).

## Acceptance Criteria

- [ ] ac-1: A root with one word and no verse is not listed and `Visible` is false; with two words, or one word and a verse, it is listed. (REQ-006)
- [ ] ac-2: Lessons come only from units this learner can see (shared or their own scope). (REQ-006)
- [ ] ac-3: Verse lookup is capped and does not scan more than the existing index allows (timed test on a fixture corpus).

## Deliverables

- [ ] `internal/roots/{doc,family}.go` and tests

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** none. **Rollback:** revert.

## Security

- **Sensitivity:** standard — learner states and learner-scoped units are filtered by the session's user id.

## Verification Strategy

- **Claim:** root families are built correctly and follow the visibility rule.
- **Check + executor:** mechanized — `go test ./internal/roots/...`.
- **Pass condition:** the visibility table test, the scope test and the capped-verse test pass.

## Integration Points

- SPEC-002 and INIT-021/SPEC-003 render it; INIT-019/SPEC-001's verb forms fill `Family.Forms`.
- **Provenance tag:** `INIT-018/SPEC-001`
