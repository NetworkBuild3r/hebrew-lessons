# ADR-0027: Numbers you can trust

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-11-06 |
| **Authored by** | INIT-032/SPEC-001 |
| **Initiative** | INIT-032 (numbers-you-can-trust), SPEC-001 |
| **Extends** | [ADR-0020](ADR-0020-lesson-word-face-and-speed.md) (speed grading), [ADR-0022](ADR-0022-learner-day-and-outside-teaching.md) (learner-written data), [ADR-0023](ADR-0023-review-that-moves-on.md) D4 (implied credit) |
| **Supersedes** | — |
| **Superseded by** | — |

## Summary

Each learner number has one meaning, shared by the page and the AI. "Words you know" is known or solid in at least one skill, with the per-skill counts beside it. A placement run estimates only the skill it tested. Spelling is writing the Hebrew from memory, from English or from sound, and a correct typed answer credits it. A value called current comes from the live model and carries `asOf`. Retention is one 30-day share and names its window and date. The device zone is stored when none is set, and a stated zone wins.

## Context

[ADR-0020](ADR-0020-lesson-word-face-and-speed.md) holds the speed rule for checked answers. [ADR-0022](ADR-0022-learner-day-and-outside-teaching.md) holds the learner day: `users.time_zone`, or the app zone when that column is empty, and UTC is never a fallback. [ADR-0023](ADR-0023-review-that-moves-on.md) D4 schedules an implied review as Good only. None of those records says what "words you know" counts, which skill a placement band may estimate, or how a typed Hebrew answer meets spelling.

INIT-032 read the live learner on 2026-11-06 (code at `d064144`). The same learner produced different numbers for one fact:

| What the surface showed | What the code did | Where |
| --- | --- | --- |
| Progress about 0 words known | The headline counts a word only when every practiced skill is known or solid. `StateOf` / `Overall` is the weakest skill, so 18 reading words that are still weak elsewhere count as 0. The AI receives that 0 in `get_context.model.words`. | `fillOverview`; `StateOf` / `Overall` |
| Home 55 known words | Read cards whose latest rating is Good or Easy. | `knownLexemeUnion` |
| Digest 0 known in every skill | `digestSkills` reads the same-day snapshot. The daily job writes yesterday, so that row is absent and every skill reads 0. | `digestSkills` |
| Listening level 2, about 522 words, and a spelling estimate near 508 | The run tested listening only. `estimateKnown` applies that one `PlacementBands` table to every skill, including spelling, which has no placement evidence. | `estimateKnown`; `PlacementBands` |
| A word spelled right, spelling still 0 | `type_it` is stored as recall. Nothing creates a spelling card, so dictation is never dealt. Spelling attempts and spelling cards are both 0. | no `SkillSpell` card creation |
| Retention figures that disagree | At least four shares, different windows, different days: digest snapshots, the live model, `get_progress.retention`, and the weekly report. None names its window or its day. | `DueRetention`; `digestSkills` |
| Study day an hour off the device | `users.time_zone` is NULL, so the app zone is the day. | `Resolver.For` |

## Decision

**D1 · Words you know = known or solid in at least one skill.** The headline counts a lemma that is known or solid in at least one of `listen`, `read`, `recall`, and `spell`. The per-skill known and solid counts sit beside it. A word you can read but cannot yet hear still counts, and the skill rows show what is missing. Home, Progress, and the AI share that count (`knownAnySkill`). The strict count stays available: `StateOf` / `Overall` and `words` remain known in every practiced skill, and they are not the headline. The read-card rule (latest rating Good or Easy, `knownLexemeUnion`) stays the reading set for the lesson brief and `knownLexemes`. It is the reading row, not the headline. Rejected: a headline of "known in every skill". It hid real progress and stayed at 0.

**D2 · Placement estimates only the skill it tested.** `listen`, `read`, and `recall` each take the newest finished run that answered at least one item of that skill. An unfinished newest run is ignored. Letters and typing items are left out of those band tallies. A listening run sets the listening band rates. Reading uses the newest run that tested reading. A v2 run (no `placement_v3_runs` row) is self-rated read only; its bands map to `read`. A skill no run has tested has no entry and gets an estimate equal to its own known and solid words, with `estimateSource: "evidence"`, and says it was not tested by placement. Spell never has a placement entry. A skill with bands is estimated by `estimateKnown` on that skill's bands only, with `estimateSource: "placement"` and `placementRunAt` set to that run. Rejected: one placement part standing in for four skills. `estimateKnown` applied one `PlacementBands` table to listening, reading, recall, and spelling, and spelling showed an estimate with no spelling evidence.

