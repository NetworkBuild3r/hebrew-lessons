# Dialogue: two voices, one female and one male

INIT-015 / SPEC-026. The authoring skill (SPEC-022) and the MCP tools (SPEC-021) read this page.

## When to use it

Any content that is a conversation between people is a `dialogue`, not a `story`. A `story` stays one narrator. Narration and single words keep the default voice, which is the female voice.

## The shape

A `dialogue` activity has `lines` and no `text`:

```json
{
  "type": "dialogue",
  "lines": [
    { "speaker": "a", "text": "<pointed Hebrew, NFC>" },
    { "speaker": "b", "text": "<pointed Hebrew, NFC>" }
  ]
}
```

- Two speakers only. Speaker `a` is the **female** voice. Speaker `b` is the **male** voice. They are fixed for the whole unit, so a learner can follow who is talking. The player shows the names Dana (a) and Avi (b), and Avi's lines sit at the other end of the column.
- 2 to 8 lines, in spoken order. Both speakers must speak at least once.
- The `speaker` is exactly `a` or `b`. A voice id is never part of a unit or a request.

## What the validator checks

Each line is judged as its own passage, the way a story is. An issue points at the line, for example `/lessons/0/activities/2/lines/1/text`.

| Rule | Blocks when |
| --- | --- |
| `unit.dialogue_lines` | fewer than 2 or more than 8 lines; a dialogue carries `text`; an activity that is not a dialogue carries `lines` |
| `unit.dialogue_speakers` | a speaker is not `a` or `b`; speaker `a` or `b` has no line |
| `schema.required` | a line has no text |
| `audio.missing` | a line has no clip **in its own speaker's voice**. A clip in the other voice does not count. |
| `story.unpointed`, `story.unmapped` | a line has an unpointed word or a word that is not in the lexicon |
| `brief.coverage`, `brief.text_too_long` | a line's known-word share is under the tier figure, or a line has more tokens than the tier allows. Both are per line. |
| `brief.new_min_uses`, `brief.new_intro_before_test`, `brief.struggle_reuse`, `brief.gloss_out_of_brief` | the words of every line count, exactly as a story's words do. A dialogue is not a way round the brief. |
| `unit.no_markup` | a line's `speaker` or `text` has `<`, `>`, or `javascript:` |

A dialogue is an exposure step: it can introduce a new word before a checked step. It trains the **read** skill.

## Making the clips: `generate_audio`

Input (all optional, but at least one of the three):

| Field | Meaning |
| --- | --- |
| `texts` | pointed Hebrew in the default (female) voice. A text is skipped when it already has a clip in any voice. |
| `lines` | `[{ "text": "...", "speaker": "a" \| "b" }]`. `speaker` may be left out for the default voice. Anything else is refused before any work is done. A line is skipped only when it has a clip **in that speaker's voice**, so the same Hebrew can have one clip per voice. |
| `unitId` | the id from `put_unit`. Adds the unit's Hebrew activity texts, and each dialogue line with its speaker. |

Each result has `text`, `textNfc`, `speaker` (when set), `status` (`generated`, `skipped`, or `failed`), `audioId`, and `error`.

- At most **40** texts and lines per call, counting those added by `unitId`.
- A call stops starting new clips after about **22 seconds**, because the server's write timeout is 30 seconds and a reply written later is lost. Texts it did not reach come back `failed` with `not attempted: the time budget of one call was used up; call again`. Call again: stored clips are skipped, so it continues where it stopped. Ten clips in a call is the safe size.
- If `ELEVENLABS_VOICE_ID_MALE` is not set, each speaker `b` line comes back `failed` with an error that names that setting. Speaker `a` lines are still made. Nothing is ever made in the other voice.

## Settings

| Variable | Meaning |
| --- | --- |
| `ELEVENLABS_VOICE_ID` | the female voice, speaker `a`, and the default for narration and every clip made without a speaker. Unchanged. |
| `ELEVENLABS_VOICE_ID_MALE` | the male voice, speaker `b`. No default. Read from the environment only. It must differ from the female voice. |

A voice id is checked to be a single URL path segment (letters, digits, `-`, `_`, at most 64) before it is used. Neither id is logged.
