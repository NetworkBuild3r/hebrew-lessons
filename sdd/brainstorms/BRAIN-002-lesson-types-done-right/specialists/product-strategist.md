# Product Strategist — Brainstorm Analysis

**Session:** Lesson types done right
**Date:** 2026-11-10
**Domain:** Product & Design
**Primary Lens:** User value and time to value. Each lesson type, step, and spec is judged by one test: does it help Brian keep what his tutor teaches, in 20 to 30 minutes a day, so he can hold a simple spoken conversation in 6 to 12 months? And does it reach him soon enough that he keeps studying?

---

## Phase 3: Independent Analysis

### Chain of Thought

#### 1. Primary lens

There is one user, and he told us what he wants: "I want to know Hebrew better. However we achieve that." (`session-context.md`, Phase 2). The product is a retention tool for his tutor's classes, backed by research. Speaking happens with the tutor and with his own AI, outside the app (`session-context.md`, Clarification).

So the app has three jobs, in this order:

1. Get the words and sentences from each class into his long-term memory. This is checked review on later days, which is the strongest evidence we have (RSCH-003 §1.1, spacing High).
2. Teach the new class words well enough that review can start on them. That is the lesson's job.
3. Get him ready for speaking: words he can produce from English, verb forms he can use, and sentences he has typed. His AI and his tutor do the talking.

I score everything by user value per minute of his time and per day of build time. The process lesson from BRAIN-001 is part of the lens: 34 initiatives and 258 commits produced about three hours of study (`BRAIN-001 decision-document.md`, Key Evidence). A plan that ships 13 specs before he studies with anything new repeats that.

#### 2. Initial assessment

The five-type design is sound in its parts, but it is too big for the first release, and two of its five types do not earn their place now. A **word lesson** built from the newest class, with a graded sentence step keyed to the tutor's own lines, delivers most of the value. The **easy lesson** already exists in code and covers heavy-review days. **Conjugation** is the next type worth building. **Listening** is blocked by the voice, and **spelling** solves a problem Brian does not have. The way to make lessons stop feeling the same is to change where the content comes from (his classes, their topics, the tutor's sentences) and to rotate a few lesson types. Adding more lesson types or more shape rules will not fix it.

#### 3. Facts that changed since BRAIN-001, and why they matter

These are on disk now. They change the plan more than any design preference does.

| Fact | Source | What it means for lessons |
| --- | --- | --- |
| The Drive audit passed: 286 usable tutor lines, 10 of 10 classes loadable without Brian | `sdd/initiatives/INIT-038-today-you-can-trust/specs/completion-summaries/SPEC-012-audit.md` | G2 is met. The tutor sentence bank is real, so the sentence step can be graded against the tutor's lines. |
| 742 of 974 unique tokens in those classes missed the lexicon | same file | A word lesson needs a pointed lexicon entry for every word it teaches: the grid, the meaning check, and the write step all read the lexicon. Getting class words into the lexicon is on the critical path, ahead of any screen. Some misses are prefixed or inflected forms, so the true gap for target words is smaller but unknown. |
| 158 of 350 tutor lines carry vowel points | same file | Most tutor lines cannot be shown as-is in a pointed app. Tutor lines work best as answer keys compared without points (BRAIN-001 Stage 2 outline). They are weaker as display text. |
| Brian refused the computer-read voice: "not good enough" | `sdd/complaints.md`; `SPEC-012-completion-summary.md` | The Listen button on the grid, the meaning check by ear, dictation, "which line did you hear", and the whole listening lesson depend on a voice he accepts. None exists yet. The held-out probe (S3) is not made either. |
| The tutor never logs in, never reviews, never uses MCP | `session-context.md`, Clarification | P-11 ("the weekly note for the tutor") has no reader. BRAIN-001's "AI writes a weekly note for Aaron" is dead. Import must stay zero-touch, and a failing tutor line can only be dropped, never sent back. |
| Speaking practice is with his own AI | `session-context.md`, Clarification | The app does not need open "write a sentence" or speaking steps. It should feed his AI the right words and sentences. |
| Private reuse of the tutor's text (G3) is not yet recorded as decided | `ADR-0033` G3; `INIT-038` initiative line 220 | If the answer is no, the sentence step has no trusted key and stays an ungraded reveal (D3). The most valuable new step depends on this one decision. |

#### 4. Main considerations

**4.1 The value is in the content source, not in the number of lesson types.**

Every past trust break came from Hebrew the AI chose or wrote (BRAIN-001 Product Strategist §3.7). The old lessons felt the same because they shared one template, used tiny stories, and rotated the same checkpoint mistakes (BRAIN-001 Product Strategist §3.3). The new design fixes the source: words from class packs, keys from the lexicon and the tutor's lines, and AI only in the ungraded story (`session-context.md`, Decisions). A tutor's class is already grouped by situation, which RSCH-004 §1.1 prefers over same-category word lists (Low). Content from his classes gives him a theme per lesson for free.

**4.2 The sentence step is the biggest single gain, and it depends on the tutor bank.**

Under D3 every multi-word step today is an ungraded reveal (`ASMT-006`, A8; `ADR-0033` D3). With the tutor's lines as keys, "put the words in order" becomes a checked sentence-level answer. RSCH-011 ranks sentence build second of six layouts (3.50) and first among the tutor shapes. It is also the step closest to conversation, because it works on whole sentences, not single words. The `order` activity already exists. What changes is that its key comes from a trusted line.

