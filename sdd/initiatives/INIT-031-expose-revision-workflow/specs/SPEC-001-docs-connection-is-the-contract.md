# SPEC-001 · docs: the agent owns its lessons, the connection feeds the learner

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-031
title: "ADR: the agent owns its lessons, the connection feeds the learner"
domain: docs
status: planned
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 1 hour
blocked_by: []
blocks: [SPEC-002]
```

## Description

ADR-0027 records the initiative's decisions. Confirm 0027 is free before writing it. It extends ADR-0026 and supersedes that record's rule that only an untouched unit may be revised.

The record says:

- The MCP connection returns learner data: progress, accuracy, struggles, and the text of lessons the agent already published.
- The agent decides. It may replace, unpublish, delete, or recombine any learner unit it published, including one the learner has started or finished.
- Shared units and another learner's units are out of scope.
- A field the tool list does not name is not exposed.
- This initiative writes no live lesson.

`docs/north-star.md` section 8 gets one sentence: the lesson connection gives the agent the learner's data, and the agent may rebuild any learner unit it published.

## Acceptance criteria

- [ ] `docs/decisions/ADR-0027-connection-is-the-contract.md` exists, extends ADR-0026, and names the superseded "untouched only" rule.
- [ ] It lists replace, unpublish, delete, and recombine as agent actions, and progress as data rather than a lock.
- [ ] It says this initiative publishes no lesson.
- [ ] North-star section 8 has the one sentence.
