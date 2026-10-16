# Lesson plan schema

A plan is one JSON object, UTF-8. Known `schemaVersion` values are `1.0.0`, `1.1.0`, and `1.2.0` (see ADR-0009 §7 and ADR-0014). Unknown properties fail validation. An unknown `schemaVersion` is rejected.

| | |
| --- | --- |
| Decision | `docs/decisions/ADR-0003-course-and-lesson-plan-language.md` |
| Machine schema | `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` |
| Samples | `docs/decisions/ADR-0003-assets/samples/*.plan.json` |

This file points at that decision. It does not restate validator caps. The server still enforces them.

## Id paths

Address by id path, not by array index (ADR-0003 §4):

```text
units/{unitId}
units/{unitId}/lessons/{lessonId}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}/words/{id}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}/paragraphs/{id}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}/lines/{id}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}/questions/{id}
```

`read_part` with an empty path returns the whole plan and its revision. `set_field` names one field under a path (`title`, `meaning`, `alt`, `newWords`, and the other names in the machine schema).

## Revision

`revision` is an integer on the plan, starting at 1. Every successful mutation increments it by 1. Sends of `replace_part`, `insert_block`, `move_block`, `remove_block`, `set_field`, and `publish_plan` include `expectedRevision` from the last read. A mismatch returns `revision_conflict`, leaves the document unchanged, and includes the stored revision. Read again. Do not merge.

## Blocks

Block `type` includes the ADR-0003 core blocks plus ADR-0009 §7 and ADR-0014 types. At `schemaVersion` `1.2.0`, `exercise` and `assignment` are available; `conjugation-table` may use `tense: future`. What each type is for is ADR-0003 §2 and ADR-0014.

### `assignment` block (`schemaVersion` `1.2.0`)

| Field | Required | Notes |
| --- | --- | --- |
| `id` | yes | `entryId` slug |
| `type` | yes | `"assignment"` |
| `prompt` | yes | `text300` |
| `minSentences` | yes | integer 1–12 |
| `useWords` | no | 1–12 entries, each `text80` |
| `homework` | no | 1–6 lines, each `text200` |

Never scored. Stores no learner answer.

### `conjugation-table` · `tense`

| `tense` | `schemaVersion` | Cell codes |
| --- | --- | --- |
| `present` | 1.1.0+ | `ms`, `fs`, `mp`, `fp` |
| `past` | 1.1.0+ | nine person codes (`1s` … `3p`) |
| `future` | **1.2.0 only** | same nine person codes as past |

Imperative is not a table tense.

### `exercise` block (`schemaVersion` `1.2.0`)

| Field | Required | Notes |
| --- | --- | --- |
| `id` | yes | `entryId` slug |
| `type` | yes | `"exercise"` |
| `kind` | yes | `choose` \| `true-false` \| `unscramble` \| `translate` \| `dictation` |
| `heading` | no | `text120` |
| `items` | yes | 1–12 items; shape depends on `kind` (ADR-0014) |

Per-item fields by `kind` (each item has an `id` slug):

| `kind` | Required | Optional |
| --- | --- | --- |
| `choose` | `prompt`, `choices` (2–5), `answer` (must equal one choice) | `why` |
| `true-false` | `statement`, `answer` (boolean) | `why` |
| `unscramble` | `words` (3–10) | `english`, `alt` (up to 3 alternate orders, same multiset, not identical to `words`) |
| `translate` | `english`, `answer` (unpointed) | `accept` (unpointed alternates, up to 8) |
| `dictation` | `audioPath`, `answer` (unpointed) | `accept` (unpointed alternates, up to 8) |

A picture is allowed only on an introduction word:

```json
{ "mediaId": "coffee-cup", "alt": "coffee" }
```

`mediaId` is an opaque slug, not a URL or a filename. `alt` is required. `meaning` stays on the word whether or not an image is present.

## Audio paths

`audioPath` and `sentenceAudioPath` are relative `.mp3` paths when they already exist. Copy them unchanged. Do not invent a path, and do not write an absolute, scheme-bearing, or protocol-relative path. A missing clip stays absent.

Before any regeneration, run `npm run audio:inventory`. After it, run `npm run audio:verify`.

## Validator ids

N1–N12 are ADR-0003 §6. N13–N19 are ADR-0009 §7. N20–N23 are ADR-0014 (`exercise`, `assignment`, and `topic-conversation` exercise count). Cite the id. Do not copy the numeric caps into this skill.

N1 warns when `migration.legacy` is true and blocks publish otherwise. Do not delete words from `my-morning`, `anal-sex-vocab`, or `shai-date-indirect-speech` to satisfy N1.

## What never goes in the file

No account name (the sample id is `example-learner`), no learner profile, no answer text, no transcript, and no secret. The profile loop is in [SKILL.md](SKILL.md).

## Legacy worksheet schema

`upsert_lesson` still writes `window.WORKSHEET` in `worksheets/<slug>/worksheet.js` for a lesson that has not moved. It does not accept a plan document. [example.md](example.md) is this shape.

| Field | Required | Notes |
| --- | --- | --- |
| `id` | yes | Same as the folder slug |
| `title` | yes | Shelf and header |
| `level` | recommended | `basic`, `intermediate`, or `advanced` |
| `subtitle` | no | Shown under the title |
| `activities` | no | Ordered `{ type, ...fields }`. Omit it and the player builds a default from the sections that exist |
| `vocab.items[]` | often | `word`, `meaning`, `sound`, `trap`; optional `sentence`, `from`. Do not set `audio` or `sentenceAudio` |
| `fill` | no | `bank[]`, `sentences[]` with `textPre`, `textPost`, `answer`, `english` |
| `grammar` | no | One `rules[]` entry, `reminders[]`, `items[]` with `choices`, `answer`, `pointed`. Do not set `audio` |
| `reading` | no | Pointed `paragraphs`, optional `questions`, `retell`, optional `glosses`. Do not set `reading.audio` |
| `askFriend.questions[]` | no | `question`, `pointed`, optional `youFeminine`. Do not set `audio` |

`reading.glosses` is an optional map of story token to English gloss. A value that is not a string-to-string object is a validation error. A gloss key missing from the story is a warning.

Worksheet warnings (recycle, encounters, coverage, glosses) come back from `upsert_lesson`. They are not the plan rules N1–N12.

Activity types come from `list_activity_types`. Do not edit `index.html`.

Catalog row: `{ slug, title, subtitle, blurb, level }`.
