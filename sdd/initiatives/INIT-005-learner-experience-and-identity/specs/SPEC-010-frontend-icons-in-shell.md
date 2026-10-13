# SPEC-010 · Frontend: Icons, logo and favicon in the shell

## Metadata

```yaml
spec_id: SPEC-010
initiative_id: INIT-005
title: "Icon sprite, logo and favicon in the shell and every view"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-006, SPEC-008]
blocks: [SPEC-011, SPEC-012]
```

---

## Description

**Context:** SPEC-006 produced assets; SPEC-008 locked the look. Wire the picked logo, favicon and icon sprite into the shell and views per the usage guide.

**Scope (in):** self-hosted logo in the shell; favicon links on the shell document; SVG sprite used by name; replace text-only controls where the guide says so; icon-only controls get accessible names; primary actions keep a visible word.

**Scope (out):** full page restyle colours (SPEC-011).

---

## Understood

- [x] und-1: No external icon CDN. _(`test/icons-in-shell.test.js` scans `shared/` and `login/` for external hosts and icon fonts.)_
- [x] und-2: Test hooks stay unless updated in the same change. _(Every ADR-0006 `data-*` hook and class kept; `test/player-ui-a11y.test.js` still passes. Tests changed only where this spec changed behaviour: see the completion summary.)_

## Acceptance Criteria

- [x] ac-1 (REQ-010, REQ-018): logo and favicon appear on signed-in screens; assets are local. _(Mark A in the shell header, favicon/touch/manifest links in `app.html`, `login/index.html`, `shared/plan/index.html`, and added by the shell script to lesson-player pages; `/shared/brand/` is public. Checked in headless Chrome at 360 and 1280 px.)_
- [x] ac-2 (REQ-010): every icon-only control has an accessible name; primary actions keep visible text. _(Hebrew-only `aria-label` on speak, record, play-recording, gloss-audio, speak-built and admin-speak buttons; nav, Today buttons, retry, sound toggle keep their visible words. Enforced by a source pattern test over `<button>` + icon only; not a full runtime accessibility audit.)_
- [x] ac-3: SVG contains no script or external reference (test or grep). _(`test/brand-assets.test.js` for `shared/brand/`; `test/icons-in-shell.test.js` for `/icon.svg`, same-origin `<use>` and no external hosts.)_

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Which logo | The SPEC-006 operator pick | resolved | yes — at SPEC-006 gate |

> **Readiness** = 100.

---

## Deliverables

- [x] Shell and view wiring
- [x] Sprite include
- [x] Tests / grep for safe SVG
- [x] Completion summary (`specs/completion-summaries/SPEC-010-completion-summary.md`)

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test`.
- **Rollback plan:** revert the branch.

---

## Security

- **Sensitivity:** light. SVG safety.
- **Review depth:** light.

---

## Verification Strategy

- **Claim:** brand assets are in the shell safely.
- **Check + executor:** `mechanized` — `npm test`.
- **Pass condition:** exit 0.
- **Provenance tag:** `INIT-005/SPEC-010`
