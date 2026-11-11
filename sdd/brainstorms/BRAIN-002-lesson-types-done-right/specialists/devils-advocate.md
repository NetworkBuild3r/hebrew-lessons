# Devil's Advocate — Brainstorm Analysis

**Session:** Lesson types done right
**Date:** 2026-11-10
**Domain:** Security & Risk
**Primary Lens:** Challenge assumptions and expose blind spots. Ask what Brian could do tomorrow with what already exists, before any new lesson type is built.

---

## Phase 3: Independent Analysis

### Chain of Thought

#### Initial Assessment

The design in `docs/design/lesson-architecture.md` is careful. It is built on research, it respects the trust rules, and it fixes real complaints. I am not saying it is wrong.

I am saying it answers the wrong first question. The design asks "how should the five types look?" Brian's own words ask something else: "I want to know Hebrew better. However we achieve that." (`sdd/brainstorms/BRAIN-002-lesson-types-done-right/session-context.md`, Phase 2 answers.)

Those two questions are different. The second one allows an answer with no new lesson type at all.

Four facts make me uneasy:

- Brian's goal is spoken conversation. The app cannot host speaking, and listening is on hold with no accepted voice. Of the five types, the ones that can ship are mostly reading and typing.
- 742 of 974 class tokens have no lexicon entry (`SPEC-012-audit.md`, quoted in `session-context.md`). Every lesson type needs lexicon entries. The types sit on top of a gap that is not yet filled.
- BRAIN-001 found 34 initiatives and 258 commits in a month produced about three hours of study (`sdd/brainstorms/BRAIN-001-is-the-hebrew-app-built-right/decision-document.md`, Key Evidence). The cure was a guardrail: one initiative at a time. Five types is a lot of surface for one initiative.
- The tutor never touches the app. So class material reaches the app only when an agent imports it. That is a new weekly chore with no owner.

#### Steelman: the best case for the plan

I will argue for the plan first, as strongly as I can.

1. **It builds on what Brian actually asked for.** He wants a tool to "relearn our lessons" (`decision-document.md`, Problem Statement). Lessons built from the tutor's class packs are exactly that.
2. **Code assembly kills the worst failure.** The trust failures were AI-written buttons and clones (ASMT-004, in BRAIN-001). If code builds every card from the lexicon, the stored verb forms, and the tutor's own lines, a bad button cannot come from an author's mistake. Brian also said "let's do as much as we can programmatically" (`session-context.md`). This plan does that.
3. **The word lesson order is sound.** Grid, meaning check, story, write it, use it. The grid is ungraded, so no surprise words. The meaning check is a real retrieval step. Four new words matches classroom practice (`lesson-architecture.md`, "How many new words").
4. **The AI is boxed in.** It writes one ungraded story. It cannot grade, so it cannot cause a false wrong answer (ADR-0033 D3).
5. **Labels help.** A lesson-type label tells Brian what he is doing and why. That also gives the brief a way to vary the pace.
6. **The audit passed.** 286 usable tutor lines and 10 of 10 classes loadable is a strong result against a gate of 150 lines and 5 classes. The raw material exists.

That is a good plan. Now the cracks.

#### Main Considerations

**1. Are five types needed? Probably not. One and a half are.**

I walked each type against what is blocked and what is missing (`lesson-architecture.md`; "What the code has today" in `session-context.md`):

| Type | Buildable now? | What it adds that review or the word lesson does not |
| --- | --- | --- |
| Word | Yes, after the grid and typed produce are built | The only type that teaches new words. Needed. |
| Easy | Mostly yes. The consolidate path "is the easy lesson". | A longer passage plus the usual questions. This is word-lesson steps 3 and 5 without new words. |
| Spelling | Not yet. The recorded letters check is missing. | Typing known words with the Hebrew hidden. Review already does this with `type_it` and `GradeTyped`. It is review with a different label. |
| Conjugation | Not yet. The forms screen is missing. | A real gap, because only a lesson teaches the other forms of a verb. But it only matters once Brian knows verbs from a word lesson. |
| Listening | No. Blocked until Brian names a voice he accepts (`sdd/complaints.md` line 11). | The goal skill, but unbuildable now. |

