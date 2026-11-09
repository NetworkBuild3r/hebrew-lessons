# Assessment Report: Hebrew Web App, Why Lesson 12 Is Broken and How to Stop Shipping Lessons Like It

**Assessment ID:** ASMT-004
**Status:** Complete. Eval MITIGATE (3 of 4 in each of three independent rounds; every flag fixed, no fourth round run). The rounds are in Evidence & Falsification.
**Source:** the Hebrew app at `d1e9490` (local `main` = `origin/main` = live server), plus learner 1's published lessons read over MCP
**Assessed:** 2026-11-08
**Analyst:** AI agent (Cursor, Claude), read-only on code and content
**Recommendation:** Refactor
**Confidence:** 0.85

---

## Bottom Line Up Front (BLUF)

**Recommendation: Refactor. First take units 12 to 34 off Today, then fix six faults in the lesson pipeline (one in how answer buttons are chosen, five in how units are generated and checked), then rebuild the units one at a time as the learner reaches them.** Lesson 12 is not a one-off: all 23 units from there on share one structure whose checkpoints put untaught words, including wrong forms, into the pool the answer buttons are drawn from. INIT-036 does not fix this and should ship after the pace fix, and INIT-032's planned cleanup would rerun the same generator under an inverted "same shape" rule. About 26 hours of new work, plus finishing INIT-031 and INIT-032, gives a pipeline that blocks these shapes on write and flags the ones already published.

---

## Objectivity & Independent Validation

- **Who wrote this.** The same kind of AI agent that built and revised these lessons over MCP wrote this report. That is a conflict of interest. Every load-bearing claim was therefore re-read this session in the cited file, or in a live MCP read, and a fresh judge agent that did not write the report scored it against a fixed oracle.
- **What was read.** The code at `d1e9490`; the working tree differs only in line endings. All 34 published units on learner 1's path, through `get_unit`. Each was passed to `validate_unit` as a dry run under its own scope: unit 1 is `shared`, and units 2 to 34 are `learner:1`. Also read: learner 1's study digest, all 234 of their cards (`get_learner_words`), and lexicon lookups. The live server reports the same commit (`/version`). The sweep output, the card read, the activity sequences and the `/version` response are saved in the agent's store as `scan.json`, `cards_check.out.txt`, `shape_units.out.txt` and `version.out.txt`. The digest read was not saved. No MCP write tool was called. The SQLite database was not opened directly, the app was not run, and the exact four buttons on today's failed step were not captured.
- **Limit of the dry run.** Without a `briefId`, `validate_unit` counts a word as known only if it is a new word of an earlier published *shared* unit (`internal/mcpstore/lookup.go:13-23`). On this path that is unit 1 alone, so learner units 2 to 11 are ignored. Its `coverage.story` findings therefore say little about this learner. They are held at Tier B and do not carry the recommendation. `unit.cloze_context`, `unit.lesson_repeat`, and the structural findings don't depend on what counts as known.
- **Tooling note.** The kit launcher `<KIT>/scripts/dev/run_py.sh` is missing on this machine, as in ASMT-003. The bundled property-check script was run directly with `python -I`. The MCP sweep scripts were written to the agent's own store, not into the source tree. They read the bearer token from `.cursor/mcp.json` without printing it, and they assert that only read tools are called.
- **Falsification.** Five attempts were made to overturn the findings. They are listed in Evidence & Falsification. None reversed the recommendation.

---

## Executive Summary

**Options key**

1. **No action: wait for INIT-032 SPEC-007 and SPEC-008 as planned.** Leaves 23 broken units on Today. When it runs, the rewrite goes through the same reviser loop, the same pace rule, and the inverted `unit.same_shape` rule that did not flag the clones.
2. **Do INIT-036 instead.** It is the wrong target. It changes the "known" rule, the say step, slow audio, and guidance rows C1 and C2. It shares at least four files with these faults: `internal/web/lesson.go` for the say step, `internal/units/rules.go` for the guidance rows, and `internal/mcp/resources/authoring.md` and `.cursor/skills/hebrew-lesson-author/SKILL.md` for the say wording (INIT-036 SPEC-004 lines 31 and 42). The same spec also rewords "the validator", which may add a file under `internal/units`. It changes none of the faulty functions: the word-list builder, the button builder, the cloze and same-shape rules, the pace switch, or the mistake rotation. It does feed one of them. Its SPEC-003 keeps a word in Learning until it has a second day (line 28), and `PaceFor` switches on the Learning count (`internal/brief/build.go:573-575`). Shipped before the pace fix, it would produce more consolidation units, not fewer.
3. **Stop, fix the generator, then rebuild (recommended).** Take units 12 to 34 off Today now. Fix the six faults below, about 26 hours of new specs. Add a standing health check of published units. Then finish INIT-031 and INIT-032 and rewrite units one or two at a time.
4. **Delete the learner path and regenerate everything now.** Fast, but it runs the same faulty generator. Expect the same output.

