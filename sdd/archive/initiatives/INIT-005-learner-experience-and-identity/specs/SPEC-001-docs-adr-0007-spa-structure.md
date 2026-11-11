# SPEC-001 · Docs: ADR-0007 single-page app structure

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-005
title: "ADR-0007: single-page app structure"
domain: docs
status: complete
primary_prompt: .claude/agents/principal-architect/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 3 hours
blocked_by: []
blocks: [SPEC-002]
```

---

## Description

**Context:** The operator approved a hand-built single-page app so the header stays and only content swaps. ADR-0005 chose Node and no framework, but left pages as one HTML file each. That page model is rejected.

**Scope (in):**

- ADR-0007 recording: one shell document, History API router, view `mount`/`unmount` contract, server route handling, first-paint user embedding, 401 → login, accessibility behaviours on route change, legacy-lesson fallback, what of ADR-0005 stays.
- A "Superseded in part by ADR-0007" note and changelog row on ADR-0005.
- No application code in this spec.

**Scope (out):** writing the router (SPEC-002); restyle (SPEC-008 onward); INIT-004 edits (called out as follow-ups only).

---

## Understood (agent's read of the intent)

- [x] und-1: No new npm dependency. No build step. Node, `node:sqlite`, in-process MCP stay.
- [x] und-2: Login stays a separate full page. The shell is for signed-in screens only.
- [x] und-3: Legacy lessons may open as full pages if the SPEC-002 spike cannot mount them; the ADR records that as the decided fallback, not as a second option to reopen later.

## Acceptance Criteria

- [x] ac-1 (REQ-001, REQ-002): ADR-0007 states one shell document, a hand-built History API router, and a view contract of `mount(container, ctx)` and `unmount()`.
- [x] ac-2 (REQ-003, REQ-007): ADR-0007 states how the signed-in user is known on first paint (server-embedded from the session) and that a 401 sends the learner to `/login/` with a safe `next`.
- [x] ac-3 (REQ-004, REQ-005, REQ-006): ADR-0007 states title, focus, scroll, announce, reduced-motion transitions, direct-URL and unknown-route behaviour.
- [x] ac-4 (REQ-009): ADR-0007 names the legacy-lesson rule and the SPEC-002 gate that confirms or falls back.
- [x] ac-5: ADR-0005 gains a "Superseded in part" note naming the page model only. Stack choice (Option A) is not reopened.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Router library | Hand-written. No package. | resolved | yes — operator A4 default |
| aud-2 | First-paint user | Server embeds a minimal `/api/me`-shaped object into the shell. | resolved | yes — initiative A5 |

> **Readiness** = 100.

---

## Deliverables

- [x] `docs/decisions/ADR-0007-single-page-app-structure.md`
- [x] ADR-0005 metadata and changelog updated
- [x] Completion summary

---

## Technical Requirements (domain-specific)

- [ ] ADR number claimed by scanning `docs/decisions/ADR-*.md`. Next free is 0007.
- [ ] Cite INIT-005 REQ-001 through REQ-009.
- [ ] No secrets, learner names, or transcript text.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** files exist; ADR-0005 note present; planned ADR number unique.
- **Rollback plan:** delete ADR-0007 and revert the ADR-0005 note.

---

## Security

- **Sensitivity:** light. Triggers matched: auth-adjacent (401, session embed). No new secret surface.
- **Security acceptance criteria:**
  - [ ] Embedded user object is the same fields `/api/me` already returns; no password hash, no session token in the HTML body beyond what cookies already carry.
  - [ ] `next` path rules cite the existing `safeNextPath`.
- **Review depth:** light · **focus areas:** what is embedded in HTML.

---

## Verification Strategy

- **Claim:** ADR-0007 records the SPA decision and ADR-0005 is marked superseded in part for the page model only.
- **Check + executor:** `manual` — file presence, ADR number uniqueness, ADR-0005 note, REQ coverage.
- **Pass condition:** all five acceptance criteria hold on a read of the files.
- **Test levels:** documentation. No runtime test.
- **Test data:** none.

---

## Integration Points

- Blocks SPEC-002.
- INIT-004 SPEC-013–016 should be re-read against ADR-0007 before they start (not edited in this spec).
- **Provenance tag:** `INIT-005/SPEC-001`
