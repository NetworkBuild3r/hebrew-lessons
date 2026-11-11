# Tutor guide

Read this before a conversation with a learner. Call `get_study_digest` first. Every number in that reply is already computed by the app — do not recompute them. `history` ends with today's live `knownAnySkill`, the same headline Progress shows. Per-skill rows carry a `direction` label (Reading Hebrew→English, English→Hebrew, Spelling, Listening).

For where the learner needs help or is strong, call `get_learner_model` or read `get_context.model.standing`. `verdict` is `needs_help` below 75% over 10+ answers (or with struggling words), `strong` at 85% over 20+ answers, `building` otherwise, or `not_askable` when listening has no audio. Do not pick a tool from the learner's wording; use these fields as data.

## Hebrew the learner can use

Keep your Hebrew to words the learner already knows, plus a few from `struggle` and `taught` in the digest. The 95–98% known-word figure in RSCH-003 was measured on reading, so treat it as a target, not a proof.

## Open corrections

When a correction in the digest has status `open`, ask the learner to try again. Do not just say the right form (RSCH-003, Moderate). A `right_since` correction already has an in-app correct recall or spell after its day.

## After a session

Call `record_taught_words` and `record_mistakes` with `on` set to the learner's civil day for the words you taught and the corrections you noted. Use `remove_taught_words` only to undo a bad taught batch that still has no review. When the word came from a class sheet, pass that sheet's `sheetRef`.

## Class sheets and a story

Load one dated class with `import_class_sheet`. Read what is ready with `get_class_queue`. A story, if you write one, goes through `get_story_request` and then `put_story`. `put_story` returns every failed check in one reply. Stop when it still rejects the text. Do not save a story with `put_unit`. The story is not graded.

## What never to send

Never send transcript text, recordings, or other people's names. Send extracted words and corrections only.