**D3 · Spelling is writing the Hebrew from memory.** The prompt is English (`type_it`) or sound (dictation). This matches `SkillSpell` ("a word spelled from memory"). A correct `type_it` answer credits spelling: it writes a `spell` attempt and, when the letters gate is open, creates the spelling card if there is none, then schedules that card. A near miss (one letter off) is spelling Hard and still logs the letter pair. A wrong answer or "don't know" writes no spelling attempt and no spelling review. When the letters gate is closed, a correct or near-miss typed answer stores the spelling attempt and does not create a card. The spelling credit is an implied review, so accuracy counts the answer once. This extends [ADR-0023](ADR-0023-review-that-moves-on.md) D4 by reference: implied reviews were Good only, and typed spelling may also be Hard. [ADR-0023](ADR-0023-review-that-moves-on.md) is not edited in place. `Recorder.Implied` stays Good-only; spelling uses its own helper. Dictation reviews those cards. Past typed answers are back-filled once: a correct answer as Good, a one-letter miss as Hard, and a wrong or don't-know answer is skipped. [ADR-0020](ADR-0020-lesson-word-face-and-speed.md) still grades a checked exact answer by speed. The one-letter spelling Hard is that typed grade, not a new speed band. Rejected: spelling as dictation only. The row stayed empty, because nothing created a spelling card and dictation was never dealt.

**D4 · "Now" is the live model. Snapshots are history.** Any value the AI or a page calls current comes from the live model at request time and carries `asOf`. Per-skill known and solid counts, including the digest's `knownSolidNow`, come from that live model rather than a missing same-day snapshot. A snapshot series names its last day, and a chart says "Through" that day. Stored snapshots are not rewritten. The estimate line changes on the first day the new rule writes. Rejected: reading the same-day snapshot for a live count. `digestSkills` did that, the daily job had not written the row, and every skill read 0.
> Superseded in part by ADR-0028 D3: snapshots are rebuilt as of each day under the current rules.

**D5 · Retention has one definition and says it.** Retention is the share of due reviews (`elapsed_days >= 1`, not implied) rated above Again, over 30 days. Every payload gives `window` and `asOf`. The 30-day share uses window `30d due reviews`. The weekly report's number uses window `week` and is labeled "this week". `DueRetention` is that share. Rejected: several unlabeled shares, each with its own window and day.

**D6 · The device zone is an observation, and a stated zone wins.** When `users.time_zone` is NULL, the first valid browser zone is stored. `set_time_zone` overwrites it, and the browser never overwrites a zone that is already stored. A missing or invalid browser zone stores nothing and leaves the app zone in place. Nothing defaults to UTC. A device that reports UTC is stored as UTC, because the device said so. This is not a new column. The write, when a later spec adds it, is `POST /_hx/zone` for the session learner (not a user id in the body), behind the existing `/_hx/` CSRF and Origin gate, accepting only a zone `studyday.ParseZone` accepts, and `UPDATE users SET time_zone = ? WHERE id = ? AND time_zone IS NULL`. [ADR-0022](ADR-0022-learner-day-and-outside-teaching.md) is otherwise unchanged: the app zone still fills a missing zone for the study day, and UTC is still never invented for that day. Rejected: leaving `users.time_zone` NULL and treating the app zone as the learner's device, and rejected: substituting UTC when the browser zone is missing.

## Consequences

- North Star §4.1, §4.2, §4.3, §4.4, §5.2, the study day in §8, and the Progress overview in §9 follow this record.
- [ADR-0020](ADR-0020-lesson-word-face-and-speed.md)'s speed rule for checked answers is unchanged.
- [ADR-0022](ADR-0022-learner-day-and-outside-teaching.md) still refuses a UTC fallback for a missing zone. D6 adds only the first stored observation and the rule that a stated zone wins.
- [ADR-0023](ADR-0023-review-that-moves-on.md) D4's "Good only" is amended by reference, not edited in place. `Recorder.Implied` stays Good-only. Typed spelling is the only implied review that may be Hard, and it uses its own helper.
- Later INIT-032 specs implement per-skill placement tallies, the shared headline and per-skill estimates, typed spelling credit, live labeled numbers, the pages, the device-zone write, the one-time spelling back-fill, and the agreement check against this record.
