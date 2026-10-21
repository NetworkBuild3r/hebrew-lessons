# SPEC-014 · Frontend: Today, plan, and six block renderers

## Metadata

```yaml
spec_id: SPEC-014
initiative_id: INIT-004
title: "Today, plan overview, and renderers for the six block types"
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
blocked_by: [SPEC-004, SPEC-013]
blocks: [SPEC-015, SPEC-020]
```

---

## Description

**Context:** ADR-0006 components and ADR-0003's six block types. ADR-0005's component contract: a function `(props) => string` in `shared/ui/` that escapes text. The spike's render and `public/app.js` `textContent` behavior are the base. Port them. Do not import `spikes/skeleton` from a served page.

**Scope (in):**

- Renderers for `introduction`, `graded-story`, `listening`, `short-dialogue`, `fluency-pass`, and `spaced-review-checkpoint`.
- Today: one next action, due line without points, streaks, or a leaderboard. The page can show a published plan's lesson list.
- Plan overview and a lesson step. One item, then the primary control. On a narrow viewport the primary control is in the bottom dock (`breakpoint.narrow`).
- Introduction cluster: pointed word, English gloss, optional picture, audio control. Recall state: no picture; the attempt stays; the correction includes the correct form. An empty attempt does not reveal the word.
- Picture: `mediaId` resolved only through the gated media URL the app already fetched with the session. Empty alt: no `img`. The spike's four ids are not hardcoded.
- `dir="rtl"` and `lang="he"` on the document. Each English gloss is `dir="ltr"` and `lang="en"`. Direction is an attribute, not a CSS `direction` property.
- Learner load is `fetch` of `GET /api/plans/:id/published` only. No `fetch` of a worksheet URL. Legacy worksheet pages are not rewritten in this spec.
- Escape plan text. Hostile strings do not become tags. One script element for the page behavior, no event-handler attributes built from plan text.

**Scope (out):** the builder (SPEC-015, SPEC-016); switching a live lesson link (SPEC-020); the AI-planning disclosure sentence on the sign-in page and the account menu (INIT-006 SPEC-017, amended 2026-10-09: nothing in INIT-004 sends learner data to the MCP, so no disclosure is needed here, and ADR-0009 §1 replaces the ADR-0004 §3 wording). This spec adds the screens and a way to open them for a plan id. The operator chooses the cutover.

---

## Understood (agent's read of the intent)

- [x] und-1: Checkpoint `wordKeys` come from the plan's `recycles` / `wordKeys`. The page does not invent a word list.
- [x] und-2: Audio with no `audioPath` is a control that does not request a missing clip. Do not invent a path.

## Acceptance Criteria

- [x] ac-1 (REQ-009): a fixture page for `beginner-unit-1` renders Today, the unit overview, an introduction, a recall state, and the checkpoint. The checkpoint lists each recycled word with `fromLessonId`.
- [x] ac-2 (REQ-009): each of the six `type` values has a renderer test. `graded-story`, `listening`, `short-dialogue`, `fluency-pass`, and the checkpoint emit no `img`.
- [x] ac-3 (REQ-009): a plan string `<script>` and an image with empty alt produce no extra script, no event-handler attribute, and no `img`. Text over the render path is escaped.
- [x] ac-4 (REQ-009): stylesheet rules added for these screens use the token variables. Text contrast in the rendered fixture meets 4.5:1 in light and dark. Primary controls are at least 44px. Other pointer targets are at least 24px.
- [x] ac-5 (REQ-009): the sign-in page and the account menu still contain the existing admin-visibility sentence, unchanged by this spec.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Page shape | One column, server-rendered HTML strings, no client router. Matches the spike and a-calm-type. | resolved | yes — ADR-0006 and the spike RESULTS |

> **Readiness** = 100.

---

## Deliverables

- [x] `shared/ui/` component functions
- [x] Lesson and Today entry that fetches the published snapshot when signed in
- [x] Tests: `test/plan-render.test.js`

---

## Technical Requirements (domain-specific)

- [x] Escape with the helper the app already uses for HTML text. Do not add a sanitizer package.
- [x] Keep existing `data-nav`, `data-today-card`, and other hooks ADR-0006 Decision 8 marks `kept`, until a screen switch updates that test in the same change. This spec's new screens use new `data-*` names and do not rename the old ones.
- [x] `data-action="set-dir"` stays retired.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-render.test.js` and `npm test`.
- **Rollback plan:** stop linking the new screens. Legacy Today stays.

---

## Security

- **Sensitivity:** standard. Triggers matched: rendering untrusted plan text. No learner data is rendered here.
- **Security acceptance criteria:**
  - [x] Plan strings are text, not HTML.
  - [x] Image requests go to `GET /api/media/:id` with the session, not to a URL stored in the plan.
- **Review depth:** standard · **focus areas:** XSS, alt text.

---

## Verification Strategy

- **Claim:** the six block types render from a published snapshot, hostile text cannot inject a tag, and the admin-visibility sentence is still present.
- **Check + executor:** `mechanized` — `node --test test/plan-render.test.js`.
- **Pass condition:** exit 0; the hostile fixture's serialized HTML contains one script `src` and no `onerror=` or `onclick=`; the admin-visibility sentence is unchanged.
- **Test levels:** unit rendering to a string, plus the existing chrome suite still green. Browser click of recall is covered by the string and by the recall function test the spike used (`decideRecall`). A full Chrome click is not required for this spec if the driver is still absent; say so in the completion summary. No coverage tool is configured; the bar is this file plus `npm test`.

---

## Integration Points

- Consumes SPEC-004's published GET and SPEC-013's variables. SPEC-020 points a learner link here only after the operator's gate.
- Produces **Change:** plan renderers and Today/plan screens.
- **Provenance tag:** `INIT-004/SPEC-014`
