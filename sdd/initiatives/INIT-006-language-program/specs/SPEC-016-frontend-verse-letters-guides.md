# SPEC-016 · Frontend: Verse-reading and letter-intro renderers; Guides shelf

## Metadata

```yaml
spec_id: SPEC-016
initiative_id: INIT-006
title: "Render verse-by-verse readings and letter introductions; new SPA route `guides`"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-014]
blocks: [SPEC-020]
```

---

## Description

**Context:** the teacher's verse-by-verse readings (Genesis, Isaiah) and the letters-and-vowels sheets in the attached zip are two recipes (INIT-006 §2). Guides (recipe `guide`) are lessons of explainer blocks (SPEC-014). ADR-0007 is the route contract.

**Scope (in):**

- `shared/ui/blocks/verse-reading.js`: reference line; new words first (pointed, gloss, audio); the pointed verse with tap-to-gloss on known lexicon words; vowels toggle respected; simple Hebrew questions with optional English under them; one personal question rendered with SPEC-014's personal-question component.
- `shared/ui/blocks/letter-intro.js`: the letter large, its name, the sound with each vowel (pointed and sound text, audio when present), "which word contains the letter" as tap-to-choose over `findIn[]` with feedback. No picture match: the block has no picture field (ADR-0009 §7, "Pictures"; amended by SPEC-001). "Write the letter" is shown as a paper-practice prompt, not scored.
- Route `{ id: 'guides', path: '/guides/' }` in `shared/app-routes.js`, view `shared/guides-page.js`: lists published lessons whose `recipe` is `guide`, grouped by topic; opens them in the plan lesson view. Nav label "Guides · מדריכים".
- Lessons shelf shows a recipe chip on each plan lesson (Conversation, Review, Verse, Verbs, Letters, Guide).

**Scope (out):** authoring content (SPEC-018).

---

## Understood (agent's read of the intent)

- [x] und-1: Verse text is shown as authored; the renderer does not fetch scripture.

## Acceptance Criteria

- [x] ac-1 (REQ-010): renderer tests for both blocks with hostile strings render as text; `dir="rtl"` on Hebrew.
- [x] ac-2 (REQ-010): a Chrome smoke opens `/guides/`, sees `data-route-ready="guides"`, opens a fixture guide, and sees its explainer sections.
- [x] ac-3: a fixture verse-reading lesson shows new words before the verse and the personal question last.
- [x] ac-4: `test/spa-shell.test.js` and `test/chrome-navigation-smoke.js` cover `guides`.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Guides as their own plan | Yes: one shared plan "Guides" with one unit per topic; lessons recipe `guide`. | resolved | yes — ADR-0009 §7 exempts `guide` from N2/N7 |

> **Readiness** = 100.

---

## Deliverables

- [x] `shared/ui/blocks/verse-reading.js`, `letter-intro.js`; `shared/guides-page.js`; route and shell changes
- [x] Tests: `test/render-verse-letter.test.js`, `test/chrome-guides-smoke.js`; shell/navigation test updates

---

## Technical Requirements (domain-specific)

- [x] INIT-004 escape helpers; `textContent` only.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** tests and `npm test`.
- **Rollback plan:** remove the route and unregister the renderers.

---

## Security

- **Sensitivity:** standard. Escaping.
- **Review depth:** quick · **focus areas:** DOM insertion.

---

## Verification Strategy

- **Claim:** verse readings, letter introductions, and the Guides shelf render and navigate under ADR-0007.
- **Check + executor:** `mechanized` — renderer tests and Chrome smoke.
- **Pass condition:** exit 0.
- **Test levels:** unit, browser smoke.

---

## Integration Points

- SPEC-018's guide appears on this shelf.
- Produces **Change:** two renderers and the Guides route.
- **Provenance tag:** `INIT-006/SPEC-016`
