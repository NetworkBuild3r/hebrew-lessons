# ADR-0006: Visual language and interface rules

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-10-08 |
| **Authored by** | `INIT-003/SPEC-008` |
| **Initiative** | INIT-003 · lesson-system-foundations |
| **Research grounding** | `RSCH-006` rule table, grades, and labels. The pick is the operator's, quoted below. |
| **Supersedes** | — |
| **Superseded by** | [ADR-0008](ADR-0008-learner-look-and-feedback.md) and [ADR-0010](ADR-0010-player-visual-language-and-competence-feedback.md) in part (visual look, 18px radius, elevation, audio states, and competence feedback for in-lesson surfaces; all 18 interface rules and the 121-row hook contract remain active) |

## Summary

The learner interface uses the **a-calm-type** mockup: one column, one primary action, the pointed word larger than the English, Rubik, a 2px corner, and no shadow. Dark mode ships. Both schemes are the hex values in [tokens.json](ADR-0006-assets/tokens.json). The stack that carries this is ADR-0005 Option A: custom properties and hand-built functions, no front-end framework.

Every `RSCH-006` rule is adopted with the same grade and label. The one scoped exception is the introduction's example sentence, which the pick keeps because the lesson-plan language asks for it. Points, streaks, badges, and leaderboards stay out. Existing test hooks stay, except `data-action="set-dir"`, which a test already forbids.

---

## Context

### Problem statement

SPEC-006 put three looks on disk. Without a record of the pick, the tokens, and the rules, every later spec would choose a color and a layout again. This record is what a builder implements when no designer is in the room.

### Current state

The live stylesheet is still `shared/worksheet.css`. ADR-0005 Decision 6 says a token block is added there when a screen is touched, and that a class or `data-*` a test queries changes only in the same change as that test. This record names the values and the hooks. It does not edit the stylesheet.

The mockup that was measured is `spikes/look/a-calm-type/look.css`. Contrast uses the function in `spikes/look/check.js`. The command is `node spikes/look/check.js`.

### Requirements (must)

| Id | Requirement |
|----|-------------|
| R1 | The operator's pick, in the operator's words, and what was taken from the other looks. |
| R2 | Named tokens for colour (light and dark), type, spacing, radius, elevation, and motion, each with a purpose, and a measured contrast for each foreground and background pair. |
| R3 | A component inventory with states. |
| R4 | The `RSCH-006` rule table, row for row, each with its grade and label, and the surfaces it governs. |
| R5 | An accessibility baseline of numbers, and the command that checks them. |
| R6 | Right-to-left and bidirectional rules. |
| R7 | What is excluded: no points, streaks, or badges, and no decorative motion that cannot be turned off. |
| R8 | A class and `data-*` contract for the hooks the named tests and the Chrome smokes depend on, each with a fate. |

### Constraints

- Plan only. No application code.
- ADR-0005 Option A is the stack: Node stays, the design system is hand-built, tokens are custom properties. This record does not reopen that choice.
- No secret values, no learner names, and no transcript text.
- A number that is a token lives in `tokens.json` and is repeated in Decision 2 so a reader and a contrast check see the same figure. Prose outside those two places cites the token id.

---

## Options considered

| Option | What it is | Why it is here |
|--------|------------|----------------|
| a-calm-type | Narrow column, empty paper, one filled action, pointed word much larger than the English, 2px corners, no illustration. | SPEC-006's closest match to `RSCH-006`. The operator's pick. |
| b-warm-illustrated | Warmer palette, 28px corners, drawings on the picture and a banner. | The illustrated alternative. The phone action sits in the card. |
| c-dense-tool | Tight type, an index of three words, a bar of three destinations. | The dense alternative. It lists more than one item and puts the main action in the top bar. |
| Light only | Ship the light hex values and do not ship a dark scheme. | `RSCH-006` does not treat dark mode as a reading aid. The operator was asked separately. |

Detail is Decision 1. Tokens are Decision 2.

---

## Decision

### 1. The pick

The operator's words, 2026-10-08: **a-calm-type**. **Ship dark mode: yes.**

a-calm-type is the whole look. Nothing is blended in from b or c.

| From | Taken | Left |
|------|-------|------|
| a-calm-type | The column, the type scale, the space scale, the 2px radius, the one primary action, the phone dock, the cluster, the miss panel, both palettes, Rubik. | — |
| b-warm-illustrated | Nothing. | The warm hex values, the 28px and 14px radii, the decorative colour, the drawings, the banner. Those trade off `RSCH-006` rule 11, and the start action is not in the thumb zone (rule 17). |
| c-dense-tool | Nothing. | The dense space scale, the index column, the three-link bar, the 32px word. Those trade off rules 10, 11, 16, and 17. |

Rubik, the 24px target, the 44px primary target, the contrast floors, and reduced motion are in all three mockups because `RSCH-006` requires them. They are not borrowings from b or c.

**Dark mode ships.** It is a product choice. `RSCH-006` says not to require dark mode in order to help reading: one large study found light text on a dark ground slower, in a Latin script, and pointed Hebrew was not tested (Low). Shipping the dark column does not change that finding and does not rewrite any rule-table row. Light stays available. The scheme follows the system. An explicit choice wins, by `data-theme="light"` or `data-theme="dark"` on the document element, which sets `color-scheme`. That is the product form of the spike's radios, and it is the override ADR-0005 Decision 6 already allows. `light-dark()` selects the hex pair. Both columns meet the contrast floors in Decision 2.

