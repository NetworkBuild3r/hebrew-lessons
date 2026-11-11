# Decision Document: Is the Hebrew app built right, and what could we do better?

**Topic:** Is the Hebrew app built right, and what could we do better?
**Session ID:** BRAIN-001
**Session type:** Quick Cross-Domain EPIC (Quick depth)
**Date:** 2026-11-09
**Status:** Recommendation
**Participants:** Technical Architect, QA Advocate, Product Strategist (lead), Data Analyst, Devil's Advocate
**Decision owner:** Brian Nelson (owner, and learner 1)
**Recommendation:** Option B. Stabilize trust first, then re-center the app on the tutor's lessons, in two stages, with Option D as the written fallback.
**Confidence:** Medium-High

---

## Executive Summary

**Short answer: the learning core is built right, but the content side and its center, the measurement, and the process are built wrong. Recommendation: Option B, which stabilizes trust first and then re-centers the app on the tutor's lessons in two stages, with Option D (freeze and go simpler) written down as the fallback.** The answer log, FSRS scheduling, the pointed lexicon, the learner model rebuilt from logs, the audio cache, and the MCP server are sound, and none of the defects Brian hit live there. The content side has the wrong shape and the wrong center: an AI writes free-form units from a frequency-band syllabus, a validator that can only list bad patterns already seen gates them, and the tutor's lessons, which are Brian's real course, never reach the app. The measurement asks for more proof than the app collects, so Progress tells him he knows almost nothing and the same counts stop new lessons. The process turns each complaint into a new initiative and a new rule, which produced 34 initiatives and 258 commits against about three hours of tracked study in a month. All five specialists preferred Option B.

- **Stage 1, trust and measurement.** This is one initiative with a closed list, finished within 21 days. It voids wrong answers the app cannot prove wrong, adds a "my answer was also right" tap, stops grading word-choice blanks, turns listening back on with word clips, makes Progress one honest ladder frozen for 90 days, stops auto-publishing AI-written units, and freezes the side features.
- **A measured gate, then Stage 2.** Stage 2 opens only when Stage 1's exit is met, an agent's audit of the tutor's Drive folder passes, Brian has ruled on private reuse of the tutor's text, and enough evidence exists under the new rules. Stage 2 imports each class into a class pack, grades only answers with a trusted key, makes the AI a supplementary tutor, and retires the unit pipeline by archiving units, never deleting them.
- **The process guardrails are part of the decision.** One initiative is in flight at a time, each with Brian's explicit go. Complaints go on a weekly list after a 24-hour cooling-off. A new rule retires an old one, deletion comes first, and no agent assesses its own work.
- **Option D is the fallback.** Stop building and make the app review-only if Stage 1 runs past 21 days, a second initiative opens before Stage 1 closes, study time has not reached about 120 minutes a week by week 4, or the monthly listening probe is flat for two months.
- **Options and accepted risks.** The panel scored four options: B 122 of 150, D 102, C 69, and A 60. The plan accepts fewer graded question types for a while, one more announced move in the Progress numbers, and no new lessons until Stage 2.

---

## Background & Context

### Problem Statement

Brian asked whether the app is built right at all: its lessons, review, progress, placement, audio, and the way lessons are produced. He wants a short, ranked list of changes that would make the app teach Hebrew better and stop bad lessons reaching Today, with an honest view of what to keep, change, or stop.

In his Phase 2 answers he defined the product himself: "I have a tutor. I want a tool to help me relearn our lessons but actually retain what I learned through science backed research," and "AI is to be my supplemental Tutor. knowing where I am, where I struggle and helping me tune my lessons." The app serves only him. His goal for the next 6 to 12 months is a simple spoken conversation: understanding Hebrew at normal speed and answering. He named trust, not time, as what stops him: when a lesson looks wrong he skips it or stops, and lately that happens often. If sessions are worth it, he will give 20 to 30 minutes a day.

### Trigger

On 2026-11-08 Brian opened lesson 12 and found answer buttons for words he had never been taught. ASMT-004 then found 23 learner units (12 to 34) cloned from one broken structure: no new words, a lesson that repeats another, 128 one-word clozes, an authoring note shown in the learner's English, and wrong answer buttons drawn from untaught words and from the wrong side of tutor-mistake contrasts. INIT-036 ("known means remembered on a later day") and INIT-037 ("a lesson asks only about words you were taught") fixed those faults and shipped on 2026-11-09. Brian then asked whether the app is built right at all.

### Key Evidence

Each finding below was read in code, in project records, or in learner 1's live data during the session. The source is named with each one.

**What is built right: the learning core**

- **Brian's history does not depend on units.** Attempts and exposures are append-only, and their `unit_id` is set to null rather than deleted when a unit goes (`migrations/006_evidence.sql`). Learner state is computed from the logs by pure functions, and a rule change rebuilds derived history from `review_log` (ADR-0031 D5). Review (`internal/session`, `internal/activity`) does not depend on units. Source: Technical Architect, confirmed in the Phase 4 dialogue.
- **One caveat shapes the migration.** `lesson_runs` and `lesson_items` are deleted along with their unit (`migrations/001_init.sql`), so units must be archived, never deleted. Source: Technical Architect.

**What is built wrong: the content side and its center**

