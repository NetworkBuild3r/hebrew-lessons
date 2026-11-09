# ADR-0031: Known means remembered

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-11-08 |
| **Authored by** | INIT-036/SPEC-001 |
| **Initiative** | INIT-036 (known-means-remembered), SPEC-001 |
| **Extends** | [ADR-0024](ADR-0024-review-is-a-checked-test.md) (review is a checked test), [ADR-0027](ADR-0027-numbers-you-can-trust.md) (numbers you can trust), [ADR-0030](ADR-0030-typed-spelling-counts.md) (typed spelling counts) |
| **Amends** | [ADR-0027](ADR-0027-numbers-you-can-trust.md) D1 by reference (the set behind `knownLexemes` and the suggested course start) |
| **Supersedes** | [INIT-020 SPEC-002](../../sdd/initiatives/INIT-020-speaking/specs/SPEC-002-service-say-activity.md) line 31 (say self-check feeds the scheduler), for scheduling and known credit |
| **Superseded by** | — |

## Summary

A skill is known only when its checked right answers fall on at least two different study days in the learner's zone. Lesson and review answers both count. Self-reported say checks write nothing and count for nothing until speech can be checked. The same strict set feeds "Words you know", the comfortable-text tier, the validator's coverage check, `get_context.knownLexemes`, and the suggested course start. Words that pass the old same-day rule but not yet a second day show as "learned today, check tomorrow". History rebuilds under rule version 4; `review_log` is never edited.

## Context

[ADR-0024](ADR-0024-review-is-a-checked-test.md) D3 says a word and skill are asked once per sitting, and a same-session repeat is practice, not the review. [ADR-0027](ADR-0027-numbers-you-can-trust.md) D1 made the headline "known or solid in at least one skill" and kept the read-card Good/Easy set (`knownLexemeUnion`) for the lesson brief and `knownLexemes`. [ADR-0030](ADR-0030-typed-spelling-counts.md) let typed spelling's implied credit raise spelling successes and rebuilt history under rule version 3.

None of those records requires a later day before a skill is known. Today three same-day right answers, with recall chance at least 0.85 and the newest three right, can mark a skill known. Self-reported speaking checks count in that success total. That state sets the comfortable-text tier and the validator's coverage check. `knownLexemes` and the suggested course start still use one Good or Easy read answer.

[RSCH-003](../../sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md) line 18 (High): spacing out practice beats massing it. Line 173: schedule delayed recall across separate days, and count a same-session repeat as practice, not as the review. [ADR-0024](ADR-0024-review-is-a-checked-test.md) D3 already cites that checklist line for the sitting. [ASMT-003](../../sdd/assessments/ASMT-003-hebrew-web-app/ASMT-003-hebrew-web-app-assessment-report.md) falsifier 1 confirmed the same-day known mechanism in code. How often the old rule called a word known on day one in real data was never measured; no application data was read.

Operator answers for this record (2026-11-08): self-reported speaking counts toward nothing until speech can be checked; the later-day rule follows the research; the headline follows the research.

## Decision

**D1 · Known needs a later study day.** A skill is known only when its checked right answers fall on at least two different study days in the learner's zone. Lesson and review answers both count: both are retrieval, and a day apart is the delay the research asks for. Rejected: review-only later-day credit. A later lesson that brings a word back is also delayed retrieval, and lessons are where most words return.

**D2 · Checked means not self-reported.** Rows that are `peeked` or from the say self-check never count as successes, as later-day answers, or toward the newest-three rule. Typed spelling's implied credit still counts, as [ADR-0030](ADR-0030-typed-spelling-counts.md) set. Other implied credits do not. Rejected: counting self-reported say checks toward known.

**D3 · One strict set for every consumer.** The same later-day known rule feeds the headline "Words you know", the comfortable-text tier, the validator's coverage check, `get_context.knownLexemes`, and the suggested course start. This amends [ADR-0027](ADR-0027-numbers-you-can-trust.md) D1 by reference: the read-Good set for `knownLexemes` and the suggested start is replaced by this strict set. ADR-0027 is not edited in place. Rejected: keeping a looser author set beside a strict headline.

**D4 · Self-reported say counts for nothing.** A self-reported say check writes no attempt, review, or card. It supersedes [INIT-020 SPEC-002](../../sdd/initiatives/INIT-020-speaking/specs/SPEC-002-service-say-activity.md) line 31, which fed the say self-check to the scheduler. The Got it / Not yet buttons go away. Self-report returns when speech can be checked. Rejected: keeping say rows for scheduling while dropping them from known only.

**D5 · Show the gap; rebuild under rule version 4.** Words that pass the old same-day rule but have only one study day show as "learned today, check tomorrow", a count beside "Words you know". Rule version 4 rebuilds derived history from `review_log` the same way [ADR-0030](ADR-0030-typed-spelling-counts.md) did under version 3. Past say rows and peeked reviews stay in the log and stop counting under the new rule. `review_log` is never edited. Rejected: deleting old say or peeked rows, and rejected: hiding the drop in the headline without naming what is still waiting for a second day.

## Consequences

- [ADR-0024](ADR-0024-review-is-a-checked-test.md) D3 stays in force for once-per-sitting review. This record extends the same research line (RSCH-003 line 173) from the sitting to the known state.
- [ADR-0027](ADR-0027-numbers-you-can-trust.md) D1's headline shape (known or solid in at least one skill, with per-skill rows beside it) still applies once each skill uses the later-day rule. The read-Good set for `knownLexemes` and the suggested start does not.
- [ADR-0030](ADR-0030-typed-spelling-counts.md) stays in force for typed spelling's implied credit and for as-of rebuild under a stamped rule version. Snapshots stamped under rule version 3 are older rules; rebuild under 4.
- Later INIT-036 specs change the checked-success day count, bump the rule version, stop the say self-check from writing evidence, and show "learned today, check tomorrow" on Progress and Today.
- [ASMT-003](../../sdd/assessments/ASMT-003-hebrew-web-app/ASMT-003-hebrew-web-app-assessment-report.md) falsifier 1 remains the code proof of the old same-day path. Frequency of day-one "known" in production data is still unmeasured.
