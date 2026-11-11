# ADR-0033: Stage 1, trust and measurement

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted (Brian confirmed the lines on 2026-11-09, with the wider reading of D3) |
| **Date** | 2026-11-09 |
| **Authored by** | INIT-038/SPEC-001 |
| **Initiative** | INIT-038 (today-you-can-trust), SPEC-001 |
| **Source** | [BRAIN-001](../../sdd/brainstorms/BRAIN-001-is-the-hebrew-app-built-right/decision-document.md), Option B, Stage 1 |
| **Supersedes** | [ADR-0031](ADR-0031-known-means-remembered.md) D1 (known needs a later study day) and D5's "learned today, check tomorrow" display |
| **Amends** | [ADR-0031](ADR-0031-known-means-remembered.md) D3 (one strict set for every reader); [ADR-0032](ADR-0032-taught-words-only.md) D1, for counting only (placement-only cards are not taught) |
| **Superseded by** | — |

## Summary

The app stops recording wrong answers it cannot prove wrong, and takes back the ones it already recorded, as flags that never delete a row. "Known" becomes one frozen "proved" ladder per skill, counted out of the words Brian was taught. "Due" means askable today, and Today offers one session of 20 to 30 minutes. No AI-written unit reaches Today. Every line that will judge this stage is written below, before anything ships, so no line can be moved after the numbers come in.

## Context

Brian, 2026-11-09: "Asking me questions like pick one, i____, eat, sleep, walk. Only one is right but technically all are right. I do a review, spell words, go to progress and it says you don't know any words … it's frustrating to not be able to know where I really am." BRAIN-001 found the core sound (the answer log, FSRS, the lexicon, the learner model, the audio cache, MCP) and these faults:

- **Unprovable wrong grades.** A lesson's buttons are chosen at play time from taught words, and only an exact gloss match is kept out (`glossKey`, `internal/web/lesson.go`). So "אַתָּה __ פֹּה?" in unit 1 lesson 2 has three right buttons.
- **Grades that are never undone.** Report a problem hides a unit but voids nothing (`ReportUnit`, `internal/db/learner_publish.go`). Units 12 to 34 were unpublished on 2026-11-08, but their answers still count. Today's struggle words גָּר and אוֹמֵר were both last missed inside the cloned lesson 12.
- **Listening is effectively off.** Review reads a word's clip only from `lexemes.audio_id`, which no code writes, while the clips sit in `audio_assets` by text. Due listening cards with no visible clip are skipped and stay due (`filterReviewAskable`, `internal/session/mix.go`).
- **"Known" is too strict to read.** It needs recall probability of 0.85 or more, the newest three answers right, and two study days, and it decays with the calendar. Each word has been asked about twice per skill, so almost nothing qualifies.
- **The process is a treadmill.** Ten initiatives were open at once, and same-day complaints became specs.

## Decision

**D1 · A void is a flag, never a delete.** A void is a row beside the answer it covers, with a reason (`unit_12_34`, `no_cue_blank`, `also_right`, `report`), an actor, a batch id, and a time. No `attempts`, `review_log`, or `confusion_misses` row is edited or removed. Every evidence reader skips a voided answer. A batch can be undone, and the card is then replayed with the answer counted again. A void replays the card from its remaining `review_log` rows, so the card ends exactly where it would be without that answer. Rejected: deleting rows, because nothing could check or undo it. Also rejected: editing card fields by hand, because `review_log` holds no prior state to restore.

**D2 · What is voided.**
- **Once, in release B:**
  - every answer from learner 1's units at positions 12 to 34;
  - every answer from a lesson word-choice blank whose step had no English cue.

  Right answers are voided too, because a cloned or cue-less step proves nothing either way. If a past answer cannot be matched to its step, every wrong lesson cloze answer is voided instead.
- **From then on:**
  - an answer the learner marks "My answer was also right";
  - a reported unit's wrong answers, for that learner.

  A tap only voids. It never credits.

**D3 · An answer the app cannot prove wrong records nothing.**
- **Blanks with buttons become reveals.** They show the answer and record no attempt, in every unit, finished units included.
- **No AI-written sentence is graded.** Every unit today is AI-written, so `order` steps and multi-word `listen_pick` steps also become ungraded reveals.
- **No judge model is built,** in either stage.
- **Fairer buttons where grading remains.** On the graded asks that remain, a wrong button is dropped when its gloss overlaps the target's (`activity.GlossesOverlap`), not only on an exact match.

