# SPEC-005 · frontend: Five destinations, labels intact

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-014
title: "Five destinations, labels intact"
domain: frontend
status: ready
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-002]
blocks: [SPEC-006, SPEC-007, SPEC-008]
```

## Description

**Context:** `shared/app-shell.js` NAV lists seven learner destinations. Guides and Verbs have no icon (`ICON_NAMES` around line 36). At 390px the dock shows "Progre…". At 360px it also shows "Lesso…" (ASMT-002 `phone-today.png`, `probe-phone360-today.png`). RSCH-007 asks for 3–5 destinations, each with an icon and a visible label. A new learner's Guides and Verbs pages are empty. REQ-005.

**Scope (in):**

- Learner nav is Today, Lessons, Review, Words, Progress. Each has an icon and a label.
- Guides becomes a tab on the Lessons page. `/guides/` still opens the guides content (the route stays; it is not a dead link).
- Verbs becomes a tab on the Review page. `/verbs/` still opens the verbs content.
- Admin and Editor stay admin-only and off the learner dock.
- At 320, 360, and 390px, no learner label is clipped (no ellipsis, `scrollWidth` not greater than `clientWidth`).

**Scope (out):** Lesson sort and search (SPEC-006). Empty-state wording (SPEC-008). The player shell (SPEC-007).

## Understood (agent's read of the intent)

- [ ] und-1: Five is the learner set. Admins may still see Editor and Admin.
- [ ] und-2: Tabs, not deletion. Someone with the old URL still lands on the content.
- [ ] und-3: This spec edits `shared/worksheet.css` only after SPEC-002, in the dock and tab rules, not in the Today grid.

## Acceptance Criteria

- [ ] ac-1: A learner's dock or rail shows exactly five destinations: Today, Lessons, Review, Words, Progress. Each contains an `svg` (or the existing icon sprite) and a full label. — covered: no
- [ ] ac-2: At 320, 360, and 390px, every learner label's `scrollWidth` is less than or equal to its `clientWidth`. — covered: no
- [ ] ac-3: From Lessons, a Guides tab shows the guides list. From Review, a Verbs tab shows the verbs practice. Opening `/guides/` and `/verbs/` directly still shows that same content. — covered: no
- [ ] ac-4: An admin still sees Editor and Admin. A learner does not. — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Where Guides and Verbs go | Guides tab on Lessons, Verbs tab on Review, old URLs kept. The alternative (hide until they have content) was not chosen. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `shared/app-shell.js` — NAV and icons
- [ ] `shared/lessons-page.js` — Guides tab
- [ ] `shared/review-page.js` — Verbs tab
- [ ] `shared/app-routes.js` — only if a route id or tab hook is required; do not remove `/guides/` or `/verbs/`
- [ ] `shared/worksheet.css` — dock and tab layout only
- [ ] Tests: extend the responsive Chrome test for ac-1 and ac-2; a render test for ac-3 and ac-4

## Technical Requirements (domain-specific)

- [ ] Reuse the icon sprite. Add a sprite id only if Guides or Verbs need one inside a tab; the five dock items already have icons.
- [ ] Do not change who the server allows into `/admin/` or `/editor/`.
- [ ] Provenance comment: `INIT-014/SPEC-005`.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** responsive tests and the new nav test pass.
- **Rollback plan:** revert the commit. Old URLs were not removed, so bookmarks survive a revert too.

## Security

- **Sensitivity:** standard — triggers matched: none new. Admin links must remain gated by the existing role check.
- **Security acceptance criteria:**
  - [ ] A learner response for the shell does not include the admin or editor link.
  - [ ] Tab content uses the same visibility rules the standalone Guides and Verbs pages use today.
- **Review depth:** standard · **focus areas:** authz of admin links.

## Verification Strategy

- **Claim:** learners get five fully labeled, icon-bearing destinations, and Guides and Verbs remain reachable as tabs and by their old URLs.
- **Check + executor:** mechanized — `node --test` with the Chrome helper at 320, 360, and 390 for label overflow, plus a DOM test for the tab and the admin link.
- **Pass condition:** ac-2's measurement is `scrollWidth <= clientWidth` for every learner label at all three widths, and ac-4 checks the learner shell and the admin shell separately.

## Integration Points

- SPEC-006 adds sort and search on the lessons page after the Guides tab exists.
- SPEC-008 changes empty-state copy on Review, which will also host the Verbs tab.
- Produces **Change:** the phone bar fits five labeled destinations.
- **Provenance tag:** `INIT-014/SPEC-005`
