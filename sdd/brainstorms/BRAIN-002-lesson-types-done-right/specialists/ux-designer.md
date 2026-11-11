# UX Designer — Brainstorm Analysis
**Session:** Lesson types done right
**Date:** 2026-11-10
**Domain:** Product & Design
**Primary Lens:** User experience and accessibility. Can Brian move through a lesson without thinking about the interface, on a desktop keyboard and on a phone held in two thumbs? Does any screen give him a reason to stop trusting it?

---

## Phase 3: Independent Analysis

### Chain of Thought

#### Primary Lens

I judge each lesson type on four things:

1. **Cognitive load.** One job per screen. Nothing on screen that is not the word, its meaning, or the answer.
2. **One way to answer.** The same control works the same way in every lesson type and in review.
3. **Both devices.** Every step works from the keyboard alone, and with two thumbs on a phone. No drag-only control.
4. **Trust at the moment of a miss.** The miss screen is where Brian decides whether the app is fair. It must be the best-designed screen in the app.

#### Initial Assessment

The five-step word lesson in `docs/design/lesson-architecture.md` puts the steps in a sound order, and the research backs it. As written, though, it has three problems. Several steps depend on audio that is blocked. The sentence step has no fair way to grade. And there is no single answer-and-feedback pattern that ties the steps together. The usability risk is not any one screen. It is the seams: between steps, between review and lesson in one session, and right after a miss.

#### Main Considerations

**1. Two answer controls are enough for every type.**
Every graded step is either "pick one" or "type it". If both behave the same way everywhere, a new lesson type never needs to be learned.

- **Pick one.** On desktop, keys 1 to 4 choose an answer, and Enter goes on. On a phone the buttons are full width, stacked in the lower half of the screen, and at least 44 CSS pixels tall. RSCH-006 rules 8 and 17 say primary controls should be 44 px and sit in the thumb's lower reach (`sdd/research/RSCH-006-learner-interface-design/RSCH-006-learner-interface-design-report.md` §1.6).
- **Type it.** One Hebrew input box set to `lang="he"` and `dir="rtl"`, with autocorrect, autocapitalize, spellcheck, and word suggestions turned off. On a phone, a Hebrew keyboard's suggestion bar can finish the word for him. That turns recall into recognition. The prompt and the input sit above the keyboard. Enter submits.
- **Shortcuts must not depend on the keyboard layout.** Brian types with a Hebrew layout. A letter shortcut such as "F" gives כ when the Hebrew layout is on. Digits and Enter are the same in both layouts. So shortcuts use digits and Enter. Any other shortcut must read the physical key (`KeyboardEvent.code`), not the character, and must only work when focus is not in the input box. The player already marks Enter as `aria-keyshortcuts` on primary buttons (`internal/web/views/activity.templ` lines 65, 288, 377). That is the right start.

**2. The miss screen is one shared component, and it carries the "I was right" button.**
RSCH-006 §1.3 says to say more than "wrong" and to show the correct form on the same turn (Moderate). It also says one retry and then the answer, never a loop (Low). The feedback strip should be the same in lessons and in review:

- A verdict word.
- The right answer, in pointed Hebrew at 24 px or more.
- For typed answers, his answer with the wrong and missing letters marked. `internal/activity/align.go` already produces substitution, insertion, and deletion steps.
- One quiet button: **"I was right / this question is wrong."**

The parts already exist. ADR-0033 D1 and D2 define a void with the reasons `also_right` and `report`, and "a tap only voids, it never credits" (`docs/decisions/ADR-0033-stage-1-trust-and-measurement.md` lines 32–44). `POST /_hx/also-right` is built (`internal/web/also_right.go`). Today already shows "That lesson is hidden for you" after a whole-lesson report (`internal/web/views/today.templ` line 38).

Two things are missing against Brian's answer in `session-context.md`. First, the void does not flag the item to be fixed: `also_right.go` refuses a right answer and only writes the void. Second, a report hides the whole lesson. Brian wants per-item control. I propose:

- One button, one tap, no reason form. It voids the answer and flags that one item.
- It shows on a miss and on a right answer, because "this is dumb" can apply to a question he got right.
- A short "Marked. Undo" note follows. Void batches can be undone under D1.
- A flagged item does not come back to him until it is fixed. The rest of the lesson goes on.

**3. Audio is blocked. Plan the no-audio screens as the real design, not a stopgap.**
`sdd/complaints.md` line 11 stops all `generate_audio` batches. So the no-audio versions are what Brian will actually use for the next two weeks:

- A step that needs audio is left out, not shown greyed out.
- There is no Listen button when there is no clip, and no "coming soon" note.
- A dead button is hidden functionality. It also reads as "this app is unfinished", which is a trust cost.

This rules out, until there is a voice:

- the grid's Listen button;
- the meaning check by ear;
- the story's "repeat the line";
- dictation;
- "which line did you hear";
- the listening lesson;
- the by-ear half of the spelling lesson.

**4. Brian's "it all feels the same" complaint was about screens and content. More screen types is the wrong fix.**
Brian said both the screens and the content repeated (`session-context.md`, Phase 2 table). Consistent controls are a usability gain. The fix is to keep the controls and the stages fixed, and to vary three other things:

- what fills the stages (the tutor class, the topic, the story's style);
- which lesson type Today picks;
- the form of the one sentence step.

A 4-word lesson today would be about 12 near-identical screens (ASMT-006 counts 8 single-word steps out of 12: `sdd/assessments/ASMT-006-hebrew-lesson-types/ASMT-006-hebrew-lesson-types-assessment-report.md`, `unit.drill_share` row). It should feel like five named stages instead.

**5. Typed grading may mark a right spelling wrong.**
`Normalize` in `internal/activity/align.go` strips vowel points and compares letters. I found no rule that accepts full spelling, meaning the extra vav or yod that unpointed Hebrew usually adds (for example שולחן for שֻׁלְחָן). I also found no separate unpointed spelling column in `internal/db`.

Brian learns from tutor documents, and fewer than half of their lines are pointed (158 of 350, per `session-context.md`). So he will often type the tutor's unpointed spelling. If the key is the stripped pointed form, the app marks that answer wrong. That is the second trust-breaker he named. I have not run it, so treat this as a strong suspicion for QA to confirm, not a proven defect.

#### Opportunities

1. **Most of what is needed is already built.** The single-word meaning check, typed answers graded letter by letter, tap-for-meaning in the story, the "also right" void, and one-session Today all exist (ASMT-006 Part A, "What already works"; `internal/web/views/today.templ` lines 3–5). The word lesson is mostly a re-sequence plus two new screens: the grid, and typed input in lesson `produce`. That fits the two-week target.
2. **The flag button can be the main quality loop.** Brian has no human reviewer (`session-context.md`, Phase 2 table). A one-tap flag from the only learner, sent back to the code and the MCP author, is the cheapest reviewer the app can have. It needs a visible queue, and a rule that a flagged item stays out until it is fixed.
3. **Lessons can be tied to the class they came from.** A label such as "Word lesson · from your 28 Oct class" tells Brian why these words, and supports his reason for using the app at all: "relearn our lessons" (`session-context.md`, The learner). That label is ASMT-006 gap A12, so it is planned already. It also adds content variety for free, because each class has its own topic.
4. **The forms of a verb fit a phone well as a short list.** The conjugation lesson can reuse the grid's card layout, with no new interaction.

#### Risks and Concerns

| # | Risk | Severity | Why |
| --- | --- | --- | --- |
| R1 | Typed answers in full spelling are marked wrong (Consideration 5). | **High** | Write-it, the spelling lesson, and review's `type_it` all depend on it. This is exactly "a right answer marked wrong". |
| R2 | The prompt for a verb form is ambiguous. "You wrote" in English hides gender and number, so a right Hebrew form is graded wrong. | **High** | The conjugation lesson is built on these asks. Each prompt must carry person, gender, and number, for example "you (one woman) wrote". |
| R3 | The second ask after a miss comes right after the answer was shown. That is copying, not recall, and it feels pointless. | **Medium** | RSCH-005 §1.3 (Moderate): copying while the word is visible is poor. The fix is to move the second ask to the end of the check. |
| R4 | The flag button is pressed by accident, or used to dodge hard words. | **Medium** | A void never credits (ADR-0033 D2), so a false flag cannot inflate "proved". It can still hide a fair item. A 5-second Undo and a weekly look at the flag list cover it. |
| R5 | A tap in the story shows a word that has no gloss, or a gloss that does not fit the sentence. | **Medium** | That is the "words he was never taught" failure inside the one AI-written part. Every token must resolve to a lexicon entry before the story is shown. |
| R6 | Review fills the 20 to 30 minutes and the lesson never starts, so every day feels like a test. | **Medium** | D10 makes Today one session (`ADR-0033` line 113). The split between review and lesson needs a cap, or the new lessons are never seen. |
| R7 | Phone layout: the grid of 4 to 8 cards, each with an example and closed forms, is too dense at 24 px Hebrew and needs scrolling in two directions. | **Low** | Solved by reflow to one column. WCAG 1.4.10 requires no two-way scrolling at 320 px (RSCH-006 §1.5). |

#### Recommended Approach

**Shared across every type**

- **A stage bar at the top.** It shows named stages, not a screen count, for example "See · Check · Read · Write · Use". The current stage is highlighted. On a phone it is a single line of short labels.
- **Two answer controls only:** pick one and type it, as in Consideration 1.
- **One feedback strip** on every graded item, in lessons and in review:
  - the verdict;
  - the right answer in pointed Hebrew;
  - the letter diff for typed answers;
  - the "I was right / this question is wrong" button, which voids the answer, flags the item, and offers Undo.
  - Enter or the bottom button goes on. A right answer does not move on by itself, so the flag button stays reachable.
- **No score at the end of a lesson.** The end screen says what happens next. For example: "4 new words from your 28 Oct class. Review will ask them on Thursday." The lesson teaches. Review decides (`docs/design/lesson-architecture.md` line 7). Words count as "proved" only on Progress. No streaks or points (RSCH-006 rule 14).

**Today: one session, review first, then the lesson**

1. **The start screen shows a two-part map.** For example: "Review · 12 words, about 8 minutes, then Word lesson · 4 new words from 28 Oct, about 15 minutes." One button starts it.
2. **Review comes first.**
   - It is the later-day test the science counts (RSCH-003 via `lesson-architecture.md` line 68).
   - Doing it first keeps today's new words from mixing with the test.
   - Brian is fresh for the part that counts.
3. **A pause screen sits between the two parts.**
   - It reads, for example: "Review done. Next: 4 new words."
   - **Continue** is the main button. **Stop for today** is a quiet second option. A clean way out supports his own choice (RSCH-006 §1.4).
   - If he leaves in the middle, Today resumes at the same step.
4. **A cap stops review from crowding out the lesson.** When review would fill the whole session, Today offers review and then an easy lesson (a story, no new words). It does not offer a word lesson. The exact cap is a Data Analyst question.
5. **One progress bar for the whole session, in two colored parts.** It does not count items.

**1. Word lesson (build first)**

| Stage | Desktop | Phone | Without audio |
| --- | --- | --- | --- |
| **See** (the grid) | A true grid: 2 × 2 for 4 words, 2 × 4 for 8. Each card shows the pointed word at 32 px or more, the meaning, the part of speech, the binyan as a small label, and one example sentence from the tutor's lines. Verb forms sit in a closed disclosure. One button: "Start the check". | The same cards in one column, scrolled down. The word and its meaning are always visible. The example and the forms open on a tap. The "Start the check" button is fixed at the bottom. No swipe carousel, because cards off screen are hidden functionality, and WCAG 2.5.7 forbids drag-only controls. | No Listen button at all. |
| **Check** (meaning) | Pointed Hebrew, four English buttons, keys 1 to 4. A miss shows the right meaning and the card's example line. The missed word is asked again **after the other words**, with the buttons shuffled. Only the first answer is recorded (ASMT-006 A3). | The same, with the buttons stacked low. | Read-and-pick is the only version. The by-ear version waits for a voice. |
| **Read** (the story) | The full story, short enough to fit one screen. The new words get a light underline as the signal (RSCH-006 §1.1). Click or tab to a word to see its meaning in a popover next to it. Arrow keys move between words inside a line, so Tab does not stop on every word. "Show English" opens per line. Nothing is graded. | Tap a word and a bottom sheet shows its meaning, and the line scrolls above the sheet. If the story is long, it pages by paragraph with a "Next" button at the bottom. | No "hear the line" and no "repeat it". Read and tap only. |
| **Write** | The English meaning is large. One Hebrew input. Vowel points are never required. Full and short spellings are both accepted (see R1). The letter diff is shown on a miss. One retry, then the answer. | The input sits above the keyboard. Suggestions are off. | Fully works. |
| **Use** (one sentence step) | **One** step per lesson, from a tutor sentence, never from an AI one (ADR-0033 D3). The form rotates between lessons: (a) type the missing word, with the English of the whole line shown, graded only when the gap is exactly this lesson's word; (b) build the line from tappable word chips, ungraded, then compare: "Your tutor said it this way". Never "wrong" on word order, because Hebrew allows more than one order. | Chips are tap to place and tap to remove. No dragging. | Both forms work. Dictation and "which line did you hear" wait for a voice. |

**2. Easy lesson (build second, because it is the review-heavy day)**

- It is the word lesson without See.
- A longer story of words he is still learning comes first, with tap-for-meaning.
- Then a few meaning checks and write steps on words from that story. They use the same controls and the same feedback strip.
- It is the natural partner for a heavy review day (Today, point 4).

**3. Conjugation lesson (third, after the prompt rule in R2)**

- **Forms.**
  - On desktop, a table with person, form, and English. On a phone, a list.
  - The binyan is named once at the top.
  - Each English line carries person, gender, and number, for example "she wrote" or "you (one man) wrote".
- **Practice.**
  - "Type the form" from that English line, graded against the stored forms in `internal/db/verbforms.go`.
  - Typing is better than picking here. The forms differ by a letter or two, so picking turns into spotting the difference.
- **Sentences.**
  - A short passage with those forms highlighted, tap for meaning.
  - Nothing is graded.

**4. Listening lesson: do not show it until there is a voice Brian accepts.**

- The brief must not pick it.
- There is no placeholder on Today.

**5. Spelling lesson: defer.**

- Without audio, it is "English shown, type the word". That is already the word lesson's Write step and review's `type_it` (`internal/activity/plan.go` line 51).
- A separate type would add a screen that looks the same as one he already uses. That is the "feels the same" complaint again.
- Its gate, the letters check, is not recorded yet (ASMT-006 A11).
- Bring it back as a dictation lesson once there is a voice.

**How variety comes without confusion**

- **Fixed:**
  - the stages of each type, always in the same order;
  - the two answer controls;
  - the feedback strip;
  - the stage bar.
- **Varies:**
  - which tutor class the words come from, named on the lesson label;
  - the story's topic and style;
  - the form of the one sentence step;
  - which type Today picks.
- **Rules that keep content from repeating:**
  - no two lessons in a row on the same class topic;
  - the grid's example, the check, and the story never reuse the same sentence;
  - a word lesson is never followed by another word lesson on the same day.

**What to cut**

- **Until there is a voice:**
  - every Listen button;
  - "repeat the line";
  - the by-ear meaning check;
  - dictation;
  - "which line did you hear";
  - the listening lesson.
- **Now:**
  - the spelling lesson as its own type;
  - more than one sentence step per lesson;
  - any graded order or multi-word pick step;
  - an immediate repeat right after a miss;
  - any percent score at the end of a lesson;
  - any disabled or "coming soon" control.
- **Retire** `unit.same_shape`, `unit.drill_share`, and `brief.story_length`, as ASMT-006 proposes. The shape is now set by the lesson type, so those rules fight the design.

### Questions for Other Specialists

1. **QA Advocate:** Does typed grading accept full spelling (the extra vav or yod) for a word stored only in pointed form? `internal/activity/align.go` strips vowel points but adds nothing else. Can you build a fixed regression lesson of 20 tutor-spelled words that must pass before write-it ships?
2. **QA Advocate:** For the "use it in a sentence" gap, how do we prove the gap has exactly one right answer when the tutor's line uses a prefixed form, such as ו־, ב־, or ה־ joined to the word?
3. **Technical Architect:** Where does an item flag go, and how does a flagged item stay out of Brian's lessons until it is fixed? Today `also_right` voids and refuses right answers, and `report` hides the whole unit. Can one per-item flag reuse the `report` void reason without a new table?
4. **Data Analyst:** What cap on review time keeps room for a lesson in a 20 to 30 minute session? Should the cap come from Brian's own answer times, which D10 already uses?
5. **Data Analyst:** Can the flag rate per lesson type and per step be the usability measure, with no new tracking? For example, more than 1 flag in 20 graded items means that step is broken.
6. **Product Strategist:** Do you agree the first version is word lesson plus easy lesson, with conjugation third and spelling and listening deferred? Do you agree that "review first, then lesson" is the default order?
7. **Devil's Advocate:** If the controls and stages stay fixed for consistency, will Brian still feel that lessons are "the same"? What would show that varying content and the sentence form is enough, before we build more screen types?

### Summary

**Position:** Keep one fixed set of stages per lesson type and one shared answer-and-feedback pattern for every graded item. Get variety from tutor-class content and from which type Today picks, not from new screen types. Design the no-audio versions as the real product for now.

**Recommendation:** Ship the word lesson (See, Check, Read, Write, one Use step) and the easy lesson inside one Today session: review first, a pause screen, then the lesson. Add a one-tap "I was right / this question is wrong" button that voids the answer and flags the item. Defer listening and spelling until there is an accepted voice.

**Top Risk:** Typed answers in normal full spelling may be marked wrong, because grading compares against the pointed form with the points stripped. That is the exact trust break Brian named, and it would hit write-it, conjugation practice, and review.

**Confidence:** Medium. The layout and feedback rules rest on Moderate and High research (RSCH-006, WCAG 2.2), and most parts already exist in code. Three things are not checked: the full-spelling grading gap, the review cap, and whether fixed stages answer the "feels the same" complaint.

---

## Phase 4: Dialogue Contributions

### Questions Posed

1. **QA Advocate:** Does typed grading accept full spelling (the extra vav or yod) for a word stored only in pointed form? Can you build a fixed regression lesson of 20 tutor-spelled words that must pass before write-it ships?
2. **QA Advocate:** For the "use it in a sentence" gap, how do we prove the gap has exactly one right answer when the tutor's line uses a prefixed form (ו־, ב־, ה־ joined to the word)?
3. **Technical Architect:** Where does an item flag go, and how does a flagged item stay out of Brian's lessons until it is fixed? Can one per-item flag reuse the `report` void reason without a new table?
4. **Data Analyst:** What cap on review time keeps room for a lesson in a 20 to 30 minute session? Should the cap come from Brian's own answer times, which D10 already uses?
5. **Data Analyst:** Can the flag rate per lesson type and per step be the usability measure, with no new tracking? For example, more than 1 flag in 20 graded items means that step is broken.
6. **Product Strategist:** Do you agree the first version is word lesson plus easy lesson, with conjugation third and spelling and listening deferred? Do you agree that "review first, then lesson" is the default order?
7. **Devil's Advocate:** If the controls and stages stay fixed for consistency, will Brian still feel that lessons are "the same"? What would show that varying content and the sentence form is enough, before we build more screen types?

### Questions Answered

- **Technical Architect** asked whether the flag button should appear after every graded answer or only after a miss. I answered: one small control after every graded answer, never a dialog. On a miss it reads "I was right"; on a right answer it reads "This question is wrong." It has Undo.

### Tensions Involved In

- **Tension 1: Study first, or build first?** My stance: backfill first is required by everyone, and I want a visible new screen as soon as the gate allows. I do not want Brian's study week run on the current typed grader (see Dissent).
- **Tension 2: How many lesson types in the first release?** My stance: the word lesson first. The easy lesson is a thin variant of it (the word lesson without See), and the review cap needs it. I want it in slice 1, or the heavy-review rule dropped.
- **Tension 3: Does the AI story earn its place?** My stance: tutor lines as the main passage, AI story optional. Every tap in the story must resolve to a lexicon entry (R5), and AI text is the most likely source of unglossed words.
- **Tension 4: How much measurement?** My stance: the middle ground. The flag rate is the quality signal, it needs no new logging beyond the void reason, and I oppose a hold-out group.
- **Tension 5: Who certifies the lexicon?** My stance: grade only the top-tier words in the first release. A wrong key shows up first at the miss screen, so the flag must also work on right answers, and an uncertified word is shown but never quizzed.
- **Tension 6: The review cap inside Today.** My stance: review first, a soft cap, and a pause screen between the two parts. A heavy backlog gets a lesson with no new words, not a word lesson.

### Synergies Identified

- **Typed spelling accept-sets before typed write ships:** with Technical Architect and QA Advocate, who each found the extra-ו/י problem on their own (R1).
- **One flag control that voids, flags, and suspends one item:** with Technical Architect and QA Advocate (one request, suspend until fixed), and with Data Analyst, who uses the flag rate as the quality signal.
- **Tutor lines as the reading passage:** with Product Strategist and Technical Architect. It uses the G3 ruling, fixes the "same content" complaint through class topics, and removes the dependency on the AI story.
- **Code-built lessons skip the old validator:** with Technical Architect, so no-audio lessons can appear on Today.
- **Ungraded word order:** with QA Advocate. The "use" step grades only the gap word, never the order.
- **No-audio versions as the real design:** with Product Strategist, who also wants listening deferred.
- **Copyable "this week's words and sentences" page for Brian's own AI:** with Product Strategist and Devil's Advocate, as cheap speaking prep.

---

## Phase 5: Consensus Position

### Option Scores

| Dimension | A | B | C | D |
| --- | --- | --- | --- | --- |
| Learning effect (research cited) | 3 · five types, two blocked by no audio | 4 · typed recall and spaced review (RSCH-003, RSCH-005) | 3 · one type; spacing only in week one | 4 · as B, after a baseline study week |
| Trust (no right answer marked wrong, no untaught words) | 2 · more typed keys and an AI story with untaught words | 3 · accept-sets and tier-gated keys, but more graded surfaces | 3 · study week runs on the unfixed typed grader | 3 · tier gating, but the study week has the same grader gap |
| Time to new study (about 2 weeks) | 1 · five types cannot land in two weeks | 2 · two builds plus backfill, and the gate is in the way | 3 · new words in review at once; new screen after the study week | 3 · new words in review now; new screen only after the 30 Nov gate |
| Build cost and treadmill risk (fewest specs and rules) | 1 · five specs and five rule sets | 2 · two builds and a queued conjugation spec | 4 · one build and one trigger list | 3 · one build, plus tier and tripwire rules and a conditional easy lesson |
| Fixes "feels the same" | 2 · more screen types, which is the wrong fix by this lens | 4 · fixed stages, varied content, and a second shape on heavy days | 3 · one new shape; content varies | 3 · one new shape; variety depends on content |
| Measurable (can we tell if it works) | 2 · five arms, too thin per type for one learner | 3 · flag rate per type, but two arms are noisy at one learner | 3 · one type, before-and-after only | 4 · baseline study week, flag rate, and retire rule set in advance |
| **Total (out of 30)** | **11** | **18** | **19** | **20** |

### Final Position

**Preferred Option:** D, with three changes to be decided before slice 1 (see Dissent).

**Rationale:** D gets new words into Brian's review within days, which answers the "same again" complaint on content, and it makes the word lesson the only new screen to build first. That keeps one fixed stage set and one answer-and-feedback pattern, and the study week gives a baseline to measure the lesson against.

**Dissent:** Not aligned with D as written, on three points from my lens:
1. **No fallback for heavy-review days.** D makes the easy lesson conditional, but the review-cap rule in Tension 6 depends on it. Either the easy lesson ships in slice 1 (it is the word lesson without See, so it is small), or heavy days show review only and offer no new-words lesson. Decide one before slice 1.
2. **The study week runs on a grader that marks tutor spelling wrong (R1).** D says no code before the gate, so the accept-set fix and the flag would land after the study week. Treat the grader fix as a bug fix rather than Stage 2 work, or keep backfilled words out of typed review until it lands.
3. **Timing.** The first new screen cannot land before the 30 Nov gate, which is about three weeks from today, not two. If the two-week target matters, the gate needs an explicit decision, not a workaround.

---
## Information Injection R1: Lessons not tied to a recent class
**Injected at:** After Phase 5, before Phase 6

### Re-Evaluation
**Impact Assessment:** High
**Position Change:** Changed
**Analysis:** The injection removes the premise under my Today design: that a lesson follows the newest class, and that a heavy-review day is the exception. The stages, the two answer controls, and the feedback strip stay as they are, because they are what makes lessons feel usable. What changes is what fills them, how the lesson is labelled, and how Today reserves room for a lesson before review takes the session. A word lesson can shrink to 2 new words, so heavy days need no new lesson type, and I withdraw my dissent that the easy lesson must ship in slice 1.
**Prior Recommendation:** Review first with a soft cap; on a heavy-review day, Today shows review only; the easy lesson is the heavy-day fallback and ships in slice 1.
**Revised Recommendation:** Reserve a lesson slot before review is counted, so review is capped and any overflow rolls to tomorrow. On heavy days, size the word lesson down to 2 to 4 new words, and keep the easy lesson in slice 2 as a fallback for when fewer than 2 untaught usable words remain.
**New Risks or Opportunities Surfaced:**
- **Label by source, not topic.** Class words read "Word lesson · from class of 14 Oct". Lexicon-only words read "Word lesson · from your word list · [unit]". Topic can be a second word, never the only one. A date tells Brian why these words are on his screen, which trust needs, and it matches his aim to relearn his lessons.
- **Heavy-day rule.** The start map reads "Review · 18 due, about 12 min (6 roll to tomorrow), then Word lesson · 2 new words from class of 14 Oct, about 8 min". The lesson's time is reserved first, so "always something new" holds by design. The lesson is smaller, not a different type.
- **Stages drop out, they are not replaced.** If no tutor line uses a new word, the Read or Use step is left out, never filled with an AI sentence (ADR-0033 D3). The stage bar shows only the stages in that lesson, so there is no dead step.
- **Variety without a recency rule.** Pick the class with the most untaught usable words that is not the class of the previous lesson. The label changes each day and no lesson depends on the newest class.
- **Risk: older classes are less checked.** The audit covered the last 10 classes, and only 10 of about 85 documents are audited. A wrong key in an older class can surface on any day, so draw only from documents that pass the loader and the code check, and keep the one-tap flag on right answers.
- **Risk: "new" must come from the taught record.** Words already taught in the old app must not reappear as new. The untaught queue needs the backfill's taught flags, or he sees words he has met before, and Progress disagrees with what he remembers.
- **Risk: fewer known words in older passages.** Coverage checks may fail more often on older class lines. The check decides, and the Read step drops out instead of showing an unfair passage.
- **Risk: the pool can run dry.** After the classes are used up, heavy days have no new words. That is the trigger for the easy lesson, not a reason to add it now.

**Confidence Adjustment:** Decreased. The Today rule is now clearer, and that raises confidence in the design. But every lesson draws on a pool that is mostly unchecked, so confidence in the word source falls. Net, I am less sure the lessons will be trustworthy than I was in Phase 5.
