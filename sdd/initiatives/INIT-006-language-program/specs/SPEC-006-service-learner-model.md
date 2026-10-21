# SPEC-006 · Service: Learner model — strength, weak words, concept mastery, journey

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-006
title: "Derive word strength, weak flags, concept mastery, and journey stage from the learner's own rows"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-004, SPEC-005]
blocks: [SPEC-007, SPEC-008, SPEC-009]
```

---

## Description

**Context:** `server/progress.js` already computes per-direction stage counts and 30-day accuracy (`metrics()`, `:241`–`:290`). It has no per-word strength, no weak list, and no grammar dimension. ADR-0009 §3–§5 define thresholds; this spec implements them once.

**Scope (in):** `server/learner/model.js`:

- `wordStates(userId, now)`: one entry per word with a review row: `word`, per direction `{ stage, retrievability, strength_bucket, lapses, due_in_days }`, `weak` (`null` / `weak` / `very_weak`) per ADR-0009 §3, `suspended`, `sources`. Strength from `fsrs.retrievability` (SPEC-004); with `SRS_SCHEDULER=ladder`, a documented ladder approximation.
- `recordConceptEvent(userId, { word, kind, lemma, result }, now)`: called inside `applySignals`' transaction, and only for a signal with `kind: 'form'` and result `pass` or `miss`. A word-card grade (`kind` absent or `'word'`) updates nothing, and neither does `not_yet` (ADR-0009 §4, "What updates a concept"). Looks up the form's lexicon entry and its lemma, collects each concept key once (`tense` and `cell` from the form; `binyan` and `root-class` from the form, or from the lemma when the form has none), and updates `concept_mastery` attempts, correct, and `ewma` with α from ADR-0009 §4. An untagged form updates nothing.
- `conceptStates(userId)`: every registry concept with `attempts`, `accuracy` (correct/attempts), `ewma`, `status` per ADR-0009 §4: `unseen` at 0 attempts, `emerging` at 1 to 4, and the four judged statuses from 5 attempts.
- `journey(userId, now)`: stage label per ADR-0009 §5 from the count of words whose `he_en` direction is `longTerm`; counts (`new`, `learning`, `longTerm`) per direction; tenses and binyanim with at least one form attempt; completed lesson slugs from `lesson_progress` (`completedRounds > 0`, ADR-0009 §1 `journey.lessons_completed`); recipe counts when plans exist; vowels-off share from lesson blobs' `vowelsOn` (`server/progress.js:31`), rounded as ADR-0009 §1 says.
- `rebuildConceptMastery(userId)`: recompute from `practice_events` (used by tests and once after the lexicon is tagged).
- Comment at the top of `model.js`: thresholds come from `server/learner/thresholds.js` only; do not inline numbers.

**Scope (out):** what of this leaves the server (SPEC-007); routes (SPEC-009).

---

## Understood (agent's read of the intent)

- [ ] und-1: A miss on חָשַׁבְתָּ counts against `binyan:paal`, `tense:past`, `cell:past.2ms`, and `root-class:regular`. That is the ADR-0009 attribution rule, not a diagnosis. It applies to form practice only, not to the word card for חָשַׁב.
- [ ] und-2: Everything is computed at read time except `concept_mastery`, which is incremental for speed and rebuildable.

## Acceptance Criteria

- [ ] ac-1 (REQ-006): fixture rows with chosen lapses, difficulty, and due dates produce the `weak` / `very_weak` flags the ADR-0009 §3 table predicts, one test per row of that table.
- [ ] ac-2 (REQ-006): a `form` signal on a tagged past-tense form updates exactly the four concept rows; on an untagged form, none. A word-card `pass` or `miss` on the same lemma updates none. A concept with 1 to 4 attempts has status `emerging`.
- [ ] ac-3 (REQ-006): `rebuildConceptMastery` after a sequence of signals equals the incremental result.
- [ ] ac-4 (REQ-006): journey stage boundaries: one fixture per boundary in ADR-0009 §5 (49 and 50, 149 and 150, 499 and 500, 1499 and 1500 words long-term in `he_en`).

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Re-attribute history when tags change later | `rebuildConceptMastery` exists; it runs on demand, not automatically on every lexicon write. | resolved | yes — keeps writes cheap |

> **Readiness** = 100.

---

## Deliverables

- [ ] `server/learner/model.js`, `server/learner/thresholds.js`
- [ ] One call from `server/progress.js` `applySignals`
- [ ] Tests: `test/learner-model.test.js`

---

## Technical Requirements (domain-specific)

- [ ] One SQL query per collection, no N+1 over words (lexicon lookups batched with `IN`).
- [ ] `now` is a parameter everywhere.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/learner-model.test.js` and `npm test`.
- **Rollback plan:** remove the `applySignals` call; the model is read-only otherwise.

---

## Security

- **Sensitivity:** standard. Internal; SPEC-007 decides what leaves.
- **Review depth:** standard · **focus areas:** user scoping on every query.

---

## Verification Strategy

- **Claim:** weak flags, concept mastery, and journey match ADR-0009 thresholds and attribution.
- **Check + executor:** `mechanized` — `node --test test/learner-model.test.js`.
- **Pass condition:** exit 0; each ADR-0009 threshold row has a test.
- **Test levels:** integration on a temporary database with seeded lexicon tags.

---

## Integration Points

- SPEC-007 projects `wordStates`, `conceptStates`, `journey`. SPEC-008 picks weak words and the weakest concept. SPEC-009 shows library status.
- Produces **Change:** learner model.
- **Provenance tag:** `INIT-006/SPEC-006`
