# QA Advocate — Brainstorm Analysis

**Session:** Is the Hebrew app built right? (BRAIN-001)
**Date:** 2026-11-09
**Domain:** Engineering
**Primary Lens:** Quality and reliability: whether every graded question can be shown to have exactly one right answer, whether a wrong grade can damage the learner model, and whether our gates check the thing the learner actually sees.

---

## Phase 3: Independent Analysis

### Chain of Thought

#### 1. Primary lens

I look at this app as a test system that grades a human. Each graded step is a test case, and the learner is the system under test. If the test case is wrong, for example because two buttons are right but only one is marked right, then the app records a false failure. That false failure goes into the learner model and then into the next lesson brief. My questions are: what exactly is shown to Brian, what checked it before he saw it, and what happens to his data when the check was wrong.

#### 2. Initial assessment

The app has a lot of tests and rules, but they check the unit document, while Brian sees answer buttons that the player picks later, at play time, by a seeded shuffle that deliberately prefers words of the same part of speech. "Is a second button also right?" is a question about meaning, and the project's own decision record admits the server cannot answer it. So the gates are structurally blind to Brian's main complaint. The fix is not rule 62. It is to stop generating graded questions whose correctness only a Hebrew speaker could confirm, to validate exactly what is shown, and to make sure an unprovable "wrong" never counts against him.

#### 3. What I checked

- Code: `internal/units` (rule catalog `rules.go`, `validate_cloze.go`, `validate_target.go`), `internal/web/lesson.go` (button selection, lines 1330 to 1700), `internal/choices/pick.go`, `internal/activity/distractors.go` and `check.go`, `internal/fsrs/typed.go`, `internal/db/learner_publish.go` (report handling), `internal/db/put_unit.go` (word lists), `.github/workflows/ci.yml`.
- Records: INIT-032 and its SPEC-005, INIT-037 and both SPEC-011 completion summaries, ASMT-004, ADR-0030, ADR-0032, the author skill, the authoring guide, and the tutor-course worksheets in "Aharon's code".
- Live, read-only MCP calls for learner 1: `get_unit` for units 1, 8, and 11; a `validate_unit` dry run of unit 11; `get_study_digest`; and `lookup` for עָסוּק, חוֹפְשִׁי, יֵשׁ, אֵין, and "free". No write tool was called.

#### 4. Main considerations

##### 4.1 Why the gates miss what Brian sees (focus question 1)

**The validator never sees the buttons.** A unit document has no answer buttons. The player builds them when Brian reaches the step. `wideChoices` in `internal/web/lesson.go` (lines 1380 to 1436) fills a cloze from four tiers, and the first tier is "same part of speech, outside the line, this unit first." `choices.Pick` in `internal/choices/pick.go` shuffles with a seed made from the run id and step. So the screen Brian sees is a function of the document, the lexicon, his taught-word set, and a random seed. `validate_unit` checks only the first of those four. A gate cannot check an artifact that does not exist yet when the gate runs.

**Same part of speech first is designed to produce plausible wrong answers, and plausible often means grammatical.** In Hebrew a present-tense verb after a pronoun is almost always grammatical, and so is any adjective after אֲנִי. INIT-032 found this ("In 'אֲנִי __' with לַעֲשׂוֹת, רוֹאֶה, אוֹמֵר, לְהָכִין, every verb makes a sentence") and chose the English line as the cue. Its own decisions section then says: "The server cannot tell whether a verb fits a sentence." The follow-up that would let an author list other right words, INIT-032 SPEC-005 (`accept` on a cloze), is still "planned", and its out-of-scope line says: "Telling, by grammar, whether a word fits the line. The lexicon does not hold enough to do that safely."

**Real examples of "several answers are right" on published units:**

