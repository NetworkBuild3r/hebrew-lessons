# SPEC-014 Completion Summary — Explainer, conjugation-table, and personal-question renderers

**Traceability:** INIT-006/SPEC-014
**Status:** complete (orchestrator closed 2026-10-10 — 809 suite pass; chrome verb-workshop smoke pass)
**Domain:** frontend
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `shared/ui/blocks/explainer.js` — headings, paragraphs, root-highlighted examples, a plain table, callouts whose tone is a word as well as a colour. No pictures.
- [x] `shared/ui/blocks/conjugation-table.js` — study tables and an untimed drill. Header comment: form signals never create cards; never drill an unverified form.
- [x] `shared/ui/blocks/personal-question.js` — Hebrew prompt, English toggle, say-aloud, optional text box. The lesson blob stores a done boolean only.
- [x] Registry entries on `HebrewPlanRender.blocks`, and screens in `renderScreens` for these three types. Verse and letter stay unregistered.
- [x] Styles appended in `shared/worksheet.css` (ADR-0006 tokens).
- [x] Tests: `test/render-explainer.test.js`, `test/render-conjugation.test.js`, `test/chrome-verb-workshop-smoke.js`.

## Acceptance Criteria

- [x] ac-1 (REQ-011): hostile strings in explainer and conjugation fields render as escaped text. No markup, no picture.
- [x] ac-2 (REQ-011): the drill queue is verified cells only. An unpointed answer passes. A wrong answer says "try again" once, then shows the form. Each attempt is one `kind: 'form'` row. No new review row.
- [x] ac-3: present headers are SM, SF, PM, PF. Past study rows are אני, אתה, את, הוא, היא, אנחנו, אתם, אתן, הם/הן.
- [x] ac-4: after Done, the only new `/api/` call is `PUT /api/lessons/:slug/progress`, and the body holds a boolean. The typed answer stays in the input.

## AC Verification (independent)

Mechanized checks were run in this implementing session. A fresh read-only verifier was **not** dispatched (this run is already a subagent; nesting one was skipped and disclosed). Status stays `in_review` until the orchestrator records an independent all-PASS.
**Eval notes:** [`SPEC-014-eval-report.md`](SPEC-014-eval-report.md)

**Verification tier:** `full` · **pre-flight:** dispatch did not include an AC-verification-tier line, so the tier is full · gate_actions=`[]` `pass` · security_review=`standard` `fail` (light-tier condition) · domain=`frontend` `fail` (light-tier condition) · autonomy_level=`auto_tests` `fail` (light-tier condition) ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS (in-session) | `node --test test/render-explainer.test.js test/render-conjugation.test.js` exit 0. Hostile `<script>alert(1)</script>` is absent as markup and present escaped in explainer fields (12 or more) and conjugation fields. No `<img>`, no `onerror=`. |
| ac-2 | PASS (in-session) | `node test/chrome-verb-workshop-smoke.js` exit 0. Unverified `חָשַׁב` is not a drill cell. Unpointed `חשבתי` passes `חָשַׁבְתִּי`. `WRONG-XYZ` prompts "try again", then reveals `חָשַׁבְתָּ`. `practice_events` has 3 rows, all `kind = 'form'`, `activity = 'conjugation-drill'`, lemma `לַחְשֹׁב`. `review_items` count unchanged. Signal posts do not contain the typed strings. |
| ac-3 | PASS (in-session) | `test/render-conjugation.test.js` study-table test. Present `<th>` order SM, SF, PM, PF. Past table has 9 `<tr>` and the person labels in order. The smoke also checks SM, SF, PM, PF on the published study table. |
| ac-4 | PASS (in-session) | Same smoke. After Done, the only new `/api/` request is one progress PUT. `activityState['personal-question'].you === true`. The stored blob and the request body do not contain `pq-secret-typed`. The input still holds that string. |

---

## Change Produced

- **Title:** Guides and verb workshops render explainer sections, conjugation tables, and personal questions.
- **Files changed:** 6 code files, 3 tests, this summary, the eval note, and the spec status.
- **Authored by:** INIT-006/SPEC-014
- **Summary:** Three block renderers join the INIT-004 registry. Study mode shows the present table as four columns and the past table as nine person rows, with root letters marked and "not yet checked" on an unverified form. Drill mode asks for one verified form at a time, with a letter pad, one retry, then the form. A pass or a miss posts `kind: 'form'` and does not create a card. A personal question can be said or typed on the page; Done writes only a boolean into the lesson blob. There is no timer and no picture on these blocks.

## Verification

| Command | Result |
| --- | --- |
| `node --test test/render-explainer.test.js test/render-conjugation.test.js test/plan-render.test.js` | 29 pass, exit 0 |
| `node test/chrome-verb-workshop-smoke.js` | exit 0 (`chrome-verb-workshop-smoke: PASS`) |
| `npm test` | 809 pass, 0 fail, exit 0 |

**Coverage:** this repo has no coverage tool. The spec's pass condition is those commands exiting 0, form events on the server, and no new review rows.

## Gates & Controls

