# ADR-0028: Where you stand

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-11-06 |
| **Authored by** | INIT-033/SPEC-001 |
| **Initiative** | INIT-033 (see-where-you-stand), SPEC-001 |
| **Extends** | [ADR-0027](ADR-0027-numbers-you-can-trust.md) (numbers you can trust) |
| **Supersedes** | [ADR-0027](ADR-0027-numbers-you-can-trust.md) D4 in part ("stored snapshots are not rewritten") |
| **Superseded by** | — |

## Summary

Every Progress number means one thing and ends today. History is rebuilt as of each day under the current rules. Directions are named for what they test. Help and strong use fixed accuracy bars. Review asks only what it can check. Placement dates are per skill.

## Context

On 2026-11-06 the Progress page showed two different "words you know" figures for one learner: the headline said 36 and the chart said 15. They measure different things at different times. The 36 is live: words known or solid in any skill ([ADR-0027](ADR-0027-numbers-you-can-trust.md) D1). The chart plots one skill only, from daily snapshots that end yesterday, with no point for today and no label for which skill.

The chart history is wrong, not just old. When the daily job catches up on missed days, it computes one snapshot of the learner as they are now and writes it under every missed day. No code evaluates the learner as of a past day. Days written before this record also used older rules.

Every review feels the same. Review has asked reading and recall picks and typed recall, and zero listening or dictation questions, because 0 of 1,076 lexemes have an audio clip. Spelling's only review activity was dictation, so due spelling cards become self-graded flashcards. Nothing on the page says where the learner needs help or where they are strong.

## Decision

**D1 · One quantity per line, and one meaning per name.** "Words you know" is [ADR-0027](ADR-0027-numbers-you-can-trust.md) D1 everywhere, including the main chart. A direction's line is that direction's known and solid count. A placement estimate is a separate, labeled number. It is never drawn on an evidence line.

**D2 · Every series ends today.** Charts and the week table end with a live point computed at request time. That point equals the headline on the same page. Snapshot days are history up to yesterday.

**D3 · History is computed as of each day, under the current rules.** A day's snapshot is the model evaluated on the answers given up to the end of that day in the learner's study zone. Card stability is replayed from `review_log`. Snapshots are a derived cache stamped with `rule_version` 2, and an operator command rebuilds them. This supersedes [ADR-0027](ADR-0027-numbers-you-can-trust.md) D4's "stored snapshots are not rewritten". Regression events are not rebuilt.

**D4 · Directions are named for what they test.** `read` is "Reading (Hebrew → English)"; `recall` is "English → Hebrew"; `spell` is "Spelling"; `listen` is "Listening". Skill keys in code and in MCP do not change. MCP adds a `direction` label.

**D5 · Help and strong are deterministic.** Accuracy counts checked, non-implied review, lesson, and practice answers over 14 days. Placement answers are excluded. A direction needs help when accuracy is below 75% over at least 10 answers, or when it has struggling or regressed words. It is strong when accuracy is at least 85% over at least 20 answers. Needs-help words are a direction's struggling or regressed words, plus words missed at least twice in 14 days. They are ranked by misses, then by the most recent miss. Each carries `learner.Diagnose`'s cause and its dominant letter pair or confused word. Help and strong lists show the top 5 per direction. The thresholds are named constants in `internal/learner`.

**D6 · Review asks only what it can check, across every askable direction.** Spelling is asked by `type_it` when the word has no clip. It is graded letter by letter, and a correct answer also credits recall ([ADR-0027](ADR-0027-numbers-you-can-trust.md) D3). A skill with no fitting checked activity is not served as a self-graded card in review: listening waits for audio. Session slots are shared among askable directions in proportion to their due cards, and each due direction gets at least one slot. Within a direction, the question formats alternate. Selection is by schedule and typed facts, never by text matching.

**D7 · Placement dates are per skill.** Each placed skill reports the time of the run that supplied its bands.

## Consequences

- Progress shows one picture: today's headline matches the chart's last point, each direction is labeled, and help and strong use the bars in D5.
- The daily job and `hebrew history rebuild` write each day as of that day under `rule_version` 2. Catch-up no longer stamps today's state onto past days.
- Review serves spelling via `type_it` when there is no clip, and does not fake listening without audio.
- The AI coach reads the same standing from the same function as the page.
- [ADR-0027](ADR-0027-numbers-you-can-trust.md) D1–D3, D5, and D6 stay in force. D4's "stored snapshots are not rewritten" does not.

## Alternatives considered

- **Live point only, keep old snapshots.** Add today's live point and leave past snapshot rows as written. Rejected: catch-up already wrote today's state under missed days, and days before this record used older rules, so those points are wrong, not merely old. History has to be recomputed as of each day under the current rules (D3).
- **Drop history before Nov 6.** Delete every snapshot day before this record and start the series fresh. Rejected: as-of replay under current rules can rebuild a correct series from the answer and review log. Dropping the days would hide real progress without fixing the write path.