So: one type is needed (word). One is cheap (easy). One duplicates review (spelling). One is real but later (conjugation). One is blocked (listening). Calling this "five types" makes the first build look five times bigger than the first use. I would plan one type and let use decide the rest.

There is a second cost. RSCH-005 found no effect of keyboard practice on vocabulary (`lesson-architecture.md`, Spelling lesson). The design admits this and still builds a whole lesson type around typing.

**2. Does building beat studying with what exists? For the first two weeks, I think studying wins.**

Today the app already has:

- a single-word `recall` lesson, with English buttons from lexicon glosses;
- a story with tap-for-meaning;
- typed review with letter-level grading;
- stored verb forms;
- the MCP tool `record_taught_words` (listed in `internal/mcp/policy.go` and tested in `internal/mcp/tool_taught_words_test.go`).

Brian can start tomorrow: pick the last two or three classes, add their target words to the lexicon, record them as taught, and let review and the existing story run. That tests the thing the new types depend on: do class words, in the existing screens, hold up on a later day?

I did not verify how `record_taught_words` behaves for a word that has no lexicon entry. If it needs the entry, that proves point 4 below: the lexicon is the gate, not the lesson type.

The plan's own success measure (S2, 50% of words taught 14 or more days ago proved by 2027-01-04, from `session-context.md`) is a measure of retention of taught words. It does not need a new lesson type. It needs taught words. The sooner words are recorded as taught, the sooner S2 can move.

**3. Does the AI-written story earn its place? Not yet. It is the least measurable and the riskiest part.**

