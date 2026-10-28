# SPEC-006 · mcp: Write and read word forms over MCP

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-023
title: "Write and read word forms over MCP"
domain: mcp
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-005]
blocks: [SPEC-009]
```

## Description

**Context:** `upsert_verb_forms` writes verb forms. Nothing writes noun forms, and `lookup` does not return a word's forms, so an author cannot see which forms are missing.

**Scope (in):**
- `upsert_noun_forms`, admin only, at most 50, all or nothing. Its schema matches SPEC-005 field by field. The description gives one possessive example and one agreement example in original words.
- `lookup` returns `forms` for a verb (tense, person, number, gender, he, translit, hasAudio) and for a noun or adjective (kind, number, person, gender, ownerNumber, he, translit, hasAudio).
- `generate_audio` with `lemmaId` makes clips for every form of that word that has none.
- `hebrew://rules` and the `hebrew-lesson-author` skill say: when a unit introduces a verb, fill at least the present tense and the infinitive. When it introduces a noun, fill the plural and the construct singular. Then call `generate_audio` with the lemma.

**Scope (out):** A learner-facing write path.

## Acceptance Criteria

- [ ] ac-1: A learner token calling `upsert_noun_forms` is refused.
- [ ] ac-2: An admin batch with a missing lemma writes nothing and names the lemma.
- [ ] ac-3: `lookup` for לִבְחוֹר returns its present forms with `hasAudio` set per form.
- [ ] ac-4: `generate_audio` with a lemma id stores a clip for each form that had none, and skips forms that already have one.
- [ ] ac-5: The tool list and schema snapshot tests include the new tool.

## Deliverables

- [ ] `internal/mcp/tool_upsert_noun_forms.go` and test
- [ ] `internal/mcp/tools.go` (`lookup`) and test
- [ ] `internal/mcp/tool_generate_audio.go` and test
- [ ] `internal/mcpstore` forms reads
- [ ] `internal/mcp/resources` rules text
- [ ] `.cursor/skills/hebrew-lesson-author/SKILL.md`

## Verification Strategy

- **Claim:** an author can see, write, and voice every form of a word.
- **Check + executor:** mechanized — `go test ./internal/mcp ./internal/mcpstore`.
- **Pass condition:** ac-1 to ac-5 pass.

## Integration Points

- **Provenance tag:** `INIT-023/SPEC-006`