**4.3 Producing from English is the skill S2 measures and the one speaking needs.**

S2 is the share of words proved in "Hebrew from English" (`ADR-0033`, Success measures). Productive recall builds productive knowledge (RSCH-003 §1.1, Moderate). Brian's typed practice holds up best on a later day: `type_it` 87% over 31 and `produce` 83% over 42, against `read_pick` 69% over 35 (BRAIN-001 Product Strategist §3.4, small samples). The word lesson's "write it" step serves S2 directly. Today lesson `produce` is three Hebrew buttons, not typing (`ASMT-006`, A6). Fixing that is high value and small, because review's typed input and `GradeTyped` already exist.

**4.4 Spelling is not his problem.**

He has misspelled 1 of the 22 words he has ever spelled, and spelling is rated "strong" at 97% over 33 answers (`BRAIN-001 decision-document.md`, Key Evidence). RSCH-005 §1.4 found no shown effect of keyboard practice on learning words (as cited in `lesson-architecture.md`). A spelling lesson would need a recorded letters check that does not exist (`ASMT-006`, A11; P-09). It is a solution looking for a problem. The word lesson's write step and review's typed asks already cover spelling from memory, which is the task RSCH-005 §1.3 supports (Moderate).

**4.5 Listening is the goal skill, but it cannot ship without a voice he trusts.**

His goal is to understand Hebrew at normal speed (`session-context.md`, The learner). A listening lesson built on a voice he called "not good enough" would break trust on the skill he cares about most. Three things soften the loss while the voice is open:

- Reading while listening is not reliably better than reading alone when the learner sets the pace (RSCH-003 §1.2, Moderate). A story without audio still teaches.
- His own AI conversation gives him Hebrew by ear at conversational speed, outside the app.
- Even with a voice, a listening lesson's graded asks would be thin. Only a one-word meaning pick from the passage has a provable key (`ASMT-006`, A10).

So listening waits on Brian naming a voice. It is not cut for good.

**4.6 Code effort competes with study time.**

Brian's answer was "After 2 weeks": he wants to be studying with new lessons about two weeks after Stage 2 opens (`session-context.md`, Phase 2). The provisional list has 13 specs, including a one-engine refactor, generated docs, new logging, and a tutor note (`spec-skeleton.md` §5). Each spec that does not change what he sees on Today pushes the first new lesson back.

**4.7 Capacity: four words a lesson cannot clear the backlog.**

This is an estimate, and the inputs are labelled.

- The audit's 10 classes span 2026-09-14 to 2026-11-03, about 7 weeks, so about 1.4 classes a week (`SPEC-012-audit.md`).
- The one tutor worksheet in the repo lists 10 vocabulary words (`Aharon's code/worksheets/shai-date-indirect-speech/worksheet.js`). *Assumption:* 10 target words per class is typical.
- So about 14 new class words arrive each week.
- A word lesson teaches 4 below the 100-word tier (`lesson-architecture.md`, How many new words). Keeping up with new classes takes about 3.5 word lessons a week.
- At 5 study days a week (the W1 target in `ADR-0033`), with one easy or conjugation lesson, there are about 4 word lessons a week. That leaves room for about 2 old-class words a week.
- The 10 old classes hold about 100 target words. Through word lessons alone, the backfill would take about a year.

So the backfill cannot all go through word lessons. Old-class words need a cheaper path, and the newest class must come first.

#### 5. Which lesson types matter, in order

| Rank | Type | Why | When |
| --- | --- | --- | --- |
| 1 | **Word lesson** | The only type that brings new class words in. It feeds S2 through the write step. With the tutor bank, its sentence step is the one new graded sentence practice. | First release, week 1 to 2 |
| 2 | **Easy lesson** | It already exists as the consolidate path (`ASMT-006`, What already works). It is the right lesson on heavy-review days: no new words, a fresh story on the latest class's topic. Nation's fluency pass uses familiar language only (RSCH-004 §1.4). Almost no build. | First release, as the fallback |
| 3 | **Conjugation lesson** | He cannot speak with one form per verb. The tutor's worksheet teaches verb forms and indirect speech (`worksheet.js`). Pattern instruction helps (RSCH-003 §1.3, Moderate). Keys are the stored `verb_forms`, which are trusted data. It needs a new forms screen and a source for each form's English line (`ASMT-006`, A9 and Decision 3). | Second release, weeks 3 to 4 |
| 4 | **Listening lesson** | The goal skill, but blocked on a voice. Its graded part is thin. | Only after Brian names a voice he accepts, and after the S3 probe exists to judge it |
| — | **Spelling lesson** | Cut. Folded into the write step and review's typed asks. | Not built |

Two lesson types in the first release and three by week 4 is enough. The variety comes from content and from which type runs that day (section 9), not from having five.

#### 6. What to cut

From the provisional list in `spec-skeleton.md` §5 and the design in `lesson-architecture.md`:

