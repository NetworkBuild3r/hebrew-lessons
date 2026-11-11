# Technical Architect — Brainstorm Analysis

**Session:** Lesson types done right
**Date:** 2026-11-10
**Domain:** Engineering
**Primary Lens:** System design and the fewest moving parts. Where does each piece of a lesson come from, when is it fixed, which existing code carries it, and what can be deleted?

---

## Phase 3: Independent Analysis

### Chain of Thought

#### 1. Primary Lens

I ask one question: what is the smallest set of parts that can build every lesson in code, with no human review, and still let Brian take back a bad grade?

I judge every option by three tests:

- **Reuse.** Does it run on code that already works and is already tested?
- **Deletion.** Does it let us remove code, rules, and tools, as ADR-0033 D13 requires?
- **Failure.** When a part fails (no story, no clip, a bad key, a restart), does the lesson still run?

All code claims below were read in this session at commit `6258362`. Line counts are true totals, blank lines included.

#### 2. Initial Assessment

The app already has most of the parts. They are wired for the wrong author.

**What already fits code assembly:**

- **One question engine already exists, in review.** `activity.Engine` builds each graded item from the lexicon at ask time. It keeps the answer on the server under a signed, single-use token (`internal/activity/doc.go:20-27`, `internal/activity/build.go:173-314`). It already has every graded step a word lesson or a spelling lesson needs:
  - `read_pick`: see the Hebrew, pick the English. This is the meaning check.
  - `listen_pick`: hear it, pick the English. This is the meaning check by ear.
  - `type_it`: English shown, type the Hebrew. This is "write it".
  - `dictation`: hear it, type it. This is the spelling lesson.

  Its wrong buttons skip glosses that share a content word with the answer (`fair` and `GlossesOverlap`, `distractors.go:95-103`). It can be told the source is a lesson and which unit (`WithSource`, `WithUnit`, `build.go:146-153`). A lesson source "stores the attempt and leaves every card alone" (`doc.go:37`).
- **The take-back already exists.** `postAlsoRight` voids a wrong answer with reason `also_right`, for a review card or a lesson step (`internal/web/also_right.go:21`, `:102`). A void is a row beside the answer, never a delete (`migrations/023_answer_voids.sql:7-18`). Reason `report` exists too (`internal/db/voids.go:14-17`).
- **Writing a lesson needs no MCP and no validator.** `Store.PutUnit` is a plain Go call that writes the unit, a revision, and its `lesson_items` in one transaction. It does not validate (`internal/db/put_unit.go:29-43`).
- **The unit row already does the hard bookkeeping:**
  - Finishing a lesson creates the cards for its new words (`internal/db/lesson.go:585-592`).
  - Review holds a word back until its intro lesson is finished, through `lesson_items` (`internal/db/learning.go:14-31`).
  - New cards are ordered by the intro unit's position (`internal/session/doc.go:82-84`).
  - `attempts.unit_id` ties each answer to its unit (`migrations/015_attempt_source_self.sql:13`). The void path needs that link.
  - `lesson_runs` holds the resume cursor (`migrations/001_init.sql:120-128`). Guardrail W2 (lesson abandon rate) reads that table.
- **The story checks already exist:** `story.unpointed`, `story.unmapped`, coverage, `brief.text_too_long`, `unit.no_markup`, `unit.story_repeat`, and `unit.meta_text` (`internal/units/rules.go:140-200`).
- **The word card view exists.** `wordCard` in `internal/web/views/player.templ:291` already shows a word's meaning and sound on tap.
- **Golden fixtures exist.** There are 47 unit JSON fixtures in `internal/units/testdata` and browser scripts in `internal/web/testdata`.

**What works against it:**

- **The lesson player is a second question engine, built to defend against AI text.** `internal/web/lesson.go` is 2,156 lines. It has its own button code (`askChoices`, `wideChoices`, `lessonChoices`, `choicePool`, `lineMateExclude`, `dropBorrowedGloss`, lines 1422-1812). It has its own listening lines (`listenlines.go`, 146 lines) and `internal/choices/pick.go` (186 lines). Much of it turns unsafe AI steps into reveals (`unprovableAsk`, 1273-1282). Code-built keys need none of that.
- **The two engines disagree on three things.**
  - A lesson `recall` or `listen_pick` schedules the card at once (`lesson.go:602-616`). The engine's lesson source leaves cards alone (`doc.go:37`).
  - Lesson `produce` is three Hebrew buttons, not typing (ASMT-006 A6).
  - **Button pools differ.** The lesson player filters buttons to taught words (`taughtChoicePools`, `lesson.go:1354-1356`, ADR-0032 D1). The review engine draws from the *whole* lexicon (`ActivityPool`, `internal/db/activity.go:28-40`), and `read_pick` asks with `seenFirst` off (`build.go:248`). Those buttons are English glosses, so no untaught Hebrew is shown. But it is a different rule, and the earlier draft of this file missed it.
