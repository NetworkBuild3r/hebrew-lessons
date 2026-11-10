# Data Analyst — Brainstorm Analysis

**Session:** Is the Hebrew app built right? (BRAIN-001)
**Date:** 2026-11-09
**Domain:** Product & Design
**Primary Lens:** Measurability. Do the app's numbers describe what Brian can actually do in Hebrew, do the automatic decisions that read those numbers behave sensibly, and can we tell whether a redesign worked?

---

## Phase 3: Independent Analysis

### Chain of Thought

#### 1. Primary lens

I treated the app as a measurement system first. Every number on Progress and Today, and every automatic decision in the lesson pipeline (lesson tier, new-word pace, "review first"), comes from one learner model. If that model's numbers are wrong, unclear, or measured against the wrong denominator, the learner loses his bearings and the pipeline makes bad choices on its own. I read the code that defines each number, then compared it with learner 1's live data through the read-only MCP tools (`get_study_digest`, `get_learner_model`, `get_learner_words`, `get_progress`, `get_context`) at about 14:30 UTC on 2026-11-09. No write tools were called.

#### 2. Initial assessment

The app logs good raw evidence: every checked answer with skill, activity, latency, hint, and the typed or picked response, plus every scheduler review. It then compresses that evidence into a "known" label that needs more proof than the app has collected, decays with review lateness, and is drawn against the whole 1,076-word lexicon. The result is that Brian has a perfect spelling record on 21 of the 22 words he has ever spelled and the app calls 4 of them known, while the same strict counts silently hold his lessons at "beginner, no new words". The measure and the display are both wrong for this learner, and the larger damage is that the lesson pipeline reads the same numbers.

#### 3. What the evidence says

##### 3.1 Why Progress says he knows almost nothing (focus question 1)

**What the page counts.** The overview headline "Words you know" is `standing.KnownAnySkill` (`internal/web/progress.go`, `fillOverview`, computed in `internal/learner/standing.go`). It counts words that are known or solid in at least one of four skills. Today shows the same "Words you know" label (`internal/web/views/today.templ`). Each skill tab instead shows that skill's known plus solid count, or a placement estimate, above a stacked bar of all 1,076 lexicon words (`fillSkillPanel`).

**Live numbers (2026-11-09):**

| Measure | Value |
| --- | --- |
| Words you know (known or solid in any skill) | 36 |
| Listening known or solid | 10 |
| Reading (Hebrew to English) known or solid | 32 |
| English to Hebrew known or solid | 3 |
| Spelling known or solid | 4 |
| Words known in every skill they have evidence in (strict overall) | 2 of 110 words with evidence, 0 solid |
| "Learned today, check tomorrow" | 2 |

**The rule.** A skill is known only when all five of these hold (`IsKnown` and `StateOf` in `internal/learner/state.go`, counts from `LearnerEvidence` in `internal/db/learner.go`):

1. A scheduler card exists for that word and skill.
2. The card has at least 3 checked successful reviews (`LearningSuccesses = 3`).
3. The newest 3 checked answers are all right (`KnownRecentRight = 3`).
4. Checked successes fall on at least 2 study days (`KnownStudyDays = 2`, ADR-0031 D1, rule version 4).
5. The scheduler's recall probability at this moment is at least 0.85 (`KnownRetrievability`).

The page explains this as one sentence: "A word counts as known once you get it right on a second day." (`internal/web/views/progress.go`, line 392). That sentence leaves out conditions 1, 2, 3, and 5. A learner who spells a word right on two days and then opens Progress is told a rule the app is not applying.

**His answer history against that rule** (per word and skill from `get_learner_model`):

| Skill | Words answered | Answers | Words never answered wrong | Words with 2 or more answers, none wrong | Known or solid |
| --- | --- | --- | --- | --- | --- |
| Spelling | 22 | 45 | 21 | at least 13 | 4 |
| English to Hebrew | 51 | 140 | 25 | at least 11 | 3 |
| Listening | 75 | 155 | 61 | at least 33 | 10 |
| Reading | 78 | 401 | 33 | at least 30 | 32 |

The fifth column comes from paged word lists that can miss a few high-id words, so it is a floor.

Spelling is the clearest case. He has 45 spelling answers spread over 22 words, about 2 per word. 21 of the 22 words have never been spelled wrong. Of the 17 perfect-record words that are not known, 16 have only 1 or 2 spelling answers ever, so condition 2 cannot be met yet. The 17th, אַתָּה, has 3 right answers, but its card is 5 days overdue and its recall probability has decayed to the 0.85 line, so condition 5 fails. On the same Progress row, Spelling carries the verdict "strong" (97% right over 33 answers in 14 days, ADR-0028 D5). One row tells him he is strong and that he knows 4 words. Nine of his 20 spelling cards on the first page were last rated Hard, which for typed answers means right after a hint, right but slow, or one letter off (`internal/evidence/grade.go`, `spell.go`). The 97% counts all of those as right.

