# SPEC-012 Completion Summary — Four-grade flashcards on Review

**Traceability:** INIT-006/SPEC-012
**Status:** complete (orchestrator closed 2026-10-10 — unit + chrome smokes + suite 787/787) (mechanized AC checks passed in this session; a fresh independent verifier and the run-trace close belong to the orchestrator)
**Domain:** frontend
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `shared/review-page.js` — Review is a decked flashcard session. Header comment: grades go to the server; never schedule in the browser.
- [x] `shared/player/flashcard-answer.js` — vowel-stripped comparison, grade suggestion, key map, signal builder that omits typed text.
- [x] Styles appended in `shared/worksheet.css` (ADR-0006 tokens). The SPEC-013 words block above that section was not edited.
- [x] Tests: `test/flashcard-answer.test.js`, `test/chrome-flashcards-smoke.js`; `test/chrome-review-smoke.js` updated for the new hooks.

## Acceptance Criteria

- [x] ac-1 (REQ-003): Chrome smoke seeds two library words (four direction-cards), presses Space then a grade key, and the server row for one of those cards has `last_grade` 3.
- [x] ac-2 (REQ-003): the same smoke dispatches keys 1, 2, 3, and 4 (Space before each) and checks four `data-days` labels against `intervals`.
- [x] ac-3: before Show answer, the answer string is not in the stage text or in `data-speak` / `data-vowels` / `data-audio` / `data-key`. The answer node is created only after reveal.
- [x] ac-4: unit test covers pointed vs unpointed, an `accept[]` variant, and a wrong answer.
- [x] ac-5: `test/chrome-navigation-smoke.js` reaches Review with `data-route-ready="review"`.

## AC Verification (independent)

Mechanized checks were run in this implementing session. A fresh read-only verifier was **not** dispatched (this run is already a subagent; nesting one was skipped and disclosed). Status stays `in_review` until the orchestrator records an independent all-PASS.
**Eval notes:** [`SPEC-012-eval-report.md`](SPEC-012-eval-report.md)

**Verification tier:** `full` · **pre-flight:** dispatch did not include an AC-verification-tier line, so the tier is full · gate_actions=`[]` `pass` · security_review=`standard` `fail` (light-tier condition) · domain=`frontend` `fail` (light-tier condition) · autonomy_level=`auto_tests` `fail` (light-tier condition) ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS (in-session) | `node test/chrome-flashcards-smoke.js` exit 0. Server `last_grade` 3 on `קָם` / `en_he`. Signals `[1, 2, 3, 4]`. Default scheduler is ladder; the screen only sends `grade`. |
| ac-2 | PASS (in-session) | Same smoke. Keys 1–4 each produced a signal. The last graded card's four `data-days` values matched that card's `intervals` from `GET /api/me/cards?deck=due`. |
| ac-3 | PASS (in-session) | Same smoke rejects the step if the answer is in stage text or in the listed attributes before Space. `test/chrome-review-smoke.js` exit 0: no `#review-answer` before the click; after it, the answer node has no element children and `innerHTML === textContent`. |
| ac-4 | PASS (in-session) | `node --test test/flashcard-answer.test.js` — 5/5, exit 0. Pointed `בֹּקֶר` vs `בקר`, accept `בוקר`, wrong `קם`. |
| ac-5 | PASS (in-session) | `node test/chrome-navigation-smoke.js` exit 0. Review entry: `data-route-ready` path `/review/`, title `חזרה היום — Review today`. |

---

## Change Produced

- **Title:** Review is four-grade flashcards (decks, both directions, audio, keyboard) with server-authoritative grades.
- **Files changed:** 8 code/test files, plus this summary, the eval note, and the spec status.
- **Authored by:** INIT-006/SPEC-012
- **Summary:** The Review route loads a deck from `GET /api/me/cards?deck=`, shows the prompt until Space / Show answer, then four buttons labeled with the card's `intervals`. Grades 1–4 are posted through `HebrewSync.queueSignal` (`miss` for 1, `pass` for 2–4). English-to-Hebrew cards can be typed; the comparison is local and the typed string is not sent. The session summary counts cards and grades. No points and no streak reward.

## Verification

| Command | Result |
| --- | --- |
| `node --test test/flashcard-answer.test.js test/review-page.test.js` | 17 pass, exit 0 |
| `npm test` | 787 pass, 0 fail, exit 0 |
| `node test/chrome-flashcards-smoke.js` | exit 0 (re-run after the answer-attribute fix) |
| `node test/chrome-review-smoke.js` | exit 0 |
| `node test/chrome-navigation-smoke.js` | exit 0, Review ready |

