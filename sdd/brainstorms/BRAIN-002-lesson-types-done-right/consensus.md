# BRAIN-002 · Phase 5: Consensus

**Scoring:** each specialist scored options A to D from 1 (poor) to 5 (excellent) on six dimensions. The per-cell reasons are in each file under `specialists/`. Scoring ran on Haiku 5.5.

**Dimensions:**
- learning effect (backed by research);
- trust (no right answer marked wrong, no untaught words);
- time to new study (about 2 weeks);
- build cost and treadmill risk;
- fixes "feels the same";
- measurable.

## Totals per specialist (out of 30)

| Specialist | A. Full five types | B. Word + easy now | C. Study first, build one | D. Hybrid | Preferred |
| --- | ---: | ---: | ---: | ---: | --- |
| Product Strategist | 12 | 19 | 18 | **21** | D, with the easy lesson in slice 1 |
| UX Designer | 11 | 18 | 19 | **20** | D, with three changes |
| Technical Architect | 14 | 17 | 18 | **20** | D, on three conditions |
| Data Analyst | 13 | 17 | 19 | **20** | D, on three conditions |
| QA Advocate | 13 | 16 | 19 | **20** | D, with a smaller flag scope |
| Devil's Advocate | 11 | 15 | **21** | 20 | C, borrowing from D |
| **Sum (of 180)** | **74** | **102** | **114** | **121** | |

## Mean score per dimension (across the six specialists)

| Dimension | A | B | C | D |
| --- | ---: | ---: | ---: | ---: |
| Learning effect | 3.2 | 3.7 | 3.5 | 3.8 |
| Trust | 2.0 | 2.8 | 3.2 | 3.8 |
| Time to new study | 1.0 | 2.2 | 2.7 | 3.2 |
| Build cost and treadmill | 1.0 | 2.2 | 4.0 | 3.2 |
| Fixes "feels the same" | 3.2 | 3.2 | 2.2 | 2.3 |
| Measurable | 2.0 | 3.0 | 3.5 | 3.8 |

**Reading the table:**
- D leads on trust, speed, and measurement.
- C leads on build cost.
- A scores best on "feels the same" but worst on everything else.
- **No option fixes "feels the same" well (all 3.2 or below).** It stays a stated bet, with a tripwire.

## Agreement level

**Strong consensus on D (5 of 6).** The Devil's Advocate prefers C by one point. His dissent is about scope inside slice 1, not about direction. The two options share the backfill, studying in review now, one lesson type first, tutor lines as the passage, and a cut rule set in advance.

## Information injection R1: impact summary

**Injection:** "I won't be doing this just after a tutor lesson. So we need lessons that don't always align to my tutor's lessons but can help me keep learning." Re-evaluations ran on Haiku 5.5. Each one is appended to its specialist file.

| Specialist | Impact | Position | Main amendment |
| --- | --- | --- | --- |
| Product Strategist | High | Changed | Always a lesson. Words come from all classes, ranked by need, with the newest class only as a tie-break. Conjugation moves up. |
| UX Designer | High | Changed | Reserve a lesson slot. Label the source first (class date), topic second. Drop a step when no tutor line fits. |
| Technical Architect | High | Changed | A ranked T0 pool across every loaded class. The easy lesson moves into slice 1 as the heavy-day lesson. Add pool depth to the readout. |
| Data Analyst | High | Changed | Measure class usefulness by class-age band. Freeze a small baseline set. Count offers and starts in the use check. |
| QA Advocate | High | Changed, narrowly | Apply T0 per sheet. A minimum-lesson floor. The golden set includes older sheets. Words outside the classes are never shown in the first release. |
| Devil's Advocate | High | Changed in part | Draw from the whole archive in rotation. The backfill becomes a checked queue. Conjugation is triggered by an empty queue. Words outside the archive are exposure only. |

**Resume decision:** all six changed details, and none changed their preferred option. Five still prefer D, and the Devil's Advocate keeps his principle of "one build, study first", which D now absorbs. Scoring was not re-run. The recommendation below is amended in place, and every line R1 changed is marked **(R1)**.

**How the panel's splits were settled:**
- **Heavy days: easy lesson or a smaller word lesson?**
  - Product Strategist, Technical Architect, and QA Advocate: the easy lesson.
  - UX Designer: a word lesson with 2 to 4 words. Devil's Advocate: 1 to 2 words.
  - **Chosen: the easy lesson.** New words add review load on the days review is already over its cap. The easy lesson's keys have already passed their checks. It runs on the same thin player, with small extra cost.
- **Words from outside the classes:**
  - QA Advocate: never shown. Devil's Advocate: exposure only.
  - **Chosen: none in the first release.** No tier exists for them, and showing them would break the taught-only rule. Revisit once a second source exists.
- **Conjugation timing:**
  - Product Strategist: next, no trigger. Devil's Advocate: when the checked-word queue empties.
  - **Chosen: slice 2, started after the word lesson's day-14 use check.** It needs no new words and no voice.

