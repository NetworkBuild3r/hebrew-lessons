# Architecture & Design — Responsive Desktop & Mobile Redesign (`INIT-013`)

**Initiative:** `INIT-013-responsive-desktop-mobile-redesign` · **Reflects specs:** `SPEC-001`…`SPEC-007` · **Date:** `2026-10-17`  
**Provenance tag:** `INIT-013/DESIGN-DIAGRAMS`  
**Canvas:** in-doc Mermaid only  
**Grounding:** [ADR-0015](../../../docs/decisions/ADR-0015-responsive-desktop-mobile-architecture.md), Figma file `NJ8HfTPCQgrFty2VVQBMhY` (`30:20`, `31:2`, `32:2`, `33:2`, `33:74`, `34:2`), `shared/worksheet.css`, `shared/app-shell.js`, `shared/ui/blocks/conjugation-table.js`, and `scripts/seed-verb-tense-workshop.js`.

This document visualizes the 3-tier responsive layout architecture (`<768px`, `768px–1023px`, `≥1024px`), the Light (`Jerusalem Stone`) / Dark (`Judean Night`) token & theme engine, the Bento workspace and study stage, and the research-backed (`R1`–`R6`) multi-tense verb conjugation workshop & interleaved drill pipeline after `INIT-013` shipped `SPEC-001` through `SPEC-007`.

---

## 1. Component & Responsive Shell Architecture  <!-- diagrams: component -->

How `ADR-0015` design tokens, the theme/direction engine in `shared/app-shell.js`, the adaptive shell/Bento rules in `shared/worksheet.css`, the study stage & interleaved verb drill in `shared/ui/blocks/`, and the `schemaVersion: "1.2.0"` verb workshop seed (`scripts/seed-verb-tense-workshop.js`) connect across Desktop and Mobile viewports.

```mermaid
graph TD
  subgraph tokens["Design Tokens & Theme Engine (INIT-013/SPEC-001, SPEC-002)"]
    ADR["ADR-0015<br/>3-tier breakpoints · Bento rules<br/>19 Light + 19 Dark contrast pairs<br/>R1–R6 verb pedagogy<br/><i>INIT-013/SPEC-001</i>"]
    CSS_TOK["shared/worksheet.css (:root)<br/>Jerusalem Stone (Light) #F7F5F0<br/>Judean Night (Dark) #0E121B<br/>Tekhelet Indigo #254EDB / #6085FF<br/><i>INIT-013/SPEC-002</i>"]
    THEME["shared/app-shell.js<br/>normalizeTheme · getStoredTheme<br/>applyTheme · toggleTheme<br/>[data-theme-toggle]<br/><i>INIT-013/SPEC-002</i>"]
  end

  subgraph shell["Adaptive Application Shell (INIT-013/SPEC-003)"]
    DESK_SHELL["Desktop ≥1024px<br/>body.has-hebrew-shell CSS Grid<br/>260px sticky sidebar rail (.hebrew-shell)<br/>RTL/LTR logical mirroring<br/><i>INIT-013/SPEC-003</i>"]
    MOB_SHELL["Mobile <768px<br/>Compact top header<br/>Fixed bottom tab dock (.hebrew-shell-nav)<br/>44px targets · scroll-padding-bottom 6.5rem<br/><i>INIT-013/SPEC-003</i>"]
  end

  subgraph bento["Bento Workspace & Catalogs (INIT-013/SPEC-004)"]
    TODAY["shared/today-page.js + worksheet.css<br/>Desktop 7fr / 5fr Bento grid<br/>Judean Midnight Hero (#today-continue-card)<br/>Mobile compact 1-col feed<br/><i>INIT-013/SPEC-004</i>"]
    CATS["Lessons · Words · Guides · Progress<br/>Desktop auto-fill multi-col grids<br/>Mobile full-width CTAs (≥46px)<br/><i>INIT-013/SPEC-004</i>"]
  end

  subgraph stage["Ergonomic Study Stage & Multi-Tense Verb Workshop (INIT-013/SPEC-005, SPEC-007)"]
    PLAYER["Lesson Player & Exercise Stage<br/>Desktop 68rem + 2×2 choice grid<br/>Mobile thumb tray + 22-letter pad<br/>SRS Review (≥36px lemma, ≥48px grades)<br/><i>INIT-013/SPEC-005</i>"]
    DRILL["shared/ui/blocks/conjugation-table.js<br/>Interleaved drillQueue + showItem()<br/>data-drill-lemma · data-drill-tense · data-drill-cell<br/>Non-owner [data-drill-owner='0'] collapse<br/><i>INIT-013/SPEC-005</i>"]
    SEED["scripts/seed-verb-tense-workshop.js<br/>4 verbs (hifil · piel · paal) × 22 cells = 88 forms<br/>verified=0 + SPEC-007-forms-checklist.md<br/>1.2.0 lesson decide-choose-believe-win<br/><i>INIT-013/SPEC-007</i>"]
  end

  subgraph verify["Automated Verification Matrix (INIT-013/SPEC-006)"]
    TESTS["test/responsive-layout.test.js<br/>test/restyle-gate.test.js<br/>test/verb-tense-workshop.test.js<br/>test/chrome-restyle-a11y-smoke.js<br/><i>INIT-013/SPEC-006</i>"]
  end

  ADR --> CSS_TOK
  CSS_TOK --> THEME
  CSS_TOK --> DESK_SHELL
  CSS_TOK --> MOB_SHELL
  THEME --> DESK_SHELL
  THEME --> MOB_SHELL
  DESK_SHELL --> TODAY
  DESK_SHELL --> CATS
  MOB_SHELL --> TODAY
  MOB_SHELL --> CATS
  CSS_TOK --> PLAYER
  SEED --> DRILL
  DRILL --> PLAYER
  TODAY --> TESTS
  PLAYER --> TESTS
  SEED --> TESTS
```

