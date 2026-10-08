# Worksheet schema

`window.WORKSHEET` fields:

| Field | Required | Notes |
| --- | --- | --- |
| `id` | yes | Same as folder slug |
| `title` | yes | Shelf + header |
| `level` | recommended | `basic` \| `intermediate` \| `advanced` |
| `subtitle` | no | Shown under the title |
| `activities` | no | Ordered path. Each item is `{ type, ...fields }` |
| `vocab.items[]` | often | `word`, `meaning`, `sound`, `trap`; optional `sentence`, `from`. Do not set `audio` / `sentenceAudio` — `upsert_lesson` fills them |
| `fill` | no | `bank[]`, `sentences[]` with `textPre`, `textPost`, `answer`, `english` |
| `grammar` | no | One `rules[]` entry, `reminders[]`, `items[]` with `choices`, `answer`, `pointed` (optional `audio` filled by generator) |
| `reading` | no | Pointed `paragraphs`, optional `questions`, `retell: { seconds, cues }`. Generator adds `reading.audio[]` (one clip per paragraph) and question `audio` |
| `askFriend.questions[]` | no | `question`, `pointed`, optional `youFeminine` (optional `audio` filled by generator) |

Word clips live under `LESSONS_DIR/_audio/words/` and are referenced as `../_audio/words/<hash>.mp3`. Lesson clips live under `<lesson>/audio/`.

## Activity types

Call MCP `list_activity_types` for the live list. Current types:

- Meaning: `words`, `story`, `pattern`, `retell`, `ask-friend`
- Drill: `match`, `fill`, `speed-quiz`, `build-sentence`, `listen-pick`, `type-it` (advanced)

## Catalog entry

`worksheets/catalog.js` (or `LESSONS_DIR/catalog.js`):

```js
{ slug, title, subtitle, blurb, level }
```