**Where the pick and a rule meet.** Rule 11 says the screen does not add decoration beyond the word, the meaning, the audio, and the picture. The pick keeps one example sentence under an introduction cluster. ADR-0003's introduction asks the learner to read an example. The operator's pick wins. The exception is a product choice, limited to that sentence on that block. No banner, no extra drawing, no second destination. The rule text in Decision 4 is unchanged.

### 2. Tokens

The machine-readable copy is [tokens.json](ADR-0006-assets/tokens.json). It has 48 tokens. SPEC-009 reads that file. The tables here are the same values. A contrast ratio is `toFixed(1)` of the WCAG ratio, using the function in `spikes/look/check.js`.

#### Colour

| Token | Light | Dark | Purpose |
|-------|-------|------|---------|
| `color.text` | `#1c1917` | `#f6f3ec` | Default text, including a pointed word and its vowel marks. |
| `color.muted` | `#44403c` | `#d6d0c6` | Secondary text: the English gloss, kickers, and legends. |
| `color.bg` | `#f7f6f3` | `#141413` | Page background. |
| `color.surface` | `#ffffff` | `#1e1e1c` | Card and correction-panel background. |
| `color.line` | `#8a847c` | `#6e6a64` | The 1px edge of a card, a divider, and the phone dock. |
| `color.control` | `#1e3a2f` | `#c9ddd0` | Fill and border of the one primary action. |
| `color.controlText` | `#f7f6f3` | `#132019` | Label on the primary action. |
| `color.focus` | `#1e3a2f` | `#e4f2e9` | Focus ring. |
| `color.picture` | `#1c1917` | `#f6f3ec` | Stroke of a picture that carries meaning. |
| `color.pictureBg` | `#f7f6f3` | `#1e1e1c` | Ground behind the picture stroke. |
| `color.onPicture` | `#1c1917` | `#f6f3ec` | Caption on the picture ground. |

#### Contrast

Text pairs must be at least 4.5:1. UI pairs (control, picture stroke, focus, line) must be at least 3:1. The vowel marks use `color.text`, so they take the text ratio. `color.focus` on `color.surface` is included because a ring also sits on a card. The spike's own comment did not list that pair; it is measured the same way.

| Pair | Kind | Light | Dark |
|------|------|------:|-----:|
| `color.text` / `color.bg` | text | 16.2:1 | 16.6:1 |
| `color.text` / `color.surface` | text | 17.5:1 | 15.1:1 |
| `color.muted` / `color.bg` | text | 9.5:1 | 12.0:1 |
| `color.muted` / `color.surface` | text | 10.3:1 | 10.9:1 |
| `color.controlText` / `color.control` | text | 11.4:1 | 11.8:1 |
| `color.onPicture` / `color.pictureBg` | text | 16.2:1 | 15.1:1 |
| `color.control` / `color.bg` | ui | 11.4:1 | 12.9:1 |
| `color.control` / `color.surface` | ui | 12.3:1 | 11.7:1 |
| `color.picture` / `color.pictureBg` | ui | 16.2:1 | 15.1:1 |
| `color.focus` / `color.bg` | ui | 11.4:1 | 16.0:1 |
| `color.focus` / `color.surface` | ui | 12.3:1 | 14.5:1 |
| `color.line` / `color.bg` | ui | 3.4:1 | 3.4:1 |
| `color.line` / `color.surface` | ui | 3.7:1 | 3.1:1 |

The lowest text pair is `color.muted` on `color.bg` in light, 9.5:1. The lowest UI pair is `color.line` on `color.surface` in dark, 3.1:1. That line cannot be lightened without failing 3:1.

#### Type

| Token | Value | Purpose |
|-------|-------|---------|
| `font.family` | Rubik, sans-serif | One face for Hebrew and Latin. Self-hosted, SIL OFL, the three woff2 files under `shared/fonts/`. |
| `font.weightWord` | 500 | Pointed word and headings. |
| `font.weightBody` | 400 | Body, glosses, and buttons. |
| `font.weightAxis` | 300 700 | The range inside the font files. Not a second text style. |
| `font.sizeWord` | 48px, 40px below `breakpoint.narrow`, 56px from `breakpoint.wide` | The meet-screen pointed word. Larger than `font.sizeGloss` at every breakpoint. |
| `font.sizePointed` | 24px | Floor for every other pointed line. |
| `font.sizeGloss` | 20px | English gloss. |
| `font.sizeBody` | 18px | Body, section headings, primary label. |
| `font.sizeTitle` | 32px | Page title and the lesson name on Today. |
| `font.sizeKicker` | 16px | Label above a title, and the picture caption. |
| `font.sizeIcon` | 24px | Speaker glyph. |
| `font.lineHeightWord` | 1.5 | Author default on a pointed line. |
| `font.lineHeightBody` | 1.5 | Author default on body text and headings. |
| `font.letterSpacing` | 0 | Tracking is not tightened. |

Pointed lines set `overflow: visible` and block padding of `space.2`, so a vowel mark is not clipped by the line box.

