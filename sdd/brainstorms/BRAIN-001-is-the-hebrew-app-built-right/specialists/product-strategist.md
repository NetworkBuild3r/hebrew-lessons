# Product Strategist — Brainstorm Analysis

**Session:** Is the Hebrew app built right? (BRAIN-001)
**Date:** 2026-11-09
**Domain:** Product & Design
**Primary Lens:** User value and outcomes. Every part of the app is judged by one test: does it help Brian keep what his tutor teaches, so that within 6 to 12 months he can hold a simple spoken conversation at normal speed?

---

## Phase 3: Independent Analysis

### Chain of Thought

#### 1. Primary lens

There is one user and one outcome. The user is Brian, an adult learner of modern Hebrew who has a tutor. The outcome he named is a simple spoken conversation: he understands Hebrew at normal speed and can answer. He defined the product himself: "a tool to help me relearn our lessons but actually retain what I learned through science backed research", with "AI as my supplemental tutor: knowing where I am, where I struggle and helping me tune my lessons." His binding constraint is trust, not time. He will give 20 to 30 minutes a day if the sessions are worth it.

So I judge each screen and each pipeline by three questions. Does it carry the tutor's lessons into daily practice? Does it make him retain them, by methods with good evidence? Does it move him toward listening and speaking, and does it show him honestly where he stands?

#### 2. Initial assessment

The engine is roughly right, but the product is centered on the wrong thing. Spaced review, a pointed lexicon with audio, a per-skill learner model and an MCP interface for an AI are the right parts. But the app was designed as a whole course for "a learner without a tutor". It invents its own syllabus from word-frequency bands and teaches it through AI-written units that all share one shape. Meanwhile about 85 of the tutor's class documents have never been loaded, and the tutor data loop that was built for them has not been used once. The shortest path to value is to re-center the app on one loop: the tutor's class comes in, Brian practices it daily, the app shows what stuck, and the AI tunes the practice and hands notes back for the next class. Then cut most of what does not feed that loop.

#### 3. Main considerations

**3.1 The app was built for a learner without a tutor. Brian has one.**

- The design document that set the current direction is titled "The hybrid model" and lives at `docs/design/learner-without-a-tutor.md`. It says the tool "works on its own for someone without a tutor, and alongside a tutor for someone who has one", and it plans a "default path" course. It also says "Do not load class data yet."
- The lesson brief picks new words with no reference to the tutor. `newWords` in `internal/brief/build.go:906-961` takes "the unseen words of the lowest band that has any", in seeded random order, with an optional theme. Words the tutor taught are never the source of a lesson.
- The class backfill runbook (`docs/operations/class-data-through-mcp.md`) still reads "Backfill under way: not yet." The INIT-016 SPEC-004 completion summary says "Nothing was loaded."
- The tutor's material exists and is large. INIT-012 was planned from "Brian's Hebrew lessons with Aaron (Preply)", checked against a Google Drive folder of "about 85 class docs" (INIT-012 initiative, Source line). It lists the tutor's five lesson types: topic conversation, review worksheet, Bible verse, verb workshop, and short story with true or false.
- The early product (INIT-008 to INIT-012, a Node app with a teacher plan builder) was built around the tutor's lessons. The Go port and "v3" (INIT-014, INIT-015) replaced that with AI-built lessons from a learner model. The tutor-centered direction was dropped, not finished.
- The worksheet in `Aharon's code/worksheets/shai-date-indirect-speech/worksheet.js` matches the INIT-012 citation of class doc "Copy of 08: המלצות". The worksheet's title is "המלצות 1 — הדייט של שי". It is labelled level A1+ / A2 and teaches indirect speech. *Assumption (likely, not verified):* "Aharon" is the tutor Aaron, and this worksheet is a hand-built version of one of his classes. Of its 10 vocabulary words, only כְּדַאי and טֶבַע appear among the first 200 of learner 1's 234 cards (from `get_context`; the list was cut off at 200). So the app and the tutor are teaching largely different words.

**3.2 How far the product is from Brian's own definition**