## Recommendation: Option D, amended

**Before the Stage 2 gate (now to 2026-11-30): data entry and studying only, no new specs.**
1. **Count the true missing words** after stripping prefixes and inflected forms. **(R1)** Count across all 10 audited classes, not only the newest 3, so the size of the archive is known.
2. **Backfill T0 entries only.** T0 means the tutor's own document shows the word with its English gloss beside it. Use `upsert_lexemes`. Mark each entry for re-checking once the full-spelling column exists.
   - **(R1)** T0 is decided per sheet: only dated class sheets count, never study notes, the zip, or the html.
   - **(R1)** Before the gate, start with the newest 3 classes. The archive then becomes a checked queue. Older sheets join in date order, each one after it passes the per-sheet check. The 75 sheets not yet audited need that audit first.
3. **Record those words as taught** (`record_taught_words`) and study them in today's review. **(R1)** Every word carries its sheet reference.
4. **Fix typed grading inside Stage 1 (INIT-038), as a trust bug, not a new initiative.**
   - **The bug:** review marks the tutor's full spelling wrong today. One extra ו or י is graded Hard, and two are graded Again (`internal/fsrs/typed.go`).
   - **The fix:** an accept-set per word, a safe fold of ו and י (allowed only when no other taught word folds to the same letters), and a fixed test table of about 150 rows.
   - **Brian's call:** he must confirm this fits INIT-038 under D13, which allows one initiative in flight.
   - **If he does not confirm:** keep the backfilled words out of typed review until the fix ships.
5. **Agree the retire rule and the "same again" tripwire** (see below).

**Slice 1, after the gate: the word lesson, plus the easy lesson as the heavy-day lesson (R1), both on a new thin player.**
- **Word source (R1):**
  - **The pool:** every T0 word from a loaded class sheet that Brian has no card for, that has no open flag or suspension, and that passes the entry checks.
  - **Order:**
    1. Words the tutor corrected.
    2. Words used in two or more classes.
    3. Words that appear in a tutor line.
    4. The most recent class, used only as a tie-break.
  - **Variety:** each lesson draws from a class other than the previous lesson's, which gives topic rotation.
  - **No words from outside the classes** in the first release.
- **Label (R1):** the source comes first ("from class of 14 Oct") and the topic second.
- **Floor (R1):** a word lesson needs at least 2 new T0 words and 4 graded items. With fewer, Today offers the easy lesson.
- **Steps:**
  1. A grid of the new words: one card at a time on a phone, the full grid on a desktop.
  2. A meaning check with a second ask after a miss.
  3. A passage of the tutor's own lines that use the new words (G3 allows this). The AI story is an optional extra, and the lesson never waits for it. **(R1)** If no tutor line fits, the step is dropped, never filled with an AI sentence, and the stage bar shows only the stages that remain.
  4. Typed write with the accept-sets (the Devil's Advocate dissents; see below).
  5. Word order as an ungraded reveal.
- **Graded keys:** T0 or T1 lexicon words only.
- **No audio needed.** Listen buttons appear only where a clip exists.
- **Assembly:** code builds and freezes the lesson when Brian taps Start. Wrong-answer buttons are drawn fresh at each question. The lesson type is stored on the code-built unit.
- **Validation:** code-built lessons get their own release check, the three tests below. They do not run the old author-unit validator's `audio.missing` rule or the shape rules. Those rules are deleted only when the old units are archived, not before (Devil's Advocate and QA Advocate).
- **Flag control:** extend `internal/web/also_right.go` so one tap voids, flags, and suspends one item. It shows on right answers too ("This question is wrong") and has Undo.
  - This needs one new void reason in a small migration.
  - Report is changed to act on one item.
  - No flag table or fix-queue tool until there is real flag data (QA Advocate).
- **Today:**
  - Review comes first, with a soft cap. Due cards that don't fit roll over, and the most overdue go first.
  - **(R1)** Today always offers one lesson after review, and its time is reserved before review is counted:
    - review under the cap: a word lesson;
    - review over the cap: the easy lesson, with no new words. The rest of review rolls over.
  - **(R1)** Today shows review only when no passage passes its checks, and it says so ("no lesson today: nothing passed its checks").
  - **(R1)** Runs on heavy days are tagged "heavy-day".
- **Speaking prep:** a copyable "this week's words and the tutor's sentences" page for Brian's own AI. Check first whether the `hebrew://tutor` MCP resource already covers it.

**Slice 2:**
- **(R1)** The easy lesson moved into slice 1 (see above).
- **(R1)** **Conjugation lesson:** next, started after the word lesson's day-14 use check. It needs no new words and no voice. Its keys are only forms that are stored, or attested in the class text, on verbs Brian already knows.

