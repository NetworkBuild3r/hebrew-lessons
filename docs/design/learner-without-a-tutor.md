# The hybrid model: site map and build plan

**Status:** draft for review. No code yet. First written 2026-10-23, revised the same day for the hybrid model.

The tool does what the research says helps, and it tunes every lesson to the one learner. It works on its own for someone without a tutor, and alongside a tutor for someone who has one. Live conversation is the one part a site cannot be, so the plan gets as close to it as it can.

Names of the tutor and the first learner, and class transcripts, stay out of the site and of this repo.

This builds on [the North Star](../north-star.md): the learner model, the lesson brief, and its research rules are already designed and built there. This doc adds the course around them, tutor input, and the rules not yet in code.

## The hybrid model

Three sources decide each lesson:

1. **What the research says helps** (the rules below). Fixed in code, the same for everyone.
2. **The learner's own record**: what they get right, what they miss, what they have not seen. Read from the learner model every time a lesson is made.
3. **A tutor, when there is one**: new words and patterns from class, and the learner's mistakes, sent in through MCP. Without a tutor, the default path stands in.

### The rules every lesson follows

| Rule | What it means in a lesson | In the repo today |
| ---- | ------------------------- | ----------------- |
| **Mix weak with strong** | Every lesson has struggling words, new words, and comfortable words the learner already gets right. The comfortable words carry the story and give quick wins, so a lesson never feels like a wall of misses. | Yes: the lesson brief (`internal/brief`) picks struggle words, new words and comfortable words from the learner model. |
| **Few new things at once** | 5 to 8 new words, 3 to 4 for a beginner. One new pattern per unit. | Words: yes (brief limits). Patterns: not tracked. |
| **Spacing** | A word comes back just before it would be forgotten. | Yes: FSRS review schedule. |
| **Retrieval before recognition** | The learner recalls before the answer is shown. A card never shows the meaning before the answer. | Yes: recall steps and the card rule (`cardsIn`). |
| **Interleaving** | Words and patterns from earlier units are mixed in, not drilled in blocks. | Partly: struggle and comfortable words come from anywhere. Patterns are not mixed yet. |
| **Aim at a success rate** | Hard enough to stretch, easy enough to win most of the time (roughly 8 in 10 right). If the learner is missing more, the next lesson leans on comfortable material; if they are cruising, it adds more new. | **Not built.** The brief does not yet look at the recent success rate. |
| **Produce, not only recognise** | The learner says and builds Hebrew, not only taps. | Partly: `produce`, `shadow`, `dialogue` steps. No say-it-aloud step. |
| **Explain at the point of need** | When a pattern first appears, a short explainer is one tap away. | **Not built.** |
| **Fix what keeps coming back** | A mistake the learner repeats gets targeted practice and its contrast ("you said X, it is Y"). | Partly: confusions are recorded per learner. No mistakes list. |
| **Short and daily** | Short sessions, a streak, and a visible "you can now say these things". | Partly: daily job and progress. No "can say" list. |

### Who decides what

- **Code decides which words and patterns** go into a lesson, from the rules and the learner model. This is already how the brief works: "the code picks the words and the AI writes the lesson."
- **An author writes the lesson** from that plan: a person, or any MCP client (a swarm, a bot, an agent). It cannot override the plan's word choice.
- **A tutor adds to the record**, through MCP: class words, patterns, and mistakes. That changes what the code picks next.

## Two paths, one tool

- **Default path, for a learner without a tutor.** The course in units, written as our own material. It **starts from the class order and is then reordered** wherever a pattern appears before what it depends on. Class order followed one tutor's choices, Bible passages and a schedule, which suits a tutored learner but not always a beginner alone. The prerequisites table makes the order checkable. The hybrid rules still tune each lesson on top of it.
- **Tuned path, for a learner with a tutor.** Lessons for that learner only (`scope = 'learner:<id>'`), built from their record and their tutor's input. The shared course is `scope = 'shared'`. Both exist in the repo today.
- **The tutor stays.** The tool does the drilling, spacing and recall between classes. The tutor's time goes to conversation and correction.

