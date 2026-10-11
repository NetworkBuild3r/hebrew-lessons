# SPEC-003 · Frontend: Player shell, step transitions, and thumb-zone mobile ergonomics

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-007
title: "Player shell, step transitions, and thumb-zone mobile ergonomics"
domain: frontend
status: ready
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-001]
blocks: [SPEC-004, SPEC-005, SPEC-006]
```

---

## Description

**Context:** Over 70% of language practice happens on mobile phones. Currently, the player header contains small, cluttered text links, while the bottom action bar has varying heights that can cause layout shifts or overlap content. RSCH-006 Rule 17 and RSCH-007 emphasize thumb-zone ergonomics: the primary action must live in the lower reachable area, pointer targets must be ≥44px, and transitions between activity steps must feel smooth rather than jarring.

This spec overhauls the outer player shell (`shared/worksheet.js` / `shared/player/core.js` and player CSS):
1. **Thumb-Zone Bottom Dock:**
   - A sticky, glassmorphic bottom bar (`.action-bar`) with generous safe-area padding (`env(safe-area-inset-bottom)`).
   - Primary advance action ("המשך / Continue" or "בדוק / Check") centered with minimum 48px height, bold label, and icon.
   - Never overlaps focused inputs or active cards.
2. **Clean Minimalist Top Header:**
   - Left: Back / Exit button with intuitive icon and title ("← שיעורים / Lessons").
   - Center: Calm step indicator (e.g. "שלב 3 מתוך 7") with a smooth, amber progress bar that animates seamlessly between steps.
   - Right: Clean Nikud (vowels) toggle pill (`.btn-vowels`) that switches between pointed and unpointed Hebrew with clear state indication.
3. **Smooth Step Transitions:**
   - Forward step transition slides the current step out and new step in (120ms ease-out).
   - Instant transition under `prefers-reduced-motion: reduce`.
   - Automatic scroll-to-top on step advance so learners always start at the top of the new activity.

**Scope (in):**
- Redesigning `.player-top`, `.action-bar`, and `.progress-track` in `shared/worksheet.css`.
- Updating step transition hooks in `shared/player/core.js` and `shared/worksheet.js`.
- Responsive behavior across 360px mobile viewports through 1280px desktops.

**Scope (out):**
- Inner activity body formatting (handled in SPEC-004 and SPEC-005).

---

## Understood

- [ ] und-1: The bottom action bar must never obscure any input or focused control (WCAG 2.4.11).
- [ ] und-2: The vowel toggle button must remain accessible and functional on every screen.
- [ ] und-3: Primary touch targets must measure at least 44x44 CSS pixels.

---

## Acceptance Criteria

- [ ] ac-1: On mobile viewports (≤599px), the primary action button sits inside a fixed bottom dock with min-height 48px and safe-area inset support.
- [ ] ac-2: The progress track displays accurate progress across lesson activities without showing points, streaks, or gamified counters.
- [ ] ac-3: Toggling the Nikud button toggles vowel visibility across all Hebrew text on the screen while keeping Hebrew line heights intact.
- [ ] ac-4: Advancing between steps resets window scroll to the top of the main container and announces the new step to screen readers.
- [ ] ac-5: Step change animation finishes in ≤150ms and is completely disabled when `prefers-reduced-motion: reduce` matches.

---

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
|---|---|---|---|---|
| aud-1 | Bottom bar styling | White/translucent card with top border and subtle shadow for contrast against page ground | resolved | yes |
| aud-2 | Step numbering | "שלב X מתוך Y" (Step X of Y) format is informative and non-gamified | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- Updates to `shared/worksheet.css` (`.player-top`, `.action-bar`, `.progress-track`)
- Updates to `shared/player/core.js` (scroll reset, transition class management)
- Completion summary at `sdd/initiatives/INIT-007-core-learning-experience/specs/completion-summaries/SPEC-003-completion-summary.md`

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test` passes; `test/chrome-navigation-smoke.js` passes.
- **Rollback plan:** Revert player CSS and `core.js` changes.

---

## Verification Strategy

- **Claim:** Bottom dock provides 48px primary action in thumb zone; progress bar tracks step position; step advance scrolls to top; reduced motion suppresses transition.
- **Check + executor:** `mechanized` — run `npm test` and verify CSS styles and touch target dimensions.
- **Pass condition:** ac-1 through ac-5 verified.
- **Provenance tag:** `INIT-007/SPEC-003`