| Cut or defer | Reason |
| --- | --- |
| **Spelling lesson and P-09** (record the letters check) | Not his problem (4.4). It removes a spec and a gate that only exists to serve it. |
| **P-11, the weekly note for the tutor** | The tutor never reads anything from the app. Dead on arrival. |
| **P-10, new logging** (playback speed, replays, number of options) | No current measure needs it. It is BRAIN-001's "new logging" item again (`spec-skeleton.md` §5, Deferred). |
| **Generated authoring docs from the rule catalog** (part of P-08) | When the AI writes only the story, the docs shrink to one story request. The generator machinery is for a pipeline that is retiring. |
| **Span-graded comprehension questions** | New grading machinery for a product choice that is not the recall event (RSCH-004 §3 rule 13; RSCH-011, layout D at 2.75). The story stays ungraded. |
| **Open "write a sentence"** in the word lesson's step 5 | It cannot be graded without a judge, and no judge is built (`ADR-0033` D3). His AI handles open production. |
| **Verb forms closed on the grid card** (deferred) | Forms belong to the conjugation lesson. The grid is a study surface, and RSCH-011 scores a show-everything card lowest (1.50) and allows it only briefly before retrieval. Fewer fields keep it short. |
| **Listening lesson, the meaning check by ear, dictation, and "which line did you hear"** (deferred) | They all need a voice he accepts (4.5). They come back the day he names one. |
| **P-06 as a full refactor before first use** (question for the Architect) | The first release needs typed input in lessons, the second ask, and a graded order step. If those can reuse review's code without merging the two engines first, the merge can wait. |

What stays: the grid (short), the meaning check with one second ask (RSCH-003 §1.3 supports prompting a retry, Low for prompts over recasts), the story with tap-for-meaning (RSCH-003 §1.2, first-language glosses Moderate), typed "write it", and the graded order step from tutor lines.

#### 7. How lesson and review share Today

Brian delegated this and the review-heavy days. My call is **review first, with a cap, then the lesson.** The reasons:

- **Review is the test.** If the lesson comes first, its grid and story can show the answers to words that review is about to ask. That inflates review results and S2. Review first keeps the test clean.
- **Review carries the strongest evidence.** Spaced retrieval is High (RSCH-003 §1.1). If he stops early, the high-value part is done.
- **The cap protects the lesson.** If review can fill the whole session, he never reaches anything new, and that brings back the "same every day" feeling. So review gets a fixed slot and the lesson always starts.

The no-research gap: RSCH-004 §1.7 found no evidence on session length or on order inside a session. This is a product choice.

A 25-minute Today, with minutes as an *assumption* (BRAIN-001 assumed about 10 to 12 seconds per answer and did not verify it):

| Part | Minutes | What |
| --- | --- | --- |
| Review | up to 12 to 15 | Due cards, typing from English first (`ADR-0033` D10). Anything over the cap waits for tomorrow. |
| Lesson | about 10 to 12 | One lesson, labelled with its type and its class ("From your 26 Oct class"). |

The picker's rules for heavy days:

1. If the due cards fit the review slot, the lesson is a **word lesson**.
2. If they do not fit, the lesson is an **easy lesson**: no new words, so the queue stops growing, but there is still a fresh story on the latest class's topic. This uses the review-load gate BRAIN-001 already chose in place of the old pace gates (`BRAIN-001 decision-document.md`, Stage 1 item 5).
3. There is never a Today with no lesson. On the worst day the lesson is short and easy, but it is there.
4. A **conjugation lesson** runs when a taught verb has forms of a pattern the class has covered but the app has not taught. *Product limit:* at most about one in five lessons, so word lessons keep pace with new classes.

#### 8. Which class words first

Brian delegated this. My call is **the newest class first, then work backwards, and send old-class words he may already know through a cold check in review, not through lessons.**

Why the newest class first:

- **Usefulness for classes.** The next class builds on the last one. Recalling last week's words is what makes the next tutor session go better, and he will notice. That is the fastest visible value.
- **Spacing.** The class was the first meeting. The app's lesson a few days later is a second meeting on a later day, which is what the spacing evidence supports (RSCH-003 §1.1, High).
- **Capacity.** New classes alone fill most word lessons (4.7). If the backfill went first, he would always be studying weeks-old material.

Inside a class, in order:

1. Words the tutor corrected, because they are known weak spots.
2. Words that appear in two or more classes. This is a frequency signal from his own course. The audit already counts tokens that recur across classes (`SPEC-012-audit.md`, note under the table).
3. Words that appear in a tutor line, so the sentence step has a trusted key.
4. Words that can get a clean pointed lexicon entry. A word with no trustworthy entry waits.

For old classes: record the remaining target words with `record_taught_words`, and let their first review asks be the cold check. This is the same move ADR-0033 D7 already uses, and it adds no new code. Words he gets right stay in review. Words he misses go into the word-lesson queue. That shrinks a year of backfill to the words he has actually lost. With the voice still open, only typed answers count as the check.

#### 9. The "feels the same" fix

Brian said both the screens and the content caused it (`session-context.md`, Phase 2). Each needs its own fix.

**Content (most of the fix):**

- **Every lesson names its class and topic.** "From your 26 Oct class." The words and the story's situation come from that class. Ten classes give ten topics, and new classes keep adding more.
- **The story uses the class's situation, and its form rotates.** It might be a short narrative, a two-person dialogue, or a message. This is one field in the story request and adds no validator rule. `ASMT-006` Decision 2 asks whether dialogue is allowed as the passage. From the product side, yes: a two-voice passage is not a conversation with him.
- **The sentences are the tutor's, not the AI's.** He has met them in class, so they feel like his course, not like generated filler.
- **The rotated checkpoint mistakes go away** with the unit pipeline (BRAIN-001 Product Strategist §3.3).

