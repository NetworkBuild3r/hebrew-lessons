# BRAIN-002 · Phase 4: Structured dialogue

**Inputs:** the six Phase 3 files in `specialists/`, the session context (with the Phase 2 answers, the facts found, and Brian's G3 ruling: tutor text may be reused privately).

**Process note:** the Phase 3 subagents for Data Analyst, QA Advocate, and Devil's Advocate were blocked several times by a usage-policy error, which gave no reason. They ran in the end on Sonnet 5.5, at Brian's request, because of cost. The Data Analyst ran with a reduced reading list: the session context and its persona only. Product Strategist, UX Designer, and Technical Architect ran on the default model.

## Where the panel started

| Specialist | Position in one line | Top risk |
| --- | --- | --- |
| Product Strategist | The newest class's words in one well-built word lesson. The easy lesson as a fallback on heavy-review days. Conjugation next, listening when there is a voice, spelling cut. | The lexicon gap: a wrong gloss in week 1 breaks trust. |
| UX Designer | The same named stages per type, and one answer-and-feedback pattern with a flag button. Review first, then the lesson. No-audio versions are the real design for now. | Typed answers in full spelling get marked wrong. |
| Technical Architect | Keep the unit row and its tables. Replace who writes the lesson and the player. Graded steps run on review's engine. Assemble at Start, and the story is optional. | The lexicon, not the story, is the author of every graded key. |
| Data Analyst | The whole app is judged by S1, S2, and S3. A lesson type is judged only keep or retire, by a rule agreed in advance. Flags are the quality signal. | Reading noise as a verdict. |
| QA Advocate | Grade only answers code can prove. Lexicon tiers T0, T1, and T2. Typed accept-sets, a perfect-student check, and fixed sample lessons. | Typed spelling marks a right answer wrong, or accepts a different word. |
| Devil's Advocate | The five-type design is fine but not the first thing to build. The lexicon backfill is the real project. Study first, build one thing. | An unsized, agent-written backfill with no reviewer gates every type. |

**New facts from the code** (Technical Architect, UX Designer, and QA Advocate; cited at file and line in their files):

- **Today would hide a code-built lesson.** Today skips any untouched learner unit that fails the full validator (`internal/web/shell.go:101-142`). With no accepted voice, `audio.missing` fails every Hebrew step (`validate_audio.go:12-47`).
- **Typed grading would mark the tutor's spelling wrong.**
  - It counts distance 1 as Hard and 2 or more as Again (`internal/fsrs/typed.go`).
  - A full spelling with an extra ו and an extra י grades Again.
  - The lexicon has no column for full spelling.
- **The "My answer was also right" button exists** (`internal/web/also_right.go`). It voids a miss, but it does not flag the item, and it refuses right answers.
- **Report hides the whole lesson** (`shell.go:396-400`). It does not act on one item.
- **Review and lessons pick wrong-answer buttons differently.** Review picks them from the whole lexicon (`internal/db/activity.go:28-40`), and the lesson player picks only from taught words.

## Round 1: Answers to cross-specialist questions

**Devil's Advocate asked the Technical Architect:** how many of the 742 missing tokens are real dictionary words, and can `record_taught_words` record a word with no lexicon entry?
**Technical Architect:** Nobody has counted. Prefixes (ו, ה, ב, ל, מ, ש, כ) and inflected forms account for a large share, but the number is unknown. So the first job is to count, before any screen is built. A taught word needs a lexicon entry, because the grid, the meaning check, and the write step all read the lexicon. Recording a word with no entry would show a card with nothing behind it.

**Devil's Advocate asked the QA Advocate:** who checks agent-written lexicon entries when there is no human reviewer?
**QA Advocate:** The tutor does, without knowing it. An entry is **T0** when the tutor's own document uses the word and an English gloss sits next to it in that document. A T0 entry may be an answer key. A **T1** entry passed every code check and a second independent source agrees with it, so it may also be a key. A **T2** entry is AI-proposed with no second source, so it appears only in story taps and ungraded steps. The AI that builds an entry can't pass this check by grading its own work, because T0 needs the tutor's text and T1 needs a source the AI did not write.

**Product Strategist asked the Technical Architect:** can lessons reuse review's typed input and grade word order without the full one-engine merge?
**Technical Architect:** The typed input, yes. A thin lesson player calls the same `activity.Engine` and `GradeTyped`, so no merge is needed for the first slice. Graded word order, no.
**QA Advocate:** Hebrew word order is often free, so one tutor line is not a single right answer. Order stays an ungraded reveal in the first release.

**UX Designer asked the Product Strategist:** is the first version the word lesson plus the easy lesson?
**Product Strategist:** Yes, with the easy lesson only as the fallback for days when review is heavy.
**Devil's Advocate (reply):** That is two builds before any evidence. The word lesson alone covers most sittings.

**Technical Architect asked the UX Designer:** should the flag buttons appear after every graded answer, or only after a miss?
**UX Designer:** One small control after every graded answer, never a dialog. On a miss it reads "I was right". On a right answer it reads "This question is wrong". It has Undo.
**QA Advocate:** It must void the answer, flag the item, and suspend that item from lessons in one request. A suspended item is never asked again until it is fixed.

**Data Analyst asked the Technical Architect:** does the answer log already carry the run id, the type, and a soft-delete void?
**Technical Architect:** Partly. Checked in the migrations:
- **What attempts carry:** each attempt records its `unit_id` and its `source`, which is lesson, review, practice, placement, or self (`migrations/015_attempt_source_self.sql`). It does not record a run id.
- **What runs carry:** each run in `lesson_runs` records its unit, its lesson index, and its start and finish times (`migrations/001_init.sql:120`).
- **Voids:** voids are separate rows in `answer_voids`, so no attempt is ever deleted (`migrations/023_answer_voids.sql`).
- **The limit on void reasons:** `reason` only accepts `unit_12_34`, `no_cue_blank`, `also_right`, and `report`. So a per-item "flag and suspend" needs a new reason in a small migration.

The type can live on the code-built unit. A run then gets its type through its unit, with no new column on the run.

**Product Strategist asked the Data Analyst:** S1 counts only time spent answering, so do grid and story minutes count?
**Data Analyst:** Count active ungraded time, meaning time with the screen focused and input within the last 60 seconds. Show it separately from graded time. Otherwise a better lesson could push Brian toward the abandon line.

**Data Analyst asked the Product Strategist:** will Brian accept holding out about 1 in 4 class words from lessons, as a comparison group?
**Product Strategist:** Probably not, and he shouldn't have to. His instruction was "whatever helps me learn Hebrew better", and holding back a quarter of his class words works against that. I'd prefer a natural comparison: class words he met only in review against class words he met in a lesson. That is weaker evidence, but it costs him nothing.
**Data Analyst (reply):** Accepted. It is a rough read, labelled as such.

**QA Advocate asked the Product Strategist:** does "taught" include any word that appears in a class document he attended?
**Product Strategist:** For reading, yes. The tutor already put it in front of him, so it isn't a surprise in a passage. For buttons and answer keys, no. Only lexicon words at T0 or T1 that the app has taught count there.

**Devil's Advocate asked the Product Strategist:** would a "bring this to your AI" export serve the speaking goal better than a new lesson type?
**Product Strategist:** Yes, it's cheap and it fits. Brian practises speaking with his own AI. A one-screen "this week's words and the tutor's sentences" page, copyable as text, gets that AI ready. The existing `hebrew://tutor` MCP resource may already cover most of it.

## Round 2: Tensions

### Tension 1: Study first, or build first?
- **Devil's Advocate:** Backfill the recent classes' words, record them as taught, and study them in today's review for a week before building anything. That is the cheapest test of the plan.
- **Product Strategist, Technical Architect, and UX Designer:** Brian wants something new within about two weeks, and old lessons felt the same, so review alone repeats the complaint.
- **Resolution in progress:** These are not really opposed. The backfill blocks every option, so it comes first whichever is chosen. While the word lesson is being built, the backfilled words can be studied in review at no extra cost.
- **Caution:** The open question is whether that week counts as "starting Stage 2" before the gate. A2 forbids a second initiative, so the backfill must run as data entry through the existing MCP tools, with no new spec.

### Tension 2: How many lesson types in the first release
- **Devil's Advocate and the Technical Architect's first slice:** the word lesson only.
- **Product Strategist and UX Designer:** the word lesson plus the easy lesson as the fallback for heavy-review days.
- **Everyone agrees:**
  - Listening waits for an accepted voice.
  - The spelling lesson is cut or deferred. Typed write inside the word lesson already does the spelling work (RSCH-005).
  - Conjugation is third.

### Tension 3: Does the AI story earn its place?
- **Devil's Advocate:** Doubtful. It is AI text that is never graded, and it is the part most likely to have stilted Hebrew.
- **Product Strategist and Technical Architect:** Keep it optional. The lesson never waits for it.
- **New because of G3:** The tutor's own lines that use the new words can be the reading passage. They are real Hebrew, already pointed in about half the lines, and already tied to his class. That makes the AI story an optional extra rather than a dependency.

### Tension 4: How much measurement
- **Data Analyst:** a run row with a type label, a retire rule judged at day 14 and day 28, a flag rate, and a weekly summary of five lines.
- **Product Strategist:** cut P-10, the new logging spec.
- **Devil's Advocate:** at one learner, is measuring each type worth it?
- **Middle ground:** Store the lesson type on the code-built unit. No new tables. One new void reason for the flag. Use the flag rate as the quality signal. Agree the retire rule in advance, as Brian asked. No hold-out group.

### Tension 5: Who certifies the lexicon
- **QA Advocate:** the T0, T1, and T2 tiers. T1 needs a second independent source.
- **Technical Architect:** tie each entry to the tutor's spelling, and stop grading an item the moment it is flagged.
- **Problem:** T1's second source may not exist, because no word list with a usable licence has been identified.
- **Fallback:** The first release grades only T0 words. A word that is not T0 is shown in the grid and the passage, but not quizzed, until it reaches T0 or T1.

### Tension 6: The review cap inside Today
- **Product Strategist:** cap review at 12 to 15 minutes, then the lesson.
- **The research behind this:**
  - RSCH-003 rates spacing as **High** confidence. Review is the spaced test, so it is the highest-value part of the session.
  - RSCH-004 says nothing on new words per hour that has been tested.
- **Resolution in progress:**
  - Review comes first.
  - The cap is soft. Due cards that don't fit roll over to the next day, and the most overdue go first.
  - The lesson is offered only once review is under its cap. A long backlog gets easy-lesson days instead of new words, to stop the snowball.

## Round 3: Synergies

1. **The lexicon backfill is the critical path (unanimous).** Every type, every grid card, and every answer key reads the lexicon. The first work item is to count the true missing words in the newest classes, then backfill them with tiers.
2. **Typed spelling must be fixed before typed write ships.** The UX Designer, Technical Architect, and QA Advocate all found the extra-ו/י problem independently. The fix is:
   - an accept-set of spellings per word;
   - a fold that removes ו and י only where no other taught word folds to the same letters, so בית and בת stay distinct;
   - a fixed test table of about 150 rows.
3. **One flag control that already half exists.** Extend `also_right.go` so it voids, flags, and suspends one item. It should work on right answers too and have Undo. Report is changed to act on one item, not the whole lesson. The flag rate becomes the quality signal.
4. **Tutor lines as the passage (made possible by G3).** This removes the dependency on the AI story, fixes much of the "same content" complaint because each class has its own topic, and ties the lesson to class.
5. **Code-built lessons skip the old validator.** A thin player and a lesson-type label let code-built lessons appear on Today without passing `audio.missing` and the shape rules. Delete those rules for these lessons in the same spec (D13: delete first).
6. **No audio is the real design for now.** Every first-release step works silently. Listen buttons appear only where a clip exists.
7. **Speaking prep, not speaking practice.** A copyable "this week's words and sentences" page feeds Brian's own AI.

## Round 4: Devil's Advocate challenge to the emerging consensus

The emerging consensus is:
1. Backfill the newest classes first.
2. Build the word lesson: grid, meaning check with a second ask, a passage of tutor lines, typed write with accept-sets, and order as an ungraded reveal.
3. Put review first in Today.
4. Easy lesson only if needed, conjugation later, listening when there is a voice, spelling cut.

**Challenges:**

1. **"Feels the same" is not answered by fixed stages.** Brian named both the screens and the content. The consensus fixes the content (class topics, tutor lines) but deliberately keeps the screens fixed (UX).
   - **The counter:** the screens felt the same partly because every lesson was the same unit shape written by an AI. Here the steps change with the class: a different passage each time, a different number of words, and "use it" in a different form. That is still a bet.
   - **Demand:** treat it as a bet. If Brian says "same again" in week 2, the response is to change the content mix, not to add a lesson type.
2. **Two weeks is not realistic if the backfill is large.** If the newest three classes hold 150 or more true missing words, backfill with tiers takes longer than the build.
   - **Demand:** count first. If the count is over about 100, the first release teaches only the T0 words. Those have a gloss in the tutor's own document, so they are cheap.
3. **Starting Stage 2 work before the gate is the treadmill again.** Stage 1 closes by 2026-11-30.
   - **Demand:** before the gate, only data entry (lexicon upserts, `record_taught_words`) and studying. No specs, no code.
   - **Partial concession by the panel:** the gate items are already met. The audit passed (G2), and Brian has ruled on tutor text (G3). Only the Stage 1 exit and enough evidence remain.
4. **The QA bar is nine gates.** That risks becoming the next rulebook.
   - **Demand:** the release bar is three tests:
     1. the perfect-student run passes on the fixed sample lessons;
     2. the typed accept-set table passes;
     3. no graded key is below T0 or T1.
   - Everything else is a flag the loop handles.
5. **The retire rule must exist before the first build, not after.** The panel agrees.

**Status after the challenge:**
- The consensus holds, with challenges 2, 3, and 4 accepted as constraints.
- Challenge 1 stays open as a stated bet with a tripwire.
- Challenge 5 is accepted.

## Options for Phase 5 scoring

- **A. The full design, built in sequence.** All five types, one after another, as in `docs/design/lesson-architecture.md`.
- **B. Word lesson plus easy lesson now.**
  - Code-built: grid, meaning check with a second ask, a tutor-line passage (the AI story optional), and typed write with accept-sets.
  - Review first in Today.
  - Conjugation next. Listening waits for a voice, and spelling is cut.
- **C. Study first, build one thing.**
  - Backfill and record the newest classes' words.
  - Study them for one week in today's review and existing screens.
  - Then build the word lesson only. Every other type needs a trigger agreed in advance.
- **D. Hybrid: backfill and study now, word lesson as the only first slice.**
  - Before the gate: count and backfill the newest 3 classes with tiers (T0 first), record them as taught, and study them in review. This is data entry only, with no code.
  - After the gate, slice 1 is the word lesson with a tutor-line passage, typed write with accept-sets, and the flag control that voids, flags, and suspends one item. Old validator paths for these lessons are deleted.
  - The easy lesson is added only if heavy-review days appear in week 1 or 2.
  - Conjugation comes third.
  - Listening waits for an accepted voice.
  - Spelling is cut.
  - A retire rule and a "same again" tripwire are agreed before slice 1.
