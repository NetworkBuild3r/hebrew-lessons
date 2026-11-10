# Devil's Advocate — Brainstorm Analysis

**Session:** Is the Hebrew app built right? (BRAIN-001)
**Date:** 2026-11-09
**Domain:** Security & Risk
**Primary Lens:** Test the assumptions behind both "keep fixing the app" and "rebuild it around the tutor's lessons". Expose the risks that come from the development process itself, from progress numbers that keep changing meaning, and from the gap between what the app trains and the goal of holding a conversation.

---

## Phase 3: Independent Analysis

### Chain of Thought

#### 1. Primary lens

I look for the things nobody wants to say out loud. What if the obvious fix is wrong? What if the process that builds the app is the defect? What if the app, even when perfect, is not what moves Brian toward conversation? I rate risks by how likely they are and how much damage they do to the one constraint Brian named as binding: trust.

Evidence was read only. I read the session context, the initiatives from INIT-026 to INIT-037, ASMT-004, the ADR metadata, the class-data runbook, the "learner without a tutor" design doc, RSCH-011, Aharon's worksheet code, the lesson brief builder, the review mix, and the standing code. I also made read-only MCP calls for learner 1 (`get_study_digest`, `get_progress`, `get_learner_model`). Anything I did not verify is marked as an assumption.

#### 2. Initial assessment

The app's main defect is a loop, not a function. A complaint becomes an initiative the same day. The initiative adds rules or changes what a number means. The rules shrink what a lesson can be until lessons are small and repetitive, and that produces the next complaint. "Rebuild around the tutor's lessons" is the obvious answer, but the tutor data has never been loaded. The one time tutor sentences entered the pipeline, they broke lesson 12. And the goal, conversation, depends mostly on listening hours and speaking with people, which the app does not currently train or measure.

#### 3. Main considerations

**C1. The process is the engine of the defects, and a rebuild through it will likely repeat them.**

- **Volume against study.** There are 258 commits from 2026-10-07 to 2026-11-09 (`git log`), 34 initiative folders, and 31 ADR files (ADR-0029 is missing, and INIT-036 and INIT-037 say their ADR and INIT numbers were assigned by hand). The four weekly reports show 0, 35, 57 and 64 minutes of study, and the digest shows 45 minutes in the last seven days (`get_progress` weeklyReports; `get_study_digest` minutesInApp). That is roughly three hours of tracked study against 258 commits, about 75 to 100 commits per hour of study. Tracked time leaves out tutor sessions and study outside the app, so the real ratio is lower, but the imbalance is large.
- **The same complaint, twice, with the fix causing the second one.** INIT-030 (2026-11-05) opens with Brian's words: "Our lesson plans are the same over and over. They kind of suck." (`sdd/initiatives/INIT-030-lessons-that-build/INIT-030-lessons-that-build-initiative.md` line 4). Its fixes, a consolidation pace rule and a `unit.same_shape` check, produced the 23 cloned units (ASMT-004, faults 2 and 5). In Phase 2 Brian said it again: "every lesson is the same shit."
- **Fixes collide with each other.** INIT-036's stricter "known" rule raises the Learning count that the pace switch reads. On its own, that would have produced more consolidation units, and the only reason it didn't is that ASMT-004 caught it (`INIT-036...initiative.md` line 99; ASMT-004 option 2).
- **Tests encode the agent's misunderstanding.** The inverted `unit.same_shape` rule was pinned by its own test (`internal/units/shape_rules_test.go:145-163`, cited in ASMT-004). The repo has about 92,000 lines of tests, and none caught a lesson asking about words Brian never had. Tests written by the agent that wrote the code check the agent's reading of the spec, not the owner's intent.
- **Agents grade their own work.** ASMT-004 states the conflict of interest itself: "The same kind of AI agent that built and revised these lessons over MCP wrote this report" (line 21). INIT-036 and INIT-037 score their own plans 39/50 and 40/50 in sections titled "self-review, not independent". The eval judges are AI agents too. The only human acceptance test is Brian sitting a lesson after it is live.
- **Written rules are not enforced.** The class-data runbook says "Backfill under way: not yet" and "Until that line is filled in, load nothing" (`docs/operations/class-data-through-mcp.md` lines 10 to 12). Yet ten personal mistakes were recorded on 2026-10-26 (`get_learner_model` mistakes). The INIT-026 completion report says "Not committed. Not pushed. Not deployed." while its status is complete (line 5). ADR-0031 D5 says history is rebuilt under rule version 4, but the live digest still stamps every past day with rule version 3 and today with 4, so the trend line Brian sees mixes two definitions (`get_study_digest` history).
- **The rules only ratchet one way.** There are 61 validator rule ids, a 364-line author skill, and a 352-line authoring guide. Each complaint added rules. I did not audit whether any rule was ever removed (assumption: few or none). Each new rule narrows the set of valid lessons.

