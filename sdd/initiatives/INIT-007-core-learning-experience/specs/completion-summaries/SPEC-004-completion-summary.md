# SPEC-004 completion summary

**Spec:** SPEC-004 · Word cluster and story reading experience  
**Initiative:** INIT-007  
**Completed:** 2026-10-11  
**Status:** complete (orchestrator closed 2026-10-11 — 855 pass; story-gloss ok)  
**Provenance:** `INIT-007/SPEC-004`

## What shipped

- `shared/activities/words.js`: unified `.vocab-cluster` card — live pointed `.he-word` at `--size-word` (clickable via `data-speak`), English gloss, speaker button (SPEC-002 `is-playing`), optional `{ mediaId, alt }` image, POS/trap as `.vocab-pill` chips. Recognition + recall (`he_en` / `en_he`) layouts preserved; all `data-*` hooks kept.
- `shared/activities/story.js`: warm story surface hooks; dedicated Pause / Resume / Stop controls (`.story-ctrl`, ≥44px); gloss popover with escaped pointed Hebrew + meaning + audio (`data-speak` / `playClip` for pulse); Escape / outside-click dismiss unchanged.
- `shared/worksheet.css`: INIT-007/SPEC-004 block for `.vocab-card`, `.he-word`, `.story-body`, `.reading-now`, `.story-gloss-popover`, `.story-token`, `.story-ctrl` only — no drill/review/shell dock rewrites.

## Acceptance criteria

| Id | Result | Evidence |
|---|---|---|
| ac-1 | Pass | `.he-word` uses `var(--size-word)` (≥40px narrow / 48px default / 56px wide), `line-height: var(--line-height-word, 1.5)`, `overflow: visible`. |
| ac-2 | Pass | Word, meaning, speak control, and optional image share `.vocab-cluster` / `.vocab-card`. |
| ac-3 | Pass | `paintStory` still applies `.reading-now` with warm amber wash + inset border. |
| ac-4 | Pass | `.story-token` opens `.story-gloss-popover` with pointed word, escaped gloss, audio button. |
| ac-5 | Pass | `.story-ctrl` min 44×44; Pause / Resume / Stop with icon + label and clear show/hide by status. |

## Understood

- und-1: Pointed Hebrew remains live text (`button.he-word` / story tokens), never an image of text.
- und-2: Line-height ≥1.5 via `--line-height-word` / story `line-height: 1.7`.
- und-3: Follow-along highlight tracks `Speech.story` via existing `onStoryPaint` / `paintStory`.

## Tests

- `npm test` — pass (855 / 855, exit 0)
- `node --test test/story-gloss.test.js` — pass
- `node test/chrome-story-gloss-smoke.js` — pass (`ok: true`)

## Scope discipline

- CSS ownership limited to vocab / story / gloss selectors.
- No `worksheets/*/index.html` edits, no new deps, no gamification, no commit.

## Follow-ups

- SPEC-005 / SPEC-006 own drill and review surfaces.
- SPEC-007 can add visual/a11y smokes for 48px cluster and story control targets.
