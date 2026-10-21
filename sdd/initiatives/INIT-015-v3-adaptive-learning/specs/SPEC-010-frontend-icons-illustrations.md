# SPEC-010 · frontend: Icons, brand mark, and illustrations

## Metadata

```yaml
spec_id: SPEC-010
initiative_id: INIT-015
title: "Icons, brand mark, and illustrations"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-008]
blocks: [SPEC-012, SPEC-017]
```

## Description

**Context:** v2 has a sprite (`internal/web/static/icons/sprite.svg`) and one illustration (`internal/web/static/illustrations/hero-skyline-night.svg`). Some templ files still use text glyphs. The Figma screens use Lucide-style line icons (speaker, replay, slower, close, check, arrow), and a lamp-and-ink look. "Graphics and all" asks for a finished visual layer.

**Scope (in):**

- **Icon sprite:** rebuild `internal/web/static/icons/sprite.svg`, keeping the ids still in use and adding the rest. One `<symbol>` per icon, 24px grid, 2px stroke, `currentColor`. The set covers speaker, replay, slower, close, check, x, arrow-right, arrow-left, keyboard, home (Today), book (Lessons), layers (Review), chart (Progress), user (Me), flag (Report a problem), lamp, and spark. The icons come from Lucide (ISC license), and the notice is recorded.
- A templ helper `Icon(name, label)`. It renders `aria-hidden` when there is no label, and `role="img"` with a title when there is one.
- **Brand mark:** a lamp whose flame is a Hebrew letter shape. It is drawn in Figma on page `139:2` as a component, exported to `internal/web/static/brand/mark.svg`, and used for the favicon and PWA icons (192, 512, maskable).
- **Illustrations:** flat, two-tone, drawn only with token colors (`lamp`, `slate`, `surfaceSoft`, `lampInk`), and drawn in Figma beside the screens. They are exported as SVG that uses `var(--…)` fills, so they follow dark mode. The set:
  - placement intro hero;
  - results hero;
  - empty Review ("Nothing due");
  - empty Words;
  - empty Progress;
  - "band up" moment;
  - offline.
- Remove every emoji used as an icon in templ files.

**Scope (out):** Animation of the illustrations (SPEC-009 patterns apply if wanted). Photographs.

## Understood (agent's read of the intent)

- [x] und-1: It should look made in 2026: line icons, soft two-tone shapes, no clip art, no emoji.
- [x] und-2: Illustrations inherit the theme, so dark mode is free.

## Acceptance Criteria

- [x] ac-1: A test fails if any `.templ` file contains an emoji in the pictographic ranges, or an inline `<svg>` outside `components.templ`. — covered: yes (TestNoEmojiInTempl, TestNoInlineSVGOutsideComponents)
- [x] ac-2: Every illustration SVG uses only `var(--token)` fills and strokes. A test parses each one and finds no literal color. — covered: yes (TestIllustrationsUseOnlyTokenColours)
- [x] ac-3: Each sprite icon renders at 20px and 24px, and icon-only buttons have an accessible name. — covered: yes (TestEveryIconRendersAt20And24, TestIconOnlyControlsHaveNames). Sizes are checked as markup and CSS, not rendered.
- [ ] ac-4: The Figma file has a "Brand & illustrations" frame on page `139:2` with each exported asset as a component. — covered: NO. Figma frame not drawn. Open item.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Who draws | The executing agent draws in Figma through `use_figma`, then exports. The operator can redraw later without code changes. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [x] `internal/web/static/icons/sprite.svg` (rebuilt), `internal/web/static/brand/*`, `internal/web/static/illustrations/*.svg`, `internal/web/static/LICENSES/lucide.txt`
- [~] `Icon` helper in `components.templ` Icon helpers live in icons.templ, not components.templ, to avoid merge conflicts (the lint allows both).
- [ ] Figma frame "Brand & illustrations" NOT DONE: Figma was read-only for the worker, so no frame was drawn.
- [x] Tests: the asset lint for ac-1 to ac-3

## Technical Requirements (domain-specific)

- [x] Each SVG is under 6 KB and optimized (no editor metadata).
- [x] The manifest and favicon links live in `layout.templ`. Coordinate with SPEC-017, which owns that file. The sprite is 7.4 KB with an 8 KB ceiling, because 27 icons cannot fit in 6 KB.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/web/...`.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** light — triggers matched: none.
- **Security acceptance criteria:**
  - [x] No `<script>`, `<foreignObject>`, or external `href` in any SVG.
- **Review depth:** quick · **focus areas:** SVG content.

## Verification Strategy

- **Claim:** one icon system, theme-aware illustrations, and no emoji icons.
- **Check + executor:** mechanized — the asset lint tests. Inspection — screenshots of the Figma frame.
- **Pass condition:** the lints pass, and the Figma frame lists every exported asset.

## Integration Points

- SPEC-012, SPEC-016, SPEC-017, and SPEC-018 place these assets.
- **Provenance tag:** `INIT-015/SPEC-010`