Would a redesign run through the same process produce the same treadmill? I think so, because none of the five drivers lives in the code: complaints become initiatives the same day, agents grade their own work, acceptance is checked at the code level, rules are only added, and the learner model only believes in-app evidence. A rebuild replaces the code and keeps all five.

**C2. The app's model of Brian is anchored near zero, and every safety rule inherits that.**

- **Placement disagrees with itself by a factor of 25.** The four placement runs estimated 0 (2026-10-19), 0 (2026-10-21), 20 (2026-10-29), and 522 listening words (2026-11-06) (`get_progress` placements). Since 2026-11-06 the listening snapshot says about 505 words estimated known, while listening known or solid is 10.
- **His own mistakes show a much higher level.** The mistakes recorded on 2026-10-26 are real A1+/A2 attempts: "אֲנִי חָשַׁבְתִּי שֶׁהַכֶּלֶב אָכַל אֶת הַפִּיצָה", "הֶחְזַרְתִּי אֶת הַנַּעֲלַיִים", "לִפְנֵי שְׁלוֹשָׁה יָמִים", "כְּשֶׁאֲנִי בְּטִיּוּל", and "תִּצְלֶה אֶת הָעוֹף בַּתַּנּוּר". Someone attempting past tense, future tense, construct phrases, and subordinate clauses is not a learner with 2 known words in English to Hebrew, which is what the standing says (recall known 2, solid 1).
- **The tutor's course is pitched at the same level.** Aharon's worksheet is labeled "Level A1+ / A2" and teaches indirect speech (`Aharon's code/worksheets/shai-date-indirect-speech/worksheet.js` line 6).
- **Brian is right about spelling.** Spelling is 97% right over 33 checked answers in 14 days, yet it shows 3 known and 1 solid (`get_learner_model` standing). The headline is measuring how young the cards are (two study days, FSRS stability), not whether he can spell. That is exactly his complaint: "I know how to spell words... go to progress and says you don't know any words."
- **Yesterday's fix works against his stated job.** INIT-037 defines "taught" as a word with a card, a new word in this unit, or a word in a unit he finished (INIT-037 Decisions, lines 105 to 110). The record of words taught outside the app is empty (`get_study_digest`: `taught` is `[]`, `closedLoop.total` is 0 since 2026-10-11). So words he learned with his tutor are now untaught by rule, and a lesson may not ask about them. The app has moved further from "relearn our lessons", not closer.
- **Sameness is the expected output.** With about 36 known words and "taught words only", the author can only write stories like "I live. You work. You say: ___." (ASMT-004, units 20, 27 and 34). Brian wants "interesting stories". That wish can't be met under the current model and the current rules. This is not a bug in one function. It is what the constraints produce.

**C3. A likely structural trap: listening can't be reviewed, so "review first" may never end.**

