# SPEC-006 · Frontend: Logo, icons, illustration style guide

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-005
title: "Identity: logo, wordmark, favicon, icon set, illustration guide"
domain: frontend
status: ready
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: gated
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 5 hours
blocked_by: []
blocks: [SPEC-007, SPEC-010]
```

---

## Description

**Context:** The site has no logo and no icon set. The operator asked for icons and logos as part of making the UI feel better.

**Scope (in):** three logo options in SVG; a wordmark using the self-hosted Rubik/Hebrew type; favicon and apple-touch set; ~20 icons on one grid (play, replay, next, back, check, wrong, retry, home, lessons, review, progress, settings, theme, mute, lock, done, help, admin, upload, image); one-page illustration style guide (palette, line weight, background, prompt sentence for INIT-004 image generation). Operator picks the logo.

**Scope (out):** wiring icons into the shell (SPEC-010); image generation (INIT-004).

---

## Understood

- [ ] und-1: All assets are self-hosted. No external font or icon CDN.
- [ ] und-2: No jokes or mascot characters (operator). The logo may be abstract or typographic.
- [ ] und-3: Icon-only controls need accessible names in the usage guide; primary actions keep a visible word.

## Acceptance Criteria

- [ ] ac-1 (REQ-010): three logo options, wordmark, favicon set, ~20 icons, and a usage guide exist under a dedicated assets path.
- [ ] ac-2 (REQ-011): illustration style guide includes the sentence that INIT-004 SPEC-009 will append to every image-generation prompt.
- [ ] ac-3 (REQ-018): every asset is a local file; no remote URL in the guide's examples.
- [ ] ac-4: operator has picked one logo (or asked for a revision round).

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Brand name | Keep דרך עברית / existing product name unless operator says otherwise | resolved | yes — default; Q1 open |
| aud-2 | Asset path | `shared/brand/` for logo/icons; guide at `docs/design/illustration-style.md` | resolved | yes — planning default |

> **Readiness** = 100.

---

## Deliverables

- [ ] `shared/brand/` assets
- [ ] `docs/design/illustration-style.md`
- [ ] Usage guide for icons
- [ ] Completion summary with the operator's pick recorded

---

## Gates & Controls

- **Autonomy level:** `gated`. Operator picks the logo before SPEC-007 / SPEC-010 use it in production chrome.
- **Blocking checks:** files present; operator pick recorded.
- **Rollback plan:** delete `shared/brand/` and the guide.

---

## Security

- **Sensitivity:** light. SVG only; no scripts inside SVG.
- **Security acceptance criteria:**
  - [ ] SVG files contain no `<script>`, no event handlers, no external references.
- **Review depth:** light.

---

## Verification Strategy

- **Claim:** identity assets and guides exist; operator pick recorded.
- **Check + executor:** `manual` — file presence and pick; `mechanized` — optional grep that brand SVGs have no `<script`.
- **Pass condition:** ac-1 through ac-4.
- **Provenance tag:** `INIT-005/SPEC-006`
