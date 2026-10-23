# A learner without a tutor: site map and flow

**Status:** draft for review. No code yet. Written 2026-10-23.

The goal is that someone who does not want to pay a tutor can follow the same road a tutored learner takes. Live conversation is the one part a site cannot be, so the plan gets as close to it as it can.

Names of the tutor and the first learner, and class transcripts, stay out of the site and of this repo.

## Organizing idea

The site is organized around **what you are learning**. A worksheet type is one way in, not the top level. Worksheet types describe what is on the page, not how class time is spent. **Roots tie everything together:** a root page links the words, the verb forms, the lessons and the verses that share it.

## Six sections

| Section | What it is | Today in this repo |
| ------- | ---------- | ------------------ |
| **Speaking path** | The core course. Short units in the order things were learned, each adding words on the last. A unit has one pattern, a few new everyday verbs, and 3 to 5 old words brought back, and it makes you say things aloud. | Exists: units, lessons, review schedule, Today, Progress. `shadow` and `dialogue` step types exist. |
| **Practice by worksheet type** | Topic conversation, review sheet, verb workshop, short story with true/false, foundations and reading. Each type is a reusable template, so a new class becomes a practice set quickly. | Partly: `story`, `cloze`, `pattern`, `dialogue` steps cover some. No template layer, no "type" browse. |
| **Learn (explainers)** | One idea per page, plain English, pointed examples. Lessons link to the page at the spot a learner gets stuck. | Missing. |
| **Isaiah** | Verse by verse. Layers: pointed text with word-by-word gloss; the root of each key word; "the manner of the Jews" (parallel lines, wordplay and sound play, idioms, how Jewish tradition reads the verse); a bridge to modern Hebrew. | A verse importer exists (`internal/tanakh`). No reading UI, no glosses, no layers. |
| **Roots hub** | One page per root (for example שׁ־מ־ע, "hear"): everyday words, verb forms, lessons that use it, verses that use it. | Data column exists (`lexemes.root`). No pages. Coverage depends on how many lexemes have a root filled in. |
| **My stuff** | Word bank, verb tables for known verbs, and the mistakes you keep making. | Basic `/me` and a progress view. No verb tables, no mistakes list. |

## What a tutor gives, and what replaces it

| A tutor gives | The site's answer |
| ------------- | ----------------- |
| Order and pacing | A fixed path in class order; the review schedule decides what returns and when. |
| Explanations | Explainer pages, one idea each, linked from the exact step. |
| Hearing real Hebrew | Recorded audio for every word, sentence and story, slow and normal speed. Short stories built only from words the learner already knows. |
| Being made to speak | Prompts that need an aloud answer: an English cue or picture, the learner says it, the model plays, they compare. Shadowing right after the audio. Self-recording. |
| Corrections | Start with self-check against the model plus a "learners often say X, it is Y" list drawn from real mistakes. Later, speech recognition for pronunciation. This is the hardest part. |
| Accountability | Short daily sessions, streaks, and a plain "you can now say these 20 things" view. |
| Conversation | In steps: scripted dialogues where the learner plays one side; branching role-plays with a few right answers; eventually an AI partner limited to the learner's known words and current pattern. |

## Order to build

Each step stands alone and ships on its own.

1. **Explainers.** The cheapest and most useful. Five to start: present-tense agreement, past tense with ־תִּי, "going to" plus a verb, אֶפְשָׁר plus a verb, prepositions with endings (לִי, לְךָ). Lessons link to them.
2. **Roots hub.** Read-only pages from `lexemes.root`, plus a link from each word card to its root. First fill roots for the words already in units.
3. **Say-it-aloud step.** English cue, say it, model plays, self-check. `shadow` is the nearest existing step.
4. **Navigation around the six sections**, showing only sections that have content.
5. **My stuff:** verb tables and the mistakes list, filled from class data.
6. **Isaiah**, one verse at a time. Mostly writing, so it runs in parallel.
7. **Scripted dialogues with a side to play**, then branching role-plays. The AI partner comes last.

## Open questions

- **The class spreadsheet.** It is not in this repo. The path order, roots, verb tables and mistakes all assume it. Where does it live, and what columns does it have?
- **Worksheets are someone else's work.** For a site others use, write our own material in the same structure and order. Do not republish the sheets unless the author agrees.
- **Root coverage.** How many lexemes have a root today? A roots hub with few roots looks empty.
- **Isaiah text and gloss source.** The glosses and the "manner of the Jews" layer are authored content, and each verse is real work.

## Not in scope here

Speech recognition, an AI conversation partner, and payments or accounts for other learners.
