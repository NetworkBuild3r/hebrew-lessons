# SPEC-001 Completion Summary — ADR-0002

**Initiative:** INIT-002-multi-user-progress
**Spec:** SPEC-001 · ADR-0002: accounts, server-side progress, site map
**Status:** complete
**Provenance:** INIT-002/SPEC-001

## Change

| File | Change |
|------|--------|
| `docs/decisions/ADR-0002-accounts-server-progress-site-map.md` | Created. Fourteen decision sections, role × route matrix, public-path table. |
| `docs/decisions/ADR-0001-learner-review-model.md` | Superseded-by line and one changelog row only. |

## Gates

- Autonomy `plan_only`. `gate_actions: []`. No one-way action.
- Pre-flight: gate_actions PASS · security_review=required FAIL · domain=docs PASS · autonomy_level=plan_only PASS ⇒ light_tier_eligible=false. Tier: full.

## AC verification

Independent of the author: the orchestrator read ADR-0002 §§1–14, §6, §7, §12, and the carried-constraints table, and the ADR-0001 metadata diff. Verdict: all PASS. See `SPEC-001-eval-report.md`.

## Security

Decision record only. Residual risks written into the ADR: hash-named word mp3s are not filtered per lesson; admins can read full answer history (learners are told; reads are audited); no CSP yet; `node:sqlite` is experimental; sessions last 30 days with no idle timeout. No Critical or High code findings.

## Degradation

- Timer and trace skipped: `scripts/dev` is absent.
- No implementation-planner persona in this repo. Executed as a general-purpose agent.
- No commit.

## Harness

Cursor. Model passed: `cursor-grok-4.5-high-fast` (`cursor-grok-4.6-xhigh-fast` is not in this session's allowlist).
