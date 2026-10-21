# SPEC-008 · Service: Daily-mix composer

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-006
title: "Compose a session from due cards, weak words, one concept drill, and one known-word reading"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-006]
blocks: [SPEC-010]
```

---

## Description

**Context:** the operator asked for "a dynamic program that knows words I learned … so we can dynamically stack them". RSCH-004 §3 rule 14 limits that: a personal queue may add due reviews and skip known words; it may not reorder unseen words off the spine. RSCH-003 §1.1 supports later-day recall (High) and interleaving (Moderate). ADR-0009 §8 is the composition.

**Scope (in):** `server/learner/mix.js` — `compose(userId, now, opts)` returns ordered sections:

1. `review` — due cards from `review_items` (not suspended), capped by the existing `DEFAULT_CAPS` in `shared/player/review.js:9` (`due: 30`, `new: 10`). New cards only from the learner's library, never from an unseen lesson.
2. `weak` — up to 8 words flagged `very_weak` then `weak` that are not already in `review`, both directions.
3. `drill` — the weakest concept with status `weak` or `very_weak` and at least the minimum attempts; up to 6 **verified** lexicon forms tagged with that concept whose lemma the learner has in the library; interleaved across at least two lemmas when available. Omitted when no such concept or form exists.
4. `reading` — up to 3 sentences, from the `sentence` fields of the published lessons' vocab as merged by `server/vocab-index.js` (the lexicon has no example field), whose coverage by SPEC-007 `checkCoverage` (`audience: 'self'`) is at least 0.95. Omitted when none qualify.

Each item carries what the renderer needs (keys, direction, prompt, accepted spellings for drills) and nothing about other learners. Deterministic for a given `now` and seed so tests are stable.

**Scope (out):** route (SPEC-010); UI (SPEC-015); AI-authored personal plans (INIT-004 personal plans via the skill, SPEC-019).

---

## Understood (agent's read of the intent)

- [ ] und-1: "Stacking" happens through review and reading built from known words, not by inventing new words in the mix.
- [ ] und-2: The mix is computed on request and not stored.

## Acceptance Criteria

- [ ] ac-1 (REQ-012): a fixture with 40 due cards returns 30 in `review`; every `review` and `weak` word is in the learner's review rows or library, every `drill` form has a lemma in the library, and a `reading` sentence has at most 5% unknown running words.
- [ ] ac-2 (REQ-012): `drill` contains only `verified` forms and spans two lemmas when two qualify.
- [ ] ac-3 (REQ-012): every `reading` sentence has coverage ≥ 0.95 for the fixture learner; with none qualifying, the section is absent.
- [ ] ac-4: same inputs and `now` produce the same output.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Use plan stories as reading sources | Worksheet vocab sentences now. Published plan stories are added once INIT-004 plans exist, read through INIT-004's learner read function, not SQL (ADR-0009 §8). | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- [ ] `server/learner/mix.js`
- [ ] Tests: `test/learner-mix.test.js`

---

## Technical Requirements (domain-specific)

- [ ] Caps imported from `shared/player/review.js`, not restated.
- [ ] Seeded shuffle; no `Math.random()` without a seed.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/learner-mix.test.js` and `npm test`.
- **Rollback plan:** delete the module.

---

## Security

- **Sensitivity:** standard. Reads only the requesting learner's rows.
- **Review depth:** quick · **focus areas:** user scoping.

---

## Verification Strategy

- **Claim:** the mix follows ADR-0009 §8 and RSCH-004 rule 14.
- **Check + executor:** `mechanized` — `node --test test/learner-mix.test.js`.
- **Pass condition:** exit 0; ac-1 to ac-4 named tests.
- **Test levels:** integration on a temporary database.

---

## Integration Points

- SPEC-010 exposes `GET /api/me/mix`. SPEC-015 renders it.
- Produces **Change:** daily mix.
- **Provenance tag:** `INIT-006/SPEC-008`