- **Unit 1 (shared, the very first lesson), lesson 2.** The cloze is "אַתָּה __ פֹּה?" ("Do you ___ here?"), target גָּר ("live"), with no `translation`, so no English cue is shown. The INIT-037 SPEC-011 verification (`specs/completion-summaries/SPEC-011-buttons-unit-1.json`) recorded the real buttons for three runs. Run 101 was עוֹבֵד, לוֹמֵד, גָּר, אֲנִי. "Do you work here?", "Do you study here?", and "Do you live here?" are all correct Hebrew, so three of the four buttons are right. "אַתָּה עוֹבֵד פֹּה?" is itself a `listen_pick` line in lesson 6 of the same unit. The SPEC-011 check passed this step, because it only checked that every button was a taught word.
- **Unit 11 (learner 1, finished).** Four clozes have no English cue, and in each the only same-part-of-speech word in the unit forms a sentence the unit itself teaches. Lesson 5: "יֵשׁ לִי זְמַן." with target יֵשׁ, where אֵין is the other particle in the unit, and "אֵין לִי זְמַן." is in lesson 3's story. Lesson 4: "אֵין לִי כֶּסֶף." with target כֶּסֶף, where זְמַן is the other noun. Lesson 1: "יֵשׁ לִי זְמַן." with target זְמַן, where כֶּסֶף is the other noun. Lesson 3: "אֲנִי חוֹפְשִׁי." with target חוֹפְשִׁי, where עָסוּק is the other adjective, and "אֲנִי עָסוּק." is in the same lesson. I confirmed the parts of speech with `lookup`: both יֵשׁ and אֵין are `particle`, and both עָסוּק and חוֹפְשִׁי are `adj`. I did not capture the rendered buttons for unit 11. The tier logic makes the collision very likely, but that part is an inference.
- **A predictable future collision even with the English cue.** `lookup` shows חוֹפְשִׁי glossed "free" and פָּנוּי glossed "free; available", both adjectives. Coach note 33 says פָּנוּי is "still to come." The lesson player removes a wrong button only when its gloss equals the answer's gloss exactly (`glossKey` in `lesson.go`), so once פָּנוּי is taught, "אֲנִי __." with the cue "I am free." can offer two right answers.

**The tests certify the code, not the intent.** Three findings show the same pattern:

- `unit.same_shape` was specified to warn on a copy, and the code warned on a varied unit instead. The test pinned the inverted behaviour (ASMT-004, citing `internal/units/shape_rules_test.go:145-163`). The tests agreed with the bug.
- The live MCP server's instructions tell authors that "a recall, produce, cloze, or say with two or more words that are in the lexicon sets target." The same live server's validator blocked all five `say` targets in unit 11 with "only a recall, listen_pick, produce, or cloze activity may set one" (`internal/units/validate_target.go:97-99`). The repo's `internal/mcp/server.go:37` and `schema.go` disagree with each other on this too. The AI author is being told two opposite things.
- The only check that used real production content, SPEC-011's button run, verified a property (buttons are taught) that was not the property Brian complained about (buttons are wrong).

**There are two answer-button engines with different safety logic.** Review uses `activity.Pick` (`internal/activity/distractors.go`), which drops a wrong option whose gloss shares any stemmed content word with the answer. Its comment states the limit honestly: it "cannot see synonyms that share no word (big and large), which only a reviewed lexicon can rule out." Lessons use `wideChoices`, which drops only an identical gloss. The weaker engine serves the riskier exercise.

**Grading-relevant data lives in punctuation conventions.** INIT-037 keeps the wrong side of a tutor-mistake contrast out of word lists by splitting on " — " (`internal/db/put_unit.go:259-266`). Finished units 8 and 11 write contrasts as "אֲנִי קוֹנֶה, לֹא אֲנִי לִקְנוֹת." with a comma, so the split does not apply. Words from finished units count as taught under ADR-0032 D1. So the wrong form לִקְנוֹת is probably still eligible as a button. I verified the separator and the unit text, but not the stored word list, so this is a strong inference. The learner digest shows אֶת "confused with" לֹא four times, which is consistent with this.

**Which class of defect is beyond rules.** A structural validator can prove shape: field presence, token counts, lexicon membership, coverage ratios, repetition. It can sometimes prove ambiguity, for example when substituting a wrong button produces a sentence that already exists in trusted text. It can never prove uniqueness or naturalness. Defects beyond rules are:

- Another option is also correct (semantic).
- The Hebrew is grammatical but unnatural, or the niqqud is wrong.
- The English translation is loose or wrong, so the cue picks the wrong answer.
- The audio clip mispronounces the text.
- The question is technically valid but teaches nothing, for example "pick the Hebrew for 'free'" where only one button is an adjective.

Every new rule in INIT-029 through INIT-037 was written after the previous incident. That pattern will continue, because the defect space is open-ended and the rules are closed.

##### 4.2 Bad questions corrupt the learner model, and nothing undoes it

This is the most important reliability problem I found, and it connects Brian's two complaints ("only one is right but technically all are right" and "Progress says you don't know any words").

