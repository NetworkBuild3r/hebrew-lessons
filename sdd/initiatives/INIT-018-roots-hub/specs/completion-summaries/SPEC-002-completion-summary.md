# SPEC-002 completion summary

**Initiative:** INIT-018 · **Spec:** SPEC-002 (frontend) · **Status:** complete · **Date:** 2026-10-23

## Change

`/roots` lists the roots that have a page. `/roots/{root}` shows the words as cards, the lessons, and the verses. A root that is not visible is 404, and so is a path that is not Hebrew letters, maqaf, and shin or sin dots. A word card links its root only when that page exists. Verse lines are text until the Isaiah reader exists.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestRootsHubAndCardLink`: the index matches the visible root, and the lonely root and a bad path are 404. |
| ac-2 | PASS | The same test: the card links `/roots/` plus the visible root, and the lonely root has no link. |
| ac-3 | PASS | `TestLearnRootsInBrowser` at 390 and 1440, light and dark. Hebrew runs are `bdi` with `lang="he"` and `dir="rtl"`. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- The page CSS is `learn.css`. The card markup stays in `player.templ`.
- Roots is not in the dock. That navigation is INIT-019.
- Verse lines do not link out. That link is INIT-021.
