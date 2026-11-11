# SPEC-008 · frontend: Empty states that tell the truth

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-014
title: "Empty states that tell the truth"
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

**Context:** RSCH-010 cites one Nielsen Norman Group article: an empty state should say the status, say what could appear, and link to the action that fills it. The Verbs page already does this (`shared/verbs-page.js` around line 47). The others do not. REQ-008.

- Review, for a learner with no reviews, says "All caught up!" above zero counts and two blank filters (`shared/review-page.js`).
- Words leads with nine filter controls, then "No words yet" (`shared/words-page.js`).
- Guides is one line, "(No published guides.)", with no link (`shared/guides-page.js` around 94–97).
- Progress for a new learner is about 3,126px tall, mostly "unseen" rows under "What your AI sees" (`shared/progress-page.js` around 614–640). That section is a real transparency feature (INIT-006/SPEC-017). It stays. It starts collapsed.

**Scope (in):** the empty and new-learner states of those four pages. Copy in both languages. Reuse the Verbs wording pattern. Do not add CSS unless a reused class cannot do the job; if CSS is required, add it only after reading SPEC-007's rules so the two edits do not share a selector.

**Scope (out):** The Verbs empty state (already good). The Verbs tab placement (SPEC-005). Today's first-run card (already good).

## Understood (agent's read of the intent)

- [ ] und-1: "All caught up" is for someone who has reviewed everything, not for someone who has never learned a word.
- [ ] und-2: Filters appear when there is something to filter.
- [ ] und-3: "What your AI sees" remains available. It is not the first thing a new learner scrolls through.

## Acceptance Criteria

- [ ] ac-1: A learner with no review items sees a state that says there is nothing to review yet, says words appear after a lesson, and links to Lessons. The heading is not "All caught up". The filter row is not shown. — covered: no
- [ ] ac-2: A learner who has finished the day's reviews still sees "All caught up" (or the current done heading) and a link back to Today. — covered: no
- [ ] ac-3: Words, with an empty list, shows the empty message and the add-a-word form before any filter control. Filters are collapsed or absent until at least one word exists. — covered: no
- [ ] ac-4: Guides, when empty, states the status, says what a guide is, and links to Lessons. — covered: no
- [ ] ac-5: Progress renders "What your AI sees" inside a closed disclosure. "unseen" rows are not in the initial paint for a learner with no reviews. Opening the disclosure shows them. — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Collapsed means | A native `<details>` closed by default, or the existing disclosure pattern in `progress-page.js`. No new widget library. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `shared/review-page.js`
- [ ] `shared/words-page.js`
- [ ] `shared/guides-page.js`
- [ ] `shared/progress-page.js`
- [ ] Tests: a node render test per page for the empty learner and, for Review, the caught-up learner

## Technical Requirements (domain-specific)

- [ ] Follow `verbs-page.js` structure: status sentence, explanation, link.
- [ ] Both languages of each new sentence ship together.
- [ ] Provenance comment: `INIT-014/SPEC-008`.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** the new render tests pass.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** standard — triggers matched: none. The AI profile disclosure stays. Do not add new data to it.
- **Security acceptance criteria:**
  - [ ] The collapsed profile still contains only what `GET /api/me/profile` already returns. This spec changes presentation, not the payload.
- **Review depth:** quick · **focus areas:** no new personal data on screen.

## Verification Strategy

- **Claim:** empty Review, Words, and Guides tell the truth and link onward, and the AI profile starts closed.
- **Check + executor:** mechanized — `node --test` on the four render tests.
- **Pass condition:** each test asserts the heading text, the link href, and (ac-5) that the disclosure is closed and the unseen rows are not visible until opened. ac-2 asserts the caught-up heading is still used when the learner has reviewed everything due.

## Integration Points

- Review also hosts the Verbs tab from SPEC-005. The empty state sits in the review pane, not in place of the tab.
- Produces **Change:** a new learner is not told they are caught up, and Progress is no longer a wall of unseen rows.
- **Provenance tag:** `INIT-014/SPEC-008`
