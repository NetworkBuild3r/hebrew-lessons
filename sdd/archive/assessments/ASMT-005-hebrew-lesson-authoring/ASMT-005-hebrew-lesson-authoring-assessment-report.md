# ASMT-005 · Current lesson authoring versus the 4-phase design

**Source:** this repo's lesson player, `get_lesson_brief` / `validate_unit`, and `.cursor/skills/hebrew-lesson-author`  
**Target:** the 4-phase lesson design in `docs/design/lesson-architecture.md` and `docs/north-star.md` §7.0 (working tree, not yet committed)  
**Assessed:** 2026-11-10  
**Harness:** cursor  
**Recommendation:** Hybrid  
**Confidence:** 0.82  

Read-only on the source. Nothing in the player, the MCP server, or the author skill was changed.

## BLUF

The daily review already matches the design: a checked test, not a flashcard. The lesson side does not. The author skill, which is also paused, tells the model to **open with the story**, then ask questions, then drill bare words. The new design **opens with the new words, then a short ungraded-feeling check, then the story, then application**. The validator will publish a lesson that never does that, because no rule requires the phase order. Closing the gap means a deterministic shape rule in the brief and the validator, and a rewrite of the author skill's "shape" section. The agent cannot be trusted to invent the order while the skill says the opposite.

## What already matches

| Design piece | Where it already lives | Evidence |
| --- | --- | --- |
| Review is a checked test. Forgot / Hard / Got it / Easy stay off that screen. | Review player | `internal/web/views/activity.templ` `ActivityFrame` grades with Check. `flashFrame` is a separate sitting. ADR-0024. |
| Story coverage near 95% known, staged for beginners | Validator + brief tier | Author skill "Coverage tiers". `docs/north-star.md` §7.3. |
| At most about 40% bare-word drills | `unit.drill_share` | Author skill line 121. ADR-0026 D4. |
| One new pattern, two reused, recycle earlier words | Brief fields | Author skill lines 106–111. `internal/units/brief_rules.go` skill mapping. |
| Tap a word in a story for sound and meaning | Lesson player | `internal/web/views/player.templ` `wordCard`, `flipHint`. |
| Build-the-sentence with Check, tap a placed word to send it back | `order` | `player.templ` `orderAskBlock`. |
| Say-it from an English cue, Hebrew after "I said it" | `say` | Author skill row for `say`. `player.templ` `SayAsk`. |
| Dialogue with two speakers and a side to say | `dialogue` | Author skill lines 94 and the dialogue row. |
| Code picks the words, the model writes the lesson | Brief + skill step 2 | Author skill lines 20 and 26. |

## Gaps

### 1. The prescribed lesson order is the reverse of the design

**Design.** Meet the words, quick check, story, then apply.

**Today.** The skill's shape section says each lesson opens with a story or dialogue, then sentence questions, then bare-word drills only if needed (`.cursor/skills/hebrew-lesson-author/SKILL.md` lines 115–121).

A unit that follows the skill never shows the new words before the story. The design's phase 1 does not exist as a required step. `exposureActivity` in `internal/units/brief_rules.go` (lines 159–164) allows a story, dialogue, shadow, or say to introduce a word. It does not require an introduction before the passage.

### 2. There is no "meet the words" screen

**Design.** Three to six new words, pointed, with audio and a gloss, and no grade.

**Today.** No activity type is "here are the new words." A new word is introduced inside a story, a dialogue, a shadow, or a say. The story's tap-a-word card is the closest thing, and it appears only after the learner is already in the passage.

### 3. The quick check is the wrong question

**Design.** Phase 2 is a short recognition check: hear the new word, pick the English meaning, or read it and pick the meaning. High success, meant to wake the word up before the story.

**Today.** A lesson `listen_pick` asks "What did you hear?" and the buttons are **lines of Hebrew**, the real line and the same line with the target word swapped (author skill lines 79 and 86–88). The English translation is stored and **the player does not show it**. That tests whether the learner caught which Hebrew line it was, not whether they know what the new word means. Review's listen-and-pick (English options, no Hebrew on screen) is the screen the design describes, and it is not what a lesson step does.

### 4. Nothing forces phase order, so the model can skip it

**Design.** Every lesson walks the four phases. Variety changes which apply-step you use (story lesson, dialogue lesson, pattern lesson, listening lesson), not whether the words are met before the story.