**Screens and flow:**

- **Different types on different days:** word lessons on most days, easy lessons on heavy days, and a conjugation lesson about once a week. Each is labelled on Today.
- **The word lesson's screens are new to him:** the grid and typed writing do not exist today (`ASMT-006`, A1 and A6).
- **The sentence step rotates** between order, dictation, and "which line did you hear" once a voice exists. Until then it is order only, and that is the main variety gap of the first release.
- **Review and the lesson are clearly two parts** of Today, so the lesson never looks like more review.

**What not to do.** Do not add a rule that forces lessons to differ in shape. INIT-030's `unit.same_shape` check helped produce the 23 cloned units (`BRAIN-001 decision-document.md`, process evidence). A steady recipe is a usability feature: he knows what comes next. Sameness is only a problem when the content is also the same.

#### 10. Getting him studying with something new within about two weeks

Stage 1 must close by 2026-11-30 (`ADR-0033` E4). G2 is already met. G3 (private reuse) still needs Brian. G4 needs 150 graded answers and 50 delayed reviews under the new rules. So Stage 2 opens in early December at the earliest, and the first new lesson should be in use by about mid-December.

**The first release: five specs or fewer, each with a line saying what Brian will see.**

| # | Spec | What Brian sees |
| --- | --- | --- |
| 1 | **ADR** (P-01, short): the types in this order, the cut list in section 6, review-first Today, newest class first, and the cut rule in section 11. It supersedes ADR-0026's lesson shape. | Nothing on screen. It locks scope. |
| 2 | **Class packs for the newest 3 classes** (minimal P-02 and P-03): target words, the tutor's lines in their own field, and a pointed lexicon entry for each target word. Any word or line that fails a check is dropped, never repaired by hand. | His own class words exist in the app. |
| 3 | **The picker and Today** (minimal P-05): review first with a cap, then a word lesson or an easy lesson. | Today shows two parts and a lesson label with the class date. |
| 4 | **Word lesson screens** (subset of P-07): a short grid without forms, the meaning check with one second ask, the existing story with tap-for-meaning, typed "write it" reusing review's typed input, and the order step graded against a tutor line. | A new lesson that looks and works differently from before. |
| 5 | **Story slot** (minimal P-08). A lesson never waits for a story. If the story is missing or fails its checks, the lesson runs without it, and the sentence step from tutor lines becomes the reading. | Stories about his class topics, or a lesson that still works without one. |

**Second release, weeks 3 to 6, in this order:**

1. The conjugation lesson: the forms screen and form practice keyed to `verb_forms`.
2. The old-class cold check (section 8), using existing tools.
3. Dictation and "which line did you hear", and the Listen button on the grid, as soon as Brian accepts a voice.
4. Unit pipeline retirement (P-12), after two to three weeks of use, as BRAIN-001 already planned.
5. A decision on the listening lesson, using the cut rule and the S3 probe once the probe exists.

#### 11. How we will know, with no new machinery

| Measure | Source | What it tells us |
| --- | --- | --- |
| S1 study minutes, at least 120 a week, aiming for about 150 once lessons return | `ADR-0033` S1 | Whether sessions are worth his time. **Caveat:** S1 counts only time spent answering, so grid and story time adds no minutes (`ADR-0033`, Consequences and Pass and fail lines). The Data Analyst needs to rule on this before lessons ship, or the new lessons will look like they reduced study time. |
| S2 proved share in "Hebrew from English" | `ADR-0033` S2 | Whether lesson words stick. The write step feeds this skill. |
| W2 lesson abandon rate | `ADR-0033` W2 | Trust and interest, per lesson type. |
| "I was right" and "this is dumb" flags per lesson type | Brian's QA answer in `session-context.md` | Which steps misgrade or annoy him. |
| One weekly yes or no from Brian: "Did the app help in this week's class?" | Asked with the weekly complaint review (`ADR-0033` D13) | Usefulness for classes, from the only judge there is. |
| Time to value: first word lesson from the newest class used within 14 days of Stage 2 opening | Ship date and the first `lesson_runs` row | Whether we avoided the treadmill. |

**Proposed cut rule, agreed before building** (the Data Analyst may tighten the numbers): after about 15 lessons of a given type, or 3 weeks, cut or rework that type if any of these hold:

- its abandon rate is more than 20 points above the word lesson's;
- it collects more than one "this is dumb" flag per 3 lessons;
- Brian says it does not help.

With one learner, a per-type learning-outcome comparison needs about 120 words per arm over 8 to 10 weeks (`BRAIN-001 decision-document.md`, Data Analyst dissent). So the cut rule reads behaviour, not learning gains.

#### 12. Opportunities

1. **The tutor bank turns sentence practice into checked practice.** This is the first graded sentence-level step since D3. It is the most direct preparation for conversation the app can offer, and the `order` activity already exists.
2. **Speaking prep with his own AI for nearly zero build.** The digest and the `hebrew://tutor` resource were already written for an AI conversation partner (BRAIN-001 Product Strategist §5). If his AI reads this week's class words, the tutor sentences, and the current pattern from there, the app preps his speaking without hosting it. The Architect should confirm whether the existing resource already covers this before anyone builds anything.
3. **The easy lesson is free variety.** It already exists and uses the same story slot.
4. **Typing is his strength.** Typed writing plays to the format that holds best for him on a later day, and to the skill S2 measures.
5. **The cold check shrinks the backfill.** It reuses ADR-0033 D7's mechanism and needs no new code.