- The standing marks listening "not askable" only when the whole lexicon has zero word clips (`internal/learner/standing.go:90`, `:182-189`). The live standing for learner 1 says listening is "not_askable", with the reason "No audio yet — practiced in lessons only".
- The review mix drops due listening cards that have no clip, and the code says "Those stay due" (`internal/session/mix.go:263-281`).
- 56 of the 161 due cards are listening cards (standing: listen 56, read 37, recall 54, spell 14).
- The brief stops adding new words when more than 40 words are in Learning (`internal/brief/build.go:113-116`). It refuses a new unit when a consolidation unit is already queued (`:588-601`). Listening alone has 60 words in Learning.
- Put together: listening, the goal skill, only moves in lessons. Lessons carry no new words while Learning is above 40. The two-day rule keeps words in Learning longer. I have not run this end to end, so it is a hypothesis. But every step of it is in code or in live data. If it holds, "every lesson is the same" is the steady state of the system, and 56 due cards can never be cleared by review.

**C4. "Rebuild around the tutor's lessons" rests on assumptions nobody has tested.**

- **The data has never been loaded.** The runbook gate is unfilled (C1). The design doc said on 2026-10-23: "Do not load class data yet... the backfill is four classes" (`docs/design/learner-without-a-tutor.md` line 91). More than two weeks later, it still hasn't started.
- **There is very little machine-readable tutor material.** Aharon's catalog has exactly one worksheet (`Aharon's code/worksheets/catalog.js`). I do not know who Aharon is: the tutor, a classmate, or Brian porting the tutor's sheets (assumption, needs Brian).
- **Most of it is unpointed.** In that worksheet only the vocabulary headwords carry vowel points. The example sentences, fill-ins, and reading are unpointed (`worksheet.js` lines 10 to 19 against 26 to 29 and 133 to 144). The app needs pointed Hebrew for every graded item (author skill, "The unit document"). Pointing tutor sentences means an AI adds the vowels with no human check, which brings back the unverified AI Hebrew we are trying to get away from.
- **Rights and consent are unresolved.** The design doc says the content is "our own: same structure and order as the class sheets, not the sheets themselves, unless the author agrees" (`learner-without-a-tutor.md` line 110). ADR-0022 D3 says the app stores no worksheet files or transcripts. Now that the app serves only Brian, this may be easy to settle, but it is a decision to make, not one to assume.
- **The one time tutor material entered the pipeline, it broke lesson 12.** The steak sentence and the אֶת הַנַּעֲלַיִים contrast came from his personal mistakes bank and pulled untaught words into the answer buttons (ASMT-004 fault 1; INIT-030 line 17, "the same three fix-the-mistake pairs"). Tutor material sits at the tutor's level, and the app models him far below it. Feeding one into the other produces untaught-word failures unless the model is fixed first.
- **The process already judged the tutor's material and threw most of it out.** RSCH-011 scored five tutor worksheet layouts and recommended building only the scrambled sentence (line 55). It rejected the story with comprehension questions as practice. It also rejected the layout where "the productive act is off-screen". Aharon's "ask a friend" interview is that layout, and it is the only one that makes Brian speak to a person. Brian now asks for exactly those stories. The process put AI-summarized research above the owner's and the tutor's judgment.

**C5. Remembering class material may not be what limits conversation.**

- The goal is to understand Hebrew at normal speed and answer (Phase 2).
- The app trains reading recognition reasonably well: 32 words known or solid in reading. It trains almost none of the goal. Listening review is off (C3). English to Hebrew is 3 known or solid, at 59% retention and 73% accuracy over 14 days. Speaking now records nothing (INIT-036 SPEC-004).
- General knowledge, not verified in this repo: understanding speech at normal speed grows with many hours of listening to natural speech, and speaking grows with real conversation. At about one hour a week in the app, even perfect retention won't produce conversation in 6 to 12 months on its own. His tutor sessions, and any conversation practice (ADR-0022 mentions Grok conversations), are probably doing more for the goal than the app.
- Brian said "nobody" can check his Hebrew, and he also has a tutor. Those answers conflict, so I read them together: the tutor won't review the app's content, but the tutor already is his speaking time with a human. The design doc said so itself: "The tutor's time goes to conversation and correction" (`learner-without-a-tutor.md` line 43). The plan should use that time to measure the goal, not to proofread the app.

**C6. What an AI supplemental tutor can honestly do with no human checker, and where the line is.**

It can do these well, because they are deterministic or anchored to a trusted source:
- schedule review with FSRS and track accuracy per skill;
- keep the record of what he was taught, if someone feeds it;
- grade answers that have a trusted key: typed recall of a lexicon word, a cloze whose answer is the original word of a tutor sentence (typed, not picked from buttons), or a meaning question whose English comes from the tutor or the lexicon;
- write English comprehension questions about a fixed Hebrew text that it did not write;
- make listening audio, keeping in mind that synthetic voices may blur fine contrasts (INIT-036 lists this as a high-impact risk);
- summarize where he stands and choose what to practice next (the digest and learner model already do this).

It can't do these honestly:
- judge whether a Hebrew sentence is natural, or whether several choices are all acceptable. INIT-032 concedes "The server cannot tell whether a verb fits a sentence" (line 49), and that is exactly the "eat, sleep, walk" complaint;
- grade free production ("create a sentence") as right or wrong;
- grade pronunciation or free speech;
- certify its own Hebrew, its own vowel points, or its own wrong-answer buttons.

The line: the AI may generate practice and give feedback freely. Only a trusted source (the tutor's material, a lexicon entry, or the original sentence) decides right and wrong. Only answers with a trusted key change the numbers. "Create a sentence" can exist, but as unscored practice with feedback labeled "suggestion, not checked", and it writes nothing to the learner model.

**C7. Simpler alternatives deserve a fair trial before a custom rebuild.**

- **Anki with a deck built from the tutor's sentences.** Anki has FSRS built in (general knowledge; I did not verify the version), typed answers, cloze, and audio. It costs almost nothing to build. It loses the per-skill model, the checked review without self-grading (partly, since Anki can check typed answers), and the MCP digest.
- **Aharon's worksheet player plus FSRS on the vocabulary.** It is one shared player and one data file per lesson (`Aharon's code/README.md`). The Hebrew app's own first commit was "Hebrew worksheet player, lessons, container and MCP" (2026-10-07). The project started where Aharon's code is and grew to about 67,000 lines of Go.
- **Off-the-shelf listening material, plus an LLM voice partner for speaking practice.** Used as practice, not as a grader.