| Brian's words | What exists | Gap |
| --- | --- | --- |
| "relearn our lessons" | No tutor lesson is in the app. The brief draws new words from frequency bands. | Far |
| "actually retain ... science backed" | FSRS spaced review, retrieval before answer, and a retention measure: 80% last week against an 85% target. | Partly there. English-to-Hebrew recall retention is 59%, and 161 cards are due against about 9 minutes a day of study. |
| "knowing where I am" | A detailed learner model, a study digest, and coach notes. | The data exists, but what Brian sees contradicts itself (see 3.5). |
| "where I struggle" | Struggle words, confusion pairs, and a mistakes bank. | Works, but the mistakes are drilled the same way in every unit (see 3.3). |
| "helping me tune my lessons" | The AI tunes lessons it invented. | It should tune practice of *his tutor's* lessons. |
| A simple spoken conversation | `say` and dialogue "play a side" are self-checked, ungraded and unrecorded (INIT-020 notes; `SKILL.md` line 76). Listening is single words and short lines. | Far. Nothing yet trains understanding connected speech at normal speed, or answering a question. |

**3.3 "Every lesson is the same" is designed in, not an accident**

- The author skill prescribes one shape for every lesson (`.cursor/skills/hebrew-lesson-author/SKILL.md` lines 113 to 121): a story, then sentence questions (`listen_pick`, `cloze`, `order`, `say`), then bare-word drills, then a checkpoint.
- In practice the finished units follow an even tighter template. I read units 6 and 11 over `get_unit`. Each lesson opens with a story of 3 to 12 words ("יֵשׁ לִי זְמַן." is the whole first story of unit 11). Then each new word is drilled three times in a row, as `shadow`, `listen_pick` and `recall`. Then come a `pattern`, a sentence `listen_pick`, a `cloze`, a `produce` and a `say`. Two checkpoints repeat that shape.
- Both units end with the same checkpoint, word for word: "I buy, not I to-buy", "two steaks", "I returned the shoes". These are real corrections, but they rotate unchanged from unit to unit (ASMT-004 also found this as the "A A B" mistake rotation).
- The stories are not interesting. In unit 6 the story says "The mouth says. The heart says. I say." Its situation says it was "Built from Isaiah 29:13", which shows the Bible feature leaking into conversation lessons.
- The massed drilling inflates scores inside a lesson. Learner 1's lessons in the last 7 days score 88% on average over 21 lessons. Review over the same 7 days is 72% Good or Easy over 244 reviews, and only 48% over the last 50 reviews (`get_context` `recentAccuracy`). A word asked three times in a minute feels learned. It is not yet remembered the next day. This gap is a big part of why Progress feels like it contradicts the lessons.

**3.4 What Brian asked for, and what exists**

| He asked for | Today | Note |
| --- | --- | --- |
| "create a sentence" | `order` builds a sentence from chips. Nothing asks him to produce a sentence of his own. | The tutor ends most topic lessons with "write 6 to 8 sentences using given words" (INIT-012 section 0b). |
| "typing" | `produce` and review `type_it` type one word only. | His typed practice holds up best on a later day: `type_it` 87% over 31 and `produce` 83% over 42, against `read_pick` 69% over 35 (`practiceResults`). The samples are small, but the direction matches the research on recall practice. |
| "More reading interesting stories" | Stories of 3 to 12 words with no plot. | The tutor's worksheet has a story of about 13 sentences with a plot (Shai's date), six comprehension questions and a "what would you recommend?" question. |
| "Easy to understand what is going on" | Lessons do not say why they exist. Progress shows several numbers that disagree. | See 3.5. |
| No "pick one, where all are right" | INIT-032 and INIT-037 added rules against it. | Multiple choice with made-up wrong answers is fragile by nature. Production graded against a known sentence is safer. |

**3.5 Progress tells him he knows almost nothing, and the numbers disagree**

From learner 1's live model (`get_context`, `model`):

- Spelling is rated **strong**: "97% right over 33 answers in 14 days". Yet spelling shows 3 known and 1 solid. That is exactly his complaint: "I know how to spell words ... and progress says you don't know any words."
- The headline "Words you know" is 36. The strict count of words known in every skill practiced is 2. Placement estimates 502 known words for listening from 30 rated cards. Listening is labelled "not_askable — No audio yet, practiced in lessons only", while listening retention is 96%.
- The page itself (`internal/web/views/progress.templ`) has skill tabs, a stacked bar, two line charts, help and strong cards, a week table, placement history, a weekly report and a "You can now say" list.

