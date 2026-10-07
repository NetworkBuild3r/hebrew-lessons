# Hebrew worksheets

Practice pages for דרך עברית. Each lesson is a folder of content. One shared player draws the tabs, speech, checking, and saved answers.

Open `index.html` in a browser. The home page is the shelf of published lessons. One person, this browser. There is no account.

## Repeat a lesson

On a lesson, **עוד סיבוב (Practice again)** clears the answers and starts the next round.

- **A day between rounds.** The lesson page and the shelf show when the next round is due. Pressing early asks you to confirm.
- **Round 1 shows meanings.** Later rounds hide the English until you ask for it, then ask *knew it / missed it*. Words you missed come first in the next round.
- **Round 2 and later** keep the word bank closed until you have tried the blank, and add a timed retell of the story.
- **Vowels** are on by default. A button hides them when you want to read without them.
- **Read aloud** on the Story tab has Pause, Resume and Stop. "Follow along" highlights the line being read and is remembered between visits.
- **Say it, then hear yourself.** Each example sentence and speaking question has a microphone button. The recording stays in the page until you reload; it is not uploaded.

The shelf lists lessons that are ready for another round first.

## Add a lesson

1. Copy `worksheets/_template` to `worksheets/<new-slug>`.
2. Edit only `worksheet.js` in the new folder. Give it a new `id`.
3. Add one object to `worksheets/catalog.js`.

Leave a section out and its tab disappears. A lesson can be vocab only, or vocab plus reading, or the full set.

`index.html` inside a lesson folder is the same file every time. Behavior lives in `shared/worksheet.js`. Appearance lives in `shared/worksheet.css`.

## Run in a container

```
docker compose up --build
```

Open `http://127.0.0.1:8080`. The container serves this folder, so a lesson written on the host shows up without rebuilding the image.

The same process listens for agents at `http://127.0.0.1:8787/mcp`.

## Agents

Cursor loads `.cursor/mcp.json`, which starts `node mcp/stdio.js` in this folder. Those tools write lessons here:

- `list_lessons`
- `get_lesson`
- `list_words` — every vocab item already published, so a new lesson can bring back 3 to 5 earlier words
- `upsert_lesson`

`upsert_lesson` copies the lesson shell, writes `worksheet.js`, updates the catalog, and runs the worksheet validator. If the check fails, the previous lesson stays as it was. Warnings, such as too few brought-back words, come back in the reply. Reload MCP servers in Cursor after the first setup so the tools appear.

## What a lesson can include

| Key | Tab |
| --- | --- |
| `vocab.items` | Word cards, and a matching game. `sound` and `trap` teach pronunciation. The transliteration stays hidden until the learner asks for it. `from` marks a word brought back from an earlier lesson. |
| `fill` | Word-bank sentences. The learner chooses a word, then clicks the blank. `english` and `gloss` explain the sentence without giving the answer. `prefix` sticks a letter such as ב to the blank. |
| `grammar` | One stated pattern, a few one-line `reminders`, and multiple choice. `answer` is the index of the correct choice, starting at 0. |
| `reading` | The story, written with vowel points. New words are marked with `{ word: "..." }`. An array can mark `{ reported: "..." }` or `{ direct: "..." }`. `retell` sets the cue words for the timed retell. Optional comprehension questions. |
| `askFriend` | Interview questions. The audio button reads `pointed`, the line with vowel points. |

Answers are stored in the browser under `hebrew-worksheet:<id>`. To back up or move progress, use **Save progress** and **Load progress** on the lesson's card on the home page. Save downloads `<id>_progress.json`; Load puts it back, and refuses a file that belongs to another lesson.

The page text pairs Hebrew with English in brackets, the way the buttons already do. Lesson authors do the same in `lede`, `gloss`, and notes.

Speech uses the computer’s Hebrew voice at 85% speed. It is a rough guide. A recording by a person is the accurate model, especially for ח and for which syllable is stressed. To add one, put the file in `worksheets/<slug>/audio/` and set `audio` (a word or question) or `sentenceAudio` (an example sentence). That button then plays the recording and falls back to the computer voice if the file cannot play.