---

## 2. Data & Interaction Flow — Multi-Tense Verb Seed, Human Verification, and Interleaved Drill  <!-- diagrams: sequence -->

How `scripts/seed-verb-tense-workshop.js` (`SPEC-007`) seeds the 88 pointed verb forms (`present`, `past`, `future`) behind the human verification boundary (`ADR-0014` D13, `RSCH-009`), and how `shared/ui/blocks/conjugation-table.js` (`SPEC-005`) interleaves verified forms into a single active-recall station (`R1`–`R3`) that feeds `formSignal` mastery into the `Today` Bento workspace (`SPEC-004`).

```mermaid
sequenceDiagram
  autonumber
  participant Op as Operator / Teacher
  participant Seed as scripts/seed-verb-tense-workshop.js<br/>(INIT-013/SPEC-007)
  participant Check as SPEC-007-forms-checklist.md<br/>(88 forms, default [ ])
  participant DB as SQLite Lexicon & Plans<br/>(server/lexicon · server/plans)
  participant Player as Learner Browser<br/>conjugation-table.js (INIT-013/SPEC-005)
  participant Mastery as /api/learner/attempt<br/>#today-mix Bento (INIT-013/SPEC-004)

  Op->>Seed: Run seed({ checklistPath })
  Seed->>Check: Read ticked [x] vs unticked [ ] boxes
  Seed->>DB: checkForeignSources() (abort if source != 'admin')
  Seed->>DB: lexicon.upsert(4 lemmas + 88 present/past/future forms, verified=0)
  alt Form ticked [x] by human in checklist
    Seed->>DB: lexicon.setVerified(lemmaKey, formKey, true)
  else Unticked [ ] (default)
    Note over Seed,DB: Remains verified=0 (N14 warning; skipped in drillQueue)
  end
  Seed->>DB: Create/replace 1.2.0 plan verb-tense-workshop (decide-choose-believe-win)

  Op->>Player: Open /shared/plan/?id=verb-tense-workshop (390×844 or 1440×960)
  Player->>Player: Render study tables + collapse [data-drill-owner="0"] sections
  Player->>Player: drillQueue() shuffles verified cells across verbs & tenses
  loop Each interleaved prompt (R1, R2)
    Player->>Player: showItem() sets data-drill-lemma, data-drill-tense, data-drill-cell<br/>Displays "lemma · tense · person/gender"
    Player->>Player: Learner types answer (unpointed/plene accepted; 1 retry before reveal)
    Player->>Mastery: formSignal({ lemmaKey, cell, correct }) (boolean only, no typed text)
  end
  Player->>Player: Queue empty → "סיימתם. / Done." (clears data-drill-* attrs)
```