The model is rigorous: "known" needs a card, three successes, recall chance of at least 0.85, the last three answers right, and success on two different study days (`internal/learner/state.go:56-74`). But it reports to Brian in the model's terms, not his. He cannot tell where he stands, so the measurement fails at its product job even if it is statistically sound.

**3.6 Much of the app does not serve the loop**

- Placement is 3,670 lines of Go plus 4,208 lines of tests, and 32 of 258 commits mention it. For one learner whose tutor already knows his level, it produced the 502-word listening estimate that contradicts everything else.
- Isaiah verse by verse appears in the menu only when a person has checked a verse (`internal/web/shell.go:263-269`). Brian says nobody will check the Hebrew, so the feature waits for a reviewer who does not exist. The Bible verse is one of the tutor's five lesson types, so it has some claim. But it does not serve the conversation goal, and it has already leaked into conversation units.
- The admin plan builder is gone from the Go code: there is no `create_plan` outside docs. But `.cursor/skills/hebrew-worksheet/SKILL.md` still tells agents to use `create_plan`. That stale skill could mislead an agent.
- Multi-user machinery is still there and still costs effort: auth is 3,230 lines plus 2,267 test lines, and there are "shared" versus "learner" scopes, a general mistakes list, and privacy rules for a shared course (`docs/operations/class-data-through-mcp.md`). Brian said anything built for other learners can go.
- Initiatives 27 to 37 are almost all corrections to review and to the AI lesson pipeline. The build effort has gone into making invented lessons less wrong, not into the loop Brian described.

**3.7 Trust is the binding constraint, and invented content is what breaks it**

Every trust break Brian named comes from Hebrew the AI made up or chose. Examples are wrong answer buttons drawn from untaught words, an authoring note shown in his English, and a blank where several answers are grammatical. With no human checker, the cheapest trust gain is to change where the Hebrew comes from. When the sentences come from his tutor's class, he has seen them before, and he knows what the lesson is for. The AI then mainly selects and schedules, which is where code checks are strong. It writes much less new Hebrew, which is where checks are weak.

#### 4. The core loop and the MVP (questions 1 and 2)

**The loop.** Class in, practice daily, see what stuck, tune and hand back.

1. **Class in.** After each tutor session, the class material becomes a *class pack*: the words taught, with the tutor's example sentences; the key sentences or dialogue; the reading text, if any; the grammar point; and the corrections the tutor made. Brian's effort is at most one step (see section 5).
2. **Practice daily.** Each day's session draws from the class packs. Review comes first, spaced as now. Then he practices the newest class: he listens to its text, answers questions on it, types its sentences and says answers aloud.
3. **See what stuck.** Progress answers questions in Brian's terms, per class. For example: "Class 08: 8 of 10 words recalled on a later day, 4 of 6 sentences typed right, the story understood at normal speed."
4. **Tune and hand back.** The AI reads the digest. It moves weak words and mistakes into tomorrow's session. Once a week it writes a short note for the next tutor class, such as "you keep saying אֲנִי לִקְנוֹת for 'I buy'; ask about כְּדַאי". That is the "supplemental tutor" Brian described.

**The MVP of that loop.** The smallest version that proves it works:

- **In:** a class pack import for new classes and for the last 8 to 10 past classes; cards for every taught word, through the existing `record_taught_words`; tutor corrections, through `record_mistakes`; and one *class practice* built only from the pack's own sentences.
- **In:** one new practice type, "type the sentence". An English cue is shown, he types the Hebrew sentence, and it is graded word by word without requiring vowel points. He asked for typing and sentences, and typed recall is already his strongest practice.
- **In:** one plain Progress view per class pack, plus one honest overall line.
- **In:** Today runs one 20 to 30 minute session with a clear order and a closing summary.
- **Out of the MVP:** AI-invented stories, speech recognition, an AI conversation partner, Isaiah, roots, placement, and any new multi-user work.

**A 25-minute session.** The minutes are an *assumption*: I did not measure seconds per card.