#### 13. Risks and concerns

| Severity | Risk | Mitigation |
| --- | --- | --- |
| **High** | **Lexicon gap.** 742 of 974 class tokens missed the lexicon. Every target word needs an AI-written pointed entry with a gloss, part of speech, and binyan. The grid presents those as fact, so a wrong gloss there is a direct trust break with no checker behind it. | Start with only the newest 3 classes' target words. Compare each entry with the tutor's own use of the word where a tutor line contains it. Drop words that fail a check. Use the "this is dumb" flag to catch the rest. Ask QA what code check can catch a wrong gloss. |
| **High** | **No accepted voice.** Listen buttons, the meaning check by ear, dictation, "which line", the listening lesson, and the S3 probe are all blocked. The first release is reading-only and less varied, and the goal skill gets no new practice in the app. | Brian names a voice source before Stage 2 opens, or he accepts a reading-only first release, stated plainly. Each audio step turns on as soon as clips exist. |
| **High** | **Scope creep, the treadmill again.** 13 provisional specs, plus refactors that change nothing he sees. | Five specs or fewer in the first release. Every spec names what Brian sees. The cut list in section 6 goes into the ADR. |
| **Medium** | **G3 says no reuse.** The order step has no trusted key and stays an ungraded reveal. The first release loses its main new graded step. | Ask Brian for the G3 ruling before Stage 2 opens. If the answer is no, the word lesson still has a graded meaning check and a graded write step. |
| **Medium** | **Backfill pace.** Four words a lesson cannot keep up with new classes and also clear old ones (4.7). | Newest class first. Old-class words go through the cold check. Allow up to 8 words past the beginner tier, as the design already permits. |
| **Medium** | **S1 undercounts lesson time.** Ungraded steps add no minutes, so S1 may show a drop exactly when lessons get better. A3 fires at under 120 minutes. | The Data Analyst rules on counting lesson time before release. |
| **Medium** | **The story slot stalls lessons.** It needs an outside AI to run. If no one runs it, lessons wait. | A lesson never waits for a story (section 10, spec 5). |
| **Low** | **Too few types and it still feels the same.** Two types in the first release, with order as the only sentence format. | Content variety by class and topic, story form rotation, easy days, and conjugation in weeks 3 to 4. The weekly question and W2 will show it. |

#### 14. Scenarios

**Optimistic.** Stage 2 opens on about 2026-12-02. By mid-December Today runs about 13 minutes of review and then a word lesson from his latest class: four of his words, a short story about the class's situation, typing each word, and putting two of the tutor's sentences in order. He notices at his next class that last week's words come out when he speaks. Minutes rise to about 150 a week. A voice is chosen in December, and dictation and "which line" switch on. Conjugation lands in week 4. S2 reaches 50% by 2027-01-04.

**Pessimistic.** The lexicon entries for class words are shaky. Two wrong glosses on the grid in the first week, and Brian stops trusting the new lessons the same way he stopped trusting the old ones. The voice stays open, so lessons are reading-only and the order step is the only sentence format. It feels the same within a week. Meanwhile three of the 13 specs are refactors, the first new lesson slips to January, and S1 shows a drop because lesson time is not counted. A3 fires and the app falls back to review only.

**Most likely.** The word lesson from the newest class lands within about two to three weeks and is clearly better than the old units, because the words are his and he types them. Lexicon quality is the main friction. A few flags in the first weeks, then cleaner entries. Variety stays a little thin until a voice is chosen. The deciding factors are holding to five specs and getting the voice decision early.

#### 15. Recommended approach

1. **Before Stage 2 opens, Brian makes two decisions:** G3 (may tutor lines be used as answer keys) and the voice (name a source he accepts, or accept a reading-only first release).
2. **First release, at most five specs, studied with by about two weeks after opening:** a short ADR, class packs for the newest 3 classes with clean lexicon entries, a review-first Today with a cap, the word lesson (short grid, meaning check with a second ask, story, typed write, order graded against tutor lines), and a story slot that never blocks a lesson. The easy lesson is the heavy-day fallback.
3. **Second release, weeks 3 to 6:** the conjugation lesson, the old-class cold check, audio steps once a voice exists, and unit pipeline retirement.
4. **Cut:** the spelling lesson and the letters check, the tutor note, new logging, generated docs, span-graded comprehension, and open "write a sentence". Defer the listening lesson until there is a voice and a probe.
5. **Judge it** by S1 (with lesson time counted), S2, W2, the flags per type, and Brian's weekly yes or no. Apply the cut rule after about 15 lessons of a type.

#### 16. Questions for other specialists

