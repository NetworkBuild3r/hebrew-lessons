# SPEC-002 · Frontend: Shell and router spike

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-005
title: "Shell and router spike: one entry, route table, Today and Lessons"
domain: frontend
status: in_progress
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: gated
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-001]
blocks: [SPEC-003, SPEC-007]
```

---

## Description

**Context:** ADR-0007 (SPEC-001) records the SPA. This spike proves it: one signed-in shell document, a hand-built router, the header rendered once, Today and Lessons as views, and one legacy lesson mounted and unmounted.

**Scope (in):**

- Server serves one shell HTML for `/`, `/lessons/`, `/review/`, `/progress/`, `/admin/` (and direct URLs), embedding a minimal signed-in user.
- `shared/router.js`: History API, route table, `mount`/`unmount`, popstate and click interception for same-origin app links.
- Today and Lessons ported as views that mount into `#app-main`.
- One legacy lesson (`my-morning`) tried inside the shell; result recorded (go or fallback).
- Header and nav do not repaint on in-app navigation. Active link uses `aria-current`.
- Operator tries it locally (gate).

**Scope (out):** Review, Progress, Admin ports (SPEC-003/004); restyle; full a11y suite beyond title and no-flash (SPEC-003 expands).

---

## Understood (agent's read of the intent)

- [ ] und-1: Existing `HebrewShell.mount` is adapted so it paints once from the embedded user and does not call `Api.me()` on every route.
- [ ] und-2: Old per-page HTML files for Today and Lessons either redirect into the shell or are retired by the server route. Learners who bookmark `/lessons/` still land on Lessons.
- [ ] und-3: The spike lives in production paths (not only `spikes/`), because the operator asked for the live site first. A feature flag or easy revert is required.

## Acceptance Criteria

- [ ] ac-1 (REQ-001, REQ-003): clicking Lessons from Today does not reload the document; the nav stays painted; no `…` placeholder flash.
- [ ] ac-2 (REQ-002, REQ-006): refreshing `/lessons/` shows Lessons; an unknown path shows a not-found view inside the shell.
- [ ] ac-3 (REQ-008): Today and Lessons behaviour matches the previous pages for the cases covered by `test/app-shell.test.js` and the lessons-related unit tests that still apply.
- [ ] ac-4 (REQ-009): a written spike note records whether `my-morning` mounts and unmounts cleanly, or falls back to a full page.
- [ ] ac-5: operator has clicked through the spike and said go (or named the fallback).

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Where the spike lives | Production paths with revert, not a throwaway folder | resolved | yes — operator "all of it" + "first" |
| aud-2 | Which lesson for the trial | `my-morning` | resolved | yes — shortest published lesson |

> **Readiness** = 100.

---

## Deliverables

- [ ] Shell HTML entry and server routing for app paths
- [ ] `shared/router.js` and view modules for Today and Lessons
- [ ] Spike note: `sdd/initiatives/INIT-005-learner-experience-and-identity/INIT-005-shell-spike-decision.md`
- [ ] Tests covering no full reload on nav click (as far as `node --test` can) and direct URL
- [ ] Completion summary

---

## Technical Requirements (domain-specific)

- [ ] No new npm dependency. No build step.
- [ ] Test hooks (`data-nav`, shell classes) stay unless the same change updates their test.
- [ ] Embedded user: same fields as `/api/me`. Escaped. No session token in the body.
- [ ] Login remains `/login/` as a full page.

---

## Gates & Controls

- **Autonomy level:** `gated`. Operator must try the spike before SPEC-003 starts.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`; operator go or fallback decision recorded.
- **Rollback plan:** revert the branch; restore per-page HTML serving.

---

## Security

- **Sensitivity:** standard. Triggers matched: session embed in HTML, route gating.
- **Security acceptance criteria:**
  - [ ] Unauthenticated requests to app routes still redirect to login.
  - [ ] Embedded payload is not a secret and matches `/api/me` fields.
- **Review depth:** standard · **focus areas:** gate, embed, XSS via embed escape.

---

## Verification Strategy

- **Claim:** Today ↔ Lessons navigation does not reload the document; the header stays; direct URLs work; legacy-lesson fate is recorded.
- **Check + executor:** `mechanized` — `npm test`; `manual` — operator click-through and spike note.
- **Pass condition:** tests exit 0; spike note present; operator decision recorded.
- **Test levels:** unit + existing suite; operator manual for feel.
- **Test data:** none that includes learner names or transcripts.

---

## Integration Points

- Blocked by SPEC-001. Blocks SPEC-003 and SPEC-007.
- **Provenance tag:** `INIT-005/SPEC-002`