| Minutes | Part | What happens | Why |
| --- | --- | --- | --- |
| 0 to 10 | Review | Due cards, capped at about 70, with class words and struggle words first. More English-to-Hebrew and typing items than now. | Spacing and retrieval have the strongest evidence (RSCH-003, cited in INIT-026). Recall is the weakest skill at 59%. |
| 10 to 13 | Listen | The newest class text or dialogue, at normal speed and then slow, with the text hidden. Then two or three comprehension questions. | His goal is understanding normal-speed speech. |
| 13 to 20 | Produce | Type three to five class sentences from English cues. Build one sentence with `order`. Then write one sentence of his own with a given word, compared with a model answer. | He asked for "create a sentence" and "typing", and the tutor ends topic lessons with this kind of task. |
| 20 to 24 | Speak | Hear one of the tutor's questions (the worksheet's "ask a friend" items), answer aloud, then hear a model answer. Then play one side of the class dialogue. | Practicing answering is the core of conversation. Self-checked for now. |
| 24 to 25 | Close | "Today: 52 of 60 right; 6 class-08 words passed their next-day check; tomorrow about 55 due." | "Easy to understand what is going on." |

A story, when one exists, replaces the Listen part on alternate days. The first stories should be the tutor's own reading passages, such as Shai's date. AI-written graded readers come later, and only from words he has been taught.

#### 5. Getting the tutor's lessons in with the least effort (question 4)

**What exists.** `record_taught_words` takes lexicon ids only. It creates new read, recall and (when there is a clip) listen cards, due at the next 4:00 in his zone. It never grades and never adds sentences, texts or grammar (`internal/evidence/taught.go:43-83`). `record_mistakes` stores dated corrections. `get_study_digest` and the `hebrew://tutor` resource were written for an AI conversation partner ("a Grok tutor", INIT-026 Source line), not for Aaron's classes.

**How much it has been used.** It has not been used. The digest today shows `taught: []` for the last 14 days and `corrections: []`, and `closedLoop` reports 0 corrections since 2026-10-11. INIT-026 was created on 2026-11-01, so the 14-day window covers every day since it shipped. That is a strong inference, not a direct read of the tables.

**Options, by Brian's effort, lowest first:**

1. **Zero steps per class.** If Aaron's class docs keep landing in the same Drive folder, an agent reads new docs on a schedule, builds the pack, and leaves a coach note: "Loaded class 23: 9 words, 1 story, 2 corrections." *Assumption:* the folder is still in use and an agent can be given read access to it. A Google Drive integration is available in this environment, but I did not use it.
2. **One step per class.** Brian drops the doc, a photo or a few lines of notes into an agent chat with "load today's class". The agent does the rest.
3. **Several steps.** Brian types words into a form or calls tools himself. He will not keep this up. The loop built that way has not been used.

**Recommendation:** option 1, with option 2 as the fallback. Start with a one-off backfill of the most recent 8 to 10 classes, so the first week of sessions is already about his own lessons.

**Trust rules for a pack, since no person checks it:**

- A practice sentence is either verbatim from the tutor's text, or built from taught words and checked against the lexicon. A wrong answer button may only use a taught word.
- Text Brian typed in the class doc is not trusted as a model answer. INIT-012 found that "the live docs hold learner errors and some pointing that looks swapped (`אַכָל`, `לַמָד`)". But the tutor's corrections in those docs are the best source for the mistakes list.
- The vowel points come from the lexicon, never from the doc. The worksheet's reading passage has no points, and the app requires pointed text, so this step needs a reliable path. That is a question for the Technical Architect.
- **A decision for Brian:** INIT-012 decided "Content stays original ... None of their text is copied." That made sense for a shared course. For a personal tool built from his own classes, reusing the tutor's sentences looks reasonable, but it is his call, and perhaps Aaron's.

#### 6. Keep, shrink, or stop (question 3), in order of value to the loop

