# Data Analyst — Brainstorm Analysis

**Session:** Lesson types done right
**Date:** 2026-11-10
**Domain:** Product & Design
**Primary Lens:** Measurability and data-driven decisions, with small-sample care (one learner)

---

## Phase 3: Independent Analysis

### Chain of Thought

#### Primary Lens

I ask one question: how will Brian know a lesson type works, and not just feel that it does? He is the only user. So the usual tools (A/B tests, significance) do not apply. I will use counts, simple comparisons, and rules agreed in advance. I will not use p-values.

#### Initial Assessment

- The app already has the right outcome measures: S1 (minutes), S2 (words proved), S3 (listening probe). They judge the **whole app**.
- The five lesson types need something smaller. They need a signal that says "keep this in Today" or "take it off". That signal cannot prove cause. It can only say "this type is not helping."
- A "proved" word needs checked right answers on 3+ days spanning 7+ days. So proof is slow. The earliest a word can be proved is 7 days after its first lesson. In practice it is 10 to 14 days.
- Words are proved by review, not by the lesson. A lesson only introduces and warms up a word. So any per-type proof number is "words this type introduced," not "words this type taught." I must say that plainly in the summary.
- Brian's study time was 0, 35, 57, 64 minutes a week. That is four noisy points. A new feature will lift minutes for a while just because it is new.
- There is a good rule already in place: "no new machinery." I will ask for as little new logging as I can.

#### Main Considerations

**1. Per type or whole app? Both, with different jobs.**

- **Whole app decides "is it working."** S1, S2 and S3 are the verdict. They are the only numbers I would show as success or failure.
- **Per type decides only "keep or retire."** It uses two things:
  - *Use:* did he start it, and did he finish it?
  - *Later-day right rate:* of the checked answers on words this type touched, how many were right on a **later day**? The app already reports this by kind of practice (`practiceResults`). I would reuse it and add no new system.
- Each type gets one named job measure, so we do not compare apples to oranges:

| Type | Job measure |
| --- | --- |
| Word lesson | Later-day right rate on its new words; feeds S2 |
| Conjugation | Later-day right rate on forms |
| Spelling | Later-day right rate on typed words |
| Easy | Finish rate and minutes (it teaches no new words, so no proof is expected) |
| Listening | S3 only, and it is blocked until a voice is accepted |

- With n=1, a type is "not helping" only if it is clearly worse than the baseline or he does not use it. Better than baseline is **not** proof that it helps.

**2. A retire rule, agreed before building.**

Write this into the decision document before any lesson type is built. Day 0 is the type's first run on Today.

- **Day 14: usability check.** Proof is not ready yet, so judge use only. Retire if either is true:
  - fewer than 6 runs started in 14 days; or
  - he finished fewer than half of his last 6 or more runs.
- **Day 14: flag check.** If flags are over 10% of graded answers, the type gets **one** fix. It is not retired for this.
- **Day 28: proof check.** By now, words first taught in days 0 to 14 are at least 14 days old. That matches the S2 window. Compare the type's later-day right rate on those words with the baseline (the app's other words from the same weeks). Retire if all three hold:
  - at least 8 runs;
  - at least 15 words;
  - the type's rate is 15 points or more below the baseline.
- **If counts are below the floor at day 28:** wait once until day 42. At day 42 decide on use alone (runs per week).
- **Latest start for S2:** a type must start by **2026-12-07** (28 days before 2027-01-04) to be judged on proof by the S2 date. Later types are judged on use and flags only.
- **"Retire" means** off Today within a day, and the code deleted in the next spec (ADR-0033 D13: delete first).
- **Only these checkpoints can retire a type.** The weekly summary never does. If Brian keeps a type against the rule, he writes one line, and it is marked "kept by choice."
- The rule fits the 7-day proved rule: a word taught on day 0 can be proved on day 7 at the earliest, and a day-28 check looks at words that are 14 to 28 days old.

**3. Showing the app helps with his weekly classes, with no input from the teacher.**

Class notes arrive as dated documents. That is enough.

- **Primary: a held-out comparison inside the same class.**
  - Take the words from each class document. Use a fixed rule (for example, a hash of the word id) to hold out about **1 in 4** from all lessons. Decide the rule before building.
  - About 14 days after the class, ask a checked question on each held-out word. Do the same for the words the app did teach.
  - Compare the right rate: app-taught words versus class-only words from the same classes.
  - This uses **words** as the sample, not weeks. It is the same learner and the same class, so difficulty is matched.
  - It is biased toward "no difference." Held-out words also come back in later classes. So a gap that still shows is real.
  - Expect about 25 held-out words by 2027-01-04. Only a big gap will show. That is fine. Say "7 of 10 versus 3 of 9," not "70% versus 33%."