What the custom app uniquely gives, and is worth keeping:
- a checked review with state per skill, and a strict "known" rule that is honest even if it is unflattering;
- the MCP read surface (digest, learner model, mistakes) that lets any LLM know where he stands. This is the "supplemental tutor that knows where I am" piece, and nothing off the shelf provides it;
- the tutor data tools (`record_taught_words`, `record_mistakes`), which exist and are almost unused;
- the pointed lexicon of about 1,000 entries, and the cached audio for lesson texts (no lexicon word has a word-level clip, per C3).

What it gives that may not be worth keeping: AI-invented lesson units, the reviser loop, consolidation units, placement (four runs, four very different answers), and activity types beyond a handful.

#### 4. Opportunities

- **O1. A trust win with no new code.** Stop AI-invented graded Hebrew. Today already offers review only since INIT-037 SPEC-001. Keep it that way and add only tutor-sourced items.
- **O2. Fix the model instead of adding rules.** Record taught words from tutor sessions with `record_taught_words` (the tool exists) and use the mistakes bank as evidence. Then INIT-037's "taught words only" rule works for him instead of against him, and lessons can use his real vocabulary.
- **O3. Turn listening on.** Either give word clips to the words that have listening cards, so the 56 due listening cards become reviewable, or stop creating listening cards that review can never ask. This needs the Technical Architect to confirm C3.
- **O4. One stable number.** Freeze the definition of the headline for 90 days, and show checked accuracy beside it so "97% right, 4 known" stops reading as an insult.
- **O5. Measure the real goal with time that already exists.** A five-minute check at each tutor session (E4 below).
- **O6. Deletion as a feature.** Every feature removed is one less place for bad Hebrew to reach Today.

#### 5. Risks and concerns

