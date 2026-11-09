# SPEC-006 · frontend: "learned today, check tomorrow" on Progress and Today

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-036
title: "\"Learned today, check tomorrow\" on Progress and Today"
domain: frontend
status: complete
primary_prompt: none
complexity: medium
complexity_reason: two screens plus the coach digest, and a copy change on a headline number
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-003]
blocks: []
```

## Description

After SPEC-003 the headline is stricter. Show the gap instead of hiding it.

- Progress (`internal/web/views/progress.templ`, `internal/web/progress.go`) shows "Words you know" as before, and beside it a second count: "learned today, check tomorrow", from the model's waiting count. One line explains it: a word counts as known once you get it right on a second day.
- Today (`internal/web/views/today.templ`) uses the same words wherever it shows the known count.
- A history chart that crosses the rule change marks it, using the rule-version stamp, so the drop is not read as lost learning.
- The coach digest and `get_study_digest` carry the same two numbers with the same names (ADR-0028 D8: one meaning, shared by the page and the AI).

**Scope (out):** a new page, notifications, streaks.

## Acceptance criteria

- [x] Progress shows both counts, with the explanation line, and the second count is absent or zero-state when none wait.
- [x] Today never shows a known count that disagrees with Progress.
- [x] A chart spanning rule version 3 to 4 shows the change marker.
- [x] `get_study_digest` returns the waiting count under the same label.
- [x] Both counts read at 360px and in dark mode, and are announced by a screen reader as separate figures.

## Verification Strategy

- **Claim:** the page and the AI report the same two numbers with the same meaning.
- **Check + executor:** mechanized. View tests and the Progress browser test; a digest test compares the digest to the page model on one seeded learner; `go test ./internal/web/... ./internal/mcpstore/...`.
- **Pass condition:** on the seeded learner, page, Today and digest give identical counts, and the marker renders only on a mixed-version range.
