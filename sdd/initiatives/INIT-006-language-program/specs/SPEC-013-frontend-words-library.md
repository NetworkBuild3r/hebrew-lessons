# SPEC-013 · Frontend: Words screen (personal library)

## Metadata

```yaml
spec_id: SPEC-013
initiative_id: INIT-006
title: "New SPA route `words`: search, filters, word detail with tags and paradigm, add, suspend"
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
blocked_by: [SPEC-009]
blocks: [SPEC-020]
```

---

## Description

**Context:** routes are declared in `shared/app-routes.js` (`:48`–`:115`) and laid out by `shared/layout.js`; ADR-0007 is the contract (route table, `mount` / `unmount` / `canLeave`, ready signal). The operator asked for "my word library".

**Scope (in):**

- Route `{ id: 'words', path: '/words/' }` in `shared/app-routes.js`, nav label "Words · מילים", view `shared/words-page.js`; server shell route in `server/spa.js` as the other routes.
- List from `GET /api/me/library`: Hebrew (pointed, vowels toggle), gloss, stage chips per direction, strength bar, weak / very weak badge, tags (pos, root, binyan, root class with its traffic-light colour), sources.
- Filters: text search (Hebrew or English), status, weak, part of speech, binyan, tense, source (`lesson`, `self`, `ai`), suspended. Filter state in the URL query so back/forward works.
- Word detail panel: tags; for a verb lemma, its paradigm from `GET /api/lexicon/:key` grouped by tense and cell, unverified forms marked "not yet checked"; audio; lessons it came from; Suspend / Resume.
- Add a word: Hebrew + English (+ optional transliteration); posts `POST /api/me/library`; validation errors shown inline.
- "Review these": opens Review with the deck matching current filters (`weak`, `concept:*`, `lesson:*`).
- Empty and loading states; paginated with "Load more".
- Comment at top of `words-page.js`: data comes only from `/api/me/library` and `/api/lexicon`; do not read review state from `localStorage`.

**Scope (out):** editing lexicon tags (admin/MCP only); "What your AI sees" (SPEC-017).

---

## Understood (agent's read of the intent)

- [ ] und-1: This is the learner's view of their own words. It is not an admin screen.
- [ ] und-2: INIT-005 SPEC-006/SPEC-010 icons: if present, use a `words` icon; if not, text label only.

## Acceptance Criteria

- [ ] ac-1 (REQ-005): a Chrome smoke signs in, opens `/words/`, sees the ready signal `data-route-ready="words"`, and finds a seeded word with its gloss and stage.
- [ ] ac-2 (REQ-005): filtering by `binyan=hifil` shows only hifil-tagged words; the URL carries the filter; Back restores the previous list.
- [ ] ac-3 (REQ-005): adding a word shows it in the list as `new`; suspending hides it from the Due deck.
- [ ] ac-4: `test/spa-shell.test.js` and `test/chrome-navigation-smoke.js` cover the new route.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Nav order | Today, Lessons, Review, Words, Guides (SPEC-016), Progress, Admin. | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- [ ] `shared/words-page.js`; changes to `shared/app-routes.js`, `server/spa.js`, `shared/worksheet.css`
- [ ] Tests: `test/chrome-words-smoke.js`; updates to `test/spa-shell.test.js`, `test/chrome-navigation-smoke.js`

---

## Technical Requirements (domain-specific)

- [ ] `textContent` only; `dir`/`lang` on Hebrew.
- [ ] Abort in-flight fetches with `ctx.signal` on unmount.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** the smokes and `npm test`.
- **Rollback plan:** remove the route entry.

---

## Security

- **Sensitivity:** standard. Renders lexicon text; escapes it.
- **Review depth:** quick · **focus areas:** DOM insertion.

---

## Verification Strategy

- **Claim:** the learner can browse, filter, inspect, add, and suspend words.
- **Check + executor:** `mechanized` — Chrome smokes on the signed-in test server.
- **Pass condition:** exit 0 for each smoke.
- **Test levels:** browser smoke, shell unit test.

---

## Integration Points

- Links into Review decks (SPEC-012).
- Produces **Change:** Words screen.
- **Provenance tag:** `INIT-006/SPEC-013`
