# ADR-0025: Practice that teaches

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-11-03 |
| **Authored by** | INIT-029 SPEC-001 |
| **Initiative** | INIT-029 (practice-that-teaches), SPEC-001 |
| **Extends** | [ADR-0024](ADR-0024-review-is-a-checked-test.md) (review is a checked test) |
| **Supersedes** | — |
| **Superseded by** | — |

## Summary

A new lesson step asks the learner to build a sentence: the English line is the cue, the chips are pointed Hebrew only, and the server grades the order. Pattern drills in review become a checked two-choice pick, or are skipped when no real question exists. Later-day results by kind of practice guide the author's mix and do not change FSRS. Review stays one known word and one skill per question ([ADR-0024](ADR-0024-review-is-a-checked-test.md)). Layouts that print the answer, need a partner, or only look like practice are left out on purpose.

## Context

[ADR-0024](ADR-0024-review-is-a-checked-test.md) made Today's review a checked test. That record stands. The lesson player still has no step that asks the learner to put words in order. Pattern drills in review still use Show answer and Forgot, Hard, Got it, and Easy. Nothing compares how words practiced in different ways do when they first come back on a later day.

[RSCH-011](../../sdd/research/RSCH-011-practice-layouts-that-teach/RSCH-011-practice-layouts-that-teach-report.md) ranked six practice layouts. A checked question on a known word ranked first (already ADR-0024). A scrambled sentence build ranked second and is the only tutor layout worth building as practice. Supporting baseline: spacing and retrieval in [RSCH-003](../../sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md); four strands and meaning-focused work on known words in [RSCH-004](../../sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md); hide supports while producing in [RSCH-005](../../sdd/research/RSCH-005-pictures-typing-spelling/RSCH-005-pictures-typing-spelling-report.md); feedback shows the right answer in [RSCH-006](../../sdd/research/RSCH-006-learner-interface-design/RSCH-006-learner-interface-design-report.md).

SPEC-001 independently checked three RSCH-011 findings against the public sources named below. Practice-test format evidence from Adesope et al. (2017) is confirmed. The self-paced reading-while-listening figure (g = 0.06) is confirmed. The Heift drag-and-typing finding is weakened: the directional claim holds, but the study is one small German L2 trial with short-term measures only. Brian accepted this plan by asking to build it; weakened findings stay in the record as weakened, and every decision below stays.

## Decision

**D1 · Sentence build is a lesson step.** Review stays one known word and one skill per question ([ADR-0024](ADR-0024-review-is-a-checked-test.md)). A sentence has several words, and a review card schedules one. A sentence build in review can come later, once the later-day measure says it helps. Rejected: putting a multi-word order step into Today's review sitting now.

**D2 · The cue is the English line.** The chips are pointed Hebrew only. The line's reading, meaning, and clip appear after the check. This follows [RSCH-005](../../sdd/research/RSCH-005-pictures-typing-spelling/RSCH-005-pictures-typing-spelling-report.md): hide supports while the learner produces. Rejected: English or transliteration on the chips during the attempt.

**D3 · Only the line's own words.** The first version adds no extra chips that do not belong. This is a product choice. It keeps a wrong answer about order, not about vocabulary. Rejected: distractor chips from outside the line.

**D4 · Build on known words.** Every chip is a word the learner already knows, or a new word that was introduced earlier in the same unit. The validator blocks anything else. This follows meaning-focused work on known words in [RSCH-004](../../sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md). Rejected: an order step that introduces a brand-new word on the chips.

**D5 · Exact order, or an order the author listed.** Hebrew allows more than one order for some lines, so the author can list other full orders that are also right. The learner gets one try, with no partial credit. A wrong answer shows the right order ([RSCH-006](../../sdd/research/RSCH-006-learner-interface-design/RSCH-006-learner-interface-design-report.md)). Rejected: scoring by shared prefix or by counting how many chips sit in the right slot.

**D6 · Drag is the main way in.** [RSCH-011](../../sdd/research/RSCH-011-practice-layouts-that-teach/RSCH-011-practice-layouts-that-teach-report.md) reports that dragging and typing beat clicking for word-order practice (Heift). Independent check: **weakened**. Heift (2003) does show drag and typed completion both beat multiple-choice click for German word order on a post-test (drag mean errors 4.1, type 5.9, click 8.7; n = 27), but it is one small CALL study, German not Hebrew, and the author states the results apply to short-term learning only. The decision stays as Brian's product choice: drag is the main input; tap-to-place and the keyboard also work so the step is usable on a phone and with a screen reader. Rejected: click-only multiple choice as the main order UI.

**D7 · A pattern pick asks a real question or is skipped.** When the two forms have different glosses, the English gloss is the cue. When they share a gloss, the cue is a sentence from the learner's own agreement miss with the form blanked. When neither exists, the step is not served. This is the same rule as the letter blank in [ADR-0024](ADR-0024-review-is-a-checked-test.md) D4. Adesope, Trevisan, and Sundararajan (2017) is **confirmed**: practice testing beats restudy, and mixed-format (g = 0.80) and multiple-choice (g = 0.70) practice tests produced larger effects than short-answer (g = 0.48) — which supports checked recognition formats inside review, not restudy or self-grade. Rejected: a review pattern step whose grade is Forgot, Hard, Got it, or Easy.

**D8 · Measure before adding more.** For each kind of practice, the app counts how often words practiced that way are answered right when they first come back on a later day. The number needs at least 20 answers before it is shown. Below that, the author sees "not enough yet" and keeps the default mix. The number guides the mix and does not change FSRS. Spacing and delayed recall remain the schedule's job ([RSCH-003](../../sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md)). Rejected: changing FSRS weights from practice-type results, and rejected: shipping more practice layouts before this measure exists.

## Layouts left out

These shapes appear in [RSCH-011](../../sdd/research/RSCH-011-practice-layouts-that-teach/RSCH-011-practice-layouts-that-teach-report.md). They are not built as practice that teaches:

- **All-at-once vocab sheet.** Audio, pointed Hebrew, reading, and meaning visible together with no graded input. Fine as a first-meeting study card (already exists); not practice after that ([RSCH-003](../../sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md), [RSCH-005](../../sdd/research/RSCH-005-pictures-typing-spelling/RSCH-005-pictures-typing-spelling-report.md), [RSCH-006](../../sdd/research/RSCH-006-learner-interface-design/RSCH-006-learner-interface-design-report.md)).
- **Glossed word bank.** Pointed Hebrew chips that also show English glosses. The gloss gives the answer away while the learner "practices" the Hebrew.
- **Partner interview.** Needs a partner; the productive act is off-screen. Not for a solo adult app.
- **Story comprehension questions.** Optional meaning-focused input later, after the later-day measure shows how sentence build and pattern pick are doing. Independent check **confirmed** the small self-paced reading-while-listening effect RSCH-011 cited (Clinton-Lisell, 2023: overall RWL g = 0.18; self-paced g = 0.06, n.s.), which does not raise this layout's priority now ([RSCH-004](../../sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md)).

## Consequences

- North Star section 6 adds the `order` (build-the-sentence) activity; section 12 points here for practice beyond review.
- [ADR-0024](ADR-0024-review-is-a-checked-test.md) is unchanged: review stays one checked question per word and skill; self-grade buttons stay on flashcards only.
- Later INIT-029 specs implement the `order` schema and screen, the pattern pick, and later-day results against this record.
- The weakened Heift finding does not reopen click-only order practice; drag remains the main input with tap and keyboard as accessibility paths.