| # | Risk | Rating | Why |
| --- | --- | --- | --- |
| R1 | The redesign runs through the same complaint-to-initiative process and repeats the treadmill: new rules, new definitions, new broken lessons. | **High** | C1. All five drivers sit outside the code. INIT-030 shows a fix producing the next failure. |
| R2 | The learner model stays anchored near zero, so every rule starves lessons and Progress keeps telling him he knows almost nothing. | **High** | C2. Placement ranges from 0 to 522; recall shows 2 known while his mistakes show A2 attempts; the taught-words record is empty. |
| R3 | "Rebuild around the tutor" fails because the material is thin, unpointed, not consented, or never loaded. | **High** | C4. The runbook gate is unfilled, the catalog holds one worksheet, and its sentences are unpointed. |
| R4 | The meaning of the progress numbers keeps changing, and each change, even a correct one, tells Brian the app doesn't know where he stands. | **High** | "Words you know" was redefined four times in three days: ADR-0027 and ADR-0028 on 2026-11-06, ADR-0030 on 2026-11-07, and ADR-0031 on 2026-11-08. The live trend line mixes rule versions 3 and 4. |
| R5 | Goal mismatch: the app gets better at reading recognition while the goal is listening and speaking, and success gets declared on proxies. | **Medium** | C5. The app's job may legitimately be retention, but nothing measures the 6 to 12 month goal. |
| R6 | Adding "create a sentence" with AI grading brings wrong verdicts back at the step he trusts most. | **Medium** | C6. No human checks free production. |
| R7 | Building displaces studying, and sunk cost cuts both ways: keeping 67,000 lines because they exist, or rebuilding because building is more satisfying than studying. | **Medium** | C1's commit-to-study ratio. That building is more fun for Brian is an assumption; ask him, don't assume. |
| R8 | The listening trap keeps "review first" and zero-new-word units going indefinitely. | **Medium** | C3. Every link is in code or live data; it hasn't been run end to end. |
| R9 | Deploy and toolchain fragility (20 to 30 minute CI, no local Go, hand-edited generated code). | **Low** | Brian tolerates it. It slows each fix loop but isn't the core problem. |

Who benefits from us being wrong? Nobody acts in bad faith, but the incentives are lopsided. The process rewards output: every complaint becomes specs, ADRs, and a self-scored plan. No participant in the loop, human or agent, is measured on Brian's Hebrew. That is why an outside measure (E4) and an owner acceptance check (Step 3) matter more than any single code fix.

#### 6. Recommended approach

**Step 0. Freeze for 14 days.** No new initiatives, ADRs, validator rules, or activity types. Allowed: removing or hiding things, and a defect fix only when it blocks a study session, batched once a week. Brian studies 20 to 30 minutes a day on review, which is already what Today offers.

**Step 1. Run five cheap experiments during the freeze, with pass and fail set in advance.**

- **E1. Audit the tutor material (Brian, 1 to 2 hours).** Collect the last four weeks of class material. Count the usable sentences, how many are pointed, how many words are missing from the lexicon, and whether the tutor agrees to their use. *Pass:* at least about 50 usable sentences per week of class, exportable in under 30 minutes a week, with consent or Brian's explicit decision that personal use is fine. *Fail:* a tutor-centered rebuild is off the table, and the app needs another content source.
- **E2. Cold-test the learner model (30 minutes).** Take 40 words from the tutor material and the mistakes bank that the app marks unseen or learning. Test him cold: English to Hebrew typed, and listening. *If he gets more than about 60% right,* the model is anchored too low. Seed it from evidence (taught words, mistakes, placement) before any lesson work. *If he doesn't,* the strict number is honest, and the fix is how Progress explains it.
- **E3. Run a simple baseline (14 days).** Build an Anki deck, or one Aharon-style sheet, from the E1 sentences: typed cloze, typed English to Hebrew, and audio first. Split each day's time between it and the app. Track voluntary minutes, sessions abandoned because something looked wrong (a simple tally), and right answers on a later day. *The custom app earns its keep* if it wins on minutes and matches on accuracy. If the deck wins, the custom lesson generator isn't worth its cost.
- **E4. Check the goal weekly (5 minutes, at a tutor session).** The tutor talks for about a minute at normal speed on a familiar topic. Brian writes down what he understood, answers three questions aloud, and scores himself 0 to 10. This doesn't ask the tutor to check the app. It is the only number aimed at the actual goal.
- **E5. Confirm or kill the listening trap (Technical Architect and Data Analyst, read-only).** Check whether due listening cards can ever clear without lessons, and whether Learning can fall below 40 under rule version 4 at his pace.

