# Hebrew worksheets

Practice pages for דרך עברית. Each lesson is a folder of content. One shared player draws the tabs, speech, checking, and saved answers.

Open `index.html` in a browser.

## Add a lesson

1. Copy `worksheets/_template` to `worksheets/<new-slug>`.
2. Edit only `worksheet.js` in the new folder. Give it a new `id`.
3. Add one object to `worksheets/catalog.js`.

Leave a section out and its tab disappears. A lesson can be vocab only, or vocab plus reading, or the full set.

`index.html` inside a lesson folder is the same file every time. Behavior lives in `shared/worksheet.js`. Appearance lives in `shared/worksheet.css`.

## What a lesson can include

| Key | Tab |
| --- | --- |
| `vocab.items` | Word cards, and a matching game when the list is not empty |
| `fill` | Word-bank sentences. `answer` is the blank. `prefix` sticks a letter such as ב to the blank. Put one extra word in `bank` if you want a distractor. |
| `grammar` | A short rule list plus multiple choice. `answer` is the index of the correct choice, starting at 0. |
| `reading` | Paragraphs and optional comprehension questions. A string is a plain paragraph. An array can mark `{ reported: "..." }` or `{ direct: "..." }`. |
| `askFriend` | Interview questions. `speak` is what the audio button reads when the written question uses slash forms such as את/ה. |

Answers are stored in the browser under `hebrew-worksheet:<id>`. The Save button downloads that same progress as JSON.

Speech uses the computer’s Hebrew voice at 85% speed. If Hebrew is not installed, the audio will sound wrong.
