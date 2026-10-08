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
| `reading` | no | Pointed `paragraphs`, optional `questions`, `retell: { seconds, cues }`, optional `glosses`. Generator adds `reading.audio[]` (one clip per paragraph) and question `audio` |
| `askFriend.questions[]` | no | `question`, `pointed`, optional `youFeminine` (optional `audio` filled by generator) |

### `reading.glosses` (optional)

Object map: story token as written → English gloss. Absent or `{}` is fine. Keys should appear in the story text. Lookup order in the player: gloss map → lesson vocab meaning → audio-only (ADR-0001).

```js
reading: {
  paragraphs: [ /* pointed Hebrew */ ],
  glosses: {
    'וְקָמָה': 'and she got up',
    'מַהֵר': 'quickly'
  }
}
```

Malformed `reading.glosses` (not a string→string object) is a **validation error**. A gloss key missing from the story is a **warning**.

### Validator warnings (SPEC-008)

Non-blocking nudges from `validate.js` / `upsert_lesson` (thresholds: recycle ≥ 3, encounters ≥ 2, coverage ≥ 90%):

| Warning | When |
| --- | --- |
| Recycling | Fewer than 3 vocab words reused from earlier lessons (`from`, catalog, or vocab index). Also warns if more than 5. |
| Encounters | A new (non-recycled) vocab word appears fewer than 2 times across story / sentences / activities. |
| Coverage | Estimated known-word coverage of the story is below 90% (estimate only; prefixes not stripped). Author toward ~95%+. |
| Glosses | A `reading.glosses` key is not found in the story text. |

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