The story is never graded (ADR-0033 D3). So it produces no evidence. Brian cannot tell from any number whether stories help. A one-learner test of "with story versus without" needs far more words than he has (the Data Analyst's figure in BRAIN-001: about 120 words per group over 8 to 10 weeks).

It is also the one place where unreviewed AI Hebrew reaches him. He has no human reviewer ("I have you and Grok", `session-context.md`). Coverage of 90 to 95% known words is a Low-grade figure (RSCH-004). With only a few hundred known words, hitting that coverage forces very short stories.

And it adds a dependency. The story comes from an outside AI on the MCP. If Brian does not run an agent, the slot is empty. The question "story missing or failing" was delegated to the panel and is not answered yet.

There is a cheaper source of context: the tutor's own lines. 286 real sentences from his own classes, in the setting he learned them. They need no AI, they carry no invented Hebrew, and he will recognize them. Of 350 tutor lines, only 158 are pointed (`SPEC-012-audit.md`). That limits them, but it is a smaller problem than unreviewed AI pointing.

My challenge: fill the story slot with tutor lines first. Use an AI story only where tutor lines run out, and measure whether anyone misses it.

**4. Will code-built lessons recreate the sameness complaint? Likely yes, for the screens.**

Brian said sameness came from both the screens and the content (`session-context.md`, Phase 2 table). The plan treats this as solved by type variety plus tutor content. I do not think that holds.

- The design says "Most sittings are a word lesson" (`lesson-architecture.md`, opening). Most sittings will run the same five steps in the same order: grid, check, story, write, use. That is the original complaint, built on purpose.
- Code assembly is a template. A template is sameness with a guarantee. The earlier fix for sameness (INIT-030, `unit.same_shape`) produced 23 cloned units (BRAIN-001, "Fixes cause the next complaint"). A rule cannot make lessons feel different. Variety needs a reason to differ.
- The five types give five labels, not five experiences, because four of them are rare by design. The brief picks them only "when a pattern is due, listening is the weak skill..." and so on.
- Content variety depends on the class. A class on, say, a new topic gives new words. But a class pack gives 4 words per lesson from one topic for several lessons in a row. That is the same-topic repetition he already named.

I am not sure the fix is anywhere in the design. It may be a variety rule inside the word lesson (which steps appear, in what order, by word) or simply less lesson and more review. Either way, it is the unresolved risk, and the design does not name it.

**5. Is the lexicon backfill the real project? Yes.**

Count what depends on the lexicon: the grid (pointed form, part of speech, binyan, meaning), the meaning check (English buttons from glosses), the write-it step, review cards, the coverage check on stories, and the answer keys. The plan lists "the lexicon" as one input among several. It is the floor under all of them.

742 of 974 class tokens have no entry. Some are prefixed or inflected forms of words that do have an entry, so the true gap is smaller, but nobody has counted it (`session-context.md`, facts found). Until someone counts the real number of missing dictionary words, the plan's cost is unknown.

Two more problems sit inside the backfill:

- **Correctness.** Entries are agent-written. BRAIN-001 flagged that "how many entries are verified is still unknown" (`decision-document.md`, Appendix C). With no tutor review, a wrong gloss or wrong pointing in a new entry becomes a wrong answer key and a false wrong answer. That is the original trust failure again.
- **Pointing.** Less than half the tutor lines are pointed. Pointing the rest by machine is unverified Hebrew.

If the backfill is the real project, it should be planned, sized, and checked as one. Today it is hidden inside P-01 to P-13.

**6. The plan does not move the goal skills.**

Brian's goal is to understand normal-speed Hebrew and answer (`session-context.md`, The learner). Listening is blocked. Speaking happens outside the app. The measures S3 (a held-out probe of 20 listening items) also needs audio.

So the stage can close with the main goal skill neither taught nor measured in the app. BRAIN-001 made "monthly listening probe flat for two months" an abandon trigger. If there is no accepted voice, the probe cannot run, and that trigger can never fire. This is a quiet gap in the safety net. I suggest someone confirm whether the probe has an audio source.

There is also an opportunity the design does not use. Speaking practice happens with Brian's own AI. The app knows which words he has proved and which he struggles with. A short "bring this to your AI" list (words taught this week, words proved, words that keep failing) might do more for the goal than a spelling lesson. The design says "the app may prep him for it" (`session-context.md`) but then lists nothing that does.

**7. The calendar.**

The stage must close by 2026-11-30, with one initiative at a time (ADR-0033 D13; abandon line A2). Brian wants to be studying with something new within about two weeks of the next stage opening.

The Stage 2 gate also needs 150 graded answers and 50 delayed reviews under the new rules, and a week to pass (`decision-document.md`, Gate to Stage 2). It also needs Brian's ruling on private reuse of the tutor's text. I could not find that ruling in the files I read. It may exist elsewhere. If it does not, it is the real critical path, and it costs Brian one decision, not any code.

Two weeks is not short for a code agent. The history shows agents land about one initiative a day. It is short for verification. The risk is not "can we build it". It is "can we build it and still trust it". The 13-spec provisional list (P-01 to P-13) in the BRAIN-001 `spec-skeleton.md` §5 is the shape that produced the treadmill.

#### What failure looks like

I would call the plan failed if any of these happen:

1. Two weeks after Stage 2 opens, Brian is not studying with something new. This is the user's own deadline.
2. Weekly study minutes are under 120 by week 4 (S1; also an abandon condition in `decision-document.md`).
3. Brian says "these lessons are all the same" again. He has said it twice already (BRAIN-001, Fixes cause the next complaint).
4. The "I was right" button is tapped more than about once per 50 graded items. That is the stop line already set in BRAIN-001, and it measures key quality.
5. A lesson fails because the story slot was empty, or the class pack was not imported that week.
6. The lexicon backfill is still the open item when the first lesson ships, so lessons use only the few words that already have entries.

#### Pre-agreed cut rule (proposed)

Write this down before building. Do not move it later.

- **Start with one type.** Build only the word lesson, plus the label. Everything else waits.
- **A type earns its build.** Another type is built only if, after 10 study days with the word lesson, Brian names a specific need that review or the word lesson cannot meet. A guess from a design document does not count.
- **A type is cut if** it has run for 3 weeks and it is under 15% of Brian's sittings, or Brian skips or abandons it more than twice in a week, or it needs a rule that has no retire-an-old-rule partner.
- **The story is cut if** it is missing in more than 1 of 5 lessons, or if tutor lines alone give Brian the same later-day results over the same weeks.
- **The whole stage is paused if** week 2 of use shows under 60 minutes of study. Fall back to review only, as BRAIN-001 already allows.
- **No new type opens while a lexicon backfill batch is failing its checks.**

#### The cheapest test that could prove the plan wrong in a week

Run this before any Stage 2 build, using only existing tools:

1. Take the last 2 classes. Pick the 15 to 20 target words the tutor actually taught.
2. Add the missing ones to the lexicon, and record them all as taught.
3. For 7 days, study only through what exists: review, the `recall` lesson, the tap-for-meaning story.
4. On day 7, count: (a) how many of these words he got right on a later day; (b) how many lexicon entries needed a fix; (c) how many minutes he studied; (d) whether he still wanted a "new" kind of lesson, and what exactly was missing.

What this could prove wrong:

- **If the backfill is slow or error-prone,** the lexicon is the real project, and the types are premature.
- **If words are retained and he studies,** the existing screens are good enough to start, and the first new build should be the smallest thing he named in (d).
- **If he studies less than 60 minutes,** the problem is not the lesson types. It is something else (the habit, the content, the device) and no type will fix it.
- **If he says "same again,"** the sameness is in the content or the habit and the lesson design is not the lever.

It costs about a day of an agent's time and one week of his time. It also gives an early reading on S1 and S2.

One caution. This test breaks the "one initiative at a time" rule if it is run as an initiative. Run it as Brian's own use of existing MCP tools during the Stage 1 close, or as the first step of Stage 2. It must not become spec work. If it needs code, it is already too big.

#### Opportunities

1. **Use the class words now.** The audit passed. Backfilling and recording the words taught in recent classes gives Brian new material this week, with no new lesson type and no Stage 2 build. It also starts the S2 clock sooner.
2. **Make the lexicon backfill a named, measured piece of work.** Count the true missing dictionary words (not tokens), batch them by class, and check each batch with code rules. This turns the biggest hidden cost into a visible one, and it benefits every lesson type, review, and the story check.
3. **Tutor lines as the content engine.** 286 usable lines are more content than a four-word lesson can use. Using them as the passage, the example sentence, and the answer key removes the AI from the graded path and may fix the "same words, same topics" complaint, because the tutor changes topics weekly.
4. **A "bring this to your AI" list.** A one-screen or one-text export for his outside conversation practice, built from words taught and words proved. It serves the goal skill the app cannot host, and it is tiny next to a lesson type.
5. **Let use pick the second type.** Run the word lesson for 10 days, then read the digest. The data (what he skips, what he repeats) will say which type is wanted, if any.

#### Risks and Concerns

| # | Risk | Rating | Why | Mitigation |
| --- | --- | --- | --- | --- |
| 1 | **The plan is sized as five types but the real work is the lexicon backfill, which is unsized.** Brian waits two weeks and gets lessons built on few words. | **High** | 742 of 974 tokens lack entries. The true count is unknown. Every type depends on the lexicon. | Count true missing dictionary words before committing to a scope. Plan the backfill as its own work with its own checks. |
| 2 | **Agent-written lexicon entries and machine pointing become wrong answer keys.** This recreates the trust failure that stops Brian. | **High** | No human reviewer. Entry verification rate is unknown (BRAIN-001, Appendix C). Under half the tutor lines are pointed. | Prefer the tutor's own pointed lines as keys. Limit new entries per batch. Use the "I was right" flag and the 1-in-50 stop line. Sample-check each batch by code and by a second model, never by the builder. |
| 3 | **Code-built lessons feel the same.** The most common lesson runs one fixed five-step flow. | **High** | Brian named screens and content. Templates guarantee sameness. The previous anti-sameness rule created clones. | Name the variety goal as a testable item. Vary steps by word and by sitting inside the word lesson. Watch for "same again" in the weekly complaint list. Treat it as the main failure signal. |
| 4 | **Scope runs back into the treadmill.** Five types is the P-01 to P-13 shape again, under a two-week promise and one-initiative limit. | **High** | 34 initiatives and 258 commits gave about three hours of study. Spec lists grow. | Build one type first. Apply the cut rule above. Tie every spec to "what Brian will see differently". |
| 5 | **The goal skills (listening and speaking) stay out of the app, and the listening probe cannot run without audio.** The stage can pass with the goal untouched. | **Medium** | No accepted voice. S3 needs audio. The abandon trigger for a flat probe can never fire. | Check whether the probe has an audio source. If not, say so now and pick another listening measure. Ship the "bring this to your AI" list as the cheap link to speaking. |
| 6 | **The story slot depends on an agent session, and the class import depends on one too.** Brian is the only operator. The tutor will never use the app. | **Medium** | If a week goes by without an import or a story, the app has nothing new. | Lessons must work with no story. Make the import a one-command routine and track "weeks since last import" as a visible number. |
| 7 | **The ruling on private reuse of the tutor's text is not settled in the files I read.** | **Medium** | It is a Stage 2 gate item (`decision-document.md`). It costs one decision and blocks the whole content plan. | Ask Brian for the ruling now, in this session. |
| 8 | **Spelling and typing get built because they are easy, not because they help.** | **Low** | RSCH-005 found no effect of keyboard practice. Review already types. | Defer the spelling lesson. Reuse review's typed path. |
| 9 | **A label such as "Easy lesson" feels like a demotion or confuses the Today plan.** | **Low** | Trust is what stops Brian. A confusing session plan could cost a sitting. | Test the labels with Brian in words, before any screen is built. |

#### The strongest alternative

**Alternative: "Study first, build one thing."** Call it Option E, to sit beside the B-versus-D options in BRAIN-001.

1. Now, before Stage 2 opens: count the true missing words, backfill the recent classes' targets, record them as taught, and study them in the existing screens for a week or two. This is the cheapest test above.
2. When Stage 2 opens: build only the word lesson, in its smallest form. The grid, the meaning check, tutor lines as the passage, and typed produce. No new types. No AI story.
3. Add a "bring this to your AI" export for speaking practice.
4. Review the data after 10 study days. Then decide the second type, if any.

What E gives up: the full five-type design, an AI story, and the labels. What it keeps: the tutor-first content, code assembly, the trust rules, and a clear first use within two weeks.

E is not "do nothing". It is the same plan, in a different order, with a use test before each build. It also answers Brian's own sentence better: he wants to know Hebrew better, however we get there.

#### Where I could be wrong

- The five types may be cheap to build once the word lesson and the lexicon exist, because they reuse the same pieces. If so, my "one type" cut saves little. I have not sized the code.
- The word lesson may be so close to what exists (recall plus story) that the "study first" test and the first build are almost the same thing.
- Brian may want the five-type design because it is a design he trusts and can see. Wanting a coherent product is a reason. I have weighed it lightly.
- 286 tutor lines may include many near-duplicates or lines Brian cannot use for answer keys. I did not read the audit's line list.
- I did not read the ASMT-006 report itself, only the summary in `session-context.md`.

### Summary

**Position:** The five-type design is good, but it is not the first thing to build. One type (the word lesson) is needed now. The lexicon backfill is the real project and it is not sized. A week of study with the existing screens and class words would tell us more than any build, and it is the cheapest way to prove the plan wrong.

**Recommendation:** Count the real missing words, backfill and record the recent classes' target words, and study them with what exists for a week. Then build only the word lesson, with tutor lines as the passage and a pre-agreed cut rule for every other type and for the AI story.

**Top Risk:** The lexicon backfill is hidden, unsized, and agent-written with no reviewer. It gates every lesson type, and a wrong entry becomes a wrong answer key, which is the trust failure that makes Brian stop.

**Confidence:** Medium. I am confident the backfill is the gate and that five types is too wide for a first build. I am less sure how much code the types cost, because I did not size it, and I have not seen the audit's line list.

### Questions for Other Specialists

1. **To Product Strategist:** Brian said sameness came from screens and from content. Which exact feature in this plan changes the screens inside the word lesson, since the word lesson will be most sittings? If the answer is "none", what does the plan do about the complaint he named first?
2. **To Product Strategist:** The goal is spoken conversation, and speaking happens in his own AI. Is a "bring this to your AI" export in scope, and would you rank it above the spelling or conjugation lesson?
3. **To UX Designer:** Can you design the word lesson so its steps change by word and by sitting (for example, skip the grid for a word he got right twice, reorder the steps), and still keep "no surprise words"? Show me the variety is real and not just a label.
4. **To UX Designer:** What does Brian see in the first 60 seconds of a word lesson when the story slot is empty or fails? That is the fallback case the architect has not answered yet.
5. **To Technical Architect:** How many true dictionary words are missing from the lexicon once prefixes and inflected forms are removed from the 742? Can `record_taught_words` record a word that has no lexicon entry? What does that say about the order of work?
6. **To Technical Architect:** Which of the five types reuse the word-lesson pieces so cheaply that building them early costs little, and which need their own screens? Size it in days, not specs.
7. **To Data Analyst:** Can the plan be tested in a week with one learner? What can a 7-day study run show about retention, and what can it never show? Is there any audio source for the S3 listening probe while no voice is accepted?
8. **To Data Analyst:** Is the cut rule I proposed (10 study days, 15% of sittings, a missing-story rate of 1 in 5) measurable with the logs that exist now? If not, which one number would you use per lesson type?
9. **To QA Advocate:** Who checks agent-written lexicon entries and machine pointing when there is no human reviewer? Give a concrete check that a builder cannot pass by grading its own work, and say how many entries per batch you would allow before it must pass.
10. **To QA Advocate:** What is the stop line for the lexicon backfill itself, the same way "also right" taps have 1 in 50?

---

## Phase 4: Dialogue Contributions

### Questions Posed

1. **To Product Strategist:** Brian named both the screens and the content as sources of sameness. Which feature changes the screens inside the word lesson? If none, what does the plan do about the complaint he named first?
2. **To Product Strategist:** Is the "bring this to your AI" export in scope, and would you rank it above the spelling or conjugation lesson?
3. **To UX Designer:** Can the word lesson's steps change by word and by sitting while keeping "no surprise words"? Show that the variety is real.
4. **To UX Designer:** What does Brian see in the first 60 seconds when the story slot is empty or fails?
5. **To Technical Architect:** How many true dictionary words are missing once prefixes and inflected forms are removed from the 742? Can `record_taught_words` record a word that has no lexicon entry?
6. **To Technical Architect:** Which of the five types reuse the word-lesson pieces cheaply? Size each in days.
7. **To Data Analyst:** Can the plan be tested in a week with one learner? Is there any audio source for the S3 listening probe while no voice is accepted?
8. **To Data Analyst:** Is the cut rule (10 study days, under 15% of sittings, story missing in more than 1 of 5 lessons) measurable with the logs that exist now? Which one number would you use per lesson type?
9. **To QA Advocate:** Who checks agent-written lexicon entries and machine pointing when there is no human reviewer? Give a check a builder cannot pass by grading its own work, and say how many entries per batch are allowed.
10. **To QA Advocate:** What is the stop line for the lexicon backfill itself?

### Questions Answered

My only explicit reply in `dialogue.md` was to the UX Designer's question about whether the first version is the word lesson plus the easy lesson. I said that is two builds before any evidence, and the word lesson alone covers most sittings.

Answers my questions received in the dialogue:

- **Q2 (Product Strategist):** Yes, the export is cheap and fits. No ranking was given.
- **Q5 (Technical Architect):** Nobody has counted. A taught word needs a lexicon entry, so recording one with no entry would show a card with nothing behind it.
- **Q8 (Data Analyst):** Partly. A retire rule judged at day 14 and day 28, with the flag rate as the quality signal.
- **Q9 (QA Advocate):** Yes. The T0, T1, and T2 tiers, with the tutor's own text as the check.
- **Q10 (QA Advocate):** Partly. QA gave a three-test release bar, not a stop line for the backfill itself.

Not answered in the dialogue: Q1, Q3, Q4, Q6. Q7 was not answered directly; the panel's working answer is a no-audio design for now.

### Tensions Involved In

- **Study first, or build first:** Study first. Backfill and studying are data entry and use, not specs. Build waits until after the study week.
- **How many lesson types in the first release:** One type, the word lesson. The easy lesson is added only if heavy-review days appear. Conjugation is third. Spelling is cut.
- **Does the AI story earn its place:** Not in the first slice. Tutor lines are the passage. If an AI story ships at all, it is optional and measured, and it is cut if it is missing in more than 1 of 5 lessons.
- **How much measurement:** I accept the middle ground: a type label, the flag rate, and a retire rule agreed in advance. I would not track more per type than the cut rule needs.
- **Who certifies the lexicon:** The first release grades only T0 words. I would not plan around a T1 second source, because no word list with a usable licence has been found.
- **Review cap inside Today:** No stance in my analysis. I accept it with one caution. A cap that holds back new words is the one place the plan could slow study, so it needs a measured count of review minutes and the easy-lesson fallback.

### Synergies Identified

- **Lexicon backfill as the critical path:** All six specialists, unanimous. It was already my top risk.
- **Typed-spelling accept-sets:** UX Designer, Technical Architect, and QA Advocate each found the extra-ו/י problem. It is also a bug in review today (`internal/fsrs/typed.go`), so the fix pays off even if the word lesson never ships.
- **Flag control extending `also_right.go`:** QA Advocate (suspend the item), Technical Architect (report acts on one item), and Data Analyst (flag rate as the quality signal).
- **Tutor lines as the passage (G3):** Product Strategist and Technical Architect. It removes the AI-story dependency and ties each lesson to its class topic.
- **"Bring this to your AI" page:** Product Strategist. It serves the speaking goal the app cannot host.
- **Retire rule written before the first build:** Data Analyst. Rules agreed in advance are the guard against the treadmill.

### Round 4 Challenge

1. **"Feels the same" is not answered by fixed stages.** Status: **Open.** The panel calls it a bet with a tripwire: if "same again" appears in week 2, change the content mix, not add a lesson type. I would push on this one. The design does not fix the screens.
2. **Two weeks is not realistic if the backfill is large.** Status: **Accepted as a constraint.** Count first. Over about 100 missing words, the first release teaches T0 words only.
3. **Starting Stage 2 work before the gate is the treadmill again.** Status: **Accepted as a constraint, with a partial concession.** Gate items G2 (audit) and G3 (tutor-text ruling) are met. Stage 1 exit and enough evidence remain. Before the gate, only data entry and studying.
4. **The QA bar is nine gates.** Status: **Accepted.** The release bar is three tests. Everything else is a flag the loop handles.
5. **The retire rule must exist before the first build.** Status: **Accepted.**

---

## Phase 5: Consensus Position

### Option Scores

| Dimension | A | B | C | D |
| --- | --- | --- | --- | --- |
| Learning effect (research-backed) | 3 | 3 | 4 | 4 |
| Trust (no wrong key, no untaught words) | 2 | 2 | 3 | 4 |
| Time to new study (about 2 weeks) | 1 | 2 | 4 | 3 |
| Build cost and treadmill risk | 1 | 2 | 4 | 3 |
| Fixes "feels the same" | 2 | 3 | 2 | 3 |
| Measurable | 2 | 3 | 4 | 3 |
| Total | 11 | 15 | 21 | 20 |

Reasons, one per row:

- **Learning effect:** The word lesson rests on spaced review (RSCH-003, High confidence). Keyboard practice showed no vocabulary effect (RSCH-005). A and B add types with weak evidence (an ungraded AI story, a spelling lesson). C and D stay with the evidence.
- **Trust:** A adds an unreviewed AI story and five new graded types on a 742-word gap, with no backfill step. B has no backfill-first or T0 gate. C backfills first. D backfills with tiers, grades only T0 words, and keeps the story optional.
- **Time to new study:** A cannot start in two weeks. B needs the backfill, the accept-set fix, the flag control, and two types. C gives new words in review within days and needs one build. D needs the flag control, the accept-set fix, and the validator deletion before its first lesson.
- **Build cost and treadmill risk:** A is the P-01 to P-13 shape again. B adds conjugation next and typed write. C is one type with cut rules. D adds the validator deletion, a flag migration, and an accept-set rewrite to slice 1.
- **Fixes "feels the same":** The five-step word lesson is what most sittings will feel like in every option. A's rare types are rare by design. D is the only option that changes the passage source per class, which is a bet. C changes which words, not the lesson's shape or passage source.
- **Measurable:** C is one change after a clean study-week baseline. D changes several things at once (passage source, flag control, new screens), so attribution is weak. A and B measure too many things at once. S3 needs audio for every option, which is a shared gap.

### Final Position

**Preferred Option:** C, with two items from D: tutor lines as the passage, and the "same again" tripwire and retire rule agreed before any build.

**Rationale:** Start with the newest classes' words in the existing screens for a week, after a backfill that is data entry only. That tests what every lesson type depends on (do class words hold on a later day?) with no new code, and it fixes the lexicon gap first. The one build after that is the word lesson alone, because it is the only type that teaches new words and the part with research behind it.

**Dissent:** From D, I dissent on the scope of slice 1. D puts three builds ahead of the first lesson: typed write with accept-sets, the flag control (void, flag, and suspend, with a new migration), and deleting the validator path. Each adds specs and rules, which is the treadmill I warned about.

- The accept-set fix is a bug in review today. Fix it now as a review fix, not as part of a lesson build.
- Typed write inside a lesson adds nothing measurable over review, and RSCH-005 found no vocabulary effect from keyboard practice. Keep it out of slice 1 until the study week shows a need.
- Deleting the validator path for code-built lessons removes the audio and shape checks that protect trust. Keep validation. Do not skip it.
- The flag control is the one piece of D I would keep in slice 1, because it is how the 1-in-50 stop line gets measured. Keep it small by extending `also_right.go`.

D and C score within one point of each other. The disagreement is about scope, not direction.

---
## Information Injection R1: Lessons not tied to a recent class
**Injected at:** After Phase 5, before Phase 6
### Re-Evaluation
**Impact Assessment:** High
**Position Change:** Changed (in part). The one-build, study-first principle is reinforced. The week-one test seed, the heavy-day rule, and the conjugation trigger are changed.

**Analysis:** Brian's constraint breaks two premises of my case: that the newest class drives the lesson, and that a review-only day is acceptable. "Always a lesson" does not add lesson types, but it adds a daily draw on content, so the backfill now scales with days and with the whole archive rather than with three classes. The archive may hold enough words for months of daily lessons, but that is an unverified extrapolation (the audit's 286 usable lines came from 10 sampled classes, out of about 85). It is the tutor's vocabulary, not the goal's, so words beyond the archive need an ungraded path until a T0 or T1 source exists. Brian's "words and forms beyond the classes" also points at conjugation, which needs no new words.