- A wrong pick writes an attempt, and "a wrong pick of another word writes a confusion" (`internal/activity/check.go`, header comment).
- The live digest for learner 1 lists today's struggle words: גָּר ("lives"), last seen 2026-11-08 21:27 UTC, and אוֹמֵר ("says"), last seen 21:27 the same evening. ASMT-004 places both in the broken lesson 12 session. It also lists אֶת with cause "confusion", confused with לֹא, count 4.
- Struggle words feed the brief (`brief.struggle_reuse` requires them to be reused). So a bad question produces a false struggle, which makes the next lesson drill that word more, often with the same kind of question.
- "Report a problem" hides the unit (`ReportUnit`, `internal/db/learner_publish.go:283-367`), but it does not void a single recorded answer, lapse, or confusion. It also works only on a whole unit, not on one step.

I infer, but did not measure, that part of the low English-to-Hebrew recall retention (59% on 2026-11-08 in the digest) and the low "known" counts come from answers that were never fairly gradable.

##### 4.3 With no human checker, what can actually guarantee correctness (focus question 2)

Brian's answer is that no person will ever check the Hebrew. So a guarantee has to come from construction (the exercise cannot have a second right answer) or from a trusted source (a human wrote the sentence). Probabilistic checks, such as an AI judge, can lower the defect rate but cannot guarantee anything. Here is how each option holds up:

- **Exercises that are checkable by construction: strong.**
  - *Dictation* (hear a sentence, type it): the answer key is the exact text that produced the audio. The only ambiguity is homophones. Modern Hebrew merges several letter sounds (for example אֶת and עֵט sound the same). That can be checked from data: two taught words with the same `translit` collide. `fsrs.GradeTyped` already strips niqqud and normalizes.
  - *Listen and pick the line you heard* (the current multi-word `listen_pick`): this is unique by audio for the same reason, with the same homophone guard.
  - *Typed recall of a single word from an English cue*: this is checkable only if no other taught word shares the gloss. That is computable with the stronger `GlossesOverlap`, and the item should be skipped when there is overlap.
  - *Form choice*, where the wrong buttons are other forms of the same word (other person, tense, gender, or number) and the cue fixes which form is meant: this is checkable from the verb and noun form tables, provided the code compares the surface spellings. Some cells share a spelling. For example, future tense 2nd person masculine and 3rd person feminine are the same word (תָּגוּר), and present tense has no person.
  - *Building a sentence from known words*: this is not checkable by construction, because Hebrew word order is flexible. Unit 8 itself contains both "אֶתְמוֹל חָזַרְתִּי." and "חָזַרְתִּי אֶתְמוֹל.". The `accept` list can never be proven complete. It is safe only if an unlisted order is graded "not counted" rather than "wrong".
- **Drawing sentences from trusted sources: strong, if supply exists.** If the graded Hebrew is the tutor's own sentence, correctness and naturalness are inherited from a person. This matches Brian's stated goal ("relearn our lessons"). The risk is supply. The digest shows no tutor corrections in 30 days and an empty `taught` list for the last 14 days. The tutor-course worksheets I read are unpointed and would need automatic niqqud, which is itself an error source. I did not verify whether a person wrote them. The lexicon is not an independent trusted source either. The author skill lets the AI author add entries (`upsert_lexemes`), and the `verified` flag "is for a person," who by Brian's answer does not exist.
- **An automated learner-simulation run-through: useful, but deterministic first.** The valuable part is rendering every step exactly as the player would, including the real buttons for the real learner state, and running checks on those screens. SPEC-011 did a tiny version of this (one step, three seeds). An AI "pretend learner" on top adds the same value and the same limits as a judge.
- **A second AI judge: worth having as a one-way filter.** Ask a model from a different family than the author: "With this English cue, is more than one option correct? Is this line natural modern Hebrew? Is the translation faithful?" Any doubt removes the step or turns it into ungraded exposure. A clean verdict is not proof. It must judge the full candidate pool for a gap, not just one sample of buttons, because the buttons vary by seed. Judges from the same model family make correlated mistakes, so pick a different family.
- **Holding lessons for a day: no value by itself.** A delay helps only if something checks during it, and nobody will. If a slower judge or an audio check needs time, run it as a batch before publishing rather than as a waiting period.
- **Erasing answers from a reported lesson: yes, and make it finer.** This is the cheapest trust repair available. A report should void the wrong answers, lapses, and confusions that came from the reported steps. A one-tap "My answer was also right" on the feedback screen should regrade that one answer as right, void its lapse and confusion, and add the option to that step's exclusion list. This uses Brian as an oracle only at the moment he notices a problem, which is not the same as asking him to check lessons.

