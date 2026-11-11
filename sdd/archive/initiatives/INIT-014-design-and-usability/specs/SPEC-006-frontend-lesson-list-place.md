# SPEC-006 · frontend: Lessons list keeps the learner's place

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-014
title: "Lessons list keeps the learner's place"
domain: frontend
status: ready
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-005]
blocks: [SPEC-009]
```

## Description

**Context:** `shared/lessons-page.js` `sortRows` (around 223–228) sorts by status rank, then index. `shared/player/utils.js` gives `new` rank 1 (around 266–269) and `in_progress` rank 2 (around 355–357), so a lesson in progress sorts after every new lesson. ASMT-002's `probe-desktop1440-lessons.png` shows three "new" cards, then "in progress" on the next row. The page has no `aria-current`, no "Next" marker, and no search (RSCH-010, WCAG 2.4.5 and 2.4.8). REQ-006.

**Scope (in):** `shared/lessons-page.js` and the status rank in `shared/player/utils.js` if the rank is the sort key.

- Order: due, then in progress, then new, then done.
- The in-progress card carries `aria-current="true"` (or `aria-current="step"` if that better matches the control).
- The next new lesson is marked with a visible "Next" label in both languages.
- A search box filters the visible cards by the Hebrew title and the English gloss. An empty search shows the full list.

**Scope (out):** The Guides tab (SPEC-005). Changing lesson content. A server-side search API.

## Understood (agent's read of the intent)

- [ ] und-1: The learner should see the lesson they are in without scrolling past new ones.
- [ ] und-2: Search is client-side over the cards already loaded. No new endpoint.
- [ ] und-3: Due stays first. This spec must not bury a due lesson under an in-progress one.

## Acceptance Criteria

- [ ] ac-1: Given one in-progress lesson and two new lessons, the in-progress card is the first card. — covered: no
- [ ] ac-2: That card has `aria-current` set. The following new card shows a "Next" label. — covered: no
- [ ] ac-3: Typing a substring of one lesson's English gloss hides the other cards. Clearing the box shows them again. — covered: no
- [ ] ac-4: A due lesson, when present, is ordered before the in-progress lesson. — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Search matching | Case-insensitive substring on the Hebrew title and the English gloss. No fuzzy match. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `shared/lessons-page.js`
- [ ] `shared/player/utils.js` — only if the rank values are the sort key this spec changes
- [ ] Tests: a node test for the four acceptance criteria using a fixture catalog

## Technical Requirements (domain-specific)

- [ ] Do not remove the level filter SPEC-005's page already has.
- [ ] The search input has a visible label, not placeholder-only.
- [ ] Provenance comment: `INIT-014/SPEC-006`.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** the new lessons-list test passes.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** standard — triggers matched: none. Search filters DOM already sent to this learner.
- **Security acceptance criteria:**
  - [ ] The search string is not interpolated into HTML. Filtering uses text comparison, not `innerHTML`.
- **Review depth:** quick · **focus areas:** output encoding.

## Verification Strategy

- **Claim:** in-progress sorts first after any due lesson, is marked current, the next new lesson is labeled, and search filters by title.
- **Check + executor:** mechanized — `node --test` on the lessons-list test with a fixture of four lessons (due, in progress, new, done).
- **Pass condition:** the test asserts DOM order, the `aria-current` attribute, the Next label, and the filtered count for a known substring.

## Integration Points

- Depends on SPEC-005 having added the Guides tab, so this spec edits the lessons page only after that tab exists.
- Produces **Change:** the learner can see where they are and find a lesson by name.
- **Provenance tag:** `INIT-014/SPEC-006`
