---
name: hebrew-worksheet
description: >-
  Builds a published Hebrew practice lesson for this project: a worksheet folder
  plus a catalog entry the shared player can open and repeat. Use when the user
  asks to create, add, or publish a Hebrew worksheet, lesson, or דף עבודה, or
  to turn class notes or a transcript into a lesson.
---

# Hebrew worksheet

Build a lesson the existing player can open. Do not add accounts, a new page per lesson, a paid voice, or a second player.

## Build

1. Copy `worksheets/_template` to `worksheets/<slug>/`. Keep that folder's `index.html` unchanged.
2. Replace `worksheet.js`. Set `id` to the slug. Leave out a section to drop its tab.
3. Add one object to `worksheets/catalog.js`: `slug`, `title`, `subtitle`, `blurb`.
4. Run `node .cursor/skills/hebrew-worksheet/scripts/validate.js <slug>` and fix every error. Read every `WARN` line and fix those too.
5. Open `index.html` at the repo root and confirm the new card is there. Open the lesson and confirm the tabs you included render.

When the `hebrew-lessons` MCP is connected, use it for steps 1–4. Call `list_lessons` and `list_words` first. Call `get_lesson` before changing a lesson that already exists. Call `upsert_lesson` with the slug and the worksheet object. The tool writes `worksheet.js`, copies the lesson shell if the folder is new, updates `catalog.js`, and runs the validator. A failed check leaves the previous files in place. Warnings come back in the reply; fix them and call it again.

`<slug>` is lowercase, hyphenated, and matches the folder name. Do not commit unless the user asks.

Field shapes are in [schema.md](schema.md). Filled examples are in [example.md](example.md).

## What to put in the lesson

Take the words, one pattern, the story, and the speaking questions from the user's notes. Do not invent a second grammar topic. The research behind these rules is RSCH-001 in `sdd/research/`.

- **The story is most of the lesson.** About three quarters of a round is the story and speaking. The word list and one pattern are the short deliberate slice. Write the story with vowel points. Wrap each new word in `{ word: '...' }`. Removing the points must give the plain story. Add `reading.retell.cues` (pointed words for the story beats) for the retell on round 2.
- **One pattern.** `grammar.rules` holds one transparent, one-step rule, and 2 to 4 items that practise only that rule. Anything else goes in `grammar.reminders` as a one-liner. Complex structure belongs in the story, where the learner only has to notice it. Vary which choice is correct. Give each item a `pointed` correct sentence.
- **Words.** Hebrew is what the learner reads. `meaning` is the English gloss. `sound` is the stressed pronunciation (`ET-za`). `trap` is one line on the mistake English speakers make with that word. `sound` and `translit` stay hidden until the learner asks; `trap` is always visible. `word` and `sentence` both carry vowel points.
- **Bring words back.** When earlier lessons exist, bring back 3 to 5 of their words: copy the item from `list_words`, give it a new `id`, and set `from` to its lesson slug. Use them in the story and the fill sentences.
- **Fill sentences.** `english` uses `____` and must not contain the answer's English meaning. `gloss` explains surrounding words only. The word bank includes every answer once, plus one extra word that is not an answer.
- **Speaking questions.** `pointed` is the question with vowel points in one gender form. `question` is the same line without vowels. The learner says the answer out loud, then writes it. Each question has a record button so the learner can hear themselves next to the model.
- **Human audio.** If a person recorded the lesson, put the files in `worksheets/<slug>/audio/` and set `audio` or `sentenceAudio`. The computer voice is a rough guide for the learner, not a pronunciation model. Do not add another voice service.
- **English beside the Hebrew.** In notes, ledes, and buttons the learner reads, put the English in brackets after the Hebrew: `בחרו את המשפט הנכון (Choose the right sentence)`. The learner is an adult English speaker; the English keeps the instructions understood without hiding the Hebrew.
- **Vowels.** They stay on by default. The learner has a button to hide them. Do not build a mode that fades or strips them for the learner.

The player already hides English on round 2, closes the word bank until it is tried, asks "knew it / missed it" on each word, puts missed words first on the next round, suggests a day between rounds, and clears answers when the learner presses **עוד סיבוב**. Do not reimplement any of that.

## Do not

- Edit `shared/worksheet.js` or `shared/worksheet.css` unless the user asks for a player change.
- Edit a lesson's `index.html`.
- Fetch a JSON file. These pages open as `file://`. The lesson is `window.WORKSHEET` in `worksheet.js`.
- Add drills on roots or verb patterns (binyanim) as a way to go faster. No published trial shows one.
- Put a learner's name, email, or a class transcript into a worksheet.
