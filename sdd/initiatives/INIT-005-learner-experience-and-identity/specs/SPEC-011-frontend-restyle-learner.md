# SPEC-011 · Frontend: Restyle learner views

## Metadata

```yaml
spec_id: SPEC-011
initiative_id: INIT-005
title: "Restyle Today, Lessons, player, Review, Progress"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-004, SPEC-009, SPEC-010]
blocks: [SPEC-013]
```

---

## Description

**Context:** Structure, tokens, feedback kit and icons are ready. Apply ADR-0008 to every learner view in `shared/worksheet.css` without renaming queried hooks.

**Scope (in):** Today, Lessons, lesson/player views, Review, Progress; token application; stray English label cleanup (REQ-016); feedback kit wired on correct/wrong and complete.

**Scope (out):** login and admin (SPEC-012); screenshot suite (SPEC-013).

---

## Understood

- [x] und-1: Class and `data-*` hooks a test or smoke queries change only in the same change as that test. _(Closure: no hook, class or id was renamed or removed. No test file was edited. The only markup removed is the page-heading kicker, which no test queries.)_
- [x] und-2: Default is to restyle the legacy player now (initiative Q2 default). _(Closure: the lesson player chrome (top bar, path list, buttons) now matches the shell. The activities were already restyled by INIT-007 and were not touched.)_

## Acceptance Criteria

- [x] ac-1 (REQ-017): learner views use ADR-0008 tokens; contrast pairs still pass. _(Closure: the look is ADR-0010, which is the adopted look; ADR-0008 was superseded. Colours come from `var(--color-*)` tokens declared in the SPEC-011 block, with a `data-theme="dark"` set. Measured pairs: text on paper 16.5, muted on white 10.3, current nav 5.8, white on primary 6.7, status chips 6.9 to 9.1, control border on white 3.7. `test/design-tokens.test.js` passes. A few fixed hexes remain (green and amber button fills, two progress-bar segments); they are light-theme values.)_
- [x] ac-2 (REQ-016): no stray English-only chrome labels on Hebrew screens (glosses remain). _(Closure: removed the "Hebrew Way" kicker from every page heading; English-only `aria-label`s on the Lessons filter, the stage bar, the Words strength bar and the typed-answer box are now Hebrew; the lesson level shows as a Hebrew word with a gloss (cards and the player path) instead of the raw word "basic"; the Progress empty-state sentence lost its word-by-word glosses. Not changed: English lesson blurbs and the English-only admin field names come from catalog and lesson data; the blurb is now marked `lang="en" dir="ltr"` as a gloss.)_
- [x] ac-3 (REQ-008): existing learner tests and applicable smokes still pass. _(Closure: `npm test` with `SRS_SCHEDULER=ladder` 1040 pass, 0 fail; all 17 `test/chrome-*-smoke.js` exit 0.)_

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Restyle legacy player | Yes, per initiative default | resolved | yes — unless operator changes Q2 |

> **Readiness** = 100.

---

## Deliverables

- [x] Updated `shared/worksheet.css` and view markup as needed _(Closure: one delimited block at the end of `shared/worksheet.css`; small markup edits in `layout.js`, `app-shell.js`, `lessons-page.js`, `today-page.js`, `review-page.js`, `progress-page.js`, `words-page.js`, `guides-page.js`, `player/core.js`.)_
- [x] Tests still green
- [x] Completion summary _(Closure: `completion-summaries/SPEC-011-completion-summary.md`.)_

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test`.
- **Rollback plan:** revert the branch.

---

## Security

- **Sensitivity:** light.
- **Review depth:** light.

---

## Verification Strategy

- **Claim:** learner views match ADR-0008 without breaking hooks.
- **Check + executor:** `mechanized` — `npm test`.
- **Pass condition:** exit 0.
- **Provenance tag:** `INIT-005/SPEC-011`