## Organizing idea and sections

The site is organized around **what you are learning**. A worksheet type is one way in, not the top level, because a worksheet type describes what is on the page, not how class time is spent. **Roots tie everything together:** a root page links the words, the verb forms, the lessons and the verses that share it.

| Section | What it is | Today in this repo |
| ------- | ---------- | ------------------ |
| **Speaking path** | The core course. Short units, each adding words on the last. A unit has one pattern, a few new everyday verbs, and 3 to 5 old words brought back, and it makes you say things aloud. | Exists: units, lessons, brief, review schedule, Today, Progress. `shadow` and `dialogue` steps. |
| **Practice by worksheet type** | Topic conversation, review sheet, verb workshop, short story with true/false, foundations and reading. Each type is a reusable template. | Partly: `story`, `cloze`, `pattern`, `dialogue` steps. No template layer, no browse by type. |
| **Learn (explainers)** | One idea per page, plain English, pointed examples, keyed to a grammar pattern. | Missing. |
| **Isaiah** | Verse by verse: pointed text with word-by-word gloss; the root of each key word; "the manner of the Jews" (parallel lines, wordplay and sound play, idioms, how Jewish tradition reads the verse); a bridge to modern Hebrew. | A verse importer (`internal/tanakh`). No reading UI, glosses or layers. |
| **Roots hub** | One page per root: everyday words, verb forms, lessons and verses that use it. | `lexemes.root` column. No pages, and nothing can write roots yet. |
| **My stuff** | Word bank, verb tables for known verbs, the learner's own mistakes, and what they can now say. | Basic `/me` and Progress. |

## What a tutor gives, and what replaces it

| A tutor gives | The site's answer |
| ------------- | ----------------- |
| Order and pacing | The default path (class order, fixed by prerequisites), with the brief and the review schedule choosing each lesson. |
| Explanations | Explainer pages, keyed to patterns, linked from the step. |
| Hearing real Hebrew | Recorded audio for words, sentences and stories, slow and normal speed. Short stories from known words only. **Depends on the voice service** (see Dependencies). |
| Being made to speak | Say-it-aloud: an English cue or picture, the learner says it, the model plays, they compare. Shadowing. Self-recording. |
| Corrections | Self-check against the model, plus the mistakes lists. Speech recognition later. The hardest part. |
| Accountability | Short daily sessions, a streak, and a "you can now say these things" list. |
| Conversation | Scripted dialogues where the learner plays one side, then branching role-plays, then an AI partner limited to known words and the current pattern. |

## Mistakes: two lists

- **General list, for the shared course.** "Learners often say X, it is Y", drawn from patterns across mistakes, never anyone's actual answers. Recorded answers are a tutor's typing of speech, so they are weak evidence on their own.
- **Personal list, for the tuned path.** One learner's own repeated mistakes, private to that learner, used by the brief to target practice and shown in My stuff. Never shared, never copied into the general list as is.

## How class data gets in: MCP

Class data is not copied by hand and is not a repo file. It arrives through the MCP server, so any client can load it and tune.

| Class table | MCP tool today | Gap |
| ----------- | -------------- | --- |
| Words (with root and verb pattern) | `upsert_lexemes` | **It cannot write `root` or `binyan`.** The columns exist, but `lexicon.Input` has no field for them. |
| Verb forms by tense and person | none | New table and tool. |
| Grammar patterns, each with what must come first | none (`put_unit` has a pattern step, no prerequisites) | New table and tool. |
| Mistakes, with how often they come back | `add_coach_note` (free text); `get_learner_model` reads confusions | Personal list per learner, and the general list. |
| Phrases with an English speaking prompt | `put_unit` (`shadow`, `dialogue`) | A prompt field for say-it-aloud. |
| Class order | `put_unit` (`position`) | Reorder against prerequisites. |