- **Secondary: carry-over.** For each class, show how many of its words reached a lesson within 7 days, and how many were proved within 21 days. This needs no hold-out. It says whether the app keeps up with the class.
- **Baseline:** the 85 older class documents are not loaded. Words from classes before the app are a rough "class only" baseline. Do not lean on it. It is not matched.
- **Not usable:** anything that needs the tutor to grade, rate, or enter data.

**4. The "I was right / this question is wrong" button.**

The button erases a miss and flags the question. That is good for trust. It is also a way to dodge hard words. To keep results honest:

- **A voided answer is removed from both sides.** It counts as neither right nor wrong. It cannot add a "right day" toward proof. It can only take away a wrong answer.
- **Keep the void on file** (a soft delete with a reason code). Never hard-delete it. We need it for the flag rate.
- **Show accuracy as a range.** Lower bound: voids counted as misses. Upper bound: voids dropped. If the range is wide, the flag rate is the story, not the accuracy.
- **Flag rate = flags ÷ graded answers**, by type, by question template, and by item.
  - Under about 3%: healthy.
  - Over 10% for a type: the key or the template is probably wrong. That feeds the "one fix" rule above.
  - Two flags on one item: pull the item from circulation until it is checked.
  - A cluster of flags on one template points to a bug in the answer key code. One user's mood does not make a cluster.
- **Sample check:** each week, a code check re-grades about 10 flagged items against the lexicon and the stored forms. An AI reviewer may help, but it never grades. This tells us whether he flags real errors or only hard ones.
- **Watch for drift:** if the flag rate climbs while the same items were fine before, it is fatigue or avoidance. Show it, and do not act on one week.

**5. What to record for each lesson run (kept minimal).**

One row per run. Everything else comes from the answer log by `runId`.

| Field | Why |
| --- | --- |
| `runId` | Joins the run to its answers |
| `lessonType` | Per-type numbers |
| `designVersion` (a small integer) | Keeps old and new designs apart when we change a lesson |
| `startedAt` | When he studied |
| `finished` (yes/no) and `lastStep` | Finish rate, and where he quits |
| `activeSeconds` and `gradedSeconds` | S1 and the graded/ungraded split |
| `flaggedCount` | Flag rate |

The answer log needs: word id, step, result (right, wrong, or voided), time, and `runId`. If it has these already, add nothing else. Do not log scroll, taps on words, or clicks. They are not decisions.

**6. Does ungraded time (word grid, story) count toward S1?**

- **Yes, count it.** Reading the grid and the story is real study. It is also the time he is most likely to skip. S1 is a measure of whether he shows up.
- **But count active time only.** Stop the clock after about 90 seconds with no tap. Otherwise a grid left open inflates S1.
- **Report the split** (graded and ungraded minutes) in the weekly summary. S2 is the learning measure, and it ignores minutes. So S1 cannot be "gamed" without S2 showing it.
- **Keep it comparable:** the earlier weeks (0, 35, 57, 64) must have been counted the same way. If they were not, show the new measure from its start and do not chart the two together.

**7. S3 waits for an accepted voice.**

- **Do not substitute** the computer voice or a text-only version. Brian rejected that voice. A different test would also break the before/after comparison.
- **The 10 English prompts need no audio.** Run them now. They give a real baseline.
- **Prepare the 20 listening items now.** Freeze the list and keep it out of every lesson and review. Only the recording waits.
- **There will be no "before" for listening.** The first run, after a voice is accepted and some lessons have run, is a starting level. Say "trend from the first run," and do not claim that lessons caused the first score.

**8. A one-screen weekly summary he will read.**

Five lines. No chart he has to decode. It fits a phone sitting down.

1. **Minutes:** "74 of 120 this week (52 graded, 22 reading)," and the last 4 weeks as small bars.
2. **Proved:** "Taught 14+ days ago: 11 of 30 proved (target 50% by 2027-01-04)," and "new this week: 3."
3. **By lesson type:** one line each, such as "Word: 5 runs, 5 finished, 8 of 11 right on a later day." Show "too few yet" when there are under 8 answers.
4. **Flags:** "4 flagged, 2% of answers; 1 question pulled."
5. **From class:** "Last class: 9 of 12 words practised. Held-out check at 14 days: 7 of 10 taught, 3 of 9 not."