**Prior Recommendation:** Study the newest 2 to 3 classes' words for a week in existing screens, then build the word lesson alone; on heavy-review days show review only.

**Revised Recommendation:** Keep one build and the study-first order, but seed every lesson from the whole archive in rotation (never the newest class), always offer one lesson (1 to 2 new words on heavy days), keep beyond-archive words ungraded, and promote conjugation to the next build, triggered by an empty checked-word queue rather than by a count of heavy-review days.

**New Risks or Opportunities Surfaced:**
- Risk (High): the treadmill moves from lesson types to content. A daily lesson draws on a backlog of checked entries, so the backfill becomes a queue that must stay ahead of the days. It is unsized and has no human reviewer.
- Risk (Medium): review debt. A lesson every day plus a review cap means overdue cards roll over, on exactly the heavy days when forgetting is highest.
- Risk (Medium): "new" is ambiguous. An old-class word he was taught and forgot can feel like review, and a code-picked order can feel arbitrary.
- Risk (High): the archive is the tutor's vocabulary, not the goal's. Normal-speed conversation needs words the tutor never chose, and no T0 or T1 source exists for them yet.
- Opportunity: rotating by class topic across the archive is a concrete variety lever against "same topics," and it is the content-mix change the "same again" tripwire already calls for.
- Opportunity: words from his outside AI conversations are a natural beyond-archive source. Log them as exposure only (ADR-0033 D3: an ungraded item records nothing), and promote them only after a T0 or T1 source exists.
- Opportunity: conjugation gives something new with no new words, using forms already stored. It meets the "always something new" rule better than any extra word type.
- Unverified: the archive count (about 85 class docs, 10 audited, 286 usable lines) is an extrapolation from the sample. Confirm it before any backfill plan is sized.

**Confidence Adjustment:** Decreased. The week-one test and the heavy-day rule were built on the newest-class assumption, which Brian has rejected. The one-build principle is unaffected, so the direction holds.
