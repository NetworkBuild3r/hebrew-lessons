# ASMT-006 · What the code needs for the five lesson types, and how MCP clients are made to follow them

**Source:** the brief (`internal/brief`), the validator (`internal/units`), the MCP server and its resources (`internal/mcp`), and the lesson player (`internal/web`)
**Target:** `docs/design/lesson-architecture.md`, the five lesson types: word, conjugation, listening, easy, and spelling (working tree, uncommitted)
**Source tree:** local `6258362`. `origin/main` is `134e1e3`, one commit ahead. That commit changes only `internal/learner/*` and `internal/jobs/*_test.go`. No file cited here differs.
**Assessed:** 2026-11-10
**Harness:** cursor
**Recommendation:** Refactor (server side). The external AI keeps writing the Hebrew. The server decides the lesson type, the step order, and the word cards.
**Confidence:** 0.80
**Supersedes:** ASMT-005's target. ASMT-005 measured against the older four-phase design, which `lesson-architecture.md` replaced on 2026-11-10.

Read-only on the source. Nothing outside `sdd/assessments/` was changed.

## Update 2026-11-10: Brian's choice

Brian chose **code assembles each lesson, and the AI writes only the story**. That fits BRAIN-001, which stops AI-written units and retires the unit pipeline in Stage 2.

- **Part A still holds.** The player gaps (no grid, `produce` is buttons not typing, no second ask), the rules that block the design, the skill-name mismatch, and the unrecorded letters check are all still true. Stage 2's question engine and screens need them.
- **Part B's approach is replaced.** The outside AI no longer writes whole units, so the recipe rule, `lessonType` in the unit, and the template mapping are not needed. The design is enforced by taking the shape away from the AI: it fills one story slot, checked by the existing story rules. The surfaces Part B found out of date (`hebrew://authoring`, the worksheet templates, `get_lesson_brief`) retire with the unit pipeline.
- **The handoff changed.** It is not a new initiative. It is BRAIN-001's Stage 2 outline, rewritten the same day around the five lesson types: [`spec-skeleton.md` §5](../../brainstorms/BRAIN-001-is-the-hebrew-app-built-right/spec-skeleton.md). Stage 2 still opens only after its gate.

The rest of this report is as it was assessed.

## BLUF

**Build it in the server, not in prompts.** An outside AI that uses the MCP never reads our skills. It reads the tool descriptions and four resources. Then it gets checked by the validator. So the design has to live in three places the server owns:

1. The brief tells the AI which lesson type to write and the steps in order.
2. The validator blocks a lesson whose steps don't match that type, with or without a brief.
3. The server fills in the word cards from the lexicon, so the AI cannot get them wrong.

**Today's rules would block the design.** A word lesson with 4 new words is blocked three ways:

- `budget.lesson` allows 3 new words per lesson.
- `unit.drill_share` caps single-word steps at 40%. The meaning check plus the write step makes 8 of 12 steps.
- `brief.new_intro_before_test` doesn't count a word grid as an introduction, because there is no grid step.

Spelling, listening, and conjugation lessons are blocked by `brief.story_length`, which wants a story in every lesson.

**Nothing can start yet.** ADR-0033 lists "a second initiative opens before Stage 1 closes" as an abandon line (A2). Stage 1 adds no validator rule (D9). The pace and tier gates in `internal/brief` are frozen, and so is new work on word-page verb forms (D12). This report is the plan for after Stage 1 closes, which is due by 2026-11-30, and only with Brian's go.

## Before anything: the Stage 1 gates

| Gate | Source | What it means here |
| --- | --- | --- |
| A2 abandon line | `docs/decisions/ADR-0033-stage-1-trust-and-measurement.md` line 209 | Opening this as an initiative before Stage 1 closes trips an abandon line. |
| No new validator rule in Stage 1 | ADR-0033 D9, line 111 | The recipe rule below waits. |
| Rule budget: a new rule retires an old one | ADR-0033 D13, line 137 | The plan below adds 2 rules and retires 4. |
| No new work on the pace and tier gates, or on word-page noun and verb forms | ADR-0033 D12, lines 131–132 | Changing the new-word count waits. D12 doesn't name a conjugation lesson. The forms screen reads the same `verb_forms` data, though, so Brian should say whether it counts as frozen work. |
| No AI-written sentence is graded | ADR-0033 D3, lines 46–49 | "Use it in a sentence" and listening questions are ungraded reveals unless the answer key comes from the lexicon. |
| Author and reviser paused, auto-publish off | ADR-0033 D11, lines 120–122 | Even a correct unit stays a draft until Brian lifts the pause. |