**Is the measure wrong, the display wrong, or both? Both.**

- **The measure.** "Known" is a defensible definition of "proved and likely recallable today", but it needs at least three checked asks per word per skill, and the app has asked each word about twice per skill. It also decays with the calendar. On the go-fsrs default curve (recall probability equals (1 + 19/81 × days elapsed / stability) to the power of −0.5), recall probability is 0.90 when a card falls due and drops under 0.85 once the review is about 64% of its interval late. With 2 to 4 days between study days, words will drop in and out of "known" with nothing changing in his head. It mixes "has he proved it" with "is he up to date on reviews" and reports both as one word.
- **The display.** "Learning" lumps a word spelled right every time but asked once together with a word he gets wrong half the time. The skill bars use all 1,076 lexicon words as the denominator (spelling has 1,028 unseen), so even real progress fills less than half a percent of the bar. The Listening tab shows a placement estimate of about 503 words next to 10 known from evidence. The explanation sentence is incomplete.
- I could not verify which screen Brian looked at. The overview says 36. The Spelling and English to Hebrew tabs say 4 and 3 above bars that are almost entirely "unseen". Either reading fits his complaint.

**The most recent fix moved the headline by 2 words.** The headline history (`get_study_digest`): 14 on Oct 27, 20 on Nov 5, 35 on Nov 6, 38 on Nov 7 and Nov 8 under rule version 3, and 36 today under rule version 4, with 2 words "waiting for a second day". The INIT-036 initiative states: "How often it happens in real data is unknown, and no application data was read." The same-day loophole it closed accounts for about 2 words. The main reasons the counts are low (condition 2, the volume of asks, and condition 5, decay with lateness) have not been measured by any initiative.

##### 3.2 The four-skill state machine (focus question 2)

- **Volume.** There are 234 scheduler cards for about 110 words with evidence, roughly 2.1 per word (the 200 due-soonest cards cover 92 words). Each skill needs its own three checked successes on two days, so calling one word fully known takes roughly 12 checked asks across at least 2 days, before decay.
- **Weakest skill wins.** A word's overall state is its weakest skill with evidence (`Overall` in `state.go`). 88 of his 110 words are "learning" overall and 2 are known. Touching a new skill once (one listening pick in a lesson) can pull a word down.
- **The strict numbers drive the lesson pipeline, not just the page.** `internal/brief/build.go` sets the lesson tier from the strict overall known plus solid count (2 today), so he stays "beginner" (texts of at most 20 tokens, 3 to 4 new words) until 100 words are known in every skill they have evidence in. `PaceFor` forces "consolidate, no new words" when more than 40 words are learning overall (88 today), or when fewer than 75% of the last 50 reviews were rated Good or Easy (48% today, from `get_context.recentAccuracy`). A right answer rated Hard counts as a failure in that share (`reviewAccuracy` in `internal/brief/progression.go`). Both gates are tripped. With an untouched consolidation unit already queued, the brief refuses with "Review first" (`ErrReviewFirst`). Because overall "learning" grows with every word taught across four skills, the 40-word gate will stay tripped for weeks. This is a direct data path to his complaints "every lesson is the same" and "more reading interesting stories".
- **Contradictions on one screen.** Spelling is "strong" with 4 known. Listening says "No audio yet, practiced in lessons only" while 56 listening cards count as due.

**Verdict for one learner.** The seven states are reasonable as internal scheduler signals. They are the wrong interface for him, because he cannot act on "learning", and they are the wrong input for the pipeline, because a weakest-of-four-skills count grows with vocabulary and so cannot serve as a fixed threshold. An honest "where am I" is described in section 8.2.

##### 3.3 Conversation: what the app can measure on its own (focus question 3)

**Practice mix in the last 14 days** (standing): reading 339 answers (60%), English to Hebrew 104 (18%), listening 88 (16%, lessons only, multiple choice), spelling 33 (6%). The goal skill, understanding and answering, is the least practiced and the weakest measured:

- English to Hebrew: 73% right over 104 answers in 14 days, 59% delayed retention over 27 delayed reviews in 30 days, 3 words known.
- Listening: 96.6% right over 88 answers, but these are lesson multiple-choice picks. Review cannot ask listening at all because 0 lexemes have a clip (standing verdict "not_askable"; `filterReviewAskable` in `internal/session/mix.go` drops due listening cards without a clip).