**What happened in lesson 12** (unit 12, `learner:1`, revision 2, position 11):

- **Repeated lessons.** Lessons 2 and 3 are the same activities in the same order, and both repeat the first nine steps of lesson 1.
- **One-word fill-ins.** Lesson 1 ends with four of them: אֶת, רוֹאֶה, מֵבִיא, לְהוֹצִיא. Each line is only the word itself.
- **Untaught checkpoint sentences.** The last checkpoint holds three tutor-mistake contrasts ("three days ago", "what special thing are you doing", "to prepare a steak you need"). Each carries the authoring note "Correct form first; incorrect form second." in the English the learner sees.
- **Today's struggles.** The digest logged both of them during this lesson, at 21:27 UTC today: גָּר (lives) and אוֹמֵר (says).

**Fault 1: why the buttons included words the learner never had.** For a cloze, the wrong buttons come from the unit's own word list first, same part of speech, then from earlier units (`internal/web/lesson.go:1339-1352`, `:1391-1441`). That list is built from every dictionary word in every line of the unit, including both sides of a contrast drill (`internal/db/put_unit.go:214-252`, `internal/units/doc.go:241-250`). So the steak checkpoint puts לְהָכִין (to prepare) and עוֹשֶׂה (does) into the pool, and its wrong side adds לַעֲשׂוֹת (to do). All three are verbs in the lexicon. Learner 1 has no card at all for לְהָכִין or עוֹשֶׂה, and only one listening card for לַעֲשׂוֹת (one review, 2026-11-06). INIT-032 recorded לַעֲשׂוֹת and לְהָכִין as buttons in this exact gap, "אֲנִי __" (INIT-032 initiative line 11).

