# SPEC-001 · Docs: ADR-0010 player visual language, interaction tokens, and competence feedback rules

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-007
title: "ADR-0010: Player visual language, interaction tokens, and competence feedback rules"
domain: docs
status: complete
primary_prompt: .claude/agents/principal-architect/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 3 hours
blocked_by: []
blocks: [SPEC-002, SPEC-003, SPEC-004]
```

---

## Description

**Context:** The live player interface was designed around `ADR-0006`'s "a-calm-type" look (2px radius, no shadow, flat borders), which was chosen for minimalism but left the interactive experience feeling clinical, flat, and unappealing ("vibe-coded"). Research in `RSCH-006` and `RSCH-007` demonstrates that learning self-study benefits from warm physical affordances, distinct audio playback feedback, tactile button response, and Self-Determination Theory (SDT) competence feedback (immediate corrective display of the right form on miss, crisp confirmation on hit, without points or streaks).

This spec authors **ADR-0010**, formally recording:
1. The new visual design tokens for the player (elevations, warm surfaces, level color codings, generous pointed Hebrew line-heights 1.5–1.7, and 18px rounded cards).
2. Interaction tokens (active press scaling `scale(0.98)`, focus rings, audio loading/playing animation states, and reduced-motion fallbacks).
3. Feedback rules based on SDT competence support (immediate display of correct pointed Hebrew upon miss, one retry before reveal, calm non-gamified completion summary).
4. Explicit scope boundaries: supersedes ADR-0006's visual look for player/practice surfaces while strictly preserving all WCAG 2.2 Level AA accessibility baselines, target sizes (≥44px primary, ≥24px secondary), logical RTL properties, and the 121-row test-hook contract.

**Scope (in):**
- Author `docs/decisions/ADR-0010-player-visual-language-and-competence-feedback.md`.
- Define the token extensions in CSS custom properties and document their light/dark contrast values.
- Add "Superseded in part" note to ADR-0006.

**Scope (out):**
- Modifying stylesheet files or player JavaScript (handled in SPEC-002 through SPEC-006).

---

## Understood

- [x] und-1: Points, streaks, badges, and leaderboards stay out permanently (operator constraint + RSCH-006 rule 14 + RSCH-007 SDT findings).
- [x] und-2: Dark mode support is maintained with WCAG 2.2 4.5:1 text and 3:1 UI contrast minimums.
- [x] und-3: All existing test hooks (`data-*` attributes and queried class names from ADR-0006) remain strictly stable.

---

## Acceptance Criteria

- [x] ac-1: ADR-0010 document is authored at `docs/decisions/ADR-0010-player-visual-language-and-competence-feedback.md` with full context, decision points, token tables, and consequences.
- [x] ac-2: Token table includes light/dark hex pairs, contrast ratios (≥4.5:1 text, ≥3:1 UI), line-height standards (1.5–1.7 for pointed Hebrew), and motion tokens (≤150ms duration, complete shutoff under `prefers-reduced-motion`).
- [x] ac-3: Competence feedback rules are explicitly defined: on correct, subtle confirmation; on miss, immediate display of pointed Hebrew correction + one retry; on session end, competence summary ("Words met", "Words ready for review") with zero gamification points or streaks.
- [x] ac-4: ADR-0006 is updated with an amendment note marking its visual styling section superseded in part by ADR-0010 while its interface rules and hook contract remain binding.

---

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
|---|---|---|---|---|
| aud-1 | ADR Number | Claimed next sequential number `ADR-0010` (ADR-0008 reserved by INIT-005, ADR-0009 authored by INIT-006) | resolved | yes |
| aud-2 | Design tokens scope | Extends `:root` tokens in `shared/worksheet.css` without requiring a build step or CSS preprocessor | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- `docs/decisions/ADR-0010-player-visual-language-and-competence-feedback.md`
- Amendment notice in `docs/decisions/ADR-0006-visual-language-and-interface-rules.md`
- Completion summary at `sdd/initiatives/INIT-007-core-learning-experience/specs/completion-summaries/SPEC-001-completion-summary.md`

---

## Gates & Controls

- **Autonomy level:** `plan_only` (requires human review of the architectural decisions).
- **Blocking checks:** ADR-0010 file exists; contrast ratios meet WCAG 2.2 AA; test-hook stability verified.
- **Rollback plan:** Delete ADR-0010 and revert ADR-0006 amendment.

---

## Verification Strategy

- **Claim:** ADR-0010 accurately codifies research findings into concrete design tokens, interaction rules, and competence feedback contracts without violating existing accessibility or hook baselines.
- **Check + executor:** `mechanized` — check ADR-0010 markdown existence and contrast ratio arithmetic; `human` — operator architectural review.
- **Pass condition:** ac-1 through ac-4 satisfied.
- **Provenance tag:** `INIT-007/SPEC-001`