**Step 2. Decide after 14 days, with evidence.** My prior, not a conclusion: keep the review engine, the learner model, the MCP read tools, the lexicon, and the audio cache. Replace AI-invented units with practice built from the tutor's own sentences, where the original sentence is the answer key. Drop placement, the reviser, and consolidation units after Brian confirms. Keep stories as reading with English available, ungraded, so they can be interesting without needing a trusted answer key.

**Step 3. Change the process, whatever gets built.**
- At most one initiative a week, with complaints batched and a 24-hour cooling-off before any becomes work.
- Every initiative's "Done when" includes an owner check, for example "Brian finishes five sessions without using Report a problem".
- A rule budget: a new validator rule has to retire an old one. There are 61 today.
- Delete first: every build starts with a list of what it removes.
- The agent that builds never grades its own work. Brian's sessions and E4 are the grade.

**What would prove me wrong.**
- E1 passes easily: the tutor material is rich, easy to load, and consented. Then a tutor-centered rebuild is the right call, as long as Step 3 comes with it.
- E2 fails: he doesn't know the words the app calls unseen. Then the model is right, and my "anchored near zero" argument shrinks to a display problem.
- E3: the custom app beats the simple deck on voluntary minutes and accuracy. Then the custom build has earned its place.
- E4 rises week over week with app changes alone. Then remembering class material is the bottleneck after all.
- After the freeze, with no bad lessons at all, he still studies less than about 15 minutes a day. Then trust isn't the binding constraint, and no redesign will fix study time.

**Optimistic scenario.** E1 shows a steady supply of tutor sentences. Seeding the model (E2) lifts "known" to something he recognizes as true. Practice is built from his own class material, with the original sentence as the answer key and stories he can read for interest. Trust returns, he gives 25 minutes a day, the listening trap is fixed, and E4 shows him following the tutor at normal speed within six months. The MCP digest makes his conversation practice with an LLM targeted at what he actually struggles with.

**Pessimistic scenario.** A rebuild starts now through the same process. Four to eight weeks of agent time produce a new app with new rules and a fifth definition of "known". The tutor backfill keeps stalling, as it has since 2026-10-23. New lessons arrive with new failures, Brian loses trust again, and study stays around an hour a week. In six months the app has been rebuilt twice, and whatever conversation ability he has came from his tutor. The app turns into the hobby that replaced the studying.

#### 7. Questions for other specialists

- **For the Technical Architect:** If we deleted AI-invented units, the reviser, placement, and consolidation, what would remain of the roughly 67,000 lines? Can the review engine, the learner model, and the MCP read tools stand alone as the core? And can you confirm or refute the listening trap in C3: zero lexicon word clips, 56 due listening cards that review skips, and a pace rule that blocks new words above 40 Learning?
- **For the QA Advocate:** The inverted `unit.same_shape` rule was pinned by its own test, and 92,000 lines of tests didn't catch a lesson asking about untaught words. What single owner-level acceptance test, run by Brian or from his sessions, would have caught lesson 12? Can every change be required to pass it?
- **For the Product Strategist:** Brian has a tutor, yet says "nobody" can check his Hebrew. Is human speaking time the real bottleneck to conversation? If so, should money go to more tutor or conversation hours rather than more agent hours? And how do we settle "nobody checks" against "I have a tutor"?
- **For the Data Analyst:** Which one number should Brian see, and can we commit to freezing its definition for 90 days? Spelling is 97% right over 33 answers but shows 4 known or solid. Is the headline measuring skill or card age? And how far below his real level is the model, given his A2-level mistakes and the 522-word listening placement?
- **For the Data Analyst and the Technical Architect together:** Under rule version 4 and his current pace, how many days until Learning falls below 40 and new words come back? If the answer is "never without lessons", the pace rule needs to change before anything else.