Tuning tools already there: `get_learner_model`, `get_learner_words`, `get_progress`, `get_lesson_brief`, `put_unit`, `validate_unit`, `publish_unit`, `generate_audio`, `lookup`, `get_context`, `add_coach_note`.

**Do not load class data yet.** Roots, verb tables and mistakes are only as good as the backfill, and the backfill is four classes. The backfill starts alongside build 1, so real data is ready when the pages are.

## Dependencies

- **Voice service.** Recorded audio needs the voice service, which is down to 3 credits, and there is no male voice set up. Until both are fixed, audio for every word and the say-it-aloud step cannot ship at scale. Dialogues with two speakers need the second voice.
- **Backfill.** Builds 3, 5, 6 and the personal mistakes list need it.
- **Isaiah drafting.** An agent that knows the Masoretic text and the Jewish-reading side drafts one verse at a time, and a person who reads Hebrew checks it.

## Decisions (2026-10-23)

Recorded as [ADR-0019](../decisions/ADR-0019-hybrid-learning-model.md).

- **Hybrid model**: research rules in code, tuned per learner from their record, with tutor input through MCP when there is a tutor.
- **Class data source**: a four-class sample (ten tables; 57 of 94 content words have a root, 54 distinct roots). Loaded through MCP, not committed here. Not loaded until the backfill is under way.
- **Root coverage in the site**: unknown. Needs a read of the production database. Likely close to none until roots can be written.
- **Explainers are keyed to grammar patterns**, so lessons link by data, not by hand.
- **A root page appears only when the root has at least two words or a verse.**
- **Default path order**: class order, reordered so no pattern comes before what it depends on.
- **Two mistakes lists**: general for the shared course, personal and private for the tuned path.
- **Content is our own**: same structure and order as the class sheets, not the sheets themselves, unless the author agrees.

## Roadmap

Six small initiatives, each with a one-screen plan under `sdd/initiatives/`. Each ships on its own and gives the learner something visible.

| Initiative | What you get | Specs | Effort | Waits on |
| ---------- | ------------ | ----- | ------ | -------- |
| [INIT-016 Hybrid foundations](../../sdd/initiatives/INIT-016-hybrid-foundations/INIT-016-hybrid-foundations-initiative.md) | "You can now say"; lessons that react to accuracy; roots loadable through MCP; backfill starts | 7 | 21h | nothing |
| [INIT-017 Patterns and explainers](../../sdd/initiatives/INIT-017-patterns-and-explainers/INIT-017-patterns-and-explainers-initiative.md) | Explainer pages linked from lessons; prerequisite check on the path; worksheet templates | 4 | 21h | INIT-016 |
| [INIT-018 Roots hub](../../sdd/initiatives/INIT-018-roots-hub/INIT-018-roots-hub-initiative.md) | A page per root; root link on word cards | 2 | 9h | INIT-016, backfill |
| [INIT-019 My stuff](../../sdd/initiatives/INIT-019-my-stuff/INIT-019-my-stuff-initiative.md) | Your mistakes, verb tables, word bank; navigation by section | 4 | 21h | INIT-016, 017, 018, backfill |
| [INIT-020 Speaking](../../sdd/initiatives/INIT-020-speaking/INIT-020-speaking-initiative.md) | Say it aloud; play one side of a dialogue | 4 | 16h | INIT-017, voice service |
| [INIT-021 Isaiah](../../sdd/initiatives/INIT-021-isaiah/INIT-021-isaiah-initiative.md) | Verse-by-verse reader with checked layers | 3 | 12h | INIT-016, 017, 018, human checking |

Order: INIT-016 through INIT-019 are in the app. The class backfill has not started. INIT-020 and INIT-021 go when their outside dependencies are ready.
## Not in scope here

Speech recognition, an AI conversation partner, and payments or accounts for other learners.
