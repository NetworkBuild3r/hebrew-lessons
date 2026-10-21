# Pull Request: INIT-015 — v3 Adaptive Learning: placement, learner model, activities, and AI-authored lessons

> **Artifact only.** No PR was opened. `v3` shares no history with `main`, so GitHub cannot open a PR between them. `v3` was pushed directly to `origin/v3` (`9e58365..844f16d`), and then merged into `main` as merge commit `1351938` (parents old `main` and `v3`, tree equal to `v3`'s, fast-forward push `c06f722..1351938`). That overrides ADR-0018 D6 (`v3` never merges into `main`) at the operator's request. Use this text as the body of any release note for those pushes.

## Summary

Turns the Hebrew app into an adaptive learner. A real placement test (200 verified words, audio, no feedback until the review) seeds a learner model. The model tracks each skill (listen, read, recall, spell) per word with states and diagnosis, and a daily job snapshots it. Activities and the review card grade by one rule and write one kind of evidence. An AI coach reads the model and a `LessonBrief` over MCP and writes lessons that the server validates and publishes with no human step (D5). A new design system (Lamplight tokens, motion, icons, Hebrew keyboard), a five-destination app shell and a Progress page ship with it, along with two-voice dialogue and a weekly report.

**Delta against the deployed `origin/v3` baseline:** 60 commits, 396 files, +82,251 and -7,973 lines. Migrations 006 and 008 to 011 (007 is the OAuth change that shipped first; see D9).

> **Merging this deploys it.** A push to `v3` builds `ghcr.io/networkbuild3r/hebrew:sha-<commit>`. Both Image Updaters (production `hebrew`, staging `hebrew-next`) deploy the newest sha tag, and production migrates its database on start. Back up first (`hebrew backup --to`); the old image cannot read a migrated database. See `docs/ops/v3-cutover.md`.

## Work Performed (by Spec)

- **SPEC-001, 002 (docs, infrastructure):** ADR-0018 and the north-star; `v3` cut and CI moved to it, publishing `ghcr.io/networkbuild3r/hebrew` only.
- **SPEC-003, 004 (data):** evidence tables (`attempts`, `exposures`, directional `cards`) and learner-model tables (snapshots, regressions, briefs, coach notes, publish audit, reports).
- **SPEC-005 (service):** one attempt recorder and one grading rule.
- **SPEC-006, 007 (data, service):** the placement deck (200 words, 206 clips as a manifest) and the placement engine.
- **SPEC-008 to 012 (frontend):** Lamplight tokens, motion system, icons and illustrations, Hebrew keyboard, placement screens.
- **SPEC-013, 014 (service, scheduled):** learner-model states and diagnosis; the daily snapshot and regression job.
- **SPEC-015, 016 (service, frontend):** the skill-aware activity engine; activity screens and the review card.
- **SPEC-017, 018 (frontend):** the app shell (Today, dock and rail, lesson frame, report path); Progress per skill.
- **SPEC-019, 020, 021, 022 (service, api, docs):** `LessonBrief`; brief checks and auto-publish safety; MCP learner and coach tools with roles and rate limits; one authoring skill.
- **SPEC-023 (scheduled):** the weekly report and the coach automation definition.
- **SPEC-024 (frontend):** the visual and usability regression matrix.
- **SPEC-025 (infrastructure, `plan_only`):** the cutover runbook, smoke script, `hebrew migrate --check` and `hebrew backup --to`.
- **SPEC-026 (service):** two-voice dialogue.
- **Review fixes (not specs):** the read-only MCP token can no longer unpublish; learner `validate_unit` and `lookup` are rate limited; placement `Replace` is capped on the server; migration renumber (D9).

## Tests & Smoke Verification

- `go test ./...` passes in every package; about 1,040 test functions. `gofmt`, `go vet ./...`, the tokens check and `templ generate` are clean.
- Browser suites (Shell, Placement, Dialogue, Motion, Activity, Progress) pass, and `TestShots` passes: 63 screens, light and dark, four widths, with axe, one-gold, feedback-colour, 44 px target, no-scroll and reduced-motion checks.
- Migration upgrade from the deployed schema is tested (`TestMigrateFromTheDeployedV3Database`).
- **Smoke:** `scripts/smoke-v3.sh` checks `/healthz`, `/version`, the sign-in redirect, an anonymous MCP 401, MCP initialize, `tools/list` and `get_learner_model`. It passed 7 of 7 against a local server. **It has not run against staging or production.**

## Eval & Security Reports

- **Completion summaries:** `specs/completion-summaries/SPEC-001` to `SPEC-026`. Acceptance criteria: 116 total, 109 met, 4 partial, 3 open (SPEC-010 ac-4 Figma frame; SPEC-025 ac-2 smoke against staging and ac-4 rehearsed rollback, both need the operator).
- **Security review:** `INIT-015-security-review.md`. No unresolved Critical or High. Four findings fixed with tests. **Partial coverage:** `internal/mcpstore`, `internal/units`, most web handlers and templates, and `internal/auth` cookie flags were not reviewed.
- **Architecture:** `design/INIT-015-v3-adaptive-learning-architecture.md`.
- **Operator steps, open decisions and risks:** the completion report, "What the operator must do".