- **New words come from frequency bands, not from the tutor.** The lesson brief takes unseen words from the lowest frequency band that still has any, and it never reads tutor-taught words; its only "taught" mentions concern grammar patterns. Source: `internal/brief/build.go`, read by the parent before the dialogue, and by the Technical Architect and Product Strategist. The design document that set this shape (`docs/design/learner-without-a-tutor.md`) plans a default course for a learner without a tutor.
- **The tutor's material has never been loaded.** The class backfill runbook still reads "Backfill under way: not yet." Source: `docs/operations/class-data-through-mcp.md`, line 11. The INIT-016 SPEC-004 completion summary says "Nothing was loaded," and the live study digest shows no taught words in the last 14 days and no tutor corrections in the last 30. About 85 class documents from Brian's Preply classes exist in a Drive folder (INIT-012, Source line). Source: Product Strategist and Technical Architect.
- **The very first lesson has three right buttons.** Unit 1, lesson 2 asks "אַתָּה __ פֹּה?" ("Do you ___ here?") with the target גָּר ("live") and no English cue. A recorded run offered עוֹבֵד, לוֹמֵד, גָּר, and אֲנִי, so "work", "study", and "live" all make correct Hebrew, and "אַתָּה עוֹבֵד פֹּה?" is itself a line in lesson 6 of the same unit. Source: the INIT-037 SPEC-011 verification (`specs/completion-summaries/SPEC-011-buttons-unit-1.json`), cited by the QA Advocate. The validator never sees these buttons, because the player picks them at play time and prefers words of the same part of speech (`internal/web/lesson.go`, `internal/choices/pick.go`).
- **Bad questions write false evidence, and nothing undoes it.** Today's struggle words גָּר and אוֹמֵר were both last missed at 21:27 UTC on 2026-11-08, inside the cloned lesson 12. Struggle words feed the next brief, and "Report a problem" hides a unit but voids no recorded answer, lapse, or confusion (`internal/db/learner_publish.go`). Source: QA Advocate.

**What is built wrong: the measurement**

- **The "known" rule needs more proof than the app collects.** A skill counts as known only when its card has at least 3 checked successes, the newest 3 answers are right, successes fall on at least 2 study days, and the recall probability right now is at least 0.85. Source: `internal/learner/state.go:56-63,157-161`, read by the parent; the full list of conditions is from the Data Analyst. The app has asked each word about twice per skill, and the recall condition decays with the calendar.
- **21 of 22 spelled words were never wrong, yet Progress shows 4 known.** Of the 22 words Brian has ever spelled, he has misspelled one. The same Progress row rates spelling "strong" at 97% right over 33 answers. Source: live `get_learner_model` read, Data Analyst. This is his complaint in his own words: "I know how to spell words ... go to progress and says you don't know any words."
- **56 of the 161 due cards are listening cards that review cannot ask.** The review mix drops due listening cards whose word has no clip, and they stay due. Source: `internal/session/mix.go:263-281`, read by the parent, with the live count from the Data Analyst. No lexicon word has a word-level clip, so listening, the goal skill, is effectively switched off in review, while the brief gates new lessons on the full 161 ("Review first").
- **The same strict counts lock the lesson pipeline.** The brief forces "consolidate, no new words" when more than 40 words are learning (88 today) or fewer than 75% of the last 50 reviews were rated Good or Easy (48% today), and it sets the tier from words known in every skill (2 today). Source: `internal/brief/build.go` and `internal/brief/progression.go`, Data Analyst.

**What is built wrong: the process**

- **34 initiatives, 258 commits, and about three hours of study in a month.** Since the first commit on 2026-10-07 the project has produced 34 initiatives, 291 specs, 31 ADRs, and 258 commits. The weekly reports show 0, 35, 57, and 64 minutes of study, and the digest shows 45 minutes in the last seven days, about three hours of tracked study in all. Tracked time leaves out tutor sessions. Source: `git log`, `get_progress`, and `get_study_digest`, Devil's Advocate.
- **Fixes cause the next complaint.** INIT-030 opened with Brian's words "Our lesson plans are the same over and over." Its consolidation pace rule and `unit.same_shape` check produced the 23 cloned units (ASMT-004), and in Phase 2 Brian said again that every lesson is the same. The inverted `unit.same_shape` rule was pinned by its own test (`internal/units/shape_rules_test.go:145-163`). Source: Devil's Advocate and Technical Architect.
- **Written rules are skipped, and agents grade their own work.** Personal mistakes were recorded despite the runbook's "load nothing" gate. INIT-026 was marked complete while its report says "Not committed. Not pushed. Not deployed." ASMT-004 states that it was written by the same kind of agent that built the lessons. Source: Devil's Advocate.
- **One rule contract lives in five prose copies, and they drift.** The rule catalog, a doc comment, the authoring guide, the author skill, and the MCP tool descriptions each restate the rules. For example, the `get_context` description still describes `knownLexemes` under an older rule (`internal/mcp/tools.go:628`). Source: Technical Architect.

**Evidence that shapes the fix**

- **The earlier "write original lines" rule.** INIT-012 SPEC-008 (line 39) told authors to "write original lines … never copy a learner's typed answer," because "the class docs hold real errors." That is why reusing the tutor's text is a decision for Brian, not an assumption, and why the tutor's lines and Brian's typed lines must be kept apart. Source: read by the parent before the dialogue.

