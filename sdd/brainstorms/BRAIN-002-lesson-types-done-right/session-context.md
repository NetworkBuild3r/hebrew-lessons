# BRAIN-002 · Session context

**Topic:** How to build the five lesson types right: lesson design, flow, and making lessons usable, useful, and effective
**Session type:** 🎯 Product / Feature, Standard depth
**Date:** 2026-11-10
**Decision-maker:** Brian Nelson. He is the only learner, the owner, and the developer.
**Panel:** Product Strategist (lead), UX Designer, Technical Architect, Data Analyst, QA Advocate, Devil's Advocate. Marketing and Cost were dropped because the app serves one person.
**Output:** Decision document plus a spec skeleton that replaces the provisional Stage 2 list in BRAIN-001 `spec-skeleton.md` §5.

## Goal

Decide how the five lesson types should look and flow, screen by screen, so a lesson is:

- **Usable:** clear, fast, no confusion about what to do next.
- **Useful:** it teaches words Brian needs for his tutor classes and his goal.
- **Effective:** the words are remembered on a later day.

Then decide the smallest build order that delivers that.

## The learner

- One adult learner (Brian), English speaker, learning modern Hebrew with a tutor on Preply.
- About 85 class documents from those classes are in a Google Drive folder, and none are loaded yet.
- His goal for 6 to 12 months is a simple spoken conversation: understand Hebrew at normal speed and answer.
- He will give 20 to 30 minutes a day if sessions are worth it. Weekly study was 0, 35, 57, then 64 minutes over the last month.
- **Trust is what stops him.** When a lesson looks wrong he skips it or stops. Past complaints: buttons for words he was never taught; a gap where several answers are right but only one counts; Progress saying he knows nothing.
- In his words: "I have a tutor. I want a tool to help me relearn our lessons but actually retain what I learned through science backed research." And: "AI is to be my supplemental Tutor."

## The design on the table

Read in full: `docs/design/lesson-architecture.md`. The five types are:

1. **Word lesson** (most sittings). About 4 new dictionary words, up to 8 past the beginner tier. The steps:
   1. A grid of all the new words, ungraded: pointed form, Listen, part of speech, binyan, meaning, one example, and verb forms closed.
   2. A meaning check. The meaning is hidden; he picks the English. A miss shows the meaning and asks once more.
   3. A story at about 90 to 95% known words. Tap a word for its meaning and sound. He can repeat a line, ungraded.
   4. Write it: type the word from the English, with the Hebrew hidden.
   5. Use it in a sentence (order, dictation, "which line did you hear"), once he can produce the word.
2. **Conjugation lesson.** No new dictionary words. One set of forms (for example, past tense) on verbs he already knows: see the forms, practise them, read them in sentences.
3. **Listening lesson.** No new words. Hear a passage, answer from the audio, and the text stays hidden until the answer.
4. **Easy lesson.** No new words. A longer passage of words he is learning, then the usual questions.
5. **Spelling lesson.** No new words. Gated by the letters check. Type known words with the Hebrew hidden. Speed is not the score.

Outside lessons: **review** on a later day is the checked test, and it is what decides a word was remembered. **Flashcards** are optional, in My stuff. The **tutor** covers spoken conversation. Conversation is not a lesson type, because the app cannot score speech.

## Decisions already made (treat as ground truth)

- **Code assembles every lesson (Brian, 2026-11-10).** Words come from the tutor's class packs. Cards and answer keys come from the lexicon, the stored verb forms, and the tutor's own sentences. The AI writes one thing per lesson, the story. The story is checked for pointing, lexicon, coverage, and length, and it is never graded. An outside AI on the MCP fills that slot only.
- **BRAIN-001 (Brian, 2026-11-09)**, `sdd/brainstorms/BRAIN-001-is-the-hebrew-app-built-right/decision-document.md`:
  - Stage 1 (INIT-038, trust and measurement) is in flight and must close by **2026-11-30**.
  - Stage 2 re-centers the app on the tutor's lessons. It opens only after a gate:
    - the Stage 1 exit is met;
    - a Drive audit finds 150 or more usable tutor lines, and 5 of the last 10 classes loadable without Brian;
    - Brian has ruled on private reuse of the tutor's text;
    - there is enough evidence under the new rules.
  - If the audit fails, practice is review plus ungraded stories, and words are added with `record_taught_words`.
  - The provisional Stage 2 list (P-01 to P-13) is in `spec-skeleton.md` §5, revised today around the five types.