- **Typed grading knows only one spelling.** `GradeTyped` strips the points from the pointed form and compares (`internal/fsrs/typed.go:17-28`, `activity/check.go:190`). Tutor documents are mostly unpointed (158 of 350 lines pointed), so they use full spelling with extra ו and י.
  - One extra letter is distance 1. That grades **Hard, a near miss**, on a right answer.
  - Two extra letters (for example קיבוץ against the skeleton קבץ) grade **Again, wrong**.
  - Brian named exactly this kind of fault as what breaks his trust.
- **The lexicon has no column for the full spelling.** `bare` is the consonant skeleton, derived by stripping points (`internal/lexicon/prepare.go:37-38`, `migrations/001_init.sql:39`). The validator assumes that (`internal/units/validate.go:222`). Repurposing `bare` would break story matching.
- **Today would hide every code-built lesson until a voice exists.** Today runs the full write-mode validator on each untouched learner unit and skips any with a block (`internal/web/shell.go:101-142`, `internal/units/write_mode_blocks.go:13-21`). `audio.missing` blocks any activity with Hebrew text and no clip (`internal/units/validate_audio.go:12-47`). Brian has rejected the computer voice, so new class words have no clips. A code-built lesson saved as today's unit document would never appear.
- **About 60 validator rules and 30 guidance lines** (`rules.go:140-200`, `215-245`) plus `internal/brief` (5 files, 1,836 lines) exist to steer an AI author. Under code assembly, almost all guard a closed door.
- **A report today hides the whole lesson.** `postReport` writes `unit_reports` and hides the unit (`shell.go:396-400`). "This is dumb" on one item needs a void on one answer, not a hidden lesson.
- **An attempt stores the prompt and the response, not the other buttons** (`migrations/015_attempt_source_self.sql:6-21`). That is enough for a fix ticket: the picked gloss or the typed text is the evidence. It is not enough to replay a pick screen exactly.
- **`record_taught_words` creates cards at once, due at the next 4:00** (`internal/mcp/tool_taught_words.go:30`). Tutor words would go straight to review with no word lesson.
- **The engine's pending answers live in memory.** A restart drops them (`doc.go:26-27`). A lesson must rebuild its current item on resume, not trust a stored token.

#### 3. Main Considerations

##### 3.1 When a lesson is built: plan at class load, assemble and freeze at Start, draw buttons at ask

Brian asked "load time, open time, or both". My answer is both, split by what each moment can know.

1. **At class load (import): build and check the ingredients.** Code stores the class lines, matches every token to the lexicon, and lists the true misses. It runs the lexicon entry checks (3.5). It produces the word queue: taught words with no card, in class order. Failures show up here, in the import reply, days before Brian could meet them.
2. **At Start: assemble the lesson.** When Brian taps Start, one function, `lessons.Assemble(learner, now)`, picks the type and the words from current state. It runs the pre-flight (3.7) and drops any failing step. It writes one learner unit through `PutUnit`, publishes it, and opens the `lesson_runs` row. That unit is frozen from then on. Resume reopens the same unit.
3. **At each graded step: draw the item.** `activity.Engine.Build` makes the item from the current lexicon. The key is fixed by the frozen step (a lexeme id and a question kind). The buttons are fresh, so a fixed gloss applies at the very next ask.

