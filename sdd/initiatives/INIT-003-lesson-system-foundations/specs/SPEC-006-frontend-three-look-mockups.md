# SPEC-006 · Frontend: three look mockups for Today and a lesson step

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-003
title: "Three look mockups for Today and a lesson step"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_branch
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-003, SPEC-004]
blocks: [SPEC-008]
```

---

## Description

**Context:** The operator finds the site "super lame" and wants the look to be backed by evidence. Evidence narrows the choices but cannot pick a look, so the operator chooses among concrete options. This spec produces the options. It does not touch the live site.

**Scope (in):** three static mockups, each showing the same two screens, so they can be compared side by side:

- **Today:** the home screen with one clear next action, the review-due summary, and the next lesson in the plan.
- **A lesson step:** a "meet the words" screen with a Hebrew word with points, its English meaning, an audio control, and a picture, and one recall screen with a miss shown (answer kept on screen, retry prompt) — the behaviors from `RSCH-003` and INIT-001.

Each mockup follows the interface rules in `RSCH-006` and differs from the others in direction (for example, one calm and typographic, one warm and illustrated, one dense and tool-like). Each is a self-contained set of HTML and CSS files, with:
- real Hebrew text with points, in the self-hosted Rubik face (`shared/fonts/`) or a face the mockup names and ships under its licence;
- right-to-left layout with English mixed in, a phone width and a desktop width, a light and a dark colour scheme, and reduced-motion handling;
- placeholder pictures that are plainly placeholders (no real photographs, no learner images);
- a short `README` per mockup stating which `RSCH-006` rules it follows and which it breaks, with the reason.

**Scope (in, also):** a one-page comparison file that lists, for each mockup, the rules followed, the rules traded off, and a screenshot of each screen at both widths.

**Scope (out):** wiring any mockup to real data, changing `shared/worksheet.css`, any page under a served folder, and any build tooling. The operator's pick is recorded by SPEC-008.

---

## Understood (agent's read of the intent)

- [x] und-1: The mockups live under `spikes/look/` at the repository root, which is outside every served folder and excluded from the Docker image (confirmed in `.dockerignore` as part of this spec).
- [x] und-2: No points, streaks, or badges appear in any mockup, because `RSCH-003` graded gamification as unproven.
- [x] und-3: The block types shown are the ones ADR-0003 defines, so the lesson step is a real screen, not an invented one.

## Acceptance Criteria

- [x] ac-1 (REQ-008): three mockup folders exist under `spikes/look/`, each with a Today page and a lesson-step page, and each opens from the file system with no server or build step.
- [x] ac-2 (REQ-008): each page renders correctly at 360 px and 1280 px wide, in light and dark colour schemes, with right-to-left Hebrew and left-to-right English in the same view.
- [x] ac-3 (REQ-008): each mockup states in its README which `RSCH-006` rules it follows and which it trades off, and the three together cover every row of `RSCH-006`'s interface rule table.
- [x] ac-4 (REQ-008): a mechanical check passes on every page: text contrast meets the `RSCH-006` threshold, touch targets meet the stated minimum size, and nothing animates when reduced motion is requested.
- [x] ac-5: the comparison file has screenshots of both screens at both widths for all three mockups, and no learner names, transcripts, or secrets appear anywhere.
- [x] ac-6: `spikes/` is excluded from the Docker image and from every served path (checked by a test).

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Number of options | Three. The operator said "whatever is backed by science", so the three differ in how the evidence is balanced, and the operator picks | resolved | yes — operator 2026-10-08, plan default |
| aud-2 | Dark mode in scope | Mockups show it; whether to ship it is ADR-0006's call | resolved | yes |
| aud-3 | Illustrations | Placeholders only; real art is produced later (uploaded or generated, per ADR-0004) | resolved | yes |

---

## Deliverables

- [x] `spikes/look/a-*/`, `spikes/look/b-*/`, `spikes/look/c-*/` — each with `today.html`, `lesson-step.html`, its own CSS, and `README.md`
- [x] `spikes/look/COMPARE.md` — the comparison with screenshots in `spikes/look/shots/`
- [x] `spikes/look/check.js` — the mechanical contrast, target-size, and reduced-motion check
- [x] `.dockerignore` — add `spikes/` if absent
- [x] Tests: `test/spikes-not-served.test.js` (the spikes folder is not reachable through the server and not in the image context)

---

## Technical Requirements (domain-specific)

- [x] Plain HTML and CSS only; CSS custom properties for every colour, size, and spacing value; no remote fonts, scripts, or stylesheets (consistent with `test/self-hosted-assets.test.js`).
- [x] All colour pairs meet the `RSCH-006` contrast threshold in both schemes. Every interactive element meets the stated minimum target size.
- [x] The Hebrew with points is set at a size and line height that keep the points legible, per `RSCH-006`.
- [x] No inline event handlers; no script that fetches anything.

---

## Gates & Controls

- **Autonomy level:** `auto_branch`.
- **Gate actions triggered:** none — branch-only, fully reversible.
- **Blocking checks:** `node spikes/look/check.js` exits clean; `npm test` still passes.
- **Rollback plan:** delete `spikes/look/` and the `.dockerignore` line.

---

## Security

- **Sensitivity:** light. Triggers matched: none.
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** each mockup is viewable, follows the stated interface rules, meets the accessibility thresholds, and is not reachable through the server.
- **Check + executor:** two checks, declared separately.
  - `mechanized`: `node spikes/look/check.js` measures contrast, target size, and reduced-motion behavior on every page; `npm test` runs `test/spikes-not-served.test.js`.
  - `human`: the operator views the three mockups and the comparison file and picks one (or a blend). A second reader checks that each README's stated trade-offs are visible in the mockup.
- **Pass condition:** the script exits 0 for every page; the not-served test passes; the operator names a pick or a blend in writing; each README lists at least one rule it trades off and a reader can find the effect on screen.

---

## Integration Points

- Consumed by SPEC-008 (ADR-0006 records the pick and turns it into tokens and a component list) and SPEC-009 (the spike uses the picked look).
- Produces **Change:** three mockups, a comparison, a check script, and one test.
- **Provenance tag:** `INIT-003/SPEC-006`
