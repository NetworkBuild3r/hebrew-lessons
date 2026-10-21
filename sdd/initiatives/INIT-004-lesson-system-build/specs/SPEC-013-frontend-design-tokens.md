# SPEC-013 · Frontend: Design tokens in the shared stylesheet

## Metadata

```yaml
spec_id: SPEC-013
initiative_id: INIT-004
title: "ADR-0006 tokens in the shared stylesheet"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 3 hours
blocked_by: []
blocks: [SPEC-014]
```

---

## Description

**Context:** ADR-0005 §6 and ADR-0006 Decision 2. One stylesheet, `shared/worksheet.css`, gains a token block. Existing selectors keep their names. Hex values already in the file move into tokens only when a later spec touches that screen, not in this spec's sweep.

**Scope (in):**

- Custom properties on `:root` for the tokens in `docs/decisions/ADR-0006-assets/tokens.json`: color, type, space, radius, target sizes, motion, breakpoints as the ADR tables state them.
- Dark scheme overrides the same variables under `prefers-color-scheme: dark`, and `data-theme="light"` or `data-theme="dark"` on the document element wins over the media query.
- `prefers-reduced-motion: reduce` sets the `motion.reduced` values.
- New rules in this block use variables. They do not add a raw hex.
- A test reads `tokens.json` and the stylesheet and checks that each color token's light and dark values appear as variables.

**Scope (out):** rendering a lesson (SPEC-014); restyling the legacy player; replacing every existing hex in the file.

---

## Understood (agent's read of the intent)

- [x] und-1: Rubik stays the self-hosted files already under `shared/fonts/`. This spec does not fetch a font.
- [x] und-2: `data-theme` is the product form of the spike's radios. The radios themselves are not copied as a new control on every page in this spec.

## Acceptance Criteria

- [x] ac-1 (REQ-009): every color token in `tokens.json` has a light and a dark custom property, and the test's contrast check still meets 4.5:1 for text pairs and 3:1 for UI pairs using the same ratios ADR-0006 Decision 2 lists.
- [x] ac-2 (REQ-009): `size.target` is 24px and `size.targetPrimary` is 44px in the variable block. `radius.default` is 2px. `elevation` adds no box-shadow in the new rules.
- [x] ac-3 (REQ-014): a snapshot of class selectors that `test/app-shell.test.js` and the chrome smokes query is unchanged by this diff, unless this spec did not touch those selectors. The test fails if a queried class is renamed here.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Which file | `shared/worksheet.css`, as ADR-0005 §6. Not a new framework stylesheet. | resolved | yes — ADR-0005 |

> **Readiness** = 100.

---

## Deliverables

- [x] Token block in `shared/worksheet.css`
- [x] Tests: `test/design-tokens.test.js`

---

## Technical Requirements (domain-specific)

- [x] Logical properties are not required in this spec's token block. SPEC-014 uses them in components.
- [x] No new npm dependency.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/design-tokens.test.js` and `npm test`.
- **Rollback plan:** revert the stylesheet.

---

## Security

- **Sensitivity:** light. Triggers matched: none. The block is presentation.
- **Review depth:** quick · **focus areas:** no remote URLs in the CSS.

---

## Verification Strategy

- **Claim:** the stylesheet exposes the ADR-0006 token values, and existing test hooks are not renamed.
- **Check + executor:** `mechanized` — `node --test test/design-tokens.test.js`.
- **Pass condition:** exit 0; each token name from `tokens.json` is found; no new `url(http` appears in the added block.
- **Test levels:** unit over the file text and a contrast function shared with `spikes/look/check.js` or reimplemented in the test without importing the spike into the server. No coverage tool is configured; the bar is this file plus `npm test`.

---

## Integration Points

- SPEC-014's component CSS uses these variables and does not introduce a raw hex.
- Produces **Change:** token block.
- **Provenance tag:** `INIT-004/SPEC-013`