##### 4.4 What should happen to content when rules change (focus question 3)

Today the policy is one setting for every rule. A new rule blocks new writes and only warns on published revisions (`validate_cloze.go:12-20`), so known-bad content keeps serving. INIT-037 SPEC-010 now skips an untouched unit that blocks, but a started or finished unit still plays. The unit 11 dry run shows why one setting is wrong. Its 13 blocks are 8 missing cloze cues, which are a real correctness problem, and 5 `say` targets, which are a contradiction between the docs and the validator that harms nobody.

The policy should depend on what the rule protects:

- **Correctness rules** (one right answer, missing cue, untaught target or button, authoring notes in learner English): enforce them when the step is served. A failing graded step is shown as ungraded exposure (show the answer, record no grade) until it is rebuilt. The learner stops being harmed the day the rule ships, without waiting for a rewrite.
- **Teaching-shape rules** (drill share, recycle counts, same shape, story length): apply them to new writes only. Old units keep their shape.
- **Mechanical or schema rules** (for example a stray `say` target): fix the stored documents with a deterministic data migration, or downgrade the rule to a warning. They should not inflate block counts.
- **Version the rule set.** Stamp each published revision with the rule-set version it passed, the same way the learner model stamps `rule_version`. Then "which servable steps fail today's correctness rules" becomes one query, and a nightly read-only sweep can report it.

##### 4.5 Which exercise types are risky to auto-generate (focus question 4)

This ranking is about AI-generated graded Hebrew with no human check:

| Exercise | Risk | Main failure |
| --- | --- | --- |
| `cloze` choosing between different words | High | Another word fits; the cue is missing or loose |
| `produce` ("Pick the Hebrew for X", 3 buttons) | High | Near-synonyms; also only 3 buttons, so guessing passes 1 time in 3 |
| `recall` (meaning buttons) | Medium-High | Gloss near-duplicates, because lessons use the exact-gloss check |
| `order` | Medium | Other valid word orders missing from `accept` |
| `pattern` with a right/wrong contrast | Medium | Wrong form leaks into word lists through punctuation conventions |
| Story, dialogue, translations (ungraded) | Medium | Unnatural Hebrew, wrong niqqud, wrong English; they never fail a test, but they mislead |
| Audio for everything | Medium | Text-to-speech mispronunciation; only presence is checked (`audio.missing`) |
| `listen_pick` (which line did you hear) | Low | Homophones only |
| Dictation, typed spelling of a heard word | Low | Homophones; a one-letter slip on a two- or three-letter word can be another word |
| `shadow`, `say` (exposure, not graded) | Low | Bad audio only; no grade is recorded |

##### 4.6 Release confidence (focus question 5)

Brian said a broken morning is tolerable ("I keep going"). The code release gates are already reasonable. CI runs gofmt, templ drift, and go vet in shard C (`.github/workflows/ci.yml` lines 532 to 577), plus three test shards, and the hand-edited templ problem is exactly what that gate caught today. More code tests will not move trust. QA effort should move to three places:

- **Content gates at publish time**, because that is where trust is lost.
- **Learner-model integrity**: no lapse from an unprovable wrong answer, and voiding on report.
- **A two-minute post-deploy smoke test**: check `/version` against the pinned commit (the image updater is out of sync), answer one lesson step and one review card with the read-only smoke token, and try the Argo pin revert once so rollback is known to work.

Also stop writing tests that are derived from the code just written. Acceptance tests should come from the spec's acceptance criteria and from a snapshot of real production units and learner state. The golden validator tests use a synthetic eight-word vocabulary (`validate_golden_test.go`), and the defects Brian hits come from real content meeting real learner state.

#### 5. Opportunities

