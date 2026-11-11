# Decision Document: Lesson types done right

**Topic:** Lesson types done right
**ID:** BRAIN-002
**Date:** 2026-11-10
**Session type:** Product / Feature (Standard)
**Status:** Recommendation
**Decision-maker:** Brian Nelson (the only learner, the owner, and the developer)
**Participants:** Product Strategist (lead), UX Designer, Technical Architect, Data Analyst, QA Advocate, Devil's Advocate
**Recommendation:** Option D (amended): backfill and study now; word lesson + easy lesson as the first build
**Confidence:** Medium-High. Five of six specialists chose D, and the facts it rests on were checked in the repo. It is not High because the lexicon gap is not yet counted, there is no accepted voice, the evidence for fixing "feels the same" is weak, and one decision is still open. See [Confidence Level](#confidence-level).

---

## Executive Summary

**Recommendation: Option D (amended).** Start now with data entry and study. Put the tutor's class words into the lexicon and study them in today's review. After the Stage 1 gate (2026-11-30), build two lesson types on one new thin player: the word lesson, and the easy lesson as the lesson for heavy-review days.

Today always offers one lesson after review. Words come from all the loaded class sheets, not only the newest class, because Brian will not always study right after a class. Code builds every lesson. Only words with a checked answer key are graded. Conjugation comes next, once the word lesson shows it is used. Listening waits for a voice Brian accepts. The spelling lesson is cut, because typed write inside the word lesson does that work.

Before any screen is built, four things must happen. The true number of missing words must be counted. The typed-spelling bug must be fixed (review marks the tutor's full spelling wrong today). The retire rule must be agreed. And the backfill must stay data entry, with no new spec, so INIT-038 stays the only initiative in flight.

The panel scored four options. D scored 121 of 180, C 114, B 102, A 74. No option fixes "feels the same" well, so that is a stated bet with a tripwire, not a promise.

---

## Background & Context

### Problem Statement

Decide how the five lesson types should look and flow so a lesson is usable (clear, fast), useful (it teaches words Brian needs for his tutor classes and goal), and effective (the words are remembered on a later day). Then pick the smallest build order that delivers that. Source: `session-context.md`.

### Trigger

BRAIN-001 (2026-11-09) re-centered the app on the tutor's lessons and left a provisional Stage 2 list. `docs/design/lesson-architecture.md` now describes five lesson types. This session decides what to build first and what to cut.

### The learner

- One adult learner, an English speaker, learning modern Hebrew with a tutor on Preply.
- Goal for 6 to 12 months: a simple spoken conversation.
- He will give 20 to 30 minutes a day if sessions are worth it. Weekly study was 0, 35, 57, then 64 minutes over the last month.
- Trust is what stops him. When a lesson looks wrong, he skips it or stops.
- The tutor never looks at this code or app and never will have a login. The tutor gives no review and no data entry.
- Speaking happens with his own AI, outside the app. The app may prep him for it but does not host it.

### Constraints

- **Timeline:** Stage 1 (INIT-038) must close by **2026-11-30**. A new lesson type must start by **2026-12-07** to be judged on proof before S2 (2027-01-04).
- **Process:** One initiative in flight (ADR-0033 D13). A new rule retires an old one. Delete first. Opening a second initiative before Stage 1 closes is abandon line A2. BRAIN-001 found 34 initiatives and 258 commits in a month produced about three hours of study. Avoid that treadmill.
- **Technical:**
  - No computer-read voice is accepted. Brian rejected it on 2026-11-09 (`sdd/complaints.md`).
  - Today hides any untouched learner unit that fails the full validator (`internal/web/shell.go:101-142`). With no accepted voice, `audio.missing` fails every Hebrew step (`validate_audio.go:12-47`).
  - Typed grading counts distance 1 as Hard and 2 or more as Again (`internal/fsrs/typed.go`). The tutor's full spelling gets marked wrong.
  - Attempts carry `unit_id` and `source` but no run id (`migrations/015_attempt_source_self.sql`). Voids only accept four reasons (`migrations/023_answer_voids.sql`).
- **Review:** There is no human reviewer. Brian said: "I have you and Grok. So let's do as much as we can programmatically." Code checks do the work. An AI reviewer may help but never grades.
- **Facts already settled:**
  - The Drive audit passed (gate G2) on 2026-11-09: 286 usable tutor lines, 10 of the last 10 classes loadable. 158 of 350 tutor lines are pointed. Of 974 unique tokens, 742 have no lexicon match (`sdd/initiatives/INIT-038-today-you-can-trust/specs/completion-summaries/SPEC-012-audit.md`).
  - G3 is ruled: Brian may reuse the tutor's text privately, for his own study. It is never shared or published.

### Success Criteria

- **Usable:** one answer-and-feedback pattern in every type; works on a desktop keyboard and on a phone with two thumbs.
- **Useful:** every sitting has something new to learn, drawn from the tutor's classes.
- **Effective:** words are proved on a later day. The app-wide measures stay S1 (120 or more study minutes a week by week 4), S2 (50% of words taught 14 or more days ago proved in "Hebrew from English" by 2027-01-04), and S3 (a held-out probe of 20 listening items and 10 English prompts).
- **Trust:** no step marks a right answer wrong, and no word appears that was not taught.

---

## Options Considered

Scores are the sum of six specialists, each scoring six dimensions from 1 to 5 (so 30 per specialist, 180 in total). Source: `consensus.md`.

| Option | Sum (of 180) | Product | UX | Architect | Data | QA | Devil's |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| A. Full five types, in sequence | 74 | 12 | 11 | 14 | 13 | 13 | 11 |
| B. Word + easy lesson now | 102 | 19 | 18 | 17 | 17 | 16 | 15 |
| C. Study first, build one thing | 114 | 18 | 19 | 18 | 19 | 19 | 21 |
| D. Hybrid: backfill and study now, word lesson first | **121** | 21 | 20 | 20 | 20 | 20 | 20 |

Mean score per dimension, across the six specialists (1 to 5):

| Dimension | A | B | C | D |
| --- | ---: | ---: | ---: | ---: |
| Learning effect | 3.2 | 3.7 | 3.5 | 3.8 |
| Trust | 2.0 | 2.8 | 3.2 | 3.8 |
| Time to new study | 1.0 | 2.2 | 2.7 | 3.2 |
| Build cost and treadmill | 1.0 | 2.2 | 4.0 | 3.2 |
| Fixes "feels the same" | 3.2 | 3.2 | 2.2 | 2.3 |
| Measurable | 2.0 | 3.0 | 3.5 | 3.8 |

### Option A: The full design, built in sequence

**Description:** All five types, one after another, as in `docs/design/lesson-architecture.md`.

**Pros:**
- Best on "feels the same" (3.2).
- Complete design, nothing left out.

**Cons:**
- Worst on trust (2.0), time to new study (1.0), and treadmill risk (1.0).
- Listening and spelling depend on a voice Brian has not accepted.
- Repeats the BRAIN-001 pattern: many specs before any study.

**Specialist support:** None preferred it. All six scored it lowest (11 to 14 of 30).

### Option B: Word lesson plus easy lesson now

**Description:** Code builds both. Grid, meaning check with a second ask, a tutor-line passage (AI story optional), typed write with accept-sets. Review first in Today. Conjugation next. Listening waits for a voice. Spelling is cut.

**Pros:**
- Today always has a lesson.
- Uses the tutor's lines, so each class brings its own topic.

**Cons:**
- Two builds before any evidence.
- The lexicon gap blocks it, and it does not say what happens before the gate.

**Specialist support:** Not preferred by anyone. Scores 15 to 19 of 30.

### Option C: Study first, build one thing

**Description:** Backfill and record the newest classes' words. Study them for a week in today's review and existing screens. Then build the word lesson only. Every other type needs a trigger agreed in advance.

**Pros:**
- Cheapest. Best on build cost (4.0).
- Tests the plan before building.

**Cons:**
- Weakest on "feels the same" (2.2). Review alone repeats the old complaint.
- Slower to anything new on screen.

**Specialist support:**
- Preferred by: Devil's Advocate (21 vs 20 for D).
- Opposed by: Product, UX, Architect, Data, and QA scored D at least 1 point higher.

### Option D: Hybrid (recommended, amended)

**Description:** Count and backfill the class words now, record them as taught, and study them in review. This is data entry only. After the gate, build the word lesson on a thin player. R1 added the easy lesson to slice 1 as the heavy-day lesson. Details are in [Recommendation](#recommendation).

**Pros:**
- Leads on trust (3.8), time to new study (3.2), and measurability (3.8), and on learning effect (3.8).
- Puts the one blocker (the lexicon) first, which every option needs anyway.
- New class words reach review within days, long before any new screen.

**Cons:**
- Weak on "feels the same" (2.3), like every option.
- The first new lesson screen lands after 2026-11-30, about 3 weeks away, not 2.

**Specialist support:**
- Preferred by: Product (21), UX (20), Architect (20), Data (20), QA (20).
- Opposed by: none. The Devil's Advocate preferred C by one point but scored D at 20 and says D absorbs his principle of "one build, study first."

---

## Recommendation

### Primary Recommendation

#### Option D (amended): backfill and study now; word lesson + easy lesson as the first build

Every line that R1 changed is marked **(R1)**. R1 is Brian's answer on heavy-review days: "I won't be doing this just after a tutor lesson. So we need lessons that don't always align to my tutor's lessons but can help me keep learning."

### Before the Stage 2 gate (now to 2026-11-30): data entry and study only, no new specs

1. **Count the true missing words** after stripping prefixes (ו, ה, ב, ל, מ, ש, כ) and inflected forms. **(R1)** Count across all 10 audited classes, not only the newest 3, so the size of the archive is known.
2. **Backfill T0 entries only**, with `upsert_lexemes`. T0 means the tutor's own document shows the word with its English gloss beside it. Mark each entry for re-checking once a full-spelling column exists.
   - **(R1)** T0 is decided per sheet. Only dated class sheets count, never study notes, the zip, or the html.
   - **(R1)** Before the gate, start with the newest 3 classes. The archive then becomes a checked queue. Older sheets join in date order, each after it passes the per-sheet check. The 75 sheets not yet audited need that audit first.
   - If the count is over about 100 true missing words, the first release teaches only the T0 words.
3. **Record those words as taught** with `record_taught_words`, and study them in today's review. **(R1)** Every word carries its sheet reference. This runs through the existing MCP tools. No new spec is created, so rule A2 is not triggered.
4. **Fix typed grading inside INIT-038, as a Stage 1 trust bug.** Brian decided this.
   - **The bug:** review marks the tutor's full spelling wrong. An extra ו or י is graded Hard, and two are graded Again (`internal/fsrs/typed.go`). The lexicon has no column for full spelling.
   - **The fix:** an accept-set of spellings per word; a safe fold of ו and י, allowed only when no other taught word folds to the same letters (so בית and בת stay distinct); and a fixed test table of about 150 rows.
   - This is one trust-fix spec inside INIT-038. It is the only code before the gate. If it is not done, keep the backfilled words out of typed review until it ships.
5. **Agree the retire rule and the "same again" tripwire** (below).

### Slice 1, after the gate: word lesson plus easy lesson, on a new thin player

- **Word source (R1):**
  - The pool is every T0 word from a loaded class sheet that Brian has no card for, with no open flag or suspension, that passes the entry checks.
  - Order: (1) words the tutor corrected, (2) words used in two or more classes, (3) words that appear in a tutor line, (4) the most recent class, as a tie-break only.
  - Each lesson draws from a class other than the previous lesson's, so topics rotate.
  - No words from outside the classes in the first release. No tier exists for them, and showing them would break the taught-only rule.
- **Label (R1):** the source first ("from class of 14 Oct"), the topic second.
- **Floor (R1):** a word lesson needs at least 2 new T0 words and 4 graded items. With fewer, Today offers the easy lesson.
- **Steps of the word lesson:**
  1. A grid of the new words: one card at a time on a phone, the full grid on a desktop.
  2. A meaning check, with a second ask after a miss.
  3. A passage of the tutor's own lines that use the new words (G3 allows this). The AI story is an optional extra and the lesson never waits for it. **(R1)** If no tutor line fits, the step is dropped, never filled with an AI sentence, and the stage bar shows only the stages that remain.
  4. Typed write, with the accept-sets.
  5. Word order, as an ungraded reveal.
- **Easy lesson (R1):** the lesson for heavy-review days. No new words. It runs on the same thin player, not the old `lesson.go`, so the old player can be archived later.
- **Graded keys:** T0 or T1 lexicon words only. **(R1)** T1 is unavailable in the first release (no second source exists), so in practice every key is T0.
- **No audio needed.** Every step works silently. Listen buttons appear only where a clip exists.
- **Assembly:** code builds and freezes the lesson when Brian taps Start. Wrong-answer buttons are drawn fresh at each question. The lesson type is stored on the code-built unit.
- **Validation:** code-built lessons get their own release check (the three tests below). They skip the old author-unit validator's `audio.missing` and shape rules. Those rules are deleted only when the old units are archived, not before.
- **Flag control:** extend `internal/web/also_right.go` so one tap voids the answer, flags the item, and suspends it from lessons. It shows on right answers too ("This question is wrong") and has Undo.
  - This needs one new void reason in a small migration.
  - Report is changed to act on one item, not the whole lesson (`internal/web/shell.go:396-400`).
  - No flag table or fix-queue tool until there is real flag data.
- **Today:**
  - Review comes first, with a soft cap of 12 to 15 minutes. Due cards that don't fit roll over, most overdue first.
  - **(R1)** Today always offers one lesson after review, and its time is reserved before review is counted. Review under the cap gets a word lesson. Review over the cap gets the easy lesson, and the rest of review rolls over.
  - **(R1)** Today shows review only when no passage passes its checks, and it says so ("no lesson today: nothing passed its checks").
  - **(R1)** Runs on heavy days are tagged "heavy-day".
- **Speaking prep:** a copyable "this week's words and the tutor's sentences" page for Brian's own AI. Check first whether the `hebrew://tutor` MCP resource already covers it.

### Slice 2

- **Conjugation lesson (R1):** started after the word lesson's day-14 use check. It needs no new words and no voice. Its keys are only forms that are stored, or attested in the class text, on verbs Brian already knows.
- The easy lesson moved up into slice 1 **(R1)**.

### Deferred or cut

| Item | Decision |
| --- | --- |
| Listening lesson | Deferred until Brian accepts a voice. S3's 20 listening items are frozen now; its 10 English prompts can run now. |
| Spelling lesson | Cut. Typed write in the word lesson does that work (RSCH-005). |
| Letters check (P-09), tutor note (P-11), generated docs, graded comprehension | Cut. |
| Graded word order | Cut for now. Hebrew word order is often free, so one tutor line is not one right answer. Revisit after two weeks of flag data. |
| Words from outside the classes | Not in the first release. Revisit once a second source exists. |
| New logging spec (P-10), flag table, fix-queue tool | Not built. Use what exists. |

### Release bar for slice 1: three tests, not nine gates

1. **The perfect-student run** passes on at least 6 fixed sample lessons, and a wrong-student run fails on every graded step.
   - **(R1)** The fixed set includes at least 2 lessons from sheets outside the newest 10 classes, and at least 1 with a held line.
   - **(R1)** A lesson with zero graded keys fails.
2. **The typed accept-set table** (about 150 rows) passes.
3. **No graded key is below T0 or T1**, and no word on screen is untaught. **(R1)** In practice every key is T0.

Everything else is handled by the flag loop.

### Retire rule (agreed before slice 1)

Counted from a type's first run on Today.

- **Day 14 (use only):** retire if there are fewer than 6 runs, or if Brian finished fewer than half of his last 6 or more runs. If more than 10% of answers are flagged, the type gets one fix, not retirement.
  - **(R1)** "Runs" means lessons offered and started under the written rotation, because the app chooses what it offers.
- **Day 28 (proof):** needs at least 8 runs and 15 words. Retire if the later-day right rate is 15 points or more below the baseline. The baseline is the backfilled words studied in review only, from the same weeks. With fewer runs or words, wait until day 42 and decide on use alone.
  - **(R1)** The baseline is a frozen set of at least 15 backfilled words, kept out of lessons until day 42. This is Brian's decision (still open, below). If he declines, the type is judged on use alone.
- **Start-by date:** a type must start by **2026-12-07** to be judged on proof before S2 (2027-01-04).

### "Same again" tripwire

If Brian says lessons feel the same in week 2, change the content mix: the passage source, the word count, and the form of the "use it" step. Do not add a lesson type.

### Minimal measurement

- **Lesson type:** stored on the code-built unit. A run gets it through its unit. No new tables.
- **Joining answers to runs:** attempts carry `unit_id` and `source` but no run id. Join by unit and time window, or add a run id in the slice-1 migration. Decide this before building.
- **S1:** add active ungraded time (screen focused, input within the last 60 seconds), shown separately from graded time.
- **Voids:** kept on file. A voided answer counts as neither right nor wrong. Accuracy is shown as a range: voids counted as misses, and voids dropped.
- **Flag rate:** about 3% is healthy. Over 10% means one fix.
- **No hold-out group.** Brian's goal is to learn, not to run an experiment.
- **Weekly readout:** five lines. It says "too early" when counts are small and never tells him to retire a type. **(R1)** It adds pool depth (ready, untaught T0 words) and the number of heavy days.
- **(R1) Class usefulness:** class words proved, grouped by how long ago the class was, and split by whether the word was first met in a lesson or only in review. No timing tied to class. This is a rough read, and it is labelled as one.

### Rationale

1. **The lexicon is the blocker for every option, so it goes first.** The grid, the meaning check, and the write step all read the lexicon. The Technical Architect and Devil's Advocate both put the count and backfill ahead of any screen. Doing it now also gives Brian new class words in review within days.
2. **Trust is the product.** Code builds every lesson. Only T0 keys are graded. One flag control lets Brian take back a bad grade and suspends the item. The QA Advocate's rule: if code cannot prove the key, the step is ungraded (ADR-0033 D3).
3. **It answers Brian's own words.** He said "whatever helps me learn Hebrew better" and "I won't be doing this just after a tutor lesson." So the pool spans all classes and there is always a lesson.
4. **It avoids the treadmill.** Before the gate: data entry plus one trust-fix spec inside INIT-038. After the gate: one thin player, two lesson types, three release tests, and a retire rule agreed in advance.
5. **Tutor lines remove the dependency on the AI story.** G3 lets the passage be the tutor's own real Hebrew, tied to his class. The AI story becomes optional.

### Confidence Level

**Medium-High.**

Why it is high enough:
- Five of six specialists preferred D, and the sixth scored it 20 of 30 (one point under his pick).
- The key facts were checked in the repo: the gate facts, the typed-grading code, the validator hook, and the void reasons.
- After R1, all six specialists changed details and none changed their preferred option.

Why it is not High:
- The true lexicon gap is not counted (742 of 974 is an upper bound).
- No accepted voice, so listening and every audio step are blocked.
- No option fixes "feels the same" well. It is a bet with a tripwire.
- The panel scoring was not re-run after R1. The recommendation was amended in place.
- n=1, so any readout is noisy.
- One decision is still open (the 15-word baseline).

### Accepted Trade-offs

| Trade-off | Why Accepted |
| --- | --- |
| The first new lesson screen lands after 2026-11-30, about 3 weeks away, not 2. | New class words reach review within days. About a week of slack remains before the 2026-12-07 start-by date. |
| Only T0 words are graded. A word that is not T0 is shown but not quizzed. | A wrong key breaks trust. T1 needs a second source that does not exist yet. |
| No words from outside the classes in the first release. | No tier exists for them, and it would break the taught-only rule. |
| Heavy days get the easy lesson, not a small word lesson. | New words add review load on days review is already over its cap. The easy lesson's keys have already passed checks. |
| Graded word order is cut. | Hebrew word order is often free. Revisit after two weeks of flag data. |
| The old validator stays. | Deleting it early adds risk. It goes when the old units are archived. |
| Evidence on class usefulness is rough (no hold-out group). | Brian will not hold back a quarter of his words, and he should not have to. |

---

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation Strategy | Owner |
| --- | --- | --- | --- | --- |
| **Lexicon gap.** 742 of 974 tokens have no lexicon match. Some are prefixed or inflected forms, so the true gap is unknown. A wrong gloss in week 1 breaks trust. | High | High | Count first, after stripping prefixes and inflections, across all 10 audited classes. Backfill T0 only. If the count is over about 100, teach only T0 words. A non-T0 word is shown but never quizzed. | Agent (backfill); Brian reviews the count |
| **Typed spelling.** Review marks the tutor's full spelling wrong (extra ו/י graded Hard or Again). It will go live in lessons the moment they use `GradeTyped`. | High (live in review today) | High | Fix now inside INIT-038: accept-set per word, safe ו/י fold, about 150-row test table. Until it ships, keep backfilled words out of typed review. | Brian and agent, inside INIT-038 |
| **No accepted voice.** Brian rejected the computer-read voice. Listen, meaning check by ear, dictation, listening lesson, and the S3 listening probe are blocked. `audio.missing` also hides code-built lessons from Today. | High | Medium | Design every first-release step to work silently. Code-built lessons use their own release check. Listening is deferred. Run no `generate_audio` batch until Brian names a source he accepts. | Brian (choose a voice); agent |
| **75 of the class sheets are unaudited.** Only 10 were audited. Older sheets may fail the per-sheet T0 check. | Medium | Medium | Audit each older sheet before it joins the pool, in date order. The archive is a checked queue. Pool depth is on the weekly readout. | Agent |
| **Timing.** The gate is 2026-11-30 and the start-by date is 2026-12-07. A slip leaves no time for a day-28 proof before S2 (2027-01-04). | Medium | High | Keep pre-gate work to data entry plus one trust-fix spec. Slice 1 stays at two types and three release tests. If a type starts late, judge it on use alone. | Brian |
| **Treadmill risk.** BRAIN-001 found 34 initiatives and 258 commits produced about three hours of study. | Medium | High | No new initiative before the gate (A2). Delete first (D13). Release bar is three tests, not nine gates. No flag table or fix-queue tool until there is flag data. Study time (S1) is watched weekly. | Brian |
| **Noise with n=1.** One learner, small counts, and a new feature lifts minutes for a while just because it is new. | High | Medium | Labelled "too early" at small counts. The readout never tells him to retire a type. Retire rule has minimum runs and words. Day 14 judges use only. No p-values. | Data Analyst's rule, run by agent |
| **"Feels the same" is not fixed by fixed stages.** Brian named both the screens and the content. The stages stay fixed. | Medium | Medium | Variety comes from content: a different class, passage, and word count each time. Tripwire: if he says "same again" in week 2, change the content mix, not the number of types. | Brian |
| **The easy lesson is a new dependency.** It must run on the thin player, or the old `lesson.go` cannot be archived. | Low | Medium | Build it on the new player from the start (Architect's condition). | Agent |

---

## Dissenting Views

### Devil's Advocate
**Position:** Prefers Option C (21 vs 20 for D). Keep typed write out of slice 1. Keep the validator rather than bypass it. Wanted 1 to 2 words on heavy days.
**Rationale:** Review already does typed recall, and keyboard practice showed no vocabulary effect (RSCH-005). Fewer new parts means less treadmill. The AI story is the part most likely to have stilted Hebrew.
**Response:** Partly accepted. The validator is not deleted. Code-built lessons get their own check instead, and the old rules go only when the old units are archived. The AI story is optional and never graded. Typed write stays in because producing a spelling after it is hidden is the spelling task (RSCH-005) and it replaces the cut spelling lesson. Heavy days get the easy lesson, not 1 to 2 words, because new words add review load.

### Product Strategist and UX Designer
**Position:** The easy lesson should be in slice 1, so Today always has a lesson. (R1: the UX Designer later favored a word lesson with 2 to 4 words on heavy days.)
**Rationale:** Review alone repeats the old complaint.
**Response:** Accepted. The easy lesson is the heavy-day lesson in slice 1. The UX Designer's 2 to 4 word option and the Devil's Advocate's 1 to 2 word option are recorded but not chosen.

### Product Strategist
**Position:** Revisit graded word order.
**Response:** Accepted as a review date: after two weeks of flag data.

### Technical Architect
**Position:** The easy lesson must run on the new thin player, not the old `lesson.go`.
**Response:** Accepted, so the old player can still be archived.

### UX Designer and Data Analyst
**Position:** The first new lesson screen lands after the gate, about 3 weeks away, not 2.
**Response:** Recorded as a trade-off. New class words reach review within days. About a week of slack remains before 2026-12-07.

---

## Decisions

### Made by Brian

1. **Phase 2 answers.** Overall direction: "I want to know Hebrew better. However we achieve that."
   - Study with the new lessons after about 2 weeks.
   - Lessons felt the same in both the screens and the content.
   - Desktop with a keyboard, and a phone held sitting down with both thumbs. Sound is normally available.
   - Typing Hebrew is fine. A Hebrew keyboard is installed.
   - When the app is unsure an answer is wrong, grade it, and give an "I was right" or "this is dumb" button that erases the miss and flags the item.
   - No human reviewer: "I have you and Grok. So let's do as much as we can programmatically."
   - Everything else was delegated to the panel.
2. **Tutor clarification.** The tutor never looks at this code or app and never gets a login. Anything that needs tutor input or review is out. Speaking practice happens with his own AI, not in the app.
3. **G3: yes.** Private reuse of the tutor's text for his own study, covering sentences, word lists, and everything else. Never shared or published. ADR-0033 G3 is now decided.
4. **R1 (heavy days).** "I won't be doing this just after a tutor lesson." So lessons must not depend on a recent class, there must always be something new, and a review-only Today is not the default.
5. **Typed-grading fix now.** Inside INIT-038, as a Stage 1 trust bug.
6. **The agent does the backfill.** Through the MCP, data entry only, T0 words.

### Still open

- **Freeze about 15 backfilled words out of lessons until day 42, as the day-28 baseline.** If Brian says yes, the day-28 check works. If he says no, the day-28 check is dropped and each type is judged on use alone.
- Related: Brian must confirm the typed-grading fix fits INIT-038 under D13 (one initiative in flight). He decided to fix it now inside INIT-038. If the fit is not confirmed, keep backfilled words out of typed review.

---

## Next Steps

The detailed plan is in [action-plan.md](action-plan.md). The proposed specs are in [spec-skeleton.md](spec-skeleton.md).

**The Stage 2 initiative is NOT created until the BRAIN-001 gate passes.** The gate needs the Stage 1 exit (by 2026-11-30) and enough evidence under the new rules. G2 (Drive audit) and G3 (tutor text) are already met. The spec skeleton replaces the provisional Stage 2 list in BRAIN-001 `spec-skeleton.md` §5.

**Pre-gate work is data entry plus one trust-fix spec inside INIT-038.** No second initiative opens (A2).

| Action | Owner | Target Date | Dependencies |
| --- | --- | --- | --- |
| Count the true missing words across the 10 audited classes | Agent | Before 2026-11-30 | None |
| Backfill T0 entries for the newest 3 classes with `upsert_lexemes`; record them with `record_taught_words` | Agent | Before 2026-11-30 | Count |
| Study the backfilled words in today's review | Brian | From the first backfill | Backfill |
| Typed-grading fix (accept-sets, ו/י fold, about 150-row table), inside INIT-038 | Brian and agent | Before 2026-11-30 | Brian confirms D13 fit |
| Decide the open item: freeze about 15 baseline words | Brian | Before slice 1 | Backfill |
| Audit the 75 unaudited sheets, in date order | Agent | Ongoing, before each joins the pool | Per-sheet T0 check |
| Brian names a voice he accepts (unblocks listening) | Brian | Not scheduled | None |
| Stage 1 exit; gate review | Brian | 2026-11-30 | INIT-038 closes |
| Create the Stage 2 initiative from `spec-skeleton.md` | Brian and agent | After the gate passes | Gate |
| Slice 1 live on Today; first run starts | Brian | By 2026-12-07 | Initiative, build |
| Day-14 use check on the word lesson; start slice 2 (conjugation) | Brian | Day 14 after first run | Slice 1 |

---

## Process notes

Phase 3 subagents (Data Analyst, QA Advocate, Devil's Advocate) hit repeated usage-policy blocks that gave no reason. At Brian's request, later phases used Sonnet 5.5 and Haiku 5.5 to cut cost. The Data Analyst's Phase 3 run used a reduced reading list: the session context and its persona only. Phase 5 scoring and the R1 re-evaluations ran on Haiku 5.5. See `dialogue.md` and `consensus.md`.

---

## Appendix

### A. Full Specialist Perspectives

- [Product Strategist](specialists/product-strategist.md)
- [UX Designer](specialists/ux-designer.md)
- [Technical Architect](specialists/technical-architect.md)
- [Data Analyst](specialists/data-analyst.md)
- [QA Advocate](specialists/qa-advocate.md)
- [Devil's Advocate](specialists/devils-advocate.md)

### B. Related Documents

- [session-context.md](session-context.md): the goal, learner, decisions already made, Phase 2 answers, and R1.
- [dialogue.md](dialogue.md): Phase 4 structured dialogue, tensions, and options.
- [consensus.md](consensus.md): Phase 5 scores, the amended recommendation, and decisions. Authoritative.
- [action-plan.md](action-plan.md) and [spec-skeleton.md](spec-skeleton.md).
- `docs/design/lesson-architecture.md`: the five-type design.
- `sdd/brainstorms/BRAIN-001-is-the-hebrew-app-built-right/decision-document.md`: the parent decision and its gate.
- `docs/decisions/ADR-0033-stage-1-trust-and-measurement.md`: D3, D4, D10, D13, A2, S1 to S3, G3.
- `sdd/assessments/ASMT-006-hebrew-lesson-types/ASMT-006-hebrew-lesson-types-assessment-report.md`: what the code does today.
- `sdd/initiatives/INIT-038-today-you-can-trust/specs/completion-summaries/SPEC-012-audit.md`: the Drive audit.

### C. Information Evolution (R1)

What changed when Brian answered the heavy-review-day question. Source: `consensus.md`, "Information injection R1".

| Specialist | Before R1 | After R1 |
| --- | --- | --- |
| Product Strategist | The newest class's words in one word lesson; easy lesson as a fallback on heavy-review days | Always a lesson. Words come from all classes, ranked by need, with the newest class only as a tie-break. Conjugation moves up. |
| UX Designer | Review first, then the lesson; same stages per type | Reserve a lesson slot. Label the source first (class date), topic second. Drop a step when no tutor line fits. |
| Technical Architect | A first slice of the word lesson only | A ranked T0 pool across every loaded class. The easy lesson moves into slice 1 as the heavy-day lesson. Pool depth added to the readout. |
| Data Analyst | Judge each type keep or retire; no hold-out group | Measure class usefulness by class-age band. Freeze a small baseline set. Count offers and starts in the use check. |
| QA Advocate | Lexicon tiers T0, T1, T2; grade only provable keys | Apply T0 per sheet. A minimum-lesson floor. The golden set includes older sheets. Words outside the classes are never shown in the first release. |
| Devil's Advocate | Backfill the newest classes, study first, build one thing | Draw from the whole archive in rotation. The backfill becomes a checked queue. Conjugation is triggered by an empty queue. Words outside the archive are exposure only. |

**Outcome:** all six changed details and none changed their preferred option. Scoring was not re-run. The recommendation was amended in place.

**Impact on the plan:**

| Area | Before R1 | After R1 |
| --- | --- | --- |
| Which words a lesson teaches | Newest class | Ranked pool across all loaded classes; newest is a tie-break |
| What Today offers on a heavy day | Review, or review plus a lesson | Always one lesson. Easy lesson over the cap. |
| Easy lesson | Added only if heavy-review days appear | In slice 1 |
| Conjugation | Third | Slice 2, after the word lesson's day-14 use check |
| Class-usefulness measure | Not specified | By class-age band, split by lesson vs review first met |
| Backfill scope | Newest 3 classes | Count all 10 audited classes; start with 3; older sheets join as a checked queue |

**Settled splits:**
- Heavy days: easy lesson (Product, Architect, QA) chosen over a 2 to 4 word lesson (UX) or a 1 to 2 word lesson (Devil's Advocate).
- Words outside the classes: none in the first release (QA: never; Devil's Advocate: exposure only).
- Conjugation timing: slice 2, after the day-14 use check (Product: next, no trigger; Devil's Advocate: when the queue empties).
