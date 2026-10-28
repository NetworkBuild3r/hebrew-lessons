# SPEC-001 · frontend: Play buttons that carry their audio

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-023
title: "Play buttons that carry their audio"
domain: frontend
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 1 hour
blocked_by: []
blocks: [SPEC-007]
```

## Description

**Context:** `canSayItem` in `progress.templ` and the verse header in `isaiah.templ` call `orbButton(..., "")` with no `<audio>` beside it. `clips.js` `audioOf` looks up `data-audio` or the enclosing `.clip`, finds nothing, and calls `fail`, which paints `data-state="error"`: the crossed-out speaker.

**Scope (in):**
- Render both through `clip(id, src, label, false)` with a unique id per row (`cansay-<n>`, `verse-clip`).
- Keep the `w.Audio != ""` guard, so a word with no clip shows no button.
- Add a test that fails when any `orb` button on a page has neither a `data-audio` that names an `<audio>` on the page nor a `.clip` parent.

**Scope (out):** New clips for words that have none (SPEC-009).

## Acceptance Criteria

- [ ] ac-1: On `/progress`, every `cansay__item` with audio contains `<audio … src="/audio/<id>.mp3">` inside the same `.clip` as its orb.
- [ ] ac-2: The Isaiah verse orb sits in a `.clip` with its `<audio>`.
- [ ] ac-3: The page-wide orb check passes for Progress, Isaiah, Today, a lesson page, and Roots.
- [ ] ac-4: In the browser, tapping פֹּה under "You can now say" moves the orb to `playing`, and it never reaches `error`.

## Deliverables

- [ ] `internal/web/views/progress.templ` and `progress_templ.go`
- [ ] `internal/web/views/isaiah.templ` and `isaiah_templ.go`
- [ ] `internal/web/static/css/progress.css` (orb size inside the row, if needed)
- [ ] `internal/web/orb_audio_test.go`

## Verification Strategy

- **Claim:** every orb has the audio it plays.
- **Check + executor:** mechanized — `go test ./internal/web -run Orb`. Browser — load `/progress` in the local container and tap a "You can now say" word.
- **Pass condition:** ac-1 to ac-3 tests pass, and the tap plays.

## Integration Points

- **Provenance tag:** `INIT-023/SPEC-001`
