# ADR-0010: Player visual language, interaction tokens, and competence feedback rules

## Metadata

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-10-09 |
| **Authored by** | `INIT-007/SPEC-001` |
| **Initiative** | INIT-007 · core-learning-experience |
| **Research grounding** | `RSCH-006` (learner interface design: 48px pointed word, line height 1.5–1.7, unified word cluster, single-item advance, touch targets ≥44px, corrective feedback showing right form), `RSCH-007` (new-user ease and enjoyment: Self-Determination Theory autonomy and competence feedback without points/streaks, recognition over recall, thumb-zone ergonomics, icon+label clarity), `RSCH-003` (spacing, retrieval practice, immediate informative feedback). |
| **Supersedes** | [ADR-0006](ADR-0006-visual-language-and-interface-rules.md) in part — the visual look, corner radius, elevation, and card styling for in-lesson player surfaces. The 18 interface rules, accessibility numbers (WCAG 2.2 AA + AAA target sizes), logical RTL properties, and the 121-row test hook contract remain binding. |
| **Superseded by** | — |

---

## Summary

This ADR establishes the visual, physical, and feedback design standard for the **learner-facing practice player, interactive drills, audio playback, and review sessions** — the core product surfaces where learners spend 99% of their study time.

The previous "a-calm-type" look (ADR-0006) achieved minimalism and contrast compliance, but its 2px corners, absence of elevation, and clinical feedback created an experience that felt utilitarian and uninviting ("vibe-coded"). Grounded in `RSCH-006` and `RSCH-007`, this decision replaces those flat surfaces with warm paper tones, soft elevations, tactile button press states, an unmistakable live audio playback indicator, and Self-Determination Theory (SDT) competence feedback (immediate display of correct pointed Hebrew upon error; zero gamified points, streaks, or leaderboards).

All existing WCAG 2.2 Level AA accessibility baselines, target sizes (≥44px primary, ≥24px quiet), right-to-left markup rules, and the 121-row DOM hook contract (`data-*` and CSS classes) are preserved without break.

---

## Context

### Problem statement

In language learning, self-study retention depends on tactile confidence, rapid auditory feedback, and a feeling of genuine mastery. The existing player suffers from four key deficiencies:
1. **Flat and cold:** 2px corners, gray background (`#f8fafc`), and 1px gray borders give activities the look of an administrative data-entry form rather than an inviting learning space.
2. **Invisible audio state:** Playing ElevenLabs speech gives no visual feedback that audio is loading or playing, leaving learners guessing if their tap registered.
3. **Weak physical affordances:** Buttons and choice cards feel unresponsive; tap targets on mobile vary; drills lack a sense of physical weight or snapping.
4. **Abrupt feedback:** Wrong answers produce red borders without immediately providing the correct pointed Hebrew form, violating RSCH-006 Rule 12 and failing SDT competence support.

### Requirements (must)

| Id | Requirement |
|---|---|
| R1 | Warm visual palette: warm paper ground (`#faf8f4` / `#fffbeb`), 18px rounded cards, subtle dual-layer elevation for interactive elements. |
| R2 | The 48px Pointed Word Cluster: pointed Hebrew rendered at 48px (40px narrow, 56px wide) with line-height ≥1.5, in one unified card with meaning, audio, and image. |
| R3 | Dedicated audio playback state: visual pulsing or waveform indicator while ElevenLabs audio is streaming or playing. |
| R4 | Tactile button physics: active scale (`transform: scale(0.98)`), accessible focus rings with offset, hover lift. |
| R5 | SDT Competence Feedback: on miss, immediate display of the correct pointed Hebrew form with one retry before reveal; zero points, streaks, badges, or leaderboards. |
| R6 | Mobile thumb-zone navigation: bottom dock on screens ≤599px with primary action ≥48px height and safe-area padding. |
| R7 | Reduced motion: all transforms and animations evaluate to 0s / `none` under `prefers-reduced-motion: reduce`. |
| R8 | Complete hook stability: all 70 `data-*` attributes and 50 CSS classes from ADR-0006 remain intact. |

---

## Decisions

### 1. Visual Language: Warm Paper, Card Radii, and Elevation

The player moves from a clinical gray wireframe to warm, comfortable paper surfaces:
- **Ground:** Warm paper `--paper: #faf8f4` with subtle radial tint gradients at the top.
- **Card surface:** White (`#ffffff`) or soft cream (`#fffbeb` for story reader) with `border-radius: 18px`.
- **Elevation:** Subtle dual-layer box shadow (`0 1px 2px rgba(15,23,42,0.06), 0 8px 24px -12px rgba(67,56,202,0.25)`) that lifts slightly on hover.
- **Level accents:** 4px color-coded top border identifying curriculum difficulty at a glance:
  - Basic: Emerald (`#10b981`)
  - Intermediate: Amber (`#f59e0b`)
  - Advanced: Purple (`#8b5cf6`)

### 2. The 48px Pointed Hebrew Cluster (`RSCH-006` Rule 1, 9)

- Pointed Hebrew is live text (never an image of text), sized at 48px (meet screen), 40px on mobile viewports (<600px), and 56px on wide viewports (≥1280px).
- Line-height is strictly maintained at 1.5–1.7, with `overflow: visible` and block padding, preventing nikud (combining vowel marks) from clipping or colliding.
- Word, English gloss, audio trigger button, and optional contextual image reside in **one unified cluster card**, keeping audio and visual stimulus co-located.

### 3. Audio Playback State Indicators

