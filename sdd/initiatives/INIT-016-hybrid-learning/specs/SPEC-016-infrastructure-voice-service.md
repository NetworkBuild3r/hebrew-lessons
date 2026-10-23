# SPEC-016 · infrastructure: Voice service — credits and a male voice

## Metadata

```yaml
spec_id: SPEC-016
initiative_id: INIT-016
title: "Voice service: credits and a male voice"
domain: infrastructure
status: planned
primary_prompt: .claude/agents/devops-engineer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: [external_service]
reversibility_class: one_way
security_review: standard
estimated_effort: 1 hour
blocked_by: []
blocks: [SPEC-018]
```

## Description

**Context:** Recorded audio is a dependency (D9). The voice service is down to 3 credits, and `generate_audio` already accepts speaker `b` (male) but no male voice id is configured. Say-it-aloud needs audio for every prompt; dialogues need both voices.

**Scope (in):** the operator adds credits to the voice account, chooses a male Hebrew voice, and sets its id in the deployment's secret (home_k3), next to the female voice id. Then one `generate_audio` call with a speaker `b` line confirms it end to end. Record the credit level and monthly need estimate (clips per unit × units per week) in `docs/operations/`.

**Scope (out):** Changing the audio provider.

## Acceptance Criteria

- [ ] ac-1: A `generate_audio` call with a speaker `b` line produces a clip in the male voice on production. (REQ-017)
- [ ] ac-2: The credit balance covers at least the estimated next month of units, written down with the estimate. (REQ-017)

## Deliverables

- [ ] Secret updated in home_k3 (outside this repo)
- [ ] `docs/operations/voice-service.md` (credits, voices, estimate; no secrets)

## Gates & Controls

- **Autonomy level:** plan_only. **Gate actions:** `external_service` (buying credits, account settings) — operator only. **Rollback:** the old voice id stays valid.

## Security

- **Sensitivity:** standard — voice ids and keys live only in the secret, never in the repo or the doc.

## Verification Strategy

- **Claim:** both voices work on production and credits cover the plan.
- **Check + executor:** human — the operator runs the test call and listens; mechanized — the `generate_audio` result reports a clip for speaker `b`.
- **Pass condition:** a clip exists for the speaker `b` line, and the doc records a balance ≥ the estimate.

## Integration Points

- Unblocks SPEC-018 at scale and two-speaker dialogues in SPEC-024.
- **Provenance tag:** `INIT-016/SPEC-016`