- **Technical Architect:** Can the first release reuse review's typed input and `GradeTyped` inside lessons, and grade `order` against a tutor line, without the full one-engine merge (P-06)? What is the smallest trustworthy path to pointed lexicon entries for the newest 3 classes' target words, given that 742 of 974 tokens missed? What runs the story slot each day, so no lesson waits for someone to start an AI session? And does the existing `hebrew://tutor` resource already give Brian's own AI this week's words and sentences?
- **Data Analyst:** S1 counts only answering time, so the grid and the story add no minutes. How should lesson time count, so better lessons do not trip A3? Is the per-type cut rule in section 11 readable with one learner in about 3 weeks? What are better thresholds?
- **QA Advocate:** What code check can catch a wrong gloss or a wrong binyan on an AI-written lexicon entry before the grid shows it as fact? Is grading `order` against one tutor line safe, given Hebrew's free word order? What fixed regression lessons does the first release need for the word and easy lessons?
- **UX Designer:** On a phone with two thumbs, how does Today show review and then the lesson as two clear parts, without looking like the old backlog? How short can the grid be and still work as a first meeting with the words? Where does the class label go?
- **Devil's Advocate:** Is a first release with two lesson types and one sentence format enough to end "it feels the same", or will Brian see the same five-step recipe every day and quit? Is routing old-class words through a cold check in review a fair shortcut, or does it bring back the "Progress says I know nothing" feeling?

#### Evidence ledger

| Claim | Source |
| --- | --- |
| Audit: 286 usable lines, 10 of 10 loadable, 742 of 974 tokens missed the lexicon, 158 of 350 lines pointed | `sdd/initiatives/INIT-038-today-you-can-trust/specs/completion-summaries/SPEC-012-audit.md` |
| Computer-read voice refused; probe not made | `sdd/complaints.md`; `SPEC-012-completion-summary.md` |
| G3 still open | `ADR-0033` Gate to Stage 2; `INIT-038` initiative, line 220 |
| S1 counts answering time only; ungraded reveals add no minutes | `ADR-0033`, Pass and fail lines and Consequences |
| Lesson `produce` is buttons; no grid; no second ask; no forms screen; letters check unrecorded; easy lesson exists | `sdd/assessments/ASMT-006-hebrew-lesson-types/ASMT-006-hebrew-lesson-types-assessment-report.md`, Part A |
| Sentence build ranks second (3.50); a show-everything card is a study surface only (1.50); passage plus typed answer 2.75 | `sdd/research/RSCH-011-practice-layouts-that-teach/RSCH-011-practice-layouts-that-teach-report.md` §1 |
| Spacing High; retrieval Moderate; recall direction Moderate; reading while listening not reliably better, Moderate; first-language glosses Moderate; grammar instruction Moderate | `sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md` §1.1 to §1.3 |
| Theme over same-category lists, Low; no tested words per lesson; fluency uses familiar language; no evidence on session order or length | `sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md` §1.1, §1.2, §1.4, §1.7 |
| Spelling 97% right; 1 of 22 words ever misspelled | `sdd/brainstorms/BRAIN-001-is-the-hebrew-app-built-right/decision-document.md`, Key Evidence |
| Typed practice holds best on a later day | BRAIN-001 `specialists/product-strategist.md` §3.4 |
| One tutor worksheet: 10 vocabulary words, verb forms, indirect speech | `Aharon's code/worksheets/shai-date-indirect-speech/worksheet.js` |
| The provisional Stage 2 list P-01 to P-13 | `sdd/brainstorms/BRAIN-001-is-the-hebrew-app-built-right/spec-skeleton.md` §5 |

### Summary

**Position:** Most of the value is in one well-built word lesson from Brian's newest class, with typed writing and a sentence step graded against the tutor's own lines. The easy lesson is the heavy-day fallback, and conjugation comes next. Listening waits for a voice he accepts, and spelling is cut. "Feels the same" is fixed mainly by content from his classes and by rotating a few types, not by having five types or more shape rules.

**Recommendation:** Ship a first release of five specs or fewer within about two weeks of Stage 2 opening: class packs for the newest 3 classes, a review-first Today with a cap, the word lesson (short grid, meaning check with a second ask, story, typed write, graded order), an easy-lesson fallback, and a story slot that never blocks a lesson. Add conjugation in weeks 3 to 4. Cut the spelling lesson, the letters check, the tutor note, new logging, and generated docs. Get Brian's decisions on G3 and the voice before Stage 2 opens.

**Top Risk:** The lexicon gap. 742 of 974 class tokens are missing from the lexicon, so every word a lesson teaches needs an AI-written pointed entry that the grid shows as fact. A few wrong glosses in week 1 would break trust in the new lessons the same way the old units did.

**Confidence:** Medium. It is High on the ranking and the cuts, which rest on his own data, the audit, and research graded in the repo. It is Medium on the two-week timeline and the capacity math, which depend on lexicon quality, the voice decision, G3, and an estimate of 10 target words per class.

---

## Phase 4: Dialogue Contributions

### Questions Posed

- **To Technical Architect:** Can the first release reuse review's typed input and `GradeTyped` inside lessons, without the full P-06 merge? (Answered: typed input yes; graded word order no.)
- **To Technical Architect:** What is the smallest trustworthy path to pointed lexicon entries for the newest 3 classes, given 742 of 974 tokens missed? (Partly answered: count first.)
- **To Technical Architect:** What runs the story slot each day, so no lesson waits on someone starting an AI session? (Not answered in dialogue.)
- **To Technical Architect:** Does the existing `hebrew://tutor` resource already give Brian's own AI this week's words and sentences? (Not answered in dialogue.)
- **To Data Analyst:** How should lesson time count so better lessons do not trip A3? (Answered: active ungraded time, shown separately.)
- **To Data Analyst:** Is the per-type cut rule readable with one learner, and are the thresholds right? (Not answered in dialogue.)
- **To QA Advocate:** What code check catches a wrong gloss or wrong binyan before the grid shows it as fact? (Answered: the T0, T1, T2 tiers.)
- **To QA Advocate:** Is grading `order` against one tutor line safe? (Answered: no; order stays an ungraded reveal in release one.)
- **To QA Advocate:** What fixed regression lessons does the first release need? (Partly answered: the three-test release bar.)
- **To UX Designer:** How does Today show review and then the lesson as two clear parts? How short can the grid be? Where does the class label go? (Not answered in dialogue.)
- **To Devil's Advocate:** Is two lesson types with one sentence format enough to end "feels the same," or will Brian see the same recipe and quit? (Raised again as Round 4 challenge 1; left open as a bet with a tripwire.)
- **To Devil's Advocate:** Is routing old-class words through a cold check fair, or does it bring back "Progress says I know nothing"? (Not answered in dialogue.)

