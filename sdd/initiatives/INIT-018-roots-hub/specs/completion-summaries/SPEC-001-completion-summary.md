# SPEC-001 completion summary

**Initiative:** INIT-018 · **Spec:** SPEC-001 (service) · **Status:** complete · **Date:** 2026-10-23

## Change

`roots.Family` builds one root for one learner: words, state, lessons they can see, and up to ten verses with Isaiah first. `List` and `Visible` include a root only when it has two words, or one word and a verse. Verb forms stay empty until they are loaded.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestFamilyVisibilityAndScope`: one word and no verse is hidden; two words are listed. |
| ac-2 | PASS | The same test: the learner sees the shared lesson and their own, and not another learner's. |
| ac-3 | PASS | `TestVersesIsaiahFirstAndCapped` returns ten verses, Isaiah first, from one scan of the existing verse table. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- The result type is `Page`. A function and a type cannot share the name `Family` in Go. The function is still `Family`.
- `Forms` is always empty. Verb forms are INIT-019.