**Why not "build ahead and keep one waiting"** (the earlier draft's call)? It needs a trigger at finish, a recheck at open, and rules for a stale draft. Assembling at Start needs none of that. There is never a backlog of untouched units, so the reviser problem cannot come back. `Assemble` is deterministic for a given state, so Today can call the same function as a dry run to show the type and word count before Start. The cost is a few SQLite reads inside one request.

**Why not freeze the buttons?** A fix could not reach a lesson already built. ADR-0032 D1 also wants pools drawn at ask time.

##### 3.2 Story missing or failing: the lesson never waits for the AI

Brian's AI runs only when someone runs it. So a lesson must be whole without a story. The story is a bonus.

1. **The story slot is keyed to the plan, not to a unit.** `get_story_request` runs `Assemble` as a dry run and returns the next word lesson's new words, the known-word list, the coverage figure, and the length. `put_story` stores the text in a `story_slots` row and returns pass or fail with rule ids. The AI may retry as often as it likes. There is never more than one open slot per lesson type.
2. **At Start, the story is checked again** against current state. If it passes, it becomes the story step. Its lines also give each grid card its one example. Code takes the first story line that uses the word, so the AI writes nothing extra.
3. **If the story is missing or fails at Start, the step is dropped.** The lesson runs without it. A failing story is never shown "with warnings". A story that arrives after Start is kept for the next lesson only if it still passes then.
4. **Fallback text, if G3 allows reuse:** an ungraded "from your class" view of the tutor's own lines that use the new words. That text is human-written and Brian has already seen it. If G3 says no, there is no fallback, and that is fine.
5. **Story audio is optional.** "Repeat the line" shows only when a clip exists. A missing clip never hides a lesson.

##### 3.3 Keep or replace the unit document and player: keep the spine, replace the author and the player

- **Keep the `units` row as the container.** It already carries `lesson_items` (card gating and new-card order), `lesson_runs` (resume and W2), `attempts.unit_id` (voids), and the unit's revision history. Replacing it would mean rewriting card gating, the void path, and W2. That is more moving parts, not fewer.
- **Keep the `units.Unit` JSON shape, with two additions.** Add `type` to the lesson (word, conjugation, listening, easy, spelling), as ASMT-006 Part B #2 proposed. Add one step kind, `words` (the grid), and later `forms`. A step holds only lexeme ids and a question kind. No step holds a gloss, a binyan, or a button. One design lesson is one unit with one lesson.
- **Do not run the unit validator on a built lesson.** The pre-flight (3.7) replaces it. Today offers built lessons through `Assemble`, not through `nextTodayLesson`'s `WriteModeBlocks` loop.
- **Replace the player for built lessons.** Write a thin player, about 300 to 400 lines, in a new file in `internal/web`. Every graded step calls `activity.Engine` with the lesson source and the unit id. Ungraded steps (grid, story) reuse the existing views. The cursor reuses `lesson_runs` and the existing step calls (`AnswerPlayerRun`, `ContinuePlayerRun`, `db/lesson.go:577-592`).
- **Freeze the old player; do not edit it.** It stays only to open old units from the lesson list until they are archived (P-12). Then most of `lesson.go`, `internal/choices`, and `listenlines.go` go.
- **Three small engine changes:**
  1. **Typed grading accepts a set.** `GradeTyped` against the stripped pointed form and the stored full spelling, best result wins. Review gets the same fix.
  2. **Taught words first.** For the lesson source, rank seen words and this lesson's new words first, then fall back to the lexicon. Any step that shows Hebrew buttons (the later conjugation form pick) is taught-only, as ADR-0032 D1 says.
  3. **An unrecorded second ask.** After a miss, show the meaning and build the same word again with an option that records nothing. Only the first answer counts.

##### 3.4 What to reuse and what to delete

| Reuse as is | Change slightly | Delete (when) |
| --- | --- | --- |
| `activity.Engine` Build/Check, tokens, `GlossesOverlap`, `Pick` | `GradeTyped` takes a set of spellings | `internal/brief`, 1,836 lines (when the story slot ships) |
| `evidence.Recorder`, `answer_voids`, `postAlsoRight` | Lesson source ranks taught words first | `get_lesson_brief`, `put_unit`, `publish_unit`, `unpublish_unit`, `validate_unit`, `hebrew://authoring`, worksheet templates (Spec 2) |
| `units` row, `lesson_items`, `lesson_runs`, card creation on finish, `introNotFinished` | `units.Lesson` gains `type`; new step kind `words` | Today's `WriteModeBlocks` gate (Spec 2) |
| `verb_forms`, `noun_forms`, `lookup`, `upsert_lexemes`, `lexicon.Prepare` | `lexemes` gains `full_spelling`; upsert runs the entry checks | `record_taught_words` (once import works; it skips the word lesson) |
| Story rules: `story.unpointed`, `story.unmapped`, coverage counting, `brief.text_too_long`, `unit.no_markup`, `unit.story_repeat`, `unit.meta_text` | Wrapped in one `units.CheckStory` call with a closed list of rule ids | The other ~50 validator rules and the 30 guidance lines (P-12) |
| `wordCard` and story views | `answer_voids` gains `fixed_at` | Old `lesson.go` engine, `internal/choices`, `listenlines.go`, author and reviser skills (P-12, after archive) |

The MCP surface drops from 28 tools to about 15. Every new check retires an old rule, which meets D13 with room to spare.

##### 3.5 How lexicon entries for class words get created and checked

This is the real authoring problem. The story is ungraded. **The lexicon is the author of every graded key.** 742 of 974 class tokens are not in it (`SPEC-012-audit.md:20`).

1. **The server reads the class itself.** `import_class(fileId)` fetches the sheet's export from Drive with a read-only credential shared on the one class folder. No model copies the tutor's Hebrew, so no model can drop a vowel point on the way in. Rows are stored verbatim, tutor and Brian's text in separate fields, with the file id, its modified time, and a hash per line. A re-import of the same modified time does nothing.
2. **The matcher shrinks the 742.** Many misses are prefixed or inflected forms. For each token, try in order: exact pointed form, skeleton, full spelling, `verb_forms`, `noun_forms`. Then strip one known prefix (ו ה ב כ ל מ ש, and pairs such as וה, וב, של, כש) and try again. Two different candidates means a miss, never a guess.
3. **Only target words need entries.** Code proposes the class's target words: lemmas from that class with no card for Brian. The import reply lists the true misses among them. It does not ask for all 742.
4. **The AI proposes each missing entry through `upsert_lexemes`.** Code checks it before the word can be graded:
   - **Tied to the tutor's spelling.** The entry carries the tutor's token as `full_spelling`. The pointed form, stripped, must equal it, or differ only by added ו or י. That is a cheap, exact check, and it ties every new entry to human-written text.
   - **Fully pointed.** Every letter that needs a vowel has one.
   - **A gloss of 1 to 4 English words,** with no Hebrew and no markup.
   - **Part of speech from the fixed list.** A noun has a gender. A verb has a root, a binyan, and present-tense forms. The root's letters appear in order in the skeleton.
   - **No twin in its lesson.** No two new words in one word lesson have overlapping glosses. If they do, code puts them in different lessons.
5. **An entry that fails is held out.** The word is not offered in any lesson until it passes. The import reply names the failing check.
6. **What code cannot check is whether the English is right.** The grid shows every new word first, the flag loop (3.6) catches the rest, and an open flag stops the word being graded again. A second source for glosses is an opportunity (section 4), not part of the first slice.

##### 3.6 The flag-to-fix loop

Brian's rule: grade it, and give him a way out that erases the miss and flags the item to fix. Most of this exists.

1. **Tap.** "I was right" voids the answer with `also_right`. "This is dumb" voids it with `report`, on that one answer. It does not hide the lesson. Both reuse `answer_voids`.
2. **Suppress at once.** While a flag is open, the engine will not grade that word in that question kind again, in lessons or in review. It deals another kind or skips the word. A bad key cannot hurt him twice. This is one indexed query in `Build`.
3. **List.** `list_flags` returns each open flag with the word, the question kind, the prompt, and the response. The response is the evidence: the gloss he picked, or the Hebrew he typed.
4. **Fix.** There are three kinds of fix:
   - a lexicon edit through `upsert_lexemes` (a gloss, a full spelling);
   - a code change (a synonym pair the engine must never show together, or a grader change), with a regression fixture in the same commit;
   - retiring a class line.
5. **Close with a check.** `close_flag` rebuilds the item against current data. It refuses to close if the same answer would still be graded wrong, or if the same two glosses could still appear together. The other way to close is "not a fault", with a note. The void stays either way, because a tap only voids.
6. **Measure.** W3 already counts `also_right` per 50 attempts. No new logging is needed.

Fixes are batched weekly, as D13's complaints rule says.

##### 3.7 Fixed regression sample lessons, and the same checks on every real lesson

- **Sample worlds in the repo.** Each world is a folder with a small lexicon, a few class lines, a learner's cards, a clock, and a seed. The test runs `Assemble` and compares the frozen lesson JSON and each step's rendered HTML to golden files, regenerated with an update flag. Start with three:
  1. four new nouns, at the beginner tier;
  2. verbs with forms and a full spelling two letters longer than the skeleton;
  3. the edges: overlapping glosses, no story, no clips.

  Add one world per lesson type as each type ships.
- **A robot run on each world.** For every graded step, it builds the item and answers it:
  - the key answer grades right;
  - every wrong button grades wrong;
  - both spellings of a typed key grade right;
  - every Hebrew word shown is taught or new in this lesson;
  - no step needs a clip that is missing.
- **The pre-flight is the same robot, run at Start on the real lesson.** A step that fails is dropped, and the drop is written into the frozen document with its check id, so `get_unit` shows it. If fewer than two words survive with both a meaning check and a write step, the lesson does not open and Today goes to review. This replaces the nightly sweep deferred in BRAIN-001. Every real lesson is checked like a fixture, at the moment it matters.
- **Flag fixtures.** Each code-side fix adds its flag as a fixture: the lexicon rows involved, the prompt, the response, and the expected result.

#### 4. Opportunities

1. **A large deletion is now safe.** D11 already keeps every AI unit off Today. Once Today offers built lessons, the unit-writing tools, the brief, and most validator rules guard nothing. They can go in the same specs that add the new parts. That is about 1,800 lines of `internal/brief`, about 50 rules, about 13 MCP tools, and later most of `lesson.go`.
2. **One engine for lessons and review.** Every fix to buttons, typing, or flags lands once and helps both. The full-spelling fix also stops review grading Brian's tutor-style spelling as a near miss.
3. **The class import makes record-keeping free.** The words of each class reach the app without Brian typing them. The tutor never has to touch anything.
4. **Conjugation is the cheapest second type.** It needs no voice and no new words. It reads `verb_forms`, which the entry checks already require for new verbs. It is one `forms` step plus a typed or taught-only pick.
5. **A second gloss source, if flags show the need.** An open dictionary dump (for example English Wiktionary's Hebrew entries) could cross-check each AI gloss with `GlossesOverlap` before a word is gradable. Add it only if gloss flags pass an agreed line. It is a data file and one check.

#### 5. Risks and Concerns

| Level | Risk | Mitigation |
| --- | --- | --- |
| **High** | **The lexicon is the real author of every key, and hundreds of new entries will be AI-proposed.** A wrong gloss or a wrong vowel on a fresh class word grades Brian wrong on day one. That is the fault that stops him. | Entries are tied to the tutor's own spelling. Failing entries are held out. The grid shows every word before any check. An open flag stops the item at once. A second gloss source is held ready (section 4). |
| **High** | **Two players and two engines live side by side, and the old one never dies.** That is the treadmill BRAIN-001 warned about. | Today offers only built lessons from day one. The old player gets no edits. P-12 has a date: three weeks after the first built lesson, archive old units and delete. |
| Medium | **The matcher maps a token to the wrong word.** Prefix stripping on unpointed text is ambiguous: ה can be the article or a root letter. | Strip only when the whole token does not match. Two candidates means a miss. The import reply shows every match, so a wrong one is visible. A world fixture covers the known traps. |
| Medium | **No voice means a thin lesson.** No Listen on the grid, no check by ear, no dictation, no listening lesson. The S3 listening probe cannot move. | Accepted for the first slice. Every audio step hides itself when a clip is missing, so a voice later turns them on with no code change. |
| Medium | **English buttons from untaught words.** The engine's pool is the whole lexicon. It shows no untaught Hebrew, but it is a different rule from ADR-0032 D1, and elimination gets easier. | Rank taught words and this lesson's new words first for the lesson source. Hebrew buttons stay taught-only. |
| Medium | **Sentence order has many right answers.** Hebrew word order is free, and an order step graded on one line is "several answers are right but only one counts". | Leave sentence steps out of the first slice. Grade dictation on target words once a voice exists. Order stays an ungraded reveal. |
| Low | **A restart or a 30-minute pause kills the current item's token** (`doc.go:26-27`). | The player rebuilds the current item on every load of an ask step. Nothing stores a token. |
| Low | **The Drive credential is a new external part.** | One read-only credential on one folder. If it fails, import fails loudly and the lessons already built keep working. |

#### 6. Recommended Approach

**Decisions on the three delegated questions:**

1. **When:** check the ingredients at class load. Assemble and freeze the lesson at Start. Draw buttons at each ask.
2. **Story:** optional. A story is used only if it passes its checks at Start. Otherwise the step is dropped and the lesson runs.
3. **Document and player:** keep the `units` row and its tables as the spine. Code writes a thin document (a type plus lexeme ids and question kinds). A new thin player runs every graded step on review's engine. The old player is frozen, then deleted.

**The smallest first slice: the word lesson only, from the three most recent classes.**

Steps in the lesson:
1. the grid, without Listen until a voice exists;
2. a meaning check with English buttons, with one unrecorded second ask after a miss;
3. the story, if a passing one is waiting;
4. write it: typed, with both spellings accepted.

Both graded steps carry the two flag buttons. The pre-flight runs at Start. No sentence steps.

**Specs, in order. One initiative, Stage 2, after the gate:**

| Spec | Days | Builds | Deletes |
| --- | --- | --- | --- |
| P-01 ADR | 1 | The decisions above, superseding ADR-0026's lesson shape | — |
| Spec 1: class packs and lexicon | 4 | `class_lines` table; `lexemes.full_spelling`; `import_class` with the server reading Drive; the matcher; entry checks on `upsert_lexemes`; import the last three classes and fill their target-word misses | worksheet templates; `PlayerGuidance`; `record_taught_words` once import is proven |
| Spec 2: word lesson | 5 | `lessons.Assemble` and the pre-flight; `units.Lesson.type` and the `words` step; the thin player; `GradeTyped` on a set; taught-first ranking; second ask; item-level flags, suppression, `list_flags`, `close_flag`, `answer_voids.fixed_at`; three sample worlds; Today offers built lessons | Today's `WriteModeBlocks` gate; `get_lesson_brief`, `put_unit`, `publish_unit`, `unpublish_unit`, `validate_unit`, `hebrew://authoring`; rules that only shape AI units (`unit.drill_share`, `brief.story_length`, `unit.same_shape`, `budget.*`, `recycle.*`, `worksheet.shape`) |
| Spec 3: story slot | 2 | `story_slots`; `get_story_request`, `put_story`; `units.CheckStory` over the closed list of story rules | `internal/brief`; the author and reviser skills |
| P-12 archive | 1 | Archive old units, three weeks after the first built lesson | old `lesson.go` engine, `internal/choices`, `listenlines.go`, the remaining unit rules |

**Timeline:** Brian studies a built word lesson by about day 9 or 10 after Stage 2 opens. Stories join by about day 12. Conjugation is the next type after two weeks of use. Spelling waits for a recorded letters check. Listening waits for a voice. Easy waits for span-graded questions on a story.

**Cut rule hook:** every lesson's unit row carries its type. W2 and W3 can be read per type from existing tables, with no new logging.

#### 7. Questions for Other Specialists

1. **Product Strategist:** Which class words go first: the newest class, or the words that recur most across classes? And is a word lesson with no story an acceptable default for the first week, while the story slot is still being built?
2. **UX Designer:** Where do "I was right" and "this is dumb" sit: on the feedback after every graded answer, or only after a miss? Should a grid card also get a "this looks wrong" tap, even though there is no answer to void?
3. **Data Analyst:** What flag rate, per lesson type and per 50 attempts, should hold a type back, and must it be agreed before building? And should lesson-source answers be left out of every D4 count, not only by the same-day rule?
4. **QA Advocate:** Are the entry checks in 3.5 enough to grade a gloss an AI proposed, or must a second source agree before a new word can be graded? And is "drop the failing step, open the lesson if two words survive" the right pre-flight floor?
5. **Devil's Advocate:** Is even the two-spec slice too much? Would importing the class words and studying them in review alone, with no word lesson, give most of the value in three days instead of ten?

### Summary

**Position:** Keep the database spine (unit row, lesson runs, attempts, voids) and replace the author and the player. Code assembles each lesson at Start from class words and the lexicon. Every graded step runs on review's existing engine. The story is optional and never blocks.

**Recommendation:** Ship the word lesson first (grid, meaning check, optional story, typed write with both spellings), assembled and pre-flighted at Start on `activity.Engine`, with item-level flags that suppress the item at once. Delete the brief, the unit-writing tools, and the shape rules in the same specs.

**Top Risk:** The lexicon, not the story, is the real author of every graded key. Hundreds of new class-word entries will be AI-proposed, and a wrong gloss or vowel grades Brian wrong on day one. Tie each entry to the tutor's own spelling, hold failing entries out, and make an open flag stop the item at once.

**Confidence:** 0.75. Every code claim was read at `6258362`. The less certain parts are how far prefix matching shrinks the 742 misses, whether the entry checks catch enough bad glosses without a second source, and the day estimates.

---
## Phase 4: Dialogue Contributions

### Questions Posed

1. **Product Strategist:** Which class words go first: the newest class, or the words that recur most across classes? Is a word lesson with no story an acceptable default for the first week, while the story slot is still being built?
2. **UX Designer:** Where do "I was right" and "This is dumb" sit: on the feedback after every graded answer, or only after a miss? Should a grid card also get a "this looks wrong" tap, even though there is no answer to void?
3. **Data Analyst:** What flag rate, per lesson type and per 50 attempts, should hold a type back, and must it be agreed before building? Should lesson-source answers be left out of every D4 count, not only by the same-day rule?
4. **QA Advocate:** Are the entry checks in section 3.5 enough to grade a gloss an AI proposed, or must a second source agree before a new word can be graded? Is "drop the failing step, open the lesson if two words survive" the right pre-flight floor?
5. **Devil's Advocate:** Is even the two-spec slice too much? Would importing the class words and studying them in review alone, with no word lesson, give most of the value in three days instead of ten?

### Questions Answered

- **To Devil's Advocate (are the 742 missing tokens real words; can `record_taught_words` record a word with no entry):** Nobody has counted yet. Prefixes and inflected forms make up a share of the 742, so counting comes first. A taught word needs a lexicon entry, because the grid, the meaning check, and the write step all read it. Recording one without an entry would show a card with nothing behind it.
- **To Data Analyst (does the answer log carry a run id, a type, and a soft-delete void):** Partly. Attempts carry the unit id and source, but not a run id. Runs carry their unit, so the type can sit on the unit. Voids are separate rows, so nothing is deleted. The void reason list is closed, so the flag needs one new reason in a small migration.
- **To Product Strategist (can lessons reuse review's typed input and grade word order):** Typed input, yes: a thin lesson player can call the same engine and typed grader. Graded word order, no. Hebrew word order is often free, so order stays an ungraded reveal.

### Tensions Involved In

- **Study first, or build first:** Study first, but only once the typed-answer fix is in place. The backfill is the critical path either way, and a study week in review tests the glosses before a lesson grades from them. Typed review today would mark tutor-style spellings wrong, so that fix comes first.
- **How many lesson types in the first release:** One, the word lesson. The easy lesson comes in only when heavy-review days appear, and it must run on the new thin player, not the old one.
- **Does the AI story earn its place:** Not in the first slice. Tutor lines do the passage job, so my story slot (Spec 3) drops out of the first build and can come later if it is still needed.
- **How much measurement:** Middle ground. Store the type on the unit, add one void reason, read rates from existing tables, add no new logging, and keep no hold-out group. Agree the retire rule before the first build.
- **Who certifies the lexicon:** Code checks plus the tutor's own spelling gate every new entry. The first slice grades only words that pass the tutor-document tier (T0), and a second source is needed only when one exists. No AI-only entry is graded.
- **The review cap inside Today:** The cap is a sort order and a count on the Today query, not a new build. Offer the lesson only once review is under the cap, and have the Start pre-flight read the same count, so it adds no spec.

### Synergies Identified

- **Lexicon backfill as the critical path (shared with Product Strategist, QA Advocate, Data Analyst, Devil's Advocate):** One matcher and one set of entry checks serve every lesson type.
- **Typed spelling fix (shared with UX Designer and QA Advocate):** One change to the typed grader fixes both review and the lesson's write step. It is a trust fix, so it belongs in INIT-038's scope.
- **One flag control (shared with UX Designer, QA Advocate, Data Analyst):** Extend the existing "I was right" button to void, flag, and suspend one item. The flag rate becomes the quality signal with no new logging.
- **Tutor lines as the passage (shared with Product Strategist, UX Designer, Devil's Advocate):** This replaces the AI story in the first slice. It removes an AI dependency and a spec, and it gives each class its own content.
- **Lesson type label on the unit (shared with Product Strategist, Data Analyst):** The type lives in the unit document, so per-type measurement needs no new table.
- **No-audio design (shared with UX Designer):** Audio steps hide themselves when no clip exists, so a voice later turns them on with no code change.
- **Speaking prep page (shared with Product Strategist, Devil's Advocate):** A copyable page of this week's words and tutor sentences can read the same class-line table I am building, so it costs one read-only page and no new spec.

---

### Option Scores

| Dimension | A | B | C | D | Reason |
| --- | --- | --- | --- | --- | --- |
| Learning effect | 4 | 4 | 3 | 3 | Every option gets spaced review (RSCH-003, High). A and B add more of the RSCH-004 strands sooner (the easy fluency pass, more shapes). C and D have one lesson shape until conjugation lands. |
| Trust | 2 | 3 | 4 | 4 | A ships the most graded surfaces, including typed spelling and listening, which is blocked by the voice. B grades new keys with no pre-release evidence. C and D test the glosses in review first, once the typed fix ships. |
| Time to new study (about 2 weeks) | 1 | 2 | 2 | 3 | D starts the backfilled words in review now, and the word lesson lands about 10 days after the gate. C waits a week before building. B builds two types first, and A has five. |
| Build cost and treadmill risk | 1 | 2 | 4 | 4 | C and D build one lesson type in about four specs, and the AI story spec drops out. B adds the easy lesson and conjugation soon after. A is the BRAIN-001 treadmill: five types, each with its own spec and rules. |
| Fixes "feels the same" | 4 | 3 | 2 | 2 | A changes the screens for each type, but only once all five ship. B changes the shape twice, though the easy lesson is only a fallback. C and D fix the content (each class has its own tutor passage) but keep one screen shape, so this is a bet with a "same again" tripwire. |
| Measurable | 2 | 3 | 3 | 4 | One type gives one clean contrast. D's longer pre-gate review gives a before-and-after baseline and more words old enough for the 14-day measure (S2). A has five types and too many confounds for one learner. |

### Final Position

**Preferred Option:** D. Back-fill and study the newest classes' words now in review, then build only the word lesson after the gate.

**Rationale:** D starts spaced review on the new class words now, which is the best-supported finding in the research, and it keeps the first build to one lesson shape with the fewest specs and rules. It also lets the lexicon be tested in review before any lesson grades from it.

**Dissent:** Partial. I agree with D on three conditions:
1. Pre-gate study in review is safe only after the typed-answer fix ships. Today, one extra letter in a tutor-style spelling is graded Hard, and two extra letters are graded Again (`internal/fsrs/typed.go`). Brian named this exact fault as a trust breaker. The fix is a Stage 1 trust fix, so it sits inside INIT-038, but the panel should confirm that under the no-second-initiative rule (A2).
2. Pre-gate backfill cannot run my entry checks, because the full-spelling column and checks arrive in Spec 1. So the backfill should accept an entry only when the tutor's own document shows the word with its gloss next to it (the T0 rule), and each entry should be marked for re-checking when Spec 1 ships.
3. If the easy lesson is added later, it must run on the new thin player, not the old `lesson.go`. Otherwise the P-12 archive date slips.

---
## Information Injection R1: Lessons not tied to a recent class
**Injected at:** After Phase 5, before Phase 6

### Re-Evaluation

**Impact Assessment:** High

**Position Change:** Changed

**Analysis:** The injection breaks two of my assumptions. First, my queue was "taught words with no card, in class order", which lets the newest class drive lessons. Second, the consensus sends a heavy-review day to review only. The graded word pool is the tutor's archive, not the lexicon: under the tier rule (T0 only), a word can be a key only when a class document attests it with its gloss, and every other lexicon entry is AI-proposed (T2) and ungraded. That makes the easy lesson the cheapest always-available lesson, because its keys are words he has already been taught. Conjugation gets no cheaper, because its keys need form-level attestation in the class text.

**Prior Recommendation:** Word lesson first, from the newest three classes, in class order. Easy lesson triggered later. A heavy-review day shows review only.

**Revised Recommendation:** Keep the spine and the thin word-lesson player. Draw new words from a ranked pool of T0 words across every loaded class, not from class order. Put the easy lesson in slice 1 on the same thin player, and make it the heavy-day lesson, so Today always offers something without adding new words on a heavy day.

**Concrete answers (from the architecture lens):**
- **Picker when no recent class exists.** The pool is every lexeme that a loaded class attests with its gloss (T0), that he has no card for, that has no open flag or suspension, and that passes the entry checks. Lexicon words outside any class are never graded. They may appear only as ungraded tap-for-meaning. Order: (1) the number of loaded classes that use the word, most first; (2) the most recent class that contains it, as a tie-break only; (3) line order in that class. Take the first four that do not share a gloss with each other. If fewer than two survive, the picker offers the easy lesson.
- **Backfill scope.** Pre-gate entries stay at the newest three classes, because the pre-gate study week must stay small. The true-miss count (consensus step 1) should run across all loaded classes, roughly the 85 documents or the 10 audited ones, to size the archive pool. After the gate, the rest of the archive enters through import and code-checked entries in batches. None of it is hand-typed.
- **Easy lesson and conjugation.** The easy lesson becomes the cheapest always-available lesson: its passage is tutor text, its keys are taught T0 words, and it needs no AI, no voice, and no new key risk. Conjugation is not cheaper. Its keys are form-level, so only forms a class line actually uses can be graded. That makes its pool smaller than the word pool. Keep it in slice 2.
- **Does the consensus change?** Yes. The four amendments are below.

**New Risks or Opportunities Surfaced:**
- **Risk (pool depth, unmeasured):** nobody knows how many T0, untaught words the archive holds until import runs. Mitigation: add pool depth to the weekly readout. Today falls back to the easy lesson when fewer than two words are ready.
- **Risk (easy lesson coverage early):** in the first two weeks few words are taught, so few tutor passages will pass coverage. Mitigation: a passage is up to three consecutive tutor lines with at least 90% of tokens taught. Unknown tokens show with tap-for-meaning, which is ungraded. If no passage passes, Today says so, plainly.
- **Risk (queue starvation):** the pool empties if the AI does not fill entries ahead of Start. Mitigation: a queue floor (for example, under 12 ready words triggers a batch), and the same code checks as the import.
- **Risk (class order as a filter):** if class order is used as a filter, the newest class drives lessons again. Use it as a tie-break only.
- **Opportunity:** recurrence across classes is a cheap, honest proxy for usefulness, and it costs one count on the class table.
- **Opportunity:** the easy lesson reuses the existing story view and read_pick engine, so it adds little build cost (about 1 to 2 days on top of Spec 2).

**Consensus amendments (specific):**
1. **Heavy-review days.** Replace "On a heavy-review day, Today shows review only" with: "On a heavy-review day, Today offers the easy lesson, with no new words. Review only is the state only when no passage passes, and Today says so."
2. **Easy lesson in slice 1.** Move it from slice 2 into slice 1, on the thin player, after the word lesson. Drop the trigger of three or more heavy days. Its day-14 retire rule is use only.
3. **New-word pool.** Replace "Graded keys: T0 or T1 lexicon words only" with the pool definition above. T1 stays out until a second source exists. Class order is a tie-break, never a filter.
4. **Backfill and readout.** Pre-gate entries stay at the newest three classes. The true-miss count runs over all loaded classes. The weekly readout adds pool depth (ready, untaught T0 words) and the count of heavy days.
5. **Brian's decision (rewritten).** On a heavy day, the panel recommends the easy lesson, so the review load does not rise on the days the cap is already hit. The alternative is a two-word mini word lesson. Brian should confirm which he wants.

**Confidence Adjustment:** Decreased. The architecture holds, but pool depth and passage coverage are unmeasured, and the easy-lesson build estimate is new. The design decision is unchanged in direction.
