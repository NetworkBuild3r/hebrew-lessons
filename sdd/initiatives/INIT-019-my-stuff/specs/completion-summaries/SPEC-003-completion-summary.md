# SPEC-003 completion summary

**Initiative:** INIT-019 · **Spec:** SPEC-003 (frontend) · **Status:** complete · **Date:** 2026-10-23

## Change

`/me/words` lists words the learner has met, with learning, known, and struggling filters. `/me/verbs` is one table per verb the learner has in learning, known, or solid, and those forms are marked. `/me/mistakes` is that learner's own list, most repeated first, with an explainer link when the pattern has a published note. Explainer pages add "Learners often say" from published common mistakes only.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestMyStuffIsolationAndSections`: learner B's pair is absent, and the more repeated row comes first. |
| ac-2 | PASS | The same test: one `data-known="true"` and one `data-known="false"`. |
| ac-3 | PASS | The same test: the explainer shows the published shared pair and not the personal row or the draft. |
| ac-4 | PASS | `TestMeInBrowser` at 390 and 1440, light and dark. The phone table scrolls inside the card. The page does not. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- "Learning or better" on a verb table is learning, known, or solid. Struggling, regressed, and seen stay off the table and unmarked.
- The known filter on the word bank includes solid.
- The contrast between a wrong form and a right form is the arrow icon, not a text arrow.
