# SPEC-007 AC Verification Eval Report — Story glosses and coverage estimate

**Traceability:** INIT-001/SPEC-007  
**Verifier:** independent readonly sub-agent · model `composer-2.5-fast` · temperature 0  
**Evaluated at:** 2026-10-08  
**Verdict:** PASS · **Remediation loops:** 1

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence |
| ---- | ----------- | ------- | -------- |
| ac-1 | Tap word → English meaning; tap elsewhere closes | PASS | `lookupGloss` / `openGloss` (`story.js`); unit `ac-1: tapping elsewhere closes`; smoke `popoverClosedByElsewhere` + vocab meaning |
| ac-2 | `not_yet` at most once per word per day | PASS | `recordNotYet` same-day guard; unit double-tap + higher-step preserve; smoke `lastResult: not_yet`, `lapses: 0` |
| ac-3 | Coverage estimate; empty store → unknown not 0% | PASS | `estimateCoverage` + badge; unit `ac-3`; smoke `coverageUnknown: true` |
| ac-4 | Lessons without `reading.glosses` still gloss vocab | PASS | Vocab fallback in `lookupGloss`; smoke on `my-morning` (no gloss map) |
| ac-5 | Gloss HTML escaped | PASS | `U.escapeHtml` in popover; unit fixture `<b>` → `&lt;b&gt;`; smoke rejects raw tags |

## Definition of done

- [x] `shared/activities/story.js`, `shared/worksheet.css` (append after SPEC-004), `shared/player/utils.js` `tokenizeHebrew`
- [x] Tests: `test/tokenize-hebrew.test.js`, `test/story-gloss.test.js`, `test/chrome-story-gloss-smoke.js`
- [x] Escape dismisses; RTL-aware popover; story audio paths unchanged

## Tests run by the verifier

```text
npm test
→ 94/94 pass (exit 0)

node test/chrome-story-gloss-smoke.js
→ {"ok":true,"popoverClosedByElsewhere":true,"meaning":"gets up (m. sg.)",...}
```

---

## Remediation log (feedback loop)

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-001/SPEC-007 · files) | Re-verify result |
| ---- | ------------- | -------------------- | ------------------------------- | ---------------- |
| 1 | ac-1 | Tap-elsewhere dismiss implemented but not mechanized (only Escape smoke-tested) | Added unit `ac-1: tapping elsewhere closes`; smoke asserts `popoverClosedByElsewhere` | PASS |

---

## Notes

- No AC was passed by weakening a test or the criterion.
- SPEC-008 local `tokenizeHebrew` left untouched (prefer shared helper in `utils.js`; no validate.js edit).
