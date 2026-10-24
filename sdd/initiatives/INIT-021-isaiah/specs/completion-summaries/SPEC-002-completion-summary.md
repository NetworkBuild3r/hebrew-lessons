# SPEC-002 completion summary

**Initiative:** INIT-021 · **Spec:** SPEC-002 (api) · **Status:** complete · **Date:** 2026-10-24

## Change

`upsert_verse_notes` drafts notes and word glosses for one Isaiah verse. A `status` of checked in the input is ignored. `get_verse` returns the pointed text and the English to any token, and drafts only to an admin. Checking is a button on `/admin/isaiah/{chapter}/{verse}` for a signed-in admin, with the session CSRF check, and it records that person's user id.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestVerseNotesStayDraftAndLearnersSeeCheckedOnly`: a note sent as checked is stored as draft. |
| ac-2 | PASS | The same test lists tools and finds no `check_verse`. `TestIsaiahCheckIsAnAdminAct`: a learner's post is forbidden, and an admin post records `checked_by`. |
| ac-3 | PASS | The same MCP test: a learner's `get_verse` omits the draft and includes the note after it is checked. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- No security-review registry is available. The check route is behind the session middleware, which requires CSRF on every post, and the handler refuses a non-admin. An MCP token has no session, so it cannot press the button. Bodies go through the restricted markdown parser before anything is written.