#### Spacing, radius, elevation, motion, and the sizes the components use

| Token | Value | Purpose |
|-------|-------|---------|
| `space.1` | 0.25rem | Focus offset and the smallest inset. |
| `space.2` | 0.5rem | Padding on the pointed word, gap inside a button. |
| `space.3` | 0.75rem | Gap inside the cluster, space before a heading. |
| `space.4` | 1rem | Scheme box and phone-dock padding. |
| `space.5` | 1.5rem | Correction panel, and the page inset on a narrow window. |
| `space.6` | 2.5rem | Cluster padding, gap between blocks, default page inset. |
| `space.7` | 4rem | Space under the mast. |
| `space.8` | 6rem | Space before a second screen, and the page inset on a wide window. |
| `space.page` | `space.6`, `space.5` when narrow, `space.8` when wide | Page inset. It points at those three tokens. |
| `radius.default` | 2px | Corner of a card, a button, a picture frame, and the scheme box. |
| `elevation.surface` | none | No shadow. The edge is `color.line` and `border.width`. |
| `motion.duration` | 0.15s | The only duration. Primary background-color, and only inside `prefers-reduced-motion: no-preference`. |
| `motion.easing` | ease | Easing of that one change. |
| `motion.reduced` | animation none; transition none; scroll-behavior auto | What `prefers-reduced-motion: reduce` sets on every element. |
| `size.target` | 24px | Minimum pointer target, both axes. |
| `size.targetPrimary` | 44px | Minimum of play, check, and next, both axes. |
| `size.picture` | 220px, 200px when narrow | Maximum width of the picture frame. |
| `size.measure` | 34rem, 100% when narrow, 38rem when wide | Width of the one column. |
| `border.width` | 1px | Card edge, control edge, picture stroke. |
| `focus.width` | 3px | Focus-ring thickness. |
| `focus.offset` | `space.1` | Gap between the control and the ring. |
| `breakpoint.narrow` | 599px max | Phone. The primary action sticks to the bottom of Today. |
| `breakpoint.wide` | 1280px min | The column and the word grow. No second navigation column. |

### 3. Components

New learner components are functions in `shared/ui/` that return escaped HTML, which is ADR-0005's component contract. Class names are styling. Behavior and tests use `data-*`. States:

| Component | States | Notes |
|-----------|--------|-------|
| App bar | resting, current page (`aria-current="page"`), admin link present, nav locked | The learner mast is the calm-type header: a kicker and one title. The existing shell keeps `.hebrew-shell` and `data-nav` until that screen is switched in the same change as its tests. |
| Button, primary | resting, focus-visible | One per learner screen. Minimum `size.targetPrimary`. Label is text. |
| Button, quiet | resting, focus-visible | Back link, scheme choice, filter chip. Minimum `size.target`. |
| Card | resting | `color.surface`, `border.width`, `radius.default`, `elevation.surface` none. |
| Word cluster | meet, recall | Meet: pointed word, English gloss, picture, and audio in one cluster. Recall: no picture; the attempt stays; the correction panel shows the right form. |
| Picture frame | placeholder, image | Placeholder has a caption and a stroke of `color.picture`. An image uses the plan's alt. An empty alt is refused, per ADR-0005. |
| Audio control | idle, focus-visible | Inside the cluster, so the word is still on screen. No autoplay. The speaker glyph does not animate. |
| Progress indicator | loading, error, empty, ready, low-data | Ready shows items done and words due, including the stage counts. It does not show a streak, points, a badge, or a leaderboard. `data-progress-state` keeps those five values. |
| Feedback message | correction, complete | Correction says more than "wrong" and includes the correct form before the learner leaves the item. Complete is a sentence, not a burst of motion. |
| Form field | resting, focus-visible, disabled | Scheme radios are `size.target`. Builder fields keep the admin hooks in Decision 8. |
| Lesson-step frame | one item, advance | One item, then the learner moves with the primary control. On a phone that control is in the bottom dock. |
| Builder panels | lessons list, editor, validation, audio plan, users, history | Same tokens when the screen is touched. The hooks in Decision 8 stay. This record does not restyle the builder. |

### 4. Interface rules

The rows are `RSCH-006`'s interface rule table, in order, with the same rule text, grade, source, and label. The Surface column says where this product applies the row. The Difference column is empty except where the pick and the row meet. That meeting is explained under the table. It does not change the row.