**What each answer row stores** (`migrations/006_evidence.sql`, `015_attempt_source_self.sql`): skill, activity, source, unit, prompt, response, correct, dont_know, hint_used, latency_ms, created_at. It does not store playback speed (the 0.75× "Slower" toggle lives only in browser localStorage, INIT-036 SPEC-007), the number of replays, the number of answer options (so the guessing rate is unknown), whether a typed answer was a near miss (visible only as a Hard rating in `review_log`), or any speech result (ADR-0031 D4 made the say step count for nothing). No current number can say "understands at normal speed" or "can say it".

**What the app can measure without a human:**

1. **Delayed production from English.** Typed English to Hebrew answers on the first ask of a study day, at least one day after the previous ask, with exact answers and near misses counted separately. The activity exists today (`type_it` on recall cards). It is the closest in-app proxy for "can answer".
2. **Production latency.** The median `latency_ms` of correct typed production for words he has proved. It is already logged. Conversation needs fast retrieval, and a falling median is real progress even when counts stall.
3. **Listening at normal speed.** Right on first play at 1.0×, no replay, on sentences rather than single words, at least a day after the last exposure. This needs playback rate and replay count logged per answer.
4. **A fixed, held-out conversation probe.** About 20 short tutor-style lines at normal speed with a comprehension check, plus 10 English prompts to answer in Hebrew. The items are never used in practice, and the probe is taken monthly and scored by code. It is the only measure that is not contaminated by practicing the same items, and the only one that is comparable across months.
5. **Speaking, later.** Speech recognition against the target sentence, logged as its own skill, and kept out of "known" until its error rate on his voice has been checked.

**Proxies and vanity metrics today:**

- **"Words you know (any skill)".** 32 of the 36 come from reading recognition, the skill least related to conversation.
- **Weekly report "You gained 28 words and lost no words"** (week of Nov 2). In that same week English to Hebrew retention fell from 77% to 59% and English to Hebrew known stayed at 3. Gains in any skill are summed, and a retention drop never shows as a lost word.
- **Placement estimates.** Listening was estimated at 0 words (Oct 21), 20 (Oct 29), and 522 (Nov 6), and the Listening tab still shows about 503 "estimated known" against 10 from evidence. A measure that moves 25-fold in 8 days is not a baseline.
- **Lesson scores.** Mean 88% over 21 lessons in 7 days, against 72% Good or Easy over 244 reviews and 48% over the last 50. Lessons are easier than reviews, so a high lesson score does not predict retention.
- **Listening accuracy of 96.6%.** Multiple choice, at an unrecorded speed.
- **`practiceResults`** (the share right on the first later-day review, by practice kind: 69% to 87%, counts 31 to 69; `internal/evidence/later_day.go`). The later review can be in any skill, and practice kinds are not randomly assigned to words, so the table cannot say which practice works.
- **Minutes in app.** This measures activity, not learning.
- **Per-skill retention against the 85% to 90% band.** It is computed on 5 to 97 delayed reviews (see 3.4).

##### 3.4 Backlog, scheduler settings, and new-word rate (focus question 4)

**Anatomy of the 161 due cards** (all 234 cards read through `get_learner_words`; every due card is on the first page):

| Slice of the 161 due cards | Cards |
| --- | --- |
| Listening (review cannot ask these without a clip) | 56 |
| Reading | 37 |
| English to Hebrew | 54 |
| Spelling | 14 |
| Scheduled 10 minutes or less after their last review (FSRS same-day learning step) | 122 |
| Reviewed only once or twice ever | 133 |
| Never reviewed | 9 |
| Review-state cards whose multi-day interval came due | 30 |

Lateness: 3 cards are under a day overdue, 76 are 1 to 3 days, 36 are 3 to 7 days, 44 are 7 to 14 days, and 2 are over two weeks.

What this means:

- **35% of the backlog cannot be cleared by review.** 56 of 161 due cards are listening cards that review is built to skip, yet the brief and the coach note gate new lessons on the full 161 ("Review first: 161 cards are due"). The reviewable backlog is 105 cards.
- **The scheduler runs on library defaults.** `internal/fsrs/review.go` uses go-fsrs v3 `DefaultParam()` with fuzz off: desired retention 0.90, default weights that were not fitted to him, and same-day learning steps on. A new card answered Good is due 10 minutes later. The app asks each word once per sitting (ADR-0024 D3), so that step is never taken and the card shows as overdue from the next morning. If those steps were whole days, about 41 of the 122 would not be due yet. The steps inflate the count by about a quarter, and irregular study days explain most of the rest.
- **Retention figures mostly measure lateness and small samples.** 30-day delayed-review retention is 81% over 153 reviews: listening 96% (24 reviews), reading 84% (97), English to Hebrew 59% (27), spelling 80% (5). The 59% has a 95% range of roughly 40% to 78%, and the spelling 80% roughly 38% to 96%. FSRS itself predicts 75% mean recall for the 96 reviewable due cards it has scheduled (reading 0.73, English to Hebrew 0.73, spelling 0.84). Getting 75% to 80% right on late cards is what the model expects. It is not evidence that the settings or the learner are failing, and the weekly report's "5 points under the 85% target" mostly measures how late the reviews were.
- **The backlog is one day of work.** In the last 7 days he answered 244 scheduler reviews and finished 21 lessons in about 45 minutes of measured app time, which is roughly 10 to 12 seconds per answer (an estimate, because the split between lesson and review time is not available). The 105 reviewable cards are about 20 minutes, or about six 20-card sessions (`SessionCap = 20`, `internal/session/mix.go`). At his stated 20 to 30 minutes a day, this is not a crisis.
- **The new-word rate is zero by the app's own rule.** At 25 minutes a day and about 11 seconds per answer he has about 130 answers a day. If half go to review and each new card costs roughly 6 to 8 reviews in its first month (a planning assumption typical of FSRS at 0.90, to be checked against his data), then two cards per word (listening and English to Hebrew) supports about 4 to 6 new words a day, and four cards per word supports about half that. Because his tutor sets the syllabus, the rate should follow tutor input, capped by this budget.
- **The tutor feed is empty.** `get_study_digest` shows no taught words in the last 14 days and 0 corrections in 30 days. The app's stated job is to help him retain what the tutor teaches, but it holds no record of what the tutor taught. That is a data silo at the center of the goal.

##### 3.5 Baseline (focus question 5)

Today's numbers worth freezing as the baseline (2026-11-09):

- English to Hebrew: 59% delayed retention (27 reviews, 30 days), 73% right over 104 answers in 14 days, 3 words known.
- Spelling: 21 of 22 spelled words never wrong, 4 known.
- Backlog: 161 due, 105 reviewable, most 1 to 7 days overdue.
- Study: 35, 57, and 64 minutes in the last three calendar weeks, 45 minutes in the last 7 days, and at least 13 study days between Oct 20 and Nov 8 (a floor taken from card last-review dates).
- Pipeline: 0 new words (consolidate), tier "beginner".
- No baseline exists for listening at normal speed or for speaking. The first run of the probe will be that baseline.

#### 4. Main considerations

1. **The headline must answer "where am I" in terms of his goal and his own taught words.** One "proved" number is fine, but it must not decay with the calendar, and it must not depend on skills he is not trying to train.
2. **Display numbers and decision numbers must be separated, and both tested against live data before they ship.** The 40-word learning gate, the 75% Good-or-Easy gate, and the 100-word tier were written when a word had one state. Under four skills and a weakest-skill overall, they lock the pipeline.
3. **Measurement cannot be better than the items.** When several answer buttons are grammatical and only one is marked right, the logged "wrong" answer feeds struggling, accuracy, pacing, and the newest-three rule. Disputed items need to be capturable and excludable.
4. **One learner means small samples.** Weekly per-skill retention on 5 to 27 reviews is noise. Every number shown should carry its count, and decisions should use rolling windows of at least 50 delayed answers.
5. **No human will ever test him.** The only defensible outcome measure is a fixed, held-out, automatically scored probe, backed by in-app delayed production and normal-speed listening.

#### 5. Opportunities

1. **The raw data already supports an honest ladder.** The attempts table has every checked answer with a timestamp. One query can count, per word and skill, the distinct study days with a right answer and whether the latest answer is right. The first version needs no new data collection.
2. **The backlog is cheap to make honest.** Keeping the 56 unaskable listening cards out of "due" (or giving lexemes clips from the existing text-to-speech pipeline), and treating same-day learning steps as due on the next study day, makes "due" mean "due today". What remains is one day of review.
3. **Latency is already logged** and is a strong, cheap proxy for conversational retrieval.
4. **Within-learner experiments are valid** if words, not days, are randomly assigned to conditions. Detecting a 15-point difference in later-day production (70% against 85%) needs about 120 words per arm. That is feasible in 8 to 10 weeks at 4 to 6 new words a day. A 10-point difference is not detectable at this scale.
5. **Unblocking the pipeline takes two threshold changes.** Fixing the pace gate and the tier input would likely change how lessons feel within days.

#### 6. Risks and concerns

