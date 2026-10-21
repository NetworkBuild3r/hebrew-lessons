# SPEC-007 · Frontend: Story glosses and coverage estimate

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-001
title: Story glosses and coverage estimate
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-003]
blocks: [SPEC-009]
```

---

## Description

**Context:** the `story` activity shows pointed paragraphs with audio but no way to look up a word in place. `RSCH-003`: glosses raise word learning, first-language glosses beat target-language ones (Moderate); reading is comfortable at about 95–98% known words (Low; listening is an extrapolation, Very Low).

**Scope (in):**
- Tap-to-gloss in `story`: tapping a word shows a small popover with its English meaning and an audio button. Meaning source order: `reading.glosses[token]` (new optional field, ADR-0001) → lesson vocab match by identity key → "no gloss" (popover offers audio only).
- Each gloss tap records a `not-yet-known` signal for that word in the review store (does not reset a word already at a high step more than once per day).
- Coverage estimate on the story step header: "≈92% מילים מוכרות (known words, estimate)" from tokens whose identity key has a passing review record or appears in an earlier completed lesson; labelled as an estimate with a one-line explanation.

**Scope (out):** automatic Hebrew prefix splitting (ו/ה/ב/ל/מ/ש/כ) — tokens are matched as written plus the vowel-stripped fallback; glosses in other activities; authoring the glosses (SPEC-009 documents, lessons supply).

**Sizing (frontend band):** 0 new screens, 2 components (gloss popover, coverage badge) plus `story` changes, states: no gloss / gloss shown / audio playing / coverage unknown / coverage shown — in band.

---

## Understood (agent's read of the intent)

- [x] und-1: Glosses are opt-in per tap; the story is never shown with English underneath by default.
- [x] und-2: Coverage is a guide for lesson choice, not a grade.

## Acceptance Criteria

- [x] ac-1 (REQ-006): tapping a story word with a gloss or vocab match shows its English meaning; tapping elsewhere closes it — covered: yes
- [x] ac-2 (REQ-006): each tap records a `not-yet-known` signal for that word (at most once per word per day) — covered: yes
- [x] ac-3 (REQ-007): the story header shows a coverage percentage labelled as an estimate, computed from the review store; with an empty store it shows "unknown" rather than 0% — covered: yes
- [x] ac-4 (REQ-009): existing lessons without `reading.glosses` still gloss their vocab words; no lesson file edits are required — covered: yes
- [x] ac-5 (security): gloss text renders as text; a fixture gloss containing HTML is escaped — covered: yes

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Gloss tap signal strength | `not-yet-known` makes the word due tomorrow but does not count as a full miss (ADR-0001 §5: no lapse bump; does not reset higher step) | resolved | yes — orchestrator 2026-10-08 (mode-e run; ADR-0001) |
| aud-2 | Tokenization | split on whitespace and Hebrew punctuation (maqaf, geresh kept with the word) | resolved | yes — orchestrator 2026-10-08 (mode-e run) |

---

## Deliverables

- [x] `shared/activities/story.js`, styles in `shared/worksheet.css`
- [x] `shared/player/utils.js` — `tokenizeHebrew(text)` helper
- [x] Tests: `test/tokenize-hebrew.test.js`; Chrome smoke step tapping a story word on `my-morning`

---

## Technical Requirements (domain-specific)

- [x] Popover is keyboard reachable and dismissible (Escape), RTL-aware, and does not cover the audio controls.
- [x] Story read-along audio keeps working while glosses are enabled.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`; smoke check.
- **Rollback plan:** revert `story.js`, `utils.js`, CSS.

---

## Security

- **Sensitivity:** standard — triggers matched: rendering author-supplied text.
- **Security acceptance criteria:**
  - [x] Gloss and meaning text pass through `U.escapeHtml`.
- **Review depth:** standard · **focus areas:** XSS.

---

## Verification Strategy

- **Claim:** story words can be glossed in English on demand, taps feed the schedule, and the coverage figure is computed from real review data and labelled as an estimate.
- **Check + executor:** `mechanized` — `npm test` (tokenizer cases including maqaf and final letters) and a headless Chrome step on `my-morning` that taps a vocab word, reads the popover, and checks the store.
- **Pass condition:** the popover shows the lesson's English meaning for the tapped word; the store has a `not-yet-known` entry dated today; the header shows "unknown" with an empty store and a percentage after seeding known words; the HTML fixture renders escaped; `npm test` exits 0.

---

## Integration Points

- Reads lesson `reading.glosses` (schema from ADR-0001, documented by SPEC-009, validated by SPEC-008); writes to `ctx.review` (SPEC-003).
- Produces **Change:** tap-to-gloss and coverage badge in stories.
- **Provenance tag:** `INIT-001/SPEC-007`
