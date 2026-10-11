# SPEC-009 Completion Summary — walking skeleton, one unit

**Traceability:** INIT-003/SPEC-009
**Status:** Complete (spike written; spec `status: complete`)
**Domain:** frontend
**Completed Date:** 2026-10-08

---

## Deliverables Checklist

- [x] `spikes/skeleton/` — source, README, tests, screenshots
- [x] `spikes/skeleton/validate.js` — schema check of the four samples and the broken plans
- [x] `spikes/skeleton/ops.js` — `apply(plan, op, expectedRev)` shared by the command and the form
- [x] `spikes/skeleton/RESULTS.md`
- [x] SPEC-009 yaml `status: complete`
- [x] No dependency added

## Acceptance Criteria

- [x] ac-1: `node spikes/skeleton/validate.js` exits 0. Four samples pass. Six broken plans are rejected for the named reason.
- [x] ac-2: Today, the overview, the introduction, recall, and the checkpoint render. The checkpoint uses the lesson `recycles` list.
- [x] ac-3: CLI and `POST /apply` both call `ops.applyBody`. The parity test compares the bytes.
- [x] ac-4: A stale revision returns `revision_conflict` and does not change the plan. The live server returned HTTP 409.
- [x] ac-5: Stylesheet colours are the ADR-0006 tokens. Text contrast is at least 4.5:1. Targets use 24px and 44px.
- [x] ac-6: Hostile text is escaped and truncated. Missing alt and a broken media id are refused.
- [x] ac-7: README commands only. No install. No outbound request.
- [x] ac-8: `node --test spikes/skeleton/test/` passed (21). `npm test` passed (294).

## AC Verification (independent)

Mechanized checks were run in this session. The operator has not yet looked at the screenshots and said whether they match the a-calm-type pick. `RESULTS.md` records that the shots follow the machine's dark scheme and that the radios can force either scheme.

Overall: **not independently verified** · remediation loops: 0.

**Verification tier:** standard (executable spike, security focus on encoding and supply chain) · **pre-flight:** `dependency_add` not triggered (no package added) · security_review=`standard` · domain=`frontend` · autonomy_level=`auto_tests`.

| AC   | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS | `node spikes/skeleton/validate.js` exit 0; `test/validate.test.js` |
| ac-2 | PASS | `test/a11y.test.js` screen and recycle assertions |
| ac-3 | PASS | `test/parity.test.js` byte compare |
| ac-4 | PASS | `test/ops.test.js`, `test/parity.test.js`, live HTTP 409 |
| ac-5 | PASS | `test/a11y.test.js` contrast and target checks |
| ac-6 | PASS | `test/hostile.test.js` |
| ac-7 | PASS | README; no new dependency; server bound to 127.0.0.1 |
| ac-8 | PASS | spike tests 21/21; `npm test` 294/294 |

---

## Change Produced

- **Authored by:** `INIT-003/SPEC-009`
- **Title:** Render one beginner unit locally on the chosen stack and look
- **Files changed:** spike under `spikes/skeleton/`, this summary, SPEC-009 status
- **Change status:** written on branch `lesson-system-foundations`
- **Reversibility:** revertible (delete `spikes/skeleton/` and this summary; set spec status back to `ready`)

---

## Tests

| Test file | New tests | Total |
| --------- | --------- | ----- |
| `spikes/skeleton/test/validate.test.js` | 2 | 2 |
| `spikes/skeleton/test/ops.test.js` | 8 | 8 |
| `spikes/skeleton/test/hostile.test.js` | 4 | 4 |
| `spikes/skeleton/test/a11y.test.js` | 5 | 5 |
| `spikes/skeleton/test/parity.test.js` | 2 | 2 |

**All tests passing:** yes · spike 21/21 · `npm test` 294/294 · **Coverage (new code):** the spike is outside `npm test` on purpose; its own runner is `node --test spikes/skeleton/test/`

---

## Gates & Controls

`gate_actions: [dependency_add]` did not fire. ADR-0005 Option A adds no library. No commit, push, or pull request.

---

## Security Review

- **Sensitivity / depth:** standard · encoding and supply chain · **method:** inline review plus `test/hostile.test.js` (`self_reviewed: true` — the author ran the checks; a second person has not)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| —       | —        | —             | —        | no findings |

Plan strings go through `escapeHtml` or `textContent`. Picture `src` is a catalog path, not a plan URL. The server listens on 127.0.0.1 only, refuses paths that leave `public/` or the font directory, and caps the POST body. No new dependency.

---

## Files Changed

Provenance is `INIT-003/SPEC-009` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `spikes/skeleton/` | Created — validator, ops, renderer, loopback server, tests, README, RESULTS | `INIT-003/SPEC-009` |
| `sdd/initiatives/INIT-003-lesson-system-foundations/specs/SPEC-009-frontend-walking-skeleton-one-unit.md` | Modified — status `complete`; checkboxes marked | `INIT-003/SPEC-009` |
| `sdd/initiatives/INIT-003-lesson-system-foundations/specs/completion-summaries/SPEC-009-completion-summary.md` | Created — this summary | `INIT-003/SPEC-009` |

`.dockerignore` already excludes `spikes/`. It was not changed. `npm test` includes the assertion that `/spikes/` is not served.

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | trace and timer scripts | This session was an orchestrator-dispatched worker; timer and `open_trace` stay with the orchestrator | Run-trace and timed effort | Effort is an estimate only |

---

## Harness Adaptation

- **Persona:** `.claude/agents/principal-frontend-developer/AGENT.md` is not in this repo. Disclosed in the spec. The spike follows ADR-0005 Option A (Node built-ins, escaped HTML, tokens as custom properties).
- **Harness:** `cursor`.
- **Worktree / branch:** shared checkout on `lesson-system-foundations`. No commit. Worktree engagement stays with the orchestrator.
- **AC verifier:** not a second person. Commands above were run in this session.

---

## Deviations from the Spec

- The validator is a focused draft-07 walker for the keywords in `lesson-plan.schema.json`, not Ajv. No package was added. An unknown schema keyword throws, so a later schema change fails closed.
- Recall is a screen state on one introduction word. The plan has no recall block. The checkpoint is the plan's own `spaced-review-checkpoint`, fed by that lesson's `recycles`.
- The audio control is a placeholder. The unit sample has no clip path, and none was invented.
- Chrome screenshots were taken. Clicks were not automated, because that needs a driver this spec is not allowed to add.

---

## ADR amendments

None. Option A was not falsified. Details are in `spikes/skeleton/RESULTS.md`.