1. **High: Goodhart's law.** An AI lesson writer told to raise "proved words" will drill the same easy words. Mitigation: the held-out probe, and new words taken from the tutor's material.
2. **High: contaminated evidence from bad items.** Answers to ambiguous items are logged as wrong and feed every metric. The size of this effect has not been measured. ASMT-004 found 23 units with structural defects, including 128 one-word clozes and distractors drawn from untaught words. Mitigation: a dispute control on each item, disputed answers excluded from evidence, and an audit of logged responses (the response text is stored, so the audit is possible).
3. **High: changing "known" yet again.** This would be the fifth rule version. Numbers that move with each release erode the trust he says is the binding constraint. Mitigation: define one ladder once, show it beside the old number for two weeks with a one-line reason, then freeze the definition.
4. **Medium: small samples.** Weekly per-skill figures are noise. Show counts, use rolling 50-answer windows, and give no band verdict under 30 answers.
5. **Medium: listening stays a proxy.** Until playback speed and replays are logged, any listening number is only a proxy for normal-speed comprehension.
6. **Medium: irregular study.** If gaps stay at 2 to 4 days, decay-based numbers flicker and the backlog regrows. Track study days per week as a guardrail.
7. **Low: FSRS default weights may be miscalibrated for him.** Check calibration (predicted recall against actual outcomes by probability bin) before tuning anything, and fit weights after about 1,000 delayed reviews.
8. **Low: speech recognition accuracy on his accent is unknown.** Keep it out of "known" until it is validated.

#### 7. Scenarios

**Optimistic.** The honest ladder ships with a frozen definition. On day one it shows his real progress: at least 13 spelling words and 11 English to Hebrew words with repeated right answers and none wrong, instead of 4 and 3. The 105 reviewable cards clear in one 20-minute sitting. The pace gate reopens, and new words come from tutor sessions at 4 to 6 a day. After 8 weeks he has about 250 taught words, 60% to 70% of the words taught 14 or more days earlier are proved in English to Hebrew, the production latency median falls, and the monthly probe shows a measurable gain. Trust rises because the numbers move only when he does.

**Pessimistic.** The change lands as yet another rule version, and the numbers jump again. Item quality stays poor, so evidence stays contaminated and "slipping" fills with words he actually knows. Study stays at 3 to 4 days a week, so the backlog regrows and the pace gate shuts again. Practice stays at single words, so English to Hebrew retention stays between 59% and 73%, and the honest ladder shows that plainly, which is accurate but discouraging. The probe barely moves because the app practices words, not sentences, and the conversation goal is not met within 6 to 12 months unless the tutor's material drives the content.

#### 8. Recommended approach

##### 8.1 Metric definitions (one page, frozen once agreed)

- **Taught word.** A word introduced in a lesson he finished, or recorded from the tutor. This is the denominator for everything.
- **Recalled later (per skill).** At least one checked right answer on a study day after the day the word was first taught.
- **Proved (per skill).** Checked right answers on at least 3 different study days spanning at least 7 days, and the latest checked answer right. It does not decay with time. A wrong answer removes it, and the word then shows as slipping until it is proved again. The 3 days over a week is a product choice grounded in the spacing research cited by ADR-0031, chosen for legibility. It is not a measured optimum.
- **Slipping.** The latest checked answer was wrong, or 2 of the last 5 were wrong.
- **Not yet asked.** Taught, but never asked in that skill. It is shown as its own count, so "not tested" is never confused with "failed".
- **Likely to remember today.** FSRS recall probability of at least 0.85. It is used for scheduling and may appear as a small secondary line, never as the headline.
- **Checked.** Not peeked, not self-graded, and not an item he disputed. Near misses count as right with a note in the display, and as Hard for scheduling.

##### 8.2 What Progress should show

1. **Top: two rows for the goal skills, measured against taught words.** "Understand by ear" and "Say it from English" (English to Hebrew), each showing proved, recalled later, not yet asked, and slipping, out of N taught. Every number is a count he can click through to the words.
2. **Second: reading and spelling as smaller rows** with the same four counts.
3. **Third: "Today: X reviews, about Y minutes".** Reviewable cards only.
4. **Fourth: two trends.** The monthly probe score, and the median time to produce a proved word from English.
5. **Removed from the headline:** the any-skill count, placement estimates, band verdicts under 30 answers, and the weekly "gained N words" sentence.
6. **Every percentage carries its count**, for example "59% (16 of 27)".

##### 8.3 Backlog and scheduler plan