### Summary

**Position:** The development process and a learner model anchored near zero are doing more damage than any single bug. A rebuild around the tutor's lessons, run through the same process, would likely repeat the treadmill, and it rests on tutor data that has never been loaded and that broke lesson 12 the one time it entered the pipeline.

**Recommendation:** Freeze new building for 14 days and run five cheap experiments with pass and fail set in advance: audit the tutor material, cold-test the learner model, compare against a simple Anki or worksheet baseline, start a weekly five-minute conversation check at tutor sessions, and confirm the listening trap. Then keep the review engine, learner model, and MCP read tools; build practice only from trusted sentences, where the original is the answer key; delete what doesn't help; and change the process: one initiative a week, an owner acceptance check on every change, a rule budget, and no agent grading its own work.

**Top Risk:** The redesign runs through the same complaint-to-initiative process and produces another month of new rules, redefined numbers, and broken lessons. That spends the trust Brian named as his binding constraint, while the conversation goal, which depends on listening hours and speaking with people, goes unmeasured and unmoved. Rated High.

**Confidence:** Medium. I am highly confident in the evidence for the treadmill, the anchored model, and the metric churn, because each is in code, ADRs, or live data. I am much less sure what the tutor material looks like and whether it can be loaded, which is why E1 comes first. The listening trap and the off-the-shelf comparisons are hypotheses until E3 and E5 run.

---

## Phase 4: Dialogue Contributions

### Questions Posed
- To the Technical Architect: what is left if units, the reviser, placement and consolidation are deleted, and is the listening trap real?
- To the QA Advocate: what single acceptance test, run by Brian, would have caught lesson 12?
- To the Product Strategist: is speaking time with a person the real bottleneck?
- To the Data Analyst: which one number, frozen for 90 days, and how far below his real level is the model?

### Questions Answered
- **From the Technical Architect:** Does narrowing lose the stories?
  **Response:** Yes, that risk is real. Use the tutor's reading texts and short AI stories built only from taught words, never graded, with comprehension questions graded only when the answer is a span copied from the story.

### Tensions Involved In
- Build now or freeze first: argued for the 14-day freeze. Accepted Stage 1 trust fixes (mostly deletions and reversals) in place of a freeze, with the tutor-material audit as the gate before any import is built.
- The process itself: insisted the guardrails are part of the decision: one initiative in flight, a line on what Brian will see differently, a rule budget, delete first, and nobody grades their own work.
- Where Brian really is: the cold check of about 40 words reseeds the model.

### Synergies Identified
- Never count an answer the app can't prove wrong: shared with QA and Data.
- Fix listening: shared with Data and the Architect.

### Round 4 Challenge (summary)
1. The tutor import rests on material nobody has loaded.
2. This is the 38th initiative, run through the same process.
3. Retaining class material may not produce conversation.
4. A fifth redefinition of "known" is still a redefinition.

The group answered:
1. An agent reads Drive, behind a measured audit gate with a fallback.
2. The work is split into two stages, with guardrails.
3. A monthly normal-speed listening check is the outside measure; if it is flat for two months, the plan is wrong.
4. One final, visible change, then frozen for 90 days.

---

## Phase 5: Consensus Position

### Option Scores
| Dimension | A: Patch in place | B: Staged | C: Re-center now | D: Freeze and go simpler |
|-----------|---|---|---|---|
| Learning value | 1 | 4 | 2 | 2 |
| Trust and content correctness | 1 | 4 | 2 | 3 |
| Effort and time to value | 2 | 3 | 1 | 3 |
| Treadmill risk (higher = lower risk) | 1 | 3 | 2 | 4 |
| Preserves progress and history | 3 | 4 | 2 | 5 |
| Reversibility | 2 | 4 | 2 | 5 |
| **Total** | **10** | **22** | **11** | **22** |

### Final Position
**Preferred Option:** B (staged), with Option D written down now as the fallback.

