# BRAIN-001 — Session Record

**Topic:** Is the Hebrew app built right, and what could we do better?
**Session type:** Quick Cross-Domain EPIC (Quick depth) · **Date:** 2026-11-09
**Panel (auto-assembled):** Technical Architect (Engineering), QA Advocate (Engineering), Product Strategist (Product & Design, lead), Data Analyst (Product & Design), Devil's Advocate (Security & Risk)
**Setup and Brian's answers:** [session-context.md](session-context.md)

---

## Phase 2 — Inquiry

The five specialists asked 15 questions, which were consolidated to 11. Brian's answers are recorded in `session-context.md`. In short:
- The app is for him alone, and his tutor is his course.
- He wants science-backed retention of the tutor's lessons, with an AI as a supplementary tutor.
- His goal is conversation.
- No human will check content.
- He will give 20 to 30 minutes a day if sessions are worth it.
- Trust is what stops him.
- Progress does not match what he can do.
- Nothing is sacred.

## Phase 3 — Independent analysis (summaries)

| Specialist | Position | Top risk | Confidence |
| --- | --- | --- | --- |
| [Technical Architect](specialists/technical-architect.md) | The core is sound (answer log, FSRS, lexicon, learner model, audio, MCP). The content side is the wrong shape: free-form AI units behind a blacklist validator cannot converge. Rebuild it as "tutor lesson in, practice out", with a checked sentence bank and a session that code assembles each day. | The tutor channel stays empty and the AI goes back to inventing content. (High) | Medium |
| [QA Advocate](specialists/qa-advocate.md) | Gates check the unit file, but buttons are picked at play time by part of speech, and "another button is also right" is a question of meaning no rule settles. Unit 1, lesson 2 has three right buttons. Make grading safe first: an answer the app can't prove wrong records nothing. | False "wrong" grades become lapses and struggle words that drive the next brief and depress Progress, and nothing undoes them. (High) | Medium |
| [Product Strategist](specialists/product-strategist.md) | Right engine, wrong center. The app is a self-contained course for "a learner without a tutor". It should be a companion that helps him keep his tutor's lessons and moves him toward conversation. Core loop: the class comes in, he practises daily, the app shows what stuck, and the AI tunes practice and writes a note for the tutor. | Loading classes has never happened. If it needs regular effort from Brian, it won't happen. (High) | Medium-High on the diagnosis; Medium on the MVP |
| [Data Analyst](specialists/data-analyst.md) | Good evidence is collapsed into a "known" label that needs more asks per word than the app makes (3 recent right, 2 study days, recall at least 0.85), decays with the calendar, and is drawn against all 1,076 lexicon words. Of the 22 words he has spelled, he has never misspelled 21, yet Progress shows 4 known. The same strict counts lock the brief at "beginner, no new words". | Contaminated and shifting measurement erodes trust. (High) | High on the diagnosis; Medium on the thresholds |
| [Devil's Advocate](specialists/devils-advocate.md) | The process (34 initiatives in a month, each complaint becoming a new rule) and a learner model anchored near zero do more damage than any bug. A tutor-centered rebuild through the same process would repeat the treadmill. Freeze for 14 days and run five experiments first. | The redesign runs through the same process and spends the trust Brian named as his binding constraint. (High) | Medium |

**Parent's verification (read in code before the dialogue):**
- **The "known" rule:** `internal/learner/state.go:56-63,157-161` requires recall at least 0.85, the newest 3 answers right, and success on 2 study days.
- **New words:** `internal/brief/build.go` never reads tutor-taught words. Its only "taught" mentions are about grammar patterns.
- **Listening review:** `internal/session/mix.go:263-281` drops due listening cards with no word clip, and they stay due. HasAudio comes from the lexeme's `audio_id` (`internal/mcpstore/lexicon.go:151`).
- **Class data:** `docs/operations/class-data-through-mcp.md:11` still reads "Backfill under way: not yet."
- **Earlier copy rule:** INIT-012 SPEC-008 (line 39) said to "write original lines … never copy a learner's typed answer", because "the class docs hold real errors".

---

## Phase 4 — Structured dialogue

### Round 1: Responses to cross-specialist questions

**Devil's Advocate asked the Technical Architect:** If AI-written units, the reviser, placement and consolidation were deleted, can review plus the learner model plus the MCP read tools stand alone? Is the listening trap real?
**Technical Architect:**
- Yes, they can stand alone. Attempts and exposures are append-only and set `unit_id` to null rather than cascading (`migrations/006_evidence.sql`). Learner state is computed from logs, and review (`internal/session`, `internal/activity`) does not depend on units.
- The listening trap is real in code. Due listening cards whose word has no clip are filtered out and stay due, which is 56 of the 161 due cards (Data Analyst's live count).
- The fix is cheap: generate word clips for every taught word (`generate_audio` takes texts), or never create a listening card without a clip.
- One caution: `lesson_runs` cascades on unit delete, so units must be archived, never deleted.

**QA Advocate asked the Technical Architect:** Can the buttons be chosen at publish time and stored with each revision?
**Technical Architect:** Yes. `lesson_items` is already rebuilt per published revision (INIT-037 added `rebuild-lesson-items`), so a stored choice set per item is a small additive change. The cheaper answer is to stop grading word-choice blanks whose wrong buttons share the target's part of speech. Then there is nothing to freeze.

**Data Analyst asked the QA Advocate:** How many of learner 1's lapses and struggle words came from questions that today's correctness rules flag?
**QA Advocate:** It isn't computed yet, but the current struggle list is already contaminated. גָּר and אוֹמֵר were both last missed at 21:27 UTC on 2026-11-08, inside the cloned lesson 12. The safe step is to void evidence from units 12 to 34 and from word-choice blanks with no English cue, before any unit is archived, because the void needs `unit_id`.
**Data Analyst follow-up:** Agreed. Recompute Progress after voiding, and show the change once so Brian sees why the numbers moved.

**Architect, QA and Data asked the Product Strategist:** Does the tutor produce material after each class that can be handed to an AI in under five minutes?
**Product Strategist:**
- The material exists. INIT-012 was planned from about 85 class docs in a Drive folder (Preply classes with Aaron, five lesson types), and `Aharon's code/` looks like the same course.
- What has never existed is a zero-effort path. This workspace has a Google Drive connection, so an agent can read the newest class doc directly. Brian's whole effort becomes "load today's class", or nothing if a job polls the folder.
- Unknowns: how clean the docs are, how many lines carry vowel points, and which lines are the tutor's and which are Brian's typed answers.

**Devil's Advocate asked the Product Strategist:** Is speaking time with a person the real bottleneck to conversation?
**Product Strategist:** Partly. The app cannot replace the tutor for conversation. Its honest job, in Brian's words, is to make him retain the lessons, plus ear training at normal speed and rehearsal of answers aloud. Its weekly note should point tutor time at his weak spots. Retention makes tutor time count, and conversation itself happens with the tutor.

**Product Strategist asked the QA Advocate:** With no human checker, which checks let class practice reach Today safely?
**QA Advocate:**
1. Grade only formats that are correct by construction from a trusted key: type the tutor's sentence from its English, put the tutor's sentence in order, dictation of a tutor line with audio, and choosing which tutor line you heard.
2. Every graded word must be an exact lexicon form that he was taught.
3. A judge model from a different family than the author flags doubtful items, and any doubt makes the item ungraded.
4. Stop threshold: more than one "my answer was also right" tap per 50 graded items in a week pauses new packs.

**Product Strategist and Devil's Advocate asked the Data Analyst:** Which single number, and can its definition stay frozen?
**Data Analyst:**
- The headline is "class words you can say from English", out of words he was taught. "Proved" means right on 3 study days spanning a week, with the latest answer right, and no calendar decay.
- The second row is "understand by ear". Freeze both for 90 days, with one announced change from the current rule.
**Devil's Advocate follow-up:** Accepted only with an outside measure: a monthly normal-speed listening check, scored by code, on items never practised.

**Data Analyst asked the Product Strategist and Technical Architect:** Should scheduled reading and spelling cards be dropped, with those skills credited from listening and production answers?
**Product Strategist:** Yes for reading. He reads well (32 known), and reading is not the goal. Keep spelling only as the typing inside production.
**Technical Architect:** It's feasible. Skills are per card, and INIT-027's move-up logic already credits a skill from a stronger answer. Stop creating reading and spelling cards for new words, and let existing ones age out. This roughly halves review load per word.

**Technical Architect asked the Data Analyst:** Can one definition be frozen and the whole history rebuilt under it?
**Data Analyst:** Yes. The rule-version rebuild from `review_log` already exists (ADR-0031 D5). Do it once.

**Devil's Advocate asked the QA Advocate:** What single acceptance test, run by Brian, would have caught lesson 12?
**QA Advocate:** He won't check each lesson, since checking is the trust problem itself. Two things would have caught it:
- **A robot run-through before Today.** A bot plays every graded item and flags any item with an untaught button, or with a wrong button that makes a sentence found in trusted text.
- **One line per change in the development process:** "what Brian will see differently", with a screenshot. Brian is the acceptance test for changes, not for lessons.

**Technical Architect asked the Devil's Advocate:** Does narrowing to review plus tutor sentences lose the interesting stories he asked for?
**Devil's Advocate:** Yes, that risk is real, and stories are a stated want. The answer is stories that are never graded. Use the tutor's reading texts (the worksheet has a reading section with reported and direct speech) and short AI-written stories built only from taught words, with comprehension questions graded only when the answer is a span copied from the story.

### Round 2: Tensions

**Tension 1: Build now, or freeze and experiment first**
- **Between:** Technical Architect, Product Strategist and QA Advocate on one side, the Devil's Advocate on the other.
- **Nature:** whether to start the tutor-centered rebuild now.
- **Build now:** the harms are known and the core is solid, so re-center now.
- **Freeze first:** 14 days with no building and five experiments with set pass and fail lines, because the rebuild rests on tutor data nobody has loaded.
- **Resolution path:** Fix the known harms now. These are small, mostly deletions or reversals, and they restore trust rather than add features. In week 1, run the tutor-material audit as the gate before building the import. Fourteen days of nothing would leave his current, untrustworthy app in place.

**Tension 2: Reusing the tutor's text versus the old "write original lines" rule**
- **Between:** Product Strategist and QA Advocate on one side, INIT-012 SPEC-008 and the Devil's Advocate's caution on the other.
- **Nature:** the class docs contain Brian's own typed errors, and tutor-mistake contrasts are what pulled untaught words into lesson 12.
- **Reuse:** the tutor's own lines are the only trusted answer key available without a human checker.
- **Caution:** keep writing original lines, and treat the docs as unreliable.
- **Resolution path:** Split the material by who wrote it. Lines the tutor wrote are trusted keys. Lines Brian typed are never keys; they are used only as the wrong side of a contrast, stored as a structured field, never parsed from " — ". For a personal tool, private reuse of the tutor's text is Brian's call, and the tutor can be asked.

**Tension 3: Fewer graded formats versus more evidence per word**
- **Between:** QA Advocate and Data Analyst.
- **QA:** stop grading word-choice blanks and void unprovable wrongs.
- **Data:** "proved" needs more asks per word.
- **Resolution path:** Production formats (typing, building a sentence, dictation) give stronger and unambiguous evidence per answer. Crediting reading from production answers cuts the asks needed. Evidence quality goes up even as the number of formats goes down.

**Tension 4: How much the AI writes**
- **Between:** Technical Architect and Product Strategist on one side, the QA Advocate and Devil's Advocate on the other.
- **Architect and Product:** the AI proposes sentences and one story, plus at most one practice lesson ahead, built from a class pack.
- **QA and Devil's Advocate:** no agent grades its own work, and AI-proposed graded sentences need an independent judge.
- **Resolution path:** The AI may write, but only items whose answer key is trusted (a tutor line, or an exact lexicon form) are graded. Everything else the AI writes (stories, explanations, coaching, the tutor note) is ungraded. A graded AI sentence must pass a judge from another model family and use only taught words, or it ships ungraded.

**Tension 5: The process itself**
- **Between:** the Devil's Advocate and the Technical Architect, who named it his second-highest risk, versus the momentum of the current spec-driven pipeline.
- **Nature:** 34 initiatives in a month, with one initiative per complaint.
- **Resolution path:** Make the guardrails part of the decision:
  - one initiative in flight at a time;
  - every spec carries a line saying what Brian will see differently;
  - a rule budget, so a new validator rule retires an old one;
  - delete first;
  - assessments and grading are never done by the agent that built the thing.

**Tension 6: Where Brian really is**
- **Between:** the Devil's Advocate (the model is anchored near zero, while his recorded mistakes are A1+/A2 sentences) and the Data Analyst (the measure is too strict).
- **Nature:** whether to reseed the model or just loosen the rule.
- **Resolution path:** Both. Loosen to the frozen "proved" ladder, and run a one-time cold check of about 40 words the app calls unseen or learning. Words he gets right seed as "recalled" with a dated note, so lessons stop treating him as a beginner.

### Round 3: Synergies

1. **Keep the core** (all five specialists): the answer log, FSRS scheduling, the pointed lexicon with verb and noun forms, the audio cache, the learner-model rebuild from logs, coach notes, and the MCP server. None of the defects live there.
2. **The syllabus comes from the tutor** (Architect, Product, QA, Devil's Advocate; Data asks which words count as taught): choosing new words from frequency bands is the wrong center for a learner with a tutor.
3. **Retire AI-written per-learner units, the reviser, and auto-publish** (Architect, QA, Devil's Advocate; Product shrinks them to at most one ahead, built from a class pack).
4. **Never count an answer the app can't prove wrong** (QA, Data, Devil's Advocate): void the contaminated evidence, add one tap for "my answer was also right", and make reporting a lesson void its wrong answers.
5. **Progress must be legible and stable** (Data, Product, Devil's Advocate, Architect): one ladder per skill, counted out of taught words, headed by the conversation skills, and frozen.
6. **Fix listening** (Data, Devil's Advocate, Architect): it is the goal skill and it is quietly switched off. Add word clips, or don't create listening cards without one.
7. **More production practice against trusted keys** (Product, QA, Architect, Data): typing, building sentences, dictation, and saying answers aloud. This is what he asked for ("create a sentence, typing").
8. **One source of truth, and a smaller surface** (Architect, QA, Devil's Advocate): generate the authoring guide and tool descriptions from the rule catalog. Freeze or stop Isaiah, the roots hub, placement, multi-user and teacher features, the stale plan-builder skill, and the sound-contrast queue.

### Round 4: Devil's Advocate challenge

**Emerging consensus:**
- Keep the core engine.
- Replace AI-written units with a tutor-class import that feeds review and practice graded only against trusted keys.
- Make grading safe.
- Rebuild Progress as a frozen ladder.
- Narrow the AI to tutoring.

**Challenges:**
1. **The tutor import rests on material nobody has loaded in a month.** If loading costs Brian anything, it won't happen, and the app drifts back to inventing.
2. **This consensus is itself a large redesign,** launched through the same process that produced 34 initiatives. It would be the 38th.
3. **Hidden assumption:** that retaining class material produces conversation. It may not, because conversation needs hours of listening and speaking with people.
4. **A fifth redefinition of "known"** is still a redefinition. Each one so far has moved his numbers for reasons unrelated to learning.

**Group response:**
1. The import is done by an agent reading the Drive folder directly. The week-1 audit is the gate: if fewer than about 150 usable tutor-written lines, or fewer than 5 of the last 10 classes, can be loaded without Brian's help, the import is replaced by Brian adding words through the existing tools, and practice stays review plus ungraded stories.
2. The work is split. Stage 1 is small, mostly deletions and reversals, and needs no new subsystem. Stage 2 starts only after the audit passes and Stage 1 has run for a week. The process guardrails from Tension 5 apply to both stages.
3. Partly accepted. The app's job is retention, ear training at normal speed, and rehearsing answers aloud. The tutor provides conversation. A monthly normal-speed listening check, scored by code, is the outside measure. If it doesn't rise over two months, the plan is wrong.
4. Accepted with conditions. There will be one final change, shown on Progress with the old and new numbers side by side, then frozen for 90 days and rebuilt from the logs.

**Consensus adjustment:** The recommendation becomes a two-stage plan with a measured gate between the stages, and the process guardrails are part of the decision, not advice.

---

## Phase 5 — Options for scoring

- **Option A: Patch in place.** Keep the current shape: AI-written per-learner units, the validator, the brief, and auto-publish. Fix each complaint as it comes, which is the status quo.
- **Option B: Stabilize, then re-center on the tutor (staged).**
  - **Stage 1, trust fixes, weeks 1 to 2.** All are small and mostly deletions or reversals:
    - void unprovable wrong answers;
    - add one tap for "my answer was also right";
    - add word clips for listening;
    - make Progress an honest, frozen ladder;
    - stop auto-publishing AI units, so Today is review plus safe practice;
    - stop grading word-choice blanks;
    - freeze the side features.
  - **Gate:** the tutor-material audit.
  - **Stage 2.** An agent imports tutor classes from Drive into class packs. Graded practice comes only from trusted keys. The AI acts as tutor (where you are, what to practise, ungraded stories, a weekly note for the tutor). The unit pipeline retires, with units archived and never deleted.
- **Option C: Re-center now, ungated.** Build the tutor import, the sentence bank, and the session assembled by code immediately, and retire the unit pipeline in parallel.
- **Option D: Freeze and go simpler.** Fourteen days with no building. Use Anki, or the worksheets, from tutor sentences alongside app review only. Decide afterwards; the app may end up review-only.

**Scoring dimensions:**
- **Learning value:** does it move him toward conversation?
- **Trust and content correctness:** no false wrong answers, and no untaught or ambiguous items.
- **Effort and time to value.**
- **Treadmill risk:** a lower risk of repeating complaint, initiative, and new rule scores higher.
- **Preserves progress and history.**
- **Reversibility.**

### Per-specialist totals (out of 30)

| Specialist | A: Patch in place | B: Staged | C: Re-center now | D: Freeze and go simpler | Preferred |
| --- | --- | --- | --- | --- | --- |
| Technical Architect | 13 | **26** | 15 | 21 | B |
| QA Advocate | 13 | **24** | 14 | 20 | B |
| Product Strategist | 13 | **25** | 14 | 19 | B |
| Data Analyst | 11 | **25** | 15 | 20 | B |
| Devil's Advocate | 10 | **22** | 11 | **22** | B (D as written fallback) |
| **Panel total (out of 150)** | **60** | **122** | **69** | **102** | **B, 5 of 5** |

### Aggregate matrix (sum of five specialists, out of 25 per cell)

| Dimension | A | B | C | D |
| --- | --- | --- | --- | --- |
| Learning value | 9 | **20** | 14 | 12 |
| Trust and content correctness | 5 | **22** | 12 | 14 |
| Effort and time to value | 10 | **19** | 9 | 15 |
| Treadmill risk (higher = lower risk) | 5 | 19 | 10 | **20** |
| Preserves progress and history | 15 | **21** | 14 | 17 |
| Reversibility | 16 | 21 | 10 | **24** |
| **Total** | **60** | **122** | **69** | **102** |

D wins only on treadmill risk and reversibility, which doing nothing always wins. B wins on learning value and trust by wide margins, and those are the dimensions Brian named.

### Consensus summary

**Strong agreement (all five):**
- Option B.
- Keep the core.
- The syllabus comes from the tutor.
- Retire AI-written units and auto-publish.
- Never count an answer the app can't prove wrong.
- One honest, frozen Progress ladder.
- Turn listening back on.
- Void wrong answers as a flag, before any unit is archived.
- The numbers move once, announced.

**Moderate agreement:**
- **Whether to build a judge model.** The Architect and the Devil's Advocate say none at first: AI-written sentences ship ungraded. QA wanted one. Adopted: no judge in either stage, and AI-written text is never graded.
- **Typing a tutor sentence from its English.** The Devil's Advocate points out that it is not correct by construction, because Hebrew allows other word orders, dropped pronouns, and synonyms. Adopted: grade only the target words, and start Stage 2 with dictation, "which line did you hear", and sentence order.
- **The Stage 2 gate.** The Data Analyst wants it set on counts, not the calendar. Adopted: about 150 graded answers and 50 delayed reviews under the new rules, as well as one week.

**Remaining disagreement:**
- **Scope of Stage 1** (the Devil's Advocate against the conditions from the other four). Resolved by making Stage 1 a closed list. Refactors, the robot run-through, and new logging move to Stage 2.
- **Whether guardrails written in a brainstorm will be enforced** (Devil's Advocate). This is not resolvable on paper. It is handled by Brian's explicit go for each initiative and by the abandon conditions below.

### Consensus recommendation

**Primary recommendation: Option B. Stabilize trust first, then re-center the app on the tutor's lessons, with a measured gate between the stages and the freeze option (D) written down as the fallback.**

**Confidence:** Medium-High. All five agree on the direction and the diagnosis is verified in code and live data. The uncertainty is the tutor material, which the audit tests.

**Stage 1: trust and measurement.** This is one initiative, a closed list, finished within 21 days:
1. Void unprovable wrong answers as a reversible flag with a reason: units 12 to 34, and word-choice blanks with no English cue. Do it before any unit is archived.
2. Add one tap for "my answer was also right", tested as a true undo of card state, stability, due date, and confusions. Reporting a lesson voids its wrong answers.
3. Stop grading word-choice blanks everywhere, including review and the 11 finished units. Lessons use review's stricter gloss-overlap check for wrong buttons.
4. Turn listening on: a word clip for every taught word, and no listening card is ever created without a clip.
5. Make "due" and pacing honest:
   - listening cards with no clip leave "due";
   - same-day learning steps fall due on the next study day;
   - replace the 40-learning-words, 75% Good-or-Easy, and all-skills tier gates with a review-load gate and a later-day production rate.
6. One frozen "proved" ladder per skill, counted out of words taught. "Proved" means right on 3 study days spanning a week, with the latest answer right, and no calendar decay. The headline rows are "Understand by ear" and "Say it from English". It is shown once beside the old numbers, then frozen for 90 days and rebuilt from the logs.
7. Stop auto-publishing AI-written units. Today becomes a capped 20 to 30 minute session weighted toward production. Freeze the side features (Isaiah, roots, placement, multi-user and teacher features, the sound-contrast queue) behind switches, without deleting them. Delete the stale `hebrew-worksheet` skill.

**Measurement items that run alongside Stage 1, with no new subsystem:**
- **Day-0 baseline:** under the new rule, plus a held-out probe of about 20 normal-speed listening items and 10 English prompts, never practised and scored by code.
- **Cold check:** about 40 words the app calls unseen or learning. Passes seed his practice level as "recalled", never as "proved".
- **Tutor-material audit:** an agent reads the Drive folder. Brian does nothing.
- **Pass and fail lines are written down before Stage 1 ships.**

**Stage 1 exit**, checked by an agent that did not build it:
- zero graded steps on Today fail a correctness rule;
- at most one "also right" tap per 50 graded items in a week;
- Brian answers yes to "Do I trust Today now?".

**Gate to Stage 2:** all of these must hold.
- The Stage 1 exit is met.
- The audit passes: about 150 or more usable tutor-written lines, and 5 of the last 10 classes loadable without Brian. Both are required, matching the Round 4 wording.
- Brian decides whether the tutor's text may be reused privately.
- About 150 graded answers and 50 delayed reviews exist under the new rules.

**Stage 2: re-center on the tutor.** This is a separate initiative, opened only after the gate:
- An agent imports each class from Drive into a class pack. The tutor's lines and Brian's typed lines are stored in separate fields, never split on punctuation.
- Graded practice uses trusted keys only. It starts with dictation, "which line did you hear", and sentence order. Typed production is graded on the target words only, compared without vowel points, accepting both full and short spellings.
- The tutor's reading texts and AI stories built from taught words are never graded. Comprehension questions are graded only when the answer is a span of the story.
- The AI acts as tutor: where you are, what to practise next, and a weekly note for the tutor.
- One question engine.
- Generate the authoring docs from the rule catalog.
- Log replays, playback speed, and the number of options.
- Retire the unit pipeline after two to three weeks of use. Archive the units, never delete them.

**Process guardrails** (part of the decision, not advice):
- One initiative in flight at a time, each with Brian's explicit go.
- Every spec has a line saying what Brian will see differently.
- Complaints go on one list Brian reviews weekly, after a 24-hour cooling-off.
- A rule budget: a new rule retires an old one.
- Delete first.
- No agent assesses its own work.
- A one-command local Docker check runs before every push.
- Generated code is never edited by hand.

**Fallback (Option D) and abandon conditions:** Stop building, make the app review-only, and move tutor material to Anki or the worksheet player if any of these happens:
- Stage 1 is not finished within 21 days;
- a second initiative opens before Stage 1 closes;
- Brian's study time has not reached about 120 minutes a week by week 4;
- the monthly listening probe is flat for two months.

**Success measures:**
- Primary:
  - Study minutes: 64 a week now, at least 120 by week 4.
  - The share of words taught 14 or more days ago that are proved from English to Hebrew.
  - The monthly held-out normal-speed probe.
- Guardrails: study days per week, lesson abandon rate, and the "also right" tap rate.

**Accepted trade-offs:**
- **Fewer kinds of graded questions for a while.** Accepted because one unfair "wrong" costs more trust than variety earns.
- **Progress numbers move once more.** Accepted because they are shown side by side and then frozen for 90 days.
- **No new lessons until Stage 2.** Accepted because Today becomes a real session of review and production, and the brief already says "review first".

**Dissenting views (documented):**
- **Devil's Advocate:** the conditions from the other four specialists are already growing Stage 1 into the same treadmill, and guardrails written in a brainstorm are no stronger than the runbook gate that was ignored. Held to the closed list, B is fine.
- **QA Advocate (reservation):** word clips put a large batch of unchecked text-to-speech audio into the goal skill. Add a speech-to-text round-trip check if it is cheap; if not, ship the clips anyway.
- **Data Analyst (adopted):** set the Stage 2 gate on counts. A 14-day freeze is too short to be a valid experiment with one learner.
- **Technical Architect (adopted):** no judge model at first.
- **Product Strategist (adopted):** Stage 1 must give Brian something visible in week 1: listening back, honest Progress, and a capped session.