## Part A · What the code needs for each lesson type

### What already works

| Design piece | Where it lives | Evidence |
| --- | --- | --- |
| Meaning check: see the word, pick the English | Lesson `recall` of one word. The Hebrew shows and the meaning is hidden. Up to 4 English buttons, with overlapping glosses dropped. | `internal/web/lesson.go` 1239–1243 (`visibleFields`), 1422–1435 (`askChoices`, `wideChoices`) |
| The meaning check is a provable ask | A single-word `recall` is not on the `unprovableAsk` list | `lesson.go` 1273–1282 |
| Story, tap a word for its meaning and sound | `story` and `wordCard` | `internal/web/views/player.templ` 291–310 |
| Repeat the sentence, ungraded | `shadow`, `say` | `internal/units/brief_rules.go` 163–165 (`exposureActivity`) |
| Story coverage, staged by tier | `brief.coverage`, tier `minKnownShare` | `internal/units/doc.go` 28, 66–72 |
| Easy lesson | The brief's consolidate path: no new words, a longer passage | `internal/brief/build.go` 425–441 (`PaceFor`), `shape_rules.go` 124–137 |
| Typed answer graded letter by letter | Review's `type_it` with `fsrs.GradeTyped` | `internal/activity/types.go` 19, 44–47; `check.go` 190 |
| Verb forms stored per person, number, and gender, each linked to a lexicon entry | `db.VerbForm` with `FormLexemeID` | `internal/db/verbforms.go` 27–38 |

### What is missing or works against the design

| # | Design step | Today | Change |
| --- | --- | --- | --- |
| A1 | **Grid**: all new words on one screen, with pointed form, Listen, part of speech, binyan, meaning, one example, and verb forms closed | No such step. A word is introduced only by a story, dialogue, shadow, or say (`brief_rules.go` 163–165). | New `words` activity type. Its only field is the list of new words. The server builds each card from the lexicon (`LexemeFacts`, `doc.go` 301–308; binyan and forms from `lookup` and `verb_forms`). Add it to `exposureActivity` so it counts as the introduction. One example sentence per word is the only text the AI writes, and it is checked like a story line. |
| A2 | **4 new words in one sitting** | `MaxNewWordsPerLesson = 3` blocks a 4th word (`internal/units/rules.go` 18, `validate_budget.go` 34). `MinNewWords = 5` above the beginner tier (`brief/build.go` 34). The middle pace gives 5 (`build.go` 112). | Lesson cap 4 at the beginner tier, up to 8 past it. Default 4 everywhere. Frozen by D12 until Stage 1 closes. |
| A3 | **Meaning check: a miss shows the meaning and asks once more** | One tap grades it, and there is no second ask (no retry path in `lesson.go`). | Player: after a miss, show the gloss and ask the same word once more. Record only the first answer. |
| A4 | **Meaning check by ear** (hear it, pick the English) | A single-word lesson `listen_pick` offers Hebrew buttons. It falls back to the meaning only when it can't build Hebrew lines (`lesson.go` 1000–1009). | Player: in a word lesson, a single-word `listen_pick` uses English buttons. |
| A5 | **The check doesn't mark the word known** | A lesson `recall` or `listen_pick` schedules that skill's card at once (`lesson.go` 602–616). The one exception is a first miss on a `listen_pick` when there is no listen card yet: that is stored and not scheduled. ADR-0033 D4 counts "proved" only across days, so one lesson answer cannot make a word proved. | Keep it as it is. Say in the design that the lesson answer seeds the card but cannot prove the word. |
| A6 | **Write it: type the word, Hebrew hidden** | The catalog says `produce` is "the pointed Hebrew the learner types" (`internal/mcp/resources/authoring.md` 66). The player hides the Hebrew and then shows **three Hebrew buttons** (`lesson.go` 1244, 1428–1430). That is picking, not typing. | Player: lesson `produce` uses review's typed input and `GradeTyped`. The code then matches what the catalog already says. |
| A7 | **One skill name for `produce`** | The validator calls it spell (`brief_rules.go` 123). The player records it as recall (`lesson.go` 658). | Pick one. Under ADR-0033 D6, "Hebrew from English" is recall, so make the validator say recall too. |
| A8 | **Use it in a sentence, after they can write it** | `cloze`, `order`, and multi-word `listen_pick` are ungraded reveals (`lesson.go` 1273–1279, ADR-0033 D3). | No change. They stay practice. The recipe rule puts them after the write step. |
| A9 | **Conjugation lesson: open the forms of one pattern on known verbs, with Listen and a short English line** | Forms are stored, but there is no step type for them and no English per form (`verbforms.go` 27–38). | New `forms` activity type: a known verb and a tense. The server fills the forms. The English line is either a new column or one translation per form that the AI writes. It may fall under D12's verb-forms freeze (see the gates table). |
| A10 | **Listening lesson: hear a passage, answer from the audio, text hidden** | A multi-word `listen_pick` is a reveal (D3). A lesson whose questions are all sentence-level has no graded step. | Grade only asks whose key is a lexicon gloss: hear the passage, then pick the meaning of one word in it. Anything else is a reveal. |
| A11 | **Spelling lesson, only after the letters check** | The brief notes "no code records the letters check yet" (`brief/build.go` 332–336). | Record the letters check before the brief can pick spelling. |
| A12 | **A lesson type the learner can see** | The outline labels each activity, not the lesson (`internal/web/views/lessonframe.go` 83–99). | A label per lesson type on Today and in the outline. |