1. **Make "due" honest (code, one-off).** Keep listening cards without a clip out of "due" and out of the brief's gate, or give lexemes clips. Treat FSRS learning-step cards as due at the start of the next study day, or turn off same-day steps, which matches the once-per-sitting rule.
2. **Clear the backlog (study, day 1 or 2).** Work through the 105 reviewable cards in five or six 20-card sessions. The order matters little, because FSRS accounts for lateness.
3. **Replace the pace gates.** Allow new words when the reviewable due count at the start of the day is under about one day's review budget (about 70 cards, roughly 12 minutes), and when the later-day production right rate over the last 50 delayed answers is at least 80%, counting Hard as right. Set the tier from words proved in reading, or proved in any skill, not proved in all four.
4. **Fewer cards per word.** Consider scheduling only listening and English to Hebrew cards, and crediting reading and spelling from those answers (a typed production answer already credits spelling, ADR-0030). That roughly halves the load per word. This needs the Technical Architect's view.
5. **Scheduler settings.** Keep desired retention at 0.90 for now, and do not tune weights until calibration has been checked. After about 1,000 delayed reviews, fit FSRS weights on his `review_log`. If 25 minutes is still exceeded, lower desired retention to 0.85 before cutting new words. That should reduce review load meaningfully, but the size is an assumption to check with the FSRS simulator.
6. **Daily shape for 20 to 30 minutes.** About 10 to 12 minutes of review (all reviewable due cards), 10 to 15 minutes of a lesson or story, and 2 minutes of a production speed drill. Guardrail: no new words on a day the due queue was not cleared.

##### 8.4 Data collection to add

- **Per answer:** playback rate, replay count, number of options, a near-miss flag, and a disputed flag tied to the item.
- **Lesson runs:** started against finished, and the step where he stopped. This is the trust signal.
- **Tutor taught words:** the feed already exists, so make entry trivial, for example pasting the lesson's word list.
- **Probe results:** a small table, one row per item per run.

##### 8.5 Experiment strategy

Run within-learner experiments with words randomly assigned to conditions, for example production-first practice against recognition-first practice for new words. Pre-register one outcome: right on the first English to Hebrew ask 7 days after teaching. Expect about 120 words per arm to detect a 15-point difference. Do not compare days or weeks against each other as experiments, because motivation and lesson quality vary by day.

##### 8.6 The baseline and the two numbers that judge the redesign

