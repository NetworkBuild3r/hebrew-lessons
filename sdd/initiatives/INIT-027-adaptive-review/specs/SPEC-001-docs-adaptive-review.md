# SPEC-001 · docs: ADR-0023, review that moves on when you know it

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-027
title: "ADR-0023: review that moves on when you know it"
domain: docs
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 1 hour
blocked_by: []
blocks: []
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:**
- ADR-0020 holds the speed rule (Easy waits until review). ADR-0022 holds the learner day. Neither covers drills, the known padding, implied credit, or new-word supply.
- `docs/north-star.md` §4.2 "Every answer is an event", §4.4 "Progression and regression", §4.5 "Finding the real struggle", and §6 "Activities" describe the review behavior this initiative changes.
- North-star docs describe the destination in present tense. No status, phase, date-as-status, or "now vs target" words.

**Scope (in):**
- `docs/decisions/ADR-0023-review-that-moves-on.md`. Status Accepted. Extends ADR-0020 and ADR-0022. Initiative INIT-027 SPEC-001. One decision per heading:
  - **D1 Drills end.** A letter drill is a timed two-option pick of the letter in a real word. Pattern drills stay self-graded, and Good or Easy is right. Every drill answer is stored. Three right in a row since the pair's last miss, none slower than twice the learner's drill median once 10 timed drill answers exist, clears the pair. Only misses after the latest clear count toward a new drill. 3 steps per pair, 6 steps per session.
  - **D2 Typing slips.** A typed answer logs a letter pair only for look-alike or sound-alike pairs. Name both sets. A wrong pick of a one-letter-off word logs the pair as before.
  - **D3 Due cards only.** A session queues due cards and new cards. A known card that is not due is never added to fill the known share. Cite RSCH-003 checklist line 173 and the "longer gaps help delayed retention" finding.
  - **D4 Move up, with implied credit.** A due card is asked through its own skill, or through the skill that implies it (recall implies read, spell implies listen) when its own skill is known or solid and the implying card has no Again or Hard in 7 days. A right implying answer schedules the due card Good, never Easy, with `review_log.implied = 1` and no attempts row. Accuracy counts skip implied rows.
  - **D5 New words follow accuracy.** The new-card cap is 2 below 80%, 5 from 80% to 90%, and 8 above 90% (last 50 non-implied reviews). At or above the 85% target, when fewer `new` cards wait than the cap, review seeds verified placement-list words with a gloss, lowest band first, in list order, as read and recall cards plus listen with a clip.
  - **D6 The review sitting follows the learner's study day,** with the 4:00 rollover in the learner's zone (ADR-0022), not the UTC date.
  - Consequences: sessions can be shorter than 20; ADR-0020's speed rule is unchanged; RSCH-003 has no speed evidence, so D1's slow cut is a product rule.
- `docs/north-star.md`, present tense, at most one paragraph or table row per section touched: §4.2 (drill answers and implied reviews are events; implied rows are marked), §4.5 (letter drills from look-alike and sound-alike pairs, and how a drill ends), §6 (the letter pick drill; move-up order), and the session paragraph that names the mix.
- `docs/decisions/` index or README, if one lists ADRs.

**Scope (out):** Code. MCP descriptions (SPEC-005 updates `mixTarget` text). Research.

## Acceptance Criteria

- [ ] ac-1: ADR-0023 exists, Accepted, with D1 to D6, each with its numbers as above, and a Consequences section.
- [ ] ac-2: The ADR names the look-alike set as the drill cue table and lists the six sound-alike pairs.
- [ ] ac-3: The north-star edits are present tense and contain none of the banned tracking words (`Current State`, `Phase`, `WIP`, `not yet`, `for now`, `remaining work`, a status emoji, or a date as status).
- [ ] ac-4: No document says a known card that is not due is added to a session, and none says a drill grade is discarded.

## Deliverables

- [ ] `docs/decisions/ADR-0023-review-that-moves-on.md`
- [ ] `docs/north-star.md`

## Verification Strategy

- **Claim:** The decisions this initiative codes are written down once, with their numbers, and the north star describes the result in present tense.
- **Check + executor:** mechanized. `rg -n 'D1|D2|D3|D4|D5|D6' docs/decisions/ADR-0023-review-that-moves-on.md`; `rg -n -i 'current state|phase [0-9]|\bWIP\b|not yet|for now|remaining work|✅|❌|🚧' docs/north-star.md` returns nothing new against `origin/main`.
- **Pass condition:** ac-1 to ac-4 hold on a read of the diff.

## Integration Points

- **Provenance tag:** `INIT-027/SPEC-001`
- SPEC-003 to SPEC-006 cite ADR-0023 decisions by number.