| # | Part | Call | Reason |
| --- | --- | --- | --- |
| 1 | FSRS spaced review and the due queue | **Keep** | This is the "science backed" core. Change the mix toward recall and typing, and cap the daily load. |
| 2 | Pointed lexicon and audio clips | **Keep** | The trusted base for every Hebrew text and for listening. |
| 3 | Learner model, study digest, coach notes over MCP | **Keep** | This is how the AI "knows where I am". Keep the model, and change how it is shown (row 8). |
| 4 | Tutor data loop (`record_taught_words`, `record_mistakes`) | **Keep and grow** | The right idea, scoped too small. Grow it into the class pack import. |
| 5 | `say`, dialogue play-a-side, `order`, `produce`, `type_it` | **Keep** | These are the production formats he asked for. Add "type the sentence". |
| 6 | Report a problem (hide a lesson) | **Keep** | The trust valve. Count its use as a measure. |
| 7 | AI-written units from frequency bands | **Shrink** | Replace them with class practice built from packs. Keep at most one ahead. Stop pre-building and revising a queue of 20 or more units. |
| 8 | Progress page | **Shrink** | One honest view: per class pack, "words you got right on a later day", and what is due. Remove the numbers that contradict each other (36, 2, 502, and 4 for spelling at 97% right). |
| 9 | Placement and the check-in | **Shrink to frozen** | It has a large code cost, was used twice, and gives a misleading estimate. His tutor knows his level. Take it off Today and Progress. |
| 10 | Verb tables and noun forms | **Shrink** | Reference only, filled from class words. The verb workshop is one of the tutor's lesson types, and his corrections are conjugation errors, so this is useful but not central. |
| 11 | Weekly report | **Shrink** | Keep it as AI-facing data in the digest. On screen, at most one line. |
| 12 | Isaiah, verse by verse | **Stop new work** | It waits for a human checker who will not come, and it does not serve conversation. If a class covers a verse, that comes in as a class pack. |
| 13 | Roots hub | **Stop new work** | It is a reading aid of little use at 36 known words. Leave the pages if they cost nothing. |
| 14 | Multi-user, teacher, shared course, general mistakes list | **Stop** | He said only he matters. Delete the stale `hebrew-worksheet` skill that still names `create_plan`. Keep auth to one admin and one learner. |
| 15 | Sound-contrast pairs waiting for native approval | **Stop** | It is blocked on a person who will not come. Revisit only if pairs can come from the tutor's own audio. |

#### 7. Opportunities

1. **About 85 tutor lessons already exist.** The content problem is mostly a loading problem. Loading the last 8 to 10 classes gives weeks of practice about his own lessons, with sentences he has already met. That is the cheapest route to both relevance and trust.
2. **The tutor's lesson formats are the activity variety he asked for.** Choose the right sentence, true or false after a story, unscramble, translate a whole sentence, dictation, write 6 to 8 sentences (INIT-012 section 0b). Building lessons from them fixes "every lesson is the same" without inventing new formats.
3. **The app can make the human tutor better.** A weekly "for your next class" note built from the digest turns the AI into the supplemental tutor Brian described, at almost no build cost: the digest already exists.
4. **He is already good at typing.** Typed recall holds best on a later day, so moving the mix toward typing plays to a strength and to the evidence.
5. **Removing things makes the app easier to follow.** Fewer screens and one honest number answer "easy to understand what is going on" more directly than any new feature.

#### 8. Risks and concerns

1. **High: the loop depends on loading classes, and that has never happened.** The runbook has waited since INIT-016 for a "backfill under way" decision. `record_taught_words` has never been called. If loading takes more than one step, it will not happen, and the app will drift back to inventing a syllabus. *Mitigation:* the agent reads the Drive folder, and the first backfill happens before anything else.
2. **High: AI-generated Hebrew is still unchecked.** Re-pointing unpointed tutor text, building new sentences, and grading typed sentences all leave room for errors that Brian will see. *Mitigation:* use verbatim tutor sentences where possible, take vowel points from the lexicon, grade without requiring points, and accept several right answers for a sentence.
3. **High: more correction cycles, with no new value.** If the next initiatives keep repairing the invented-lesson pipeline (as 27 to 37 did), the loop never ships, and Brian keeps studying about 9 minutes a day. *Mitigation:* freeze new work on the pipeline beyond what keeps Today safe, and move effort to the import and the session.
4. **Medium: grading typed sentences can recreate the "all of them are right" problem.** A strict grader will mark correct Hebrew wrong. *Mitigation:* grade word by word, ignore vowel points, accept other word orders, and let him mark "I was right".
5. **Medium: no human tests his conversation, so the app can optimize proxies again.** *Mitigation:* the tutor class is the real test. A two-tap self-rating after each class ("how much did I understand?", "how much could I answer?") is a cheap outcome signal that needs nobody to check anything.
6. **Medium: using the tutor's text conflicts with an earlier decision.** INIT-012 decided content "stays original". *Mitigation:* Brian decides, possibly with Aaron.
7. **Low: cutting features loses sunk work.** There is one user, and freezing is reversible. The real cost is attention, not code.

#### 9. Scenarios