| # | Rule | Grade | Source | Label | Surface | Difference |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Set the pointed Hebrew word at least 18 pt (about 24 px), larger than the English around it, in a font that places nikud on the visual axis of the letter. | Low | [5][7][8] | adapted | Meet word (`font.sizeWord`), other pointed lines (`font.sizePointed`), gloss (`font.sizeGloss`). Rubik. | None. |
| 2 | Render the pointed word as live text, not as a picture of text, so it can resize to 200%. | High | [1] 1.4.4, 1.4.5 | evidence-backed | Every pointed word. | None. |
| 3 | The author's default line height on a pointed line is 1.5. | Low | [5] | product choice | `font.lineHeightWord` on the word, the example, and the attempt. | None. The label was already product choice. |
| 4 | The points stay fully visible when line height is 1.5, letter spacing is 0.12 em, and word spacing is 0.16 em. | High | [1] 1.4.12 | evidence-backed | Pointed lines. `overflow: visible`. Author letter-spacing stays `font.letterSpacing`. | None. |
| 5 | The pointed word, including the vowel marks, has a contrast of at least 4.5:1 (3:1 if the word is large text: 18 pt or 14 pt bold). | High | [1] 1.4.3 | evidence-backed | `color.text` and `color.muted` pairs in Decision 2. Vowel marks use `color.text`. | None. The palette also clears 4.5:1 where large text would have allowed 3:1. |
| 6 | Controls, and the parts of a picture that carry the meaning, have a contrast of at least 3:1. | High | [1] 1.4.11 | evidence-backed | Primary control, picture stroke, focus ring, card line. | None. |
| 7 | Every pointer target is at least 24 by 24 CSS pixels, aside from the WCAG exceptions. | High | [1] 2.5.8 | evidence-backed | Quiet controls, radios, filter chips, back link. `size.target`. | None. |
| 8 | Play, check, and next are at least 44 by 44 CSS pixels. | Moderate | [1] 2.5.5; [19] | adapted | Audio, the check on a practice item, and the advance control. `size.targetPrimary`. | None. |
| 9 | When a picture is on the screen, it sits in one cluster with the pointed word and the English meaning. The audio plays while the word is visible. Whether to show the picture is RSCH-005, not this report. | Moderate | [2][3][4] | adapted | Meet state of the word cluster. | None. |
| 10 | One item is on screen at a time, and the learner advances. | Moderate | [2][3] | adapted | Lesson-step frame. | None. |
| 11 | The screen does not add decoration beyond the word, the meaning, the audio, and the picture. | Moderate | [2][3] | adapted | Today and the lesson step. | See below. The row text is unchanged. |
| 12 | After a miss, the correction is more than "wrong," and it includes the correct form before the learner leaves the item. | Moderate | [11][12] | evidence-backed | Recall state, feedback message. | None. |
| 13 | The learner gets one retry, then the correct form. The retry does not continue until they happen to guess it. | Low | [12][18] | adapted | Recall state, and the activities the retry smoke already covers. | None. |
| 14 | Progress shows what has been learned (items done, words due). It does not show points, a streak, or a leaderboard. | Low | [14][15][16][18] | product choice | Today due line, progress indicator. | None. The label was already product choice. |
| 15 | A Hebrew page sets `dir="rtl"` and `lang="he"`. Each English gloss is wrapped with `dir="ltr"`. Direction is not set with CSS. | High | [9][10] | evidence-backed | Document element, every gloss. Decision 6. | None. |
| 16 | The home screen has one next action. Other destinations are at most one level down. | Low | [21][22][24] | product choice | Today. | None. Hick's law is not the reason, which is the grade `RSCH-006` already gave. |
| 17 | The main phone action sits in the lower reachable area, and it is re-checked on a large phone. | Low | [20] | adapted | Phone dock, below `breakpoint.narrow`. | None. |
| 18 | Motion that starts by itself and lasts more than five seconds can be paused. Motion started by a tap honors reduced motion. | High for the pause; the reduced-motion half is AAA | [1] 2.2.2, 2.3.3 | evidence-backed for 2.2.2; product choice to meet 2.3.3 | `motion.duration`, `motion.reduced`. | None. Nothing starts by itself. The reduced-motion half is met on purpose. |

**Rule 11, the only difference in application.** On an introduction block, one example sentence sits under the cluster. It is not the picture, the audio, or the gloss. It is there because ADR-0003 asks the learner to read an example, and because the operator picked the mockup that keeps it. That exception is a product choice. It does not add a banner, a drawing, a word index, or a second action. Every other surface follows the row as written.

**Dark mode is not a row.** `RSCH-006`'s "Don't" list says not to require dark mode in order to help reading. Decision 1 ships it as an optional scheme. No row above was edited to justify it.

### 5. Accessibility baseline

The conformance target is WCAG 2.2 Level AA, plus two AAA criteria taken on purpose: 2.5.5 for the primary controls, and 2.3.3 for motion started by a tap. `RSCH-006` grades the AA rows High and those two additions as the product choice its rule 8 and rule 18 already name.

| Criterion | Level | Number this product commits to |
|-----------|-------|--------------------------------|
| 1.4.3 Contrast (Minimum) | AA | Text, including vowel marks, at least 4.5:1. Large text (18 pt, or 14 pt bold) would be allowed 3:1. This palette does not use that lower bar. |
| 1.4.11 Non-text Contrast | AA | Control parts, picture strokes, the focus ring, and card lines at least 3:1 against the adjacent colour. |
| 1.4.4 Resize Text | AA | Text can grow to 200% with no loss of content or function. |
| 1.4.5 Images of Text | AA | Pointed Hebrew is live text. |
| 1.4.10 Reflow | AA | At a width of 320 CSS pixels, content does not need scrolling in two dimensions, except where a two-dimensional layout is essential. |
| 1.4.12 Text Spacing | AA | No loss when line height is at least 1.5 times the font size, paragraph spacing at least 2 times the font size, letter spacing at least 0.12 times the font size, and word spacing at least 0.16 times the font size. |
| 2.5.8 Target Size (Minimum) | AA | Pointer targets at least 24 by 24 CSS pixels, aside from the exceptions in the criterion. |
| 2.4.7 Focus Visible | AA | A keyboard focus indicator is visible. |
| 2.4.11 Focus Not Obscured (Minimum) | AA | Author content does not hide a focused component entirely. The phone dock must not cover the focused control. |
| 2.5.7 Dragging Movements | AA | A drag, if one is added, also works with a single pointer without dragging. |
| 2.5.5 Target Size (Enhanced) | AAA, product choice | Play, check, and next are at least 44 by 44 CSS pixels. |
| 2.2.2 Pause, Stop, Hide | A | Motion that starts by itself, lasts more than 5 seconds, and runs beside other content has a pause, stop, or hide control. This product has no such motion. |
| 2.3.3 Animation from Interactions | AAA, product choice | Motion started by a tap is removed when `prefers-reduced-motion: reduce` matches. |
| 1.4.7 Low or No Background Audio | AAA, only if a clip has music under speech | Background sound is absent, can be turned off, or is at least 20 dB below the speech. Speech-only clips are the current case. |

