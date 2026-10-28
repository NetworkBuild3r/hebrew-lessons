# ADR-0021: Listening cards and the word page

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-10-28 |
| **Authored by** | INIT-023 SPEC-010, recording the listening-card and word-page work of SPEC-002, SPEC-003, SPEC-004, and SPEC-007 |
| **Initiative** | INIT-023 (word-pages-and-known-words), SPEC-010 |
| **Extends** | [ADR-0018](ADR-0018-v3-adaptive-learning.md) (word × skill states and FSRS cards) |
| **Supersedes** | — |
| **Superseded by** | — |

## Summary

A lesson listening check schedules a `listen` card. Finishing a lesson that listened to a new word with audio also enqueues one. Past listen answers without a card are replayed once by the daily job. The overall word state stays the weakest unlocked core skill, so a skill with answers and no card remains `seen` and holds the word there. Progress opens on every skill, and `/words/{id}` is the page for one word (verb conjugation tables first).

## Context

Before INIT-023, a lesson `listen_pick` stored an attempt and set `SkipSchedule`, so listening never left `seen`. `enqueueNewWords` created only `read` and `recall` cards. Progress with no skill query fell through to Listen and looked empty while Read had known words. There was no per-word page with forms and audio. The North Star already said each skill has its own state and the overall state is the weakest core skill; it did not say which activity creates a listening card, or describe Progress's overview or the word page.

## Decision

**D1 · A lesson `listen_pick` schedules the `listen` card.** `recordAnswer` schedules on `recall` and on `listen_pick`. A first right answer creates the card. A first wrong answer stores the attempt only and does not seed a failure card; a miss on an existing listen card rates Again. Easy still waits until the card is in review or relearning. Rejected: leaving every `listen_pick` on `SkipSchedule`.

**D2 · Finishing a lesson enqueues a listen card for a listened new word with audio.** `enqueueNewWords` still inserts `read` and `recall` for each new word. It also inserts `listen` when this lesson has a `listen_pick` that targets that new word and the lexeme has a clip. A word with no audio gets no listen card. Rejected: creating a listen card for every new word regardless of listening or audio.

**D3 · One-time replay of past listen answers.** `evidence.ReplayListenCards` finds every learner and word with listen attempts and no listen card, replays attempts oldest first with the same scheduling rules (including "wrong first creates none"), and records completion under the `job_state` key `replay.listen.v1`. The daily job runs it once; a second run is a no-op. **The production database has not been replayed.** Rejected: leaving historical listen answers without cards forever, and rejected: replaying on every daily run.

**D4 · The overall rule stays the weakest unlocked core skill.** An answered skill with no card is `seen`. That skill holds the word's overall state at `seen` until a card exists and the skill can move on. The Progress overview with no `?skill=` shows every skill; a listening skill with answers and no card shows the explanation instead of a zero bar. Rejected: treating listen answers without a card as `learning` or as known, and rejected: weakening the overall rule so listen cannot hold a word at `seen`.

**D5 · `/words/{id}` is the word page.** A signed-in learner opens one lexicon word. A verb shows the lemma header (pointed form, clip, transliteration, meaning, POS, binyan, root), per-skill state, conjugation tables by tense with clips on forms that have audio, and up to eight lesson lines that use any of those forms. A noun shows gender, an irregular-plural chip when that flag is set, a singular/plural grid (the word, the definite form, the construct) when those forms exist, and a possessive grid; an empty construct or possessive block is omitted, and a noun with only `lexemes.plural` shows singular and plural. An adjective shows the four agreement cells. Each form plays when it has a clip. Rejected: leaving forms only as a flat list on `/me/verbs` with no audio and no tense grouping.

## Consequences

- North Star sections 4.2, 4.3, and 9 follow this record.
- Listening can leave `seen` once a card exists. Until production runs the one-time replay (`replay.listen.v1`), learners whose listen answers predate this change still show listen as answered with no card on Progress.
- A noun on `/words/{id}` shows gender, an irregular-plural chip when that flag is set, the singular/plural forms grid, and possessives when present; an adjective shows the four agreement cells. Each form with a clip plays.
- ADR-0018's overall-state rule is unchanged; this record only names how listening gets a card and how Progress and the word page surface that.
