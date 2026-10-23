# SPEC-004 completion summary

**Initiative:** INIT-019 · **Spec:** SPEC-004 (frontend) · **Status:** complete · **Date:** 2026-10-23

## Change

The dock stays Today, Lessons, Review, Progress, and My stuff. Learn, Roots, and Isaiah are extra links in the page and, from 1024px, in the rail. Each one appears only when that section has something to show: a published explainer, a visible root, or a checked verse note. My stuff links to words, verbs, mistakes, and You can now say, and the account controls stay on `/me`.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestMyStuffIsolationAndSections`: Today has no section links until an explainer, a root, and a checked note exist, and then all three are there. The dock stays five. |
| ac-2 | PASS | `TestShellInBrowser` from 320 to 390: five full labels, each on one line and inside its link. "My stuff" is the fifth. |
| ac-3 | PASS | The same shell test: Today, Lessons, Review, Progress, and My stuff stay one tap. Learn is a jump on Today once an explainer exists (`TestMeInBrowser`). |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- Isaiah is a placeholder ("Checked notes will open here.") until INIT-021. `verse_notes` only stores a ref and a status so the link can appear.
- The phone dock does not grow. Learn, Roots, and Isaiah are the section jump and the desktop rail, so the existing five-link checks still hold.
- Root pages fill their verb-form table when rows exist. Until then the table stays empty.
