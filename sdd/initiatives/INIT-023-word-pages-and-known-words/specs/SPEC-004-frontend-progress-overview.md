# SPEC-004 · frontend: Progress opens on every skill at once

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-023
title: "Progress opens on every skill at once"
domain: frontend
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 2 hours
blocked_by: []
blocks: [SPEC-010]
```

## Description

**Context:** `chooseSkill` in `internal/web/progress.go` falls back to `learner.Skills[0]`, which is `listen`. Progress opens on the Listen tab and reads "0 known" while Read has 11.

**Scope (in):**
- With no `?skill=`, "What you know" shows an overview: one row per skill with known plus solid, learning, and seen, and the overall words known. Each row links to that skill's tab.
- A skill with answers and no cards shows a line in place of a zero bar: "Listening counts once you have a listening card. Lessons make one when you answer a listening question."
- The estimated-words chart uses the skill with the most known words when no tab is chosen. A day before INIT-023 keeps its stored value.
- Keep the four tabs. `?skill=listen` still opens Listen.

**Scope (out):** Changing what a snapshot stores.

## Acceptance Criteria

- [ ] ac-1: `/progress` with learner 1's fixture shows Read 11, Recall 2, Listen 0, and Spell 0 known in the overview.
- [ ] ac-2: The Listen row with 29 answered words and no card shows the explanation line, not a bar.
- [ ] ac-3: `/progress?skill=read` renders the Read tab as today.
- [ ] ac-4: The overview's numbers match `get_learner_model` counts for the same fixture.

## Deliverables

- [ ] `internal/web/progress.go` and `progress_test.go`
- [ ] `internal/web/views/progress.templ`, `progress_templ.go`, and `progress.go`
- [ ] `internal/web/static/css/progress.css`

## Verification Strategy

- **Claim:** Progress shows what is known in every skill on first load.
- **Check + executor:** mechanized — `go test ./internal/web -run Progress`. Browser — `/progress` in the local container.
- **Pass condition:** ac-1 to ac-4 pass.

## Integration Points

- **Provenance tag:** `INIT-023/SPEC-004`
