# SPEC-009 AC Verification Eval Report — Library and flashcard routes

**Traceability:** INIT-006/SPEC-009 \
**Verifier:** independent sub-agent (general-purpose, not the author) · model `composer-2.5-fast` · temperature 0 \
**Evaluated at:** 2026-10-11T02:55:00Z \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | A fixture learner with 5 reviewed words and 2 added words gets 7 library rows with lexicon tags and the correct stage per direction. | PASS | Test `ac-1: five reviewed words and two added words are seven tagged rows` passed. Five `POST /api/review/signals` passes, two `POST /api/me/library` adds, `GET /api/me/library` returns 7. Reviewed rows are `he_en.stage === 'learning'` with lexicon `pos` / verb `binyan` and `root`. Added rows are `new` in both directions. `test/library-routes.test.js:103`. `node --test test/library-routes.test.js` → 10 pass, 0 fail, exit 0. |
| ac-2 | POST without a valid origin returns 403. With one, it creates two review rows per word and is idempotent. A key with no lexicon or vocab entry and no gloss returns 400. With a gloss, the note is stored on `user_words` and the lexicon row count is unchanged. After an admin lexicon entry, the list shows the lexicon gloss and the learner note. | PASS | Test `ac-2: origin, idempotent add, personal gloss, then lexicon gloss plus note` passed. No Origin → 403 `csrf`. Missing gloss → 400 `gloss_required`. Add creates two `fsrs_state='new'` rows. Repeat → `created: 0`. `lexicon_entries` count unchanged, then the list `gloss_en` is the lexicon gloss and `note` is the learner gloss. `test/library-routes.test.js:172`. Same command, exit 0. |
| ac-3 | `GET /api/me/cards?deck=weak` returns only weak or very weak words. Each card has four intervals. | PASS | Test `ac-3: weak deck is only weak or very weak words, each with four intervals` passed. Deck words are the weak and very-weak fixtures only. Every card has interval keys `1`–`4` as numbers. `test/library-routes.test.js:236`. Same command, exit 0. |
| ac-4 | A suspended word appears in the library with `suspended: true` and in no deck. | PASS | Test `ac-4: a suspended word stays in the library and leaves every deck` passed. Before suspend the word is in `due`, `new`, and `all`. After `PATCH`, `GET /api/me/library?suspended=1` shows `suspended: true`, and `due`, `weak`, `new`, `all`, and `lesson:public-lesson` omit it. `test/library-routes.test.js:297`. Same command, exit 0. |
| ac-5 | A signed-out request to any route returns 401. Another learner's words never appear. | PASS | Test `ac-5: signed-out requests are 401 and learners cannot see each other` passed. Signed-out GETs, POST, and PATCH return 401 `no_session`. A body `userId` is rejected and does not write the other user's row. Each session's library and `deck=all` omit the other user's word. `test/library-routes.test.js:339`. Same command, exit 0. |

## Definition of done

- [x] `server/learner/library.js`, `server/learner/cards.js`, `server/api/library-routes.js` exist and the routes are mounted from `server/web.js` (the process that owns `createRouter()`).
- [x] `test/library-routes.test.js` names ac-1 through ac-5 and exits 0.
- [x] Session required on the library, card, lexicon, and grammar routes. CSRF on writes is the existing `server/web.js` origin check.
- [x] No MCP tools were added.

## Tests run by the verifier

```text
node --test test/library-routes.test.js
→ 10 pass / 10 total ; exit 0
```

The repo has no coverage tool configured. Coverage was not measured.

---

## Remediation log (feedback loop)

No acceptance criterion failed. The verifier noted that PATCH CSRF was implemented in `server/web.js` and not named in a test. That note did not fail an AC. After the verdict, the implementer added a no-Origin PATCH assertion to the ac-4 test and re-ran the file: 10 pass, exit 0.

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-###/SPEC-NNN · files) | Re-verify result |
| ---- | ------------- | ------------------- | ------------------------------- | ---------------- |
| — | none | — | — | first pass all-PASS |

---

## Notes

- The verifier is a fresh sub-agent that did not author the code. This report transcribes that verdict. The verifier was told not to edit files, so this file was written by the executing agent from the verdict.
- Graded-signal coverage hits the existing `POST /api/review/signals`. The spec text says `POST /api/progress/signals`. That path does not exist; the client posts to `/api/review/signals`.
- No AC was passed by weakening a test or the criterion.
