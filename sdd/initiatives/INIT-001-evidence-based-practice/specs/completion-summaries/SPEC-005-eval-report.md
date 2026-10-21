# SPEC-005 AC Verification Eval Report — Production recall in lessons

**Traceability:** INIT-001/SPEC-005 \
**Verifier:** independent readonly sub-agent · model `composer-2.5-fast` · temperature 0 \
**Evaluated at:** 2026-10-08T17:45:00Z \
**Verdict:** PASS · **Remediation loops:** 1

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | Recall round English-first card; Hebrew hidden until reveal; grades `en→he` | PASS | `words.js` en_he branch + `review.record(..., 'en_he', ...)`; tests `marks en_he when grading a production card`; Chrome smoke `hebrewHiddenBeforeReveal` + `enHeLastResult: pass` |
| ac-2 | `type-it` at intermediate; nikud-insensitive; records `en→he` | PASS | `available()` + catalog `levels`; tests level gate, `normalizeHebrew`, `records en_he pass/miss on check` |
| ac-3 | Round 1 unchanged (Hebrew + meaning shown) | PASS | `!recall` card path; test `shows Hebrew and meaning together when not in recall mode` |
| ac-4 | No lesson file changes required | PASS | Behavior in shared activities; smoke seeds localStorage only; existing lessons still validate |

## Definition of done

- [x] Owned activity files implement production recall / wider type-it — verified by file read + tests
- [x] Tests updated / smoke green — `npm test` exit 0; chrome production smoke ok
- [x] Records go to `en_he` via `ctx.review.record` — unit + smoke

## Tests run by the verifier

```text
npm test
→ 74+ passed / 0 failed (exit 0) on remediation re-verify; implementer confirmation 78/78 after suite growth

node test/chrome-production-recall-smoke.js  (implementer; server :8080)
→ {"ok":true,"hebrewHiddenBeforeReveal":true,"enHeLastResult":"pass","direction":"en_he"}
```

---

## Remediation log (feedback loop)

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-001/SPEC-005 · files) | Re-verify result |
| ---- | ------------- | -------------------- | ------------------------------- | ---------------- |
| 1 | ac-2, ac-3 | Missing mechanized evidence for nikud match, en_he record, round-1 dual display; over-read “offered” as auto-inject | Added tests in `test/production-recall.test.js` (matching, record, round-1 HTML); clarified offered = `available`/catalog gate | PASS |

---

## Notes

- Loop-0 also saw flaky / concurrent SPEC-006 failures; remediation re-verify and implementer `npm test` were exit 0.
- Auto-injecting `type-it` into every intermediate lesson would require `core.js` (out of owned-file scope); gate-level availability matches the spec deliverables.
- No AC was passed by weakening a test or the criterion.
