# INIT-032: Every question has one right answer, and nothing covers the lesson

**Initiative ID:** INIT-032 · **Status:** in progress · **Created:** 2026-11-06 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main`
**Source:** Brian, 2026-11-06, on https://hebrew.ibhacked.us/lesson/12/0: "now I have a card in the way of my lessons I cannot get rid of." Then: "Many can be right. but I don't pick the right one I am wrong. How can we also build it in a way that we don't have these options?" Then, on step 11 of the same lesson, a gap with no sentence at all: "Even worse."

## What we found

**The card.** It is the "How this works" explainer, a `<dialog id="how-sheet">` on the first step of a lesson that has an explainer. `.how-sheet` in `internal/web/static/css/learn.css` sets `display: flex`, which beats the browser's own rule that hides a closed dialog. The dialog is on screen from the first paint, empty, because its body loads only when the learner taps "How this works." Its Close button calls `dialog.close()`, and a dialog that was never opened has nothing to close. So it cannot be dismissed.

**The gap with several right answers.** A `cloze` shows the pointed Hebrew line with one word blanked and four Hebrew buttons. The player takes the wrong buttons from the same part of speech first (`wideChoices` in `internal/web/lesson.go`), which is what makes them believable, and also what makes them fit. In "אֲנִי __" with לַעֲשׂוֹת, רוֹאֶה, אוֹמֵר, לְהָכִין, every verb makes a sentence. Nothing on screen says which one the lesson means. The unit has an English `translation` for the line. The player never showed it on a cloze.

**The gap with no sentence.** Lesson 12 (unit 12, `learner:1`, revision 2) has four clozes whose whole text is one word: אֶת, רוֹאֶה, מֵבִיא, לְהוֹצִיא. The gap is the line, so the learner sees an empty box and four words. The validator accepted it because nothing counts the words around the gap.

**The same lesson three times.** Lessons 1, 2, and 3 of unit 12 are the same activities in the same order. The validator accepted that too.

## What you get

- **Nothing covers the lesson.** The explainer dialog is hidden until it is opened, and hidden again after Close or Escape.
- **A gap is asked by meaning.** The English of the line sits above the Hebrew. "I live." over "אֲנִי __" leaves one verb that fits.
- **An agent cannot write the bad shapes again.** A cloze needs a word around the gap and an English line. A lesson cannot repeat an earlier lesson of its unit. Each is a block on a new write. The unit already published warns, so it keeps playing until it is rewritten.
- **A word that is also right is never a wrong button.** The author can list the other words that fit a gap, and the player leaves them out of the choices.
- **The bad lessons already published are listed and rewritten.**

## Specs

| Spec | Domain | Title | Blocked by | Effort | Status |
| --- | --- | --- | --- | --- | --- |
| [SPEC-001](specs/SPEC-001-docs-one-right-answer.md) | docs | ADR: a graded question shows one right answer | — | 1h | planned |
| [SPEC-002](specs/SPEC-002-web-explainer-hidden-when-closed.md) | web | The explainer dialog is hidden until it is opened | — | 1h | built, uncommitted |
| [SPEC-003](specs/SPEC-003-content-cloze-rules.md) | content | A cloze needs a sentence and its English, and a lesson cannot repeat | SPEC-001 | 2h | built, uncommitted |
| [SPEC-004](specs/SPEC-004-web-cloze-english-cue.md) | web | Show the English line on a cloze | SPEC-001 | 1h | built, uncommitted |
| [SPEC-005](specs/SPEC-005-web-cloze-also-right.md) | web | A word that also fits is never a wrong button | SPEC-003, SPEC-004 | 4h | planned |
| [SPEC-006](specs/SPEC-006-ops-deploy-and-check.md) | ops | Deploy and check lesson 12 on the live site | SPEC-002, SPEC-005 | 2h | planned |
| [SPEC-007](specs/SPEC-007-ops-list-the-bad-lessons.md) | ops | List every published unit the new rules warn on | SPEC-006 | 2h | superseded by [INIT-037](../INIT-037-taught-words-only/INIT-037-taught-words-only-initiative.md) SPEC-009 and SPEC-011 |
| [SPEC-008](specs/SPEC-008-content-rewrite-the-bad-lessons.md) | content | Rewrite the listed units | SPEC-007, INIT-031/SPEC-004 | 4h | superseded by INIT-037 SPEC-012 (units 12 to 34 unpublished 2026-11-08, INIT-037 SPEC-001) |

**Total:** 17h. **Critical path:** SPEC-001 → SPEC-003 → SPEC-005 → SPEC-006 → SPEC-007 → SPEC-008 = 15h.

**Order.** SPEC-002 stands alone and can ship first: it is the card in the way today. SPEC-006 deploys SPEC-002 through SPEC-005 together. SPEC-008 waits for INIT-031, because lesson 12 is started and the server on `main` still refuses to revise a started unit.

## Decisions

- **A graded question has one right answer on screen.** When the Hebrew alone allows more than one, the screen adds what picks one out: the English of the line. The buttons stay pointed Hebrew with no reading and no gloss, as INIT-029/SPEC-003 set.
- **The English line is the cloze cue.** The same `translation` field the author already writes. No new field for the cue. The prompt becomes "Pick the word that makes the Hebrew say this." A cloze with no translation keeps "Pick the missing word."
- **A cloze is at least two words.** One word with a gap is a `produce`. The validator says so in the message.
- **Repeat is checked only between teaching lessons.** A checkpoint reviews on purpose and is left out of the comparison.
- **Published units warn and keep playing.** The three new rules follow `activity.target` and `unit.translation_missing`: block in `ModeWrite`, warn in `ModePublished`. Lesson 12 keeps working with the English cue until SPEC-008 replaces it.
- **Also-right words are the author's call.** The server cannot tell whether a verb fits a sentence. The author lists the words that do, in the cloze's `accept`, and the player removes them from the choices. Same field name as `order`, the same meaning: other answers that are right.

## Threat model

- **No new trust boundary.** The CSS and the cue change only what a learner already receives. `translation` is English plain text, checked by `unit.translation_lang` and escaped by templ.
- **Validator rules are content checks.** They add block and warn findings. They read no new data.
- **`accept` on a cloze** is pointed Hebrew tokens from the lexicon, checked by the validator, and only removes buttons. It cannot add a button the lexicon does not have.
- **Live writes are gated.** SPEC-006 deploys (gate `deploy`). SPEC-007 only reads. SPEC-008 publishes learner units (gate `publish_live`), and every replaced revision stays in `unit_revisions`.

## Depends on / unblocks

- Builds on INIT-029 (cloze buttons are pointed Hebrew only) and INIT-030 (shape rules).
- SPEC-008 depends on INIT-031/SPEC-004, which lets the agent replace a started unit.

## Done when

- On the live site, lesson 12 step 1 has no panel over it, and the explainer opens and closes.
- Every cloze on the live site shows the English line above the Hebrew.
- `validate_unit` blocks a one-word cloze, a cloze with no translation, and a repeated lesson.
- No published unit in the SPEC-007 list is still live in its old shape.