**Optimistic.** In two weeks the last ten classes are loaded, and each new class appears the next morning with no effort from Brian. Every item in a session is something he saw in class, so nothing looks alien, and he studies 25 minutes on most days. Later-day English-to-Hebrew recall rises from 59% to above 80%. The due backlog fits inside one session. Aaron notices that last week's words stick. In six months Brian knows several hundred words he can produce, has typed hundreds of class sentences, and can follow a simple class conversation at normal speed.

**Pessimistic.** The class docs turn out to be messy: unpointed, mixed with his own errors, in tables that are hard to parse. The import makes visible mistakes, so trust drops further. Or loading stays a manual step that he skips after a busy week. The app falls back to band-driven AI units, Progress keeps saying he knows 36 words, he keeps studying about 9 minutes a day, and in six months the app is a well-tested review deck that he opens out of habit.

**Most likely, in my view.** The import works for vocabulary and corrections almost at once, which already fixes relevance for review. Sentences and stories need a few iterations to be clean. The deciding factor is whether loading becomes automatic.

#### 10. Recommended approach

**Phase 0 (days). Stop the bleeding and simplify.**

- Keep Today on review-first until the import exists, which is roughly where it is now.
- Hide placement, the check-in, Isaiah and roots from Today and the menu.
- Delete the stale `hebrew-worksheet` skill.
- Replace the Progress headline with one honest line and one per-skill line in plain words. For example: "Spelling: 32 of 33 right in two weeks; 4 words passed their next-day check."

**Phase 1 (1 to 2 weeks). Class in.**

- A class pack import that turns words, tutor sentences, the text, the grammar point and corrections into cards and mistakes, with vowel points from the lexicon. Prefer reading the Drive folder.
- Backfill the last 8 to 10 classes.
- Review pulls class words first.

**Phase 2 (2 to 3 weeks). Practice the class.**

- One class practice per pack, built from the tutor's sentences and the tutor's lesson formats (listen and answer, type the sentence, order, say an answer, play a side).
- Today becomes one 20 to 30 minute session with a closing summary.
- Progress per class pack.

**Phase 3 (later, only if phases 1 and 2 hit their measures).**

- Graded stories from taught words.
- A weekly note for Aaron.
- Then speech recording, and an AI conversation partner limited to known words.

**Success measures.** Baselines are from today's digest and `get_context` where they exist.

| Measure | Baseline | Target |
| --- | --- | --- |
| Lessons he skips or reports as wrong, per week (trust) | "often" (his words; no count) | 0 by week 4 |
| Minutes studied per week | 64 (week of 2026-11-02) | 120 or more by week 4, then about 150 (5 days at 30 minutes) |
| Tutor classes loaded within 2 days | 0 of about 85 | All new classes, plus the last 8 to 10 |
| Later-day English-to-Hebrew recall on class words | 59% (recall retention, all words) | 80% or more by week 6 |
| Due cards at the start of a session | 161 | No more than one session's worth (about 70, an assumption) |
| Self-rating after each tutor class: understood, and could answer | none | Rising over 3 months |
| 6 to 12 months | not measured | A 5-minute simple conversation with Aaron at normal speed, judged in a normal class |

#### 11. Questions for other specialists

- **For the Technical Architect:** What is the smallest way to turn one class doc, or a worksheet shaped like `Aharon's code/worksheets/*/worksheet.js`, into cards, mistakes and one class practice: the existing `upsert_lexemes`, `record_taught_words`, `record_mistakes` and `put_unit`, or a dedicated import? How do we add reliable vowel points to unpointed tutor text? And if the Hebrew is the tutor's verbatim, how many of the 61 validator rules and the 364-line author skill can be retired?
- **For the QA Advocate:** With no human checker, what automatic checks would let a class practice reach Today safely? For example: every graded sentence is verbatim from the pack, or every token is a taught lexicon word; wrong answer buttons only from taught words; typed answers graded without vowel points. What weekly "trust budget" of visible errors should stop publishing?
- **For the Data Analyst:** Which single progress number would match Brian's own sense of what he knows and still be honest? For example, "words you got right on a later day, in any direction", shown per class. Can we measure understanding of connected speech at normal speed with the data we collect? How should 161 due cards be paced into a 25-minute session without crowding out class practice?
- **For the Devil's Advocate:** What is the strongest case that a plain spaced-repetition deck built from the class docs, plus Aaron's own worksheets, would beat this custom app for Brian's goal? Is retaining class material really the bottleneck for conversation, or is it the amount of speaking practice, which no app here provides?

