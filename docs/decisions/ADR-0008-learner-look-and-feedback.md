# ADR-0008: Learner look, identity and feedback rules (whole app)

## Metadata

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-10-12 |
| **Authored by** | `INIT-005/SPEC-008` |
| **Initiative** | INIT-005 · learner-experience-and-identity |
| **Research grounding** | `RSCH-006`, `RSCH-007` (new-user ease), `RSCH-008` (motion, sound, icons, picture style; overall confidence Low) |
| **Supersedes** | [ADR-0006](ADR-0006-visual-language-and-interface-rules.md) in part: the look only. The 18 interface rules, the accessibility numbers and the hook contract stay binding. |
| **Related** | [ADR-0007](ADR-0007-single-page-app-structure.md) (shell and router), [ADR-0010](ADR-0010-player-visual-language-and-competence-feedback.md) (player look) |

## Decision

**1. The pick.** INIT-005 planned a three-prototype contest (SPEC-007) to choose a look. It was not run. INIT-007 had already chosen, built and shipped a look in ADR-0010: warm paper ground, white 18px cards, soft two-layer elevation, tactile press, a visible audio-playing state and competence feedback. The operator then asked for INIT-005 to be closed out. This ADR adopts the ADR-0010 look for the **whole app** (shell, Today, Lessons, Guides, Words, Review, Progress, Admin, login), not only the player. No new colour direction was introduced. SPEC-007 is closed as superseded, not as complete.

**2. Identity.** The mark is option A, the "Ayin tile" (white ע on the blue-to-violet tile with an amber dot), in shared/brand/. This is a **default pick recorded for the operator to veto**; options B and C are kept in the same folder and scripts/brand/generate-brand-svgs.py rebuilds the favicon set from any of them. Icons come from shared/brand/sprite.svg (23 icons, 24px grid, 2px stroke). Usage rules are in docs/design/icon-usage.md; the picture style and the prompt sentence for image generation are in docs/design/illustration-style.md. Everything is self-hosted (REQ-018). /shared/brand/ is public so the favicon, manifest icons and the sign-in logo load without a session.

**3. Tokens.** The colour tokens in ADR-0006-assets/tokens.json are unchanged and are still the source of truth. Light and dark values and the measured contrast are below. This ADR changes **no token value**.

| Token | Light | Dark |
|---|---|---|
| `--color-text` | #1c1917 | #f6f3ec |
| `--color-muted` | #44403c | #d6d0c6 |
| `--color-bg` | #f7f6f3 | #141413 |
| `--color-surface` | #ffffff | #1e1e1c |
| `--color-line` | #8a847c | #6e6a64 |
| `--color-control` | #1e3a2f | #c9ddd0 |
| `--color-control-text` | #f7f6f3 | #132019 |
| `--color-focus` | #1e3a2f | #e4f2e9 |
| `--color-picture` | #1c1917 | #f6f3ec |
| `--color-picture-bg` | #f7f6f3 | #1e1e1c |
| `--color-on-picture` | #1c1917 | #f6f3ec |

| Pair | Kind | Minimum | Light | Dark |
|---|---|---|---|---|
| color.text on color.bg | text | 4.5 | 16.2 | 16.6 |
| color.text on color.surface | text | 4.5 | 17.5 | 15.1 |
| color.muted on color.bg | text | 4.5 | 9.5 | 12.0 |
| color.muted on color.surface | text | 4.5 | 10.3 | 10.9 |
| color.controlText on color.control | text | 4.5 | 11.4 | 11.8 |
| color.onPicture on color.pictureBg | text | 4.5 | 16.2 | 15.1 |
| color.control on color.bg | ui | 3 | 11.4 | 12.9 |
| color.control on color.surface | ui | 3 | 12.3 | 11.7 |
| color.picture on color.pictureBg | ui | 3 | 16.2 | 15.1 |
| color.focus on color.bg | ui | 3 | 11.4 | 16.0 |
| color.focus on color.surface | ui | 3 | 12.3 | 14.5 |
| color.line on color.bg | ui | 3 | 3.4 | 3.4 |
| color.line on color.surface | ui | 3 | 3.7 | 3.1 |

**4. Dark mode.** Both token sets ship, and data-theme="dark" on the document element selects the dark set. **The app is light by default**, even when the OS is in dark mode. A :root:not([data-theme="dark"]) block at the end of shared/worksheet.css pins the light tokens. The reason: much of the older chrome still uses fixed light colours, and following the OS flipped the tokens dark while the page stayed light, which gave black controls on a cream page (seen on the Words page, 2026-10-12). REQ-017 asked that dark mode ship. It ships as tokens and an explicit opt-in. **A fully dark restyle of every screen is not done and not verified.** It would be a separate piece of work.

**5. Feedback rules** (REQ-013 to REQ-015, filtered by RSCH-008).
- **Wrong answers show the correct form on the same turn** (`HebrewFeedback.showMiss` in `shared/player/feedback.js`). A flash or a sound never replaces it.
- **The audio-playing state** (`is-loading` / `is-playing`) marks which control is speaking. The loading pulse is capped at 5 seconds.
- **Press feedback** is a short scale and focus ring. The route change is a 160 ms fade and rise.
- **Reduced motion:** every transform, transition and animation goes to none under `prefers-reduced-motion: reduce`.
- **UI sounds are off by default**, have one visible control (`#btn-sfx`, `aria-pressed`), are skipped while a Hebrew word is playing, and use a neutral 440 Hz miss tone rather than a low one. They are a product choice: RSCH-008 found no evidence that they help learning.
- **Nothing runs by itself for more than 5 seconds** without a way to stop it.
- **Banned:** points, streaks, levels, badges, leaderboards, jokes, mascots and characters (operator answers, 2026-10-09). The "words you can now use" count is allowed (INIT-005 assumption A1).
- **Not built:** a separate lesson-complete animation. ADR-0010 rules out confetti, and the existing completion state already shows what the learner can now do.
- **Labels:** one language per label, with English only as a gloss (REQ-016). Primary nav keeps a visible Hebrew word with an English gloss next to its icon; icon-only controls carry a Hebrew accessible name.

## What does not change

Node, `node:sqlite`, no framework, no build step. The 18 interface rules. Contrast of 4.5:1 for text and 3:1 for interface parts. Targets of 24 px minimum and 44 px for primary actions. The hook contract (70 `data-*` attributes and 50 classes checked by `test/player-ui-a11y.test.js`), with one correction: `data-key`, `data-slug` and `lessons-grid` are listed as "kept" in ADR-0006 but production code no longer emits them (found by `test/restyle-gate.test.js`, 2026-10-12). No smoke needs them. They are pinned in an explicit allow-list in that test, and ADR-0006's table is left as written for history.

## Consequences

- One look across the app, at the cost of a stylesheet that is append-only and large (shared/worksheet.css). Consolidating it is future work.
- The logo is a default. Changing it means rerunning the brand scripts and nothing else.
- Dark mode is opt-in and partly untested. The Light by default block is the single switch to remove when a real dark restyle lands.
- RSCH-008's evidence is thin (Low). The effects here are product choices, and each can be turned off or removed without touching learning content.