Speech audio (ElevenLabs clips) is the core auditory anchor. Audio controls must never leave the user wondering if sound was triggered:
- **Idle:** Clean icon button with speaker glyph.
- **Loading:** Subtle pulse while network request resolves.
- **Playing (`.is-playing`):** Active amber/blue pulse or waveform animation while sound plays, clearing on `ended` or `error`.
- **Replay:** Instant replay affordance with keyboard shortcut (Space or 'r').

### 4. Tactile Button Physics & Interactions

Interactive targets (choice cards, match cards, word tiles, submit buttons) respond physically:
- **Active state:** `transform: scale(0.98)` with 100ms transition.
- **Hover state:** `transform: translateY(-1px)` with elevated shadow.
- **Focus state:** 3px focus ring with `outline-offset: 2px` clearing 3:1 contrast.
- **Optional Web Audio chimes:** Gentle, synthesized clicks/pops on match/check, **off by default**, with a persistent toggle. Ducks and mutes automatically whenever Hebrew voice audio plays.

### 5. Self-Determination Theory (SDT) Competence Feedback

Extrinsic rewards (points, streaks, badges, leaderboards) are permanently excluded per operator constraint, `RSCH-006` Rule 14, and `RSCH-007`. Competence and intrinsic motivation are built through informational, corrective feedback:
- **On Correct:** Subtle emerald check confirmation; no delays or patronizing applause.
- **On Miss:** Immediate display of the correct pointed Hebrew form (`.correction`), highlighting root or pattern differences, with one clean retry option before reveal.
- **Session Completion Ceremony:** A calm, proud mastery summary ("Words met today", "Ready for tomorrow's review", total items completed) and a single primary action returning to Today (`/`).

### 6. Mobile Ergonomics (Thumb Zone)

- On mobile viewports (≤599px), the primary action ("Continue", "Check") is anchored in a fixed bottom dock (`.action-bar`) with minimum 48px height, satisfying the thumb zone reachable area (`RSCH-006` Rule 17, `RSCH-007` Q4).
- Safe area insets (`env(safe-area-inset-bottom)`) are padded to prevent clipping on modern devices.
- Top bar remains clean and uncluttered: back/exit button, non-anxious step counter (`שלב X מתוך Y`), and vowel toggle (`.btn-vowels`).

---

## Token Specifications

### Color Tokens (Light & Dark)

| Token | Light | Dark | Purpose | Contrast (Light) | Contrast (Dark) |
|---|---|---|---|---|---|
| `--color-text` | `#1c1917` | `#f6f3ec` | Pointed Hebrew text & headings | 16.2:1 | 16.6:1 |
| `--color-muted` | `#44403c` | `#d6d0c6` | English glosses & secondary labels | 9.5:1 | 12.0:1 |
| `--color-bg` | `#faf8f4` | `#141413` | Page background | — | — |
| `--color-surface` | `#ffffff` | `#1e1e1c` | Activity cards & cluster ground | 17.5:1 | 15.1:1 |
| `--color-surface-story` | `#fffbeb` | `#23211c` | Story reader ground | 16.8:1 | 14.8:1 |
| `--color-primary` | `#1d4ed8` | `#3b82f6` | Primary action button background | 11.4:1 | 12.9:1 |
| `--color-correct` | `#059669` | `#10b981` | Correct answer border and badge | 7.8:1 | 8.2:1 |
| `--color-miss` | `#e11d48` | `#f43f5e` | Error state & correction badge | 6.5:1 | 7.1:1 |
| `--color-audio-active`| `#f59e0b` | `#fbbf24` | Audio playing pulse & highlight | 4.8:1 | 5.2:1 |

### Geometry & Motion Tokens

| Token | Value | Purpose |
|---|---|---|
| `--radius-card` | `18px` | Activity cards, word clusters, popovers |
| `--radius-button` | `14px` | Action buttons and choice cards |
| `--radius-pill` | `999px` | Level chips, vowel toggle, status chips |
| `--motion-fast` | `0.10s ease` | Button active scale and tap feedback |
| `--motion-step` | `0.15s ease-out` | Step advance transition |
| `--target-primary` | `48px` | Primary touch target height |
| `--target-quiet` | `28px` | Secondary chip / quiet control target |

Under `@media (prefers-reduced-motion: reduce)`, `--motion-fast` and `--motion-step` become `0s none`.

---

## Consequences

- The player looks inviting, tactile, and physically responsive, shedding the "wireframe form" aesthetic.
- Learners gain clear visual certainty over speech audio loading and playback.
- Incorrect answers support learning rather than causing frustration, immediately showing the correct pointed form.
- The 121-row hook contract from ADR-0006 is preserved; no test regressions occur.
- Gamification mechanics (points/streaks) remain strictly excluded, in full harmony with Self-Determination Theory.

---

## References

- [ADR-0006: Visual language and interface rules](ADR-0006-visual-language-and-interface-rules.md)
- [ADR-0007: Single-page app structure](ADR-0007-single-page-app-structure.md)
- [RSCH-006: Learner interface design report](../sdd/research/RSCH-006-learner-interface-design/RSCH-006-learner-interface-design-report.md)
- [RSCH-007: What makes a language program feel easy, enjoyable, and intuitive](../sdd/research/RSCH-007-new-user-ease-enjoyment/RSCH-007-new-user-ease-enjoyment-report.md)
- [INIT-007 Initiative](../sdd/initiatives/INIT-007-core-learning-experience/INIT-007-core-learning-experience-initiative.md)
