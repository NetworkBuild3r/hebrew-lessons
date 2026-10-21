# Initiative: Core learning experience — practice player visual polish, tactile feedback, and research-backed delight

**Initiative ID:** INIT-007
**Status:** complete — Mode E closed 2026-10-11. All seven specs complete (SPEC-007 last). Completion report: `INIT-007-core-learning-experience-completion-report.md`.
**Slug:** INIT-007-core-learning-experience
**Discipline Profile:** `software`
**Created:** 2026-10-09
**Owner:** Brian Nelson
**Ticket:** None (`branch_naming.ticket_system: none`; operator request 2026-10-09)
**Branch:** `core-learning-experience` (from `main` after INIT-004/006 merge; in-place, no worktree). No commit unless the operator asks.
**Target Completion:** 2026-10-20
**Source:** Operator request 2026-10-09: "screw the front door. I like the backdoor anyways. so let's make sure we can have something worth a front door. Backend and feel good where 99% of the users time will be spent. Build and initiative for making it prettier and matching our research."
**Research Grounding:**
- `RSCH-006`: Learner interface design for word, meaning, audio, and picture, including pointed Hebrew. Pointed Hebrew ≥24px/48px, line height 1.5–1.7, single-item advance, corrective feedback showing the right form, thumb-zone controls.
- `RSCH-007`: What makes a language program feel easy, enjoyable, and intuitive for new users. Self-Determination Theory (Ryan & Deci 2000, Jeno et al. 2018) autonomy and competence feedback without points/streaks/badges, recognition over recall, aesthetic minimalism, icon+visible-label navigation, mobile bottom thumb-zone ergonomics.
- `RSCH-003`: Second-language acquisition science (active retrieval, spaced recall, immediate informative feedback, first-language glosses).
- `RSCH-005`: Vocabulary presentation, typing, and spelling (picture only at introduction, active recall over passive copying).
**Decisions it builds on / amends:**
- Amends ADR-0006's visual look for in-lesson surfaces via ADR-0010 (keeping its accessibility floors, WCAG 2.2 rules, and test-hook contract).
- Builds on ADR-0007 (SPA shell and client router).
- Prepares UI player components for INIT-006's FSRS flashcard and lexicon model.

---

## 1. Executive Summary

This initiative completely overhauls the internal learning interface — the player, activities, audio playback, drills, review sessions, and round completion ceremony — where learners spend 99% of their study time. Instead of flat, utilitarian forms that feel "vibe-coded" and clinical, it delivers an interface that feels delightful, tactile, visually warm, and intuitive, while strictly honoring our research constraints (no juvenile points, streaks, badges, or leaderboards).

Anchored by `RSCH-006` and `RSCH-007`, the new experience introduces a unified 48px pointed word cluster with distinct audio-playing feedback, an immersive story reader with line-by-line follow-along highlighting and instant gloss popovers, tactile drill physics (match card snapping, magnetic sentence builder tiles, assisted Hebrew typing with error correction), mobile thumb-zone navigation, and a proud, calm round completion summary that builds genuine competence.

One new ADR, **ADR-0010**, records the player visual language, interaction tokens, and competence-based feedback rules, establishing the permanent design standard for all present and future Hebrew worksheet player activities.

---

## 2. Domain Analysis

| Domain | Affected | Key changes |
|---|---|---|
| Data | No | No database schema changes. Uses existing progress and review sync protocols. |
| Service | No | Backend endpoints remain stable. |
| API | No | Player speaks to existing `/api/lessons/<slug>/progress` and `/api/me/review`. |
| Event | No | No new background event systems. |
| Scheduled | No | No scheduled jobs. |
| Infrastructure | No | Self-hosted assets only; no CDN, no third-party scripts. |
| Frontend | Yes | Redesigned player chrome, activity templates (`shared/activities/*`), tactile feedback kit, audio playback state indicators, mobile bottom dock, and completion ceremony. |
| Docs | Yes | `ADR-0010`: Player visual language, interaction tokens, and competence feedback rules. |

---

## 3. Spec Breakdown

Each spec is single-domain, 200–800 LOC, 2–6 hours of effort. Detailed definitions live in `specs/`.