Rules for the screen:
- Show counts, not percentages, when the count is under 20.
- No red or green on numbers that are not ready. Use "too early."
- One sentence at the bottom: the single most useful next step.
- No retire advice here. Retire only happens at the checkpoints.

**9. Reading too much into noise.**

- Weekly minutes have swung from 0 to 64. One week of 40 or 90 tells us almost nothing.
- A word lesson teaches about 4 words. A type with 12 words touched changes by 8 points when one answer flips.
- The first two weeks of any new lesson look better just because it is new. Check S1 again at weeks 6 to 8.
- Do not change a lesson and read its numbers in the same window. Use `designVersion` so we compare like with like.
- Look at the type numbers **twice**: day 14 and day 28. Looking weekly invites a reading into noise.
- Mark off weeks (travel, illness, no class) with a short note. Do not read them as trends.
- Do not add stats, tests, or models. The data is too small for them.

#### Opportunities

1. **Reuse what exists.** The app already reports later-day right rate by kind of practice. The per-type check can ship with almost no new code.
2. **A within-class test with words as the sample.** The held-out words give a proof-of-use that does not need a tutor and does not need weeks of data.
3. **Flags double as a free quality signal.** They find bad answer keys faster than any reviewer, and Brian has no human reviewer.
4. **Pre-agreed rules end debate.** A retire rule written now avoids the treadmill of BRAIN-001 (34 initiatives, about three hours of study).
5. **The English prompts part of S3 can start today.** That gives an early baseline.

#### Risks and Concerns

| Risk | Level | Why it matters | Mitigation |
| --- | --- | --- | --- |
| **Reading noise as signal.** Retiring a good type, or keeping a bad one, from 1 or 2 words. | **High** | n=1 and about 4 words per lesson. A single answer moves a type's rate. | Minimum counts (8 runs, 15 words). Two checkpoints only. Counts, not percentages. "Too early" label. |
| **The void button inflates accuracy.** He might void hard words. | **Medium** | Selection bias. S2 could look better than it is. | Soft-delete. Show the accuracy range. Weekly sample check of flagged items. Flag rate in the summary. |
| **Per-type proof is muddy.** Review proves words, so a type gets credit it did not earn. | **Medium** | Wrong retire calls, or none. | Call it "words this type introduced." Use a baseline. Never claim cause. |
| **Late types cannot be judged on S2.** | **Medium** | Types started after 2026-12-07 miss the window. | Start order matters. Judge late types on use and flags only. |
| **Held-out words cost him learning.** | **Medium** | 1 in 4 words are not in the app. | Class still teaches them. Brian must agree. It can drop to 1 in 5 or stop. |
| **S1 minutes changing meaning.** Idle time or a changed count breaks the trend. | **Medium** | The 120-minute target becomes meaningless. | Idle cutoff, graded/ungraded split, and a note when the method changes. |
| **No listening baseline.** | **Low** | S3 only gives a trend from the first run. | Say so up front. Run the English prompts now. |
| **Measurement becomes a project.** | **Low** | BRAIN-001 warned about too many specs. | Keep it to one row per run, one screen, two checkpoints. |

#### Recommended Approach

1. **Measure both, with different jobs.** The whole app is judged by S1, S2, S3. Each lesson type is judged only for "keep or retire," by use and later-day right rate. We never claim cause for a type.
2. **Write the retire rule into the decision document before building.** Day 14 for use, day 28 for proof, floors of 8 runs and 15 words, a day-42 fallback, and a start-by date of 2026-12-07 for S2.
3. **Prove the class link with held-out words.** Hold out about 1 in 4 class words by a fixed rule. Check them 14 days after class. Add the carry-over count as a cheap second view.
4. **Voids erase the answer, not the record.** Soft-delete them. Show accuracy as a range. Use the flag rate (about 3% healthy, over 10% means fix once) and a weekly code re-check of flagged items.
5. **Log one row per run** with the seven fields above. Reuse the answer log.
6. **Count ungraded time in S1,** active time only, with the graded/ungraded split shown.
7. **Run the 10 English prompts of S3 now. Freeze the 20 listening items. Wait for the voice** before recording them. Do not substitute a voice.
8. **Build a five-line weekly summary.** Counts, "too early" labels, one next step, no retire advice.
9. **Look twice, not weekly.** Do not change a lesson inside its measuring window.

### Summary

