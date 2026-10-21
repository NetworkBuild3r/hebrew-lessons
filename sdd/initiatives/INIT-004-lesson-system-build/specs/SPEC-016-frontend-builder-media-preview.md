# SPEC-016 · Frontend: Builder media library, preview, and validation

## Metadata

```yaml
spec_id: SPEC-016
initiative_id: INIT-004
title: "Builder media library, live preview, and validation panel"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-011, SPEC-015]
blocks: [SPEC-020]
```

---

## Description

**Context:** the rest of the builder. Pictures and the research-rule panel sit on the outline from SPEC-015. Generation and upload go through SPEC-011. The page does not call OpenAI.

**Scope (in):**

- Media library: list pending and approved images the admin can fetch, upload a file to `POST /api/media`, run a dry-run against `GET /api/admin/images/image-plan`, and generate only after the admin confirms the shown dollar estimate (`confirm: true`).
- Approval control calls `POST /api/admin/media/:id/approval` and requires alt text in the form. The form refuses to submit an empty alt.
- Attaching an image sets `mediaId` and `alt` on an introduction word through `set_field`. Other block types have no image control.
- Validation panel shows block, warn, and inform results from the validate response the save and publish routes return. It labels N1 and the other ids. It does not recompute the caps in the browser.
- Live preview re-renders the SPEC-014 function for the current block after a successful save.

**Scope (out):** a new operation; learner-facing upload; printing the API key.

---

## Understood (agent's read of the intent)

- [x] und-1: The dollar figure on screen is the dry-run response, not a number typed into the frontend.
- [x] und-2: The browser never sees `OPENAI_API_KEY`.

## Acceptance Criteria

- [x] ac-1 (REQ-010): choosing a picture on a `graded-story` block is not offered. On an introduction word, save sends `set_field` for `mediaId` and `alt`.
- [x] ac-2 (REQ-007): the generate button stays disabled until a dry-run response is shown and the admin checks confirm. The test's fake server records no generate call before that.
- [x] ac-3 (REQ-003): a publish response that includes an N1 warning shows the id `N1` in the panel. A block failure shows that the snapshot was not published (the route's refusal).
- [x] ac-4 (REQ-006): the approval button is disabled when the alt field is empty.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Who confirms generation | The admin in this UI, and the MCP in SPEC-012, both with `confirm: true`. | resolved | yes — ADR-0004 §7 |

> **Readiness** = 100.

---

## Deliverables

- [x] Media, preview, and validation regions on the builder view
- [x] Tests: `test/plan-builder-media.test.js`

---

## Technical Requirements (domain-specific)

- [x] Upload uses the authenticated admin fetch, not a public form target.
- [x] Preview HTML is the renderer output, assigned in a way that does not eval plan text.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-builder-media.test.js` and `npm test`.
- **Rollback plan:** hide these panels. The outline from SPEC-015 still edits text.

---

## Security

- **Sensitivity:** standard. Triggers matched: admin upload and generation UI. The key itself is server-side (`required` on SPEC-009 and SPEC-011).
- **Security acceptance criteria:**
  - [x] No client code reads or displays an API key.
  - [x] Generate is a confirmed second request.
- **Review depth:** standard · **focus areas:** CSRF, confirm step, no key in the bundle.

---

## Verification Strategy

- **Claim:** the builder can attach an approved picture only on an introduction word, and it cannot generate without a confirmed dry-run.
- **Check + executor:** `mechanized` — `node --test test/plan-builder-media.test.js`.
- **Pass condition:** exit 0; the generate request is absent until confirm; `N1` is visible in the warning fixture.
- **Test levels:** admin-view tests with a fake fetch. No coverage tool is configured; the bar is this file plus `npm test`.

---

## Integration Points

- Last builder spec. SPEC-020 may cut a learner over only after this page and the renderers exist.
- Produces **Change:** builder media and validation UI.
- **Provenance tag:** `INIT-004/SPEC-016`