**Today.** `get_lesson_brief` returns `skillFocus` and a flat list of activity types (`brief_rules.go` `ActivitiesForSkill`, lines 145–156). It does not return an archetype or a phase list. `unit.same_shape` only warns when the type list is about 80% the same as the previous unit (author skill line 221). That pushes variety of sequence, which fights a stable phase order. The validator has no rule id for "new words introduced before the passage" or "a priming check sits between the introduction and the story."

### 5. The author and the reviser are switched off

Both skills open with: paused for INIT-038 Stage 1 (ADR-0033 D11); do not write or publish a learner unit until Brian lifts the pause (author skill lines 3–5). Even a corrected shape cannot reach Today until that pause is lifted. Auto-publish of learner units is also off behind `LEARNER_AUTO_PUBLISH` (INIT-038).

### 6. Apply-steps are missing the checks the design draws

| Design | Today |
| --- | --- |
| Fill-the-gap: place a word, remove it, then Check answer. A tap is not the grade. | A lesson choice is a submit button (`player.templ` `choiceButton`). The first tap is the answer. `order` does wait for Check. Cloze does not, unless the learner is dragging. |
| Reading question keeps the passage on screen, with its own "Read passage" control. | The story is an earlier step. The question step does not keep that text pinned. |
| A correction that says why, plus other accepted answers. | `order` can list other full orders in `accept` (author skill row for `order`). A wrong cloze or recall says the right answer. It does not explain the mistake in a sentence, and it does not list alternate words. |
| Placed chips show a remove cross, and number keys place a chip. | Placed words go back on a tap (`orderAskBlock` hint). No cross, no number badge. |

These are player changes. The brief cannot express them, because the activity document has no "check before grade" or "keep passage visible" field.

### 7. Word counts and "spell" disagree with the design sheet, slightly

The design says 3–6 new words. The brief says 5–8, or 3–4 under 100 known words, and pace can cut that to 2 (`docs/north-star.md` §7.1, author skill `pace`). Close enough to keep the brief's numbers. Do not add a second cap.

`produce` is "the pointed Hebrew the learner types" and `SkillOfActivity` maps it to **spell** (`brief_rules.go` lines 110–111 and 123). The North Star table maps "Pick the Hebrew" to recall and typing to recall, with spelling credited on the side. The design's apply-steps do not include a spell drill inside the lesson. A brief whose `skillFocus` is spell will ask the model for `produce` steps inside the lesson. That fights "spelling belongs in review."

## What not to rebuild

- Do not put flashcards into the lesson. The code already keeps them on `flashFrame`, off the checked review. The design agrees.
- Do not replace review's type-it, dictation, listen-pick, read-pick, or match boards. Those are the retention engine and they match Part 2 of the Figma page.
- Do not drop coverage, the drill cap, recycling, or one-new-pattern. The design sits on top of those rules. It changes order and the kind of early check, not the budgets.

## Recommendation

**Hybrid.**

- **Deterministic (Refactor):** extend the brief with a phase list (introduce, prime, passage, apply) and an archetype. Add validator rules: every new word appears in an ungraded introduce step before any passage; a prime step sits between introduce and passage; apply steps are cloze, order, a question on the passage, dialogue, or say, and bare-word drills stay under the existing 40% cap. Teach `produce`-as-spell only when the brief's skill focus is spell, and say so in one place.
- **Judgment (Agentify):** rewrite the author skill's "The shape" so it follows the phase list from the brief instead of opening with the story. Lift the pause only after that text matches the validator. The reviser gets the same shape, or it will "fix" new units back to the old order.

**No action** is wrong: the review half matches, the lesson half does not, and the skill currently teaches the old order.

## Confidence

0.82. The skill text and `brief_rules.go` were read in this session (Tier A). The player claims cite `player.templ` and `activity.templ`, read earlier in the same session. Not re-checked against `origin/main` after a fetch; the design docs themselves are uncommitted working-tree edits. An independent judge was not run.

## Handoff

An initiative under `create-initiative`, `source: ASMT-005`, two specs:

1. `service` — brief phase list and validator rules for introduce-before-passage and prime-before-passage.
2. `docs` — author skill and reviser skill shape section, still paused until the validator exists, then the pause is Brian's to lift.

Player work for Check-before-grade, a pinned passage, and chip removal is a third `frontend` spec, and it can wait until a unit actually emits those steps.
