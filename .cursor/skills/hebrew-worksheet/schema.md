# Worksheet fields

`window.WORKSHEET` in `worksheets/<slug>/worksheet.js`.

## Lesson

| Field | Required | Notes |
| --- | --- | --- |
| `id` | yes | Same as the folder slug. Storage key is `hebrew-worksheet:<id>`. |
| `title` | yes | Hebrew title. |
| `subtitle` | no | Shown under the title and on the shelf if the catalog repeats it. |
| `footer` | no | |
| `series` | no | Defaults to the school name. |
| `legacyStorageKey` | no | Only when an older save key must still load. |

Tab order: Vocabulary, Story, Vocab Practice, Pattern, Ask a Friend. A section that is left out has no tab.

## `vocab.items[]`

`id`, `word` (pointed), `meaning`, `sound`, `trap`, `sentence` (pointed). Optional: `translit`, `pos`, `infinitive`.

`sentence` is a Hebrew phrase that uses the word, written with vowel points so the computer voice does not guess. The button beside the word speaks `word`. The button beside the sentence speaks `sentence` and has a record button next to it.

Optional:

- `from`: slug of an earlier lesson the word comes from. The card gets a "word brought back" badge. The validator checks that the earlier lesson has this word. Bring back 3 to 5 words per lesson. `list_words` returns the items to copy.
- `audio`, `sentenceAudio`: a path inside the lesson folder, such as `audio/etza.mp3`. A human recording plays instead of the computer voice, and the computer voice is the fallback if the file cannot play. The validator checks that the file exists.

## `fill`

`bank`: Hebrew words, answers plus one distractor.

`sentences[]`: `id`, `textPre`, `textPost`, `answer`. Optional: `prefix` (a letter glued to the blank, such as `ב`), `english`, `gloss` (`[{ he, en }]`).

The checker strips nikud, so `עֵצָה` matches `עצה`. On round 2 and later the bank stays closed until the learner opens it.

## `grammar`

`tabLabel`, `heading`, `lede`, `prompt`, `rules[]` (`title`, `english`, `example`).

`rules` holds one pattern. Two or more fails the validator. Other points go in `reminders[]`, short strings that are not new rules.

`example` is a string or an array of strings and `{ strong }`, `{ reported }`, or `{ direct }`.

`items[]`: `id`, `choices` (strings), `answer` (0-based index), `why`. Lead line is `direct` or `prompt` and is English, so it has no audio. Optional `pointed`: the correct choice with vowel points. After Check, a button speaks it. Vary which index is correct.

## `reading`

`paragraphs[]`: a string, or an array of strings and `{ reported }`, `{ direct }`, `{ strong }`, `{ word }`. `reported` and `direct` can hold an array of parts, so a new word can sit inside a quoted span.

Write the story with vowel points. `{ word: 'עֵצָה' }` marks a new word from `vocab` with a yellow highlight. Removing the points from the whole story must give the plain text. The learner's vowel button strips and restores the points.

Optional: `notice`, `support` (`[{ he, en }]`), `questions[]` (`id`, `question`, `sampleAnswer`), `retell` (`{ cues: [pointed strings], seconds }`). The retell card shows from round 2. Without `cues` it uses the first six vocab words.

## `askFriend.questions[]`

`id`, `question` (unpointed), `pointed` (same line with nikud, one gender form). Optional: `youFeminine`, `gloss`, `followUp`, `audio`, `speak`.

The button speaks `pointed`. A record button sits beside it so the learner can hear themselves after the model.

## Catalog

`worksheets/catalog.js` sets `window.HEBREW_CATALOG` to an array of `{ slug, title, subtitle, blurb }`. `slug` is the folder name.