### Questions Answered

- **To UX Designer:** The first version is the word lesson. The easy lesson is only the fallback for heavy-review days.
- **To Data Analyst:** Probably not. Holding back a quarter of his class words works against "whatever helps me learn." I prefer a natural comparison: class words met only in review against class words met in a lesson.
- **To QA Advocate:** "Taught" includes class-document words for reading, since the tutor already showed them. For buttons and answer keys, only lexicon words at T0 or T1 that the app has taught count.
- **To Devil's Advocate:** Yes. A one-screen "this week's words and the tutor's sentences" page, copyable as text, is cheap and fits his speaking goal. `hebrew://tutor` may already cover most of it.

### Tensions Involved In

- **Study first, or build first?** Backfill comes first whichever option is chosen, and the newest classes' words should be in review before any lesson screen exists. I accept data entry before the gate, but no spec or code before the gate.
- **How many lesson types in the first release?** I want the word lesson and the easy lesson from day one. The easy lesson reuses existing code and is the only way to keep a lesson on heavy-review days. This is my main dissent (see Phase 5).
- **Does the AI story earn its place?** It stays optional. The tutor's own lines as the reading passage are better, and a lesson never waits for a story.
- **How much measurement?** Keep S1, S2, W2, the flag rate, and Brian's weekly yes or no. Add no new logging (P-10 cut). Settle how S1 counts ungraded time before release. I accept the Data Analyst's active-time rule.
- **Who certifies the lexicon?** I accept the tiers. The first release grades only T0 keys, and T1 waits for a second source that may not exist.
- **The review cap inside Today.** Review first, with a cap, is my position. I object to "the lesson is offered only once review is under its cap," because a heavy-review day would then show no lesson at all, which breaks my rule that Today always has a lesson.

### Synergies Identified

- **Lexicon backfill is the critical path.** Shared with Technical Architect, QA Advocate, and Devil's Advocate. Every type and every key reads the lexicon.
- **Typed accept-sets before typed write ships.** Shared with UX Designer, Technical Architect, and QA Advocate. The extra-ו and extra-י problem makes the word lesson's write step unsafe without it. The "write it" step serves S2 directly.
- **One flag control that voids, flags, and suspends one item.** Shared with QA Advocate, Technical Architect, and Data Analyst. The flag rate becomes the quality signal in my cut rule.
- **Tutor lines as the reading passage, made possible by G3.** Shared with Technical Architect and QA Advocate. It makes the story optional and ties each lesson to its class.
- **Code-built lessons skip the old validator.** Shared with Technical Architect. Without this, `audio.missing` and the shape rules block the word lesson before it reaches Brian.
- **Speaking prep through an export page.** Shared with Devil's Advocate and Technical Architect. It feeds his own AI without the app hosting any speaking practice.

---

## Phase 5: Consensus Position

### Option Scores

| Dimension | A | B | C | D |
| --- | --- | --- | --- | --- |
| Learning effect (research-backed) | 3 | 4 | 3 | 4 |
| Trust (no wrong right-answer, no untaught words) | 2 | 3 | 3 | 4 |
| Time to new study (about 2 weeks) | 1 | 3 | 2 | 4 |
| Build cost and treadmill risk (fewest specs and rules) | 1 | 3 | 4 | 3 |
| Fixes "feels the same" | 3 | 3 | 2 | 2 |
| Measurable (can we tell if it works) | 2 | 3 | 4 | 4 |
| **Total** | **12** | **19** | **18** | **21** |

**Reasons**

