# SPEC-017 · Frontend: "What your AI sees" panel and admin AI-planning switch

## Metadata

```yaml
spec_id: SPEC-017
initiative_id: INIT-006
title: "Progress shows the learner's profile exactly as the AI would get it; admin users view toggles the switch"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 4 hours
blocked_by: [SPEC-010]
blocks: [SPEC-020]
```

---

## Description

**Context:** Progress is `shared/progress-page.js`; the admin users view is `shared/admin/users.js` inside `shared/admin/admin-shell.js`. ADR-0009 §1 carries the disclosure sentence. The learner seeing the same object is REQ-008.

**Scope (in):**

- Progress: a "What your AI sees" section from `GET /api/me/profile`: journey stage and counts; grammar concepts as a list with status and accuracy (grouped by binyan, tense, person cells, root class with traffic-light colour); weak and very weak words (links to Words filtered); a "Show raw" toggle that prints the JSON exactly as returned (pretty-printed, `textContent`). The switch state shown as "AI planning: on/off — an admin changes this", with the ADR-0009 disclosure sentence.
- Admin users view: an "AI planning" toggle per user calling `PATCH /api/admin/users/:id/ai-planning`, with a confirm dialog quoting the disclosure sentence; shows the new state from the response, not optimistically.
- Comment at top of the Progress section: this panel must render the API object, not a recomputed copy, so it never drifts from what the AI gets.
- The disclosure sentence on the sign-in page (`login/index.html`, `shared/login.js`) and in the account menu (`shared/app-shell.js`), beside the existing admin-visibility sentence and not replacing it. Same bilingual form as that sentence (Hebrew, English in brackets). The Hebrew line is a draft until the operator approves it. This work moved here from INIT-004 SPEC-014 on 2026-10-09 (INIT-006/SPEC-001).

**Scope (out):** the profile contract (SPEC-007).

---

## Understood (agent's read of the intent)

- [x] und-1: For a single learner who is also the admin, this is how he checks what his AI knows before turning it on.

## Acceptance Criteria

- [x] ac-1 (REQ-008): the raw JSON shown equals the `/api/me/profile` response byte for byte (smoke compares).
- [x] ac-2 (REQ-008): the disclosure sentence from ADR-0009 appears on Progress, in the admin confirm, on the sign-in page, and in the account menu. The sign-in page and the account menu still contain the existing admin-visibility sentence, unchanged.
- [x] ac-3: a non-admin does not see the toggle; the route still returns 403 if called directly (covered in SPEC-010; smoke checks the UI).
- [x] ac-4: admin toggle changes state only after a 200 response; a failure leaves the old state and shows an error.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Learner self-toggle | No. ADR-0004 §3 option 3 rejected it; admins create accounts. | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- [x] Changes to `shared/progress-page.js`, `shared/admin/users.js`, `shared/login.js`, `login/index.html`, `shared/app-shell.js`, styles
- [x] Tests: `test/chrome-ai-profile-smoke.js`; update `test/admin-users-view.test.js`

---

## Technical Requirements (domain-specific)

- [x] No `innerHTML` with profile data.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** smoke, unit test, `npm test`.
- **Rollback plan:** remove the section and the toggle.

---

## Security

- **Sensitivity:** required. Triggers matched: UI for the switch that releases learner data.
- **Security acceptance criteria:**
  - [x] Toggle requires an explicit confirm with the disclosure text.
  - [x] UI hides the toggle for non-admins; server remains the enforcement.
- **Review depth:** standard · **focus areas:** confirm flow, admin gating.

---

## Verification Strategy

- **Claim:** the learner sees exactly what the AI would see, and only an admin can switch it, with disclosure.
- **Check + executor:** `mechanized` — `node test/chrome-ai-profile-smoke.js`; `node --test test/admin-users-view.test.js`.
- **Pass condition:** exit 0; byte comparison passes.
- **Test levels:** browser smoke, unit.

---

## Integration Points

- Consumes SPEC-010 routes.
- Produces **Change:** profile panel and switch UI.
- **Provenance tag:** `INIT-006/SPEC-017`