#### Evidence ledger (what I read this session)

| Finding | Source |
| --- | --- |
| New words come from frequency bands, not the tutor | `internal/brief/build.go:906-961` |
| The tutor loop schedules word cards only and has never been used | `internal/evidence/taught.go:43-83`; `get_study_digest` (learner 1): `taught: []`, `corrections: []`, `closedLoop.total: 0` |
| The class backfill was never started | `docs/operations/class-data-through-mcp.md` ("Backfill under way: not yet"); INIT-016 SPEC-004 completion summary ("Nothing was loaded") |
| The tutor's lessons: Aaron on Preply, about 85 class docs, five lesson types, formats the app lacks | INIT-012 initiative, Source line and sections 0b and 2 |
| The design premise was a learner without a tutor | `docs/design/learner-without-a-tutor.md` |
| Lessons share one template and repeat the same fix-mistake checkpoint; the stories are tiny | `get_unit` for unit 6 and unit 11; `.cursor/skills/hebrew-lesson-author/SKILL.md` lines 113 to 121 |
| Lessons score 88%, review 72% over 7 days and 48% over the last 50 | `get_context` `recentAccuracy` (learner 1) |
| Spelling is "strong" at 97% right, yet 4 words show as known; the headline numbers conflict (36, 2, 502) | `get_context` `model` (learner 1); `internal/learner/state.go:56-74` |
| Typed practice holds best on a later day | `get_context` `practiceResults` |
| Isaiah appears only after a person checks a verse | `internal/web/shell.go:263-269` |
| The plan builder is gone from code, but a skill still names it | No `create_plan` outside docs; `.cursor/skills/hebrew-worksheet/SKILL.md` |
| Speaking is self-checked, with no recording and no recognition | INIT-020 initiative, Notes |
| Code size of placement and auth | Line counts of `internal/placement` and `internal/auth`, excluding generated files |

### Summary

**Position:** The app has the right engine (spaced review, lexicon and audio, a learner model, an AI interface) but the wrong center. It is a self-contained course that invents its own syllabus, while Brian needs a companion that helps him keep his tutor's lessons and moves him toward conversation.

**Recommendation:** Rebuild around one loop: class in, practice daily, see what stuck, tune and hand back.

- **MVP:**
  - import the tutor's class docs automatically, starting with the last 8 to 10, as packs of words, sentences, the text and corrections;
  - one 20 to 30 minute session: review first, then listen, type sentences, and speak, all from class material;
  - a new "type the sentence" practice;
  - one honest Progress view per class.
- **Keep:** spaced review, the lexicon and audio, the learner model and digest over MCP, the tutor tools (grown into the import), the production formats, and Report a problem.
- **Shrink:** AI-invented units (to at most one ahead, built from packs), Progress (to one view), placement (frozen and hidden), verb tables (reference only), and the weekly report (data for the AI).
- **Stop:** Isaiah, roots, multi-user and teacher features, and the stale plan-builder skill.
- **Measures:** zero lessons skipped or reported as wrong; at least 120 and then about 150 minutes a week; every class loaded within 2 days; class-word recall on a later day of at least 80%; a rising self-rating after each class.

**Top Risk (High):** Loading the tutor's classes has never happened, despite tools and a runbook that have been ready for weeks. If loading needs any regular effort from Brian, it will not happen, and the app will drift back to AI-invented lessons that keep breaking his trust.

**Confidence:** Medium-High on the diagnosis, because it rests on code, live learner data and the project's own records. Medium on the MVP, because two things are unverified: how clean and accessible the class docs are today, and whether Brian wants the tutor's own text reused.

---

## Phase 4: Dialogue Contributions

### Questions Posed
- To the Technical Architect: what is the smallest path from a class doc to cards, mistakes and practice, and how many validator rules can go if the Hebrew is the tutor's?
- To the QA Advocate: which automatic checks make class practice safe, and what error rate stops publishing?
- To the Data Analyst: which single number is honest and matches Brian's sense of himself?
- To the Devil's Advocate: is retaining class material the bottleneck to conversation, or is speaking time?

