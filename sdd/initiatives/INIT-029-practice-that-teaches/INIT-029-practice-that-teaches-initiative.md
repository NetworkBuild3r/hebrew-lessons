# INIT-029: Practice that teaches

**Initiative ID:** INIT-029 · **Status:** complete · **Created:** 2026-11-03 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main`
**Source:** Brian, 2026-11-03: "How can we tune our program towards this research. Helping us build something that actually helps us learn?" Evidence: [RSCH-011](../../research/RSCH-011-practice-layouts-that-teach/RSCH-011-practice-layouts-that-teach-report.md), which ranked six practice layouts from his tutor's worksheets and our own review. A checked question on a known word ranked first (5.00). A scrambled sentence build ranked second (3.50) and is the only tutor layout worth building as practice. Supporting reports: [RSCH-003](../../research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md) (spacing High, retrieval Moderate), [RSCH-004](../../research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md) (four strands, meaning-focused work on known words), RSCH-005 (hide supports while producing), and RSCH-006 (feedback shows the right answer). RSCH-011's three load-bearing findings were checked on 2026-11-03: Adesope and the self-paced reading-while-listening figure confirmed, Heift weakened (see the verification note on that report).

Work is in this checkout, on main. Started 2026-11-03.

## What we found

The lesson player has nine activity types (`internal/units/doc.go`): recall, listen_pick, produce, cloze, pattern, story, shadow, dialogue, and say. None asks the learner to put words in order. The only checked sentence work is a cloze with one gap, or a listen_pick that asks what a whole line means.

Some of the research is already in the app:

- Cloze buttons are pointed Hebrew with no reading and no gloss (`askChoices` in `internal/web/lesson.go`). The tutor's glossed word bank gives the meaning away. Ours does not.
- The word card that shows Hebrew, sound, and meaning together appears when a word is first taught. After that, review asks before it shows.
- Review asks a known word one checked question (INIT-028, ADR-0024).

Three gaps remain:

- **Pattern drills still use self-grading.** In review they use `flashFrame`, which has Show answer followed by Forgot, Hard, Got it, and Easy (`internal/web/learn.go`, the comment above the review card builder and `fillDrill`). This is the last self-graded screen in a review sitting, and ADR-0024 D1 says review is a checked test.
- **We cannot tell which practice works for this learner.** Every attempt records its activity (`internal/evidence/recorder.go`), but nothing compares how words practiced in different ways do when they come back on a later day. RSCH-011's support for building sentences comes from small studies, so our own data should check it.
- **INIT-028 is not finished.** CI run [41549086232](https://github.com/NetworkBuild3r/hebrew-lessons/actions/runs/41549086232) failed on 39434cd. `TestAnsweredClosesACheckedVisit` in `internal/session` fails, and so do about 25 review tests in `internal/web`, including `TestReviewAnkiReveal`, `TestReviewTypedActivities`, `TestReviewGrade`, and `TestReviewDrillKinds`. INIT-028 SPEC-002 and SPEC-003 are still marked planned.

## What you get

- **Build the sentence.** This is a new lesson step. You see the English meaning of a line and its Hebrew words in shuffled order as pointed chips, with no reading and no English on them. You drag the chips into order and check. A right answer plays the line. A wrong answer shows the right order and plays it. Every chip is a word you already know or met earlier in the same lesson.
- **Pattern drills are a test too.** A pattern step in review becomes a two-choice pick, like the letter drill: which form fits this meaning or this sentence. The four self-grade buttons leave review completely and stay on flashcards.
- **The lesson author learns from your results.** For each kind of practice, the app counts how often words practiced that way are answered right when they first come back on a later day. The lesson author sees those numbers in `get_context` and uses more of whatever is working for you. Until there are enough answers, it says so and uses the default mix.
- **What we leave out, on purpose.** A sheet that shows every word with its meaning works as a first-meeting card. We already have that, and it is not practice. A word bank with English on the chips gives the answer away. A partner interview needs a partner. Comprehension questions on a story are worth trying later, after the measure shows how the rest is doing.

## Specs

| Spec | Domain | Title | Blocked by | Effort | Status |
| --- | --- | --- | --- | --- | --- |
| [SPEC-001](specs/SPEC-001-docs-practice-that-teaches.md) | docs | ADR and an independent check of RSCH-011 | — | 2h | complete |
| [SPEC-002](specs/SPEC-002-content-order-activity.md) | content | The `order` activity in the unit schema and validator | SPEC-001 | 4h | complete |
| [SPEC-003](specs/SPEC-003-service-order-grading.md) | service | Serve and grade a sentence build | SPEC-002 | 4h | complete |
| [SPEC-004](specs/SPEC-004-frontend-order-screen.md) | frontend | The build-the-sentence screen | SPEC-003 | 5h | complete |
| [SPEC-005](specs/SPEC-005-service-pattern-pick.md) | service | A pattern drill is a checked two-choice pick | SPEC-001 | 4h | complete |
| [SPEC-006](specs/SPEC-006-frontend-pattern-pick.md) | frontend | Pattern pick screen, and no self-grade frame in review | SPEC-005 | 3h | complete |
| [SPEC-007](specs/SPEC-007-service-later-day-results.md) | service | Later-day results by kind of practice | SPEC-001 | 4h | complete |
| [SPEC-008](specs/SPEC-008-mcp-author-uses-results.md) | mcp | The lesson author writes `order` steps and reads the results | SPEC-002, SPEC-007 | 3h | complete |

**Total:** 29h. **Critical path:** SPEC-001 → SPEC-002 → SPEC-003 → SPEC-004 = 15h.

**Order.** INIT-028 goes first: its CI failures are fixed and SPEC-002 and SPEC-003 are marked complete. After SPEC-001, three tracks can run side by side: sentence build (SPEC-002 → SPEC-003 → SPEC-004), pattern pick (SPEC-005 → SPEC-006), and results (SPEC-007). SPEC-008 waits for the schema and the results.

## Decisions

- **Sentence build is a lesson step.** Review stays one known word and one skill per question (ADR-0024). A sentence has several words, and a review card schedules one. A sentence build in review can come later, once the measure says it helps.
- **The cue is the English line.** The chips are pointed Hebrew only. The line's reading, meaning, and clip appear after the check. This follows RSCH-005: hide supports while the learner produces.
- **Only the line's own words.** The first version adds no extra chips that do not belong. This is a product choice. It keeps a wrong answer about order, not about vocabulary.
- **Build on known words.** Every chip is a word the learner already knows, or a new word that was introduced earlier in the same unit. The validator blocks anything else.
- **Exact order, or an order the author listed.** Hebrew allows more than one order for some lines, so the author can list other full orders that are also right. You get one try, with no partial credit. A wrong answer shows the right order (RSCH-006).
- **Drag is the main way in.** RSCH-011 reports that dragging and typing beat clicking for word-order practice. Tap-to-place and the keyboard also work so the step is usable on a phone and with a screen reader.
- **A pattern pick asks a real question or is skipped.** When the two forms have different glosses, the English gloss is the cue. When they share a gloss, the cue is a sentence from the learner's own agreement miss with the form blanked. When neither exists, the step is not served. This is the same rule as the letter blank in ADR-0024 D4.
- **Measure before adding more.** The later-day number needs at least 20 answers before it is shown. Below that, the author sees "not enough yet" and keeps the default mix. The number guides the mix and does not change FSRS.

## Threat model

- **No new trust boundary.** The signed-in learner posts their own lesson step and review answer through the existing CSRF- and Origin-checked handlers. The learner id comes from the session.
- **Input.** A sentence build posts a list of chip positions. The server checks it against the chips it stored when it served the step. A list that does not use each served chip exactly once is a 400 and writes nothing. A pattern pick posts one of the two served option ids, and anything else is a 400.
- **Integrity.** A step that cannot be served writes no attempt. The shuffled order is never the right answer.
- **MCP.** `get_context` returns later-day results only for the `learnerId` it was called with, under the same auth as the cards it already returns. The new unit field is validated like every other text field: pointed Hebrew, no markup, and lexicon lookup.
- **Output.** Screens stay templ-escaped.

## Depends on / unblocks

- Depends on INIT-028 being green and complete. Builds on the lesson player and validator (INIT-015), the letter drill pick (INIT-027), the speed rule (INIT-022, ADR-0020), and the attempt log (`internal/evidence`).
- Out of scope: story comprehension questions, a partner interview, a glossed word bank, typed Hebrew answers inside lessons, FSRS weights, and sentence builds inside review.

## Done when

- A lesson can contain an `order` step, which the validator checks and the player grades. A wrong answer shows the right order and plays it.
- A review sitting has no Forgot, Hard, Got it, or Easy button for any mode, including pattern drills.
- `get_context` shows, for each kind of practice, how often words practiced that way were right on their next later-day review, or "not enough yet".
- The lesson author's guide says when to use an `order` step and how to read those results.

Shipped 2026-11-03 in this checkout, on main, uncommitted. Package tests for each spec passed in Docker `golang:1.26.9`. The order screen's Playwright test is written (`internal/web/order_browser_test.go`) and was not run: Playwright is not installed here. Go tests cover the ask HTML, the wrong-answer reveal, the keyboard markup, and the chip-row CSS.