**Position:** Judge the whole app with S1, S2 and S3. Judge each lesson type only to decide "keep or retire," using use and later-day right rate. With one learner, none of it proves cause. It can only show that something is clearly not helping.

**Recommendation:** Agree the retire rule before building (day 14 for use, day 28 for proof, floors of 8 runs and 15 words, start by 2026-12-07). Prove the class link with a held-out set of about 1 in 4 class words. Soft-delete voided answers, show accuracy as a range, and use the flag rate as the quality signal. Log one small row per lesson run. Count active ungraded time in S1 with a split. Run the English half of S3 now, and wait for an accepted voice before the listening half. Show five lines a week.

**Top Risk:** Reading noise as a verdict, which retires a good lesson type or keeps a bad one on a handful of words. Counts are tiny and weeks swing from 0 to 64 minutes.

**Confidence:** Medium. The rules are sound for a sample this small. I do not yet know how many words a week he will learn, how minutes were counted before, or whether the answer log already carries run, type, and void state.

**Questions for Other Specialists:**

1. **Product Strategist:** Will Brian accept holding out about 1 in 4 class words from lessons for the class check? If not, is a weaker "carry-over only" view enough for him? Also, does he agree that "retire" means off Today in a day and code deleted next spec?
2. **UX Designer:** Can the flag button be one tap with no reason, or do we need a two-way choice ("I was right" and "this question is wrong")? Where does the five-line summary live, and does it fit a phone held in both hands without scrolling?
3. **Technical Architect:** Does the answer log already hold `runId`, lesson type, and a void state that can be kept as a soft delete? Can `practiceResults` split by lesson type and `designVersion`? How were the old weekly minutes counted, and can we set an idle cutoff without breaking the comparison?
4. **QA Advocate:** Can a weekly code check re-grade flagged items against the lexicon and stored forms? What should happen to an item with two flags: pulled at once, or after the check?
5. **Devil's Advocate:** At n=1, is per-type measurement worth the work? My fallback is simpler: "if he stops opening a type for two weeks, drop it." Would you accept that instead, and keep only the whole-app measures?

---

## Phase 4: Dialogue Contributions

### Questions Posed

1. **To Product Strategist:** Will Brian accept holding out about 1 in 4 class words from lessons for the class check? If not, is a "carry-over only" view enough? Does "retire" mean off Today within a day, with the code deleted in the next spec?
2. **To UX Designer:** Can the flag button be one tap with no reason, or do we need a two-way choice ("I was right" and "this question is wrong")? Where does the five-line summary live, and does it fit a phone held in both hands without scrolling?
3. **To Technical Architect:** Does the answer log already hold a run id, the lesson type, and a void state that can be kept as a soft delete? Can `practiceResults` split by lesson type and `designVersion`? How were the old weekly minutes counted, and can an idle cutoff be set without breaking the comparison?
4. **To QA Advocate:** Can a weekly code check re-grade flagged items against the lexicon and the stored forms? For an item with two flags, is it pulled at once or after the check?
5. **To Devil's Advocate:** At one learner, is per-type measurement worth the work? Would you accept "if he stops opening a type for two weeks, drop it," and keep only the whole-app measures?

*Replies recorded in dialogue.md: Product Strategist answered on the hold-out (declined it). Technical Architect answered on the answer log and voids. No reply is recorded for questions 2, 4, or 5.*

### Questions Answered

- **To Product Strategist, on whether ungraded time counts toward S1:** Yes. Count active ungraded time, meaning time with the screen focused and input in the last 60 seconds. Show it separately from graded time, so a better lesson cannot push Brian toward the abandon line by inflating minutes. *(My Phase 3 file said 90 seconds. The 60-second figure in dialogue supersedes it.)*
- **To Product Strategist, on the 1-in-4 hold-out:** Accepted the natural comparison instead: class words met only in review against class words met in a lesson. Labelled as a rough read, not proof.

### Tensions Involved In

- **Study first, or build first (Tension 1):** Study first, data entry only. The backfill comes first either way. The backfilled words studied in review before the lesson exists give us a baseline.
- **How many lesson types in the first release (Tension 2):** One type first, the word lesson. Each other type gets a trigger and a retire rule written before it is built.
- **Does the AI story earn its place (Tension 3):** No measure is needed for it. If it stays, judge it by use only. It must never be a dependency of the proof measure.
- **How much measurement (Tension 4):** Keep the set small: one row per run, the five-line summary, and two checkpoints (day 14 and day 28). I accepted dropping the 1-in-4 hold-out, so the class check is now a natural comparison.
- **Who certifies the lexicon (Tension 5):** The flag rate and the weekly code re-check are my measure of key quality. I would rather grade fewer words (T0 only) than backfill a large set with no quick way out.
- **The review cap inside Today (Tension 6):** Review first, and the cap stays soft. Review counts toward the later-day right rate, so the cap rule must not change during a measuring window, or the numbers stop comparing.

