# ADR-0030: Typed spelling counts

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-11-07 |
| **Authored by** | INIT-035/SPEC-001 |
| **Initiative** | INIT-035 (spelling-counts), SPEC-001 |
| **Extends** | [ADR-0027](ADR-0027-numbers-you-can-trust.md) (numbers you can trust), [ADR-0028](ADR-0028-where-you-stand.md) (where you stand) |
| **Amends** | [ADR-0027](ADR-0027-numbers-you-can-trust.md) D3 by reference |
| **Supersedes** | — |
| **Superseded by** | — |

## Summary

A successful review on a spelling card counts toward knowing that word, whether the answer came from a direct spelling question or from typed English → Hebrew credit. `implied` still means accuracy counts the answer once. Only typed spelling and its one-time backfill write implied rows onto spelling cards. History rebuilds under rule version 3.

## Context

[ADR-0027](ADR-0027-numbers-you-can-trust.md) D3 says spelling is writing the Hebrew from memory, and a correct typed answer credits it as an implied review so accuracy counts once. [ADR-0028](ADR-0028-where-you-stand.md) names Spelling as a direction and rebuilds history under `rule_version` 2. Neither record says whether an implied spelling success counts toward knowing the word.

ASMT-003 measured the gap. A word is known only after 3 successful reviews; `LearningSuccesses` in `internal/learner/state.go` is that bar of 3. Before this record, `LearnerEvidence` in `internal/db/learner.go` counted those successes only from `review_log` rows with `implied = 0`. Typed English → Hebrew credit to spelling is written implied by `spellImplied` in `internal/evidence/spell.go`. That review scheduled the card and never raised the success count. In production, 40 of 44 successful spelling reviews were implied. Spelling showed 0 known while five words passed every other part of the known rule.

## Decision

**D1 · A successful spelling review counts toward knowing the word.** A successful review on a spelling card raises that word's spelling `Successes`, so the state can move from learning to known. `LearnerEvidence` counts a success as `rating > 1 AND (implied = 0 OR skill = 'spell')`; every other per-card count still skips implied rows. Direct spelling questions and typed English → Hebrew credit both count. Typing the Hebrew from memory is spelling ([ADR-0027](ADR-0027-numbers-you-can-trust.md) D3). Rejected: lowering `LearningSuccesses` for spelling, which hides the cause.

**D2 · `implied` keeps its accuracy meaning.** Accuracy, retention, recent ratings, struggle stats, and coach counts still skip implied rows. One typed answer still counts once. Rejected: writing typed spelling as non-implied, which double-counts accuracy.

**D3 · Only typed spelling writes implied rows onto spelling cards.** `spellImplied` and its one-time backfill are the only writers of implied rows on spelling cards. Move-up never does (`activity.Implies`). Tests enforce this. Rejected: a new `review_log` source column, which is more machinery for a case this fence already closes.

**D4 · The rule version moves to 3.** History is rebuilt under rule version 3. The audit reports days stamped under an older rule. Rejected: keeping rule version 2 and patching only the spelling rows, which leaves two rules under one stamp.

## Consequences

- [ADR-0027](ADR-0027-numbers-you-can-trust.md) D3 stays in force for what spelling is and for implied credit so accuracy counts once. This record amends it by reference: implied spelling successes count toward the word's spelling state. ADR-0027 is not edited in place.
- [ADR-0028](ADR-0028-where-you-stand.md) D3's as-of rebuild still applies. Snapshots stamped under rule version 2 are older rules; rebuild under 3.
- Speed grading is unchanged. A no-clip spelling question is the same `type_it` task as typed English → Hebrew, so it shares that median. A separate spelling median was considered and rejected.
- Later INIT-035 specs change the spelling success count, fence the implied writers, bump `jobs.RuleVersion` to 3, and rebuild history.
