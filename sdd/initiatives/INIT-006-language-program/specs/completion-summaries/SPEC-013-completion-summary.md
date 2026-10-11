# SPEC-013 Completion Summary — Words library screen

**Traceability:** INIT-006/SPEC-013
**Status:** complete (orchestrator closed 2026-10-10 — suite 787/787; chrome words + navigation smokes passed per implementer)
**Domain:** frontend
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `shared/words-page.js`
- [x] `shared/app-routes.js` route `{ id: 'words', path: '/words/' }`, nav label מילים (Words)
- [x] `server/spa.js` shell path `/words/`
- [x] `shared/worksheet.css` words styles on ADR-0006 tokens
- [x] `test/chrome-words-smoke.js`
- [x] `test/spa-shell.test.js` and `test/chrome-navigation-smoke.js` cover the route

## Acceptance Criteria

- [x] ac-1 (REQ-005): Chrome smoke signs in, opens `/words/`, sees `data-route-ready="words"`, and finds a seeded word with its gloss and stage.
- [x] ac-2 (REQ-005): `binyan=hifil` shows only hifil-tagged words; the URL carries the filter; Back restores the previous list.
- [x] ac-3 (REQ-005): adding a word shows it in the list as `new`; suspending hides it from the Due deck.
- [x] ac-4: `test/spa-shell.test.js` and `test/chrome-navigation-smoke.js` cover the new route.

## AC Verification (independent)

A fresh non-implementer verifier was **not** dispatched from this subagent. The rows below are the implementer's mechanized runs, not an independent PASS. Spec status stays `in_review` until that verifier (or the orchestrator) records all-PASS.

**Verification tier:** `full` · **pre-flight:** dispatch did not include an AC-verification-tier line, so the tier is full · gate_actions=`[]` `PASS` · security_review=`standard` `FAIL` (light-tier condition) · domain=`frontend` `FAIL` (light-tier condition) · autonomy_level=`auto_tests` `FAIL` (light-tier condition) ⇒ light_tier_eligible=`false` · **checks skipped:** `None` (the implementer ran the smokes; the independent verifier itself was not run)

| AC | Implementer evidence | Command |
| ---- | -------------------- | ------- |
| ac-1 | Ready signal `words`, gloss `peace`, stage `new` on שָׁלוֹם | `node test/chrome-words-smoke.js` exit 0 |
| ac-2 | URL `?binyan=hifil`, list is only הִסְבִּיר, Review-these deck `concept:binyan:hifil`, Back clears the query and restores שָׁלוֹם | same command, exit 0 |
| ac-3 | Added כֶּלֶב gloss `dog` stage `new`; after Suspend, Due cards are the two seeded words only (כֶּלֶב absent, שָׁלוֹם still present) | same command, exit 0 |
| ac-4 | Route table, shell path, and query preservation in `test/spa-shell.test.js` (19 pass). Navigation smoke reaches Words in the same document with the ready signal, one heading, and focus on the h1 | `node --test test/spa-shell.test.js` exit 0; `node test/chrome-navigation-smoke.js` exit 0 |

The words smoke also opened the hifil verb and found the unverified past form marked `not yet checked`.

---

## Change Produced

- **Authored by:** `INIT-006/SPEC-013`
- **Title:** Words screen for the learner's personal library
- **Files changed:** the new page and smoke, plus the route table, shell, router query string, API abort signal, and styles
- **Change status:** proposed
- **Reversibility:** branch

| File | Action |
| ---- | ------ |
| `shared/words-page.js` | added |
| `shared/app-routes.js` | words route after Review |
| `shared/app-shell.js` | nav item מילים (Words), no icon |
| `shared/layout.js` | heading icon omitted when the route has none |
| `shared/router.js` | history URL keeps the query string |
| `shared/api.js` | `request` forwards `signal` so unmount aborts fetches |
| `server/spa.js` | `/words/` is a shell path |
| `shared/worksheet.css` | words layout, chips, strength bar, root-class border |
| `test/chrome-words-smoke.js` | added |
| `test/chrome-navigation-smoke.js` | Words is one of the screens |
| `test/spa-shell.test.js` | path list and query-string test |
| `test/web-gating.test.js` | signed-out `/words/` is refused |

`shared/review-page.js` was not edited.

---

## Tests

