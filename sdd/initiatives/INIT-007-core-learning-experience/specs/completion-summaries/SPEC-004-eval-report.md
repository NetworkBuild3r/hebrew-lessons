# SPEC-004 eval report

**Spec:** SPEC-004  
**Initiative:** INIT-007  
**Date:** 2026-10-11  
**Provenance:** `INIT-007/SPEC-004`  
**Verdict:** PASS (implementing agent self-check against acceptance criteria)

## Checks

| Check | Result | Notes |
|---|---|---|
| ac-1 Pointed word ≥40px, lh ≥1.5, overflow visible | PASS | `.he-word` → `--size-word` / `--line-height-word` |
| ac-2 Unified cluster (word + gloss + audio + image) | PASS | `.vocab-cluster` in recognition + recall cards |
| ac-3 `.reading-now` follow-along | PASS | Amber wash + left inset; driven by `paintStory` |
| ac-4 Gloss popover pointed + meaning + audio | PASS | `escapeHtml`; `data-speak` / `playClip` pulse |
| ac-5 Pause/Resume/Stop ≥44px | PASS | `.story-ctrl` min-width/height 44px; status visibility |
| Live text (never image of nikud) | PASS | Text nodes / button text only |
| SPEC-002 audio reuse | PASS | `is-playing` on speak / gloss / he-word taps |
| CSS ownership | PASS | Vocab/story/gloss only; no dock/drill/review rewrite |
| Blocking: `npm test` | PASS | 855 pass, 0 fail, exit 0 |
| Blocking: `test/story-gloss.test.js` | PASS | exit 0 |
| `chrome-story-gloss-smoke.js` | PASS | `ok: true` |

## Scope discipline

- Primary files: `shared/activities/words.js`, `shared/activities/story.js`, `shared/worksheet.css`
- No `worksheets/*/index.html` edits, no new deps, no gamification, no commit
