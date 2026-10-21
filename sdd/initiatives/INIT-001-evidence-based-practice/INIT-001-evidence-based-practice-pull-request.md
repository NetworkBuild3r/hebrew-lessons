# Pull Request: INIT-001 Evidence-Based Practice (INIT-001)

> Git PR artifact produced at initiative completion. Copy this into the GitHub PR body, or use it with the `git-create-pr` skill.
> **Guard:** this artifact is written, but `gh pr create` was NOT run. The developer opens the PR.
> **Scope warning:** the working tree also contains unrelated uncommitted work (see "Out of scope" below). Split it into its own PRs before opening this one, or exclude those paths.

**Title:** `INIT-001: Evidence-based practice: spaced review, production recall, glosses, retry feedback (frontend + service + docs)`
**Branch:** `main` (uncommitted working tree; create a feature branch before committing) → `main`
**Initiative:** INIT-001-evidence-based-practice · **Ticket:** none (guarded)

---

## Summary

Turns the research in RSCH-003 into product changes for the Hebrew practice site. Learners now get spaced recall of every word they have met, across all lessons, from a "Review today" page with a home card. Production recall (English to Hebrew) is added to word rounds. Pattern, fill, and listen-pick give one retry before revealing the answer. Story text supports tap-to-gloss and an estimated coverage badge. Validator nudges flag weak authoring (recycling, encounters, coverage, glosses). The learner-review model is recorded in ADR-0001.

**Also in this PR:** a security fix. Lesson `worksheet.js` files were evaluated in Node `vm`, which allowed code execution in the server process. The loaders now parse lesson data as literals and never run it.

## Type of change

- [x] Feature  ·  [ ] Bug fix  ·  [ ] Refactor  ·  [ ] Infrastructure  ·  [x] Docs  ·  [ ] Breaking change
- [x] Security fix (lesson-data loader, HIGH-1)

---

## Work performed (by spec)

| Spec | Domain | Change | Files (+/-) |
| ---- | ------ | ------ | ----------- |
| SPEC-001 | docs | ADR-0001 learner review model accepted | +1 ADR (~350 lines) |
| SPEC-002 | service | Vocab index generator (`server/vocab-index.js`), CLI, guarded rebuild after `upsert_lesson` | +3 / ~2 |
| SPEC-003 | frontend | Browser review store (`shared/player/review.js`), ADR-0001 scheduler, words self-marks, `wordMarks` migration | +2 / ~3 |
| SPEC-004 | frontend | Review today page (`review/`, `shared/review-page.js`), home due card, `/review/` allowlist | +3 / ~3 |
| SPEC-005 | frontend | Production recall in word rounds; type-it widened to intermediate | +2 / ~3 |
| SPEC-006 | frontend | Retry-before-reveal on pattern, fill, listen-pick | +1 / ~3 |
| SPEC-007 | frontend | Story tap-to-gloss, `not_yet` signal, coverage badge, shared `tokenizeHebrew` | +3 / ~2 |
| SPEC-008 | service | Validator nudges and warnings in the `upsert_lesson` reply | +1 / ~2 |
| SPEC-009 | docs | Authoring guide: evidence rules, glosses schema, Review today | 0 / ~3 |
| Remediation | service | Literal-only lesson-data loader (`server/lesson-data.js`), applied to all lesson-data loaders | +2 / ~5 |

## Tests done

| Spec | Test file(s) | New tests | Coverage | Result |
| ---- | ------------ | --------- | -------- | ------ |
| SPEC-002 | `test/vocab-index.test.js` | 6 | not measured | pass |
| SPEC-003 | `test/review-scheduler.test.js`, `test/review-store.test.js` | 19 | not measured | pass |
| SPEC-004 | `test/review-page.test.js`, `test/chrome-review-smoke.js` | 12 + 1 smoke | not measured | pass |
| SPEC-005 | `test/production-recall.test.js`, `test/chrome-production-recall-smoke.js` | not itemized + 1 smoke | not measured | pass |
| SPEC-006 | `test/retry-before-reveal-smoke.test.js` | 4 (smoke) | not measured | pass |
| SPEC-007 | `test/tokenize-hebrew.test.js`, `test/story-gloss.test.js`, `test/chrome-story-gloss-smoke.js` | 8 + 7 + 1 smoke | not measured | pass |
| SPEC-008 | `test/validate-nudges.test.js` | 8 | not measured | pass |
| Remediation | `test/lesson-data.test.js` | 20 | not measured | pass |
| **Total** | | **114 suite pass** | not measured | **all passing (0 fail)** |