### Rules today that would block the design

| Rule | Why it blocks | Proposal |
| --- | --- | --- |
| `budget.lesson` (block) | A 4th new word in one lesson (`rules.go` 18). | Change the number (A2). This doesn't add a rule. |
| `unit.drill_share` (block with a brief) | A word lesson with 4 words has 1 grid, 4 meaning checks, 1 story, 4 write steps, and 2 sentence steps. That is 8 single-word steps out of 12, or 67%. The cap is 40% (`shape_rules.go` 13, 93–113). A spelling lesson is close to 100%. | **Retire.** The recipe rule decides how many single-word steps each type may have. |
| `brief.story_length` (block) | Every lesson needs a story or dialogue of `minStoryTokens` (`shape_rules.go` 39–58). Spelling, listening, and conjugation lessons would fail it. | **Retire.** It becomes one line in the word and easy recipes. |
| `unit.same_shape` (warn) | It warns when a unit is 80% the same as the last (`shape_rules.go` 14–17). Every word lesson is the same recipe on purpose. | **Retire.** Variety now comes from the lesson type. |
| `brief.new_intro_before_test` (block) | Only story, dialogue, shadow, or say count as the introduction. | Keep it. Add `words` to `exposureActivity` (A1). |
| `brief.pattern` (block) | A brief that names `nextPattern` makes every unit teach it (`shape_rules.go` 124–149). | Keep it. The brief sets `nextPattern` only for a conjugation lesson. |

## Part B · How the MCP makes outside agents follow the design

### Why the text alone can't do it

- An outside client reads the server instructions (`internal/mcp/server.go` 28–51), the tool descriptions, and four resources (`server.go` 23–26). It never reads `.cursor/skills/hebrew-lesson-author`.
- `hebrew://rules` says outright that the validator does not enforce `playerGuidance` (`server.go` 305). Anything that is only in text is optional to the client.
- ADR-0033 says the tool text should stop restating rules and point to the response instead (Consequences, line 229).
- A shared unit is published through `publish_unit`, which runs `units.ValidateIn` **with no brief** (`internal/mcp/tool_publish_unit.go` 80). `put_unit` also saves after a check with no brief (`internal/mcp/tools.go` 553). Only a learner publish re-checks against the brief, and only while auto-publish is on (`tool_publish_unit.go` 188). So a rule that lives only in the brief rules never touches a shared unit.

### What the server text tells clients today, against the design

