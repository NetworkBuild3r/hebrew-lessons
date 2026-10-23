# SPEC-003 completion summary

**Initiative:** INIT-017 · **Spec:** SPEC-003 (frontend) · **Status:** complete · **Date:** 2026-10-23

## Change

`/learn` lists published explainers in path order. `/learn/{slug}` renders one, and a draft or unknown slug is 404. The first step of a lesson whose pattern has a published explainer links "How this works"; later steps do not. The link loads the note into a dialog. Five original starter notes are inserted when the explainer table is empty.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestLearnPageAndFirstStepLink`: the published note renders, and a draft and an unknown slug are 404. |
| ac-2 | PASS | The same test: the first step links the note, and the next step does not. |
| ac-3 | PASS | Headings, bold, italic, a list, a Hebrew run with `lang="he" dir="rtl"`, and an allow-listed link render. An ampersand is escaped. `Rich` drops a link that fails the allow-list at render time. |
| ac-4 | PASS | `TestLearnRootsInBrowser` at 390 and 1440, light and dark. The sheet opens from the keyboard and closes with Escape. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- No security-review registry is available. templ escapes text. `Sanitize` re-checks every link when the page renders.
- The handlers are in `internal/web/learn_page.go`. `internal/web/learn.go` is already the study server.
- The five starters are original and are recorded in `docs/operations/class-data-through-mcp.md`. The class backfill is still not started.
- Learn is not in the dock. That navigation is INIT-019.
