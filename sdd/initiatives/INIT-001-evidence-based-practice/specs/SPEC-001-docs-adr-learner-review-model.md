# SPEC-001 · Docs: ADR-0001 learner review model

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-001
title: ADR-0001 learner review model
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 2.5 hours
blocked_by: None
blocks: [SPEC-002, SPEC-003, SPEC-008]
```

---

## Description

**Context:** `RSCH-003` rates spaced, cross-day recall as the best-supported practice (High), but the player schedules whole lessons, not words. Before code, one decision record must fix how a word is identified across lessons, how it is scheduled, what counts as a review, and how the review page gets every lesson's words without `fetch`.

**Scope (in):** write `docs/decisions/ADR-0001-learner-review-model.md` deciding:

1. Word identity key across lessons (NFC-normalized pointed word; vowel-stripped fallback only when unique — the rule `shared/player/speech.js` uses for clips; no automatic prefix stripping).
2. Review record shape per word and per direction (`he→en` receptive, `en→he` productive): due time, interval, streak/lapses, last result, sources (lesson slugs).
3. Schedule rule: a small fixed ladder (proposed 1, 3, 7, 14, 30 days), a miss returns to step 1, same-day repeats do not advance; daily caps for due and new items.
4. What counts as a review signal from each activity (`words` self-mark, `type-it`, `listen-pick`, `speed-quiz`, `match`, `fill`, `pattern`, gloss tap) and its weight (pass / miss / not-yet-known).
5. How the review page loads cross-lesson vocab: a generated script `worksheets/_review/vocab-index.js` setting `window.HEBREW_VOCAB_INDEX` (script tag, no fetch), built server-side like the word-audio catalog.
6. localStorage keys and versioning (`hebrew-review:v1`), migration of existing per-lesson `wordMarks`, and fail-soft behavior on quota errors.
7. `reading.glosses` schema (optional map from story token to English) and the fallback to lesson vocab meanings.
8. Constraints carried: no accounts, no fetch for lesson data, no lesson `index.html` edits, no root/binyan speed drills, no forced vowel fading, bilingual labels.

**Scope (out):** any code; backup export/import (recorded as a follow-up); perception drills; habit features.

---

## Understood (agent's read of the intent)

- [x] und-1: The ADR exists so SPEC-002/003/007/008 implement one agreed model instead of each inventing one.
- [x] und-2: Evidence levels from `RSCH-003` are cited in the ADR's rationale so later changes can weigh them.

## Acceptance Criteria

- [x] ac-1: `docs/decisions/ADR-0001-learner-review-model.md` exists with Context, Options considered (at least two for identity key and for vocab loading), Decision, Consequences — covered: yes
- [x] ac-2: The decision states the identity key, record shape, schedule ladder, daily caps, signal table, storage key, migration rule, and `reading.glosses` schema concretely enough that SPEC-003's tests can be written from it — covered: yes
- [x] ac-3: Every operator constraint in REQ-009 is restated in Consequences — covered: yes
- [x] ac-4: Rationale cites `RSCH-003` for spacing, recall direction, interleaving, feedback, and glosses — covered: yes

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Schedule ladder | 1, 3, 7, 14, 30 days; miss → 1 day | resolved | yes — operator accepted 2026-10-08 |
| aud-2 | Daily caps | at most 30 due items and 10 new words per review session | resolved | yes — operator accepted 2026-10-08 |
| aud-3 | Do self-marks ("Knew it") advance the schedule as much as a typed correct answer? | Self-mark advances one step; typed correct advances one step; a miss in any activity resets | resolved | yes — operator accepted 2026-10-08 |
| aud-4 | ADR location | `docs/decisions/` (new folder; repo has no ADRs, so `ADR-0001`; `next_adr_number.py` does not exist here — disclosed) | resolved | yes — operator accepted 2026-10-08 |

> Readiness = 100%. Operator accepted the four recommendations on 2026-10-08; status → `ready`.

---

## Deliverables

- [x] `docs/decisions/ADR-0001-learner-review-model.md`

---

## Technical Requirements (domain-specific)

- [ ] Plain-language ADR, bilingual examples where Hebrew words appear.
- [ ] No learner name, transcript text, or secrets in the document.

---

## Gates & Controls

- **Autonomy level:** `plan_only` — draft the ADR for operator approval.
- **Gate actions triggered:** none.
- **Blocking checks:** operator approval of the decision.
- **Rollback plan:** delete the file.

---

## Security

- **Sensitivity:** light — triggers matched: none.
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** the ADR fixes every value the downstream specs need, with no open "TBD" in the Decision section.
- **Check + executor:** `human` — the operator reads the Decision section against ac-2's list and the four ledger rows.
- **Pass condition:** each item in ac-2 has a concrete value in the Decision section, each ledger row is marked resolved with the operator's choice, and the Decision section contains no "TBD".

---

## Integration Points

- Consumed by SPEC-002 (index format), SPEC-003 (store and scheduler), SPEC-007 (glosses), SPEC-008 (validator rules).
- Produces **Change:** one new ADR file.
- **Provenance tag:** `INIT-001/SPEC-001`