| Surface | Today | Conflict |
| --- | --- | --- |
| `hebrew://authoring` "The shape" (`authoring.md` 100–110) | Each lesson opens with a story or dialogue, then sentence questions, then bare-word drills only if needed. Vary the sequence. | The opposite of the word lesson, which starts with the grid. |
| `hebrew://authoring` line 5 | "A learner-scoped unit that passes the validator … goes straight to that learner's Today with no human step." | Out of date under D11. Auto-publish is off. |
| `hebrew://authoring` budget (lines 124–131) | At most 3 new words per lesson, so 4 words need two lessons. | Against the 4-word grid. |
| `hebrew://authoring` examples 1 and 2 (lines 219–400) | Both open with a story or dialogue. `internal/mcp/resources_test.go` runs them through the validator. | The examples teach the old order. Because the test runs them, a rewrite has to pass the new rules. |
| Catalog (`server.go` 298, `authoring.md` 56–68) | "The ten activity types." `produce` is described as typed. | No grid or forms step. The `produce` description doesn't match the player (A6). |
| Worksheet templates (`internal/mcp/templates.go` 21–25) | Five shapes, including `topic-conversation`, "a conversation about one situation" | Conversation is not a lesson type. The other four don't map to the five types. |
| `get_lesson_brief` (`internal/mcp/tool_lesson_brief.go` 14–28) | Returns `skillFocus` and a flat list of activity types to favour (`brief/build.go` 270, 820–834) | No lesson type and no step order. |
| Brief plan fields (`brief/build.go` 252–288) | No lesson-type field. A search of `internal/` for lessonType or lesson_type finds nothing. | The client can't be told which type to write. |

### The changes, in the order they protect the design

1. **The brief picks the type.** Add `lessonType` and `steps` to `brief.Plan`. The choice is deterministic:
   - Consolidate gives an easy lesson (this already exists, `PaceFor`).
   - A due pattern gives a conjugation lesson.
   - Listening as the weakest skill, with nothing new, gives a listening lesson.
   - A passed letters check plus words that are known but can't yet be typed gives a spelling lesson.
   - Anything else is a word lesson.

   `steps` is the recipe for that type, as activity types in order. The AI doesn't choose.
2. **The document says its type.** Add `type` to `units.Lesson` (`doc.go` 190–193). Then the rule works on shared units with no brief, and the player can label the lesson.
3. **One rule checks the recipe, with or without a brief.** `unit.lesson_recipe` (block) runs in `Validate`, so `publish_unit` and `put_unit` both hit it. It checks each lesson's steps against its type:
   - **word:** `words`, then a meaning check for every new word, then a story at the tier's coverage, then `produce` for every new word, then optional sentence steps.
   - **conjugation:** `forms`, then practice on those forms, then a passage. No new dictionary words.
   - **listening:** a passage, then asks. No new words.
   - **easy:** no new words and no new pattern, with a passage at least `minStoryTokens` long.
   - **spelling:** `produce` only, on words already known by meaning. An optional sentence may come at the end.

   A second rule, `brief.lesson_type` (block), checks that the type matches the brief's `lessonType`. The two new rules retire `unit.drill_share`, `brief.story_length`, `unit.same_shape`, and `budget.lesson.few`, which meets D13's budget.
4. **The server writes the cards.** The `words` and `forms` steps carry only lexicon references. The server fills in the part of speech, binyan, gloss, forms, and audio. An outside AI can't invent a binyan or a gloss on a card, because no field takes one.
5. **The text points, it doesn't teach.**
   - Rewrite "The shape" in `hebrew://authoring` to say: read `lessonType` and `steps` from the brief and write them in order. Then rewrite the two examples, one word lesson and one easy lesson, so they pass `resources_test.go`.
   - Fix line 5 for D11.
   - Update the catalog for `words`, `forms`, and typed `produce`.
   - Retire `topic-conversation`. Map the others: `short-story` to word, `verb-workshop` to conjugation, `foundations-reading` to easy. `review-sheet` is review, not a lesson.
   - Add one sentence each to the server instructions and the `get_lesson_brief` description: the brief names the type and steps, and `unit.lesson_recipe` checks them.
6. **The skills.** Rewrite them to the same pointer text, still paused. They only matter for our own sessions.

### Rule budget

| Added | Retired |
| --- | --- |
| `unit.lesson_recipe` (block, runs with and without a brief) | `unit.drill_share` |
| `brief.lesson_type` (block) | `brief.story_length` |
| | `unit.same_shape` |
| | `budget.lesson.few` |

Net: two fewer rules.

## Decisions for Brian