Repo quality gate: `npm test` (`node --test test/*.test.js`), 114 tests, 0 failures. `prek` and `nox` are not configured in this repo, so they were not run. `# type: ignore` scan: N/A (JavaScript).

## Smoke tests executed

- [x] Headless Chrome, review page: reveal, "Knew it", store due date advances (`test/chrome-review-smoke.js`, ok)
- [x] Headless Chrome, production recall: en_he card and review store (`test/chrome-production-recall-smoke.js`, ok)
- [x] Headless Chrome, story gloss: tap, meaning, `not_yet`, Escape and elsewhere-dismiss (`test/chrome-story-gloss-smoke.js`, ok)
- [x] Retry-before-reveal: wrong once and twice, miss recorded, reload restores state (`test/retry-before-reveal-smoke.test.js`, pass)
- [ ] Post-deploy check: not deployed under the guard. After deploy, run `curl http://127.0.0.1:8080/review/` and expect 200 with the review page, and open a lesson to confirm the retry and gloss behavior.

## Eval reports

- **AC verification (independent):** 9/9 specs PASS. Per spec: SPEC-001 PASS, SPEC-002 PASS, SPEC-003 PASS, SPEC-004 PASS (after loop 1), SPEC-005 PASS (after loop 1), SPEC-006 PASS, SPEC-007 PASS (after loop 1), SPEC-008 PASS, SPEC-009 PASS. See `specs/completion-summaries/SPEC-NNN-eval-report.md`.
- **Security review:** initiative-wide, targeted at the changed code. Critical 0 · High 2 (HIGH-1 fixed in this PR; SEC-OPS-1 is an operator action) · Medium 1 · Low 2. Full detail: `INIT-001-security-review-findings.md`.
  - **Risk acceptance:** SEC-OPS-1 (ElevenLabs API key exposed in tool output). Approver: Brian Nelson (operator). Expiry: **2026-10-15**. The key must be rotated before then.
- **Assessment:** none (this initiative did not come from `assess`).

---

## Database changes

- [x] None

No schema, migration, or data migration. Learner progress is stored in browser `localStorage` (`hebrew-review:v1`), with a one-time migration from the per-lesson `wordMarks` key. Generated output: `worksheets/_review/vocab-index.js`.

## Deployment notes & rollback

- **Deploy:** docker-compose (local) or the k8s deployment. Lesson content is read from the lessons folder; rebuild the vocab index with `npm run vocab-index`. Set `ELEVENLABS_API_KEY` from `.env` locally or the vault Secret in k8s. Do not add it to compose.
- **Action required before merge:** rotate `ELEVENLABS_API_KEY` (SEC-OPS-1).
- **Recommended before merge:** set `MCP_AUTH_TOKEN` and remove `MCP_ALLOW_NO_AUTH` from compose (MED-1).
- **Rollback:** revert the files listed in the completion report's Files Changed section. The `hebrew-review:v1` localStorage key is ignored by older code, so no data cleanup is needed. Rebuild the vocab index after a revert, or delete `worksheets/_review/vocab-index.js` (the review page then shows "no review data yet").

## Out of scope in this working tree (split into separate PRs)

- ElevenLabs word-audio work: `shared/word-audio-catalog.js`, `worksheets/_audio/words/` (new mp3s and catalog), `worksheets/catalog.js`, `worksheets/_audio/words/manifest.json`
- Compose and vault changes from earlier work
- The operator's personal lesson `worksheets/anal-sex-vocab/` (untracked; do not commit without the operator's explicit request)

## Checklist

- [x] Quality gate `npm test` clean (114/114)
- [x] Acceptance criteria independently verified (9/9 PASS)
- [x] Security review: no unresolved Critical/High, except SEC-OPS-1 under risk acceptance to 2026-10-15
- [x] Architecture/Design diagrams produced (`design/INIT-001-evidence-based-practice-architecture.md`)
- [x] Completion report generated

## References

- Completion report: [`INIT-001-evidence-based-practice-completion-report.md`](INIT-001-evidence-based-practice-completion-report.md)
- Initiative: [`INIT-001-evidence-based-practice-initiative.md`](INIT-001-evidence-based-practice-initiative.md)
- Diagrams: [`design/INIT-001-evidence-based-practice-architecture.md`](design/INIT-001-evidence-based-practice-architecture.md)
- Security findings: [`INIT-001-security-review-findings.md`](INIT-001-security-review-findings.md)
- Research: [`sdd/research/RSCH-003-language-learning-science/`](../../research/RSCH-003-language-learning-science/)
