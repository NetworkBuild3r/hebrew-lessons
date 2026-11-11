# SPEC-011 · web: Today offers one session, and side features sit behind switches

## Metadata

```yaml
spec_id: SPEC-011
initiative_id: INIT-038
title: "Today offers one session, and side features sit behind switches"
domain: web
status: complete
primary_prompt: none
complexity: low
complexity_reason: one main action and four config switches, with no code deleted
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-006]
blocks: [SPEC-013]
```

## Description

- **One main action.** Today's main action is "Today's session, about N minutes", using SPEC-006's session size and counting only cards that can be asked today. The files are `internal/web/shell.go`, about lines 72 to 86, and `internal/web/views/today.templ`. When nothing is due, Today says so in one line.
- **Config switches, off by default:**
  - the Roots and Isaiah menu links and routes (`shell.go`, about lines 191 to 202 and 234 to 268);
  - the placement start and the check-in (`shell.go`, about lines 79 to 86 and 594 to 635).

  A route behind a switch that is off returns 404. No code is deleted, and each switch is undone by one config change.
- **Generated code.** Templates are regenerated with `templ generate`.

**What Brian will see differently:** Today shows one "Today's session, about 25 minutes" button, and Isaiah, Roots, placement, and the check-in are gone from Today and the menu. A screenshot of the new Today goes in the completion summary.

**Removes:** the side features from Today and the menu (hidden, not deleted), and Today's competing actions.

**Scope (out):** the session size (SPEC-006); the headline count (SPEC-010); deleting any side feature (Stage 2, if ever).

## Acceptance criteria

- [ ] Today shows exactly one main action, with the session length in minutes.
- [ ] With the switches off, no Roots, Isaiah, placement, or check-in link appears, and their routes return 404.
- [ ] With each switch on, its feature behaves as it does today.
- [ ] With no cards due, Today says nothing is due and offers no empty session.
- [x] The page renders at a phone width, shown by the screenshot. Phone capture `specs/completion-summaries/shots/today-390.png` (scroll width did not exceed the screen). Figma frame `257:123` on page `257:71`, [Progress and Today](https://www.figma.com/design/NJ8HfTPCQgrFty2VVQBMhY?node-id=257-123).

## Verification Strategy

- **Claim:** Today has one thing to do, and the side features are out of the way without being deleted.
- **Check + executor:** mechanized. `go test ./internal/web/...` with each switch both ways. A human check of the screenshot by Brian.
- **Pass condition:** every acceptance case passes, and the screenshot is in the completion summary.