1.4.6 (7:1) is not the target. The lowest text pair in Decision 2 is still above 7:1. That is a property of these hex values, not a second conformance claim.

The focus ring is `focus.width` (3px), which is above the 2 CSS pixel perimeter in 2.4.13. The ring colour against the page and against a card is the focus pairs in Decision 2, both above 3:1. 2.4.13 is not added to the conformance target. 2.4.7 and 2.4.11 are.

**Check command.** `node spikes/look/check.js` fails a text pair under 4.5:1, a UI pair under 3:1, a target under 24px, a primary target under 44px, pointed text under 24px, a pointed line height other than 1.5, motion outside the reduced-motion queries, or `direction` set in CSS. For `tokens.json`, recompute each `contrast[]` entry with that file's contrast function and require the recorded one-decimal ratio. A text entry below 4.5 or a UI entry below 3 fails.

### 6. Right-to-left and bidirectional text

These are `RSCH-006` rule 15, High, evidence-backed, from the W3C internationalization articles. No user study of Hebrew-English mixing errors was read, so this is the markup requirement, not a measured drop in reading mistakes.

- A Hebrew page sets `dir="rtl"` and `lang="he"` on the `html` element. Direction is that attribute. Stylesheets do not set the `direction` property.
- Layout uses logical properties: `margin-inline`, `padding-inline`, `inset-inline-start`, `inset-inline-end`, `border-inline`, `text-align: start`. No `left` or `right` spacing, and no `text-align: left` or `text-align: right`.
- Each English gloss is an element with `dir="ltr"` and `lang="en"`. A Hebrew word inside an English sentence would be wrapped the other way. The gloss is the case this product has.
- When an English gloss ends a Hebrew question, put U+200F (right-to-left mark) before the Hebrew punctuation that follows, so the punctuation stays with the Hebrew clause.
- European digits in a Hebrew sentence follow the bidi algorithm. A due count is not wrapped and is not reordered with CSS. An English phrase that contains digits is inside the `dir="ltr"` gloss.
- Icons that mean back, forward, next, or previous flip with the inline direction. The speaker, a check mark, and a picture do not flip.

### 7. What is excluded

- No points, no streaks, no badges, no leaderboard, and no experience points. Progress is items done and words due. `RSCH-006` rule 14, product choice, Low, the same grade `RSCH-003` left on language-app gamification. `progress-view.test.js` already rejects a class name containing streak, points, badge, or leaderboard.
- No decorative motion that cannot be turned off. The only motion is `motion.duration` on the primary background, inside `prefers-reduced-motion: no-preference`. The reduce query sets `motion.reduced` on every element, with no exception.
- No autoplay of audio or animation.
- No motion that starts by itself and lasts more than 5 seconds. If one is ever added, 2.2.2 requires a pause.
- No shadow (`elevation.surface`).
- No second column of navigation, no word index on the lesson step, and no bar of extra destinations on Today. Those were looks b and c. They were not picked.

### 8. Class and attribute contracts

**Naming scheme.** Styling classes are kebab-case, one role each, taken from the calm-type mockup for new learner screens: `page`, `mast`, `kicker`, `block`, `lesson`, `pointed`, `pointed-line`, `word`, `gloss`, `cluster`, `placeholder`, `correction`, `attempt`, `due-line`, `primary`, `audio`, `dock`, `scheme`, `screen`. A bare `lesson` is the lesson title. It is not `lesson-card`, `lesson-status`, or `lesson-editor`. Those three are test hooks and stay as they are.

`.meaning` is both the gloss class in the mockup and a hook below. It stays. `.kicker` and `.example` already exist in `shared/worksheet.css` and are not in the hook list. When that screen is touched, restyle them with these tokens. Do not invent a second class for the same role.

New tests query `data-*`. A class is styling. A hook in the tables below changes only in the same change as the test that queries it. That is ADR-0005 Decision 6.

**Fate.** `kept` means the name stays. `rename` would require the test to change in that same change. `retire` means the hook must not be introduced. Every row below is `kept` except the one retired value.