Rejected: an `accept` list per blank, which still needs someone to foresee every right answer. Also rejected: a judge model, which would be an unchecked AI grading an AI-written key.

**D4 · The "proved" ladder, at rule version 5.** Each word and skill is in exactly one state, counted out of taught words (D5):
- **proved:** checked right answers on at least 3 different study days spanning at least 7 days, with the newest checked answer right;
- **recalled later:** at least one checked right answer on a study day after the day it was taught, but not proved;
- **slipping:** proved or recalled before, with the newest checked answer wrong;
- **not yet asked:** taught, with no checked answer since.

How it is counted:
- **No calendar decay.** A word leaves "proved" only by a wrong answer. FSRS recall probability schedules reviews and no longer feeds any count.
- **Checked means not self-reported.** This is unchanged from ADR-0031 D2: say self-checks and peeked answers never count. ADR-0030's implied typed-spelling credit counts.
- **Rule version 5.** Daily history is rebuilt once from the logs under it.
- **The old rule stays briefly.** Rule version 4 stays callable for 14 days after release B, for the one-time comparison only.

This supersedes ADR-0031 D1 and D5's display. Rejected: keeping "known" with looser numbers, because any rule that decays with the calendar moves while nothing changes in his head.

**D5 · Taught words.** For counting, a word is taught when the learner has a card for it that a lesson, review, or `record_taught_words` created. Cards that only placement created (`internal/db/placement.go`) are not taught. Until tutor words are recorded, Progress says "out of words the app has taught you". This narrows ADR-0032 D1 for counting only; button pools keep D1 as written. Rejected: counting placement cards, which would give a large denominator built from words he only guessed at once.

**D6 · Which reader uses which rung.** This amends ADR-0031 D3.
- **"Proved":** the Progress headline and Today's word count read this. The headline has two rows:
  - "Understand by ear" is the `listen` skill: Hebrew heard, meaning picked.
  - "Hebrew from English" is the `recall` skill: English shown, Hebrew typed or picked. Pronunciation is never checked, and a spoken answer counts for nothing (ADR-0031 D4). It was named "Say it from English" in BRAIN-001; Brian read that as a pronunciation check, so it was renamed on 2026-11-09.
- **"Recalled or proved":** these read the looser rung, so a word he has shown once on a later day lifts his level without counting as proved:
  - the AI's view in the digest and `get_context`;
  - `knownLexemes`;
  - the validator's coverage check;
  - the comfortable-text tier.

Rejected: one set for everything. ADR-0031 D3 tried that, and the strict set left the AI with almost no words to build from.

**D7 · The cold check.**
- **The words.** 40 words from the tutor material and the mistakes bank, all ones the app calls unseen or learning, are recorded with `record_taught_words` after each has a clip.
- **The check.** Their first review asks are the check. Only typed and listening answers count, because a first ask may be a multiple-choice pick.
- **What a pass is worth.** A pass shows as "recalled later" and can never make a word proved by itself.

Rejected: a separate check mode, which is new code and a rebuild of the `attempts.source` CHECK list.

**D8 · The 90-day freeze.** From release B, for 90 days, the following do not change:
- the D4 definition;
- the D5 taught-word definition;
- the two headline rows.

A fix for a defect that contradicts this ADR may change a count, and it is announced with the old and new numbers side by side. **Freeze ends:** 2027-02-08. That is 90 days after 2026-11-10, the day the release commit was built.