**Coverage:** this repo has no coverage tool. The spec's pass condition is the commands above exiting 0 and the server row showing the grade.

## Gates & Controls

| Gate | Reversibility | Approval | Blocking checks | Outcome |
| --- | --- | --- | --- | --- |
| — | branch | n/a | unit tests, Chrome smokes, `npm test` | None. `gate_actions` is empty. No new npm dependency, no migration. Rollback: revert the view; the server still accepts gradeless signals. |

## Security Review

- **Depth:** standard (quick), focus DOM text insertion. **Reviewer:** this session (`self_reviewed: true`). A separate security-review subagent was not dispatched.
- **SAST / secret scan:** not run — none configured for this repo.
- **Findings:** none. Critical 0, High 0, Medium 0, Low 0.
- Card, lexicon, catalog, and concept strings are assigned with `textContent` or `createTextNode`. `shared/review-page.js` has no `innerHTML`, `insertAdjacentHTML`, `outerHTML`, or `document.write`.
- Clip URLs are resolved and kept only when the protocol is `http:` or `https:`.
- The grade signal is built field-by-field. `typed`, `prompt`, and `answer` are not copied. The unit test asserts the typed string is absent from the JSON.
- `localStorage` under `hebrew-review-prefs:v1` stores only `vowelsOn` and `translitOn`. It does not store a schedule.
- `unmount()` removes the document `keydown` listener.

## Files Changed

| File | Change | Provenance |
| --- | --- | --- |
| `shared/review-page.js` | Replaced the two-grade local queue with decked four-grade flashcards | INIT-006/SPEC-012 |
| `shared/player/flashcard-answer.js` | Created — local comparison, suggestion, key map, signal builder | INIT-006/SPEC-012 |
| `shared/worksheet.css` | Appended flashcard rules using ADR-0006 tokens | INIT-006/SPEC-012 |
| `shared/app-routes.js` | Review route also loads `flashcard-answer.js`. Words route untouched | INIT-006/SPEC-012 |
| `test/flashcard-answer.test.js` | Created — ac-4 and the signal shape | INIT-006/SPEC-012 |
| `test/chrome-flashcards-smoke.js` | Created — ac-1, ac-2, ac-3 | INIT-006/SPEC-012 |
| `test/chrome-review-smoke.js` | Updated — reveal, text-only answer, four interval buttons | INIT-006/SPEC-012 |
| `test/review-page.test.js` | Assertions updated for textContent, four grades, and the load error | INIT-006/SPEC-012 |

## Deviations

- **Persona.** `.claude/agents/principal-frontend-developer/AGENT.md` is not in this repo. Disclosed on the spec. No AGENT.md was invented. The screen follows ADR-0006 tokens, bilingual labels, and `textContent`.
- **Timer and run-trace.** This spec was dispatched to a subagent. The orchestrator owns Start/Complete Timer and `open_trace` / `close_trace`. Those steps were not run here.
- **Commit / push / PR.** Skipped. The dispatch said not to commit.
- **Independent verifier.** Not nested from this subagent. Mechanized results are recorded above; the spec stays `in_review`.
- **Lesson slug on the signal.** `POST /api/review/signals` drops a signal whose slug the learner cannot see. The screen sends the word's lesson source, else the vocab-index lesson, else the first catalog slug. Library words for `בֹּקֶר` and `קָם` resolve to `my-morning` from the vocab index.
- **Again once more.** ADR-0009: an Again (1) card is shown once more before the session ends. A second Again is not requeued.
- **INIT-005 feedback kit** is `ready`, not landed. Buttons use ADR-0006 token styles, not that kit.
- **Words route (SPEC-013)** was not edited.

## Trace Emission Degradation

Skipped — the orchestrator owns `open_trace` / `emit` / `close_trace` for a dispatched spec. No events were written by this session. Impact: the Tier-5 invariants check and the model-dispatch check have no live trace from this subagent until the orchestrator opens and closes one.

## Harness Adaptation

Cursor session, already running as a subagent. Phase 4 security review and Phase 4.5 AC verification were done in this session (`self_reviewed: true` for security; AC verdicts are mechanized but not from a fresh verifier). Not a T4 escalation.

## Follow-ups

- SPEC-015 can open a deck by navigating to Review; the screen reads `GET /api/me/cards?deck=` and the deck controls are `data-deck` buttons plus lesson and concept selects (`lesson:<slug>`, `concept:binyan:*`, `concept:tense:*`).
- SPEC-013 can link "Review these" at the same deck query. This spec did not add a Words control.