**Deferred or cut:**
- **Listening lesson:** waits for an accepted voice. S3's 20 listening items are frozen now, and its 10 English prompts can run now.
- **Spelling lesson:** cut. Typed write in the word lesson does that work (RSCH-005).
- **Letters check (P-09), tutor note (P-11), generated docs, graded comprehension, graded word order:** cut. Graded word order is to be revisited after two weeks of flag data.

**Release bar for slice 1: three tests, not nine gates.**
1. The perfect-student run passes on at least 6 fixed sample lessons, and a wrong-student run fails on every graded step.
   - **(R1)** The fixed set includes at least 2 lessons from sheets outside the newest 10 classes, and at least 1 with a held line.
   - **(R1)** A lesson with zero graded keys fails.
2. The typed accept-set table (about 150 rows) passes.
3. No graded key is below T0 or T1, and no word on screen is untaught. **(R1)** T1 is unavailable in the first release, so in practice every key is T0.

Everything else is handled by the flag loop.

**Retire rule (agreed before slice 1; Data Analyst):** counted from a type's first run on Today.
- **Day 14 (use only):** retire if there are fewer than 6 runs, or if Brian finished fewer than half of his last 6 or more runs. If more than 10% of answers are flagged, the type gets one fix, not retirement.
  - **(R1)** "Runs" means lessons offered and started under the written rotation, because the app chooses what it offers.
- **Day 28 (proof):** needs at least 8 runs and 15 words. Retire if its later-day right rate is 15 points or more below the baseline. The baseline is the backfilled words studied in review only, from the same weeks. With fewer runs or words, wait until day 42 and decide on use alone.
  - **(R1)** The baseline is a frozen set of at least 15 backfilled words, kept out of lessons until day 42. This is Brian's decision. If he declines, the type is judged on use alone.
- **Start-by date:** a type must start by 2026-12-07 to be judged on proof before S2 (2027-01-04).
- **"Same again" tripwire:** if Brian says lessons feel the same in week 2, change the content mix: passage source, word count, and the form of the "use it" step. Do not add a lesson type.

**Measurement (minimal):**
- **The lesson type:** stored on the code-built unit. A run gets it through its unit.
- **Joining answers to runs:** attempts carry `unit_id` and `source`, but no run id. Join by unit and time window, or add a run id in the slice-1 migration. Decide this before building.
- **S1:** add active ungraded time, meaning input within the last 60 seconds, and show it separately.
- **Voids:** kept on file. A voided answer counts as neither right nor wrong. Accuracy is shown as a range: with voids counted as misses, and with voids dropped.
- **Flag rate:** about 3% is healthy. Over 10% means one fix.
- **No hold-out group.** Brian's goal is to learn, not to run an experiment.
- **Weekly readout:** five lines. It labels "too early" when counts are small, and never tells him to retire a type. **(R1)** It adds:
  - pool depth (ready, untaught T0 words);
  - the number of heavy days.
- **(R1) Class usefulness:** class words proved, grouped by how long ago the class was, and split by whether the word was first met in a lesson or only in review. No timing tied to class.

## Dissents recorded

- **Devil's Advocate** prefers C. He would keep typed write out of slice 1, because review already does typed recall and keyboard practice showed no vocabulary effect (RSCH-005). He would keep the validator rather than bypass it. Partly accepted: the validator is not deleted, and code-built lessons get their own check instead.
- **Product Strategist and UX Designer:** the easy lesson should be in slice 1, so Today always has a lesson. **(R1)** Accepted: it is the heavy-day lesson. The UX Designer withdrew the dissent in favour of a smaller word lesson, and the Devil's Advocate wanted 1 to 2 words. Both are recorded but not chosen.
- **Product Strategist:** revisit graded word order after two weeks of flag data. Accepted as a review date.
- **Technical Architect:** if the easy lesson is added, it must run on the new thin player, not the old `lesson.go`, so the old player can still be archived. Accepted.
- **Timing (UX Designer, Data Analyst):** the first new lesson screen lands after the 2026-11-30 gate, about 3 weeks from now, not 2. New class words reach review within days. That leaves about a week of slack before the 2026-12-07 start-by date.

## Decisions

**Brian decided (2026-11-10):**
1. **Typed-grading fix:** fix it now inside INIT-038, as a Stage 1 trust bug. Planned as [INIT-038/SPEC-015](../../initiatives/INIT-038-today-you-can-trust/specs/SPEC-015-service-typed-full-spelling.md) on 2026-11-10. A new initiative was not opened.
2. **Heavy-review days:** answered by injection R1. There is always a lesson, and it is not tied to a recent class. The panel's reading is the easy lesson on heavy days.
3. **Pre-gate backfill:** this agent runs it through the MCP, as data entry only, T0 entries.

**Still open:**
4. **Freeze about 15 backfilled words out of lessons until day 42** as the day-28 baseline. If no, the day-28 check is dropped, and types are judged on use alone.