**D9 · Correctness rules.** These existing validator rules decide whether a graded step is fair. The exit sweep (INIT-037 SPEC-009's read-only sweep, with no brief) counts only these:

| Rule id | What a failure means for a graded step |
|-------|-------|
| `activity.target` | The step does not name the exact word it grades. |
| `audio.missing` | A Hebrew step has no clip, so a listening step cannot be answered. |
| `lexeme.unknown` | A graded token is not a lexicon entry, so its gloss and buttons are guesses. |
| `say.prompt_required` | A say step has no English cue. |
| `unit.cloze_context` | A gap has no sentence around it. |
| `unit.cloze_translation` | A gap has no English cue, so more than one word fits. |
| `unit.meta_text` | Learner English carries an authoring note. |
| `unit.order_accept` | An order step's other accepted answers are not valid reorderings. |
| `unit.order_translation` | An order line has no English. |
| `unit.translation_missing` | A multi-word `listen_pick` has no English line. |

`unit.target_untaught` and every `brief.*` rule run only with a brief, so the no-brief sweep cannot check them; the player's taught-word button pools (ADR-0032 D1) cover the same fault. `schema.*` rules block any save, so a published unit already passes them. Teaching-shape rules are not correctness rules: `unit.same_shape`, `unit.drill_share`, `budget.*`, `recycle.*`, `coverage.*`, and `unit.lesson_repeat`. Neither are mechanical rules, such as `unit.translation_length`. Stage 1 adds no validator rule.

**D10 · Due means due today, in one session.**
- **Same-day steps move.** A review whose next due time falls inside the same study day is due at the next study-day cutoff.
- **No listening card without a clip.** A listen card exists only when its word has a clip, found by `lexemes.audio_id` or by an `audio_assets` row for its exact pointed text.
- **One session.** Today offers one session sized to 20 to 30 minutes from his own answer times, preferring typing from English and dictation over meaning picks.

Rejected: keeping same-day learning steps. The app asks each word once per sitting (ADR-0024 D3), so those steps only ever showed as overdue.

**D11 · No AI-written unit reaches Today.**
- **Auto-publish is off.** A config switch turns it off: `put_unit` with `publish: true` on a learner scope saves a draft and returns a note naming this ADR, and `publish_unit` refuses learner units. One config change undoes it.
- **Lesson writing is paused** for Stage 1, through the author and reviser skills.
- **Units are archived, never deleted.** Deleting a unit row cascades to its revisions, lesson items, lesson runs, and reports.
- **Void before archive.** No unit is archived in Stage 1, because the void needs `unit_id`.

**D12 · What is frozen.**
- **Behind config switches, off by default, with no code deleted:** Isaiah, Roots, placement, and the check-in.
- **No new work:**
  - multi-user and teacher features;
  - the sound-pair queue (INIT-036 SPEC-008 to SPEC-012, closed 2026-11-09);
  - word-page noun and verb forms;
  - the pace and tier gates in `internal/brief`, which only lesson briefs read.

**D13 · How work is taken on.** Brian agreed to these on 2026-11-09:
- **One initiative in flight,** each with Brian's explicit go.
- **Complaints wait.** They go on [the complaints list](../../sdd/complaints.md) and wait 24 hours. Brian reviews the list weekly. A same-day "fix this" is a list item, not a spec.
- **A rule budget.** A new validator rule retires an old one.
- **Delete first.** Every spec names what it removes.
- **No agent assesses its own work.** The audit, the exit check, and the Security Review are done by agents that built none of what they judge.
- **One local check before every push:** [local-check.md](../operations/local-check.md).
- **Generated code is never edited by hand.** Every `*_templ.go` change comes from `templ generate`.

## Pass and fail lines

The week numbers below are weeks of the weekly report, which start on Monday. Week 1 starts 2026-11-09, the day of Brian's go, and week 4 starts 2026-11-30. Study minutes are the weekly report's `minutesStudied`, the time spent answering (`ActiveMinutes`, `internal/jobs/weekly.go`). An ungraded reveal writes no attempt, so it adds no minutes.

### Stage 1 exit

All four must hold. An agent that built none of INIT-038's specs checks them at `complete-initiative`, at least 7 days after release B.

1. **E1.** Zero graded steps in units Today can open fail a D9 rule, by the read-only sweep.
2. **E2.** At most 1 void with reason `also_right` per 50 attempts in the 7 days before the check.
3. **E3.** Brian answers yes to "Do I trust Today now?". The answer is recorded with the date and that week's study minutes.
4. **E4.** E1 to E3 hold by **2026-11-30**.

INIT-038's acceptance criteria also prove:
- after a void, a card equals its state before the answer;
- no listen card exists without a clip, and the audit's `ReviewSkippedNoAudio` is 0;
- every daily snapshot is stamped rule version 5;
- `put_unit` with `publish: true` on a learner scope saves a draft;
- no lesson blank writes an attempt.

### Gate to Stage 2

Stage 2 is a separate initiative. It opens only when all five hold, and only with Brian's explicit go.

1. **G1.** The Stage 1 exit is met.
2. **G2.** The tutor-material audit finds both:
   - **150 or more usable tutor-written lines** across the last 10 classes. A usable line is Hebrew the tutor wrote, not Brian's typing, of 2 or more words, and not a heading or an instruction.
   - **5 or more of those 10 classes loadable without Brian.**

   Both are required.
3. **G3.** Brian has decided whether the tutor's text may be reused privately.
4. **G4.** Under the new rules there are:
   - 150 or more graded answers since release B;
   - an `also_right` rate over those answers of at most 1 per 50;
   - 50 or more delayed reviews, meaning answers on a later study day than the card's previous answer.

   At least 7 days must also have passed since release B. G4 is met at whichever of these comes last.
5. **G5.** No abandon line below has fired.

If G2 fails, Stage 2's import is not built. Brian adds words with `record_taught_words`, and practice stays review plus ungraded stories.

### Success measures

1. **S1, study minutes.** 64 in the week starting 2026-11-02, and **120 or more in week 4**.
2. **S2, proved share.** The share of words taught 14 or more days ago that are proved in "Hebrew from English". Its day-0 value, for 2026-11-09, comes from the rebuilt history. **Target: 50% or more by the week starting 2027-01-04.**
3. **S3, the held-out probe.**
   - **The items.** 20 listening items at normal speed and 10 English prompts, never practised and never AI-written.
   - **Scoring.** Each sitting is scored by a fixed comparison: unpointed, with the `fsrs.GradeTyped` normalization, and English prompts graded on their target words only.
   - **When.** Later sittings come every 28 to 35 days. Amended 2026-11-10: the first listening sitting does not use the computer-read voice Brian refused. The 10 English prompts are listed in `sdd/initiatives/INIT-038-today-you-can-trust/specs/completion-summaries/SPEC-012-probe.md`. The 20 listening items wait for a voice he names. Release B does not wait on that voice.
   - **Rose** means 7 or more items above the first sitting. A smaller move is noise, because 30 items cannot show it.
   - **Flat** means fewer than 7 items above the first sitting.

### Guardrails

These do not judge success; they catch damage. Each is read weekly, and a breach goes on the complaints list.

1. **W1, study days per week.** Target 5 or more. A breach is under 4 for two weeks in a row.
2. **W2, lesson abandon rate.** The share of `lesson_runs` started in a week and not finished. A breach is a rise of 20 or more points over its day-0 value.
3. **W3, "also right" rate.** At most 1 per 50 attempts in any week; this is the E2 line.
4. **W4, cards due at the start of the study day.** Reviewable cards only. A breach is more than one session's planned count on 3 days in a week.

### Abandon lines

If any of these fires, stop building, make the app review-only, and move tutor material to Anki or the worksheet player in `Aharon's code/` (BRAIN-001 Option D).

1. **A1.** The Stage 1 exit is not met by 2026-11-30.
2. **A2.** A second initiative opens before Stage 1 closes.
3. **A3.** Study minutes are under 120 in week 4.
4. **A4.** The probe is flat at two monthly sittings in a row.

### Set here, not in BRAIN-001

These lines were made exact in this ADR. Brian confirmed them on 2026-11-09:
- S2's target of 50% by 2027-01-04. The brainstorm's optimistic case was 60% to 70% at 8 weeks.
- What a usable line is (G2).
- What a delayed review is (G4).
- W1's breach of under 4 days for two weeks.
- W2's breach of 20 points.
- W4's breach of 3 days in a week.
- The probe window of 28 to 35 days.

## Consequences

- **What implements this ADR.** INIT-038 SPEC-002 to SPEC-014 implement D1 to D12. INIT-038's initiative file maps each decision to its specs.
- **Two releases.** The decisions land in two releases. Release A ships D10, D11's switch, and D12's switches; it changes no meaning of a number. Release B ships D1 to D8 as one announced change, shown beside the old numbers.
- **What stays in force.** ADR-0031 D2 and D4 stay in force, so self-reported say checks still count for nothing. ADR-0032 D1 still governs button pools.
- **Tool text stops restating rules.** The MCP server instructions and tool descriptions stop restating the "known" rule and point to each response's `ruleVersion`. A rule change then needs no matching text edit.
- **Skills.** The stale `hebrew-worksheet` skill is deleted; it still told agents to call `create_plan`, which no longer exists. The author and reviser skills are marked paused.
- **Some practice stops counting.** Ungraded reveals write no attempt, so lesson time on them adds no study minutes. Today offers review only, so S1 is measured on review time.
- **Stage 2 is not planned yet.** It is a sketch in the BRAIN-001 skeleton until the gate passes.
