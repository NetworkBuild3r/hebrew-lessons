# Initiative Completion Report: Evidence-Based Practice (INIT-001)

**Initiative ID:** INIT-001
**Initiative:** INIT-001-evidence-based-practice
**Completion Date:** 2026-10-08
**Status:** COMPLETE (with one recorded risk acceptance, expiring 2026-10-15; see Security Review)
**Source research:** RSCH-003 (Moderate confidence)

---

## Executive Summary

INIT-001 turned the research in RSCH-003 into product changes for the Hebrew practice site. Nine specs shipped: a learner-review model (ADR-0001), a browser review store and spaced scheduler, a "Review today" page with a home card, production recall (English to Hebrew), retry-before-reveal feedback on three activities, tap-to-gloss and a coverage estimate in stories, validator nudges for authoring, and an authoring guide. The full suite passes at 114/114 (94 before the Phase 3.5 remediation, plus 20 new loader tests). Every spec's acceptance criteria were independently verified (9/9 PASS). The initiative-wide security review found a High-severity code-execution path, which was remediated in this pass. Agent session time was not recorded, because no timer data exists in this repo.

---

## Spec Summary

Code authored under this initiative is traced as `INIT-001/SPEC-NNN`. "Change" is the status each completion summary recorded: all are `proposed` and uncommitted in the working tree, per the standing rule that commits happen only on request.

| Spec | Domain | Title | Status | Change | Agent Time | Tests |
| ---- | ------ | ----- | ------ | ------ | ---------- | ----- |
| SPEC-001 | docs | Accept ADR-0001 learner review model | complete | proposed | not recorded | 0 |
| SPEC-002 | service | Vocab index generator, CLI, and guarded upsert rebuild | complete | proposed | not recorded | 6 |
| SPEC-003 | frontend | Browser review store and ADR-0001 scheduler; words self-marks and wordMarks migration | complete | proposed | not recorded | 19 |
| SPEC-004 | frontend | Review today page, home due card, review tests | complete | proposed | not recorded | 12 unit + 1 smoke |
| SPEC-005 | frontend | Production (English to Hebrew) recall; type-it at intermediate | complete | proposed | not recorded | not itemized in summary; +1 smoke |
| SPEC-006 | frontend | Retry-before-reveal on pattern, fill, listen-pick | complete | proposed | not recorded | 4 (smoke) |
| SPEC-007 | frontend | Story tap-to-gloss, not_yet signal, coverage badge, shared tokenizer | complete | proposed | not recorded | 8 + 7 unit + 1 smoke |
| SPEC-008 | service | Validator nudges (recycling, encounters, coverage, glosses) | complete | proposed | not recorded | 8 |
| SPEC-009 | docs | Authoring guide and review-page docs | complete | proposed | not recorded | 0 |
| Remediation (Phase 3.5) | service | Literal-only lesson-data loader (HIGH-1) | complete | proposed | not recorded | 20 |
| **Total** | | | **9/9 + remediation** | | **not recorded** | **114 suite pass** |

**Status note:** each completion summary was written with `status: in_review`, because the Tier-5 and model-dispatch validators could not run without traces. The spec files now show `status: complete` after the orchestrator's verification step. Those validators were never executed.

---

## Agent Session Analytics

**Not available.** `sdd/initiatives/INIT-001-evidence-based-practice/specs/.sdd-timer.json` does not exist. `scripts/dev/` (which holds the timer and trace tooling) is absent from this repo, so no session durations were recorded.

| Metric | Value |
| ------ | ----- |
| Total specs | 9 |
| Total sessions | not recorded |
| Total agent time | not recorded |
| Average / median session | not recorded |
| Wall-clock span | not recorded |

**Disclosure:** no durations are reported for any spec. Anything shown as a time would be invented. Timer and trace tooling should be restored before the next initiative so that efficiency can be measured.

---

## Gates & Controls Summary

| Gate action | Count | All approved? | Notes |
| ----------- | ----- | ------------- | ----- |
| db_migration | 0 | n/a | none |
| merge | 0 | n/a | none; nothing committed |
| deploy | 0 | n/a | none |
| risk acceptance | 1 | yes | SEC-OPS-1 (see below) |

No one-way action was taken. Every spec ran with `gate_actions: []`. The access gate (research) and the orchestration gates were recorded in the session.

---

## Security Review

Initiative-wide review of the changed code (`thorough`-equivalent, targeted). Full detail: `INIT-001-security-review-findings.md`.

- **Findings:** Critical: 0 · High: 2 (1 remediated, 1 operator action) · Medium: 1 · Low: 2 · **overall risk:** Medium after remediation
- **Unresolved Critical/High:** SEC-OPS-1 only, covered by the risk acceptance below

