# SPEC-010 · api: an MCP tool to add sound pairs and their clips

## Metadata

```yaml
spec_id: SPEC-010
initiative_id: INIT-036
title: "An MCP tool to add sound pairs and their clips"
domain: api
status: planned
primary_prompt: none
complexity: medium
complexity_reason: a new admin write tool on the MCP surface, tied to audio generation and its credit budget
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-009]
blocks: [SPEC-011]
```

## Description

Add `upsert_sound_pairs` to the MCP server, in the shape of `upsert_lexemes` and `upsert_noun_forms`.

- **Admin only.** A learner token gets `forbidden: admin only`.
- Input: `contrast` (a slug that `docs/sound-contrasts.md` marks approved) and a list of `{a, b}` pointed words.
- Each word must be an exact pointed lexicon entry. A missing word is an issue, not an insert.
- Each saved pair gets clips for both words in both voices through the existing audio path (`generate_audio`'s store), skipping texts that already have a clip. A per-call cap on new clips guards the voice-credit budget.
- `get_context` or a new read field lists the approved contrasts and the pair count for each, so the author and the tutor can see the bank. The tool list and its locked-names test (INIT-031 SPEC-003) are updated.
- Descriptions say plainly that the tool trains hearing, and that homophone letter pairs are rejected by the inventory.

**Scope (out):** serving a pair to a learner (SPEC-011).

## Acceptance criteria

- [ ] A learner token calling the tool is refused and nothing is written.
- [ ] A contrast not approved in the inventory is rejected with a clear issue.
- [ ] A word not in the lexicon is reported and skipped; the other pairs in the call are saved.
- [ ] Clips exist for both words in both voices after a call, and a repeat call generates none.
- [ ] A call that would exceed the clip cap saves what fits and reports the rest.
- [ ] The MCP tool-list test lists the new name.

## Security

- **Sensitivity:** standard. New admin write surface.
- [ ] Authorization is checked before any write or audio spend.
- [ ] Inputs are checked against the lexicon; no raw markup reaches storage.

## Verification Strategy

- **Claim:** only an admin can add pairs, only for approved contrasts and real words, within the clip cap.
- **Check + executor:** mechanized. MCP tool tests with a learner token and an admin token; the audio store is faked so no credit is spent; `go test ./internal/mcp/... ./internal/audio/...`.
- **Pass condition:** every acceptance case has a test, and the learner-token test asserts zero rows and zero clip requests.
