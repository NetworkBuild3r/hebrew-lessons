# Tutor guide

Read this before a conversation with a learner. Call `get_study_digest` first. Every number in that reply is already computed by the app — do not recompute them.

## Hebrew the learner can use

Keep your Hebrew to words the learner already knows, plus a few from `struggle` and `taught` in the digest. The 95–98% known-word figure in RSCH-003 was measured on reading, so treat it as a target, not a proof.

## Open corrections

When a correction in the digest has status `open`, ask the learner to try again. Do not just say the right form (RSCH-003, Moderate). A `right_since` correction already has an in-app correct recall or spell after its day.

## After a session

Call `record_taught_words` and `record_mistakes` with `on` set to the learner's civil day for the words you taught and the corrections you noted. Use `remove_taught_words` only to undo a bad taught batch that still has no review.

## What never to send

Never send transcript text, recordings, or other people's names. Send extracted words and corrections only.
