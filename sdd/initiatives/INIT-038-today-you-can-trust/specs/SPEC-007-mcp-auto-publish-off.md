# SPEC-007 · mcp: auto-publish off, and tool text that stops restating rules

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-038
title: "Auto-publish off, and tool text that stops restating rules"
domain: mcp
status: complete
primary_prompt: none
complexity: low
complexity_reason: a config switch on one publish path and text removals
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-001]
blocks: [SPEC-013]
```

## Description

- **The switch.** A config value, `LearnerAutoPublish`, is added to the `internal/app` Config, read in `cmd/hebrew/config.go`, and is off by default.
- **While it is off:**
  - `put_unit` with `publish: true` on a learner scope saves a draft and returns a note naming ADR-0033 (`publishLearnerDraft`, `internal/mcp/tool_publish_unit.go:120`; `AutoPublishLearner`, `internal/mcpstore/learner.go:136`);
  - `publish_unit` refuses a learner unit with the same note.

  Shared units are unchanged. One config change undoes it.
- **Tool text stops restating rules.** The server instructions (`internal/mcp/server.go`) and the tool descriptions (`internal/mcp/tools.go`) stop restating the "known" rule and its version. The `get_context` description is already wrong. They point to the `ruleVersion` each response carries, so a rule change no longer needs a matching text edit.

**What Brian will see differently:** no AI-written lesson can reach Today. Today shows none now, so this turns that into a guarantee rather than a visible change.

**Removes:** the restated rule text in the server instructions and the tool descriptions.

**Scope (out):** generating docs from the rule catalog (Stage 2).

## Acceptance criteria

- [ ] With the switch off, `put_unit` with `publish: true` on `learner:1` saves a draft, publishes nothing, and returns the ADR-0033 note.
- [ ] With the switch off, `publish_unit` on a learner unit refuses.
- [ ] With the switch on, both behave as they do today.
- [ ] A shared unit publishes as before with the switch off.
- [ ] No server instruction or tool description contains the known-rule thresholds or a hard-coded rule version.

## Verification Strategy

- **Claim:** no learner unit can be published while the switch is off, and the tool text cannot drift from the rule.
- **Check + executor:** mechanized. `go test ./internal/mcp/... ./internal/mcpstore/...` with the switch both ways, and a text check over `server.go` and `tools.go`.
- **Pass condition:** every acceptance case passes.
