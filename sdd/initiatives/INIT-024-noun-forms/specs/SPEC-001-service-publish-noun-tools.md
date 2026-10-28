# SPEC-001 · service: Publish the build that stores and shows noun forms

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-024
title: "Publish the build that stores and shows noun forms"
domain: service
status: in_progress
primary_prompt: none
model: inherit
autonomy_level: manual_review
gate_actions: [Brian asks to commit, Brian asks to push]
reversibility_class: deploy
security_review: none
estimated_effort: 2 hours
blocked_by: []
blocks: [SPEC-002]
```

## Description

**Context:** `upsert_noun_forms`, form-aware `lookup`, `generate_audio` with `lemmaId`, migration 019, and the noun and adjective word page are implemented and tested in the working tree (INIT-023/SPEC-005, SPEC-006, SPEC-008). The live server at https://hebrew.ibhacked.us is the previous image. Its MCP catalog has `upsert_verb_forms` and not `upsert_noun_forms`. Its `generate_audio` accepts `texts`, `lines`, and `unitId`. A push to `main` builds `ghcr.io/networkbuild3r/hebrew` and Argo rolls that image out. The push carries the rest of the uncommitted INIT-023 tree with it.

**Scope (in):**
- Commit the working tree that contains the noun tool and the word page, when Brian asks.
- Leave out `.env`, `.cursor/mcp.json`, `data/`, `data-local/`, and `docs/decisions/ADR-0018-v3-adaptive-learning.md`.
- Push when Brian asks. Wait until the live MCP lists `upsert_noun_forms` and `lookup` can return a `forms` list.

**Scope (out):** New schema or a second noun tool. The listen replay. Rebuilding the local Compose image.

## Acceptance Criteria

- [ ] ac-1: The live MCP tool list includes `upsert_noun_forms`.
- [ ] ac-2: `generate_audio` on the live server accepts `lemmaId`.
- [ ] ac-3: `lookup` of a noun that has stored forms returns those forms. Before SPEC-002, a noun with no forms returns an empty form list and does not error.
- [ ] ac-4: `GET /words/{id}` on the live site renders for a signed-in learner. A verb that already has forms shows its grid.

## Deliverables

- [ ] The published image that already contains INIT-023/SPEC-005, SPEC-006, and SPEC-008
- [ ] A note in this spec of the image tag once it is live

## Verification Strategy

- Call the live MCP `lookup` and read the tool list after the rollout.
- Open one verb word page on the live site and confirm the grid renders. That page's data is already stored.

## Integration Points

- `INIT-024/SPEC-001`