- **ADR-0033** (`docs/decisions/ADR-0033-stage-1-trust-and-measurement.md`):
  - D3: an answer the app cannot prove wrong records nothing, and no AI-written sentence is graded.
  - D4: the "proved" ladder. Checked right answers on 3 or more study days spanning 7 or more days, with the newest right.
  - D10: Today is one 20 to 30 minute session.
  - D13: one initiative in flight; a new validator rule retires an old one; delete first.
  - Abandon line A2: a second initiative opening before Stage 1 closes.
  - Success measures:
    - S1: study minutes, 120 or more a week by week 4.
    - S2: 50% of words taught 14 or more days ago proved in "Hebrew from English" by 2027-01-04.
    - S3: a held-out probe of 20 listening items and 10 English prompts.
- **Process lesson from BRAIN-001.** 34 initiatives and 258 commits in a month produced about three hours of study. Avoid the treadmill: too many specs and rules, too little use.

## What the code has today (ASMT-006, Tier A)

File: `sdd/assessments/ASMT-006-hebrew-lesson-types/ASMT-006-hebrew-lesson-types-assessment-report.md`.

**Works today:**
- a single-word lesson `recall` (Hebrew shown, English buttons from lexicon glosses, overlapping glosses dropped);
- story with tap-for-meaning;
- shadow and say (ungraded repetition);
- typed answers with letter-level grading in review (`type_it`, `GradeTyped`);
- verb forms stored per person, number, and gender;
- the consolidate path (which is the easy lesson).

**Missing:**
- the word grid;
- typed lesson `produce` (today it is three Hebrew buttons);
- a second ask after a miss;
- English buttons for a by-ear meaning check;
- a forms screen;
- a recorded letters check;
- a lesson-type label.

Under D3, multi-word `listen_pick`, `cloze`, and `order` are ungraded reveals.

## Research (grades as recorded in this repo)

| Report | Path | What matters here |
| --- | --- | --- |
| RSCH-003 | `sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md` | Spacing beats massing (High). Retrieval beats rereading (Moderate). Recall vs recognition (Low). Interaction (Low). Pronunciation with feedback (Moderate). A same-session repeat is practice, not review. |
| RSCH-004 | `sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md` | Four strands: meaning input, meaning output, deliberate study, fluency (Moderate for the labels, Low for the sequence). No tested words-per-lesson count; classroom courses about 4 per hour. Coverage of about 95% known words (Low). A fluency pass uses familiar language only. |
| RSCH-005 | `sdd/research/RSCH-005-pictures-typing-spelling/RSCH-005-pictures-typing-spelling-report.md` | Producing a spelling after it is hidden is the spelling task (Moderate). Copying while visible is poor. Writing a brand-new word into a sentence at first meeting weakens form memory (Moderate). No effect of keyboard practice on vocabulary. |
| RSCH-001 | `sdd/research/RSCH-001-english-to-hebrew-learning/RSCH-001-english-to-hebrew-learning-report.md` | Synthetic speech did not help spontaneous production. |
| RSCH-006 | `sdd/research/RSCH-006-learner-interface-design/RSCH-006-learner-interface-design-report.md` | Learner interface design. |
| RSCH-011 | `sdd/research/RSCH-011-practice-layouts-that-teach/RSCH-011-practice-layouts-that-teach-report.md` | A checked question on a known word ranked first (5.00). Sentence build ranked second (3.50). |

## Main questions

1. What should each lesson type's flow be, screen by screen, so it is usable, useful, and effective? What should be cut from the current design?
2. How does a lesson fit inside Today's single 20 to 30 minute session alongside review? Lesson first, review first, or mixed?
3. How many of the five types are needed in the first version? Which can wait?
4. How do we keep trust: no step that can grade a right answer wrong, no surprise words?
5. How will we know it works, using the measures already defined (S1, S2, S3), without new machinery?
6. What is the smallest first build after the Stage 2 gate, in what order, with the fewest specs and rules?

## Information injection R1 (after Phase 5, 2026-11-10)

**Brian, asked what Today should do on heavy-review days:** "I won't be doing this just after a tutor lesson. So we need lessons that don't always align to my tutor's lessons but can help me keep learning."

**Classification:** new constraint plus a correction to an assumption. Two assumptions were wrong:
- the panel assumed the newest class drives the lesson;
- the panel assumed a review-only day is acceptable.