1. **Validate what is shown.** Freeze each graded step's button set when the unit is published, store it with the revision, and run every check on the rendered steps. This turns the play-time shuffle into a checkable artifact. INIT-032 SPEC-005's planned `accept` exclusion list and the existing `choiceExclude` hook are the building blocks.
2. **Change the distractor strategy, not the rules.** The tutor-course worksheet in "Aharon's code" (`worksheets/shai-date-indirect-speech/worksheet.js`) builds each wrong choice to break exactly one named rule (pronoun not shifted, tense shifted, ש instead of אם) and gives a `why`. Its word bank mixes parts of speech, so only one word fits each blank. That is the opposite of "same part of speech first," and it is far safer.
3. **A cheap, automatic ambiguity detector.** For every multiple-choice step, put each wrong button into the line and look for the result in trusted text: every line of every unit, the tutor's material, and the corpus. A match proves two right answers. This alone would have blocked unit 1's "Do you ___ here?" and all four unit 11 clozes above.
4. **Turn complaints into a regression suite.** Each "My answer was also right" tap, together with the step, becomes a labelled test case for the judge and the ambiguity detector. Brian's frustration becomes the oracle the project lacks.
5. **Audio round trip.** Run speech-to-text on each new clip and compare the unpointed result to the unpointed source. This matters because the goal is conversation, so listening input quality matters most. I assume a Hebrew speech-to-text service is available; I did not verify one.

#### 6. Risks and concerns

| # | Risk | Rating |
| --- | --- | --- |
| R1 | Word-choice multiple choice keeps producing "several right answers," because uniqueness is a question of meaning and rules cannot decide it | High |
| R2 | False "wrong" grades become lapses, confusions, and struggle words, which drive the next brief and depress Progress; reports undo none of it | High |
| R3 | Self-certifying gates give false confidence: tests pinned an inverted rule, docs contradict the validator, and a real-content check tested the wrong property | High |
| R4 | Trusted sources run dry: no tutor input in 30 days, the lexicon is writable by the AI author, and the `verified` flag needs a person who will not exist | Medium |
| R5 | An AI judge makes errors correlated with the author's, or flags so much that lessons become too short | Medium |
| R6 | Text-to-speech errors are invisible, even though listening is the goal skill | Medium |
| R7 | Rule-version changes move Progress numbers without explanation (knownAnySkill went from 38 to 36 when rule version 4 took effect today), which erodes trust in the numbers | Low-Medium |
| R8 | A deploy breaks a study morning | Low (Brian tolerates it, and CI already gates format, templ, and vet) |

R1 and R2 together are the core problem: a bad question costs Brian a moment of trust, and it also teaches the system something false about him, which it then acts on.

#### 7. Scenarios

**Optimistic.** Graded steps are restricted to forms that are checkable by construction. Buttons are frozen and checked at publish. An unprovable wrong answer records nothing. Within two weeks, false-wrong events reaching Brian fall close to zero. Progress becomes believable because every lapse is a real mistake. Trust recovers, and he gives the 20 to 30 minutes a day he offered. The AI's job narrows to choosing, sequencing, and explaining, which matches his own phrase "supplemental tutor."

**Pessimistic.** The project keeps adding a rule after each complaint. The author learns to satisfy the rules on paper, for example by adding an English line to every cloze that is too loose to pick one answer. New defect classes (unnatural Hebrew, wrong niqqud, mispronounced audio) stay invisible because no gate can see them. Tutor material never arrives, so "trusted source" stays an idea, and the app falls back to AI-generated graded sentences. False lapses keep feeding struggle words, the brief keeps recycling them, and lessons keep feeling "the same." Brian stops.

#### 8. Recommended approach

**Principle.** Only grade what the machine can prove. When the app cannot prove Brian is wrong, it must not record him as wrong.

**Gate G1 — What is validated is what is shown.**
Freeze the button set for each graded step at publish time and store it with the revision. The player serves the stored set, filtered to taught words at play time. If filtering leaves fewer than three, it shows the existing reveal. All gates below run on the rendered steps.
*Acceptance:* for every graded step of every servable unit, the served buttons are a subset of the stored set across 50 seeds and three learner states. A test proves that no button is ever added at play time.

**Gate G2 — Only safe question forms are graded.**
Allowed graded forms: dictation; listen and pick the line heard; typed single-word recall where no other taught word overlaps in gloss (using `GlossesOverlap`, not exact match); form choice where the wrong buttons are other surface forms of the same word that contradict the cue; and building a sentence where an unlisted order is "not counted." Word-choice clozes with same-part-of-speech wrong buttons are no longer graded. Wrong buttons, where used, must be either other forms of the same word or words that cannot fill the slot.
*Acceptance:* the validator blocks a graded word-choice cloze whose stored buttons share the target's part of speech. Unit 1 lesson 2 and unit 11 lessons 1, 3, 4, and 5 fail this gate as written.