**How far it goes** (dry run of today's validator on all 34 published units on the path):

| Units | State | What fails |
| --- | --- | --- |
| 12 to 34 (23 units) | 1 started, 22 untouched | All have 0 new words and 5 lessons. Each has 4 to 17 one-word clozes (128 in total) and one lesson that repeats another. All carry the authoring note in the English. All put at least one wrong-side-only dictionary word (לִקְנוֹת, לַעֲשׂוֹת, מָה, שְׁתֵּי) into their word list. Units 20, 27 and 34 have identical activity sequences, and their stories differ by one phrase: "I live. You work. You say: ___. I say: I live." |
| 1 to 11 (11 units) | finished | Older shapes: clozes with no English (all 11), listen-picks with no translation (4), missing targets (2). Lower priority, but they still feed the earlier-units pool. |

**Faults 2 to 6: why the generator made 23 copies and nothing caught them.** Five faults compounded:

- **Pace.** The brief switches to a no-new-words consolidation unit whenever more than 40 words are still in learning, or when recent accuracy is under 75% with at least one recent review (`internal/brief/build.go:113-116`, `:413-429`).
- **Revising the whole backlog.** The reviser skill revises every untouched unit in order (`.cursor/skills/hebrew-lesson-reviser/SKILL.md:20-26`). Each brief reads the learner as they are now. All 22 untouched units have 0 new words, which is what a consolidation brief asks for. The briefs themselves were not read.
- **Mistake rotation.** A fix-the-mistake pair is skipped only when both previous units used it (`internal/brief/progression.go:213-263`). With a small mistake bank, that cycles two sets in the pattern A A B: set B is at units 12, 15, 18, 21, 24, 27, 30 and 33.
- **An inverted anti-copy rule.** `unit.same_shape` was specified to warn when a unit is at least 80% the same as the previous one (INIT-030 SPEC-004 line 31). The code warns when it is under 80% the same (`internal/units/shape_rules.go:235-248`), and the test pins that (`shape_rules_test.go:145-163`). The authoring guide and the author skill tell the author to vary the sequence "so `unit.same_shape` does not warn" (`internal/mcp/resources/authoring.md:108`, `.cursor/skills/hebrew-lesson-author/SKILL.md:119`). Their rule tables and the rule's doc comment describe the inverted code (`authoring.md:204`, `SKILL.md:215`, `internal/units/doc.go:48`). A copy passes silently, and a varied unit gets warned.
- **Published units are never re-checked.** The one-word-cloze and repeated-lesson rules (commit `67fd2cf`, live) only warn on a published unit (`internal/units/validate_cloze.go:12-20`, `:46-52`). The units already shipped keep playing.

**Capability vs readiness.**

- **Capability averages 2.7 / 5.** The pieces exist: a brief with pace and mistakes, a validator with over 50 rules, a reviser loop, and a choice builder with part-of-speech tiers. Two parts are missing outright: a way to replace a started unit, and any watch on published content.
- **Readiness averages 1.5 / 5.** Wiring flaws stop that machinery from yielding a lesson this learner can answer.

**Harvestable deltas.** These already exist and can be reused rather than rebuilt:

- the INIT-032 cloze rules and English cue, already live;
- `unpublish_unit`, which takes a learner unit off Today and keeps its revisions;
- `rebuildReferencedUnits` in `internal/db/lexemes.go:234-239`, which already re-derives `lesson_items` for every unit that uses a given word (it scans all units);
- `choiceExclude` in `internal/web/lesson.go:1600-1608`, an existing hook for leaving words out of the buttons;
- learner 1's 158 due review cards. That is the real consolidation work while the units are rebuilt.

**Decisions needed from Brian.**

- May units 12 to 34 come off Today now? That is a production content write (gate `production_content_write`). `unpublish_unit` returns each unit to draft and keeps its document, with an audit row (`internal/db/learner_publish.go:213-261`). Bringing one back needs a new brief and `put_unit`. The learner can also hide a whole unit from Today with the web app's "Report a problem" (`internal/web/shell.go:330-343`).
- While the units are being rebuilt, should Today show review only?

---

## Assessment

- **Source.** The Go app's lesson pipeline: the brief (`internal/brief`), the validator (`internal/units`), unit storage and word lists (`internal/db/put_unit.go`, `internal/db/lesson.go`), the choice builder (`internal/web/lesson.go`), the MCP tools (`internal/mcp`), and the two authoring skills (`.cursor/skills/hebrew-lesson-author`, `hebrew-lesson-reviser`). Plus the published content for learner 1.
- **Goal.** Explain why `/lesson/12/1` offered words the learner never had, and decide what fixes both the shipped units and the generator, so no more lessons like it reach Today.
- **What "capability" and "readiness" mean here.** Capability: does the pipeline have the parts a good lesson needs? Readiness: does what it actually ships pass its own rules and match what this learner was taught?
- **Mode.** Brownfield. The seams are clean (brief, validator, storage, player, skills), and every fault sits in a named function.
- **Discovery.** `discover_codebase.py` was not re-run. ASMT-003 ran it on this same commit (`d1e9490`) yesterday, and nothing in the code has changed since.

---

## Persona-Lens Findings

Each lens's headline rests on Tier A evidence read this session.

- **Software Architect.** Every fault has one home:
  - word-list derivation is `deriveLessonItems` (`internal/db/put_unit.go:214-252`);
  - distractor selection is `wideChoices` and `lessonChoices` (`internal/web/lesson.go:1391-1441`, `:1562-1598`);
  - the pace switch is `PaceFor` (`internal/brief/build.go:413-429`);
  - mistake rotation is `fixMistakes` and `skipRightForms` (`internal/brief/progression.go:213-263`);
  - the inverted rule is `sameShapeIssues` (`internal/units/shape_rules.go:235-248`).

  No new service is needed. The contrast drill's wrong side is stored as free text after " — " in `text`. A structured right/wrong pair would make it impossible to mistake the wrong form for a unit word.
- **Data Architect.** The distractor pool has no idea what the learner was taught:
  - `UnitLexemes` is every lexicon token on any line of the unit (`internal/db/lesson.go:209-225`);
  - `EarlierUnitLexemes` is every published unit at a lower position in scope, finished or not (`internal/db/lesson.go:227-251`).

  The cards table already records what the learner has met (234 cards for learner 1). It isn't consulted when the buttons are built.
- **Agentic / Judgment.** The AI author does what its rules reward:
  - It satisfied "struggle words must be reused" by appending bare `shadow` + `cloze` pairs (unit 12, lesson 1, steps 10 to 17).
  - It satisfied `brief.fix_contrast` by pasting the tutor's whole sentence, with words the learner never studied.
  - Nothing warned it against copying the skeleton. `unit.same_shape` runs only with a brief, and when it runs it warns on a varied unit, not a copied one. Whether the author ever saw that warning is not known (Tier B).
  - It wrote its own instruction ("Correct form first; incorrect form second.") into learner-facing English because no rule forbids it.

  The fix is in the rules and the brief, not in more prompt text.
- **Capability-Parity / Reuse.** Coverage by existing initiatives:
  - **INIT-032** names this unit and ships part of the fix: cloze rules and the English cue are live in `67fd2cf`. Its SPEC-005 (`accept` list), SPEC-007 (list the bad units) and SPEC-008 (rewrite them) are planned. SPEC-008 is blocked on INIT-031/SPEC-004 because unit 12 is started (SPEC-008 line 18 `blocked_by`, line 32).
  - **INIT-031** (planned) would let the agent replace a started unit.
  - **INIT-036** edits the "known" rule, the say step, slow audio, and guidance rows C1 and C2. It shares `internal/web/lesson.go` (say step, `:52-55`, `:365`, `:438`), `internal/units/rules.go` (`:234-235`), `internal/mcp/resources/authoring.md` and the author skill (SPEC-004 lines 31 and 42) with these faults' areas. It changes none of the faulty functions. Its stricter Learning state (SPEC-003 line 28) raises the count `PaceFor` reads (`internal/brief/build.go:573-575`), so it should ship after SPEC-G.

  What no initiative covers:
  - wrong forms in the word list;
  - untaught distractors;
  - the inverted same-shape rule;
  - a graded target that was never taught;
  - authoring notes in learner-facing English;
  - revising a whole backlog from one learner snapshot.
- **Security & Compliance.** No new trust boundary. The cleanup uses the existing `unpublish_unit` (an admin must pass `learnerId`) and `put_unit`. It is a production content write (gate `production_content_write`). Unpublish keeps the unit as a draft and writes an audit row with the caller (`internal/db/learner_publish.go:213-261`). A rewrite keeps the old revision for rollback (INIT-032 SPEC-008 line 34).
- **Platform / Cloud.** No infrastructure change. The live build is the same commit as the checkout, so a fix ships with the normal deploy. Rebuilding audio for rewritten units uses voice credits, and the balance was not checked.
- **Day-2 Operations.** Nothing watches shipped content:
  - a published unit is checked only when it is saved;
  - new rules downgrade to warnings for it;
  - no job or screen lists published units that today's rules block.

  The read-only sweep in this assessment (34 units, about 90 seconds) could run as a daily check.

**Conflicts between lenses.**

- **Pace vs progress.** The Data lens supports consolidating while recall retention is 59% (digest, 2026-11-07). The Agentic lens shows consolidation becoming 23 identical units. Resolution: consolidation means review sessions plus at most one consolidation unit queued ahead, not a whole path of them.
- **Fix it now vs fix it right.** Rewriting the content today through the current reviser would recreate the clones. Resolution: take the units off Today now, which is reversible and immediate, and rewrite only after the generator faults are fixed.

---

## Capability & Readiness Scoring Matrix

Scored 1 to 5. Code paths are under `internal/` unless they start with `.cursor/`.

**Capability: does the pipeline have the parts**

| Dimension | Score | Evidence |
| --- | --- | --- |
| A brief that plans pace, struggle words and mistakes | 4 | `brief/build.go:413-429`, `brief/progression.go:213-263` |
| A validator that blocks bad shapes on new writes | 4 | Over 50 rule ids (`units/rules.go:47-104`); cloze and repeat rules in `units/validate_cloze.go` |
| Believable distractors (same part of speech, no shared gloss) | 3 | `web/lesson.go:1391-1441` |
| A reviser loop for untouched units | 3 | `.cursor/skills/hebrew-lesson-reviser/SKILL.md:20-26` |
| Replacing a started unit | 1 | Refused (`mcp/learner_data.go:32`, `mcp/tool_publish_unit.go:200`). Retiring one already works: `unpublish_unit` has no started check (`db/learner_publish.go:239-251`) |
| Watching published content against current rules | 1 | Published mode warns only (`units/validate_cloze.go:19-20`, `:51-52`); no sweep or job |
| **Average** | **2.7** | 16 / 6 |

**Readiness: does what ships match the rules and the learner**

| Dimension | Score | Evidence |
| --- | --- | --- |
| Shipped units pass today's rules | 1 | 34 of 34 have at least 7 blocks in the dry run; 23 have one-word clozes and a repeated lesson |
| Buttons are words the learner was taught | 1 | Pool is unit tokens plus earlier units, with no learner check (`db/lesson.go:209-251`); learner 1 has no card for לְהָכִין or עוֹשֶׂה |
| A wrong form is never treated as a unit word | 1 | Both sides of a contrast are tokenized (`db/put_unit.go:214-252`); 4 wrong-side-only dictionary words in 23 units |
| Units progress instead of cloning | 1 | 23 consolidation units; inverted `unit.same_shape` (`units/shape_rules.go:235-248`) |
| Learner-facing text is clean | 2 | Authoring note in the English of 23 units; no rule against it |
| Mistakes rotate | 3 | Rotation works as coded, but cycles A A B with a small bank (`brief/progression.go:248-263`) |
| **Average** | **1.5** | 9 / 6 |

---

## Technical Component Analysis

One table covers every component this assessment would touch or keep.

| Component | Nature | Home | Provided / Adapt / Build | Change |
| --- | --- | --- | --- | --- |
| `internal/db/put_unit.go` `deriveLessonItems` | Deterministic | Data | Adapt | Skip the wrong side of a contrast drill. Rebuild `lesson_items` for every unit with the existing rebuild |
| `internal/db/lesson.go` `UnitLexemes`, `EarlierUnitLexemes` | Deterministic (query) | Data | Adapt | Add a learner-taught variant: the unit's own new words, plus words the learner has a card for, plus earlier units the learner finished |
| `internal/web/lesson.go` `wideChoices`, `lessonChoices`, `choicePool` | Deterministic | Service | Adapt | Draw buttons only from the learner-taught pool. Fall back to the existing reveal when fewer than three remain |
| `internal/units/shape_rules.go` `sameShapeIssues` + test + guide | Deterministic | Service | Adapt | Fix the inversion (warn at 80% or more the same). Add a block when a story matches an earlier unit's story. Correct `authoring.md:108`, `:204`, `rules.go:187`, `doc.go:48`, and the author skill's lines 119 and 215 |
| `internal/units` new rules | Deterministic | Service | Build | `unit.target_untaught`: a graded target must be new here, taught earlier, or in the brief. `unit.meta_text`: no authoring notes in learner English. Coverage on checkpoint and contrast lines, not only stories |
| `internal/units/validate_cloze.go` published mode | Deterministic | Service | Provided | Keep as is: warn-on-published lets old units keep playing. The sweep below lists them |
| `internal/brief/build.go` `PaceFor` | Deterministic | Service | Adapt | At most one consolidation unit queued ahead. Otherwise point the learner to review |
| `internal/brief/progression.go` `fixMistakes` | Deterministic | Service | Adapt | Don't reuse a pair within three units. No fix checkpoint when no fresh pair is left. Build contrast lines from comfortable words, not the tutor's sentence |
| `.cursor/skills/hebrew-lesson-reviser` loop | Judgment (agent) | Agent skill | Adapt | Revise only the next one or two untouched units. Retire the rest with `unpublish_unit` instead of revising them |
| `internal/mcp` revise lock (INIT-031) | Deterministic | MCP | Adapt (planned) | Let the agent replace a started unit's lessons, as INIT-031 SPEC-002 plans |
| Published-unit health check | Deterministic | MCP + ops | Build | Daily read-only sweep (`get_unit` + `validate_unit`); `get_context` shows each unit's block count; Today skips an untouched learner unit with blocks |
| `unpublish_unit` | Deterministic | MCP | Provided | Use now on units 12 to 34 |
| INIT-032 SPEC-005 `accept` list | Deterministic | Service + web | Adapt (planned) | Keep; complements the learner-taught pool |

---

## Recommendation

**Refactor**, option 3 in the options key. Every fault is deterministic code or a rule in a known function, so it belongs in the existing services:

- **Agentify is ruled out.** The AI author is already the judgment layer, and it did what the rules rewarded.
- **Hybrid adds nothing.** No new judgment component is needed.
- **No action is ruled out.** Today's validator blocks all 23 forward units, and the planned rewrite would run the same faulty generator.

**Order of work:**

1. **Now (minutes; units stay as drafts).** Take units 12 to 34 off Today with `unpublish_unit`. The learner does review in the meantime: 158 cards are due.
2. **Fix the generator** (new SPEC-B to SPEC-G below), so a rewrite can't recreate the clones or the untaught buttons. Ship SPEC-G, the pace fix, before INIT-036 SPEC-003.
3. **Unblock started units** (INIT-031 SPEC-001 to SPEC-004) and add the `accept` list (INIT-032 SPEC-005).
4. **Rebuild one or two units at a time** as the learner reaches them (INIT-032 SPEC-008, rescoped). Each gets a fresh brief from the learner's state at that point.
5. **Keep watching** (SPEC-H): a daily sweep and a block count in `get_context`.

**How confidence was set.** Every load-bearing claim is Tier A: read at the cited line, or seen in a live MCP read. All five falsification tests narrowed findings without reversing the recommendation, which puts confidence at 0.85 or above under the skill's rule. It is held at 0.85 because two things were not observed directly: the exact four buttons on the step the learner just failed, and who generated the template revisions. Neither changes the direction.

**Open unknowns.**

- **The exact buttons on today's failed step.** The seed depends on the run id. The pool and two observed buttons are known.
- **Who wrote revisions 2 to 4 of units 12 to 34.** The note text is not in this repo or in Cursor's chat history, so it likely came from another MCP client session.
- **Units 1 to 11 coverage findings** are Tier B. They need a brief-based check before anyone rewrites a finished unit.

---

## Evidence & Falsification

### Claims-Evidence Ledger

| # | Claim | Tier | Load-bearing | Evidence |
| --- | --- | --- | --- | --- |
| 1 | Unit 12 revision 2: lessons 2 and 3 are identical, lesson 1 ends with four one-word clozes, and the last checkpoint has three contrasts with authoring notes in the English | A | Yes | `get_unit` 12 (live) |
| 2 | Today's validator blocks unit 12 with 8 issues (4 `unit.cloze_context`, 1 `unit.lesson_repeat`, 3 `coverage.story`) | A for the first two, B for coverage | Yes (first two) | `validate_unit` dry run, position 11, `learner:1` |
| 3 | Published units only get warnings for the cloze and repeat rules | A | Yes | `internal/units/validate_cloze.go:12-20`, `:46-52` |
| 4 | Every lexicon token on every line, including both sides of a contrast, enters the unit's `lesson_items` | A | Yes | `internal/db/put_unit.go:214-252`; `internal/units/doc.go:241-250` returns the whole `text` for a pattern |
| 5 | Cloze buttons come from that unit's `lesson_items` first, same part of speech, then from every earlier published unit in scope, with no check of what the learner was taught | A | Yes | `internal/web/lesson.go:1339-1352`, `:1391-1441`; `internal/db/lesson.go:209-251` |
| 6 | לְהָכִין and עוֹשֶׂה are lexicon verbs with no card for learner 1; לַעֲשׂוֹת has one listening card (one review) and appears in unit 12 only on the wrong side | A | Yes | `lookup` (both `pos: verb`); `get_learner_words` (234 cards, saved as `cards_check.out.txt`); `get_unit` 12 |
| 7 | לַעֲשׂוֹת and לְהָכִין were seen as buttons in "אֲנִי __" in this unit | A | Supporting | INIT-032 initiative line 11 |
| 8 | The learner struggled on גָּר and אוֹמֵר in this lesson today | A | Supporting | `get_study_digest`: `lastSeen` 2026-11-08 21:27:14Z and 21:27:02Z |
| 9 | Units 12 to 34 (23 units) have 0 new words, 5 lessons, 4 to 17 one-word clozes (128 in total), one repeated lesson each, and the authoring note in the English | A | Yes | Sweep of `get_unit` + `validate_unit` over all 34 units |
| 10 | Units 20, 27 and 34 have identical activity sequences and one-phrase story variants | A | Supporting | `get_unit` 20, 27, 34, saved as `shape_units.out.txt` |
| 11 | `unit.same_shape` is inverted against its spec, and the guide tells the author the opposite of what the code does | A | Yes | INIT-030 SPEC-004 line 31; `internal/units/shape_rules.go:235-248`; `shape_rules_test.go:145-163`; `internal/mcp/resources/authoring.md:108`, `:204`; `internal/units/rules.go:187`; `internal/units/doc.go:48`; `.cursor/skills/hebrew-lesson-author/SKILL.md:119`, `:215` |
| 12 | Consolidation means zero new words when learning > 40 or accuracy < 0.75 | A | Yes | `internal/brief/build.go:113-116`, `:413-429` |
| 13 | The reviser revises every untouched unit in order; a started unit cannot be revised | A | Yes | `.cursor/skills/hebrew-lesson-reviser/SKILL.md:10`, `:20-26`; `internal/mcp/learner_data.go:32`; `internal/mcp/tool_publish_unit.go:200` |
| 14 | Fix-mistake rotation skips a pair only when both previous units used it; observed A A B cycle | A | Yes | `internal/brief/progression.go:213-263`; sweep (set B at units 12, 15, 18, 21, 24, 27, 30, 33) |
| 15 | All 22 untouched units were revised from the same learner snapshot | B (inferred) | No | Mechanism in claims 12 and 13; revision timestamps were not read |
| 16 | INIT-036 changes none of the faulty functions. It shares at least four files: `web/lesson.go` (say step), `units/rules.go` (guidance C1, C2), and `mcp/resources/authoring.md` and the author skill (say wording). Its stricter Learning state raises the count `PaceFor` reads | A | Yes | INIT-036 SPEC-004:29, :31 and :42, SPEC-005:26 and SPEC-011:32 are its edits in those files; SPEC-003:28; `internal/brief/build.go:573-575`. No INIT-036 file references `put_unit.go`, the choice builder, `validate_cloze.go`, `shape_rules.go` or `brief/progression.go` |
| 17 | INIT-032 SPEC-007 and SPEC-008 are planned, and SPEC-008 waits on INIT-031/SPEC-004 | A | Yes | INIT-032 spec table; SPEC-008 line 18 (`blocked_by`) and line 32 |
| 18 | Live server runs `d1e9490` | A | Yes | `/version` returned `{"commit":"d1e9490c814e…","builtAt":"2026-11-07T23:55:32Z"}` this session, saved as `version.out.txt` |
| 19 | Without a brief, the dry run counts as known only the new words of earlier published shared units | A | Yes (it bounds claim 2) | `internal/mcpstore/lookup.go:13-23` |
| 20 | `unpublish_unit` returns a unit to draft, keeps its document, and writes an audit row | A | Yes (SPEC-A) | `internal/db/learner_publish.go:213-261`; tool description |

### Falsification

1. **"The learner forgot words they had been taught."** Falsified for the buttons. There is no card at all for לְהָכִין or עוֹשֶׂה, and לַעֲשׂוֹת reaches the unit only from the wrong side of a contrast (claims 4 to 6). It does narrow the finding: גָּר and אוֹמֵר themselves were taught. גָּר has 21 reading reviews, and אוֹמֵר is in relearning.
2. **"INIT-032's live rules already fixed it."** Falsified. Published units only warn (claim 3), so unit 12 still plays. The live rules don't cover untaught buttons, wrong-side words, authoring notes, or cloning.
3. **"The dry-run counts are inflated."** Partly true, so the report narrowed. Without a brief, `coverage.story` counts only shared-unit new words as known (claim 19), so it is held at Tier B. The recommendation rests on `unit.cloze_context`, `unit.lesson_repeat`, and the structural reads, which don't depend on what the learner knows.
4. **"INIT-036 is the fix."** Falsified (claim 16). It shares at least four files with these areas but changes none of the faulty functions. Shipped first, it would make the pace fault worse, because more words would sit in Learning.
5. **"Just rewrite the content and move on."** Falsified as sufficient. The rewrite would run `PaceFor` against the same backlog and `fixMistakes` against the same small bank. It would also run under an inverted `unit.same_shape`, so the faults that produced the clones remain (claims 11 to 14).

### Independent evaluation

See `eval/ASMT-004-hebrew-web-app-eval-report.md`.

- **Property checks.** 16 passed, 0 failed (`eval/ASMT-004-hebrew-web-app-property-checks.json`).
- **Round 1.** A fresh read-only judge rated it 3 of 4 (MITIGATE). It checked 36 claims, found 31 fully supported, and found nothing fabricated. It flagged:
  - an absolute about INIT-036, which in fact shares files with these faults' areas;
  - two off-by-one SPEC-008 line numbers;
  - the dry-run limit, which counts shared units only;
  - a five-versus-six fault count;
  - one Tier B inference stated as fact;
  - a long BLUF;
  - a premature status line.

  Each was checked against the files and fixed.
- **Round 2.** A second fresh judge rated it 3 of 4 (MITIGATE). It checked 45 claims and fully supported 36 of the 40 it could check. Five rested on live reads it could not repeat, and nothing was fabricated. All 12 acceptance criteria passed. It flagged:
  - INIT-036 shares at least three files, not two, and its stricter Learning state feeds `PaceFor`;
  - the same-shape text also sits in the author skill and `doc.go`;
  - Report a problem hides a whole unit (`shell.go:342-343`);
  - unit 1 was validated as `shared`;
  - the BLUF ran long and kept two absolutes;
  - a weak same-shape risk row;
  - retiring a started unit already works;
  - the card and version reads were not saved.

  Each was checked against the files and fixed, and the two reads are now saved next to `scan.json`.
- **Round 3.** A third fresh judge rated it 3 of 4 (MITIGATE). It checked 52 claims and fully supported 43 of the 46 it could check, with nothing fabricated. 11 of 12 acceptance criteria passed. AC1 failed narrowly on one count: INIT-036 shares at least four files, not three, because SPEC-004 lines 31 and 42 also edit the author skill. Two wording overreaches (the BLUF and option 1) and the "placement" label on a card were also flagged. All were fixed, and the activity-sequence read is now saved. No fourth round was run.
- **Verdict.** MITIGATE. No judge found a fabricated claim or a wrong code citation, and none disputed the recommendation or the confidence. Each round's flags were confirmed against the files and fixed.

---

## Spec Skeleton

Grouped by domain. A new initiative carries SPEC-A to SPEC-H, and existing INIT-031 and INIT-032 specs are reused where they fit. Sizes follow the one-domain, 2 to 6 hour rule.

**Ops (first, gated)**

- **SPEC-A · ops: take units 12 to 34 off Today.** `unpublish_unit` with `learnerId` 1 for each. Write the list with ids, positions and states. Gate `production_content_write`, which needs Brian's approval. Units stay as drafts with an audit row. 1h.

**Docs**

- **SPEC-B · docs: ADR, a lesson asks only about words the learner was taught, and a wrong form is never a unit word.** Covers the learner-taught pool, contrast structure, the meta-text ban, and "consolidation is review plus at most one unit". 2h.

**Data**

- **SPEC-C · data: the word list skips the wrong side of a contrast.** Change `deriveLessonItems` to skip the wrong side (after " — " on a pattern line, or a structured `wrong` field). Rebuild `lesson_items` for every unit. Test: לַעֲשׂוֹת is not in unit 12's list. 3h.

**Service**

- **SPEC-D · service: buttons come only from taught words.** Unit tier: this unit's new words plus words the learner has a card for. Earlier tier: units the learner finished. Fewer than three gives the existing reveal. Test across 50 seeds that no button lacks a learner card or a new-word intro. 4h.
- **SPEC-E · content: fix `unit.same_shape`.** Warn at 80% or more the same, and block a story that matches an earlier unit's story. Fix the test, the catalog row (`rules.go:187`), the doc comment (`doc.go:48`), and the guide and author-skill lines (`authoring.md:108`, `:204`; `hebrew-lesson-author/SKILL.md:119`, `:215`). 2h.
- **SPEC-F · content: `unit.target_untaught`, `unit.meta_text`, and coverage on checkpoint lines.** All three block on new writes. 5h.
- **SPEC-G · service + skill: pace and mistakes over a backlog.**
  - Pace: at most one queued consolidation unit.
  - Mistakes: no pair reused within three units; no fix checkpoint with no fresh pair; contrast lines built from comfortable words.
  - Reviser: revises the next one or two units only.

  5h.

**MCP / Ops**

- **SPEC-H · mcp: published-unit health.** `get_context` returns each unit's block count under write-mode rules. Today skips an untouched learner unit with blocks. A daily read-only sweep writes the list. 4h.

**Reused**

- INIT-032 SPEC-005 (`accept` list).
- INIT-031 SPEC-001 to SPEC-004 (replace a started unit).
- INIT-032 SPEC-008, rescoped: rebuild units one or two at a time after SPEC-C to SPEC-G ship.

**Total new:** about 26h. **Critical path:** SPEC-B → SPEC-C → SPEC-D → SPEC-H, about 13h. SPEC-A can run today, independent of everything else.

---

## Risks

| Risk | Probability | Impact | Mitigation |
| --- | --- | --- | --- |
| Taking 23 units off Today leaves no lessons for a while | H | L | 158 review cards are due; rebuild the next unit first |
| A learner-taught pool is too small for four buttons early on | M | L | Existing reveal fallback below three buttons (`web/lesson.go:1437-1439`) |
| The fixed `unit.same_shape` still lets a copy through | M | M | It only warns, and only with a brief, so it never blocks a save. The story-match block in SPEC-E is what stops a copy, and SPEC-H's sweep must compare shapes itself, since it runs without a brief |
| INIT-036 ships before SPEC-G | M | M | Its stricter Learning state raises the count `PaceFor` reads, so more briefs would consolidate. Ship SPEC-G first, or with it |
| SPEC-E and INIT-036 SPEC-004 both edit `authoring.md` and the author skill | M | L | Land them in sequence and re-read both files' rule tables after the second |
| The rewrite still needs INIT-031 for unit 12 | M | M | Unpublish unit 12 and write a fresh unit at its position instead |
| Voice credits for re-generated audio | M | M | Rebuild one or two units at a time; check the balance first |

---

## Next Step

Handed off on 2026-11-08 as [INIT-037](../../initiatives/INIT-037-taught-words-only/INIT-037-taught-words-only-initiative.md), with `source: ASMT-004`. Brian approved SPEC-A the same day. Units 12 to 34 were unpublished with `unpublish_unit` (learner 1), and all 23 returned `status: draft` (INIT-037 SPEC-001).