**Scan.** The rows are the class names and `data-*` attributes that `test/*.test.js` and `test/chrome-*-smoke.js` actually assert, queried on 2026-10-08. The spec names `app-shell.test.js`, `progress-view.test.js`, `review-page.test.js`, `learner-support.test.js`, `admin-users-view.test.js`, `admin-lesson-editor.test.js`, and the Chrome smokes. The same scan added hooks that only `retry-before-reveal-smoke.test.js`, `production-recall.test.js`, or `story-gloss.test.js` assert, so a search of `test/` does not find a hook this list omits.

These strings are not hooks. `--user-data-dir` is the Chrome launch flag in every smoke, and it contains the letters `data-dir`. `evil.example` is a hostile URL in a security test. The words "side-by-side" in a test title are prose. `chrome-audio-smoke.js` queries no class and no `data-*` attribute.

`pair-c0` through `pair-c7` are required by `learner-support.test.js`, which reads eight `.match-pair.pair-cN` rules out of `shared/worksheet.css`.

#### `data-*` attributes (70, all kept)

| Hook | Asserted by | Fate |
|------|-------------|------|
| `data-accuracy-state` | progress-view.test.js | kept |
| `data-act-move` | chrome-admin-lessons-smoke.js | kept |
| `data-act-type` | chrome-admin-lessons-smoke.js | kept |
| `data-action` | chrome-learner-support-smoke.js, chrome-production-recall-smoke.js, chrome-story-gloss-smoke.js, production-recall.test.js, retry-before-reveal-smoke.test.js, story-gloss.test.js | kept |
| `data-admin-action` | chrome-admin-smoke.js | kept |
| `data-admin-guard-error` | admin-users-view.test.js, chrome-admin-smoke.js | kept |
| `data-admin-history-body` | admin-users-view.test.js, chrome-admin-smoke.js | kept |
| `data-admin-load-older` | admin-users-view.test.js, chrome-admin-smoke.js | kept |
| `data-admin-progress` | chrome-admin-smoke.js | kept |
| `data-admin-trouble` | chrome-admin-smoke.js | kept |
| `data-admin-users-body` | admin-users-view.test.js, chrome-admin-smoke.js | kept |
| `data-admin-view` | admin-lesson-editor.test.js, chrome-admin-lessons-smoke.js, chrome-admin-smoke.js | kept |
| `data-attempts` | progress-view.test.js | kept |
| `data-audio` | chrome-learner-support-smoke.js, retry-before-reveal-smoke.test.js | kept |
| `data-audio-cancel` | chrome-admin-lessons-smoke.js | kept |
| `data-audio-confirm` | chrome-admin-lessons-smoke.js | kept |
| `data-audio-credit-warn` | chrome-admin-lessons-smoke.js | kept |
| `data-audio-error` | chrome-admin-lessons-smoke.js | kept |
| `data-audio-plan` | chrome-admin-lessons-smoke.js | kept |
| `data-audio-result` | chrome-admin-lessons-smoke.js | kept |
| `data-count` | progress-view.test.js | kept |
| `data-coverage` | chrome-story-gloss-smoke.js, story-gloss.test.js | kept |
| `data-direction` | chrome-production-recall-smoke.js, chrome-review-smoke.js, production-recall.test.js, progress-view.test.js | kept |
| `data-editor-error` | chrome-admin-lessons-smoke.js | kept |
| `data-event-id` | admin-users-view.test.js, chrome-admin-smoke.js | kept |
| `data-fill-feedback` | retry-before-reveal-smoke.test.js | kept |
| `data-grammar` | retry-before-reveal-smoke.test.js | kept |
| `data-id` | chrome-learner-support-smoke.js, learner-support.test.js | kept |
| `data-index` | retry-before-reveal-smoke.test.js | kept |
| `data-key` | chrome-review-smoke.js | kept |
| `data-lesson` | chrome-review-smoke.js | kept |
| `data-lesson-action` | chrome-admin-lessons-smoke.js | kept |
| `data-lesson-audio` | chrome-admin-lessons-smoke.js | kept |
| `data-lesson-level` | chrome-today-lessons-smoke.js | kept |
| `data-lesson-save` | admin-lesson-editor.test.js, chrome-admin-lessons-smoke.js | kept |
| `data-lesson-slug` | chrome-admin-lessons-smoke.js, chrome-today-lessons-smoke.js | kept |
| `data-lesson-status` | chrome-today-lessons-smoke.js | kept |
| `data-lesson-validate` | chrome-admin-lessons-smoke.js | kept |
| `data-lessons-state` | chrome-admin-lessons-smoke.js, chrome-today-lessons-smoke.js | kept |
| `data-level-filter` | chrome-today-lessons-smoke.js | kept |
| `data-listen-hint` | retry-before-reveal-smoke.test.js | kept |
| `data-mark` | chrome-production-recall-smoke.js | kept |
| `data-match` | chrome-learner-support-smoke.js, learner-support.test.js | kept |
| `data-nav` | app-shell.test.js, chrome-signin-smoke.js | kept |
| `data-pct` | progress-view.test.js | kept |
| `data-progress-state` | chrome-admin-smoke.js, chrome-signin-smoke.js, progress-view.test.js | kept |
| `data-result` | retry-before-reveal-smoke.test.js | kept |
| `data-role` | admin-users-view.test.js, chrome-admin-smoke.js | kept |
| `data-save-result` | chrome-admin-lessons-smoke.js | kept |
| `data-shell-sync` | app-shell.test.js, chrome-progress-smoke.js | kept |
| `data-signal` | review-page.test.js | kept |
| `data-slug` | chrome-admin-lessons-smoke.js | kept |
| `data-speak` | chrome-learner-support-smoke.js, learner-support.test.js, production-recall.test.js, retry-before-reveal-smoke.test.js | kept |
| `data-stage` | progress-view.test.js | kept |
| `data-status` | chrome-admin-smoke.js | kept |
| `data-step-type` | chrome-production-recall-smoke.js, chrome-story-gloss-smoke.js | kept |
| `data-today-card` | chrome-today-lessons-smoke.js | kept |
| `data-today-state` | chrome-today-lessons-smoke.js | kept |
| `data-token` | chrome-story-gloss-smoke.js | kept |
| `data-user-id` | chrome-admin-smoke.js | kept |
| `data-val-section` | admin-lesson-editor.test.js, chrome-admin-lessons-smoke.js | kept |
| `data-validate-out` | admin-lesson-editor.test.js, chrome-admin-lessons-smoke.js | kept |
| `data-vis-mode-select` | chrome-admin-lessons-smoke.js | kept |
| `data-vis-save` | chrome-admin-lessons-smoke.js | kept |
| `data-vis-status` | chrome-admin-lessons-smoke.js | kept |
| `data-vis-user` | chrome-admin-lessons-smoke.js | kept |
| `data-vocab-body` | chrome-admin-lessons-smoke.js | kept |
| `data-vocab-field` | chrome-admin-lessons-smoke.js | kept |
| `data-vocab-index` | chrome-admin-lessons-smoke.js | kept |
| `data-vocab-move` | chrome-admin-lessons-smoke.js | kept |

