# INIT-022: Recall speed that does not send a new word away

**Initiative ID:** INIT-022 · **Status:** complete · **Created:** 2026-10-27 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main` (in place; the change under review was already uncommitted)
**Source:** [the review](../../reviews/lesson-timing-and-hebrew-face/code-review-findings-2026-10-27.md)

## What you get

- A fast answer on a word just taught comes back in minutes. Easy waits until the card is already in review or relearning.
- A meaning question left open for more than a minute is stored as untimed.
- The speed on a card is the latest timed right answer for that skill.
- The North Star and ADR-0020 say the same thing the player does.

## Specs

| Spec | Domain | Title | Blocked by | Effort |
| --- | --- | --- | --- | --- |
| SPEC-001 | service | Hold Easy until the card is in review | — | 2h |
| SPEC-002 | frontend | Ignore a meaning answer left open too long | — | 1h |
| SPEC-003 | api | Report speed from the right answer of the same skill | — | 2h |
| SPEC-004 | docs | Record the word face and the speed rule | — | 1h |

**Total:** 6h. The four specs touch different files and were done in this order.

## Depends on / unblocks

- Sits on the uncommitted lesson-timing change (migration 018, timed meaning questions, Hebrew card face).
- ADR-0018 stays as it was. This initiative does not edit it.

## Done when

- A new word answered faster than the median is due within 15 minutes.
- A gap longer than a minute stores a latency of 0.
- A wrong answer and the other skill's card do not carry `recallMs`.
- ADR-0020 and the North Star sections 2, 4.2, and 6.1 agree with the player.
