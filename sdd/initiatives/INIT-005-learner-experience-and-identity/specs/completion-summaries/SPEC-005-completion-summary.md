# SPEC-005 completion summary

**Spec:** SPEC-005 · RSCH-008 (planned as RSCH-007) motion, sound, icons and picture style in L2 practice  
**Initiative:** INIT-005  
**Completed:** 2026-10-12  
**Status:** in_review (written; awaiting the operator's read of the report and the product-choice rows)  
**Provenance:** `INIT-005/SPEC-005`

## What shipped

- `sdd/research/RSCH-008-motion-sound-icons/RSCH-008-motion-sound-icons-report.md` — a `prior_art_review` of four questions with High / Moderate / Low / product choice / no evidence labels, an effects table (allowed, product choice, banned), a reconciliation with ADR-0010 and the shipped feedback kit, 33 numbered references with URLs, and appendices for access scope, scope, and method. Overall confidence: Low.
- `sdd/research/README.md` — index row for RSCH-008.
- `specs/SPEC-005-docs-rsch-007-motion-sound-icons.md` — front matter set to `in_review`, acceptance criteria and deliverables ticked, assumption aud-1 amended for the number change.
- No code, ADR, or other spec was edited. Nothing was committed.

## Findings by question

| Question | Answer | Grade |
|---|---|---|
| 1. Animated or sound feedback in L2 vocabulary practice | No evidence of benefit. Irrelevant extras lowered learning in general lessons (seductive-details meta-analysis, mean −0.33; added music and sound hurt retention and transfer). One large study found feedback sounds had no effect on performance or enjoyment. The correct form shown on the same turn has the evidence. | no evidence (benefit); Moderate (general cost); Low (cost on this screen) |
| 2. Icon plus label versus icon alone | A visible label beside the icon is better for first-time users. Icon-only catches up after practice. No test covers labels in the language being learned. | Moderate; no evidence for target-language-only labels |
| 3. Picture style (flat, realistic, cartoon) | No direct comparison found. Line drawings did at least as well as colour pictures in one 238-learner experiment. Simple pictures hindered abstract words in one study. | no evidence (direct); Low (adapted) |
| 4. A points-free "finished" moment and return to study | No trial found. Lab studies show endings shape how a session is remembered and chosen again. Closure for its own sake is not supported. The only large return trial tested leaderboards and prizes, which are excluded. | no evidence (return); Moderate (ending mechanism, lab-scale) |

## Effects table (summary)

- **Allowed (SPEC-009 may build):** correct form on a miss with one retry; static success mark; audio loading and playing state (with a cap); press state of 150 ms or less; step or route transition of 200 ms or less; a visible word beside navigation and primary-action icons; a calm static completion summary.
- **Product choice (off by default, toggle, or operator to confirm):** UI sounds; the low "miss" tone (recommend drop); the wrong-answer shake (recommend static highlight); hover lift; warm colour in cards and pictures; icon-only universal controls; one consistent picture style; word-meaning sound effects (deferred).
- **Banned:** points, streaks, levels, badges, leaderboards; mascot and jokes; confetti and celebration motion; background music or load-time sound; any sound over Hebrew audio; self-running motion over 5 s without a pause; motion that ignores reduced motion; an effect in place of the correct form; icon-only primary navigation; an unchecked photoreal AI picture.

## Reconciliation with ADR-0010 (flags for SPEC-009)

1. No visible sound control in committed code. `[data-sfx-toggle]` is queried in `feedback.js` and styled in CSS, but nothing rendered it when researched, so REQ-014's mute control was unmet. A later check found an uncommitted `#btn-sfx` button in `shared/player/core.js` (player header only) and an untracked `test/feedback-kit.test.js`, which look like SPEC-009 work in progress.
2. The 220 Hz "miss" chime is downgraded; the one study on sound valence says to avoid low-valence sound.
3. `match-wrong-shake` is in the CSS but not in ADR-0010's list, and nothing supports it.
4. The `.is-loading` pulse is unbounded, so a stalled fetch passes the 5-second rule.
5. ADR-0010 and the INIT-007/SPEC-002 summary say "ducks and mutes"; the code only suppresses new chimes while speech is active.
6. The `feedback.js` header cites "RSCH-007" for sound rules; those rules are in RSCH-008.

## Acceptance criteria

| Id | Result |
|----|--------|
| ac-1 | Pass — report §1.1–1.4, each claim graded; Q3 and Q4 are mostly recorded absences |
| ac-2 | Pass — effects table with allowed, product choice, and banned classes, plus the ADR-0010 reconciliation |
| ac-3 | Pass — `sdd/research/README.md` row RSCH-008 |
| und-1 | Pass — "found nothing" is stated where nothing direct was found |
| und-2 | Pass — points and streaks not reopened; one counter-signal recorded |

## Caveats

- Verification was the worker's own self-check. No independent citation pass was run. Several sources were abstract-only (listed in report §3 and Appendix C).
- 25 web calls were used, above the 15–20 guideline in RSCH-006, because Q3 and Q4 needed extra passes to conclude that nothing direct exists.
- The access-gate binding-verify block was not re-run. The report records the scope as `NARROW` (public sources only).

## Follow-ups

- **Number collision.** SPEC-005 (now fixed), SPEC-007, SPEC-008, SPEC-009 and the INIT-005 initiative text call this research "RSCH-007". RSCH-007 is the new-user ease report. Those four files need "RSCH-008" for this research. Not edited here.
- SPEC-009 should fix the six reconciliation items above.
- SPEC-006 and SPEC-010 should keep the English gloss on primary navigation until a test shows Hebrew-only labels work for a novice.
- The operator confirms the product-choice rows before SPEC-007 builds prototypes.
- SPEC-013 should check the speed-quiz countdown against the 5-second rule. It was not assessed here.