Values those tests treat as part of the hook, so a rename of the value is the same kind of break as a rename of the attribute:

| Attribute | Values the tests require | Fate |
|-----------|--------------------------|------|
| `data-action` | `gloss-audio`, `gloss-dismiss`, `type-hint`, `type-check`, `type-next`, `toggle-reveal`, `mark-word`, `listen-pick`, `check` | kept |
| `data-admin-action` | `create`, `reset`, `disable`, `make-user` | kept |
| `data-nav` | `admin`, `review`, `lessons` | kept |
| `data-progress-state` | `loading`, `error`, `empty`, `ready`, `low-data` | kept |
| `data-accuracy-state` | `low-data` | kept |
| `data-match` | `en`, `he` | kept |
| `data-signal` | `miss` | kept |
| `data-direction` | `he_en`, `en_he` | kept |
| `data-stage` | `new`, `learning`, `longTerm` | kept |
| `data-step-type` | `story`, `words` | kept |
| `data-today-state` | `ready`, `no-lessons`, `error` | kept |
| `data-lessons-state` | `ready`, `no-lessons`, `filtered-empty`, `error` | kept |
| `data-level-filter` | `advanced`, `all`, `basic` | kept |
| `data-admin-view` | `lessons`, `user`, `lesson` | kept |
| `data-coverage` | `unknown` | kept |
| `data-mark` | `knew` | kept |
| `data-lesson-action` | `new` | kept |
| `data-vocab-field` | `meaning` | kept |
| `data-vocab-move` | `up` | kept |
| `data-act-move` | `up` | kept |
| `data-val-section` | `activities` | kept |

#### Classes (50, all kept)

| Hook | Asserted by | Fate |
|------|-------------|------|
| `back-link` | chrome-signin-smoke.js | kept |
| `banner` | chrome-story-gloss-smoke.js | kept |
| `banner-amber` | chrome-story-gloss-smoke.js | kept |
| `card` | learner-support.test.js | kept |
| `choice-card` | retry-before-reveal-smoke.test.js | kept |
| `correct` | retry-before-reveal-smoke.test.js | kept |
| `he-word` | chrome-production-recall-smoke.js, production-recall.test.js | kept |
| `hebrew-shell` | app-shell.test.js, chrome-signin-smoke.js | kept |
| `hebrew-shell-link` | chrome-signin-smoke.js | kept |
| `hebrew-shell-nav-locked` | chrome-signin-smoke.js | kept |
| `hebrew-shell-privacy` | chrome-signin-smoke.js | kept |
| `hidden` | chrome-admin-lessons-smoke.js, chrome-admin-smoke.js, chrome-learner-support-smoke.js, chrome-production-recall-smoke.js, chrome-review-smoke.js, chrome-signin-smoke.js, learner-support.test.js | kept |
| `lesson-card` | chrome-admin-lessons-smoke.js | kept |
| `lesson-editor` | chrome-admin-lessons-smoke.js | kept |
| `lesson-status` | chrome-today-lessons-smoke.js | kept |
| `lesson-view-link` | chrome-admin-lessons-smoke.js | kept |
| `lessons-filter` | chrome-today-lessons-smoke.js | kept |
| `lessons-grid` | chrome-admin-lessons-smoke.js | kept |
| `mark-row` | chrome-production-recall-smoke.js | kept |
| `match-grid` | learner-support.test.js | kept |
| `match-pair` | chrome-learner-support-smoke.js, learner-support.test.js | kept |
| `match-pairs` | learner-support.test.js | kept |
| `meaning` | chrome-admin-lessons-smoke.js, chrome-production-recall-smoke.js | kept |
| `pair-c0` | learner-support.test.js | kept |
| `pair-c1` | learner-support.test.js | kept |
| `pair-c2` | learner-support.test.js | kept |
| `pair-c3` | learner-support.test.js | kept |
| `pair-c4` | learner-support.test.js | kept |
| `pair-c5` | learner-support.test.js | kept |
| `pair-c6` | learner-support.test.js | kept |
| `pair-c7` | learner-support.test.js | kept |
| `pair-en` | chrome-learner-support-smoke.js | kept |
| `pair-he` | chrome-learner-support-smoke.js | kept |
| `pair-num` | chrome-learner-support-smoke.js | kept |
| `path-screen` | chrome-admin-lessons-smoke.js | kept |
| `pill` | chrome-learner-support-smoke.js | kept |
| `player-top` | chrome-signin-smoke.js | kept |
| `progress-count-n` | progress-view.test.js | kept |
| `progress-stage-counts` | chrome-signin-smoke.js | kept |
| `progress-view` | chrome-admin-smoke.js | kept |
| `reveal` | chrome-production-recall-smoke.js | kept |
| `stack` | learner-support.test.js | kept |
| `step-screen` | chrome-admin-lessons-smoke.js, chrome-learner-support-smoke.js, retry-before-reveal-smoke.test.js | kept |
| `story-gloss-en` | chrome-story-gloss-smoke.js | kept |
| `story-gloss-popover` | chrome-story-gloss-smoke.js | kept |
| `story-token` | chrome-story-gloss-smoke.js | kept |
| `type-answer` | chrome-learner-support-smoke.js | kept |
| `vocab-card` | chrome-admin-lessons-smoke.js, chrome-production-recall-smoke.js | kept |
| `vocab-head` | chrome-production-recall-smoke.js | kept |
| `wrong` | retry-before-reveal-smoke.test.js | kept |

