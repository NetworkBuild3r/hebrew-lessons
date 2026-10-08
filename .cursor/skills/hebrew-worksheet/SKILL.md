---
name: hebrew-worksheet
description: >-
  Builds a published Hebrew practice lesson for this project: a worksheet folder
  plus a catalog entry the shared player can open and repeat. Use when the user
  asks to create, add, or publish a Hebrew worksheet, lesson, or דף עבודה, or to
  turn class notes or a transcript into a lesson.
---

# Hebrew worksheet

## Path

Prefer the MCP tools on the `hebrew-lessons` server:

1. `list_activity_types` — pick the path
2. `list_words` — recycle 3 to 5 earlier words
3. `list_lessons` / `get_lesson` — avoid clobbering
4. `upsert_lesson` — write and validate

Fallback without MCP: copy `worksheets/_template` to `worksheets/<slug>/`, edit only `worksheet.js`, add a catalog row, then run:

```
node .cursor/skills/hebrew-worksheet/scripts/validate.js <slug>
```

## From a class call

1. Pull out one short story (the day's meaning).
2. Pick 8 to 12 words that appear in that story.
3. State **one** transparent pattern; extras go in `grammar.reminders`.
4. Bring back 3 to 5 words from earlier lessons (`from`).
5. Choose `level`: `basic` | `intermediate` | `advanced`.
6. Build an `activities` path from `list_activity_types`. Keep more meaning steps (story, pattern, ask-friend, retell, words) than pure drills.
7. Never put the transcript, the learner's name, or class chat into the lesson file.

## Lesson shape

`window.WORKSHEET` in `worksheets/<slug>/worksheet.js`:

- `id` must equal the folder slug
- `level` sets choice count, bank openness, and quiz timing
- `activities` is an ordered list of `{ type, ... }`. Omit it and the player builds a default from the sections that exist
- Content pools: `vocab`, `reading`, `grammar`, `fill`, `askFriend`

See [schema.md](schema.md) and [example.md](example.md).

## Rules

- Story first: pointed Hebrew, new words as `{ word: "..." }`, `reading.retell` cues
- Exactly one `grammar.rules` entry
- English beside Hebrew in learner-facing notes (`lede`, glosses, traps)
- Omit audio fields when writing a lesson — `upsert_lesson` generates ElevenLabs clips automatically (words cached forever under `_audio/words/`, sentences/story/grammar regenerate when text changes). Use `generate_lesson_audio` to backfill. Computer voice is fallback only
- Vowels on by default; learner may hide them
- Do **not**: root/binyan speed drills, forced vowel fading, accounts, multi-user features
- Do **not** edit a lesson's `index.html`
- Do **not** put the API key in lesson files or the Docker image

## Progress

Answers and the current step save automatically in the browser (`localStorage`). There are no Save / Load JSON buttons.
