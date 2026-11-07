# SPEC-006 · ops: deploy and check lesson 12 on the live site

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-032
title: "Deploy and check lesson 12 on the live site"
domain: ops
status: planned
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: [deploy]
reversibility_class: remote_reversible
security_review: light
estimated_effort: 2 hours
blocked_by: [SPEC-002, SPEC-005]
blocks: [SPEC-007]
```

## Description

Commit SPEC-002 through SPEC-005 when Brian asks. Leave out the files whose diff is only line endings, `Aharon's code/`, and the `.sdd-timer.json` files. Push to `main`. Wait until `/version` shows the commit.

On https://hebrew.ibhacked.us, as Brian:

- `/lesson/12/0` step 1 has no panel over it. "How this works" opens the explainer. Close and Escape hide it.
- Step 4, the cloze on "אֲנִי גָּר.", shows "I live." above the line.
- Step 11, the one-word cloze on אֶת, shows "specific-object marker" above the gap. It stays ugly until SPEC-008, and it is answerable.

Take a screenshot of each at 390 px for the completion summary. Do not answer the lesson's questions in a way that changes Brian's progress. Read the screens only. If a step must be answered to reach the next, say so and stop.

Rollback is the previous image. This spec changes no lesson row.

## Acceptance criteria

- [ ] `/version` shows the commit.
- [ ] The three screens above match, with screenshots.
- [ ] No unit was changed.