#### Retired

| Hook | Asserted by | Fate |
|------|-------------|------|
| `data-action="set-dir"` | production-recall.test.js, which requires the markup not to contain it | retire |

The contract is 121 rows: 70 kept attributes, 50 kept classes, and 1 retired value.

---

## Consequences

- SPEC-009 and INIT-004 have one palette, one type scale, and one list of hooks. They do not choose colours again.
- Dark mode is real work on every screen that moves to tokens. ADR-0005 already said both themes meet 4.5:1. The measured floor is the muted pair at 9.5:1 and the line-on-surface pair at 3.1:1.
- The introduction example is a standing exception to rule 11. A later screen that adds a drawing or a banner is not covered by that exception.
- Dark mode is not a claim that reading gets easier. The light scheme remains the one `RSCH-006` has evidence for.
- Hand-built CSS can drop a logical property or a `dir`. The spike check and the component tests are the mitigation ADR-0005 already names.
- A hook marked kept will break a smoke if it is renamed alone. The retired value must not come back.

---

## Implementation plan (downstream)

| Consumer | What it takes from this record |
|----------|--------------------------------|
| SPEC-009 | Renders one unit with `shared/ui/` and these tokens. No new direct dependency unless ADR-0005 is amended first. |
| INIT-004 design system | Appends the token block to `shared/worksheet.css` using the custom properties named in `tokens.json`. New rules do not add a raw hex. |
| INIT-004 screens | Switch one screen at a time. A changed hook and its test land in the same change. Legacy screens keep their hooks until then. |
| SPEC-010 | The build plan cites this record for tokens, rules, and the hook fates. |

Rollback of this record is deleting this file and `docs/decisions/ADR-0006-assets/`. No runtime behavior depends on it until SPEC-009.

---

## Security considerations

- Sensitivity is light. The file holds colours, sizes, and DOM hook names. It holds no secret, no session rule, and no learner record.
- Admin hooks (`data-nav="admin"`, `data-admin-view`, `data-admin-users-body`, and the other `data-admin-*` rows) stay. Hiding the admin link from a non-admin is still `data-nav="admin"` being absent, which `app-shell.test.js` asserts.
- Tokens are not a new client bundle and not a place to put a key. ADR-0005 keeps keys in the server environment.

---

## References

- [ADR-0005: Stack and architecture](ADR-0005-stack-and-architecture.md) — Option A, token block, `data-theme`, logical properties, hook stability
- [ADR-0003: Course and lesson-plan language](ADR-0003-course-and-lesson-plan-language.md) — the introduction block's example, which is the rule 11 exception
- `RSCH-006` — `sdd/research/RSCH-006-learner-interface-design/RSCH-006-learner-interface-design-report.md`, interface rule table and the WCAG 2.2 rows in section 1.5
- `spikes/look/a-calm-type/look.css`, `today.html`, `lesson-step.html`, `README.md` — the picked look, measured 2026-10-08
- `spikes/look/COMPARE.md` — why b and c were not picked
- `spikes/look/check.js` — contrast, target, and reduced-motion check. Command: `node spikes/look/check.js`
- [tokens.json](ADR-0006-assets/tokens.json) — the same token values and the same contrast ratios

## Changelog

| Date | Change |
|------|--------|
| 2026-10-08 | Accepted — `INIT-003/SPEC-008`. Operator pick a-calm-type. Dark mode ships. |