1. **Does a "lesson" in the design mean one lesson inside a unit, or a whole unit?** Today a unit has several lessons and a checkpoint, and `budget.lesson` counts per lesson. The cleanest fit is one design lesson = one unit lesson, with the checkpoint left for review. That needs the per-lesson cap raised to 4.
2. **Is `dialogue` still allowed as the passage in a word lesson?** `lesson-architecture.md` doesn't mention it. Conversation is not a lesson type, but a two-voice passage is not a conversation with the learner.
3. **Where does a form's English line ("I wrote") come from?** Either a column on `verb_forms`, or a translation per form written by the AI and checked like any translation.
4. **What sentence steps does the word lesson allow, given D3?** All of them are ungraded reveals today. If that's fine, the recipe lists them as optional.

## Claims and evidence

Every load-bearing claim was read in this session (Tier A).

| Claim | Evidence |
| --- | --- |
| A word lesson with 4 words breaks the 40% drill cap | `shape_rules.go` 13, 93–120. `singleWordDrill` counts one-token `recall` and `produce`. |
| A lesson may introduce at most 3 new words | `rules.go` 18, `validate_budget.go` 34 |
| Every lesson needs a story when the brief sets `minStoryTokens` | `shape_rules.go` 39–58 |
| Lesson `produce` is a Hebrew-button pick, not typing | `lesson.go` 1244, 1428–1430. The search for typed input in `lesson.go`, `player.templ`, and `lessonframe.templ` found none. |
| The skill name for `produce` disagrees | `brief_rules.go` 123 (spell), `lesson.go` 658 (recall) |
| Shared publish runs no brief rules | `tool_publish_unit.go` 80, `tools.go` 490, 553 |
| The brief has no lesson type | `brief/build.go` 252–288. A search of `internal/` for lessonType or lesson_type found nothing. |
| The authoring guide teaches story-first | `authoring.md` 100–110, 219–400 |
| The letters check is not recorded | `brief/build.go` 332–336 |
| The Stage 1 gates | ADR-0033 lines 46–49, 111, 120–132, 137, 209 |

## Falsification

- **"The current rules could already express a word lesson if the AI were careful."** False. The drill cap and the per-lesson budget block a 4-word lesson however it is written. A 3-word version still has 6 drills out of 10, which is 60%.
- **"Better text in `hebrew://authoring` would be enough."** Weakened by two facts. The rules resource says the validator does not enforce guidance. And a shared unit never sees a brief, so only a rule that runs in `Validate` reaches it.
- **"Refactor is wrong because writing the Hebrew is judgment."** True, but that judgment is already outside, in the MCP client. Nothing new needs to become an agent. The change narrows what the outside AI may decide.
- **Not checked:** whether `resources_test.go` checks every rule on the examples or only some. I did not run the tests. Nothing in the recommendation depends on it.

## Confidence

0.80. The code claims are Tier A from this session. A separate checker that wrote none of this report re-read 12 load-bearing claims. It confirmed 11, found one partly true (the `listen_pick` first-miss exception, now in A5), and flagged one overreach about D12 (now softened). Verdict: MITIGATE, fixed. Its notes are in `eval/`. The design is an uncommitted working-tree file. Two of the decisions above (the lesson-to-unit mapping and dialogue) change how big the recipe rule is, but not whether it is needed.

## Handoff

Open this only after Stage 1 closes and only with Brian's go. Use `create-initiative` with `source: ASMT-006`.

1. **`service`, brief.** `lessonType` and `steps` in `brief.Plan`, the type picker, a default of 4 new words, and recording the letters check. The new-word count touches D12's pace and tier gates.
2. **`service`, validator.** `Lesson.type`, the `words` and `forms` activity types, `unit.lesson_recipe`, and `brief.lesson_type`. Retire the four rules above. Change `budget.lesson` to the tier figure. Settle the `produce` skill name.
3. **`frontend`, player.** The grid screen built from the lexicon, typed lesson `produce`, the second ask after a miss, English buttons for a single-word listening check in a word lesson, the forms screen, and lesson-type labels.
4. **`docs`, MCP text.** Rewrite "The shape" and its examples in `hebrew://authoring`, fix the D11 line, update the catalog, retire `topic-conversation` and map the other templates, and add a line to the server instructions and the `get_lesson_brief` description. Update the paused skills to match.

Spec 2 can't ship before spec 3. A `words` step the player can't draw would block every word lesson.