**Gate G3 — Ambiguity detector (deterministic).**
Substitute each stored wrong button into the line. Block when the result, normalized and compared without niqqud, appears in any trusted text: any unit line, tutor material, or the corpus. Also block when two taught options share a `translit` (homophones) on a listening or dictation step.
*Acceptance:* a fixture built from today's published units flags unit 1's "אַתָּה עוֹבֵד פֹּה?" and unit 11's "אֵין לִי זְמַן." and "אֲנִי עָסוּק.". Zero false flags on a set of known-good steps.

**Gate G4 — Independent judge (probabilistic, one-way).**
A model from a different family than the author reviews each graded step's full candidate set and each Hebrew line: "more than one correct option?", "natural modern Hebrew?", "translation faithful?". Any doubt removes the step or makes it ungraded exposure. After any change, all gates run again.
*Acceptance:* a labelled regression set grows from Brian's reports and appeals. It starts with the unit 1, unit 11, lesson 12 "אֲנִי __", and חוֹפְשִׁי/פָּנוּי cases. The judge flags at least 90% of known-bad items and at most 20% of known-good ones. The set runs in CI whenever the judge prompt or model changes.

**Gate G5 — Grading cannot hurt the learner model unless the error is provable.**
- An unprovable wrong answer (unlisted word order, near-synonym, a step G3 or G4 would now flag) is stored as "disputed". It shows the expected answer and records no lapse and no confusion.
- Add a one-tap "My answer was also right" on feedback. It regrades the answer as right, voids its lapse and confusion, adds the option to the step's exclusion list, and logs a defect.
- A unit report voids the wrong answers from that unit's runs.
*Acceptance:* after an appeal, the card's state, FSRS stability, due date, and confusion rows equal their values before the answer. This is tested. A one-time backfill voids lapses from lesson steps that today's correctness rules flag, and the Data Analyst reports the before and after numbers.

**Gate G6 — Content regression and rule classes.**
Tag each rule as correctness, teaching shape, or mechanical. Correctness rules are enforced when a step is served, by downgrading it to ungraded. Shape rules apply to new writes only. Mechanical rules are fixed by data migration. Stamp each revision with the rule-set version. A nightly read-only sweep reports servable steps that fail correctness rules, and the weekly report shows the count. CI fails a change that increases that count on a snapshot of production units.
*Acceptance:* the sweep's count of graded servable steps failing correctness rules is zero, and stays zero after every rule change.

**Gate G7 — One source of truth for rules.**
Generate the MCP instructions, tool descriptions, `authoring.md`, and the author skill's rule table from the `Rules` catalog, or test them against it.
*Acceptance:* a test fails when any document says `say` takes a target while `gradedActivity` excludes `say`. That contradiction exists in production today.

**Supporting changes:**
- Merge the two button engines so one module decides whether an option is fair.
- Store the wrong side of a contrast as a structured field, never as punctuation.
- Run the audio round trip on new clips.
- Add the post-deploy smoke test described in 4.6.

**What not to do:** add more validator rules for each new complaint; hold lessons for a day; add more code-path tests to an already gated CI.

**Order of work:** G5 first, because it stops the damage to Brian's data now. Then G1 and G2, because they remove the defect class. Then G3, G7, and G6. Then G4 and the audio round trip.

#### 9. Questions for other specialists

1. **For the Technical Architect:** Can button selection move from play time to publish time, stored per revision, while still honouring "taught words only," which depends on learner state at play time? Is "stored superset, filtered at play time" acceptable to you, and what does it cost to store rendered steps per revision?
2. **For the Technical Architect:** Should one module own "is this option fair" for both lessons (`wideChoices`) and review (`activity.Pick`), and should grading-relevant data such as the wrong side of a contrast become structured fields?
3. **For the Product Strategist:** Can Brian reliably get his tutor's lesson sentences into the app every week, as text, photos, or worksheet files like those in "Aharon's code"? If not, the trusted-source strategy has no supply. Would he accept fewer exercise types (no graded word-choice clozes) in exchange for never being marked wrong unfairly?
4. **For the Data Analyst:** How many of learner 1's current lapses, confusions, and struggle words came from lesson steps that today's correctness rules flag, such as clozes with no cue? If those are voided, how much do Progress and recall retention change? Can you define and track a "false-wrong rate" from appeals and judge audits?
5. **For the Devil's Advocate:** If AI-written graded Hebrew cannot be trusted without a human, is a lesson generator worth keeping at all, compared with review over tutor material plus an AI that only explains, sequences, and picks what to practise?

