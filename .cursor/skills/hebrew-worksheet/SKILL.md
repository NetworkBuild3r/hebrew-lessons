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
4. `list_users` — usernames for assignment (requires `MCP_AUTH_TOKEN`)
5. `upsert_lesson` — write and validate; pass optional `visibility`
6. `set_lesson_visibility` — share or hide after the fact (requires `MCP_AUTH_TOKEN`)

Fallback without MCP: copy `worksheets/_template` to `worksheets/<slug>/`, edit only `worksheet.js`, add a catalog row, then run:

```
node .cursor/skills/hebrew-worksheet/scripts/validate.js <slug>
```

Lessons can also be edited in the browser admin Lessons editor (sign in as admin).

## Visibility (required for learners to see a lesson)

New lessons are **hidden until assigned**. A lesson created with no assignment stays hidden.

- Pass `visibility` on `upsert_lesson`: `"everyone"` or `{ "users": ["username", ...] }`
- Or call `set_lesson_visibility` with `"everyone"`, `"hidden"`, or `{ "users": ["username", ...] }`
- Use `list_users` to find active usernames and display names
- Assignment tools refuse if `MCP_AUTH_TOKEN` is not set
- You can also set hidden / everyone / selected in the admin Lessons tab

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
- Run `npm run audio:inventory` before regenerating audio; run `npm run audio:verify` afterward. Archived clips live at `<lesson>/audio/_archive/<stem>.<12hex>.mp3`
- Vowels on by default; learner may hide them
- Do **not**: root/binyan speed drills, forced vowel fading
- Do **not** edit a lesson's `index.html`
- Do **not** put the API key in lesson files or the Docker image
- Do **not** put learner names or transcripts in lesson files

## What the research says

Short rules from `RSCH-003` (and ADR-0001 / validator thresholds). Evidence levels are `RSCH-003` bands. Follow them when authoring; the validator warns (does not block) when you miss the thresholds in [schema.md](schema.md).

- **Recycle known words** — bring back at least 3 earlier words (`from` or catalog/index reuse). More encounters help (**Moderate**); spaced recall across days is strongest (**High**).
- **Encounters ≥ 2** — each *new* vocab word must appear at least twice across story, sentences, and activities (**Moderate**).
- **Coverage ~95%+** — keep the story mostly known words (about 95%+ estimate; 98% is optimal). Reading threshold is **Low**; the validator warns below ~90%.
- **`reading.glosses`** — supply English glosses for story tokens outside the vocab list (first-language glosses help, **Moderate**). See [schema.md](schema.md).
- **Short grammar** — exactly one pattern, then meaningful practice (**Moderate**).
- **No learning-style variants** — do not quiz or tailor to a "learning style" (**Moderate** against matching).
- **No root/binyan speed drills** — project constraint (ADR-0001 / REQ-009); not a `RSCH-003` Do.
- **No forced vowel fading** — vowels stay on by default; learner may hide them (ADR-0001 / REQ-009); not a `RSCH-003` Do.

## Progress

Answers and the current step save on the server for the signed-in learner. Admins can review that history in the product UI. MCP tools cannot read progress or answer history. Never write learner names or transcripts into lesson files.