### Synergies Identified

- **Lexicon backfill as the critical path (with Technical Architect, QA Advocate, Devil's Advocate):** The backfill is also my baseline. Its words, studied in review first, are the clean comparison for the word lesson.
- **Typed accept-sets (with UX Designer, Technical Architect, QA Advocate):** Stops right answers being graded wrong. Without it, the later-day right rate is distorted by the spelling grader, not by the lesson.
- **One flag control, extending `also_right.go` (with Technical Architect, UX Designer, QA Advocate):** One control gives me the void record (soft delete), the flag rate, and the suspend step. This is the single quality signal.
- **Tutor lines as the passage (with Product Strategist, Technical Architect):** Each class has its own content, which gives a natural class link without a hold-out.
- **A lesson-type label on code-built units (with Technical Architect):** Each run gets its type through its unit. No new table is needed.
- **Reuse of `practiceResults` (with Technical Architect):** The later-day right rate by kind of practice already exists. The per-type check needs no new system.
- **Five-line weekly summary (with UX Designer):** This is where every measure lands, on one screen.

---

## Phase 5: Consensus Position

### Option Scores

Scale: 1 (poor) to 5 (excellent). Each cell gives the score and one short reason.

| Dimension | A | B | C | D |
| --- | --- | --- | --- | --- |
| Learning effect (research-backed) | 3: Covers every research strand, but conjugation and typed spelling are untested (RSCH-005 is Low), and listening has no voice yet. | 4: The word lesson rests on moderate evidence (active recall, glosses, coverage-graded input). Spacing comes from review (High, RSCH-003). Easy lesson gives a light day. | 4: Same word lesson as B. Spacing from review (High). No evidence on how many new words per lesson is right (RSCH-004). | 4: Same as C. Easy lesson added only if heavy-review days appear. |
| Trust (no wrong marks, no untaught words) | 2: Five types means five sets of graded keys. Typed spelling marks tutor spelling wrong today. | 3: Two types. Grading only T0 words is a plan, not yet proven. Accept-sets still to build. | 3: Fewer keys, but the backfill is agent-written with no reviewer, and nothing gives a quick exit for a bad key. | 4: Same backfill risk, but the flag control voids, flags, and suspends an item in one step. Three-test release bar. |
| Time to new study (about 2 weeks) | 1: Five types in sequence. Months, not weeks. | 2: Backfill, typed fix, and two builds before any evidence. Tight in two weeks. | 2: A week of study, then the build. Word lesson lands about week 3 or later. | 3: Backfilled words are new in review within days. Word lesson follows the gate (2026-11-30). |
| Build cost and treadmill risk (fewest specs and rules) | 1: Five types, five sets of rules, and the BRAIN-001 treadmill. | 2: Two builds, with conjugation and listening rules queued behind them. | 4: One lesson type, one player, one retire rule. | 3: C's build, plus the flag control and the "same again" tripwire. Still one type. |
| Fixes "feels the same" | 4: Five different shapes give the most variety, but it arrives last. | 3: Two shapes now (word lesson and easy lesson), plus class-specific passages. | 2: One shape. Variety comes only from content, which is a bet. | 2: Same as C. The "same again" tripwire tells us in week 2 if the bet fails. |
| Measurable (can we tell if it works) | 2: Five types each need their own measures. Small counts per type. Cause is hard to see. | 3: Clean proof measure for the word lesson. Use-only for the easy lesson. | 4: One type. Backfilled words studied in review first give a built-in baseline. | 4: Same as C, plus the flag rate as a second signal and the retire rule agreed in advance. |
| **Total (of 30)** | **13** | **17** | **19** | **20** |

### Final Position

**Preferred Option:** D (Hybrid: backfill and study now, word lesson as the only first slice).

**Rationale:** D is the only option that puts new words in front of Brian within days. The backfill is data entry, and the words are studied in review before any new screen exists. It keeps the first build to one lesson type, and that type gets a clean baseline (backfilled words studied in review only) and a flag signal. The retire rule is set before the build, so no one reads noise as a verdict. The word lesson must start after the 2026-11-30 gate. That still meets the 2026-12-07 start-by date for the S2 window, but only with about a week of slack.

**Dissent:** Aligned with consensus on D, with three conditions that change my Phase 3 numbers:
1. **Joining answers to runs.** Attempts do not carry a run id (Technical Architect). Either add a run id to attempts in a small migration, or join by unit and time window. This must be decided before slice 1, not after.
2. **Idle cutoff.** Use 60 seconds, as I told Product Strategist in dialogue. My Phase 3 file said 90 seconds; that is superseded.
3. **Day-28 baseline.** Compare the word lesson against backfilled words studied in review only, from the same weeks. Do not use all other app words, because they are not matched. I also keep the Devil's Advocate's "drop after two weeks unused" as the day-14 use check, but not as the only rule, because it does not test whether the words were learned.

I have no dissent on dropping the 1-in-4 hold-out.

---
## Information Injection R1: Lessons not tied to a recent class
**Injected at:** After Phase 5, before Phase 6

### Re-Evaluation
**Impact Assessment:** High
**Position Change:** Changed
**Analysis:** The core approach holds: judge the whole app with S1, S2 and S3, and judge each lesson type only for keep or retire. But three of my measurement details assumed a lesson follows a class. Once lessons are not tied to class timing, the carry-over measure loses its meaning, and the day-28 baseline stays clean only if its words are kept out of lessons. The retire rule's use signal also breaks, because if Today always offers a lesson, raw run counts reflect the app's rotation rather than Brian's choice.

**Answers to the four questions:**
- **Class-usefulness measure:** No, not as written. The "within 7 days of class" carry-over and the "14 days after class" held-out check both anchor on class timing. Replace them with a class-word proof rate by class-age band (0 to 14, 15 to 28, 29 or more days since class), split by whether the word first met a lesson or only review. Compare only within the same band, or older and newer words get mixed.
- **Day-28 baseline (backfilled words studied in review only):** It still works, but only if frozen. Freeze the set by word id at backfill, and keep it out of lessons through day 42. The set needs at least 15 words after the T0 filter, or the day-28 check cannot run and the type is judged on use alone.
- **Retire rule:** Yes, it needs changing. Use becomes "offered, started, finished" per type, with a fixed written rotation so each type gets a known number of offers. The 6-run floor becomes a floor on offers. The 10% flag rule and the 8-run and 15-word proof floors stay as they are.
- **Consensus:** Yes. The heavy-day "review only" default is now wrong. See the amendment below.

**Prior Recommendation:** Review first with a soft cap; a lesson is offered only once review is under the cap; heavy-review days show review only; class usefulness is judged by carry-over and a held-out comparison; the day-28 baseline is backfilled words studied in review only.
**Revised Recommendation:** Keep the two checkpoints (day 14 use, day 28 proof) and the 8-run and 15-word floors. Tag every word and run with its source (class ID or lexicon-only). Replace carry-over with class-word proof by class-age band. Freeze the baseline set by word id. Measure use as offered and started. Offer a lesson on heavy-review days, tagged as heavy-day in the run record.

**New Risks or Opportunities Surfaced:**
- **Selection bias (High):** The app now chooses which words enter lessons, for example words with lapses or new words. A lesson-met versus review-met comparison then compares hard words with easy ones, so a good lesson can look bad, or the reverse. Record how each word was chosen.
- **Baseline cost:** A frozen set of 15 or more words is held out of lessons for about six weeks. Brian has to accept this. The real count is unknown until the backfill is counted.
- **Word supply:** "Always something new" lasts only as long as the audited classes have unseen words. The count is not yet known. Once supply runs out, lessons draw lexicon-only words, and S2 mixes sources unless it is split.
- **Proof timing:** Review rolls over and puts the most overdue cards first, so heavy review delays later-day tests. The proof window then depends on review load, not only on the lesson.
- **Heavy-day lessons look busy:** A lesson offered on heavy days will show high use because it is offered when review is heavy. Its runs must be tagged and kept out of the normal-day numbers.
- **Opportunity:** The source tag allows a cheap check of whether class-sourced words are proved better than lexicon-only words taught the same way. This is the closest thing to a test of the tutor's value that the app can give.

**Confidence Adjustment:** Decreased. The class link is weaker than I assumed, because lesson timing no longer anchors it and selection bias is now the main threat. The retire rule's logic is still sound. Its inputs need fixing before it is used.
