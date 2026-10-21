# SPEC-014 · Frontend: Explainer, conjugation-table, and personal-question renderers

## Metadata

```yaml
spec_id: SPEC-014
initiative_id: INIT-006
title: "Render explainer sections with traffic-light callouts, conjugation tables in study and drill mode, personal questions"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-011, INIT-004/SPEC-014]
blocks: [SPEC-016]
```

---

## Description

**Context:** INIT-004 SPEC-014 builds `shared/ui/` renderers for the six ADR-0003 blocks from a published snapshot, with escaping and ADR-0006 tokens. This spec adds three renderers to that registry. RSCH-003 §1.3: focused pattern work with feedback (Moderate); prompts that push self-correction did at least as well as recasts.

**Scope (in):** in `shared/ui/blocks/`:

- `explainer.js`: headings, paragraphs, examples (Hebrew with root letters highlighted when `root` is set — base letters matched in order, ignoring points; English; optional transliteration), simple tables, callouts with tone `green` / `yellow` / `red` / `info` (colour plus an icon or word so colour is not the only signal). Guides like "Decoding Hebrew Verbs" are lessons made of these blocks.
- `conjugation-table.js`:
  - Study mode: present table as SM / SF / PM / PF columns; past table as person rows (אני, אתה, את, הוא, היא, אנחנו, אתם, אתן, הם/הן); root letters highlighted; audio per form when a clip exists; unverified forms marked "not yet checked".
  - Drill mode: cells blank one at a time in a shuffled, interleaved order across the lesson's tables; learner types (Hebrew keyboard) or picks; answer compared after vowel stripping against the form's `accept[]`; feedback first prompts "try again" once, then shows the form; "I was right" override counts as pass. Each attempt emits a `kind: 'form'` signal (`word` = form key, `lemma`, `result`). Only verified forms are drilled. Untimed.
  - Sentences and questions under each verb rendered as reading lines and prompts.
- `personal-question.js`: prompt (Hebrew, optional English toggle), a "say it aloud" affordance and an optional text box; nothing is sent to the server; a "Done" mark is kept in the lesson blob as a boolean only.
- Comment at the top of `conjugation-table.js`: form signals never create cards; never drill an unverified form.

**Scope (out):** `verse-reading`, `letter-intro`, Guides shelf (SPEC-016).

---

## Understood (agent's read of the intent)

- [x] und-1: No timer anywhere (ADR-0001 #4 as changed).
- [x] und-2: The learner's typed text stays in the page.

## Acceptance Criteria

- [x] ac-1 (REQ-011): a renderer unit test feeds a hostile string in every explainer and table field and finds it as text, not markup.
- [x] ac-2 (REQ-011): drill mode shows only verified cells; a correct unpointed answer passes; a wrong answer prompts once then reveals; each attempt posts one `kind: 'form'` signal (smoke checks the server row).
- [x] ac-3: present table has four columns in the order SM, SF, PM, PF; past table nine rows in the person order above.
- [x] ac-4: personal-question posts nothing to `/api/` except the lesson blob boolean (smoke watches requests).

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Hebrew keyboard | Rely on the OS keyboard; offer a small on-screen letter pad for the drill only. | resolved | yes — RSCH-005 does not support a keyboard course; a pad is an input aid |

> **Readiness** = 100 once INIT-004 SPEC-014 is complete.

---

## Deliverables

- [x] `shared/ui/blocks/explainer.js`, `conjugation-table.js`, `personal-question.js`; registry entries
- [x] Styles in `shared/worksheet.css`
- [x] Tests: `test/render-explainer.test.js`, `test/render-conjugation.test.js`, `test/chrome-verb-workshop-smoke.js`

---

## Technical Requirements (domain-specific)

- [x] Uses INIT-004's escape helpers; no `innerHTML` with data.
- [x] Callout colours from ADR-0006 tokens, contrast checked in light and dark.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** the three tests and `npm test`.
- **Rollback plan:** unregister the three renderers; plans with these blocks show INIT-004's unknown-block placeholder.

---

## Security

- **Sensitivity:** standard. Escaping of plan and lexicon text.
- **Review depth:** standard · **focus areas:** DOM insertion.

---

## Verification Strategy

- **Claim:** guides and verb workshops render correctly and drills record form results without timers or cards.
- **Check + executor:** `mechanized` — unit renderer tests and the Chrome smoke on a fixture published plan.
- **Pass condition:** exit 0; server shows form events and no new review rows.
- **Test levels:** unit, browser smoke.

---

## Integration Points

- SPEC-018's guide and workshop are the first real content. SPEC-015 reuses the drill component for the mix's `drill` section.
- Produces **Change:** three block renderers.
- **Provenance tag:** `INIT-006/SPEC-014`