- **Learning effect.** A (3): all five types carry the research, but listening has no voice and spelling is not his need, so the value arrives late. B (4): the word lesson uses spaced review, a second ask, and typed recall (RSCH-003 §1.1 to §1.3), and conjugation adds pattern instruction. C (3): the same word lesson, but its learning starts only after a study week. D (4): the same core as B, and backfilled words start spaced review before any lesson is built.
- **Trust.** A (2): five types with many graded keys and AI-written glosses at once; typed grading marks a correct spelling wrong today. B (3): accept-sets and tutor-line keys, but no T0 gate or flag control in the plan. C (3): a wrong gloss is caught during the study week, but review already grades against the lexicon. D (4): accept-sets before typed write ships, T0-only keys, a flag that voids and suspends an item, and order left ungraded.
- **Time to new study.** A (1): 13 provisional specs. B (3): backfill is not sequenced first, so two weeks slips. C (2): a study week with no new lesson, then a build, which puts the first lesson about 3 to 4 weeks out. D (4): backfill is data entry that can start now, and the word lesson is the only build, so mid-December is credible if the count stays near 100 words or fewer.
- **Build cost and treadmill risk.** A (1): five types, refactors, and 13 specs. B (3): the easy lesson is cheap, but conjugation and validator deletion are in release one. C (4): one build and only the trigger list as a rule. D (3): one build, plus the flag control, validator deletion, T0 gate, and retire rule before slice 1.
- **Feels the same.** A (3): most variety on paper, but two of five types cannot run without a voice, and the fix arrives late. B (3): two types on day one, conjugation in weeks 3 to 4, and content from his classes. C (2): one type for weeks, so the same daily recipe. D (2): one type on day one, with the easy lesson and conjugation conditional or later. The content fix carries it, with the "same again" tripwire as the response.
- **Measurable.** A (2): five types each need a measure, and one learner cannot compare them. B (3): two types with flag rate and W2 per type, but no retire rule yet. C (4): one type, clean S2 and W2, and a natural baseline in review-only words. D (4): as C, plus the flag rate and a retire rule agreed before slice 1. Still behaviour, not proof of learning, with one learner.

### Final Position

**Preferred Option:** D, with the easy lesson moved into slice 1.

**Rationale:** D scores highest (21, against 19 for B). It starts the newest classes' backfill now, builds only the word lesson that carries the research, and includes the trust controls the panel agreed on. The easy lesson is nearly free to build and is the only guarantee of a lesson on heavy-review days.

**Dissent:** Aligned with consensus on the backfill, the word lesson, the T0 gate, typed accept-sets, the flag control, the cut list, and the retire rule. I dissent on three points from D as written:
- **Easy lesson in slice 1, not conditional.** D offers a lesson only when review is under its cap, so a heavy-review day shows no lesson. That breaks my rule that Today always has one. The easy lesson reuses existing code and gives day-one variety.
- **Graded word order stays ungraded for now.** The sentence step was my biggest expected gain, and I lose it. I accept QA's point about free word order for release one, but I want a revisit after two weeks of flag data, to see whether a single tutor line can be graded safely.
- **Keep the release near five specs.** The flag control, validator deletion, T0 gate, and retire rule should go inside the word-lesson spec as rules, not as separate specs. Otherwise the treadmill comes back through the trust work.

---
## Information Injection R1: Lessons not tied to a recent class
**Injected at:** After Phase 5, before Phase 6
### Re-Evaluation
**Impact Assessment:** High
**Position Change:** Changed
**Analysis:** Brian's injection breaks two of my assumptions: that the newest class drives each lesson, and that a heavy-review day can show review only. New words still come only from his tutor's class packs, so the pool is every audited class (about 100 target words by my 10-words-per-class assumption, and more across the 85 documents), ordered by need, with the newest class as a tiebreak; verb forms on verbs he already knows are the second source, and I would not add dictionary words from outside the classes. On a heavy day, Today keeps the capped review and always adds an easy lesson, so it is never review-only; the easy lesson moves from a trigger into slice 1 and can use a passage from any class. Conjugation moves up too, because it needs no class and no voice, so it becomes the first item after the word lesson rather than a week 3 to 6 extra gated behind day 28.
**Prior Recommendation:** Newest 3 classes first; word lesson on most days; review only on heavy days; easy lesson only when triggered; conjugation third, after the day-28 check.
**Revised Recommendation:** Draw the word queue from all audited classes (tutor-corrected, then words in two or more classes, then words with a tutor line, newest class as tiebreak); always run one lesson after capped review, a word lesson when review fits the cap and the easy lesson when it does not; start conjugation as the first slice 2 item without a trigger; and make old-class cold-check misses feed the same queue.
**New Risks or Opportunities Surfaced:**
- **Source pool is larger than I assumed, and partly unknown.** The 75 documents beyond the 10 audited classes have not been checked for loadability. The Stage 2 gate needs only 5 of the last 10 classes, so the older pool needs a load check before it is counted on.
- **Lexicon gap grows.** My 742-token gap covered 10 classes. Old-class entries carry the same wrong-gloss risk, so the T0-only rule and the flag loop matter more, not less.
- **Backfill order.** The newest 3 classes (about 30 words by assumption) give about 6 to 8 word lessons, roughly 6 weeks of new material. That is enough to start Stage 2 without backfilling all 10 classes before the gate, so the pre-gate scope stays at the newest 3.
- **Heavy days add no new words.** The easy lesson gives a fresh passage but no vocabulary, so the new-word queue pauses exactly when review is heavy. This is the right trade, and it keeps review from growing.
- **Old-class lessons help retention more than the next class.** Brian's weekly "did this help this week's class" question will show whether he values the older material. The lesson label should say it is from an older class and that the words were met in class.
- **"Progress says I know nothing" risk.** Re-teaching words met in class weeks ago can feel like the app forgot his work. Mitigation: label the words "met in class, new to the app" and let the cold check clear words he already knows.
- **Opportunity:** rotating across older classes gives different topics and sentences each week, which helps the "feels the same" problem more than any new lesson type.
**Confidence Adjustment:** Decreased. The lesson design is more robust because it no longer depends on recency, but my two-week timeline and capacity math assumed the newest 3 classes and a 10-class, 100-word backlog. The wider pool, the unchecked 75 documents, and a larger lexicon gap make the timeline less certain, even though the direction holds.