1. **Primary 1: the share of words taught 14 or more days ago that are proved in "Say it from English".** Compute the day-0 value from the attempts table under the new definition. Today's closest proxies are 3 of 51 English to Hebrew words known, and at least 11 with two or more right answers and none wrong. Set the 8-week target after two weeks of data. My prior is 60% to 70%.
2. **Primary 2: the monthly held-out conversation probe score** (normal-speed comprehension plus production). The baseline is taken in the week the redesign starts.
3. **Guardrails, which do not judge success but catch damage:** study days per week (target 5 or more), the lesson abandon rate, reviewable due at the start of the day (under one day's budget), and the dispute rate per lesson.

#### 9. Questions for other specialists

1. **For the Technical Architect.** Can "checked right on N distinct study days, latest right" be computed cheaply from `attempts` for all words on every page load, or does it need a nightly table? Is it safe to keep clipless listening cards out of "due", and to turn off FSRS same-day steps, given the once-per-sitting rule?
2. **For the QA Advocate.** How can a lesson item that has several acceptable answers be detected before publishing, and flagged by the learner after answering, so that those answers are excluded from evidence? Can the logged response text be audited to estimate how many recorded "wrong" answers were actually acceptable?
3. **For the Product Strategist.** If the app's job is to help Brian retain what the tutor teaches, which words count as "taught": the tutor's list, the AI syllabus, or both? Will Brian enter tutor words, given that the taught feed has been empty for 14 days?
4. **For the Devil's Advocate.** Is any in-app number a credible stand-in for conversation ability, or should we accept that only a held-out, normal-speed probe counts and keep everything else off the headline?
5. **For the Product Strategist and the Technical Architect.** Would you accept dropping scheduled reading and spelling cards, so they are credited only from listening and production answers, to halve the review load per word?

#### Evidence log and assumptions

- **Code read:** `internal/learner/state.go`, `standing.go`, `model.go`; `internal/db/learner.go`; `internal/web/progress.go`; `internal/web/views/progress.templ` and `progress.go`; `internal/web/views/today.templ`; `internal/fsrs/review.go`; `internal/brief/build.go` and `progression.go`; `internal/session/mix.go`; `internal/evidence/spell.go`, `grade.go`, `later_day.go`; `migrations/006_evidence.sql` and `015_attempt_source_self.sql`; ADR-0028, ADR-0030, ADR-0031; the INIT-036 initiative and SPEC-007.
- **Live data:** read through MCP for learner 1 at about 14:30 UTC on 2026-11-09 (all read-only tools).
- **Assumptions, not verified:**
  - About 10 to 12 seconds per answer, from 244 reviews and 21 lessons in about 45 minutes.
  - About 6 to 8 reviews per new card in its first month.
  - The workload effect of lowering desired retention to 0.85.
  - Which Progress screen Brian looked at.
  - How much ambiguous items have contaminated the evidence.
- **Derived figures:**
  - The 95% ranges are normal approximations.
  - The "about 41 fewer due cards without same-day steps" figure assumes intervals of the larger of 1 day and the card's rounded stability.
  - Current recall probabilities were computed with the go-fsrs default curve from each card's stability and last review.

### Summary

**Position:** The app collects good evidence, but it reports and acts on a "known" label that needs more asks than the app has made, decays with lateness, and is drawn against the whole lexicon. Brian has never misspelled 21 of the 22 words he has spelled, yet he is told he knows 4, and the same strict counts keep his lessons at "beginner, no new words". The measure and the display are both wrong for one learner whose goal is conversation.

**Recommendation:**
- Freeze one legible ladder per skill: taught, recalled later, proved (right on 3 study days spanning a week, latest right, no calendar decay), slipping, and not yet asked. Measure it against words actually taught, and headline the two conversation skills: "Understand by ear" and "Say it from English".
- Keep FSRS recall probability for scheduling only.
- Make "due" honest: no clipless listening cards, and same-day learning steps due the next study day. Clear the 105 reviewable cards in one day.
- Replace the pace gates (more than 40 overall-learning words, and under 75% Good or Easy) with a review-load gate and a later-day production rate that counts Hard as right.
- Log playback speed, replays, option count, near misses, and disputes.
- Judge the redesign on two numbers: the share of words taught 14 or more days ago that are proved in English to Hebrew, and a monthly held-out normal-speed conversation probe. Use study days per week and the lesson abandon rate as guardrails.

**Top Risk:** Contaminated and shifting measurement. Ambiguous items log correct-in-spirit answers as wrong, and a fifth redefinition of "known" could again move his numbers for reasons unrelated to learning. Either one keeps destroying the trust he says is the binding constraint (High).

**Confidence:** High for the diagnosis of why Progress shows so few known words, which was verified in code and in live data. Medium for the proposed thresholds, time estimates, and new-word rate, which rest on a single learner, small samples, and stated assumptions.

---

## Phase 4: Dialogue Contributions

### Questions Posed
- To the Technical Architect: can "right on N study days, latest right" be computed cheaply, and is it safe to drop the scheduler's same-day re-asks?
- To the QA Advocate: can ambiguous items be caught, and their answers left out of the evidence?
- To the Product Strategist: which words count as "taught"?
- To the Devil's Advocate: is any in-app number a credible stand-in for conversation?
- To the Product Strategist and Technical Architect: should scheduled reading and spelling cards be dropped?

### Questions Answered
- **From the Product Strategist and the Devil's Advocate:** Which single number, and can it stay frozen?
  **Response:**
  - The headline is "class words you can say from English", out of taught words. "Proved" means right on 3 study days spanning a week, with the latest answer right, and no calendar decay.
  - The second row is "understand by ear".
  - Freeze both for 90 days, with one announced change.
- **From the Technical Architect:** Can the history be rebuilt under one definition?
  **Response:** Yes. The rule-version rebuild from `review_log` exists (ADR-0031 D5). Do it once.
- **From the QA Advocate (follow-up):** What about voiding contaminated evidence?
  **Response:** Agreed. Recompute Progress after voiding, and show the change once, with the old and new numbers side by side.

### Tensions Involved In
- Fewer graded formats or more evidence: accepted that production answers carry stronger evidence and that reading can be credited from them.
- Where Brian really is: loosen to the "proved" ladder, and also run the cold check of about 40 words to reseed the model.

### Synergies Identified
- Never count an answer the app can't prove wrong: shared with QA and the Devil's Advocate.
- Progress must be legible and stable: shared with Product, the Devil's Advocate and the Architect.
- Fix listening: shared with the Devil's Advocate and the Architect.

---

## Phase 5: Consensus Position

### Option Scores
| Dimension | A: Patch in place | B: Staged | C: Re-center now | D: Freeze and go simpler |
|-----------|---|---|---|---|
| Learning value | 2 | 4 | 3 | 3 |
| Trust and content correctness | 1 | 5 | 3 | 2 |
| Effort and time to value | 2 | 4 | 2 | 3 |
| Treadmill risk (higher = lower risk) | 1 | 4 | 2 | 4 |
| Preserves progress and history | 2 | 4 | 3 | 3 |
| Reversibility | 3 | 4 | 2 | 5 |
| **Total** | **11** | **25** | **15** | **20** |

### Score Notes
- **A** scores lowest. It keeps writing false wrong answers into the log, and nothing undoes them. It keeps redefining "known", which has had four rule versions so far. No fix has a measured before and after. The raw rows survive, but the record stops meaning the same thing from one release to the next.
- **B** loses a point on learning value because Stage 1 adds no new practice. It loses a point on treadmill risk because its seven Stage 1 items still run through the same process. It scores highest on trust because it goes straight at the two biggest sources of bad data: ambiguous items graded as wrong, and a "known" label and "due" count that don't mean what they say.
- **C** ends at the same design as B. But it changes the content, the metrics and the pipeline all at once, with no baseline. No result could be traced to one cause. It also risks archiving units before the void, and the void needs `unit_id`.
- **D** is the most reversible option and stops the treadmill for 14 days. But it leaves Progress wrong and the contaminated struggle list in place for those 14 days. Practice in Anki would also be invisible to the app's learner model and scheduler, which leaves a hole in his record.

### Final Position
**Preferred Option:** B: Stabilize, then re-center on the tutor (staged).

**Rationale:** B is the only option that fixes the measurement before it changes the content. That gives a clean baseline to judge Stage 2 against, and each movement in his numbers has one cause. Stage 1 removes the two biggest sources of bad data with small changes that can be undone. The audit gate then turns the riskiest assumption, that usable tutor material can be loaded at no cost to Brian, into a pass or fail test before anything large is built. C cannot be evaluated because it changes everything at once, and D's 14 days is too short to give a valid learning result for one learner.

**Conditions:**
1. **Baseline first.** Before any Stage 1 change ships, compute the day-0 values under the new "proved" definition from the attempts table. Also take the first held-out probe: about 20 normal-speed listening items and 10 English prompts, never practised, and scored by code. Store both results.
2. **Void as a flag, never a delete.** Before any unit is archived, mark the voided answers with a reason. These are the answers from units 12 to 34, from word-choice blanks with no English cue, and from disputed items. Count them by word and skill, so the size of the contamination is known and the void can be undone.
3. **One announced change, shown in parts.** Show three numbers side by side once: the old number, the number after the void, and the number under the new ladder. Then freeze the ladder for 90 days. Results from the cold check of about 40 words go in as "recalled", with their own source, and never count as "proved".
4. **Make "due" and the pace gates honest in Stage 1.**
   - Listening cards without a clip stay out of "due" until they have one.
   - The scheduler's same-day learning steps fall due on the next study day.
   - Three old gates go: more than 40 learning words, under 75% Good or Easy, and the tier that counts only words known in every skill. In their place come a review-load gate and a later-day production rate that counts Hard as right.
   - Without these changes, Stage 2 inherits a brief that always says "review first".
5. **Start logging in Stage 1 what Stage 2 will be judged on.** For every answer, log playback speed, replay count, number of options, a near-miss flag and a disputed flag. For every lesson run, log whether it was started and whether it was finished.
6. **Write down the pass and fail lines before Stage 1 ships, and don't move them later.**
   - **The audit gate:** at least 150 usable tutor-written lines and 5 of the last 10 classes. An agent that did not build the import does the counting.
   - **The dispute stop line:** more than 1 dispute per 50 graded items in a week.
   - **The two success numbers:** the share of words taught 14 or more days ago that are proved in "Say it from English", and the monthly probe score.
   - **The guardrails:** study days per week, the lesson abandon rate, and the reviewable due count at the start of the day.
   - **What "the probe rose" means:** set it in advance. A 30-item probe can only show a large gain, roughly 7 more items right, so read smaller moves as noise.
7. **Stage 2 starts on counts, not only on the calendar.** It starts after a week or after these counts, whichever comes later: about 150 graded answers with the dispute rate under the stop line, and at least 50 delayed review answers under the new rules.

**Dissent:** I agree with the consensus on B, with two small dissents.
- **The gate between stages.** The one-week wait should be a count (condition 7). At 2 to 4 study days a week, one calendar week may not hold enough answers to show whether the dispute rate and the honest due count hold up.
- **The 14-day freeze.** I don't accept the Devil's Advocate's freeze as a period for experiments. With one learner, comparing two ways of practising needs about 120 words in each group over 8 to 10 weeks. In 14 days the only things that can be tested are whether tutor material loads and whether Brian studies 20 to 30 minutes a day. B's audit gate and its guardrails already test both.
