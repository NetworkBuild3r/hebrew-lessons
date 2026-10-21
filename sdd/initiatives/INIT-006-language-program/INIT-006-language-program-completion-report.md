# INIT-006 Completion Report — Language program

**Initiative:** INIT-006 · `INIT-006-language-program`  
**Specs closed:** 2026-10-10  
**Mode E closed:** 2026-10-10  
**Branch:** `lesson-system-build`  
**Operator approval to start:** 2026-10-10 ("start init-006")

## Outcome

All **20** specs are `complete` and each has a completion summary under `specs/completion-summaries/`. Mode E inventory: 20/20 summaries present. Quality sweep on 2026-10-10: `SRS_SCHEDULER=ladder npm test` **846 pass / 0 fail**, `npm run audio:verify` against `data/audio-inventory/20261010T225230.json` **65 recordings OK**.

Shipped: ADR-0009, FSRS columns + lexicon/library/mastery migrations, FSRS-6 scheduler (default still `ladder`), lexicon service, learner model/profile/mix/coverage/library/cards/gate, plan schema 1.1.0 blocks and recipes, flashcards / Words / Guides / Today mix UI, MCP+HTTP learner tools, Progress AI-profile panel and admin AI-planning control, starter verb drafts + forms checklist, skill/operator docs, and rollout backup/stage-count scripts plus runbook.

No production `SRS_SCHEDULER=fsrs` flip. No learner worksheet `index.html` edited. AI-planning remains off until an admin confirms per learner.

## Specs

| Spec | Status | Summary |
|------|--------|---------|
| SPEC-001 … SPEC-020 | complete | `specs/completion-summaries/SPEC-NNN-completion-summary.md` |

## Success criteria (§10)

| Criterion | Status |
|-----------|--------|
| All twenty specs complete | Met (deploy gate for scheduler flip still open — see below) |
| `npm test` + `audio:verify` 65 | Met (846 / 65) |
| Stage counts equal across FSRS migration | Met in fixture tests (`test/rollout-scripts.test.js`); live DB compare is an operator step in the runbook |
| Words + flashcards (four grades, both directions) | Met (SPEC-012 / SPEC-013) |
| Profile off → `planning_off`; on → same object as HTTP | Met (SPEC-007 / SPEC-010 / SPEC-017) |
| Verb workshop draft: verified forms only; concept attribution | Met (SPEC-014 / SPEC-018) |
| Daily mix respects new-card / unseen spine cap | Met (SPEC-008 / SPEC-015) |
| No unresolved Critical/High security finding | Met per per-spec reviews; initiative-wide Phase 3.5 not re-dispatched (disclosed) |
| Completion report + status `complete` | This document |

## Local database (2026-10-10)

`data/hebrew.sqlite` has migrations **001–003** applied. **004** and **005** are in the tree but not yet applied to this DATA_DIR.

## Operator gates still open

| Gate | Spec | What remains |
|------|------|----------------|
| `db_migration` (apply locally/cluster) | SPEC-002 / SPEC-003 | Backup, then `npm run db:migrate` so `004-fsrs-review-state` and `005-lexicon-library-mastery` land on the real `DATA_DIR`. |
| `deploy` (scheduler flip) | SPEC-020 | Written approval to set `SRS_SCHEDULER=fsrs` on a named environment after backup, inventory, and matching stage counts. Rollback: `ladder` + restart. |
| Content | SPEC-018 | Tick verified forms on the starter verb checklist before publishing drafts. |

Runbook: `docs/operations/language-program-rollout.md`. GATE REQUEST text is in `specs/completion-summaries/SPEC-020-completion-summary.md`.

## Deviations / Mode E adaptations

- Timer/trace scripts and `initiative_inventory_check.py` are absent in this repo — inventory done by Glob (20 summaries) + status grep.
- Security completion-phase (thorough initiative-wide review) was not re-dispatched as a separate Mode E subagent; each code spec already ran security review with no unresolved Critical/High. Disclosed.
- Git PR artifact skipped (git guard; operator did not ask to commit/push/PR).
- Persona files under `.claude/agents/` absent throughout — workers followed specs + repo conventions.
- Two backup layouts coexist (`scripts/backup-db.js` flat file vs `npm run db:backup` media bundle); both documented in the runbook.

## Known follow-ups (not INIT-006 blockers)

- Apply migrations 004–005 and record live before/after stage counts under `ladder` before any `fsrs` flip.
- Hebrew disclosure line on Progress / confirm / sign-in is still a draft for operator wording approval (SPEC-017).
- INIT-004 cluster gates (`OPENAI_API_KEY`, learner-link cutover) remain separate and open.

## Next

Operator runs the language-program rollout checklist when ready to migrate and (optionally) switch the scheduler. This report closes INIT-006 Mode E for the repo Change.
