# SPEC-002 · web: the explainer dialog is hidden until it is opened

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-032
title: "The explainer dialog is hidden until it is opened"
domain: web
status: built, uncommitted
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 1 hour
blocked_by: []
blocks: [SPEC-006]
```

## Description

`.how-sheet` sets `display: flex`. A closed `<dialog>` is hidden only by the browser's own `display: none`, and the author rule wins, so the closed explainer is painted over the first step of every lesson that has one. Close does nothing because the dialog was never opened.

Add `.how-sheet:not([open]) { display: none; }` after the `.how-sheet` block in `internal/web/static/css/learn.css`. The open dialog keeps its flex layout.

Built in the working tree:

- `internal/web/static/css/learn.css`: the rule above.
- `internal/web/learn_roots_test.go`: `TestLearnPageAndFirstStepLink` fails if the rule is missing.
- `internal/web/testdata/learn-roots-browser.mjs`: the sheet is hidden before it is opened, and hidden after Escape.

Still to do: run the browser script (`learn-roots-browser.mjs`). It was not run in this session.

## Acceptance criteria

- [x] The CSS hides a closed `.how-sheet`.
- [x] `go test ./internal/web/...` passes.
- [ ] The browser script reports "closed sheet is hidden" and "sheet is hidden after Escape" as passing.
- [ ] At 390 px and 1440 px, the first step of a lesson with an explainer shows no panel until "How this works" is tapped.
