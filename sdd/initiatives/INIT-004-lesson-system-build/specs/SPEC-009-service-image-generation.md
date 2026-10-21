# SPEC-009 · Service: OpenAI image client

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-004
title: "OpenAI image client: dry-run, caps, and confirm"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 4 hours
blocked_by: [SPEC-008]
blocks: [SPEC-011, SPEC-012]
```

---

## Description

**Context:** ADR-0004 §7 pins OpenAI Images, model `gpt-image-1`, low quality, 1024×1024, at $0.011 per image. The browser and the MCP process do not hold the key. This server calls the provider. Tests use a fake client.

**Scope (in):**

- `planImage` returns the count and the estimated dollars (count times 0.011). It does not call the network.
- `generateImage` requires `confirm: true` and the estimate from the dry-run of that same prompt. Without confirm, it does not call out.
- Missing `OPENAI_API_KEY`: refuse both, and do not call out.
- Caps: 15 generation calls per clock hour shared by admin and `mcp`; $1 estimated spend per UTC day. A call that would pass either cap is refused before the provider. A call that contacts the provider counts, including a provider error after send. The dry-run counts toward neither.
- The request uses model id `gpt-image-1` only. If the fake (or, in production, the provider) rejects that id, generation stays disabled. The code must not substitute another model.
- Prompt scope: one ordinary object or action. Refuse a prompt that asks for a sexual act, a minor, or a real person's likeness. Learner free text is never the prompt. Adult lesson wording is not sent as a prompt.
- On success, pass the bytes to SPEC-008 ingest. The media row stores prompt, provider `openai`, model `gpt-image-1`, actor, and time. Status `pending`.
- The key is read from the environment at call time. It is not written to a row, a log, or an error body.

**Scope (out):** putting the key in Kubernetes (SPEC-017); the HTTP and MCP adapters (SPEC-011, SPEC-012).

---

## Understood (agent's read of the intent)

- [x] und-1: Tests never open a socket to `api.openai.com`. They inject a fake that records the model id and returns bytes.
- [x] und-2: The pinned price is the estimate. This spec does not fetch a price list.

## Acceptance Criteria

- [x] ac-1 (REQ-007): dry-run of one image returns `0.011` and the fake records zero calls. Generate without `confirm: true` records zero calls.
- [x] ac-2 (REQ-007): with the key unset, plan and generate throw a refusal and the fake records zero calls. The error object has no key-shaped string.
- [x] ac-3 (REQ-007): the 16th generate in an hour and a generate that would exceed $1 in the UTC day are refused with zero provider calls. A provider error after send still increments the counters.
- [x] ac-4 (REQ-007): the outbound body the fake receives has model `gpt-image-1`. A test that the client has no other model string in the request builder passes.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Provider | OpenAI, `gpt-image-1`, as ADR-0004 §7. No fallback model. | resolved | yes — ADR-0004 |

> **Readiness** = 100.

---

## Deliverables

- [x] `server/media/openai-images.js`
- [x] Tests: `test/openai-images.test.js` with an injected fake. No network.

---

## Technical Requirements (domain-specific)

- [x] `Authorization` header is built inside the real client only. The fake test does not read `process.env` for a real key. The missing-key test deletes the variable in a subprocess or a scoped save/restore.
- [x] Refusals for prompt scope are a fixed word list the ADR names (sexual act, minor, likeness). Do not send the refused prompt to the provider.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none. The secret is SPEC-017. This spec reads the variable and refuses when it is absent.
- **Blocking checks:** `node --test test/openai-images.test.js` and `npm test`.
- **Rollback plan:** delete the module. No image was stored if ingest is not called.

---

## Security

- **Sensitivity:** required. Triggers matched: generation key, outbound prompt, cost cap.
- **Security acceptance criteria:**
  - [x] The key never appears in a thrown error, a log line, or a media row.
  - [x] Learner-typed text is not a parameter of `planImage` or `generateImage`.
  - [x] Tests do not contain a real key. Fixtures use `test-key` only inside the fake's expectation when the missing-key case is not under test.
- **Review depth:** thorough · **focus areas:** secrets, SSRF is out of scope because the host is fixed, spend cap.

---

## Verification Strategy

- **Claim:** the provider is contacted only after confirm, under the caps, with the pinned model, and never from a test process.
- **Check + executor:** `mechanized` — `node --test test/openai-images.test.js`. The test fails if `globalThis.fetch` or `https.request` is invoked; the module accepts a transport function and the test supplies it.
- **Pass condition:** exit 0; a counter of transport calls matches the cases above.
- **Test levels:** unit with an injected transport. No live contract test against OpenAI. No coverage tool is configured; the bar is this file plus `npm test`.

---

## Integration Points

- SPEC-011 and SPEC-012 call `planImage` and `generateImage`. They do not build the HTTP request themselves.
- Produces **Change:** image client.
- **Provenance tag:** `INIT-004/SPEC-009`
