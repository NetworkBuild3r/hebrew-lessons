# SPEC-006 · frontend: "You can now say" on Progress and Today

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-016
title: "You can now say on Progress and Today"
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
blocked_by: [SPEC-005]
blocks: []
```

## Description

**Context:** SPEC-005 computes what the learner can say. The learner needs to see it where they already look.

**Scope (in):**

- Progress: a "You can now say" section, with the weekly number, the list (Hebrew in the text form, play orb where a clip exists), and "Almost there" with up to 5 words.
- Today: one line under the main action, "You can now say 23 things · 4 this week", linking to the section.
- Empty state: before any word is known, a plain line about how the list fills in.

**Scope (out):** Streaks (already partly present), My stuff (SPEC-015).

## Acceptance Criteria

- [ ] ac-1: Progress lists exactly `CanSay.Items`, shows `Total` and `AddedThisWeek`, and the Today line matches. (REQ-002)
- [ ] ac-2: Hebrew is in `bdi` with `lang="he" dir="rtl"`; the layout passes at 390 and 1440, light and dark, with no sideways scroll.
- [ ] ac-3: With no known words the section shows the empty-state line and Today shows no count.

## Deliverables

- [ ] `internal/web/views/progress.templ`, `today.templ`, handlers, CSS in the existing files
- [ ] Go render tests and a browser check with screenshots

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** deploy (push to `main`, tag bump in home_k3, `/version` check). **Rollback:** revert and redeploy.

## Security

- **Sensitivity:** light — renders the signed-in learner's own data.

## Verification Strategy

- **Claim:** the learner sees what they can say and how much it grew this week.
- **Check + executor:** mechanized — `go test ./internal/web/...` and the Progress browser suite with `-tags browser`.
- **Pass condition:** render tests for list, counts and empty state pass; the browser checks pass at both widths and schemes.

## Integration Points

- Reads SPEC-005. SPEC-019 links it from the section navigation.
- **Provenance tag:** `INIT-016/SPEC-006`
