# Action Plan: Lesson Types Done Right (BRAIN-002)

**Date:** 2026-11-10
**Based On:** BRAIN-002 decision document (Option D, amended by R1) and `consensus.md`
**Plan Owner:** Brian Nelson (decides). The AI agent does most of the work.

Owners in this plan are **Agent** or **Brian**.

---

## Decision Summary

**Recommendation:** Option D, amended. In plain terms:
- **Now to 2026-11-30:** data entry and studying only. The agent backfills the tutor's words (T0 entries only, newest 3 classes first) through the MCP. Brian studies them in review. One trust-fix spec runs inside INIT-038: [SPEC-015](../../initiatives/INIT-038-today-you-can-trust/specs/SPEC-015-service-typed-full-spelling.md), the full-spelling grade.
- **After the Stage 2 gate:** build slice 1 on a new thin player. Slice 1 is the word lesson plus the easy lesson (the heavy-day lesson), the one-tap flag control, the Today changes, and the three-test release bar.
- **2027:** conjugation lesson next. Listening waits for an accepted voice. Spelling and the other cut items stay cut.

**Confidence:**
- **Direction: High.** 5 of 6 specialists prefer D. The Devil's Advocate prefers C, by one point, and his dissent is about scope inside slice 1.
- **"Feels the same" fix: Medium-Low.** No option scored above 3.2 on it. It is a stated bet with a tripwire.

**Main Constraints:**
- **Dates:**
  - Stage 1 (INIT-038) must close by **2026-11-30**. That is the Stage 2 gate.
  - A lesson type must start by **2026-12-07** to be judged on proof before S2 on **2027-01-04**.
- **One initiative in flight (ADR-0033 D13).** Before the gate: data entry and studying, plus ONE trust-fix spec inside INIT-038 (SPEC-015, the full-spelling grade, which Brian approved).
- **Lesson-authoring skills stay paused.** No learner units are written.
- **No accepted voice yet.** Brian rejected the computer-read voice on 2026-11-09. Every first-release step works without audio.
- **The tutor never touches the app.** Nothing depends on tutor input or review.
- **Tutor text is for private study only (G3, ruled 2026-11-10).**

**Still open (Brian):** whether to freeze about 15 backfilled words out of lessons until day 42, as the day-28 baseline. If he says no, types are judged on use alone. See action I-9.

---

## Immediate Actions (now to 2026-11-30)

| # | Action | Owner | Priority | Effort | Dependencies | Done |
|---|--------|-------|----------|--------|--------------|------|
| I-1 | **Count the true missing words** across all 10 audited classes. Strip prefixes (ו, ה, ב, ל, מ, ש, כ) and inflected forms first. Report the count per class, so the size of the archive is known. | Agent | P1 | M | None | [ ] |
| I-2 | **Backfill T0 entries for the newest 3 classes** with `upsert_lexemes`. T0 means the tutor's own dated class sheet shows the word with its English gloss beside it. Use dated class sheets only, never study notes, the zip, or the html. Every entry carries its sheet reference. Mark each entry for re-checking once the full-spelling column exists. | Agent | P1 | L | I-1 | [ ] |
| I-3 | **Record those words as taught** with `record_taught_words`. Every word carries its sheet reference. | Agent | P1 | S | I-2 | [ ] |
| I-4 | **Build [INIT-038/SPEC-015](../../initiatives/INIT-038-today-you-can-trust/specs/SPEC-015-service-typed-full-spelling.md).** The spec is written. Review marks a full spelling wrong today (`internal/fsrs/typed.go`): one extra ו or י is Hard, and two are Again. The fix drops ו and י from both sides and counts it as the word only when one lexicon entry has that remainder. `בית` and `בת` stay different. The test table is synthetic, at least 40 rows, and holds no tutor text. | Agent | P1 | M | None | [ ] |
| I-5 | **Sequence I-3 against I-4.** Aim to ship the fix before, or at the same time as, the first recorded words reach typed review. If the fix is late, keep the backfilled words out of typed review until it ships. | Agent | P1 | S | I-3, I-4 | [ ] |
| I-6 | **Write the retire rule and the "same again" tripwire** in one short page, from the decision document. Brian reads it and agrees. See "Retire rule" below. | Agent drafts, Brian agrees | P1 | S | None | [ ] |
| I-7 | **Brian studies daily** in today's review. This includes the backfilled words as they are recorded. Target for S1: 120 or more minutes a week by week 4. | Brian | P1 | M | I-3 | [ ] |
| I-8 | **Freeze the S3 item lists:** 20 listening items and 10 English prompts. The 10 English prompts can run now. The listening items wait for an accepted voice. | Agent | P2 | S | None | [ ] |
| I-9 | **Decide the baseline freeze.** Keep about 15 backfilled words out of lessons until day 42, as the day-28 baseline. Yes, or no. | Brian | P2 | S | I-2 (so there are words to pick from) | [ ] |
| I-10 | **Keep the guardrails.** No lesson-authoring skills run. No learner units are written. No second spec opens before Stage 1 closes (abandon line A2). | Agent | P1 | S | None | [ ] |
| I-11 | **Close Stage 1 (INIT-038) by 2026-11-30** and confirm the gate items: Stage 1 exit met, Drive audit passed (2026-11-09), G3 ruled (2026-11-10), enough evidence under the new rules. | Brian, Agent | P1 | M | I-4 | [ ] |