**Rationale:** The dialogue answered most of my case for a freeze. My tutor-material audit became the gate, my cold check now reseeds the model, an outside listening probe measures the goal, and Stage 1 is mostly switches and deletions, which is roughly what my freeze allowed anyway. B and D tie on my totals only because doing nothing always gets top marks for reversibility and preservation. On the two dimensions that matter most, trust (Brian's binding constraint) and learning (his goal), B wins 8 to 5, because D leaves listening switched off and Progress telling him he knows nothing for two more weeks. I also accept the Data Analyst's point that 14 days with one learner is too short to be a real experiment. My freeze was mainly meant to stop the loop, and B can do that if its guardrails are enforced.

**Conditions:**
1. **Stage 1 is a closed list.** It holds the seven items in Option B, the tutor-material audit, the cold check of about 40 words, and the baseline listening probe. Only switches, deletions, one-time data jobs, word clips, and the one Progress change belong there. Refactors (for example "one question engine"), the robot run-through, and any new feature wait for Stage 2. The robot run-through has nothing to check until Stage 2 produces class packs anyway. New logging goes in only where a written pass or fail line needs it.
2. **The guardrails get a mechanism, not just a paragraph.** Written rules in this repo have been skipped before: the class-data runbook gate was ignored, and INIT-026 was marked complete while it was undeployed. Brian gives an explicit go to open each initiative. Only one initiative is in flight at a time. Complaints go into one list that he reviews once a week, after a 24-hour cooling-off. This needs Brian's agreement, because telling the AI "fix this, fix that" on the same day is half of the loop.
3. **The numbers move once, not three times.** Voiding, the new "proved" ladder, and the cold-check seeding land together as one announced change, with old and new numbers side by side. Seeded words set the level of practice but do not count as "proved"; otherwise the ladder is not frozen even on day one. Until class words are recorded, the count says plainly "out of words the app has taught you". Class words arrive later as their own row, not as another redefinition.
4. **No AI-written sentence is ever graded, so no judge model gets built.** On this I side with the Technical Architect over my own Phase 4 position. No AI adds vowel points to a graded answer key. Tutor lines are graded as the tutor wrote them: compared without vowel points, with both full and short spellings accepted.
5. **Typing a tutor sentence from its English is not correct by construction.** Hebrew often allows several right answers: different word order, a dropped pronoun, or a synonym. Putting words in order has a smaller version of the same problem. Grade only the target words inside the sentence, or start Stage 2 with dictation and "which line did you hear". Add whole-sentence typing only after the "my answer was also right" tap shows it rarely misfires.
6. **Brian's own verdict is part of Stage 1's exit.** Next to his minutes, he answers one line: "Do I trust Today now? Yes or no." An agent that did not build Stage 1 can check the numbers, but Brian's answer is the acceptance test, because trust is the thing being fixed.
7. **D is the fallback, with a date.** If his minutes have not moved by week 4 (the Product Strategist's test), stop building. The app becomes review-only, and tutor material moves to Anki or the worksheet player.

**Dissent:** Partial, and aimed at the package rather than the option.
- Stage 1 is described as small, but the other four specialists' conditions add about ten pieces of work to it. They include a refactor into one question engine, a generated rules guide with its own test, a local Docker check, re-checking the 11 finished units, a tested true undo, a capped session, honest "due" and pace gates, replay and speed logging, a held-out probe, and an independent exit check. That is the complaint-to-rule treadmill reproducing inside the plan meant to stop it. For that reason I score B 3, not 4, on effort and on treadmill risk. Held to the closed list, both would be 4.
- I concede the freeze. The dialogue answered almost everything behind it, with one exception nobody addressed: guardrails written in a brainstorm are no stronger than the runbook gate that was ignored. Condition 2 is my answer to that.
- The equal-weight rubric leans toward inaction, which is why D ties B on my scores. I do not treat the tie as a reason to prefer D.

**Abandon the consensus if:** Stage 1 is not finished within 21 days, or a second initiative opens before Stage 1 closes. Either one shows the process still cannot do something small, and Stage 2 would just be the next lap of the same treadmill. In that case, stop building: the app becomes review-only, and tutor material moves to Anki or the worksheet player (Option D).