#### 10. Assumptions and things I did not verify

- I did not capture rendered buttons for unit 11. The collisions follow from the tier logic and confirmed parts of speech, but they were not observed.
- I did not read the stored word lists (`lesson_items`) for units 8 and 11. The claim that לִקְנוֹת and לֹא remain eligible as buttons is an inference from the separator code and the unit text.
- The link between the struggle words or the אֶת/לֹא confusion and ambiguous questions is an inference from timing and code paths. I did not inspect individual attempt rows.
- I assume a Hebrew speech-to-text service and a judge model from a different family are available at acceptable cost.
- I do not know whether a person wrote the "Aharon's code" worksheets.
- The file `internal/web/prod_buttons_test.go` appeared in the session's starting git status but was not present when I looked, so I did not use it.

### Summary

**Position:** The gates miss Brian's complaints because they check the unit document while he sees buttons picked later, and because "is another answer also right" is a question of meaning that no structural rule can settle. Quality has to come from grading only questions that are correct by construction or by trusted source, checking exactly what is shown, and never letting an unprovable "wrong" count against him.

**Recommendation:**
- Make grading safe first: unprovable wrong answers record nothing; add a one-tap "My answer was also right" that voids the lapse; reports void answers.
- Freeze and validate the actual buttons at publish.
- Restrict graded steps to forms that are checkable by construction (dictation, listen and pick the line, gloss-unique typed recall, form choice, sentence building scored "not counted" when unlisted). Stop graded word-choice clozes with same-part-of-speech wrong buttons.
- Add a deterministic ambiguity detector and an independent judge as one-way filters.
- Classify rules so correctness rules apply to old content when it is served.
- Generate the docs from the rule catalog.
- Put QA effort into content and learner-data integrity, not more code tests.

**Top risk:** False "wrong" grades from ambiguous questions turn into lapses, confusions, and struggle words that drive the next lesson and depress Progress, and nothing currently undoes them (R2, High, together with R1).

**Confidence:** Medium. The diagnosis rests on direct evidence and I hold it with high confidence. How well the prescription works depends on tutor material actually arriving and on judge quality, neither of which is proven.

---

## Phase 4: Dialogue Contributions

### Questions Posed
- To the Technical Architect: can the buttons be frozen per revision, and should one module decide whether an answer option is fair?
- To the Product Strategist: will Brian accept fewer question types in exchange for never being marked wrong unfairly?
- To the Data Analyst: how many lapses came from flagged questions, and can a false-wrong rate be tracked?
- To the Devil's Advocate: is a lesson generator worth keeping at all?

### Questions Answered
- **From the Data Analyst:** How many of learner 1's lapses and struggle words came from flagged questions?
  **Response:** It isn't computed yet, but the struggle list is contaminated: גָּר and אוֹמֵר were last missed inside lesson 12. Void evidence from units 12 to 34 and from word-choice blanks with no English cue before archiving any unit, because the void needs `unit_id`.
- **From the Product Strategist:** Which checks let class practice reach Today safely?
  **Response:**
  - Grade only formats correct by construction from a trusted key: typing a tutor sentence, putting a tutor sentence in order, dictation, and choosing which tutor line was heard.
  - Every graded word must be an exact, taught lexicon form.
  - A judge from a different model family flags doubtful items, and any doubt makes the item ungraded.
  - Pause new packs if there is more than one "my answer was also right" tap per 50 graded items in a week.
- **From the Devil's Advocate:** What single acceptance test by Brian would have caught lesson 12?
  **Response:** Brian won't check lessons, since that checking is the trust problem itself. A robot run-through before Today, plus a "what Brian will see differently" line with a screenshot on every change, would have caught it.

### Tensions Involved In
- Fewer graded formats or more evidence: production formats give stronger evidence, so quality rises.
- Reusing tutor text: supports tutor-written lines as the only trusted keys. Brian's typed lines are never keys.
- How much the AI writes: only items with a trusted key are graded, and the AI never grades itself.

### Synergies Identified
- Never count an answer the app can't prove wrong: shared with Data and the Devil's Advocate.
- More production practice against trusted keys: shared with Product, the Architect and Data.
- Generate the docs from the rule catalog: shared with the Architect and the Devil's Advocate.