| Gate | Reversibility | Approval | Blocking checks | Outcome |
| --- | --- | --- | --- | --- |
| — | branch | n/a | the three tests and `npm test` | None. `gate_actions` is empty. No new npm dependency, no migration. Rollback: unregister the three renderers; plans with these blocks show no block body. |

## Security Review

- **Depth:** standard, focus DOM insertion. **Reviewer:** this session (`self_reviewed: true`). A separate security-review subagent was not dispatched.
- **SAST / secret scan:** not run — none configured for this repo.
- **Findings:** none. Critical 0, High 0, Medium 0, Low 0.
- Plan strings go through `HebrewUtils.escapeHtml` (the same helper as INIT-004). Block modules do not call `innerHTML`, `insertAdjacentHTML`, or `document.write`. Drill feedback, the revealed form, and the saved note are assigned with `textContent`. The typed answer is left in the input.
- The page paint still uses the INIT-004 assignment `mount.innerHTML = renderScreens(...)`. That string is the escaped renderer output. Hostile tests reject a raw tag, an `onerror` handler, and an image.
- Callout `data-tone` is one of `green`, `yellow`, `red`, `info`. A root is highlighted only when it matches the lexicon root pattern. An audio path is a button state, not a URL in the page. Lexicon fetches use `encodeURIComponent` on a length-capped Hebrew key.
- A form signal is `kind`, `word` (the form key), `lemma`, `result`, `direction`, `activity`, and `slug`. It has no typed field. Only a verified form is queued.
- Personal-question Done merges a boolean into `activityState['personal-question']`. Non-boolean marks under that key are dropped. Say-aloud uses `speechSynthesis` on the prompt's `textContent` and does not fetch.

## Files Changed

| File | Change | Provenance |
| --- | --- | --- |
| `shared/ui/blocks/explainer.js` | Created — explainer renderer and root highlighting | INIT-006/SPEC-014 |
| `shared/ui/blocks/conjugation-table.js` | Created — study table, drill queue, form signals | INIT-006/SPEC-014 |
| `shared/ui/blocks/personal-question.js` | Created — prompt, say-aloud, done boolean | INIT-006/SPEC-014 |
| `shared/ui/plan-render.js` | Registry, program screens, lexicon load for conjugation lemmas | INIT-006/SPEC-014 |
| `shared/plan/index.html` | Loads the block scripts and the progress client | INIT-006/SPEC-014 |
| `shared/worksheet.css` | Appended callout, table, root, and letter-pad rules | INIT-006/SPEC-014 |
| `test/render-explainer.test.js` | Created — ac-1, callout contrast, personal-question blob | INIT-006/SPEC-014 |
| `test/render-conjugation.test.js` | Created — ac-2 logic, ac-3 | INIT-006/SPEC-014 |
| `test/chrome-verb-workshop-smoke.js` | Created — ac-2 server rows, ac-4 request watch | INIT-006/SPEC-014 |

## Deviations

- **Persona.** `.claude/agents/principal-frontend-developer/AGENT.md` is not in this repo. Disclosed on the spec. No AGENT.md was invented. The screens follow ADR-0006 tokens, bilingual labels, and escaped text.
- **Timer and run-trace.** This spec was dispatched to a subagent. The orchestrator owns Start/Complete Timer and `open_trace` / `close_trace`. Those steps were not run here.
- **Commit / push / PR.** Skipped. The dispatch said not to commit.
- **Independent verifier.** Not nested from this subagent. Mechanized results are recorded above; the spec stays `in_review`.
- **Callout colour.** ADR-0006 has no yellow or red token. Green uses `color.control` / `color.controlText`. Check uses text on surface. Stop uses background on text. Note uses muted on surface. Each pair is at least 4.5:1 in light and dark. The English word (Go, Check, Stop, Note) is the non-colour signal.
- **Drill across tables.** The first drill block in a lesson owns one interleaved queue of every drill table in that lesson. Later drill blocks in the same lesson show their sentences and questions only.
- **Lesson slug.** Signals and the done blob use the plan lesson id when it is a slug. The smoke lesson id is `public-lesson`, which the fixture server already marks visible. A plan lesson id the learner cannot see would be rejected by the existing signal and progress routes.
- **Verse and letter.** Not implemented. `renderBlock` of those types returns an empty string.
- **Starter content.** Not written.
- **No new npm dependency.**

## Trace Emission Degradation

Skipped — the orchestrator owns `open_trace` / `emit` / `close_trace` for a dispatched spec. No events were written by this session. Impact: the Tier-5 invariants check and the model-dispatch check have no live trace from this subagent until the orchestrator opens and closes one.

## Harness Adaptation

Cursor session, already running as a subagent. Phase 4 security review and Phase 4.5 AC verification were done in this session (`self_reviewed: true` for security; AC verdicts are mechanized but not from a fresh verifier). Not a T4 escalation.

## Follow-ups

- SPEC-015 can reuse `drillQueue`, `decideAttempt`, and `formSignal` for the mix drill. This spec did not render the mix.
- SPEC-016 owns verse-reading, letter-intro, and the Guides shelf. Those types are still unknown to the registry.
- SPEC-018 owns the first real guide and verb workshop. This spec publishes nothing but the smoke's temporary plan.