| Finding | Severity | OWASP/control | Spec | Location | Status |
| ------- | -------- | ------------- | ---- | -------- | ------ |
| HIGH-1: vm sandbox escape evaluating lesson data | High | API8 / CWE-94 | SPEC-002, SPEC-008 (pattern predates INIT-001) | `server/lesson-data.js` (new), `server/audio.js`, `server/vocab-index.js`, `mcp/store.js`, `scripts/rescue-lesson.js`, `validate.js` | fixed; independent re-review recommended |
| SEC-OPS-1: ElevenLabs API key exposed in tool output | High (operational) | secret handling | cross-cutting | `.env` | open, risk accepted until 2026-10-15 |
| MED-1: MCP write tool runs with no bearer token in compose | Medium | API2 | cross-cutting | `docker-compose.yml`, `mcp/http.js` | open, follow-up |
| LOW-1: container runs as root (pre-existing) | Low | API8 | none | `Dockerfile` | open, follow-up |
| LOW-2: static web server has no authentication (pre-existing) | Low | API9 | none | `server/web.js` | open, follow-up |

- **Risk acceptances:** SEC-OPS-1 · approver: Brian Nelson (operator), recorded by AskQuestion selection on 2026-10-08 · rationale: the key is git-ignored and was never written to a tracked file, image, or log; the exposure is limited to the agent session transcript and the model provider's input · expiry: **2026-10-15**. Rotation of `ELEVENLABS_API_KEY` is still required before the expiry, and the acceptance lapses at expiry.
- **Positive observations:** the `publicFile` allowlist, dotfile and traversal rejection; `esc()` and `escapeHtml` coverage in every DOM sink reviewed; the API key is sent only in a header and never logged; `.env` is excluded from git and from the Docker context; compose does not wipe the key with an empty override.

---

## Test Summary

| Spec | Test file(s) | New tests | Coverage |
| ---- | ------------ | --------- | -------- |
| SPEC-002 | `test/vocab-index.test.js` | 6 | no coverage tool wired |
| SPEC-003 | `test/review-scheduler.test.js`, `test/review-store.test.js` | 19 | exercised by the two suites |
| SPEC-004 | `test/review-page.test.js`, `test/chrome-review-smoke.js` | 12 + 1 smoke | exercised |
| SPEC-005 | `test/production-recall.test.js`, `test/chrome-production-recall-smoke.js`, `test/validate-activities.test.js` (updated) | not itemized + 1 smoke | exercised |
| SPEC-006 | `test/retry-before-reveal-smoke.test.js` | 4 (smoke) | exercised |
| SPEC-007 | `test/tokenize-hebrew.test.js`, `test/story-gloss.test.js`, `test/chrome-story-gloss-smoke.js` | 8 + 7 + 1 smoke | exercised |
| SPEC-008 | `test/validate-nudges.test.js` | 8 | no coverage tool wired |
| Remediation | `test/lesson-data.test.js` | 20 | exercised |
| **Total** | | **114 suite pass, 0 fail** (94 before remediation) | |

Suite run (final): `node --test test/*.test.js`, 114 tests, 27 suites, 0 failures, duration about 14 seconds.

---

## Files Changed

| Spec | Created | Modified | Key files |
| ---- | ------- | -------- | --------- |
| SPEC-001 | ADR file, eval report, completion summary | SPEC status | `docs/decisions/ADR-0001-learner-review-model.md` |
| SPEC-002 | `server/vocab-index.js`, `scripts/build-vocab-index.js`, `test/vocab-index.test.js` | `package.json`, `mcp/tools.js` | `worksheets/_review/` (generated) |
| SPEC-003 | `shared/player/review.js`, two test suites | `shared/worksheet.js`, `shared/player/core.js`, `shared/activities/words.js` | `hebrew-review:v1` store |
| SPEC-004 | `review/index.html`, `shared/review-page.js`, `test/review-page.test.js`, `test/chrome-review-smoke.js` | `index.html`, `shared/worksheet.css`, `server/web.js` | `/review/` page |
| SPEC-005 | `test/production-recall.test.js`, `test/chrome-production-recall-smoke.js` | `shared/activities/words.js`, `type-it.js`, `catalog.js` | en_he signals |
| SPEC-006 | `test/retry-before-reveal-smoke.test.js` | `shared/activities/pattern.js`, `fill.js`, `listen-pick.js` | one retry before reveal |
| SPEC-007 | `shared/activities/story.js` additions, three test files | `shared/player/utils.js`, `shared/worksheet.css` | tokenizer, gloss, coverage |
| SPEC-008 | `test/validate-nudges.test.js` | `.cursor/skills/hebrew-worksheet/scripts/validate.js`, `mcp/tools.js` | nudges in reply |
| SPEC-009 | none | `.cursor/skills/hebrew-worksheet/SKILL.md`, `schema.md`, `README.md` | authoring guide |
| Remediation | `server/lesson-data.js`, `test/lesson-data.test.js` | `server/audio.js`, `server/vocab-index.js`, `mcp/store.js`, `scripts/rescue-lesson.js`, `validate.js` | literal-only loader |