---

## Phase 5: Consensus Position

### Option Scores
| Dimension | A: Patch in place | B: Staged | C: Re-center now | D: Freeze and go simpler |
|-----------|---|---|---|---|
| Learning value | 2 | 4 | 3 | 2 |
| Trust and content correctness | 1 | 4 | 2 | 3 |
| Effort and time to value | 2 | 4 | 2 | 3 |
| Treadmill risk (higher = lower risk) | 1 | 4 | 2 | 4 |
| Preserves progress and history | 3 | 4 | 3 | 3 |
| Reversibility | 4 | 4 | 2 | 5 |
| **Total** | **13** | **24** | **14** | **20** |

Score notes, from the quality lens:
- **A** keeps the source of the defects. The validator checks the unit file while Brian sees buttons picked at play time, and "another button is also right" is a question of meaning that no rule settles. Each fix is a new rule written after the next incident, and each bad question keeps writing false lapses into the learner model. It keeps all the history, but the history keeps getting less true.
- **B** removes my two high risks in Stage 1 with small changes: wrong answers the app can't prove stop counting, and the formats that produce them stop being graded or stop reaching Today. The audit gate means Stage 2 is built only on tutor material that has been shown to exist. It gets 4, not 5, on trust because as written it does not say that safe grading also covers review and the 11 finished units, or how Stage 1 is shown to be done.
- **C** aims at a similar end state, but it has no Stage 1 trust fixes, no void of the contaminated evidence first, and a large new surface built with no gate: telling the tutor's lines from Brian's typed lines, and adding vowel points to unpointed text by machine. It would be built by the same process whose tests pinned an inverted rule. Retiring units in parallel also risks archiving them before the void, which needs `unit_id`.
- **D** stops new bad lessons and is fully reversible, but nothing gets fixed. App review keeps its false "wrong" grades and its hidden listening cards, the contaminated struggle list stays, and anything studied in Anki or the worksheets is invisible to the learner model, so the "AI tutor that knows where I am" loses its data for 14 days.

### Final Position
**Preferred Option:** B, Stabilize, then re-center on the tutor (staged).

**Rationale:** Brian's binding constraint is trust, and from my lens the root cause is that the app records him as wrong when it can't prove he is wrong, then builds his next lesson on that false record. Stage 1 fixes exactly that with small, mostly subtractive changes that each have a clear pass line, and the audit gate stops Stage 2 from being built on tutor material nobody has loaded. B is the only option that fixes the known harm now and also puts a measured gate in front of the risky build.

**Conditions:**
1. **Void first, then rebuild Progress once.** Void the unprovable wrong answers (units 12 to 34, and word-choice blanks with no English cue) before any unit is archived, because the void needs `unit_id`. A void is a mark on the answer, not a deletion, so it can be audited and undone. Rebuild the frozen ladder only after the void, so Brian's numbers move once, shown with the old and new numbers side by side.
2. **Safe grading covers everything that is served, not only new lessons.** "No lapse and no confusion from an answer the app can't prove wrong" and "stop grading word-choice blanks" apply when a step is served, so they cover review and the 11 finished units too. Review's own answer buttons keep using the stronger gloss-overlap check, and lessons switch to it.
3. **The "my answer was also right" tap is tested as a true undo.** After a tap, the card's state, its FSRS stability, its due date, and its confusion rows equal their values before that answer. Each tap is saved with its step as a regression test case.
4. **Stage 1 has a measured exit, checked by an agent that did not build it.** Before Stage 2 starts: a read-only sweep finds zero graded steps reachable from Today that fail a correctness rule; in the past week there was no more than one "also right" tap per 50 graded items; and a robot run-through of one Today session plus a two-minute post-deploy smoke test (`/version` matches the pinned commit, one lesson step, one review card) both pass.
5. **Stage 2 grades only trusted keys.** The tutor's lines and Brian's typed lines are stored as separate structured fields, never split on punctuation. A line whose vowel points were added by machine is graded only where grading ignores vowel points, as typing and dictation already do. The validator rules retire with the unit pipeline rather than carry over to the class packs.

**Dissent:** Aligned with consensus. One reservation, not a block: generating word clips for every taught word puts a large batch of unchecked text-to-speech audio into the goal skill at once. I would add a speech-to-text round trip on new clips if one is available at reasonable cost. If not, ship the clips anyway, because 56 hidden listening cards cost more than an occasional mispronounced word.