| Test | Result |
| ---- | ------ |
| `node test/chrome-words-smoke.js` | PASS (exit 0), including a re-run after the audio URL guard |
| `node test/chrome-navigation-smoke.js` | PASS (exit 0); Words ready signal in the same document |
| `node --test test/spa-shell.test.js` | 19 pass, 0 fail |
| `node --test test/web-gating.test.js test/app-shell.test.js test/design-tokens.test.js` | 22 pass, 0 fail |
| `npm test` (`node --test test/*.test.js`) | 787 pass, 0 fail |

**Coverage (new code):** not measured. This repo has no coverage tool. The spec's pass condition is exit 0 for the smokes.

Chrome smokes are scripts, not `test/*.test.js`, so `npm test` does not run them. They were run on their own.

---

## Gates & Controls

None — branch/reversible only. No `gate_action` is configured. No commit, push, or pull request (dispatch said to skip them). No new npm dependency.

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ----------- | ------------- | --------- | --------------- | ----- |
| none | branch | — | words smoke, navigation smoke, `npm test` | Rollback is to remove the `words` route entry and `/words/` from `APP_PATHS`. |

---

## Security Review

- **Sensitivity / depth:** standard · quick, focus DOM insertion · **method:** implementer review (`self_reviewed: true`). A separate security-review subagent was not spawned from this dispatch.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0 unresolved
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| Library `audio` played through an element `src` | Low | DOM URL injection | `shared/words-page.js` `audioHref` | Remediated before the final smoke: only same-origin `http:`/`https:` URLs are passed through. `javascript:` and other origins become no URL, and speech falls back to speaking the word. |
| Signed-out `/words/` | — | session gate | `server/spa.js`, `test/web-gating.test.js` | Confirmed 302 to login, same as the other shell paths. |

The page builds the list, detail, and forms with `createElement`, `textContent`, and `setAttribute`. It does not assign library or lexicon strings with `innerHTML`. Hebrew nodes set `lang="he"` and `dir="rtl"`. Lesson links are emitted only when the source matches the deck slug pattern. Filter values are written with `URLSearchParams` and read back as form control values, not as HTML. Writes go to `/api/me/library` with the session cookie; the browser sends `Origin`, which is the existing CSRF check. The client does not send a user id.

---

## Harness Adaptation

Cursor session, dispatched as a subagent. The execute-spec Phase 4.5 verifier and Phase 4 security-review agent were not spawned here, so those two checks are implementer-run rather than T2 independent reviews. Recorded above. No `harness_fallback` trace event was emitted (trace ownership is the orchestrator's).

---

## Trace Emission Degradation

| Site | Event | Reason | What was lost | Impact |
| ---- | ----- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | `open_trace`, `emit`, `close_trace` | Dispatched subagent. execute-spec Phase 0 step 2: the orchestrator owns the timer and the run-trace sentinel. | This spec's events are not in `.sdd-trace/SPEC-013.jsonl` from this agent. | The Tier-5 invariants check and the model-dispatch check have nothing to read until the orchestrator opens and closes the trace. |

---

## Deviations

- **Persona.** `primary_prompt` `.claude/agents/principal-frontend-developer/AGENT.md` is not in this repo. The spec already names that degradation. The page follows the existing SPA view contract (`mount` / `unmount`, ready signal, `ctx.signal`) and ADR-0006 tokens.
- **Transliteration.** The add form collects an optional transliteration and sends it on `POST /api/me/library`. SPEC-009 `library.add` stores `gloss_en` only, so a brand-new word does not get that transliteration back. The list shows `translit` when the lexicon or vocab index already has one.
- **Icon.** `shared/icons.js` has no `words` icon (INIT-005). Nav and the page heading are text only (und-2).
- **Query string.** `shared/router.js` keeps a query on the history URL, including the first load. A same-path Back (filter change) is handled by the words page, because the router still ignores same-path `popstate` so admin hash tabs keep working.
- **Review.** `shared/review-page.js` was not rewritten. "Review these" is an ordinary link: `/review/` when no grammar or weak filter is set, `/review/?deck=weak` for the weak filter, `/review/?deck=concept:binyan:…` or `concept:tense:…` when those filters are set. A binyan filter wins over weak. Lesson decks are links on a word's lesson sources (`lesson:<slug>`), not on the source dropdown value `lesson`.
- **Discipline profile:** `software` (initiative doc).
- **Git.** No commit, push, or pull request.

---

## Follow-ups

- Orchestrator: independent AC verification, then the run-trace close, before `complete`.
- SPEC-012 can read `?deck=` on Review. This spec only links there.
