# SPEC-001 completion summary

**Spec:** SPEC-001 · ADR-0009 learner model and language program  
**Initiative:** INIT-006  
**Written:** 2026-10-09  
**Status:** complete. The operator accepted ADR-0009 as written on 2026-10-09, with aud-1 and aud-3 as proposed. SPEC-002 and SPEC-003 are unblocked and have not started.

## What shipped

- `docs/decisions/ADR-0009-learner-model-and-language-program.md`, status **Accepted** (2026-10-09). Nine decisions: the aggregate profile and its closed never-returned list (with the role-by-route matrix, the audit rows, and the new disclosure sentence), FSRS-6 with pinned parameters, weak-word flags, the 32-key grammar registry and concept mastery, journey stages, the lexicon and personal library, plan language 1.1.0 (five blocks, `recipe`, rules N13 to N19, changes to N1, N2, N7), the daily mix, and a superseded-text table with carried constraints.
- Metadata and changelog notes in ADR-0001, ADR-0002, ADR-0003, and ADR-0004. Each now records that ADR-0009 was Accepted on 2026-10-09 and the change is in force. No decision text in those ADRs was edited.
- INIT-004 amended (not yet approved, so editing was allowed):
  - SPEC-010 renamed and cut to personal-plan assignee checks (4h to 2h, `opus` to `sonnet`, blocked by SPEC-003, blocks SPEC-020).
  - SPEC-011 renamed, switch route removed (5h to 4h). SPEC-012 renamed, `get_learner_snapshot` removed (5h to 4h).
  - SPEC-005, SPEC-006, SPEC-014, SPEC-018, SPEC-019, SPEC-020 updated to match. The disclosure and guide sentences moved to INIT-006 SPEC-017 and SPEC-019.
  - Initiative §1, §3, §4, §5 (88h to 84h), §8, §9, §10, §12, §13, and the rollout row updated. The one remaining `get_learner_snapshot` mention is the "superseded" notes.
- INIT-006 aligned to the ADR as written: SPEC-003, SPEC-006, SPEC-007, SPEC-008, SPEC-009, SPEC-010, SPEC-011, SPEC-016, SPEC-017, SPEC-019, and initiative §2, §9, §13.

## Acceptance criteria

| Id | Result |
|----|--------|
| ac-1 | Pass. Nine decisions, each with options and the chosen one marked. Status Accepted. |
| ac-2 | Pass. Decision 1 has the returned-fields and never-returned tables. Decision 9 has the superseded-text table with line numbers for ADR-0004 and ADR-0002. |
| ac-3 | Pass. FSRS-6, the parameter source by URL, the 21 pinned defaults, the grade mapping, the stage mapping, and the backfill formula are in Decision 2. |
| ac-4 | Pass. ADR-0001 #4 is marked changed. Untimed conjugation with feedback is allowed. Timed root and binyan speed drills stay out. |
| ac-5 | Pass. The files listed above, and nothing else in INIT-004. |

Verification checks: a search of `sdd/initiatives` finds `get_learner_snapshot` in INIT-004 only inside "superseded" notes (initiative §3, SPEC-012, SPEC-018). It also appears in INIT-003's finished initiative document, which is history and was left alone. ADR-0009 has Decision headings 1 to 9. INIT-004 effort sums to 84h and INIT-006 to 93h from the spec metadata. The `human` check is met: the operator accepted ADR-0009 and answered aud-1 and aud-3 as proposed on 2026-10-09.

## Deviations from the spec text

1. **Rate and audit are not "as ADR-0004".** One cap of 60 reads per clock hour is shared by `get_learner_profile` and `check_text_coverage`. Audit rows are new (`learner_profile_read`, `learner_coverage_read`, `library_add`, `lexicon_upsert`, `lexicon_verify`, `ai_planning_switch`). The disclosure sentence is new and names the username and the vowel-points-off share.
2. **Journey `script` is a count only** (below 50 words long-term in `he_en`). The "or a letters-and-vowels unit is complete" clause was dropped, because it would depend on a `recipe` tag that exists only in `1.1.0` plans.
3. **Coverage has no prefix stripping.** SPEC-007 planned a labeled proclitic fallback. ADR-0001 A1 and A2 forbid it, so a prefixed token is unknown until a lexicon `form` entry records it. Coverage under-reports and never over-reports.
4. **Learners never write the lexicon.** SPEC-003 and SPEC-009 let a learner add a lexicon entry. The ADR keeps the learner's own meaning on `user_words.gloss_en` (at most 100 characters). `add_library_words` accepts only keys already in the lexicon.
5. **Only form practice updates grammar concepts.** A word-card grade does not. Added status `emerging` for 1 to 4 attempts.
6. **`letter-intro` has no picture field.** ADR-0003 §5 limits images to introduction words. The teacher's picture-matching sheet is a follow-up that needs an ADR-0003 amendment and a `RSCH-005` check.
7. **A part operation raises `schemaVersion`** from `1.0.0` to `1.1.0` when it adds a `1.1.0` construct. That adds one change to INIT-004's `ops.js`, owned by INIT-006 SPEC-011.
8. **The mix's "reading" section** draws on the published lessons' vocab sentences. The lexicon has no example field.
9. **The mix does not claim every word is known.** A reading sentence may have up to 5% unknown words.
10. **More INIT-004 files changed** than the spec listed: SPEC-005, SPEC-006, SPEC-014, SPEC-020 and initiative §1, §5, §8, §9, §10. SPEC-001's scope and ac-5 were extended to say so.

## Answered at acceptance

- **aud-1:** topic-conversation new words: warn from 9, block above 15, groups of at most 5. Accepted as proposed.
- **aud-3:** journey thresholds: 50, 150, 500, 1500 long-term words. Accepted as proposed.

## Still open

- Before the first learner's switch is turned on, check the data-retention terms of the model provider behind your MCP client. This record did not read them.
- The Hebrew disclosure line is written and approved at INIT-006 SPEC-017.

## Follow-ups

- INIT-006 SPEC-004 builds golden FSRS fixtures from a reference implementation kept outside this repo.
- INIT-005 SPEC-008 still reserves ADR-0008. This record is ADR-0009.
- No code, no tests, and no commit were part of this spec.

## Provenance

`INIT-006/SPEC-001`