**Goals for now to 2026-11-30:**
- [ ] The count of true missing words is known for all 10 audited classes.
- [ ] T0 words from the newest 3 classes are in the lexicon, recorded as taught, with sheet references.
- [ ] Typed grading no longer marks the tutor's spelling wrong, and the test table passes.
- [ ] Retire rule and tripwire are agreed in writing.
- [ ] Brian has studied most days.
- [ ] Stage 1 is closed.

---

## Short-Term Actions (gate to about 2026-12-21)

| # | Action | Owner | Priority | Effort | Dependencies | Done |
|---|--------|-------|----------|--------|--------------|------|
| S-1 | **Run SDD `create-initiative` from `spec-skeleton.md`** for Stage 2, slice 1. Do not open it before the gate (A2). | Agent | P1 | M | I-11 | [ ] |
| S-2 | **Decide how answers join to runs** before building: join by unit and time window, or add a run id in the slice-1 migration. | Agent proposes, Brian confirms | P1 | S | S-1 | [ ] |
| S-3 | **Build slice 1 on the new thin player** (see list below). | Agent | P1 | L | S-1, S-2 | [ ] |
| S-4 | **Pass the release bar** (three tests, see below) before slice 1 reaches Today. | Agent | P1 | M | S-3 | [ ] |
| S-5 | **Start the word lesson on Today by 2026-12-07.** This is the start-by date for proof before S2. | Brian, Agent | P1 | S | S-4 | [ ] |
| S-6 | **Audit more sheets into the checked queue.** The 75 sheets not yet audited need the per-sheet check first. Then older sheets join in date order, each after it passes. | Agent | P2 | L | I-1 | [ ] |
| S-7 | **Check whether the `hebrew://tutor` MCP resource already covers the speaking-prep page** ("this week's words and the tutor's sentences", copyable for Brian's own AI). Build only the part that is missing. | Agent | P3 | S | S-3 | [ ] |
| S-8 | **Start the weekly five-line readout** (see Communication Plan). | Agent | P2 | S | I-7 | [ ] |

**Slice 1 contents (S-3):**
- **Word lesson.**
  - Pool: every T0 word from a loaded class sheet that Brian has no card for, with no open flag or suspension, that passes the entry checks.
  - Order: words the tutor corrected; words used in two or more classes; words in a tutor line; the most recent class only as a tie-break.
  - Variety: each lesson draws from a class other than the previous lesson's.
  - No words from outside the classes.
  - Label: source first ("from class of 14 Oct"), topic second.
  - Floor: at least 2 new T0 words and 4 graded items. With fewer, Today offers the easy lesson.
  - Steps: word grid; meaning check with a second ask after a miss; passage of the tutor's own lines (dropped, never filled with an AI sentence, if no line fits); typed write with accept-sets; word order as an ungraded reveal.
- **Easy lesson** as the heavy-day lesson, with no new words, on the same thin player.
- **Flag control:** extend `internal/web/also_right.go` so one tap voids, flags, and suspends one item. It shows on right answers too ("This question is wrong") and has Undo. It needs one new void reason in a small migration. Report changes to act on one item. No flag table or fix-queue tool until there is real flag data.
- **Today:**
  - Review first, with a soft cap. Due cards that don't fit roll over, most overdue first.
  - Today always offers one lesson after review, with its time reserved before review is counted. Review under the cap: a word lesson. Review over the cap: the easy lesson.
  - Today shows review only when no passage passes its checks, and it says so ("no lesson today: nothing passed its checks").
  - Heavy-day runs are tagged "heavy-day".
- **Lesson type** stored on the code-built unit.
- **Graded keys:** T0 words only in the first release (T1 is unavailable).
- **Old validator:** not deleted yet. Code-built lessons get their own release check. The `audio.missing` and shape rules are deleted only when the old units are archived.

**Release bar for slice 1 (S-4):**
1. The perfect-student run passes on at least 6 fixed sample lessons, and a wrong-student run fails on every graded step. The set includes at least 2 lessons from sheets outside the newest 10 classes, and at least 1 with a held line. A lesson with zero graded keys fails.
2. The typed accept-set table (about 150 rows) passes.
3. No graded key is below T0 or T1, and no word on screen is untaught.

Everything else is handled by the flag loop.

**Goals for the gate to about 2026-12-21:**
- [ ] Stage 2 initiative created from the spec skeleton, after the gate.
- [ ] Slice 1 passes the release bar.
- [ ] The word lesson starts on Today by 2026-12-07.
- [ ] More sheets are in the checked queue.

---

## Medium-Term Actions (2027)

| # | Action | Owner | Priority | Effort | Dependencies | Done |
|---|--------|-------|----------|--------|--------------|------|
| M-1 | **Day-14 use check** for the word lesson. Use only, no proof yet. | Agent runs, Brian decides | P1 | S | S-5 | [ ] |
| M-2 | **Day-28 proof check.** Needs at least 8 runs and 15 words. With fewer, wait until day 42 and decide on use alone. | Agent runs, Brian decides | P1 | M | M-1, I-9 | [ ] |
| M-3 | **Conjugation lesson** (slice 2), started only after the word lesson's day-14 check. It needs no new words and no voice. Its keys are only forms that are stored, or attested in the class text, on verbs Brian already knows. | Agent | P2 | L | M-1 | [ ] |
| M-4 | **Listening lesson** when Brian names a voice he accepts. Then run the frozen S3 listening items. | Brian names the voice, Agent builds | P3 | L | An accepted voice | [ ] |
| M-5 | **Revisit graded word order** after two weeks of flag data. | Agent proposes, Brian decides | P3 | S | S-5 plus 2 weeks | [ ] |
| M-6 | **Archive the old units**, then delete the old validator rules that no longer apply (`audio.missing`, shape rules). Delete first (D13). | Agent | P3 | M | S-5 | [ ] |
| M-7 | **S2 check on 2027-01-04:** 50% of words taught 14 or more days ago proved in "Hebrew from English". | Agent reports, Brian reads | P1 | S | I-3 | [ ] |
| M-8 | **Revisit words from outside the classes** only once a second source exists. | Brian | P3 | S | A second source | [ ] |

**2027 goals:**
- [ ] The word lesson is kept or retired by the written rule, with the readout as the evidence.
- [ ] Conjugation is built only if the word lesson passed day 14.
- [ ] S2 is measured on 2027-01-04.

---

## Retire Rule and Tripwire (agree before slice 1)

Counted from a type's first run on Today. "Runs" means lessons offered and started under the written rotation.

- **Day 14 (use only):** retire if there are fewer than 6 runs, or if Brian finished fewer than half of his last 6 or more runs. If more than 10% of answers are flagged, the type gets one fix, not retirement.
- **Day 28 (proof):** needs at least 8 runs and 15 words. Retire if the later-day right rate is 15 points or more below the baseline. The baseline is the frozen set of at least 15 backfilled words, kept out of lessons until day 42, studied in review only. If Brian declines the freeze, judge on use alone.
- **Day 42:** the fallback date when day 28 has too few runs or words. Decide on use alone.
- **"Same again" tripwire:** if Brian says lessons feel the same in week 2, change the content mix: passage source, word count, and the form of the "use it" step. Do not add a lesson type.

**If the word lesson starts on 2026-12-07** (the latest start date), the dates are: day 14 on 2026-12-21, day 28 on 2027-01-04, day 42 on 2027-01-18. An earlier start moves them earlier.

---

## Validation Checkpoints

| Checkpoint | What to Validate | Who Validates | When | Pass Criteria |
|------------|------------------|---------------|------|---------------|
| CP-1 | The count of true missing words across the 10 audited classes | Agent reports, Brian reads | After I-1 | A number per class, with prefixes and inflected forms stripped. |
| CP-2 | The backfill is real and traceable | Agent | After I-3, before 2026-11-30 | Every entry is T0, from a dated class sheet, with a sheet reference. Every recorded word has one too. |
| CP-3 | The typed-grading fix | Agent | After I-4, before 2026-11-30 | SPEC-015's tests pass, including the `בית`/`בת` case. A spelling that adds only ו or י is Good when that folded form is one lexicon entry. |
| CP-4 | Stage 1 gate | Brian | 2026-11-30 | Stage 1 exit met. Audit and G3 already done. Enough evidence under the new rules. |
| CP-5 | Slice 1 release bar | Agent | Before 2026-12-07 | All three tests pass (6 fixed sample lessons, typed table, no graded key below T0 or T1). |
| CP-6 | Word lesson has started | Brian | 2026-12-07 | First run is on Today, labelled with its source class. |
| CP-7 | Day-14 use check | Agent runs, Brian decides | 2026-12-21 (if start is 2026-12-07) | At least 6 runs. Brian finished at least half of his last 6 or more. Flag rate 10% or less. |
| CP-8 | Day-28 proof check, and S2 | Agent runs, Brian decides | 2027-01-04 | At least 8 runs and 15 words. Later-day right rate is not 15 points or more below the baseline. S2 reported. |
| CP-9 | Day-42 fallback | Brian | 2027-01-18 | Only if CP-8 had too few runs or words. Decide on use alone. |
| CP-10 | Graded word order review | Agent proposes, Brian decides | Two weeks after the first flag data | Brian decides whether to try it, based on the flag data. |

---

## Success Metrics

| Metric | Baseline | Target | Measurement Method | Timeline |
|--------|----------|--------|-------------------|----------|
| **S1: study minutes** | Weekly study was 0, 35, 57, then 64 minutes over the last month. | 120 or more a week by week 4. | Graded time plus active ungraded time (input within the last 60 seconds), shown separately. | Week 4 of Stage 1, then weekly in the readout. |
| **S2: words proved** | Not measured yet. | 50% of words taught 14 or more days ago proved in "Hebrew from English". | The "proved" ladder (ADR-0033 D4): checked right answers on 3 or more study days spanning 7 or more days, newest right. | 2027-01-04 |
| **S3: held-out probe** | Not run yet. | Set by ADR-0033. | 20 listening items and 10 English prompts, frozen now. The English prompts can run now. The listening items wait for an accepted voice. | Frozen by 2026-11-30. Run on the dates ADR-0033 sets. |
| **Flag rate** | None yet. | About 3% is healthy. | Flags divided by graded answers. Voids stay on file. A voided answer counts as neither right nor wrong, so accuracy is shown as a range (voids as misses, and voids dropped). | From the first lesson run. Weekly. |
| **Pool depth** | None yet. | Never short enough that Today falls back to the easy lesson for lack of words. | Count of ready, untaught T0 words (no card, no open flag, passes the entry checks). | Weekly. |
| **Heavy days** | None yet. | Tracked, no target. | Count of runs tagged "heavy-day". | Weekly. |
| **Class usefulness** | None yet. | Tracked, no target. | Class words proved, grouped by how long ago the class was, split by whether the word was first met in a lesson or only in review. | From the day-28 check. |

---

## Risk Monitoring

| Risk | Trigger Indicator | Response Action | Owner |
|------|-------------------|-----------------|-------|
| The backfill is larger than expected | I-1 shows a very large count of true missing words. | Teach only T0 words, as planned. Do not widen to T1 or T2 to catch up. Report the size to Brian. | Agent |
| A wrong gloss breaks trust | Brian flags a gloss in week 1, or a backfilled entry has no sheet reference. | Suspend the item. Re-check against the sheet. Fix the entry. | Agent |
| Typed grading marks a right answer wrong | A tutor full spelling is graded Hard or Again before I-4 ships. | Keep backfilled words out of typed review until the fix ships (I-5). | Agent |
| The new lesson work starts before the gate | Any new spec, or a second initiative, opens before 2026-11-30. | Stop. This is abandon line A2. Escalate to Brian. | Agent |
| The gate slips and the start-by date is missed | Stage 1 is not closed by 2026-11-30. About a week of slack remains before 2026-12-07. | Tell Brian at once. A late start means the type is judged on use alone before S2. | Agent |
| Brian skips lessons | Fewer than half of his last 6 or more runs finished. | Apply the day-14 rule. Do not add types. | Agent |
| "Feels the same" returns | Brian says lessons feel the same in week 2. | Change the content mix (passage source, word count, "use it" form). Do not add a lesson type. | Brian, Agent |
| Pool runs short | Fewer than 2 new T0 words ready, so Today offers the easy lesson. | Add older sheets to the checked queue in date order. Put pool depth in the readout. | Agent |
| Flag rate is high | Over 10% of answers flagged. | One fix for the type, not a new rule set. See escalation. | Agent |
| A rulebook grows back (the treadmill) | More than the three release tests, or new rules without deleting old ones. | Delete first (D13). Keep the bar at three tests. | Agent |
| Small counts read as a verdict | A readout line is based on very few runs or words. | Label it "too early". The readout never tells Brian to retire a type. | Agent |
| The voice is never accepted | No accepted voice by the time conjugation is done. | Listening and the Listen buttons stay off. Nothing else depends on audio. | Brian |

---

## Escalation Criteria

**Escalate to Brian when:**
- [ ] **An abandon line is hit.** Examples: a second initiative opens before Stage 1 closes (A2), or any other abandon line in ADR-0033. Read ADR-0033 for the full list.
- [ ] **Flag rate is over 10%.** The type gets one fix. A second miss means retirement is on the table.
- [ ] **The pool is short.** Today offers the easy lesson because fewer than 2 new T0 words are ready, and it happens again in the next readout.
- [ ] **Brian says "same again"** in week 2. Change the content mix, not the lesson count.
- [ ] **A retire-rule trigger fires** at day 14, 28, or 42.
- [ ] **Stage 1 is not closed by 2026-11-30**, or the word lesson cannot start by 2026-12-07.
- [ ] **A wrong answer key reaches Brian** after the release bar passed. This is a trust bug and the fix comes before new features.
- [ ] **Anything needs a tutor, a human reviewer, or a new voice** to go on. None of these is planned.

---

## Communication Plan

| What | To Whom | When | How |
|------|---------|------|-----|
| **Weekly readout, five lines.** Labels "too early" when counts are small, and never tells Brian to retire a type. Adds pool depth and the number of heavy days. | Brian | Weekly | Written summary from the agent. |
| Count of true missing words (I-1) | Brian | When done | Short note with the count per class. |
| Backfill progress (I-2, I-3) | Brian | In the weekly readout | One line: words entered, words recorded, newest class covered. |
| Open decision on the baseline freeze (I-9) | Brian | Before the backfill ends | Direct question. |
| Retire rule and tripwire (I-6) | Brian | Before slice 1 | One page for agreement. |
| Risk or abandon-line alert | Brian | As needed | Direct message. |
| Gate report | Brian | 2026-11-30 | Short note: Stage 1 closed, gate items met or not. |
| Day-14, day-28, and day-42 results | Brian | On those dates | Short note with the numbers and what the written rule says. Brian decides. |
| Completion report | Brian | End of slice 1 | Short document. |

---

## Related Documents

- Decision Document: `sdd/brainstorms/BRAIN-002-lesson-types-done-right/decision-document.md`
- Options Matrix: `sdd/brainstorms/BRAIN-002-lesson-types-done-right/options-matrix.md`
- Implementation Plan (spec skeleton for SDD `create-initiative`): `sdd/brainstorms/BRAIN-002-lesson-types-done-right/spec-skeleton.md`
- Consensus: `sdd/brainstorms/BRAIN-002-lesson-types-done-right/consensus.md`
- BRAIN-001 Decision Document: `sdd/brainstorms/BRAIN-001-is-the-hebrew-app-built-right/decision-document.md`
- ADR-0033: `docs/decisions/ADR-0033-stage-1-trust-and-measurement.md`
- Lesson design: `docs/design/lesson-architecture.md`

---

**Legend:**
- Priority: P1=Must do now, P2=Should do soon, P3=Can defer
- Effort: S=Small (<1 day), M=Medium (1-3 days), L=Large (3+ days)