### Constraints

- **Budget:** No hard limit is stated. ElevenLabs audio and agent time both cost money.
- **Timeline:** Stage 1 must finish within 21 days. Brian can give 20 to 30 minutes a day if the sessions are worth it.
- **People:** There is one developer-owner, who is also the only learner. No person will check the Hebrew or test him.
- **Technical:** The app is one Go binary with SQLite, built and changed mostly by AI agents, and deployed to a home k3s cluster through Argo CD. The Windows dev machine has no Go on PATH, so tests run through Docker, and CI takes about 20 to 30 minutes. Brian tolerates a deploy that breaks a study morning.
- **Content:** INIT-012 decided that content stays original. Private reuse of the tutor's text is Brian's call, and he may ask the tutor.
- **Regulatory:** None stated.

### Success Criteria

- A short, ranked list of changes that would make the app teach Hebrew better.
- Bad lessons stop reaching Today, so Brian can trust lessons without checking each one.
- An honest view of what to keep, change, or stop.

---

## Options Considered

Each specialist scored each option from 1 to 5 on six dimensions: learning value, trust and content correctness, effort and time to value, treadmill risk (a higher score means a lower risk of repeating the complaint, initiative, and new-rule loop), preserving progress and history, and reversibility. Each specialist's total is out of 30, and the panel total is out of 150.

### Option A: Patch in place

**Description:** Keep the current shape: AI-written per-learner units, the validator, the brief, and auto-publish. Fix each complaint as it comes. This is the status quo.

