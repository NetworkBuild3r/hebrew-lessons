# SPEC-010 · web: Progress and Today lead with the ladder

## Metadata

```yaml
spec_id: SPEC-010
initiative_id: INIT-038
title: "Progress and Today lead with the ladder"
domain: web
status: complete
primary_prompt: none
complexity: medium
complexity_reason: rebuilds the Progress headline and adds a one-time comparison drawn from two rule versions
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-005]
blocks: [SPEC-014]
```

## Description

The files are `internal/web/progress.go` (`serveProgress` at line 69, `fillSkillPanel` at 263, `fillOverview` at 316), `internal/web/views/progress.templ`, and `internal/web/views/today.templ`.

- **The top rows are "Understand by ear" and "Hebrew from English".**
  - Each shows proved, recalled later, slipping, and not yet asked, out of N taught words.
  - Each count links to its words.
  - Until tutor words are recorded, the label says "out of words the app has taught you".
  - Reading and spelling sit below as smaller rows.
  - Every percentage carries its count.
- **A one-time "What changed" panel,** shown for 14 days after release B. It shows three numbers side by side, with a one-line reason:
  - the old number under rule 4, voided answers included;
  - the number after the void;
  - the number under the ladder.

  The explanation sentence states the whole rule.
- **The history chart** is drawn under one rule version only.
- **Today's "Words you know"** becomes the "Hebrew from English" proved count.
- **Generated code.** Templates are regenerated with `templ generate`.

**What Brian will see differently:** Progress says what he can do, in his terms. It shows once why the numbers moved, and no two numbers on the page contradict each other. A screenshot of the new page goes in the completion summary.

**Removes from the headline:**
- the any-skill count;
- placement estimates;
- band verdicts with under 30 answers;
- the weekly "gained N words" sentence.

**Scope (out):** the ladder rule itself (SPEC-005); Today's main action (SPEC-011).

## Acceptance criteria

- [ ] For a fixture learner, each top row's four counts sum to N taught.
- [ ] Each count links to a list of exactly those words.
- [ ] The "What changed" panel shows the three numbers and is gone after 14 days.
- [ ] None of the removed elements appears on the page.
- [ ] The history chart draws points from one rule version only.
- [ ] Today's count equals Progress's "Hebrew from English" proved count.
- [x] The page renders at a phone width with no horizontal scroll, shown by the screenshot. Phone capture `specs/completion-summaries/shots/progress-390.png` (scroll width did not exceed the screen). Figma frame `257:85` on page `257:71`, [Progress and Today](https://www.figma.com/design/NJ8HfTPCQgrFty2VVQBMhY?node-id=257-85).

## Verification Strategy

- **Claim:** Progress leads with the ladder, and its numbers agree with each other and with Today.
- **Check + executor:** mechanized. `go test ./internal/web/...` with fixture learners. A human check of the screenshot by Brian.
- **Pass condition:** every acceptance case passes, and the screenshot is in the completion summary.
