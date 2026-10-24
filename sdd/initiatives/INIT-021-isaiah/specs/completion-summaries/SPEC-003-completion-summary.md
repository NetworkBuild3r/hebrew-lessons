# SPEC-003 completion summary

**Initiative:** INIT-021 · **Spec:** SPEC-003 (frontend) · **Status:** complete · **Date:** 2026-10-24

## Change

`/isaiah` lists chapters that have a checked verse. `/isaiah/{chapter}/{verse}` shows the pointed verse large, a tap card for each checked word (gloss, and the root, linked only when that root has a page), then the checked layers. The English sits behind "Show the English". Previous and next step through the book. A verse orb plays a clip when one is already stored. Isaiah stays hidden until something is checked.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestIsaiahCheckIsAnAdminAct` and `TestMyStuffIsolationAndSections`: a draft body is absent from the reader. |
| ac-2 | PASS | `TestIsaiahCheckIsAnAdminAct`: the checked word is on the page and its root is a link. A root with no page is text only (`isaiah.templ`). |
| ac-3 | PASS | `TestIsaiahInBrowser`: 390 and 1440, light and dark, with the pointed line's height and RTL checked, and screenshots. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- No security-review registry is available. Chapter and verse are integers from 1 to 66 and 1 to 200. The page reads checked rows only.
- The verse orb uses a clip that is already stored. This page does not generate audio.