**Panel score:** 60 of 150 (Technical Architect 13, QA Advocate 13, Product Strategist 13, Data Analyst 11, Devil's Advocate 10).

**Pros:**
- It needs no new design work, and each patch is small and can be undone on its own (reversibility 16 of 25).
- It keeps all of Brian's history in place (15 of 25).

**Cons:**
- It keeps the shape that causes the defects. The validator lists bad patterns already seen, and that list can never be complete, so A scores lowest on trust (5 of 25) and on treadmill risk (5 of 25).
- Each fix has to be copied by hand into four or five places, and each new rule turns old units into a cleanup backlog.
- Bad questions keep writing false wrong answers into the learner model, and "known" keeps being redefined, so the history is kept but means less with each release.

**Specialist support:**
- Supported by: none.
- Opposed by: all five, each of whom scored it lowest.

### Option B: Stabilize, then re-center on the tutor (staged)

**Description:** Stage 1 (weeks 1 to 2) makes small trust fixes that are mostly deletions or reversals: void unprovable wrong answers, add one tap for "my answer was also right", add word clips for listening, make Progress an honest frozen ladder, stop auto-publishing AI units so Today is review plus safe practice, stop grading word-choice blanks, and freeze the side features. The tutor-material audit is the gate. In Stage 2 an agent imports tutor classes from Drive into class packs, graded practice comes only from trusted keys, the AI acts as a tutor (where you are, what to practise, ungraded stories, a weekly note for the tutor), and the unit pipeline retires, with units archived and never deleted.

**Panel score:** 122 of 150 (Technical Architect 26, QA Advocate 24, Product Strategist 25, Data Analyst 25, Devil's Advocate 22).

**Pros:**
- It is the only option that removes the structural cause, AI-written units behind a validator of known bad patterns, while keeping the core. It scores highest on learning value (20 of 25) and on trust (22 of 25).
- Stage 1 is small, mostly switches and deletions, pays off within days, and can be undone.
- It fixes the measurement before the content, so Stage 2 is judged against a clean baseline and each move in Brian's numbers has one cause (Data Analyst).
- The audit gate turns the riskiest assumption, that tutor material can be loaded with no effort from Brian, into a pass or fail test before anything is built on it.
- It keeps his history, because his evidence never depended on units (21 of 25).

**Cons:**
- Stage 1 adds no new practice, so Brian gets little of what he asked for by name, such as typing sentences and interesting stories, until Stage 2 (Product Strategist).
- Stage 1 still runs through the same process, and the other specialists' conditions already grow it (Devil's Advocate). D scores slightly higher on treadmill risk (20 against 19).
- Three specialists scored trust 4 rather than 5. The pointed lexicon was written by agents, and how much of it is verified is unknown (Technical Architect). The first draft also did not say that safe grading covers review and the 11 finished units (QA Advocate); Stage 1 item 3 now does.

**Specialist support:**
- Supported by: all five. The Devil's Advocate supports it with Option D written down as the fallback.
- Opposed by: none.

### Option C: Re-center now, ungated

**Description:** Build the tutor import, the sentence bank, and the session assembled by code immediately, and retire the unit pipeline in parallel.

**Panel score:** 69 of 150 (Technical Architect 15, QA Advocate 14, Product Strategist 14, Data Analyst 15, Devil's Advocate 11).

**Pros:**
- It aims at the same end state as B.
- It scores second on learning value (14 of 25).

**Cons:**
- It is one big push through the same process with no gate and no Stage 1 trust fixes, while today's untrustworthy app keeps running.
- If the tutor material turns out to be thin, retiring units in parallel leaves nothing to fall back on.
- A rushed retirement risks archiving units before the void, which needs `unit_id`, or deleting a unit row, which deletes his lesson history with it.
- It changes the content, the metrics, and the pipeline at once with no baseline, so no result can be traced to one cause (Data Analyst).
- It adds a large new surface with no gate, including telling the tutor's lines from Brian's typed lines and adding vowel points by machine (QA Advocate).
- It scores lowest on effort and time to value (9 of 25) and on reversibility (10 of 25).

**Specialist support:**
- Supported by: none.
- Opposed by: all five, who ranked it third.

### Option D: Freeze and go simpler

**Description:** Spend fourteen days with no building. Use Anki or the worksheet player, built from tutor sentences, alongside app review only. Decide afterwards; the app may end up review-only.

**Panel score:** 102 of 150 (Technical Architect 21, QA Advocate 20, Product Strategist 19, Data Analyst 20, Devil's Advocate 22).

**Pros:**
- It costs nothing in code and can be fully undone. It scores highest on reversibility (24 of 25) and on treadmill risk (20 of 25).
- It stops new bad lessons, and its instinct, tutor sentences and no more building, is right (Product Strategist).

**Cons:**
- It fixes nothing in the app. For 14 days review keeps its false wrong answers, its hidden listening cards, the contaminated struggle list, and a Progress page he cannot trust.
- When building resumes, the same validator and the same five copies of the rules are still there.
- Practice done in Anki or the worksheets is invisible to the learner model, so his record splits in two.
- It relies on Brian loading tutor sentences himself, which he has not done in a month.
- Fourteen days is too short for a valid learning experiment with one learner (Data Analyst).

**Specialist support:**
- Supported by: the Devil's Advocate, who scored it equal to B (22) and holds it as the written fallback. No specialist preferred it outright.
- Opposed by: the other four, who ranked it second behind B. D wins only on treadmill risk and reversibility, which doing nothing always wins.

---

## Recommendation

### Primary Recommendation

#### Option B: Stabilize trust first, then re-center the app on the tutor's lessons, with a measured gate between the stages and Option D as the written fallback

### Rationale

1. **It removes the cause and keeps the core.** The Technical Architect's case: B is the only option that takes out AI-written units behind a validator of known bad patterns while keeping the answer log, FSRS, the learner model, the lexicon, the audio cache, and the MCP server. Brian's history is safe, because his evidence never depended on units.
2. **It fixes trust first, and trust is Brian's binding constraint.** The QA Advocate's case: the app records him as wrong when it cannot prove he is wrong, and then builds his next lesson on that record. Stage 1 fixes exactly that with small changes that each have a pass line. The Product Strategist added the condition that the changes are visible in week 1: listening back, honest Progress, and a capped session.
3. **It fixes the measurement before the content.** The Data Analyst's case: Stage 1 removes the two biggest sources of bad data, ambiguous items graded as wrong and a "known" label and "due" count that do not mean what they say. Stage 2 is then judged against a clean baseline.
4. **It tests the riskiest assumption before building on it.** The Product Strategist and Technical Architect both named the same top risk: the tutor material has never been loaded. The week-1 audit costs Brian nothing and decides whether Stage 2 is built.
5. **It answers the process risk with mechanisms, not advice.** The Devil's Advocate's case: a rebuild through the same process would repeat the treadmill. B splits the work into a closed Stage 1 and a gated Stage 2, with guardrails Brian approves and abandon conditions that trigger Option D.

### Confidence Level

**Medium-High**

All five specialists agree on the direction, and the diagnosis was verified in code and in live learner data. The main uncertainty is the tutor material: how much of it exists, how clean it is, how many lines carry vowel points, and whether an agent can load it without Brian. The week-1 audit tests exactly that. A second uncertainty is whether guardrails written in a brainstorm will be enforced, which no document can settle; Brian's explicit go for each initiative and the abandon conditions are the answer.

### What to Keep, Change, and Stop

- **Keep:** the answer log, FSRS scheduling, the pointed lexicon with its verb and noun forms, the audio cache, the learner-model rebuild from logs, coach notes, and the MCP server.
- **Change:** the syllabus comes from the tutor instead of frequency bands; grading never counts an answer the app cannot prove wrong; Progress becomes one honest, frozen ladder; listening is switched back on; and practice moves toward production against trusted keys.
- **Stop:** AI-written per-learner units, the reviser, and auto-publish. Freeze Isaiah, the roots hub, placement, multi-user and teacher features, and the sound-contrast queue, and delete the stale plan-builder skill.

### Stage 1: Trust and Measurement (closed list)

Stage 1 is one initiative, finished within 21 days. Only these seven items belong in it. Refactors, the robot run-through, and new logging wait for Stage 2.

1. **Void unprovable wrong answers.** Mark them with a reversible flag and a reason: every answer from units 12 to 34, and every answer from word-choice blanks with no English cue. Do this before any unit is archived, because the void needs `unit_id`. No attempt or review row is removed.
2. **Add one tap for "my answer was also right".** Test it as a true undo: after a tap, the card's state, FSRS stability, due date, and confusion rows equal their values before that answer. Reporting a lesson voids its wrong answers.
3. **Stop grading word-choice blanks everywhere,** including review and the 11 finished units. Lessons switch to review's stricter gloss-overlap check for wrong buttons.
4. **Turn listening on.** Generate a word clip for every taught word, and never create a listening card without a clip. Review must also find clips by their text: today it reads only the word's own `audio_id` (`internal/db/activity.go:34`), which no code writes. Verb and noun forms already fall back to stored clips by text (`internal/db/verbforms.go:124`). Without that fallback, new clips would not turn listening review on.
5. **Make "due" and pacing honest.** Listening cards with no clip leave "due". Same-day learning steps fall due on the next study day. Replace the gates that hold lessons back (more than 40 learning words, under 75% of reviews rated Good or Easy, and a tier counted only from words known in every skill) with a review-load gate and a later-day production rate.
6. **Build one frozen "proved" ladder per skill, counted out of words taught.** "Proved" means right on 3 study days spanning a week, with the latest answer right, and no calendar decay. The headline rows are "Understand by ear" and "Say it from English". The void, the new ladder, and the cold-check seeding land as one announced change, shown beside the old numbers. The ladder is then frozen for 90 days, and the history is rebuilt once from the logs.
7. **Stop auto-publishing AI-written units.** Today becomes a capped 20 to 30 minute session weighted toward production. Freeze the side features (Isaiah, roots, placement, multi-user and teacher features, and the sound-contrast queue) behind switches, without deleting them. Delete the stale `hebrew-worksheet` skill.

### Measurement Alongside Stage 1

These run with Stage 1 and need no new subsystem.

- **Day-0 baseline.** Before any Stage 1 change ships, compute the day-0 values under the new "proved" rule, and take a held-out probe of about 20 normal-speed listening items and 10 English prompts that are never practised and are scored by code.
- **Cold check.** Test about 40 words the app calls unseen or learning. Words Brian gets right seed his practice level as "recalled", with a dated note, and never count as "proved".
- **Tutor-material audit.** An agent reads the Drive folder in week 1 and counts usable tutor-written lines and loadable classes. Brian does nothing.
- **Pass and fail lines.** These are written down before Stage 1 ships and are not moved later: the audit line, the "also right" stop line, the two success numbers, the guardrails, and what "the probe rose" means. A 30-item probe can only show a large gain, roughly 7 more items right, so smaller moves are read as noise (Data Analyst).

### Stage 1 Exit

An agent that did not build Stage 1 checks that all three hold:

- Zero graded steps on Today fail a correctness rule.
- There is at most one "also right" tap per 50 graded items in a week.
- Brian answers yes to "Do I trust Today now?".

### Gate to Stage 2

All of these must hold before Stage 2 opens:

- The Stage 1 exit is met.
- The audit passes: about 150 or more usable tutor-written lines, and 5 of the last 10 classes, loadable without Brian. Both are required.
- Brian has decided whether the tutor's text may be reused privately.
- About 150 graded answers and 50 delayed reviews exist under the new rules, and at least one week has passed.

If the audit fails, the import is replaced by Brian adding words through the existing tools, and practice stays review plus ungraded stories.

### Stage 2: Re-center on the Tutor (outline)

Stage 2 is a separate initiative, opened only after the gate.

- An agent imports each class from Drive into a class pack. The tutor's lines and Brian's typed lines are stored in separate fields and never split on punctuation. Brian's typed lines are never answer keys; they are used only as the wrong side of a contrast.
- Graded practice uses trusted keys only. It starts with dictation, "which line did you hear", and sentence order. Typed production is graded on the target words only, compared without vowel points, accepting both full and short spellings.
- The tutor's reading texts and AI stories built from taught words are never graded. A comprehension question is graded only when its answer is a span of the story.
- AI-written text is never graded, so no judge model is built.
- The AI acts as a tutor: it says where Brian is and what to practise next, and it writes a weekly note for the tutor.
- Lessons and review share one question engine, the authoring docs are generated from the rule catalog, and replays, playback speed, and the number of options are logged.
- The unit pipeline retires after two to three weeks of use. Units are archived, never deleted.

**Revised 2026-11-10 (Brian).** Stage 2's lessons take the five lesson types in [`docs/design/lesson-architecture.md`](../../../docs/design/lesson-architecture.md): word, conjugation, listening, easy, and spelling. Code assembles each lesson from the class packs and the lexicon. The AI writes only the story, and the story is never graded. The provisional spec list is in [`spec-skeleton.md`](spec-skeleton.md) §5. The gate is unchanged.

### Process Guardrails

These are part of the decision, not advice.

- One initiative is in flight at a time, each with Brian's explicit go.
- Every spec has a line saying what Brian will see differently.
- Complaints go on one list that Brian reviews weekly, after a 24-hour cooling-off.
- A rule budget applies: a new rule retires an old one.
- Deletion comes first.
- No agent assesses its own work.
- A one-command local Docker check runs before every push.
- Generated code is never edited by hand.

### Fallback (Option D) and Abandon Conditions

Stop building, make the app review-only, and move tutor material to Anki or the worksheet player if any of these happens:

- Stage 1 is not finished within 21 days.
- A second initiative opens before Stage 1 closes.
- Brian's study time has not reached about 120 minutes a week by week 4.
- The monthly listening probe is flat for two months.

### Success Measures

| Measure | Type | Baseline | Target |
|---------|------|----------|--------|
| Study minutes per week | Primary | 64 (week of 2026-11-02) | At least 120 by week 4 |
| Share of words taught 14 or more days ago that are proved from English to Hebrew | Primary | Day-0 value under the new rule; today's closest proxy is 3 of 51 English-to-Hebrew words known | Set after two weeks of data (the Data Analyst's prior is 60% to 70%) |
| Monthly held-out normal-speed probe | Primary | The first run during Stage 1 | Rising; flat for two months triggers the fallback |
| Study days per week | Guardrail | At least 13 study days from 2026-10-20 to 2026-11-08 (a floor) | 5 or more (Data Analyst) |
| Lesson abandon rate | Guardrail | Not counted; Brian says he skips or stops "often" | Near zero by week 4 (Product Strategist) |
| "Also right" tap rate | Guardrail | New in Stage 1 | At most 1 per 50 graded items in a week |

### Scoring Matrix

**Per-specialist totals (out of 30)**

| Specialist | A: Patch in place | B: Staged | C: Re-center now | D: Freeze and go simpler | Preferred |
|------------|---|---|---|---|---|
| Technical Architect | 13 | **26** | 15 | 21 | B |
| QA Advocate | 13 | **24** | 14 | 20 | B |
| Product Strategist | 13 | **25** | 14 | 19 | B |
| Data Analyst | 11 | **25** | 15 | 20 | B |
| Devil's Advocate | 10 | **22** | 11 | **22** | B (D as written fallback) |
| **Panel total (out of 150)** | **60** | **122** | **69** | **102** | **B, 5 of 5** |

**Aggregate matrix (sum of five specialists, out of 25 per cell)**

| Dimension | A | B | C | D |
|-----------|---|---|---|---|
| Learning value | 9 | **20** | 14 | 12 |
| Trust and content correctness | 5 | **22** | 12 | 14 |
| Effort and time to value | 10 | **19** | 9 | 15 |
| Treadmill risk (higher = lower risk) | 5 | 19 | 10 | **20** |
| Preserves progress and history | 15 | **21** | 14 | 17 |
| Reversibility | 16 | 21 | 10 | **24** |
| **Total** | **60** | **122** | **69** | **102** |

D wins only on treadmill risk and reversibility, which doing nothing always wins. B wins on learning value and trust by wide margins, and those are the dimensions Brian named.

### Accepted Trade-offs

| Trade-off | Why Accepted |
|-----------|--------------|
| Fewer kinds of graded questions for a while | One unfair "wrong" costs more trust than variety earns. Production formats such as typing, sentence building, and dictation give stronger evidence per answer, so evidence quality rises even as the number of formats falls. |
| The Progress numbers move once more | The change is shown once with the old and new numbers side by side, then frozen for 90 days and rebuilt from the logs. |
| No new lessons until Stage 2 | Today becomes a real session of review and production, and the brief already says "review first". |

---

## Risks & Mitigations

The ratings follow the specialists' own risk ratings in their files, split here into probability and impact.

| Risk | Probability | Impact | Mitigation Strategy | Owner |
|------|-------------|--------|---------------------|-------|
| The tutor material never loads, or it is too thin, unpointed, or mixed with Brian's own typed errors to use. | High | High | An agent reads the Drive folder in week 1, so Brian does nothing. The audit is the gate, and its pass line is written down before Stage 1 ships. If the audit fails, Brian adds words through the existing tools and practice stays review plus ungraded stories. The tutor's lines and Brian's typed lines are stored in separate fields, and only the tutor's lines are answer keys. | Audit agent (not the import builder); Brian decides at the gate |
| Stage 1 scope creep: conditions from the panel grow the closed list into another treadmill. | High | High | Stage 1 is a closed list of seven items plus the measurement items. Refactors, the robot run-through, and new logging wait for Stage 2. New ideas go on the weekly complaint list. If Stage 1 is not finished within 21 days, the plan falls back to Option D. | Brian; Stage 1 initiative author |
| The process guardrails are ignored, as the class-data runbook gate was. | Medium | High | Brian gives an explicit go for each initiative, only one is in flight, complaints wait for a 24-hour cooling-off and a weekly review, and no agent assesses its own work. A second initiative opening before Stage 1 closes triggers the fallback. | Brian |
| Another redefinition of "known" moves Brian's numbers for reasons unrelated to learning. | Medium | High | The void, the new ladder, and the cold-check seeding land as one announced change, shown beside the old numbers, then frozen for 90 days with the whole history rebuilt once from the logs. Seeded words count as "recalled", never as "proved". Tutor class words arrive later as their own row, not as a new definition. | Stage 1 building agent; checked by an independent agent |
| Word clips add a large batch of unchecked text-to-speech audio to the goal skill. | Medium | Medium | Run a speech-to-text round trip on new clips if one is available at reasonable cost. If not, ship the clips anyway, because 56 hidden listening cards cost more than an occasional mispronounced word. | Stage 1 building agent |
| Typed-answer grading marks correct Hebrew wrong, because Hebrew allows other word orders, dropped pronouns, and synonyms. | Medium | High | Grade only the target words, compared without vowel points, accepting full and short spellings. Start Stage 2 with dictation, "which line did you hear", and sentence order, and add whole-sentence typing only after the "also right" tap shows it rarely misfires. More than one "also right" tap per 50 graded items in a week pauses new packs. | Stage 2 building agent |
| Voiding loses its data if units are archived first, or a unit row is deleted and takes its `lesson_runs` and `lesson_items` with it. | Medium | High | Void first, as a reversible flag with a reason, before any unit is archived, because the void needs `unit_id`. Archive units and never delete them. Take a database backup before every migration and keep migrations additive. | Stage 1 building agent |
| Stage 1 only removes things, so Brian gets too little to notice and his study time does not move. | Medium | High | Week 1 ships visible changes: listening back, honest Progress, and a capped 20 to 30 minute session weighted toward production. Every spec says what Brian will see differently. Study time under about 120 minutes a week by week 4 triggers the fallback. | Stage 1 initiative author |
| Retaining class material does not produce conversation. | Medium | High | The app's job is retention, ear training at normal speed, and rehearsing answers aloud; conversation happens with the tutor, and the weekly note points tutor time at weak spots. The monthly held-out probe is the outside measure, and a probe that is flat for two months means the plan is wrong. | Brian; measurement agent |
| Errors in agent-written lexicon entries reach graded answers. | Medium | Medium | Prefer the tutor's lines as answer keys, grade only exact lexicon forms he was taught, and let the "also right" tap and its stop line catch errors that reach him. How many entries are verified is still unknown. | Stage 2 building agent |

---

## Dissenting Views

### Devil's Advocate

**Position:** The conditions from the other four specialists are already growing Stage 1 into the same treadmill, and guardrails written in a brainstorm are no stronger than the runbook gate that was ignored. Held to the closed list, B is fine.

**Rationale:** The other specialists' conditions added about ten pieces of work to Stage 1, including one question engine, a generated rules guide with its own test, a local Docker check, re-checking the 11 finished units, a tested true undo, a capped session, honest "due" and pace gates, replay and speed logging, a held-out probe, and an independent exit check. For that reason the Devil's Advocate scored B 3 rather than 4 on effort and on treadmill risk. Written rules in this repo have been skipped before: the class-data runbook gate was ignored, and INIT-026 was marked complete while undeployed. The Devil's Advocate also noted that the equal-weight rubric leans toward inaction, which is why D tied B on those scores, and did not treat the tie as a reason to prefer D.

**Response:** Stage 1 became a closed list of seven items plus the measurement items. Refactors such as the single question engine and the generated rules guide, the robot run-through, and new logging moved to Stage 2. The group accepted that enforcement cannot be settled on paper and answered it with Brian's explicit go for each initiative, one initiative in flight at a time, the weekly complaint list, and abandon conditions that trigger Option D.

### QA Advocate (reservation)

**Position:** Generating word clips for every taught word puts a large batch of unchecked text-to-speech audio into the goal skill at once.

**Rationale:** Today only the presence of a clip is checked, not whether it is pronounced correctly, and listening at normal speed is the skill Brian's goal depends on.

**Response:** Add a speech-to-text round-trip check on new clips if it is cheap. If not, ship the clips anyway, because 56 hidden listening cards cost more than an occasional mispronounced word.

### Data Analyst (adopted)

**Position:** Set the Stage 2 gate on counts, not only on the calendar. A 14-day freeze is too short to be a valid experiment with one learner.

**Rationale:** At 2 to 4 study days a week, one calendar week may not hold enough answers to show whether the dispute rate and the honest due count hold up. Comparing two ways of practising with one learner needs about 120 words in each group over 8 to 10 weeks.

**Response:** Adopted. The gate requires about 150 graded answers and 50 delayed reviews under the new rules, as well as one week. The 14-day freeze was replaced by Stage 1 and the audit gate.

### Technical Architect (adopted)

**Position:** Build no judge model at first.

**Rationale:** Any sentence the AI proposes ships ungraded, as practice or as part of a story, and only tutor lines and exact lexicon forms are graded. That keeps Stage 2's surface, cost, and rule count smaller, in line with the "delete first" guardrail. A judge from another model family would be added later only if the trusted bank proved too thin to fill a 20 to 30 minute session.

**Response:** Adopted for both stages: no judge is built, and AI-written text is never graded. The QA Advocate had wanted a judge as a one-way filter, and the Devil's Advocate sided with the Technical Architect.

### Product Strategist (adopted)

**Position:** Stage 1 must give Brian something visible in week 1: listening back, honest Progress, and a capped session.

**Rationale:** The main risk in B is not that Stage 1 does too much but that it gives Brian too little. If Stage 1 only removes things, Today becomes review alone, and he gets none of what he asked for by name until Stage 2.

**Response:** Adopted. Stage 1 ships listening back, the honest ladder, and a capped 20 to 30 minute session weighted toward production, and every spec carries a line saying what Brian will see differently.

---

## Next Steps

| # | Action | Owner | Target Date | Dependencies |
|---|--------|-------|-------------|--------------|
| 1 | Decide go or no-go on Stage 1 as one initiative, and close or pause the older initiatives still open (the skeleton lists INIT-010, INIT-011, both INIT-014 folders, INIT-023, INIT-024, INIT-028, INIT-031, INIT-032, and INIT-036), so only one is in flight. | Brian | Before any Stage 1 work starts | None |
| 2 | Agree to the process guardrails, including the weekly complaint list after a 24-hour cooling-off, in place of asking an agent for same-day fixes. | Brian | With action 1 | None |
| 3 | Decide whether the tutor's text may be reused privately, possibly after asking the tutor. | Brian, optionally with the tutor | Before the Stage 2 gate | None |
| 4 | Run SDD `create-initiative` with [spec-skeleton.md](spec-skeleton.md), source BRAIN-001, to open the Stage 1 initiative. | SDD agent, on Brian's go | Right after actions 1 and 2 | Actions 1 and 2 |
| 5 | Write down the pass and fail lines. The audit line requires both counts (see Appendix C). | Stage 1 initiative author, approved by Brian | Before any Stage 1 change ships | Action 4 |
| 6 | Take the day-0 baseline under the new "proved" rule and the first held-out probe. | Measurement agent; Brian takes the probe | Before any Stage 1 change ships | Action 5 |
| 7 | Run the tutor-material audit of the Drive folder. | An agent that will not build the import | Week 1 of Stage 1 | Action 4 |
| 8 | Run the cold check of about 40 words. | An agent prepares it; Brian answers | Before the one announced Progress change | Action 4 |
| 9 | Build the seven Stage 1 items, voiding first and before any unit is archived, and land the void, the ladder, and the cold-check seeding as one announced change. | Stage 1 building agent | Within 21 days of opening Stage 1 | Actions 5, 6, and 8 |
| 10 | Check the Stage 1 exit, then decide on the gate to Stage 2. | An agent that did not build Stage 1, then Brian | By day 21 | Actions 3, 7, and 9 |

---

## Appendix

### A. Full Specialist Perspectives

Each file holds that specialist's Phase 3 analysis, Phase 4 dialogue contributions, and Phase 5 scores. Their analysis is not repeated here.

| Specialist | Domain | File | Phase 5 preference (total out of 30) |
|------------|--------|------|--------------------------------------|
| Technical Architect | Engineering | [technical-architect.md](specialists/technical-architect.md) | B (26) |
| QA Advocate | Engineering | [qa-advocate.md](specialists/qa-advocate.md) | B (24) |
| Product Strategist (lead) | Product & Design | [product-strategist.md](specialists/product-strategist.md) | B (25) |
| Data Analyst | Product & Design | [data-analyst.md](specialists/data-analyst.md) | B (25) |
| Devil's Advocate | Security & Risk | [devils-advocate.md](specialists/devils-advocate.md) | B, with D as the written fallback (22, tied with D) |

### B. Related Documents

- [ASMT-004 assessment report](../../assessments/ASMT-004-hebrew-web-app/ASMT-004-hebrew-web-app-assessment-report.md): the assessment of the 23 cloned learner units that triggered this session.
- [INIT-036: known means remembered](../../initiatives/INIT-036-known-means-remembered/INIT-036-known-means-remembered-initiative.md): the stricter "known" rule that shipped on 2026-11-09.
- [INIT-037: taught words only](../../initiatives/INIT-037-taught-words-only/INIT-037-taught-words-only-initiative.md): a lesson asks only about taught words; it includes the unit 1 button verification (SPEC-011).
- [ADR-0031: known means remembered](../../../docs/decisions/ADR-0031-known-means-remembered.md): rule version 4 for "known", and the rule-version rebuild from `review_log` (D5).
- [ADR-0032: taught words only](../../../docs/decisions/ADR-0032-taught-words-only.md): the definition of a taught word (D1).
- [session-context.md](session-context.md): the session setup, context, and Brian's Phase 2 answers.
- [full-transcript.md](full-transcript.md): Phase 3 summaries, the parent's code verification, the Phase 4 dialogue, the Phase 5 options and scores, and the consensus.
- [spec-skeleton.md](spec-skeleton.md): the proposed spec set for SDD `create-initiative`.
- Other sources cited above: the [class-data runbook](../../../docs/operations/class-data-through-mcp.md) and [INIT-012 SPEC-008](../../initiatives/INIT-012-lesson-type-coverage/specs/SPEC-008-docs-skill-and-recipes.md).

### C. Session Notes

| Item | Detail |
|------|--------|
| Type | Quick Cross-Domain EPIC |
| Depth | Quick |
| Panel | Auto-assembled: Technical Architect, QA Advocate, Product Strategist (lead), Data Analyst, Devil's Advocate |
| Inquiry | The five specialists asked 15 questions, which were consolidated to 11 for Brian. His answers are in `session-context.md`. |
| Information injections | None occurred. |
| Models | All subagents ran on the session's inherited model. |
| Verification | Before the dialogue, the parent read the code and records behind the key claims: the "known" rule, the brief's word choice, the listening filter in review, the class-data runbook, and the INIT-012 copy rule. |

**Audit pass line (settled).** The first draft of the consensus gate said "or" where the Round 4 response and the Data Analyst's condition said both counts. The record now requires both: about 150 or more usable tutor-written lines, and 5 of the last 10 classes loadable without Brian.

**Assumptions not verified in the session.**
- Whether the worksheets in `Aharon's code/` come from Brian's tutor's course. The Product Strategist judged this likely but did not verify it.
- How clean the class documents are, how many lines carry vowel points, and which lines are the tutor's and which are Brian's.
- How many lexicon entries are verified.
- How much of the struggle list and the low recall numbers comes from ambiguous questions.
- Whether a Hebrew speech-to-text service is available at reasonable cost.
- The time per answer (about 10 to 12 seconds) used to size a 20 to 30 minute session.