**Working-tree scope warning:** the working tree also holds uncommitted work that is outside INIT-001 and should be split into its own PRs: the ElevenLabs word-audio work (`shared/word-audio-catalog.js`, `worksheets/_audio/words/`, `worksheets/catalog.js`), the Compose and vault changes, and the operator's `worksheets/anal-sex-vocab/` lesson.

---

## Deviations from the Initiative Plan

- **Timer, trace, and dehydration tooling absent:** `scripts/dev/` is missing, so there is no timer data, no trace validation, and no spec dehydration. Spec and summary files remain on disk in full.
- **Completion summary filenames:** seven summaries were written as `SPEC-NNN-completion.md`. At Phase 1 the inventory check required `-completion-summary.md`, so the seven were renamed (content unchanged). Recorded as a naming deviation.
- **Success criterion on the Chrome smoke file:** the initiative named `test/chrome-audio-smoke.js` as the file to extend. It was not extended. Smoke coverage was added as separate files: `chrome-review-smoke.js`, `chrome-production-recall-smoke.js`, `chrome-story-gloss-smoke.js`, and `retry-before-reveal-smoke.test.js`. All pass as part of `npm test`.
- **Quality tooling:** the repo has no `pyproject.toml`, `noxfile.py`, `.pre-commit-config.yaml`, or `prek.toml`, so the `prek` and `nox` steps do not apply. `npm test` is the repo's quality gate.
- **Git & Issue-Tracker Operations Guard:** no commit, stage, branch, push, or PR was made. The PR artifact was produced only, as the guard requires.
- **Persona:** `.claude/agents/` is absent, so specs ran with the principal-frontend-developer role described in `prompt-selection.md` and did not invent an AGENT.md.
- **Security registry:** `sdd/security-reviews/` and `scripts/dev/security_scan_indexer.py` are absent, so no SECSCAN or SECFIND ids were minted.
- **Security review depth:** the review was targeted at the changed code paths (see the security findings Coverage section), not a full endpoint-by-endpoint OWASP sweep. No frontend or agentic focus area was loaded.

---

## Challenges & Resolutions

- **Research grounding:** RSCH-003 went through three remediation loops. Two final edits (citations [27] and [38]) were applied after the last verifier pass without a re-check, and the research report discloses this.
- **Shared word audio:** the same Hebrew word (קָם) had two different audio clips. Resolved with a permanent shared word-audio cache and a catalog, so a word gets one clip across lessons.
- **Escape path found in security review:** the vm sandbox used by lesson loaders allowed code execution. Found by Phase 3.5 and remediated with a literal-only loader and regression tests. The earlier `hostile` fixture had only tested `require`, which is why the escape passed it.
- **Test mismatch on generated catalogs:** the generated word-audio catalogs use an `Object.assign` merge so several files can add to one global. The loader supports that exact shape only, and rejects other `Object.assign` calls.

---

## Quality Sweep Results

| Check | Result |
| ----- | ------ |
| `uv run prek run` | N/A (no prek configuration in this repo) |
| `npm test` (repo gate, derived scope = full suite) | PASS: 114 tests, 0 failures |
| `# type: ignore` scan | N/A (JavaScript repo) |
| Stale-reference scan | `loadWindow` is still used by name in the four loaders, all of which now delegate to `loadDataWindow`. No renamed symbols are unresolved. |
| Terraform fmt/validate/plan | N/A (no Terraform in the diff) |
| Independent AC verification | 9/9 specs PASS (SPEC-001..009 eval reports); remediation loops: SPEC-004 1, SPEC-005 1, SPEC-007 1 |

---

## Artifacts

- [Git PR artifact](INIT-001-evidence-based-practice-pull-request.md): PR title, body, tests, eval reports, and risk acceptance
- [Architecture/Design diagrams](design/INIT-001-evidence-based-practice-architecture.md)
- [Security review findings](INIT-001-security-review-findings.md)
- [Results report](INIT-001-evidence-based-practice-results-report.md) (PDF skipped: pandoc not on PATH)

## Completion Summary References

- [SPEC-001 Completion Summary](specs/completion-summaries/SPEC-001-completion-summary.md)
- [SPEC-002 Completion Summary](specs/completion-summaries/SPEC-002-completion-summary.md)
- [SPEC-003 Completion Summary](specs/completion-summaries/SPEC-003-completion-summary.md)
- [SPEC-004 Completion Summary](specs/completion-summaries/SPEC-004-completion-summary.md)
- [SPEC-005 Completion Summary](specs/completion-summaries/SPEC-005-completion-summary.md)
- [SPEC-006 Completion Summary](specs/completion-summaries/SPEC-006-completion-summary.md)
- [SPEC-007 Completion Summary](specs/completion-summaries/SPEC-007-completion-summary.md)
- [SPEC-008 Completion Summary](specs/completion-summaries/SPEC-008-completion-summary.md)
- [SPEC-009 Completion Summary](specs/completion-summaries/SPEC-009-completion-summary.md)

**Dehydration:** not performed. `scripts/dev/rehydrate_specs.py` is absent, and dehydration deletes files, so the specs and summaries remain on disk in full.