**What the injection means:**
- **Lessons must not depend on a recent class.** Most sittings are not right after a class.
- **There must always be something new to learn.** That includes the older classes (about 85 docs in total, of which 10 were audited), and words and forms beyond the classes.
- **A review-only Today is not what he wants** as the default.

**Brian's other answers in the same round:**
- **Typed-grading fix:** fix it now inside INIT-038, as a Stage 1 trust bug.
- **Pre-gate backfill:** this agent does it, through the MCP, as data entry only, T0 words.

**Areas affected:**
- which words a lesson teaches (the word source and order);
- what Today offers on a heavy day;
- the easy and conjugation lessons' priority;
- the class-usefulness measure;
- the backfill scope.

## Phase 2 user answers

Brian's overall direction: **"I want to know Hebrew better. However we achieve that."** For most questions he answered "Whatever helps me learn Hebrew better." On those, the panel decides from the research and the code, and must say why.

| Question | Specialist | Brian's answer | What it means for the analysis |
| --- | --- | --- | --- |
| How much to build before studying with it for two weeks | Devil's Advocate, Product | "After 2 weeks. I want to know Hebrew better. However we achieve that." | He wants to be studying with the new lessons within about 2 weeks. The scope is the panel's call. |
| What made lessons feel the same | Devil's Advocate | **The screens** (same steps, same order) **and the content** (same words, topics, story style). | Both matter. Variety must come from flow and from content. Tutor words alone do not fix it. |
| Which class words first | Product | Delegated | Panel decides. |
| Days when review fills the session | Product | Delegated | Panel decides. |
| Device | UX | **Desktop with a keyboard, and phone sitting down with both thumbs.** Not on the go. Silent sittings were not selected. | Design for both. Sound is normally available. |
| Typing Hebrew | UX | **Fine.** Hebrew keyboard installed, knows most letters. | Typing in lessons is acceptable. No in-app letter pad is needed. |
| When the app is unsure a typed or picked answer is wrong | QA | **Grade it, with an "I was right" or "this is dumb" button.** That erases the miss and **flags the item to be fixed.** | Grade it, give him a way out, and feed the flag back so the key gets corrected. |
| Checks before a lesson reaches Today | QA | **"I have no tutor [reviewer]. I have you and Grok. So let's do as much as we can programmatically."** | No human review. Maximize code checks and fixed regression lessons. An AI reviewer is allowed but must never grade. |
| Tutor-doc line fails a check | QA | Delegated | Panel decides. |
| When code builds a lesson | Architect | Delegated | Panel decides. |
| Story missing or failing | Architect | Delegated | Panel decides. |
| Keep the unit document or replace it | Architect | Delegated | Panel decides. |
| Per-type measurement | Data | Delegated | Panel decides. |
| Cut rule for a lesson type | Data, Devil's Advocate | Delegated | Panel decides, and must still propose a rule agreed before building. |
| Proof of usefulness for classes | Data | Delegated | Panel decides. |

**Facts found in Phase 3 (checked against the source files):**
- **The Drive audit already passed (gate G2)** on 2026-11-09, per `sdd/initiatives/INIT-038-today-you-can-trust/specs/completion-summaries/SPEC-012-audit.md`:
  - 286 usable tutor lines;
  - 10 of the last 10 classes loadable;
  - 158 of 350 tutor lines are pointed, so less than half carry vowel marks;
  - 974 unique tokens, of which 742 have no match in the lexicon. Some of those are prefixed or inflected forms, so the true gap for target words is smaller but unknown.
- **Brian rejected the computer-read voice** on 2026-11-09 (`sdd/complaints.md` line 11). No `generate_audio` batch runs until he names a source he accepts. So the following are blocked until there is an accepted voice:
  - the grid's Listen button;
  - the meaning check by ear;
  - dictation and "which line did you hear";
  - the listening lesson;
  - the S3 listening probe.

**Clarification (asked after Phase 2):**
- **Tutor:** "I have a tutor who will never look at this code or app. Never will have a login." The classes continue and the class docs keep coming, but the tutor never reviews, enters data, or uses MCP. Any design that depends on tutor input or tutor review is out.
- **G3, reuse of the tutor's text (ruled 2026-11-10):** **Yes, privately, for his own study.** That covers sentences, word lists, everything. It is never shared or published. ADR-0033 G3 is now decided.
- **Speaking:** "We can't do conversations on the app. Which is fine. My AI can do it." Conversation practice happens with his own AI, outside the app. The app may prep him for it but does not host it.