| Spec | Domain | Title | Prompt | Autonomy | Gates | Blocked by | Effort |
|---|---|---|---|---|---|---|---|
| SPEC-001 | docs | ADR-0010: Player visual language, interaction tokens, and competence feedback rules | principal-architect | plan_only | — | None | 3h |
| SPEC-002 | frontend | Feedback kit, tactile interactions, and audio playback states | principal-frontend-developer | auto_tests | — | SPEC-001 | 5h |
| SPEC-003 | frontend | Player shell, step transitions, and thumb-zone mobile ergonomics | principal-frontend-developer | auto_tests | — | SPEC-001 | 4h |
| SPEC-004 | frontend | Word cluster and story reading experience | principal-frontend-developer | auto_tests | — | SPEC-002, SPEC-003 | 5h |
| SPEC-005 | frontend | Interactive drills restyle (Match, Sentence Builder, Speed Quiz, Type-It) | principal-frontend-developer | auto_tests | — | SPEC-002, SPEC-003 | 6h |
| SPEC-006 | frontend | Review flashcards and session completion mastery moment | principal-frontend-developer | auto_tests | — | SPEC-002, SPEC-003 | 4h |
| SPEC-007 | testing | Player visual regression, a11y, touch-target, and Chrome smoke verification | test-automator | auto_tests | — | SPEC-004, SPEC-005, SPEC-006 | 4h |

---

## 4. Dependency Graph & Critical Path

```text
SPEC-001 (ADR-0010)
   ├── SPEC-002 (Feedback kit & audio states) ───┐
   └── SPEC-003 (Player shell & mobile dock) ────┤
                                                 ├─► SPEC-004 (Word cluster & Story) ────────┐
                                                 ├─► SPEC-005 (Interactive Drills) ─────────┼─► SPEC-007 (Verification & Smokes)
                                                 └─► SPEC-006 (Review & Mastery Ceremony) ───┘
```

- **Critical Path:** SPEC-001 → SPEC-002 → SPEC-005 → SPEC-007
- **Critical Path Duration:** 3h + 5h + 6h + 4h = 18h
- **Total Estimated Effort:** 31h

---

## 5. Timeline & Effort

- Total sequential chain: 18h
- Total engineering effort across all specs: 31h
- Realistic duration multiplier (1.3×): ~24 working hours (~4 calendar days at 6 productive hours/day).

---

## 6. Governance & Autonomy

- **Reversibility Class:** `branch` (pure front-end CSS, JS modules, and docs; no irreversible database migrations or destructive mutations).
- **Autonomy Levels:**
  - SPEC-001 is `plan_only` (requires human review of ADR-0010 architectural rules).
  - SPEC-002 through SPEC-007 are `auto_tests` (execute against existing test suite and new smoke suites).
- **Gates & Approvals:**
  - Operator approval on ADR-0010 before frontend implementation.
  - Zero test regressions on existing 318 test suite at every spec completion.
  - All touch targets strictly ≥44px for primary actions, ≥24px for secondary controls.

---

## 7. Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|---|---|---|---|
| CSS changes break existing Chrome smokes or regression tests | Medium | High | `test/chrome-*-smoke.js` and `test/*.test.js` run on every spec. Class and `data-*` attribute contracts from ADR-0006 are strictly preserved. |
| ElevenLabs audio latency makes audio indicator feel sluggish | Medium | Medium | SPEC-002 implements optimistic audio loading states (loading pulse → playing waveform → resting) with fallback to local browser speech. |
| Hebrew vowel marks (nikud) clip on smaller mobile viewports | Low | High | RSCH-006 Rule 3 enforced: generous 1.5–1.7 line height on all pointed Hebrew lines, `overflow: visible`, and block padding. Tested in 360px viewport smoke. |
| Tactile animations distract from learning | Low | Medium | Strict adherence to `RSCH-006` Rule 18 and `RSCH-007`: animations under 150ms, zero gratuitous celebration loops, instant toggle off when `prefers-reduced-motion` is active. |

---

## 8. Threat Model (Security)

- **Trust Boundaries:** Frontend execution inside learner browser session.
- **Attack Surface:** DOM injection in gloss popovers, custom lesson text, and audio URLs.
- **Data Classification:** No PII or credentials touched. Learner progress saved via authenticated session.
- **Mitigation:**
  - All dynamic HTML generation uses `HebrewUtils.escapeHtml` or `HebrewLayout.bi`.
  - SVG icons remain inline with no embedded `<script>` or external references.
  - Audio URLs strictly validated against local paths or authorized media origin.

---

## 9. Plan Reflection (Phase 5.5)

- Completeness: 9/10 (covers all learner-facing practice activities, feedback, audio, and player container)
- Accuracy: 9/10 (grounded in existing `shared/activities/*` and `shared/worksheet.css`)
- Clarity: 10/10 (clear acceptance criteria and component responsibilities)
- Technical Feasibility: 10/10 (pure CSS/JS within existing architecture, no new dependencies)
- Guardrails Compliance: 10/10 (preserves ADR-0006 test hooks, zero gamification points/streaks)
- **Overall Confidence:** 0.94 (47/50)