### Questions Answered
- **From the Architect, QA and Data:** Does the tutor produce material an AI can take in under five minutes?
  **Response:** The material exists: about 85 class docs in Drive, and `Aharon's code/` looks like the same course. A zero-effort path has never existed. With the Google Drive connection, an agent reads the newest class doc directly. Unknowns: how clean the docs are, how many lines carry vowel points, and which lines are the tutor's and which are Brian's.
- **From the Devil's Advocate:** Is speaking time the real bottleneck?
  **Response:** Partly. The app makes tutor time count through retention, ear training at normal speed and rehearsal aloud. Its weekly note points tutor time at weak spots. Conversation itself happens with the tutor.
- **From the Data Analyst:** Should scheduled reading and spelling cards be dropped?
  **Response:** Yes for reading. Keep spelling only as the typing inside production.

### Tensions Involved In
- Build now or freeze first: accepted Stage 1 trust fixes first and the audit gate before the import.
- Reusing tutor text: tutor-written lines are trusted keys. Brian decides on private reuse and may ask the tutor.
- How much the AI writes: accepted that AI stories and notes are ungraded.

### Synergies Identified
- The syllabus comes from the tutor: shared with the Architect, QA and the Devil's Advocate.
- Progress must be legible and stable: shared with Data, the Devil's Advocate and the Architect.
- More production practice: shared with QA, the Architect and Data.
- A smaller surface (freeze or stop Isaiah, the roots hub, placement, and multi-user features): shared with the Architect and the Devil's Advocate.

---

## Phase 5: Consensus Position

### Option Scores
| Dimension | A: Patch in place | B: Staged | C: Re-center now | D: Freeze and go simpler |
|-----------|---|---|---|---|
| Learning value | 2 | 4 | 3 | 2 |
| Trust and content correctness | 1 | 5 | 2 | 3 |
| Effort and time to value | 2 | 4 | 2 | 3 |
| Treadmill risk (higher = lower risk) | 1 | 4 | 2 | 4 |
| Preserves progress and history | 4 | 4 | 3 | 3 |
| Reversibility | 3 | 4 | 2 | 4 |
| **Total** | **13** | **25** | **14** | **19** |

### Final Position
**Preferred Option:** B, stabilize and then re-center on the tutor (staged).

**Rationale:** Option B is the only one that fixes the trust breaks Brian named within days, mostly by deleting and reversing things, and then moves the app's center to his tutor's lessons, which is how he defined the product himself. The audit gate tests the riskiest assumption in the plan, that class material can be loaded with no effort from him, before anything is built on it, while he still gets a better app in week 1. Option D comes second because its instinct (tutor sentences, stop building) is right, but it leaves the known harms in place for 14 days and moves the work onto Brian, which is the effort he has told us he will not spend.

**Conditions:**
1. **Brian sees a difference in week 1.** Stage 1 ships these first: listening switched back on with word clips; unprovable wrong answers voided as a reversible flag, with the old and new numbers shown once; and one Progress line in his terms, counted out of taught words and headed by "understand by ear" and "say it from English". Every change carries a line saying what Brian will see differently.
2. **Today in Stage 1 is a session, not a backlog.** Cap review at about one session's worth (about 70 cards, an assumption), weight it toward typing and English-to-Hebrew, and end with a short summary of what happened. A Today that is just 161 due cards brings back the 9-minute habit.
3. **The audit costs Brian nothing.** An agent reads the Drive folder in week 1, in parallel with Stage 1. The fallback if the audit fails is agreed before it runs.
4. **Brian decides on private reuse of the tutor's text** (and may ask Aaron) before Stage 2 starts.
5. **Judge the plan by what Brian does, not by what ships.** By week 4, minutes per week should be 120 or more (64 now), and lessons skipped or reported as wrong should be near zero. The monthly normal-speed listening check should rise over two months. If minutes have not moved by week 4, pause Stage 2 and look at Option D again.

**Dissent:** Aligned with consensus. One point of emphasis: the main risk I see in Option B is not that Stage 1 does too much, but that it gives Brian too little. If Stage 1 only removes things, Today becomes review alone, and he gets none of what he asked for by name (typing sentences, interesting stories, knowing where he stands) until Stage 2. That is why conditions 1 and 2 are required, not optional. When Stage 2 starts, its first deliverables should be the tutor's own reading texts and "type the sentence", because those are the things he asked for.
